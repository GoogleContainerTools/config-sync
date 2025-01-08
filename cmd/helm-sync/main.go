// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2/textlogger"
	"kpt.dev/configsync/pkg/api/configsync"
	"kpt.dev/configsync/pkg/auth"
	"kpt.dev/configsync/pkg/helm"
	"kpt.dev/configsync/pkg/reconcilermanager"
	"kpt.dev/configsync/pkg/util"
	utillog "kpt.dev/configsync/pkg/util/log"
)

var (
	flCACert = flag.String("ca-cert", os.Getenv(reconcilermanager.HelmCACert),
		"CA cert to use for validating HTTPS connections")
	flRepo = flag.String("repo", os.Getenv(reconcilermanager.HelmRepo),
		"helm repository url where to locate the requested chart")
	flChart = flag.String("chart", os.Getenv(reconcilermanager.HelmChart),
		"the name of the helm chart being synced")
	flVersion = flag.String("version", os.Getenv(reconcilermanager.HelmChartVersion),
		"the version of the helm chart being synced")
	flValuesYAML = flag.String("values-yaml", os.Getenv(reconcilermanager.HelmValuesYAML),
		"inline helm chart values, yaml-formatted the same as the default values.yaml accompanying the chart, will be used to override the default values")
	flValuesFilePaths = flag.String("values-file-paths", os.Getenv(reconcilermanager.HelmValuesFilePaths),
		"comma-separated list of filepaths to helm chart values, will be used to override the default values")
	flIncludeCRDs = flag.String("include-crds", os.Getenv(reconcilermanager.HelmIncludeCRDs),
		"include CRDs in the helm rendering output")
	flAuth = flag.String("auth", util.EnvString(reconcilermanager.HelmAuthType, string(configsync.AuthNone)),
		fmt.Sprintf("the authentication type for access to the Helm repository. Must be one of %s, %s, %s, %s or %s. Defaults to %s",
			configsync.AuthGCPServiceAccount, configsync.AuthK8sServiceAccount, configsync.AuthToken, configsync.AuthGCENode, configsync.AuthNone, configsync.AuthNone))
	flReleaseName = flag.String("release-name", os.Getenv(reconcilermanager.HelmReleaseName),
		"the name of helm release")
	flNamespace = flag.String("namespace", os.Getenv(reconcilermanager.HelmReleaseNamespace),
		"the target namespace of helm release; sets {{.Release.Namespace}})")
	flDeployNamespace = flag.String("deployNamespace", os.Getenv(reconcilermanager.HelmDeployNamespace),
		"the namespace in which to deploy the helm chart")
	flRoot = flag.String("root", util.EnvString("HELM_SYNC_ROOT", util.EnvString("HOME", "")+"/helm"),
		"the root directory for helm-sync operations, under which --dest will be created")
	flDest = flag.String("dest", util.EnvString("HELM_SYNC_DEST", ""),
		"the path (absolute or relative to --root) at which to create a symlink to the directory holding the retrieved files (defaults to the chart name)")
	flErrorFile = flag.String("error-file", util.EnvString("HELM_SYNC_ERROR_FILE", ""),
		"the name of a file into which errors will be written under --root (defaults to \"\", disabling error reporting)")
	flWait = flag.Float64("wait", util.EnvFloat(reconcilermanager.HelmSyncWait, 1),
		"the number of seconds between syncs")
	flSyncTimeout = flag.Int("timeout", util.EnvInt("HELM_SYNC_TIMEOUT", 120),
		"the max number of seconds allowed for a complete sync")
	flOneTime = flag.Bool("one-time", util.EnvBool("HELM_SYNC_ONE_TIME", false),
		"exit after the first sync")
	flMaxSyncFailures = flag.Int("max-sync-failures", util.EnvInt("HELM_SYNC_MAX_SYNC_FAILURES", 0),
		"the number of consecutive failures allowed before aborting (the first sync must succeed, -1 will retry forever after the initial sync)")
	flUsername = flag.String("username", util.EnvString("HELM_SYNC_USERNAME", ""),
		"the username to use for helm authantication")
	flPassword = flag.String("password", util.EnvString("HELM_SYNC_PASSWORD", ""),
		"the password or personal access token to use for helm authantication")
)

func errorBackoff() wait.Backoff {
	durationLimit := math.Max(*flWait, float64(util.MinimumSyncContainerBackoffCap))
	return util.BackoffWithDurationAndStepLimit(util.WaitTime(durationLimit), math.MaxInt32)
}

func main() {
	utillog.Setup()
	log := utillog.NewLogger(textlogger.NewLogger(textlogger.NewConfig()), *flRoot, *flErrorFile)
	log.Info("rendering Helm chart with arguments", "--repo", *flRepo,
		"--chart", *flChart, "--version", *flVersion, "--root", *flRoot,
		"--values", *flValuesYAML, "--values-file-paths", *flValuesFilePaths,
		"--include-crds", *flIncludeCRDs, "--dest", *flDest, "--wait", *flWait,
		"--error-file", *flErrorFile, "--timeout", *flSyncTimeout,
		"--one-time", *flOneTime, "--max-sync-failures", *flMaxSyncFailures)

	if *flRepo == "" {
		utillog.HandleError(log, true, "ERROR: --repo must be specified")
	}

	if *flRoot == "" {
		utillog.HandleError(log, true, "ERROR: --root must be specified")
	}

	if *flDest == "" {
		*flDest = *flChart
	}

	if *flWait < 0 {
		utillog.HandleError(log, true, "ERROR: --wait must be greater than or equal to 0")
	}

	if *flUsername != "" {
		if *flPassword == "" {
			utillog.HandleError(log, true, "ERROR: --password must be set when --username is specified")
		}
	}

	initialSync := true
	failCount := 0
	backoff := errorBackoff()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(*flSyncTimeout))

		valuesFilePaths := []string{}
		if len(*flValuesFilePaths) != 0 {
			valuesFilePaths = strings.Split(*flValuesFilePaths, ",")
		}

		hydrator := &helm.Hydrator{
			Chart:           *flChart,
			Repo:            *flRepo,
			Version:         *flVersion,
			ReleaseName:     *flReleaseName,
			Namespace:       *flNamespace,
			DeployNamespace: *flDeployNamespace,
			ValuesYAML:      *flValuesYAML,
			ValuesFilePaths: valuesFilePaths,
			IncludeCRDs:     *flIncludeCRDs,
			Auth:            configsync.AuthType(*flAuth),
			HydrateRoot:     *flRoot,
			Dest:            *flDest,
			UserName:        *flUsername,
			Password:        *flPassword,
			CACertFilePath:  *flCACert,
			CredentialProvider: &auth.CachingCredentialProvider{
				Scopes: auth.OCISourceScopes(),
			},
		}

		if err := hydrator.HelmTemplate(ctx); err != nil {
			if *flMaxSyncFailures != -1 && failCount >= *flMaxSyncFailures {
				// Exit after too many retries, maybe the error is not recoverable.
				log.Error(err, "too many failures, aborting", "failCount", failCount)
				os.Exit(1)
			}

			step := backoff.Step()

			failCount++
			log.Error(err, "unexpected error rendering chart, will retry")
			log.Info("waiting before retrying", "waitTime", step)
			cancel()

			time.Sleep(step)

			continue
		}

		if initialSync {
			if *flOneTime {
				log.DeleteErrorFile()
				os.Exit(0)
			}
			initialSync = false
		}

		backoff = errorBackoff()
		failCount = 0
		log.DeleteErrorFile()
		log.Info("next sync", "wait_time", util.WaitTime(*flWait))
		cancel()
		time.Sleep(util.WaitTime(*flWait))
	}
}

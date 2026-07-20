// Copyright 2024 Google LLC
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

package e2e

import (
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	testmetrics "github.com/GoogleContainerTools/config-sync/e2e/nomostest/metrics"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/metrics"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
)

func deploymentMissingVolume(volumeName string) testpredicates.Predicate {
	return func(o client.Object) error {
		if o == nil {
			return testpredicates.ErrObjectNotFound
		}
		d, ok := o.(*appsv1.Deployment)
		if !ok {
			return testpredicates.WrongTypeErr(o, d)
		}
		for _, volume := range d.Spec.Template.Spec.Volumes {
			if volume.Name == volumeName {
				return fmt.Errorf("Deployment %s should not have volume %s", core.ObjectNamespacedName(o), volumeName)
			}
		}
		return nil
	}
}

func deploymentHasVolume(volumeName string) testpredicates.Predicate {
	return func(o client.Object) error {
		if o == nil {
			return testpredicates.ErrObjectNotFound
		}
		d, ok := o.(*appsv1.Deployment)
		if !ok {
			return testpredicates.WrongTypeErr(o, d)
		}
		for _, volume := range d.Spec.Template.Spec.Volumes {
			if volume.Name == volumeName {
				return nil
			}
		}
		return fmt.Errorf("Deployment %s should have volume %s", core.ObjectNamespacedName(o), volumeName)
	}
}

func TestDisableMonitoringRootSync(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	nt := nomostest.New(t, nomostesting.OverrideAPI,
		ntopts.SyncWithGitSource(rootSyncID, ntopts.Unstructured))

	rootReconcilerName := core.RootReconcilerObjectKey(rootSyncID.Name)
	rootSyncV1 := k8sobjects.RootSyncObjectV1Beta1(configsync.RootSyncName)

	nt.Must(nt.WatchForAllSyncs())

	// validate initial monitoring enabled (default state)
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		rootReconcilerName.Name, rootReconcilerName.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentHasContainer(metrics.OtelAgentName),
			deploymentHasVolume("otel-agent-config-reconciler-vol"),
			testpredicates.DeploymentMissingEnvVar(reconcilermanager.Reconciler, "DISABLE_MONITORING"),
		),
	))
	err := nomostest.ValidateStandardMetricsForRootSync(nt, testmetrics.Summary{Sync: rootSyncID.ObjectKey})
	if err != nil {
		t.Fatalf("Expected standard metrics to be present when monitoring is enabled, but got: %v", err)
	}

	// disable monitoring
	nt.MustMergePatch(rootSyncV1, `{"spec": {"monitoring": {"enabled": false}}}`)

	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		rootReconcilerName.Name, rootReconcilerName.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentMissingContainer(metrics.OtelAgentName),
			deploymentMissingVolume("otel-agent-config-reconciler-vol"),
			testpredicates.DeploymentHasEnvVar(reconcilermanager.Reconciler, "DISABLE_MONITORING", "true"),
		),
	))
	nt.Must(nt.WatchForAllSyncs())

	err = nomostest.ValidateStandardMetricsForRootSync(nt, testmetrics.Summary{Sync: rootSyncID.ObjectKey})
	if err == nil {
		t.Fatal("Expected an error when validating metrics for RootSync with disabled monitoring, but got nil")
	}

	// re-enable monitoring
	nt.MustMergePatch(rootSyncV1, `{"spec": {"monitoring": {"enabled": true}}}`)

	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		rootReconcilerName.Name, rootReconcilerName.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentHasContainer(metrics.OtelAgentName),
			deploymentHasVolume("otel-agent-config-reconciler-vol"),
			testpredicates.DeploymentMissingEnvVar(reconcilermanager.Reconciler, "DISABLE_MONITORING"),
		),
	))
	nt.Must(nt.WatchForAllSyncs())

	err = nomostest.ValidateStandardMetricsForRootSync(nt, testmetrics.Summary{Sync: rootSyncID.ObjectKey})
	if err != nil {
		t.Fatalf("Expected standard metrics to be present after re-enabling monitoring, but got: %v", err)
	}
}

func TestDisableMonitoringRepoSync(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, frontendNamespace)
	nt := nomostest.New(t, nomostesting.OverrideAPI,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	repoSyncKey := repoSyncID.ObjectKey

	frontendReconcilerNN := core.NsReconcilerObjectKey(repoSyncID.Namespace, repoSyncID.Name)
	repoSyncFrontend := nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, repoSyncKey)

	nt.Must(nt.WatchForAllSyncs())

	// Verify ns-reconciler-frontend uses the default monitoring state (enabled)
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		frontendReconcilerNN.Name, frontendReconcilerNN.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentHasContainer(metrics.OtelAgentName),
			deploymentHasVolume("otel-agent-config-reconciler-vol"),
			testpredicates.DeploymentMissingEnvVar(reconcilermanager.Reconciler, "DISABLE_MONITORING"),
		),
	))
	err := nomostest.ValidateStandardMetricsForRepoSync(nt, testmetrics.Summary{Sync: repoSyncID.ObjectKey})
	if err != nil {
		t.Fatalf("Expected standard metrics to be present when monitoring is enabled, but got: %v", err)
	}

	// Disable monitoring
	repoSyncFrontend.Spec.Monitoring = &v1beta1.MonitoringSpec{Enabled: ptr.To(false)}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncFrontend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Disable monitoring of frontend Reposync"))

	// validate override and make sure otel-agent is missing
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		frontendReconcilerNN.Name, frontendReconcilerNN.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentMissingContainer(metrics.OtelAgentName),
			deploymentMissingVolume("otel-agent-config-reconciler-vol"),
			testpredicates.DeploymentHasEnvVar(reconcilermanager.Reconciler, "DISABLE_MONITORING", "true"),
		),
	))
	nt.Must(nt.WatchForAllSyncs())

	err = nomostest.ValidateStandardMetricsForRepoSync(nt, testmetrics.Summary{Sync: repoSyncID.ObjectKey})
	if err == nil {
		t.Fatal("Expected an error when validating metrics for RepoSync with disabled monitoring, but got nil")
	}

	// Re-enable monitoring
	repoSyncFrontend.Spec.Monitoring = &v1beta1.MonitoringSpec{Enabled: ptr.To(true)}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncFrontend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Re-enable monitoring of frontend RepoSync"))

	// validate container comes back
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		frontendReconcilerNN.Name, frontendReconcilerNN.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentHasContainer(metrics.OtelAgentName),
			deploymentHasVolume("otel-agent-config-reconciler-vol"),
			testpredicates.DeploymentMissingEnvVar(reconcilermanager.Reconciler, "DISABLE_MONITORING"),
		),
	))
	nt.Must(nt.WatchForAllSyncs())

	err = nomostest.ValidateStandardMetricsForRepoSync(nt, testmetrics.Summary{Sync: repoSyncID.ObjectKey})
	if err != nil {
		t.Fatalf("Expected standard metrics to be present after re-enabling monitoring, but got: %v", err)
	}
}

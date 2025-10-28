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

package e2e

import (
	"fmt"
	"testing"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/gitproviders"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/syncsource"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager/controllers"
	"github.com/GoogleContainerTools/config-sync/pkg/validate/rsync/validate"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSyncingThroughAProxy(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	nt := nomostest.New(t, nomostesting.SyncSourceGit)

	nt.T.Logf("Set up the tiny proxy service and Override the RootSync object with proxy setting")
	nt.MustKubectl("apply", "-f", "../testdata/proxy/namespace.yaml")
	nt.Must(nt.Watcher.WatchForCurrentStatus(kinds.Namespace(), "proxy-test", ""))
	nt.MustKubectl("apply", "-f", "../testdata/proxy/configs")
	nt.T.Cleanup(func() {
		nt.MustKubectl("delete", "-f", "../testdata/proxy/configs")
		nt.Must(nt.Watcher.WatchForNotFound(kinds.Deployment(), "tinyproxy-deployment", "proxy-test"))
		nt.MustKubectl("delete", "-f", "../testdata/proxy/namespace.yaml")
		nt.Must(nt.Watcher.WatchForNotFound(kinds.Namespace(), "proxy-test", ""))
	})
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(), "tinyproxy-deployment", "proxy-test",
		testwatcher.WatchPredicates(hasReadyReplicas(1))))
	nt.T.Log("Verify the NoOpProxyError")
	rs := k8sobjects.RootSyncObjectV1Beta1(rootSyncID.Name)
	nt.Must(nt.Watcher.WatchForRootSyncStalledError(rs.Name, "Validation",
		validate.NoOpProxy("RootSync").Error()))

	nt.T.Log("Set auth type to cookiefile")
	nt.MustMergePatch(rs, `{"spec": {"git": {"auth": "cookiefile"}}}`)
	nt.T.Log("Verify the secretRef error")
	nt.Must(nomostest.SetupFakeSSHCreds(nt, rootSyncID.Kind, rootSyncID.ObjectKey, configsync.AuthCookieFile, controllers.GitCredentialVolume))
	nt.Must(nt.Watcher.WatchForRootSyncStalledError(rs.Name, "Validation",
		validate.MissingKeyInAuthSecret(configsync.AuthCookieFile, "cookie_file", "git-creds").Error()))
	nt.T.Log("Set auth type to token")
	nt.MustMergePatch(rs, `{"spec": {"git": {"auth": "token"}}}`)
	nt.T.Log("Verify the secretRef error")
	nt.Must(nt.Watcher.WatchForRootSyncStalledError(rs.Name, "Validation",
		validate.MissingKeyInAuthSecret(configsync.AuthToken, "token", "git-creds").Error()))
	nt.T.Log("Set auth type to githubapp")
	nt.MustMergePatch(rs, `{"spec": {"git": {"auth": "githubapp"}}}`)
	nt.T.Log("Verify the secretRef error")
	nt.Must(nt.Watcher.WatchForRootSyncStalledError(rs.Name, "Validation",
		validate.MissingKeyInAuthSecret(configsync.AuthGithubApp, "github-app-private-key", "git-creds").Error()))
	nt.T.Log("Set auth type to none")
	nt.MustMergePatch(rs, `{"spec": {"git": {"auth": "none", "secretRef": {"name":""}}}}`)

	nt.T.Log("Verify no errors")
	commit, err := nomostest.GitCommitFromSpec(nt, rs.Spec.Git)
	if err != nil {
		nt.T.Fatal(err)
	}
	nomostest.SetExpectedSyncSource(nt, rootSyncID, &syncsource.GitSyncSource{
		Repository: gitproviders.ReadOnlyRepository{
			URL: rs.Spec.Git.Repo,
		},
		Branch:            rs.Spec.Git.Branch,
		Revision:          rs.Spec.Git.Revision,
		SourceFormat:      rs.Spec.SourceFormat,
		Directory:         rs.Spec.Git.Dir,
		ExpectedDirectory: rs.Spec.Git.Dir,
		ExpectedCommit:    commit,
	})
	nt.Must(nt.WatchForAllSyncs())
}

func hasReadyReplicas(replicas int32) testpredicates.Predicate {
	return func(o client.Object) error {
		if o == nil {
			return testpredicates.ErrObjectNotFound
		}
		deployment, ok := o.(*appsv1.Deployment)
		if !ok {
			return testpredicates.WrongTypeErr(deployment, &appsv1.Deployment{})
		}
		actual := deployment.Status.ReadyReplicas
		if replicas != actual {
			return fmt.Errorf("expected %d ready replicas, but got %d", replicas, actual)
		}
		return nil
	}
}

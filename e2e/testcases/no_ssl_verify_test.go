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
	"testing"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager/controllers"
	"k8s.io/apimachinery/pkg/types"
)

func TestNoSSLVerifyV1Alpha1(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, backendNamespace)
	nt := nomostest.New(t, nomostesting.OverrideAPI,
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	nt.Must(nt.WatchForAllSyncs())

	key := controllers.GitSSLNoVerify
	rootReconcilerNN := types.NamespacedName{
		Name:      nomostest.DefaultRootReconcilerName,
		Namespace: configsync.ControllerNamespace,
	}
	nsReconcilerNN := types.NamespacedName{
		Name:      core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name),
		Namespace: configsync.ControllerNamespace,
	}

	// verify the reconciler deployments don't have the key yet
	err := validateDeploymentContainerMissingEnvVar(nt, rootReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}
	err = validateDeploymentContainerMissingEnvVar(nt, nsReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSync := k8sobjects.RootSyncObjectV1Alpha1(configsync.RootSyncName)

	repoSyncBackend := nomostest.RepoSyncObjectV1Alpha1FromNonRootRepo(nt, repoSyncID.ObjectKey)

	// Set noSSLVerify to true for root-reconciler
	nt.MustMergePatch(rootSync, `{"spec": {"git": {"noSSLVerify": true}}}`)
	err = validateDeploymentContainerHasEnvVar(nt, rootReconcilerNN,
		reconcilermanager.GitSync, key, "true")
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set noSSLVerify to true for ns-reconciler-backend
	repoSyncBackend.Spec.NoSSLVerify = true
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync NoSSLVerify to true"))
	nt.Must(nt.WatchForAllSyncs())

	err = validateDeploymentContainerHasEnvVar(nt, nsReconcilerNN,
		reconcilermanager.GitSync, key, "true")
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set noSSLVerify to false for root-reconciler
	nt.MustMergePatch(rootSync, `{"spec": {"git": {"noSSLVerify": false}}}`)
	err = validateDeploymentContainerMissingEnvVar(nt, rootReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set noSSLVerify to false from repoSyncBackend
	repoSyncBackend.Spec.NoSSLVerify = false
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync NoSSLVerify to false"))
	nt.Must(nt.WatchForAllSyncs())

	err = validateDeploymentContainerMissingEnvVar(nt, nsReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}
}

func TestNoSSLVerifyV1Beta1(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, backendNamespace)
	nt := nomostest.New(t, nomostesting.OverrideAPI,
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	nt.Must(nt.WatchForAllSyncs())

	key := controllers.GitSSLNoVerify
	rootReconcilerNN := types.NamespacedName{
		Name:      nomostest.DefaultRootReconcilerName,
		Namespace: configsync.ControllerNamespace,
	}
	nsReconcilerNN := types.NamespacedName{
		Name:      core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name),
		Namespace: configsync.ControllerNamespace,
	}

	// verify the reconciler deployments don't have the key yet
	err := validateDeploymentContainerMissingEnvVar(nt, rootReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}
	err = validateDeploymentContainerMissingEnvVar(nt, nsReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSync := k8sobjects.RootSyncObjectV1Beta1(configsync.RootSyncName)

	nn := nomostest.RepoSyncNN(repoSyncID.Namespace, repoSyncID.Name)
	repoSyncBackend := nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, nn)

	// Set noSSLVerify to true for root-reconciler
	nt.MustMergePatch(rootSync, `{"spec": {"git": {"noSSLVerify": true}}}`)
	err = validateDeploymentContainerHasEnvVar(nt, rootReconcilerNN,
		reconcilermanager.GitSync, key, "true")
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set noSSLVerify to true for ns-reconciler-backend
	repoSyncBackend.Spec.NoSSLVerify = true
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync NoSSLVerify to true"))
	nt.Must(nt.WatchForAllSyncs())

	err = validateDeploymentContainerHasEnvVar(nt, nsReconcilerNN,
		reconcilermanager.GitSync, key, "true")
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set noSSLVerify to false for root-reconciler
	nt.MustMergePatch(rootSync, `{"spec": {"git": {"noSSLVerify": false}}}`)
	err = validateDeploymentContainerMissingEnvVar(nt, rootReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set noSSLVerify to false from repoSyncBackend
	repoSyncBackend.Spec.NoSSLVerify = false
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync NoSSLVerify to false"))
	nt.Must(nt.WatchForAllSyncs())

	err = validateDeploymentContainerMissingEnvVar(nt, nsReconcilerNN,
		reconcilermanager.GitSync, key)
	if err != nil {
		nt.T.Fatal(err)
	}
}

func validateDeploymentContainerMissingEnvVar(nt *nomostest.NT, nn types.NamespacedName, container, key string) error {
	return nt.Watcher.WatchObject(kinds.Deployment(), nn.Name, nn.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentMissingEnvVar(container, key),
		),
		testwatcher.WatchTimeout(30*time.Second))
}

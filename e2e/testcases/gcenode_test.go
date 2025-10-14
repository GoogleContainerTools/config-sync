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
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/kustomizecomponents"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/policy"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/registryproviders"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testutils"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/workloadidentity"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/declared"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	syncBranch = "main"
)

// TestGCENode tests the `gcenode` auth type.
//
// The test will run on a GKE cluster only with following pre-requisites:
//
//  1. Workload Identity is NOT enabled
//  2. Access scopes for the nodes in the cluster must include `cloud-source-repos-ro`.
//  3. The Compute Engine default service account `PROJECT_ID-compute@developer.gserviceaccount.com` has `source.reader`
//     access to Cloud Source Repository.
//
// Public documentation:
// https://cloud.google.com/anthos-config-management/docs/how-to/installing-config-sync#git-creds-secret
func TestGCENodeCSR(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, testNs)
	nt := nomostest.New(t, nomostesting.SyncSourceGit,
		ntopts.RequireGKE(t), ntopts.GCENodeTest,
		ntopts.RequireCloudSourceRepository(t),
		ntopts.SyncWithGitSource(rootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(repoSyncID),
		ntopts.RepoSyncPermissions(policy.AllAdmin()), // NS reconciler manages a bunch of resources.
		ntopts.WithDelegatedControl)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)

	if err := workloadidentity.ValidateDisabled(nt); err != nil {
		nt.T.Fatal(err)
	}

	nt.T.Log("Add the kustomize-components root directory to RootSync's repo")
	nt.Must(rootSyncGitRepo.Copy("../testdata/hydration/kustomize-components", "."))
	nt.Must(rootSyncGitRepo.CommitAndPush("add DRY configs to the repository"))
	rootSync := k8sobjects.RootSyncObjectV1Beta1(configsync.RootSyncName)
	nt.MustMergePatch(rootSync, `{
		"spec": {
			"git": {
				"dir": "kustomize-components"
			}
		}
	}`)
	nomostest.SetExpectedSyncPath(nt, rootSyncID, "kustomize-components")

	nt.T.Log("Add the namespace-repo directory to RepoSync's repo")
	repoSync := k8sobjects.RepoSyncObjectV1Beta1(testNs, configsync.RepoSyncName)
	nt.Must(repoSyncGitRepo.Copy("../testdata/hydration/namespace-repo", "."))
	nt.Must(repoSyncGitRepo.CommitAndPush("add DRY configs to the repository"))
	nt.MustMergePatch(repoSync, `{
		"spec": {
			"git": {
				"dir": "namespace-repo"
			}
		}
	}`)
	nomostest.SetExpectedSyncPath(nt, repoSyncID, "namespace-repo")

	nt.Must(nt.WatchForAllSyncs())
	kustomizecomponents.ValidateAllTenants(nt, string(declared.RootScope), "base", "tenant-a", "tenant-b", "tenant-c")
	nt.Must(testutils.ReconcilerPodMissingFWICredsAnnotation(nt, nomostest.DefaultRootReconcilerName))
	kustomizecomponents.ValidateTenant(nt, repoSyncKey.Namespace, repoSyncKey.Namespace, "base")
}

// TestGCENodeOCI tests the `gcenode` auth type for the OCI image.
// The test will run on a GKE cluster only with following pre-requisites:
// 1. Workload Identity is NOT enabled
// 2. The Compute Engine default service account `PROJECT_ID-compute@developer.gserviceaccount.com` needs to have the following roles:
//   - `roles/artifactregistry.reader` for access image in Artifact Registry.
//   - `roles/containerregistry.ServiceAgent` for access image in Container Registry.
func TestGCENodeOCI(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, testNs)
	nt := nomostest.New(t, nomostesting.SyncSourceOCI,
		ntopts.RequireGKE(t), ntopts.GCENodeTest,
		ntopts.RequireOCIArtifactRegistry(t),
		ntopts.SyncWithGitSource(rootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(repoSyncID),
		ntopts.RepoSyncPermissions(policy.AllAdmin()), // NS reconciler manages a bunch of resources.
		ntopts.WithDelegatedControl)

	if err := workloadidentity.ValidateDisabled(nt); err != nil {
		nt.T.Fatal(err)
	}

	rootImage, err := nt.BuildAndPushOCIImage(
		rootSyncID.ObjectKey,
		registryproviders.ImageSourcePackage("hydration/kustomize-components"),
		registryproviders.ImageVersion("v1"))
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.T.Log("Update RootSync to sync from an OCI image in Artifact Registry")
	rootSyncOCI := nt.RootSyncObjectOCI(rootSyncID.Name, rootImage.OCIImageID().WithoutDigest(), "", rootImage.Digest)
	nt.Must(nt.KubeClient.Apply(rootSyncOCI))

	repoSyncRef := repoSyncID.ObjectKey
	nsImage, err := nt.BuildAndPushOCIImage(
		repoSyncRef,
		registryproviders.ImageSourcePackage("hydration/namespace-repo"),
		registryproviders.ImageVersion("v1"))
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.T.Log("Update RepoSync to sync from an OCI image in Artifact Registry")
	repoSyncOCI := nt.RepoSyncObjectOCI(repoSyncRef, nsImage.OCIImageID().WithoutDigest(), "", nsImage.Digest)
	nt.Must(nt.KubeClient.Apply(repoSyncOCI))

	nt.Must(nt.WatchForAllSyncs())
	kustomizecomponents.ValidateAllTenants(nt, string(declared.RootScope), "base", "tenant-a", "tenant-b", "tenant-c")
	kustomizecomponents.ValidateTenant(nt, repoSyncRef.Namespace, repoSyncRef.Namespace, "base")

	tenant := "tenant-b"
	image := privateGCRImage("kustomize-components")
	nt.T.Log("Update RootSync to sync from an OCI image in a private Google Container Registry")
	nt.MustMergePatch(rootSyncOCI, fmt.Sprintf(`{"spec": {"oci": {"image": "%s", "dir": "%s"}}}`, image, tenant))

	rootSyncDigest, err := getImageDigest(nt, image)
	if err != nil {
		nt.T.Fatal(err)
	}

	nomostest.SetExpectedSyncPath(nt, rootSyncID, tenant)
	nomostest.SetExpectedOCIImageDigest(nt, rootSyncID, rootSyncDigest)
	nt.Must(nt.WatchForAllSyncs())
	kustomizecomponents.ValidateAllTenants(nt, string(declared.RootScope), "../base", tenant)
}

// TestGCENodeHelm tests the `gcenode` auth type for the Helm repository.
// The test will run on a GKE cluster only with following pre-requisites:
// 1. Workload Identity is NOT enabled
// 2. The Compute Engine default service account `PROJECT_ID-compute@developer.gserviceaccount.com` needs to have the following role:
//   - `roles/artifactregistry.reader` for access image in Artifact Registry.
func TestGCENodeHelm(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, testNs)
	nt := nomostest.New(t, nomostesting.SyncSourceHelm,
		ntopts.RequireGKE(t), ntopts.GCENodeTest,
		ntopts.RequireHelmArtifactRegistry(t),
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(repoSyncID),
		ntopts.RepoSyncPermissions(policy.AllAdmin()), // NS reconciler manages a bunch of resources.
		ntopts.WithDelegatedControl)

	nt.Must(workloadidentity.ValidateDisabled(nt))

	rootSyncRef := nomostest.RootSyncNN(configsync.RootSyncName)
	rootChart, err := nt.BuildAndPushHelmPackage(
		rootSyncRef,
		registryproviders.HelmSourceChart(privateCoreDNSHelmChart))
	if err != nil {
		nt.T.Fatalf("failed to push helm chart: %v", err)
	}

	nt.T.Log("Update RootSync to sync from a private Artifact Registry")
	rootSyncHelm := nt.RootSyncObjectHelm(configsync.RootSyncName, rootChart.HelmChartID)
	rootSyncHelm.Spec.Helm.ReleaseName = "my-coredns"
	nt.Must(nt.KubeClient.Apply(rootSyncHelm))

	repoSyncRef := repoSyncID.ObjectKey
	nsChart, err := nt.BuildAndPushHelmPackage(repoSyncRef,
		registryproviders.HelmSourceChart("ns-chart"))
	if err != nil {
		nt.T.Fatalf("failed to push helm chart: %v", err)
	}
	nt.T.Log("Update RepoSync to sync from a helm chart")
	repoSyncHelm := nt.RepoSyncObjectHelm(repoSyncRef, nsChart.HelmChartID)
	repoSyncHelm.Spec.Helm.ReleaseName = "my-ns-chart"
	nt.Must(nt.KubeClient.Apply(repoSyncHelm))

	nt.Must(nt.WatchForAllSyncs())

	nt.Must(nt.Validate(fmt.Sprintf("%s-%s", rootSyncHelm.Spec.Helm.ReleaseName, rootChart.Name),
		"default", &appsv1.Deployment{},
		testpredicates.IsManagedBy(nt.Scheme, declared.RootScope, rootSyncRef.Name)))
	nt.Must(nt.Validate(fmt.Sprintf("%s-%s", repoSyncHelm.Spec.Helm.ReleaseName, nsChart.Name),
		repoSyncRef.Namespace, &appsv1.Deployment{},
		testpredicates.IsManagedBy(nt.Scheme, declared.Scope(repoSyncRef.Namespace), repoSyncRef.Name)))
}

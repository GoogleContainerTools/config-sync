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
	"os"
	"os/exec"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"kpt.dev/configsync/e2e"
	"kpt.dev/configsync/e2e/nomostest"
	"kpt.dev/configsync/e2e/nomostest/ntopts"
	"kpt.dev/configsync/e2e/nomostest/policy"
	nomostesting "kpt.dev/configsync/e2e/nomostest/testing"
	"kpt.dev/configsync/e2e/nomostest/testpredicates"
	"kpt.dev/configsync/e2e/nomostest/workloadidentity"
	"kpt.dev/configsync/pkg/api/configsync"
	"kpt.dev/configsync/pkg/api/configsync/v1beta1"
	"kpt.dev/configsync/pkg/declared"
	"kpt.dev/configsync/pkg/importer/filesystem"
	"kpt.dev/configsync/pkg/kinds"
	"kpt.dev/configsync/pkg/metadata"
	"kpt.dev/configsync/pkg/testing/fake"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// All the following images were built from the directory e2e/testdata/hydration/kustomize-components,
// which includes a kustomization.yaml file in the root directory that
// references resources for tenant-a, tenant-b, and tenant-c.
// Each tenant includes a NetworkPolicy, a Role and a RoleBinding.

const (
	// publicGCRImage pulls the public OCI image by the default `latest` tag
	// The test-infra GCR is public.
	publicGCRImage = nomostesting.TestInfraContainerRegistry + "/kustomize-components"

	// publicARImage pulls the public OCI image by the default `latest` tag
	publicARImage = nomostesting.ConfigSyncTestPublicRegistry + "/kustomize-components"

	// bookinfoARImage pulls the public OCI image by the default `latest` tag
	bookinfoARImage = nomostesting.ConfigSyncTestPublicRegistry + "/namespace-repo-bookinfo"
)

// privateGCRImage pulls the private OCI image by tag
// The test environment GCR is assumed to be private.
func privateGCRImage() string {
	return fmt.Sprintf("gcr.io/%s/config-sync-test/kustomize-components:v1", *e2e.GCPProject)
}

// privateARImage() pulls the private OCI image by tag
func privateARImage() string {
	return fmt.Sprintf("us-docker.pkg.dev/%s/config-sync-test-private/kustomize-components:v1", *e2e.GCPProject)
}

func gsaARReaderEmail() string {
	return fmt.Sprintf("e2e-test-ar-reader@%s.iam.gserviceaccount.com", *e2e.GCPProject)
}

func gsaGCRReaderEmail() string {
	return fmt.Sprintf("e2e-test-gcr-reader@%s.iam.gserviceaccount.com", *e2e.GCPProject)
}

// TestPublicOCI can run on both Kind and GKE clusters.
// It tests Config Sync can pull from public OCI images without any authentication.
func TestPublicOCI(t *testing.T) {
	nt := nomostest.New(t, nomostesting.SyncSource, ntopts.Unstructured)

	rs := fake.RootSyncObjectV1Beta1(configsync.RootSyncName)
	nt.T.Log("Update RootSync to sync from a public OCI image in AR")
	nt.MustMergePatch(rs, fmt.Sprintf(`{"spec": {"sourceType": "%s", "oci": {"image": "%s", "auth": "none"}, "git": null}}`,
		v1beta1.OciSource, publicARImage))
	err := nt.WatchForAllSyncs(
		nomostest.WithRootSha1Func(imageDigestFunc(publicARImage)),
		nomostest.WithSyncDirectoryMap(map[types.NamespacedName]string{
			nomostest.DefaultRootRepoNamespacedName: ".",
		}))
	if err != nil {
		nt.T.Fatal(err)
	}
	validateAllTenants(nt, string(declared.RootReconciler), "base", "tenant-a", "tenant-b", "tenant-c")

	tenant := "tenant-a"
	nt.T.Logf("Update RootSync to sync %s from a public OCI image in GCR", tenant)
	nt.MustMergePatch(rs, fmt.Sprintf(`{"spec": {"oci": {"image": "%s", "dir": "%s"}}}`, publicGCRImage, tenant))
	err = nt.WatchForAllSyncs(
		nomostest.WithRootSha1Func(imageDigestFunc(publicGCRImage)),
		nomostest.WithSyncDirectoryMap(map[types.NamespacedName]string{
			nomostest.DefaultRootRepoNamespacedName: "tenant-a",
		}))
	if err != nil {
		nt.T.Fatal(err)
	}
	validateAllTenants(nt, string(declared.RootReconciler), "../base", tenant)
}

// TestOCIGCENode tests the `gcenode` auth type for the OCI image.
// The test will run on a GKE cluster only with following pre-requisites:
// 1. Workload Identity is NOT enabled
// 2. The Compute Engine default service account `PROJECT_ID-compute@developer.gserviceaccount.com` needs to have the following roles:
//   - `roles/artifactregistry.reader` for access image in Artifact Registry.
//   - `roles/containerregistry.ServiceAgent` for access image in Container Registry.
func TestGCENodeOCI(t *testing.T) {
	nt := nomostest.New(t, nomostesting.SyncSource, ntopts.Unstructured,
		ntopts.RequireGKE(t), ntopts.GCENodeTest)

	if err := workloadidentity.ValidateDisabled(nt); err != nil {
		nt.T.Fatal(err)
	}

	tenant := "tenant-a"

	rs := fake.RootSyncObjectV1Beta1(configsync.RootSyncName)
	nt.T.Log("Update RootSync to sync from an OCI image in Artifact Registry")
	nt.MustMergePatch(rs, fmt.Sprintf(`{"spec": {"sourceType": "%s", "oci": {"dir": "%s", "image": "%s", "auth": "gcenode"}, "git": null}}`,
		v1beta1.OciSource, tenant, privateARImage()))
	err := nt.WatchForAllSyncs(
		nomostest.WithRootSha1Func(imageDigestFunc(privateARImage())),
		nomostest.WithSyncDirectoryMap(map[types.NamespacedName]string{
			nomostest.DefaultRootRepoNamespacedName: tenant,
		}))
	if err != nil {
		nt.T.Fatal(err)
	}
	validateAllTenants(nt, string(declared.RootReconciler), "../base", tenant)

	tenant = "tenant-b"
	nt.T.Log("Update RootSync to sync from an OCI image in a private Google Container Registry")
	nt.MustMergePatch(rs, fmt.Sprintf(`{"spec": {"oci": {"image": "%s", "dir": "%s"}}}`, privateGCRImage(), tenant))
	err = nt.WatchForAllSyncs(
		nomostest.WithRootSha1Func(imageDigestFunc(privateGCRImage())),
		nomostest.WithSyncDirectoryMap(map[types.NamespacedName]string{
			nomostest.DefaultRootRepoNamespacedName: tenant,
		}))
	if err != nil {
		nt.T.Fatal(err)
	}
	validateAllTenants(nt, string(declared.RootReconciler), "../base", tenant)
}

// TestOCIARGKEWorkloadIdentity tests the `gcpserviceaccount` auth type with GKE Workload Identity.
//
//	The test will run on a GKE cluster only with following pre-requisites
//
// 1. Workload Identity is enabled.
// 2. The Google service account `e2e-test-ar-reader@${GCP_PROJECT}.iam.gserviceaccount.com` is created with `roles/artifactregistry.reader` for access image in Artifact Registry.
// 3. An IAM policy binding is created between the Google service account and the Kubernetes service accounts with the `roles/iam.workloadIdentityUser` role.
//
//	gcloud iam service-accounts add-iam-policy-binding --project=${GCP_PROJECT} \
//	   --role roles/iam.workloadIdentityUser \
//	   --member "serviceAccount:${GCP_PROJECT}.svc.id.goog[config-management-system/root-reconciler]" \
//	   e2e-test-ar-reader@${GCP_PROJECT}.iam.gserviceaccount.com
//
// 4. The following environment variables are set: GCP_PROJECT, GCP_CLUSTER, GCP_REGION|GCP_ZONE.
func TestOCIARGKEWorkloadIdentity(t *testing.T) {
	testWorkloadIdentity(t, workloadIdentityTestSpec{
		fleetWITest:  false,
		crossProject: false,
		sourceRepo:   privateARImage(),
		sourceType:   v1beta1.OciSource,
		gsaEmail:     gsaARReaderEmail(),
		rootCommitFn: imageDigestFunc(privateARImage()),
	})
}

// TestOCIGCRGKEWorkloadIdentity tests the `gcpserviceaccount` auth type with GKE Workload Identity.
//
//	The test will run on a GKE cluster only with following pre-requisites
//
// 1. Workload Identity is enabled.
// 2. The Google service account `e2e-test-gcr-reader@${GCP_PROJECT}.iam.gserviceaccount.com` is created with `roles/containerregistry.ServiceAgent` for access image in Container Registry.
// 3. An IAM policy binding is created between the Google service account and the Kubernetes service accounts with the `roles/iam.workloadIdentityUser` role.
//
//	gcloud iam service-accounts add-iam-policy-binding --project=${GCP_PROJECT} \
//	   --role roles/iam.workloadIdentityUser \
//	   --member "serviceAccount:${GCP_PROJECT}.svc.id.goog[config-management-system/root-reconciler]" \
//	   e2e-test-gcr-reader@${GCP_PROJECT}.iam.gserviceaccount.com
//
// 4. The following environment variables are set: GCP_PROJECT, GCP_CLUSTER, GCP_REGION|GCP_ZONE.
func TestOCIGCRGKEWorkloadIdentity(t *testing.T) {
	testWorkloadIdentity(t, workloadIdentityTestSpec{
		fleetWITest:  false,
		crossProject: false,
		sourceRepo:   privateGCRImage(),
		sourceType:   v1beta1.OciSource,
		gsaEmail:     gsaGCRReaderEmail(),
		rootCommitFn: imageDigestFunc(privateGCRImage()),
	})
}

// TestOCIARFleetWISameProject tests the `gcpserviceaccount` auth type with Fleet Workload Identity (in-project).
//
//	The test will run on a GKE cluster only with following pre-requisites
//
// 1. Workload Identity is enabled.
// 2. The Google service account `e2e-test-ar-reader@${GCP_PROJECT}.iam.gserviceaccount.com` is created with `roles/artifactregistry.reader` for access image in Artifact Registry.
// 3. An IAM policy binding is created between the Google service account and the Kubernetes service accounts with the `roles/iam.workloadIdentityUser` role.
//
//	gcloud iam service-accounts add-iam-policy-binding --project=${GCP_PROJECT} \
//	   --role roles/iam.workloadIdentityUser \
//	   --member "serviceAccount:${GCP_PROJECT}.svc.id.goog[config-management-system/root-reconciler]" \
//	   e2e-test-ar-reader@${GCP_PROJECT}.iam.gserviceaccount.com
//
// 4. The following environment variables are set: GCP_PROJECT, GCP_CLUSTER, GCP_REGION|GCP_ZONE.
func TestOCIARFleetWISameProject(t *testing.T) {
	testWorkloadIdentity(t, workloadIdentityTestSpec{
		fleetWITest:  true,
		crossProject: false,
		sourceRepo:   privateARImage(),
		sourceType:   v1beta1.OciSource,
		gsaEmail:     gsaARReaderEmail(),
		rootCommitFn: imageDigestFunc(privateARImage()),
	})
}

// TestOCIGCRFleetWISameProject tests the `gcpserviceaccount` auth type with Fleet Workload Identity (in-project).
//
//	The test will run on a GKE cluster only with following pre-requisites
//
// 1. Workload Identity is enabled.
// 2. The Google service account `e2e-test-gcr-reader@${GCP_PROJECT}.iam.gserviceaccount.com` is created with `roles/containerregistry.ServiceAgent` for access image in Container Registry.
// 3. An IAM policy binding is created between the Google service account and the Kubernetes service accounts with the `roles/iam.workloadIdentityUser` role.
//
//	gcloud iam service-accounts add-iam-policy-binding --project=${GCP_PROJECT} \
//	   --role roles/iam.workloadIdentityUser \
//	   --member "serviceAccount:${GCP_PROJECT}.svc.id.goog[config-management-system/root-reconciler]" \
//	   e2e-test-gcr-reader@${GCP_PROJECT}.iam.gserviceaccount.com
//
// 4. The following environment variables are set: GCP_PROJECT, GCP_CLUSTER, GCP_REGION|GCP_ZONE.
func TestOCIGCRFleetWISameProject(t *testing.T) {
	testWorkloadIdentity(t, workloadIdentityTestSpec{
		fleetWITest:  true,
		crossProject: false,
		sourceRepo:   privateGCRImage(),
		sourceType:   v1beta1.OciSource,
		gsaEmail:     gsaGCRReaderEmail(),
		rootCommitFn: imageDigestFunc(privateGCRImage()),
	})
}

// TestOCIARFleetWIDifferentProject tests the `gcpserviceaccount` auth type with Fleet Workload Identity (cross-project).
//
//	The test will run on a GKE cluster only with following pre-requisites
//
// 1. Workload Identity is enabled.
// 2. The Google service account `e2e-test-ar-reader@${GCP_PROJECT}.iam.gserviceaccount.com` is created with `roles/artifactregistry.reader` for access image in Artifact Registry.
// 3. An IAM policy binding is created between the Google service account and the Kubernetes service accounts with the `roles/iam.workloadIdentityUser` role.
//
//	gcloud iam service-accounts add-iam-policy-binding --project=${GCP_PROJECT} \
//	   --role roles/iam.workloadIdentityUser \
//	   --member="serviceAccount:cs-dev-hub.svc.id.goog[config-management-system/root-reconciler]" \
//	   e2e-test-ar-reader@${GCP_PROJECT}.iam.gserviceaccount.com
//
// 4. The cross-project fleet host project 'cs-dev-hub' is created.
// 5. The following environment variables are set: GCP_PROJECT, GCP_CLUSTER, GCP_REGION|GCP_ZONE.
func TestOCIARFleetWIDifferentProject(t *testing.T) {
	testWorkloadIdentity(t, workloadIdentityTestSpec{
		fleetWITest:  true,
		crossProject: true,
		sourceRepo:   privateARImage(),
		sourceType:   v1beta1.OciSource,
		gsaEmail:     gsaARReaderEmail(),
		rootCommitFn: imageDigestFunc(privateARImage()),
	})
}

// TestOCIGCRFleetWIDifferentProject tests the `gcpserviceaccount` auth type with Fleet Workload Identity (cross-project).
//
//	The test will run on a GKE cluster only with following pre-requisites
//
// 1. Workload Identity is enabled.
// 2. The Google service account `e2e-test-gcr-reader@${GCP_PROJECT}.iam.gserviceaccount.com` is created with `roles/containerregistry.ServiceAgent` for access image in Container Registry.
// 3. An IAM policy binding is created between the Google service account and the Kubernetes service accounts with the `roles/iam.workloadIdentityUser` role.
//
//	gcloud iam service-accounts add-iam-policy-binding --project=${GCP_PROJECT} \
//	   --role roles/iam.workloadIdentityUser \
//	   --member="serviceAccount:cs-dev-hub.svc.id.goog[config-management-system/root-reconciler]" \
//	   e2e-test-gcr-reader@${GCP_PROJECT}.iam.gserviceaccount.com
//
// 4. The cross-project fleet host project 'cs-dev-hub' is created.
// 5. The following environment variables are set: GCP_PROJECT, GCP_CLUSTER, GCP_REGION|GCP_ZONE.
func TestOCIGCRFleetWIDifferentProject(t *testing.T) {
	testWorkloadIdentity(t, workloadIdentityTestSpec{
		fleetWITest:  true,
		crossProject: true,
		sourceRepo:   privateGCRImage(),
		sourceType:   v1beta1.OciSource,
		gsaEmail:     gsaGCRReaderEmail(),
		rootCommitFn: imageDigestFunc(privateGCRImage()),
	})
}

func TestSwitchFromGitToOci(t *testing.T) {
	nt := nomostest.New(t, nomostesting.SyncSource, ntopts.Unstructured,
		// bookinfo image contains RoleBinding
		// bookinfo repo contains ServiceAccount
		ntopts.RepoSyncPermissions(policy.RBACAdmin(), policy.CoreAdmin()),
	)
	var err error
	namespace := "bookinfo"
	managerScope := string(declared.RootReconciler)
	// file path to the RepoSync config in the root repository.
	repoSyncPath := "acme/reposync-bookinfo.yaml"
	rsNN := types.NamespacedName{
		Name:      configsync.RepoSyncName,
		Namespace: namespace,
	}
	repoSyncCRPath := "acme/cluster/cr.yaml"
	repoSyncRBPath := fmt.Sprintf("acme/namespaces/%s/rb-%s.yaml", rsNN.Namespace, rsNN.Name)

	rsCR := nt.RepoSyncClusterRole()
	rsRB := nomostest.RepoSyncRoleBinding(rsNN)
	repoSyncGit := nomostest.RepoSyncObjectV1Beta1(rsNN, "", filesystem.SourceFormatUnstructured)
	repoSyncGit.Spec.Git = &v1beta1.Git{
		Repo:   "https://github.com/config-sync-examples/namespace-repo-bookinfo",
		Branch: "main",
		Auth:   configsync.AuthNone,
	}
	// Ensure the RoleBinding & ClusterRole are deleted after the RepoSync
	if err := nomostest.SetDependencies(repoSyncGit, rsRB, rsCR); err != nil {
		nt.T.Fatal(err)
	}

	// To facilitate cleanup, add the implicit namespace explicitly.
	// This ensures the implicit namespace is deleted in the right order.
	t.Cleanup(func() {
		implicitNs := &corev1.Namespace{}
		implicitNs.Name = namespace
		nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncPath, implicitNs))
		nt.Must(nt.RootRepos[configsync.RootSyncName].CommitAndPush("add implicit namespace explicitly"))
		if err := nt.WatchForAllSyncs(); err != nil {
			nt.T.Fatal(err)
		}
	})

	// Verify the central controlled configuration: switch from Git to OCI
	// Backward compatibility check. Previously managed RepoSync objects without sourceType should still work.
	nt.T.Log("Add the RepoSync object to the Root Repo")
	nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncPath, repoSyncGit))
	nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncCRPath, rsCR))
	nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncRBPath, rsRB))
	nt.Must(nt.RootRepos[configsync.RootSyncName].CommitAndPush("configure RepoSync in the root repository"))
	// nt.WaitForRepoSyncs only waits for the root repo being synced because the reposync is not tracked by nt.
	if err := nt.WatchForAllSyncs(); err != nil {
		nt.T.Fatal(err)
	}
	nt.T.Log("Verify an implicit namespace is created")
	implicitNs := &corev1.Namespace{}
	if err := nt.Validate(namespace, "", implicitNs,
		testpredicates.HasAnnotation(metadata.ResourceManagerKey, managerScope),
		testpredicates.HasAnnotation(common.LifecycleDeleteAnnotation, common.PreventDeletion)); err != nil {
		nt.T.Error(err)
	}
	if err := nt.Validate(configsync.RepoSyncName, namespace, &v1beta1.RepoSync{}, isSourceType(v1beta1.GitSource)); err != nil {
		nt.T.Error(err)
	}
	nt.T.Log("Verify the namespace objects are synced")
	err = nt.WatchForSync(kinds.RepoSyncV1Beta1(), configsync.RepoSyncName, namespace,
		nomostest.RemoteNsRepoSha1Fn, nomostest.RepoSyncHasStatusSyncCommit, nil)
	if err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate("bookinfo-sa", namespace, &corev1.ServiceAccount{},
		testpredicates.HasAnnotation(metadata.ResourceManagerKey, namespace)); err != nil {
		nt.T.Error(err)
	}

	// To facilitate cleanup, revert the RepoSync to this known good state.
	// This way, the RootSync finalizer will delete the RepoSync cleanly and in
	// the right order.
	t.Cleanup(func() {
		nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncPath, repoSyncGit))
		nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncCRPath, rsCR))
		nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncRBPath, rsRB))
		nt.Must(nt.RootRepos[configsync.RootSyncName].CommitAndPush("revert RepoSync"))
		if err := nt.WatchForAllSyncs(); err != nil {
			nt.T.Fatal(err)
		}
	})

	// Switch from Git to OCI
	nt.T.Log("Update the RepoSync object to sync from OCI")
	repoSyncOCI := repoSyncGit.DeepCopy()
	repoSyncOCI.Spec.Git = nil
	repoSyncOCI.Spec.SourceType = string(v1beta1.OciSource)
	imageURL := bookinfoARImage
	repoSyncOCI.Spec.Oci = &v1beta1.Oci{
		Image: imageURL,
		Auth:  configsync.AuthNone,
	}
	// Ensure the RoleBinding & ClusterRole are deleted after the RepoSync
	if err := nomostest.SetDependencies(repoSyncOCI, rsRB, rsCR); err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncPath, repoSyncOCI))
	nt.Must(nt.RootRepos[configsync.RootSyncName].CommitAndPush("configure RepoSync to sync from OCI in the root repository"))
	if err := nt.WatchForAllSyncs(); err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate(configsync.RepoSyncName, namespace, &v1beta1.RepoSync{}, isSourceType(v1beta1.OciSource)); err != nil {
		nt.T.Error(err)
	}
	nt.T.Log("Verify the namespace objects are updated")
	err = nt.WatchForSync(kinds.RepoSyncV1Beta1(), configsync.RepoSyncName, namespace,
		imageDigestFunc(imageURL), nomostest.RepoSyncHasStatusSyncCommit, nil)
	if err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate("bookinfo-admin", namespace, &rbacv1.Role{},
		testpredicates.HasAnnotation(metadata.ResourceManagerKey, namespace)); err != nil {
		nt.T.Error(err)
	}
	if err := nt.ValidateNotFound("bookinfo-sa", namespace, &corev1.ServiceAccount{}); err != nil {
		nt.T.Error(err)
	}

	// Verify the manual configuration: switch from Git to OCI
	nt.T.Log("Remove RepoSync from the root repository")
	nt.Must(nt.RootRepos[configsync.RootSyncName].Remove(repoSyncPath))
	nt.Must(nt.RootRepos[configsync.RootSyncName].CommitAndPush("remove RepoSync from the root repository"))
	if err := nt.WatchForAllSyncs(); err != nil {
		nt.T.Fatal(err)
	}
	nt.T.Log("Verify the RepoSync object doesn't exist")
	if err := nt.ValidateNotFound(configsync.RepoSyncName, namespace, &v1beta1.RepoSync{}); err != nil {
		nt.T.Error(err)
	}
	// Verify the default sourceType is set when not specified.
	nt.T.Log("Revert the RepoSync object to sync from Git")
	if err := nt.KubeClient.Create(repoSyncGit.DeepCopy()); err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate(configsync.RepoSyncName, namespace, &v1beta1.RepoSync{}, isSourceType(v1beta1.GitSource)); err != nil {
		nt.T.Error(err)
	}
	nt.T.Log("Verify the namespace objects are synced")
	err = nt.WatchForSync(kinds.RepoSyncV1Beta1(), configsync.RepoSyncName, namespace,
		nomostest.RemoteNsRepoSha1Fn, nomostest.RepoSyncHasStatusSyncCommit, nil)
	if err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate("bookinfo-sa", namespace, &corev1.ServiceAccount{},
		testpredicates.HasAnnotation(metadata.ResourceManagerKey, namespace)); err != nil {
		nt.T.Error(err)
	}
	if err := nt.ValidateNotFound("bookinfo-admin", namespace, &rbacv1.Role{}); err != nil {
		nt.T.Error(err)
	}
	// Switch from Git to OCI
	nt.T.Log("Manually update the RepoSync object to sync from OCI")
	nt.MustMergePatch(repoSyncOCI.DeepCopy(), fmt.Sprintf(`{"spec": {"sourceType": "%s", "oci": {"image": "%s", "auth": "%s"}, "helm": null, "git": null}}`,
		v1beta1.OciSource, imageURL, configsync.AuthNone))
	if err := nt.Validate(configsync.RepoSyncName, namespace, &v1beta1.RepoSync{}, isSourceType(v1beta1.OciSource)); err != nil {
		nt.T.Error(err)
	}
	nt.T.Log("Verify the namespace objects are synced")
	err = nt.WatchForSync(kinds.RepoSyncV1Beta1(), configsync.RepoSyncName, namespace,
		imageDigestFunc(imageURL), nomostest.RepoSyncHasStatusSyncCommit, nil)
	if err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate("bookinfo-admin", namespace, &rbacv1.Role{},
		testpredicates.HasAnnotation(metadata.ResourceManagerKey, namespace)); err != nil {
		nt.T.Error(err)
	}
	if err := nt.ValidateNotFound("bookinfo-sa", namespace, &corev1.ServiceAccount{}); err != nil {
		nt.T.Error(err)
	}
	nt.T.Cleanup(func() {
		// Reset RepoSync OCI config to be valid and managed by RootSync
		nt.Must(nt.RootRepos[configsync.RootSyncName].Add(repoSyncPath, repoSyncOCI))
		nt.Must(nt.RootRepos[configsync.RootSyncName].CommitAndPush("re-configure RepoSync to sync from OCI in the root repository"))
		if err := nt.WatchForAllSyncs(); err != nil {
			nt.T.Fatal(err)
		}
	})

	// Invalid cases
	rs := fake.RepoSyncObjectV1Beta1(namespace, configsync.RepoSyncName)
	nt.T.Log("Manually patch RepoSync object to miss Git spec when sourceType is git")
	nt.MustMergePatch(rs, `{"spec":{"sourceType":"git", "git":null, "oci": null}}`)
	nt.WaitForRepoSyncStalledError(namespace, configsync.RepoSyncName, "Validation", `KNV1061: RepoSyncs must specify spec.git when spec.sourceType is "git"`)
	nt.T.Log("Manually patch RepoSync object to miss OCI spec when sourceType is oci")
	nt.MustMergePatch(rs, `{"spec":{"sourceType":"oci"}}`)
	nt.WaitForRepoSyncStalledError(namespace, configsync.RepoSyncName, "Validation", `KNV1061: RepoSyncs must specify spec.oci when spec.sourceType is "oci"`)
}

// resourceQuotaHasHardPods validates if the RepoSync has the expected sourceType.
func isSourceType(sourceType v1beta1.SourceType) testpredicates.Predicate {
	return func(o client.Object) error {
		if o == nil {
			return testpredicates.ErrObjectNotFound
		}
		rs, ok := o.(*v1beta1.RepoSync)
		if !ok {
			return testpredicates.WrongTypeErr(rs, &v1beta1.RepoSync{})
		}
		actual := rs.Spec.SourceType
		if string(sourceType) != actual {
			return fmt.Errorf("RepoSync sourceType %q is not equal to the expected %q", actual, sourceType)
		}
		return nil
	}
}

/*
// TestDigestUpdateInAR tests if the oci-sync container can pull new digests with the same tag.
// The test requires permission to push new image to `config-sync-test-public` in the Artifact Registry,
// and permission to push new image to `config-sync-test-public` in the Container Registry.
// The test uses the current credentials (gcloud auth) when running on the GKE clusters to push new images.
func TestDigestUpdate(t *testing.T) {
	nt := nomostest.New(t, nomostesting.SyncSource, ntopts.Unstructured, ntopts.RequireGKE(t))

	rs := fake.RootSyncObjectV1Beta1(configsync.RootSyncName)

	nt.T.Log("Test oci-sync pulling the latest image from AR when digest changes")
	testDigestUpdate(nt, "us-docker.pkg.dev/${GCP_PROJECT}/config-sync-test-public/digest-update")

	nt.T.Log("Test oci-sync pulling the latest image from GCR when digest changes")
	testDigestUpdate(nt, "gcr.io/${GCP_PROJECT}/config-sync-test/digest-update")
}

func testDigestUpdate(nt *nomostest.NT, image string) {
	auth := remote.WithAuthFromKeychain(gcrane.Keychain)
	packagePath := "../testdata/hydration/kustomize-components"
	digest, err := archiveAndPushOCIImage(image, packagePath, auth)
	if err != nil {
		nt.T.Fatal(err)
	}

	rs := fake.RootSyncObjectV1Beta1(configsync.RootSyncName)
	nt.T.Log("Update RootSync to sync from a public OCI image")
	nt.MustMergePatch(rs, fmt.Sprintf(`{"spec": {"sourceType": "%s", "oci": {"image": "%s", "auth": "none"}, "git": null}}`,
		v1beta1.OciSource, image))
	nt.WaitForRepoSyncs(nomostest.WithRootSha1Func(fixedOCIDigest(digest)),
		nomostest.WithSyncDirectoryMap(map[types.NamespacedName]string{nomostest.DefaultRootRepoNamespacedName: "."}))
	validateAllTenants(nt, string(declared.RootReconciler), "base", "tenant-a", "tenant-b", "tenant-c")

	nt.T.Log("Publish new content to the image with the same tag")
	packagePath = "../testdata/hydration/helm-components"
	newDigest, err := archiveAndPushOCIImage(image, packagePath, auth)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.WaitForRepoSyncs(nomostest.WithRootSha1Func(fixedOCIDigest(newDigest)),
		nomostest.WithSyncDirectoryMap(map[types.NamespacedName]string{nomostest.DefaultRootRepoNamespacedName: "."}))
	validateHelmComponents(nt, string(declared.RootReconciler))
}
*/

// getImageDigest uses gcloud to read the image digest of the specified image.
// Using gcloud, instead of the OCI SDK, allows authenticating with local user
// auth OR default app credentials. gcloud can authenticate with Google Artifact
// Registry and Google Container Registry or pull with anonymous auth.
// This allows reading the image digest without pulling the whole image.
// Requires a sha256 image digest.
func getImageDigest(nt *nomostest.NT, imageName string) (string, error) {
	args := []string{
		"gcloud", "container", "images", "describe",
		imageName,
		"--format", "value(image_summary.digest)",
		"--verbosity", "error", // hide the warning about using "latest"
	}
	nt.Logger.Debug(strings.Join(args, " "))
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		nt.T.Log(string(out))
		return "", err
	}
	hex, found := cutPrefix(strings.TrimSpace(string(out)), "sha256:")
	if !found {
		return "", fmt.Errorf("image %q has invalid digest %q", imageName, string(out))
	}
	return hex, nil
}

// imageDigestFunc wraps getImageDigest to return a Sha1Func that caches the
// image digest, to avoid polling the image registry unnecessarily.
func imageDigestFunc(imageName string) nomostest.Sha1Func {
	var cached bool
	var digest string
	var err error
	return func(nt *nomostest.NT, _ types.NamespacedName) (string, error) {
		if cached {
			return digest, err
		}
		digest, err = getImageDigest(nt, imageName)
		cached = true
		return digest, err
	}
}

/*
// archiveAndPushOCIImage tars and extracts (untar) image files to target directory.
// The desired version or digest must be in the imageName, and the resolved image sha256 digest is returned.
// It is used for testing only.
func archiveAndPushOCIImage(imageName string, dir string, options ...remote.Option) (string, error) {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		return "", fmt.Errorf("failed to parse reference %q: %v", imageName, err)
	}

	// Make new layer
	tarFile, err := os.CreateTemp("", "tar")
	if err != nil {
		return "", err
	}
	defer func() {
		nt.Must(_ = os.Remove(tarFile.Name()))
	}()

	if err := func() error {
		defer func() {
			_ = tarFile.Close()
		}()

		gw := gzip.NewWriter(tarFile)
		defer func() {
			_ = gw.Close()
		}()

		tw := tar.NewWriter(gw)
		defer func() {
			_ = tw.Close()
		}()

		if err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relative, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			if info.IsDir() && relative == "." {
				return nil
			}

			// TODO if info is symlink also read link target
			link := ""

			// generate tar header
			header, err := tar.FileInfoHeader(info, link)
			if err != nil {
				return err
			}

			// must provide real name
			// (see https://golang.org/src/archive/tar/common.go?#L626)
			header.Name = filepath.ToSlash(relative)

			var buf *bytes.Buffer
			// write header
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			// if not a dir, write file content
			if !info.IsDir() {
				data, err := os.Open(path)
				if err != nil {
					return err
				}
				if buf != nil {
					nt.Must(if _, err := io.Copy(tw, buf)); err != nil {
						return err
					}
				} else {
					nt.Must(if _, err := io.Copy(tw, data)); err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			return err
		}

		return nil
	}(); err != nil {
		return "", err
	}

	// Append new layer
	newLayers := []string{tarFile.Name()}
	img, err := crane.Append(empty.Image, newLayers...)
	if err != nil {
		return "", fmt.Errorf("failed to append %v: %v", newLayers, err)
	}

	if err := remote.Write(ref, img, options...); err != nil {
		return "", fmt.Errorf("pushing image %s: %v", ref, err)
	}

	// Determine the digest of the image that was pushed
	imageDigestHash, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to calculate image digest: %w", err)
	}
	return imageDigestHash.Hex, nil
}
*/

// cutPrefix is like strings.TrimPrefix, but also returns whether the prefix was
// found or not. Backported from Go 1.20.
func cutPrefix(s, prefix string) (after string, found bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

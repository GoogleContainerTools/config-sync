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
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/policy"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/registryproviders"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1alpha1"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager/controllers"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func caCertSecretPatch(sourceType configsync.SourceType, name string) string {
	return fmt.Sprintf(`{"spec": {"%s": {"caCertSecretRef": {"name": "%s"}}}}`, sourceType, name)
}

func syncURLHTTPSPatch(url string) string {
	return fmt.Sprintf(`{"spec": {"git": {"repo": "%s", "auth": "none", "secretRef": {"name": ""}}}}`,
		url)
}

func syncURLSSHPatch(url string) string {
	return fmt.Sprintf(
		`{"spec": {"git": {"repo": "%s", "auth": "ssh", "secretRef": {"name": "%s"}}}}`,
		url, controllers.GitCredentialVolume)
}

func secretDataPatch(key, value string) string {
	value64 := base64.StdEncoding.EncodeToString([]byte(value))
	return fmt.Sprintf(`{"data": {"%s": "%s"}}`, key, value64)
}

func secretDataDeletePatch(key string) string {
	return fmt.Sprintf(`{"data": {"%s": null}}`, key)
}

func TestCACertSecretRefV1Alpha1(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, backendNamespace)
	nt := nomostest.New(t, nomostesting.SyncSourceGit, ntopts.RequireLocalGitProvider,
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)
	rootSyncReconcilerName := nomostest.DefaultRootReconcilerName
	repoSyncReconcilerName := core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name)

	key := controllers.GitSSLCAInfo
	caCertSecret := nomostest.PublicCertSecretName(nomostest.GitSyncSource)
	caCertPath := "/etc/ca-cert/cert"
	var err error

	// verify the deployment doesn't have the key yet
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}
	err = nt.Validate(core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name), configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSync := k8sobjects.RootSyncObjectV1Alpha1(configsync.RootSyncName)
	repoSyncBackend := nomostest.RepoSyncObjectV1Alpha1FromNonRootRepo(nt, repoSyncKey)

	// Set RootSync SyncURL to use HTTPS
	rootSyncHTTPS := "https://test-git-server.config-management-system-test/git/config-management-system/root-sync"
	nt.MustMergePatch(rootSync, syncURLHTTPSPatch(rootSyncHTTPS))
	// RootSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, rootSyncHTTPS))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set caCertSecret for RootSync
	nt.MustMergePatch(rootSync, caCertSecretPatch(configsync.GitSource, caCertSecret))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, key, caCertPath))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set RepoSync to use HTTPS
	repoSyncHTTPS := "https://test-git-server.config-management-system-test/git/backend/repo-sync"
	repoSyncBackend.Spec.Git.Repo = repoSyncHTTPS
	repoSyncBackend.Spec.Git.Auth = "none"
	repoSyncBackend.Spec.Git.SecretRef = &v1alpha1.SecretReference{}

	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync use HTTPS"))
	// RepoSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(repoSyncID.Namespace, repoSyncID.Name, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(repoSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, repoSyncHTTPS))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set caCertSecret for RepoSync
	repoSyncBackend.Spec.Git.CACertSecretRef = &v1alpha1.SecretReference{Name: caCertSecret}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync set caCertSecret"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(repoSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, key, caCertPath))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Unset caCertSecret for RootSync
	nt.MustMergePatch(rootSync, caCertSecretPatch(configsync.GitSource, ""))
	// RootSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set RootSync to use SSH again
	rootSyncSSHURL := rootSyncGitRepo.SyncURL()
	nt.MustMergePatch(rootSync, syncURLSSHPatch(rootSyncSSHURL))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, rootSyncSSHURL))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Unset caCertSecret for repoSyncBackend
	repoSyncBackend.Spec.Git.CACertSecretRef.Name = ""
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync unset caCertSecret"))
	// RepoSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(repoSyncID.Namespace, repoSyncID.Name, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(repoSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set RepoSync to use SSH again
	repoSyncSSHURL := repoSyncGitRepo.SyncURL()
	repoSyncBackend.Spec.Git.Repo = repoSyncSSHURL
	repoSyncBackend.Spec.Git.Auth = "ssh"
	repoSyncBackend.Spec.Git.SecretRef = &v1alpha1.SecretReference{Name: "ssh-key"}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync use SSH"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(repoSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, repoSyncSSHURL))
	if err != nil {
		nt.T.Fatal(err)
	}
}

func TestCACertSecretRefV1Beta1(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, backendNamespace)
	nt := nomostest.New(t, nomostesting.SyncSourceGit, ntopts.RequireLocalGitProvider,
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)
	rootSyncReconcilerName := nomostest.DefaultRootReconcilerName

	key := controllers.GitSSLCAInfo
	caCertSecret := nomostest.PublicCertSecretName(nomostest.GitSyncSource)
	caCertPath := "/etc/ca-cert/cert"
	var err error

	// verify the deployment doesn't have the key yet
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}
	err = nt.Validate(core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name), configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSync := k8sobjects.RootSyncObjectV1Beta1(configsync.RootSyncName)
	repoSyncBackend := nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, repoSyncKey)
	reconcilerName := core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name)

	// Set RootSync SyncURL to use HTTPS
	rootSyncHTTPS := "https://test-git-server.config-management-system-test/git/config-management-system/root-sync"
	nt.MustMergePatch(rootSync, syncURLHTTPSPatch(rootSyncHTTPS))
	// RootSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, rootSyncHTTPS))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set caCertSecret for RootSync
	nt.MustMergePatch(rootSync, caCertSecretPatch(configsync.GitSource, caCertSecret))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, key, caCertPath))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set RepoSync to use HTTPS
	repoSyncHTTPS := "https://test-git-server.config-management-system-test/git/backend/repo-sync"
	repoSyncBackend.Spec.Git.Repo = repoSyncHTTPS
	repoSyncBackend.Spec.Git.Auth = "none"
	repoSyncBackend.Spec.Git.SecretRef = &v1beta1.SecretReference{}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync use HTTPS"))
	// RepoSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(repoSyncID.Namespace, repoSyncID.Name, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(reconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, repoSyncHTTPS))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set caCertSecret for RepoSync
	repoSyncBackend.Spec.Git.CACertSecretRef = &v1beta1.SecretReference{Name: caCertSecret}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync set caCertSecret"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(reconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, key, caCertPath))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Check that the namespace secret was upserted to c-m-s namespace
	err = nt.Validate(controllers.ReconcilerResourceName(reconcilerName, caCertSecret), configsync.ControllerNamespace, &corev1.Secret{})
	if err != nil {
		nt.T.Fatal(err)
	}

	// Unset caCertSecret for RootSync
	nt.MustMergePatch(rootSync, caCertSecretPatch(configsync.GitSource, ""))
	// RootSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set RootSync to use SSH again
	rootSyncSSHURL := rootSyncGitRepo.SyncURL()
	nt.MustMergePatch(rootSync, syncURLSSHPatch(rootSyncSSHURL))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, rootSyncSSHURL))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Unset caCertSecret for repoSyncBackend
	repoSyncBackend.Spec.Git.CACertSecretRef.Name = ""
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync unset caCertSecret"))
	// RepoSync should fail without caCertSecret
	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(repoSyncID.Namespace, repoSyncID.Name, status.SourceErrorCode, "server certificate verification failed"))
	err = nt.Validate(reconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Set RepoSync to use SSH again
	repoSyncSSHURL := repoSyncGitRepo.SyncURL()
	repoSyncBackend.Spec.Git.Repo = repoSyncSSHURL
	repoSyncBackend.Spec.Git.Auth = "ssh"
	repoSyncBackend.Spec.Git.SecretRef = &v1beta1.SecretReference{Name: "ssh-key"}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync use SSH"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(reconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, repoSyncSSHURL))
	if err != nil {
		nt.T.Fatal(err)
	}
}

func TestCACertSecretWatch(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, backendNamespace)
	nt := nomostest.New(t, nomostesting.SyncSourceGit, ntopts.RequireLocalGitProvider,
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)
	rootSyncReconcilerName := nomostest.DefaultRootReconcilerName
	repoSyncReconcilerName := core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name)

	key := controllers.GitSSLCAInfo
	caCertSecret := nomostest.PublicCertSecretName(nomostest.GitSyncSource)
	caCertPath := "/etc/ca-cert/cert"
	var err error

	// verify the deployment doesn't have the key yet
	err = nt.Validate(rootSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}
	err = nt.Validate(repoSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentMissingEnvVar(reconcilermanager.GitSync, key))
	if err != nil {
		nt.T.Fatal(err)
	}

	repoSyncBackend := nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, repoSyncKey)

	// Set RepoSync to use HTTPS with caCertSecret
	repoSyncHTTPS := "https://test-git-server.config-management-system-test/git/backend/repo-sync"
	repoSyncBackend.Spec.Git.Repo = repoSyncHTTPS
	repoSyncBackend.Spec.Git.Auth = "none"
	repoSyncBackend.Spec.Git.SecretRef = &v1beta1.SecretReference{}
	repoSyncBackend.Spec.Git.CACertSecretRef = &v1beta1.SecretReference{Name: caCertSecret}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync use HTTPS with caCertSecret"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(repoSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, key, caCertPath))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Check that the namespace secret was upserted to c-m-s namespace
	cmsSecret := &corev1.Secret{}
	cmsSecretName := controllers.ReconcilerResourceName(repoSyncReconcilerName, caCertSecret)
	err = nt.Validate(cmsSecretName, configsync.ControllerNamespace, cmsSecret)
	if err != nil {
		nt.T.Fatal(err)
	}
	// Modify the secret in c-m-s namespace
	nt.MustMergePatch(cmsSecret, secretDataPatch("foo", "bar"))
	// Check that watch triggered resync of the c-m-s secret
	nt.Must(nt.Watcher.WatchObject(kinds.Secret(), cmsSecretName, configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.SecretMissingKey("foo"),
		)))
	// Modify the secret in RepoSync namespace
	rsSecret := &corev1.Secret{}
	err = nt.Validate(caCertSecret, backendNamespace, rsSecret)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.MustMergePatch(rsSecret, secretDataPatch("baz", "bat"))
	// Check that the watch triggered upsert to c-m-s secret
	nt.Must(nt.Watcher.WatchObject(kinds.Secret(), cmsSecretName, configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.SecretHasKey("baz", "bat"),
		)))
	// Unset caCertSecret for repoSyncBackend and use SSH
	repoSyncSSHURL := repoSyncGitRepo.SyncURL()
	repoSyncBackend.Spec.Git.Repo = repoSyncSSHURL
	repoSyncBackend.Spec.Git.Auth = "ssh"
	repoSyncBackend.Spec.Git.SecretRef = &v1beta1.SecretReference{Name: "ssh-key"}
	repoSyncBackend.Spec.Git.CACertSecretRef = &v1beta1.SecretReference{}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), repoSyncBackend))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update backend RepoSync unset caCertSecret and use SSH"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.Validate(repoSyncReconcilerName, configsync.ControllerNamespace, &appsv1.Deployment{}, testpredicates.DeploymentHasEnvVar(reconcilermanager.GitSync, controllers.GitSyncRepo, repoSyncSSHURL))
	if err != nil {
		nt.T.Fatal(err)
	}
}

// TestOCICACertSecretRefRootRepo can run only run on KinD clusters.
// It tests RootSyncs can pull from OCI images using a CA certificate.
func TestOCICACertSecretRefRootRepo(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	nt := nomostest.New(t, nomostesting.SyncSourceOCI,
		ntopts.SyncWithGitSource(rootSyncID, ntopts.Unstructured),
		ntopts.RequireLocalOCIProvider)

	caCertSecret := nomostest.PublicCertSecretName(nomostest.RegistrySyncSource)

	image, err := nt.BuildAndPushOCIImage(rootSyncID.ObjectKey,
		registryproviders.ImageInputObjects(nt.Scheme, k8sobjects.NamespaceObject("foo-ns")))
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.T.Log("Set the RootSync to sync the OCI image without providing a CA cert")
	rs := nt.RootSyncObjectOCI(rootSyncID.Name, image.OCIImageID().WithoutDigest(), "", image.Digest)
	rs.Spec.Oci.Auth = configsync.AuthNone
	rs.Spec.Oci.CACertSecretRef = nil
	nt.Must(nt.KubeClient.Apply(rs))

	nt.Must(nt.Watcher.WatchForRootSyncSourceError(rootSyncID.Name, status.SourceErrorCode, "tls: failed to verify certificate: x509: certificate signed by unknown authority"))

	nt.T.Log("Add caCertSecretRef to RootSync")
	nt.MustMergePatch(rs, caCertSecretPatch(configsync.OciSource, caCertSecret))
	nt.Must(nt.WatchForAllSyncs())
}

// TestOCICACertSecretRefNamespaceRepo can run only run on KinD clusters.
// It tests RepoSyncs can pull from OCI images using a CA certificate.
func TestOCICACertSecretRefNamespaceRepo(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, backendNamespace)
	nt := nomostest.New(t, nomostesting.SyncSourceOCI,
		ntopts.RequireLocalOCIProvider,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(repoSyncID),
		ntopts.RepoSyncPermissions(policy.CoreAdmin()))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncReconcilerName := core.NsReconcilerName(repoSyncID.Namespace, repoSyncID.Name)

	caCertSecret := nomostest.PublicCertSecretName(nomostest.RegistrySyncSource)

	upsertedSecret := controllers.ReconcilerResourceName(repoSyncReconcilerName, caCertSecret)

	cm := k8sobjects.ConfigMapObject(core.Name("foo-cm"), core.Namespace(repoSyncID.Namespace))
	image, err := nt.BuildAndPushOCIImage(repoSyncKey, registryproviders.ImageInputObjects(nt.Scheme, cm))
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.T.Log("Set the RepoSync to sync the OCI image without providing a CA cert")
	gitSource := nt.SyncSources[repoSyncID] // backup git source expectation
	rs := nt.RepoSyncObjectOCI(repoSyncKey, image.OCIImageID().WithoutDigest(), "", image.Digest)
	rs.Spec.Oci.Auth = configsync.AuthNone
	rs.Spec.Oci.CACertSecretRef = nil
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), rs))
	nt.Must(rootSyncGitRepo.CommitAndPush("Set the RepoSync to use OCI without providing CA cert"))

	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(repoSyncID.Namespace, repoSyncID.Name, status.SourceErrorCode, "tls: failed to verify certificate: x509: certificate signed by unknown authority"))

	nt.T.Log("Add caCertSecretRef to RepoSync")
	rs.Spec.Oci.CACertSecretRef = &v1beta1.SecretReference{Name: caCertSecret}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), rs))
	nt.Must(rootSyncGitRepo.CommitAndPush("Set the CA cert for the RepoSync"))
	nt.Must(nt.WatchForAllSyncs())
	nt.T.Log("Verify the ConfigMap was created")
	nt.Must(nt.Validate(cm.Name, cm.Namespace, &corev1.ConfigMap{}))
	nt.T.Log("Verify the upserted Secret was created")
	nt.Must(nt.Validate(upsertedSecret, configsync.ControllerNamespace, &corev1.Secret{}))

	nt.T.Log("Set the RepoSync to sync from git")
	nt.SyncSources[repoSyncID] = gitSource // revert source expectation to git
	rs = nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, repoSyncID.ObjectKey)
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncID.Namespace, repoSyncID.Name), rs))
	nt.Must(rootSyncGitRepo.CommitAndPush("Set the RepoSync to sync from Git"))

	nt.Must(nt.WatchForAllSyncs())
	nt.T.Log("Verify the ConfigMap was pruned")
	nt.Must(nt.ValidateNotFound(cm.Name, cm.Namespace, &corev1.ConfigMap{}))
	nt.T.Log("Verify the upserted Secret was garbage collected")
	nt.Must(nt.ValidateNotFound(upsertedSecret, configsync.ControllerNamespace, &corev1.Secret{}))
}

// TestHelmCACertSecretRefRootRepo can run only run on KinD clusters.
// It tests RootSyncs can pull from OCI images using a CA certificate.
func TestHelmCACertSecretRefRootRepo(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	nt := nomostest.New(t, nomostesting.SyncSourceHelm,
		ntopts.SyncWithGitSource(rootSyncID, ntopts.Unstructured),
		ntopts.RequireLocalHelmProvider)

	caCertSecret := nomostest.PublicCertSecretName(nomostest.RegistrySyncSource)

	chart, err := nt.BuildAndPushHelmPackage(rootSyncID.ObjectKey,
		registryproviders.HelmChartObjects(nt.Scheme, k8sobjects.NamespaceObject("foo-ns")))
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.T.Log("Set the RootSync to sync the Helm package without providing a CA cert")
	rs := nt.RootSyncObjectHelm(rootSyncID.Name, chart.HelmChartID)
	rs.Spec.Helm.Auth = configsync.AuthNone
	rs.Spec.Helm.CACertSecretRef = nil
	rs.Spec.Helm.Period = metav1.Duration{Duration: 15 * time.Second}
	nt.Must(nt.KubeClient.Apply(rs))

	nt.Must(nt.Watcher.WatchForRootSyncSourceError(rootSyncID.Name, status.SourceErrorCode, "tls: failed to verify certificate: x509: certificate signed by unknown authority"))

	nt.T.Log("Add caCertSecretRef to RootSync")
	nt.MustMergePatch(rs, caCertSecretPatch(configsync.HelmSource, caCertSecret))
	nt.Must(nt.WatchForAllSyncs())
	nt.T.Log("Verify the Namespace was created")
	nt.Must(nt.Validate("foo-ns", "", &corev1.Namespace{}))
}

// TestHelmCACertSecretRefNamespaceRepo can run only run on KinD clusters.
// It tests RepoSyncs can pull from OCI images using a CA certificate.
func TestHelmCACertSecretRefNamespaceRepo(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, backendNamespace)
	nt := nomostest.New(t, nomostesting.SyncSourceHelm,
		ntopts.RequireLocalHelmProvider,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(repoSyncID),
		ntopts.RepoSyncPermissions(policy.CoreAdmin()))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	caCertSecret := nomostest.PublicCertSecretName(nomostest.RegistrySyncSource)

	repoSyncKey := repoSyncID.ObjectKey
	upsertedSecret := controllers.ReconcilerResourceName(
		core.NsReconcilerName(repoSyncKey.Namespace, repoSyncKey.Name), caCertSecret)

	cm := k8sobjects.ConfigMapObject(core.Name("foo-cm"), core.Namespace(repoSyncKey.Namespace))
	chart, err := nt.BuildAndPushHelmPackage(repoSyncKey, registryproviders.HelmChartObjects(nt.Scheme, cm))
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.T.Log("Set the RepoSync to sync the Helm package without providing a CA cert")
	gitSource := nt.SyncSources[repoSyncID] // backup git source expectation
	rs := nt.RepoSyncObjectHelm(repoSyncKey, chart.HelmChartID)
	rs.Spec.Helm.Auth = configsync.AuthNone
	rs.Spec.Helm.CACertSecretRef = nil
	rs.Spec.Helm.Period = metav1.Duration{Duration: 15 * time.Second}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncKey.Namespace, repoSyncKey.Name), rs))
	nt.Must(rootSyncGitRepo.CommitAndPush("Set the RepoSync to use Helm without providing CA cert"))

	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(repoSyncKey.Namespace, repoSyncKey.Name, status.SourceErrorCode, "tls: failed to verify certificate: x509: certificate signed by unknown authority"))

	nt.T.Log("Add caCertSecretRef to RepoSync")
	rs.Spec.Helm.CACertSecretRef = &v1beta1.SecretReference{Name: caCertSecret}
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncKey.Namespace, repoSyncKey.Name), rs))
	nt.Must(rootSyncGitRepo.CommitAndPush("Set the CA cert for the RepoSync"))
	nt.Must(nt.WatchForAllSyncs())
	nt.T.Log("Verify the ConfigMap was created")
	if err := nt.Validate(cm.Name, cm.Namespace, &corev1.ConfigMap{}); err != nil {
		nt.T.Fatal(err)
	}
	nt.T.Log("Verify the upserted Secret was created")
	if err := nt.Validate(upsertedSecret, configsync.ControllerNamespace, &corev1.Secret{}); err != nil {
		nt.T.Fatal(err)
	}

	nt.T.Log("Set the RepoSync to sync from git")
	nt.SyncSources[repoSyncID] = gitSource // revert source expectation to git
	rs = nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, repoSyncID.ObjectKey)
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(repoSyncKey.Namespace, repoSyncKey.Name), rs))
	nt.Must(rootSyncGitRepo.CommitAndPush("Set the RepoSync to sync from Git"))

	nt.Must(nt.WatchForAllSyncs())
	nt.T.Log("Verify the ConfigMap was pruned")
	if err := nt.ValidateNotFound(cm.Name, cm.Namespace, &corev1.ConfigMap{}); err != nil {
		nt.T.Fatal(err)
	}
	nt.T.Log("Verify the upserted Secret was garbage collected")
	if err := nt.ValidateNotFound(upsertedSecret, configsync.ControllerNamespace, &corev1.Secret{}); err != nil {
		nt.T.Fatal(err)
	}
}

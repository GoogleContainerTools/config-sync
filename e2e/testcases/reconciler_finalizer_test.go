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
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/policy"
	e2eretry "github.com/GoogleContainerTools/config-sync/e2e/nomostest/retry"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/taskgroup"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testutils"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/applier"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	rgmetrics "github.com/GoogleContainerTools/config-sync/pkg/resourcegroup/controllers/metrics"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheusmodel "github.com/prometheus/common/model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cli-utils/pkg/common"
	kstatus "sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// TestReconcilerFinalizer_Orphan tests that the reconciler's finalizer
// correctly handles Orphan deletion propagation. It also validates that
// when the RootSync is recreated, no objects are accidentally pruned
func TestReconcilerFinalizer_Orphan(t *testing.T) {
	nt := nomostest.New(t, nomostesting.MultiRepos)
	rootSyncID := nomostest.DefaultRootSyncID
	rootSyncKey := rootSyncID.ObjectKey
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)

	deployment1NN := types.NamespacedName{Name: "helloworld-1", Namespace: testNs}
	namespace1NN := types.NamespacedName{Name: testNs}
	safetyNamespace1NN := types.NamespacedName{Name: rootSyncGitRepo.SafetyNSName}

	nt.T.Cleanup(func() {
		cleanupSingleLevel(nt,
			rootSyncKey,
			deployment1NN,
			namespace1NN, safetyNamespace1NN)
	})

	// Add namespace to RootSync
	namespace1 := k8sobjects.NamespaceObject(namespace1NN.Name)
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(namespace1NN.Name, namespace1NN.Name), namespace1))

	// Add deployment-helloworld-1 to RootSync
	deployment1Path := nomostest.StructuredNSPath(deployment1NN.Namespace, "deployment-helloworld-1")
	deployment1 := loadDeployment(nt, "../testdata/deployment-helloworld.yaml")
	deployment1.SetName(deployment1NN.Name)
	deployment1.SetNamespace(deployment1NN.Namespace)
	nt.Must(rootSyncGitRepo.Add(deployment1Path, deployment1))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding deployment helloworld-1 to RootSync"))
	nt.Must(nt.WatchForAllSyncs())
	nt.Must(nt.Watcher.WatchForCurrentStatus(kinds.Deployment(), deployment1.Name, deployment1.Namespace))

	// Tail reconciler logs and print if there's an error.
	// This is necessary because if the RootSync are deleted, the
	// normal Cleanup won't be able to find the reconciler.
	// Start here to catch both the finalizer injection and deletion behavior.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go nomostest.TailReconcilerLogs(ctx, nt, nomostest.RootReconcilerObjectKey(rootSyncKey.Name))

	nt.T.Log("Disabling RootSync deletion propagation")
	rootSync := &v1beta1.RootSync{}
	nt.Must(nt.KubeClient.Get(rootSyncID.Name, rootSyncID.Namespace, rootSync))

	if metadata.SetDeletionPropagationPolicy(rootSync, metadata.DeletionPropagationPolicyOrphan) {
		nt.Must(nt.KubeClient.Update(rootSync))
	}
	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace(),
		testwatcher.WatchPredicates(
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus),
			testpredicates.HasFinalizer(metadata.ReconcilerFinalizer),
			testpredicates.HasAnnotation(metadata.DeletionPropagationPolicyAnnotationKey,
				metadata.DeletionPropagationPolicyOrphan.String()),
		)))

	// Delete the RootSync
	nt.Must(nt.KubeClient.Delete(rootSync))

	// The RootSync should skip finalizing and be deleted immediately
	nt.Must(nt.Watcher.WatchForNotFound(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace()))

	tg := taskgroup.New()
	tg.Go(func() error {
		// Namespace1 should NOT have been deleted, because it was orphaned by the RootSync.
		return nt.Watcher.WatchObject(kinds.Namespace(), namespace1.GetName(), namespace1.GetNamespace(),
			testwatcher.WatchPredicates(
				testpredicates.NoConfigSyncMetadata(),
			))
	})
	tg.Go(func() error {
		// Deployment1 should NOT have been deleted, because it was orphaned by the RootSync.
		return nt.Watcher.WatchObject(kinds.Deployment(), deployment1.GetName(), deployment1.GetNamespace(),
			testwatcher.WatchPredicates(
				testpredicates.NoConfigSyncMetadata(),
			))
	})
	tg.Go(func() error {
		// ResourceGroup SHOULD have been deleted
		return nt.Watcher.WatchForNotFound(kinds.ResourceGroup(), rootSync.GetName(), rootSync.GetNamespace())
	})

	nt.Must(tg.Wait())

	resetExpectedGitSync(nt, rootSyncID)

	// Recreate the RootSync
	rootSync = nomostest.RootSyncObjectV1Beta1FromRootRepo(nt, rootSyncKey.Name)
	metadata.RemoveDeletionPropagationPolicy(rootSync)
	nt.Must(nt.KubeClient.Create(rootSync))
	nt.Must(nt.WatchForAllSyncs())

	// Validate that the objects were not deleted
	nt.Must(nt.Validate(namespace1.GetName(), "", &corev1.Namespace{}))
	nt.Must(nt.Validate(deployment1.GetName(), deployment1.GetNamespace(), &appsv1.Deployment{}))
}

// TestReconcilerFinalizer_Foreground tests that the reconciler's finalizer
// correctly handles Foreground deletion propagation.
func TestReconcilerFinalizer_Foreground(t *testing.T) {
	nt := nomostest.New(t, nomostesting.MultiRepos)
	rootSyncID := nomostest.DefaultRootSyncID
	rootSyncKey := rootSyncID.ObjectKey
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)

	deployment1NN := types.NamespacedName{Name: "helloworld-1", Namespace: testNs}
	namespace1NN := types.NamespacedName{Name: testNs}
	safetyNamespace1NN := types.NamespacedName{Name: rootSyncGitRepo.SafetyNSName}

	nt.T.Cleanup(func() {
		cleanupSingleLevel(nt,
			rootSyncKey,
			deployment1NN,
			namespace1NN, safetyNamespace1NN)
	})

	// Add namespace to RootSync
	namespace1 := k8sobjects.NamespaceObject(namespace1NN.Name)
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(namespace1NN.Name, namespace1NN.Name), namespace1))

	// Add deployment-helloworld-1 to RootSync
	deployment1Path := nomostest.StructuredNSPath(deployment1NN.Namespace, "deployment-helloworld-1")
	deployment1 := loadDeployment(nt, "../testdata/deployment-helloworld.yaml")
	deployment1.SetName(deployment1NN.Name)
	deployment1.SetNamespace(deployment1NN.Namespace)
	nt.Must(rootSyncGitRepo.Add(deployment1Path, deployment1))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding deployment helloworld-1 to RootSync"))
	nt.Must(nt.WatchForAllSyncs())
	if err := nt.Watcher.WatchForCurrentStatus(kinds.Deployment(), deployment1.Name, deployment1.Namespace); err != nil {
		nt.T.Fatal(err)
	}

	// Tail reconciler logs and print if there's an error.
	// This is necessary because if the RootSync are deleted, the
	// normal Cleanup won't be able to find the reconciler.
	// Start here to catch both the finalizer injection and deletion behavior.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go nomostest.TailReconcilerLogs(ctx, nt, nomostest.RootReconcilerObjectKey(rootSyncKey.Name))

	nt.T.Log("Enabling RootSync deletion propagation")
	rootSync := k8sobjects.RootSyncObjectV1Beta1(rootSyncKey.Name)
	err := nt.KubeClient.Get(rootSync.Name, rootSync.Namespace, rootSync)
	if err != nil {
		nt.T.Fatal(err)
	}
	if metadata.SetDeletionPropagationPolicy(rootSync, metadata.DeletionPropagationPolicyForeground) {
		err = nt.KubeClient.Update(rootSync)
		if err != nil {
			nt.T.Fatal(err)
		}
	}
	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace(),
		testwatcher.WatchPredicates(
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus),
			testpredicates.HasFinalizer(metadata.ReconcilerFinalizer),
		)))

	// Delete the RootSync
	nt.T.Log("Deleting RootSync")
	err = nt.KubeClient.Delete(rootSync)
	if err != nil {
		nt.T.Fatal(err)
	}

	tg := taskgroup.New()
	// Deleting the RootSync should trigger the RootSync's finalizer to delete the Deployment1, Namespace and ResourceGroup
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.Deployment(), deployment1.GetName(), deployment1.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.Namespace(), namespace1.GetName(), namespace1.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.ResourceGroup(), rootSync.GetName(), rootSync.GetNamespace())
	})
	tg.Go(func() error {
		// After Deployment1 is deleted, the RootSync should have its finalizer removed and be garbage collected
		return nt.Watcher.WatchForNotFound(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace())
	})
	if err := tg.Wait(); err != nil {
		nt.T.Fatal(err)
	}
}

// TestReconcilerFinalizer_MultiLevelForeground tests that the reconciler's
// finalizer correctly handles multiple layers of Foreground deletion propagation.
func TestReconcilerFinalizer_MultiLevelForeground(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	repoSyncID := core.RepoSyncID("rs-test", testNs)
	nt := nomostest.New(t,
		nomostesting.MultiRepos,
		ntopts.SyncWithGitSource(repoSyncID),
		ntopts.RepoSyncPermissions(policy.AppsAdmin(), policy.CoreAdmin()), // NS Reconciler manages Deployments
	)
	rootSyncKey := rootSyncID.ObjectKey
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)

	repoSyncPath := nomostest.StructuredNSPath(repoSyncKey.Namespace, repoSyncKey.Name)
	deployment1NN := types.NamespacedName{Name: "helloworld-1", Namespace: repoSyncKey.Namespace}
	deployment2NN := types.NamespacedName{Name: "helloworld-2", Namespace: repoSyncKey.Namespace}
	namespace1NN := types.NamespacedName{Name: repoSyncKey.Namespace}
	safetyNamespace1NN := types.NamespacedName{Name: rootSyncGitRepo.SafetyNSName}
	safetyNamespace2NN := types.NamespacedName{Name: repoSyncGitRepo.SafetyNSName}

	nt.T.Cleanup(func() {
		cleanupMultiLevel(nt,
			rootSyncKey, repoSyncKey,
			deployment1NN, deployment2NN,
			namespace1NN, safetyNamespace1NN, safetyNamespace2NN)
	})

	// Namespace created for the RepoSync by test setup
	namespace1 := rootSyncGitRepo.MustGet(nt.T, nomostest.StructuredNSPath(repoSyncKey.Namespace, repoSyncKey.Namespace))

	// Add deployment-helloworld-1 to RootSync
	deployment1Path := nomostest.StructuredNSPath(deployment1NN.Namespace, "deployment-helloworld-1")
	deployment1 := loadDeployment(nt, "../testdata/deployment-helloworld.yaml")
	deployment1.SetName(deployment1NN.Name)
	deployment1.SetNamespace(deployment1NN.Namespace)
	nt.Must(rootSyncGitRepo.Add(deployment1Path, deployment1))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding deployment helloworld-1 to RootSync"))
	nt.Must(nt.WatchForAllSyncs())
	if err := nt.Watcher.WatchForCurrentStatus(kinds.Deployment(), deployment1.Name, deployment1.Namespace); err != nil {
		nt.T.Fatal(err)
	}

	// Add deployment-helloworld-2 to RepoSync
	deployment2Path := nomostest.StructuredNSPath(deployment1NN.Namespace, "deployment-helloworld-2")
	deployment2 := loadDeployment(nt, "../testdata/deployment-helloworld.yaml")
	deployment2.SetName(deployment2NN.Name)
	deployment2.SetNamespace(deployment1NN.Namespace)
	nt.Must(repoSyncGitRepo.Add(deployment2Path, deployment2))
	nt.Must(repoSyncGitRepo.CommitAndPush("Adding deployment helloworld-2 to RepoSync"))
	nt.Must(nt.WatchForAllSyncs())
	if err := nt.Watcher.WatchForCurrentStatus(kinds.Deployment(), deployment2.Name, deployment2.Namespace); err != nil {
		nt.T.Fatal(err)
	}

	// Tail reconciler logs and print if there's an error.
	// This is necessary because if the RootSync/RepoSync are deleted, the
	// normal Cleanup won't be able to find the reconcilers.
	// Start here to catch both the finalizer injection and deletion behavior.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go nomostest.TailReconcilerLogs(ctx, nt, nomostest.RootReconcilerObjectKey(rootSyncKey.Name))
	go nomostest.TailReconcilerLogs(ctx, nt, nomostest.NsReconcilerObjectKey(repoSyncKey.Namespace, repoSyncKey.Name))

	nt.T.Log("Enabling RootSync deletion propagation")
	rootSync := k8sobjects.RootSyncObjectV1Beta1(rootSyncKey.Name)
	err := nt.KubeClient.Get(rootSync.Name, rootSync.Namespace, rootSync)
	if err != nil {
		nt.T.Fatal(err)
	}
	if metadata.SetDeletionPropagationPolicy(rootSync, metadata.DeletionPropagationPolicyForeground) {
		err = nt.KubeClient.Update(rootSync)
		if err != nil {
			nt.T.Fatal(err)
		}
	}
	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace(),
		testwatcher.WatchPredicates(
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus),
			testpredicates.HasFinalizer(metadata.ReconcilerFinalizer),
		)))

	nt.T.Log("Enabling RepoSync deletion propagation")
	repoSync := rootSyncGitRepo.MustGet(nt.T, repoSyncPath)
	if metadata.SetDeletionPropagationPolicy(repoSync, metadata.DeletionPropagationPolicyForeground) {
		nt.Must(rootSyncGitRepo.Add(repoSyncPath, repoSync))
		nt.Must(rootSyncGitRepo.CommitAndPush("Enabling RepoSync deletion propagation"))
	}
	nt.Must(nt.WatchForAllSyncs())
	nt.Must(nt.Watcher.WatchObject(kinds.RepoSyncV1Beta1(), repoSync.GetName(), repoSync.GetNamespace(),
		testwatcher.WatchPredicates(
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus),
			testpredicates.HasFinalizer(metadata.ReconcilerFinalizer),
		)))

	// Delete the RootSync
	err = nt.KubeClient.Delete(rootSync)
	if err != nil {
		nt.T.Fatal(err)
	}

	tg := taskgroup.New()
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.Deployment(), deployment1.GetName(), deployment1.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.Deployment(), deployment2.GetName(), deployment2.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.RepoSyncV1Beta1(), repoSync.GetName(), repoSync.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.Namespace(), namespace1.GetName(), namespace1.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.ResourceGroup(), rootSync.GetName(), rootSync.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace())
	})
	if err := tg.Wait(); err != nil {
		nt.T.Fatal(err)
	}
}

// TestReconcilerFinalizer_MultiLevelMixed tests that the reconciler's finalizer
// correctly handles multiple layers of deletion propagation.
// The RootSync has Foreground policy, but manages a RepoSync with Orphan policy.
func TestReconcilerFinalizer_MultiLevelMixed(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	repoSyncID := core.RepoSyncID("rs-test", testNs)
	nt := nomostest.New(t,
		nomostesting.MultiRepos,
		ntopts.SyncWithGitSource(repoSyncID),
		ntopts.RepoSyncPermissions(policy.AppsAdmin(), policy.CoreAdmin()), // NS Reconciler manages Deployments
	)
	rootSyncKey := rootSyncID.ObjectKey
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)

	repoSyncPath := nomostest.StructuredNSPath(repoSyncKey.Namespace, repoSyncKey.Name)
	deployment1NN := types.NamespacedName{Name: "helloworld-1", Namespace: repoSyncKey.Namespace}
	deployment2NN := types.NamespacedName{Name: "helloworld-2", Namespace: repoSyncKey.Namespace}
	namespace1NN := types.NamespacedName{Name: repoSyncKey.Namespace}
	safetyNamespace1NN := types.NamespacedName{Name: rootSyncGitRepo.SafetyNSName}
	safetyNamespace2NN := types.NamespacedName{Name: repoSyncGitRepo.SafetyNSName}

	nt.T.Cleanup(func() {
		cleanupMultiLevel(nt,
			rootSyncKey, repoSyncKey,
			deployment1NN, deployment2NN,
			namespace1NN, safetyNamespace1NN, safetyNamespace2NN)
	})

	// Add deployment-helloworld-1 to RootSync
	deployment1Path := nomostest.StructuredNSPath(deployment2NN.Namespace, "deployment-helloworld-1")
	deployment1 := loadDeployment(nt, "../testdata/deployment-helloworld.yaml")
	deployment1.SetName(deployment1NN.Name)
	deployment1.SetNamespace(deployment2NN.Namespace)
	nt.Must(rootSyncGitRepo.Add(deployment1Path, deployment1))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding deployment helloworld-1 to RootSync"))
	nt.Must(nt.WatchForAllSyncs())
	nt.Must(nt.Watcher.WatchForCurrentStatus(kinds.Deployment(), deployment1.Name, deployment1.Namespace))

	// Add deployment-helloworld-2 to RepoSync
	deployment2Path := nomostest.StructuredNSPath(deployment2NN.Namespace, "deployment-helloworld-2")
	deployment2 := loadDeployment(nt, "../testdata/deployment-helloworld.yaml")
	deployment2.SetName(deployment2NN.Name)
	deployment2.SetNamespace(deployment2NN.Namespace)
	nt.Must(repoSyncGitRepo.Add(deployment2Path, deployment2))
	nt.Must(repoSyncGitRepo.CommitAndPush("Adding deployment helloworld-2 to RepoSync"))
	nt.Must(nt.WatchForAllSyncs())
	nt.Must(nt.Watcher.WatchForCurrentStatus(kinds.Deployment(), deployment2.Name, deployment2.Namespace))

	// Tail reconciler logs and print if there's an error.
	// This is necessary because if the RootSync/RepoSync are deleted, the
	// normal Cleanup won't be able to find the reconcilers.
	// Start here to catch both the finalizer injection and deletion behavior.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go nomostest.TailReconcilerLogs(ctx, nt, nomostest.RootReconcilerObjectKey(rootSyncKey.Name))
	go nomostest.TailReconcilerLogs(ctx, nt, nomostest.NsReconcilerObjectKey(repoSyncKey.Namespace, repoSyncKey.Name))

	nt.T.Log("Enabling RootSync deletion propagation")
	rootSync := &v1beta1.RootSync{}
	nt.Must(nt.KubeClient.Get(rootSyncID.Name, rootSyncID.Namespace, rootSync))
	if metadata.SetDeletionPropagationPolicy(rootSync, metadata.DeletionPropagationPolicyForeground) {
		nt.Must(nt.KubeClient.Update(rootSync))
	}
	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace(),
		testwatcher.WatchPredicates(
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus),
			testpredicates.HasFinalizer(metadata.ReconcilerFinalizer),
		)))

	nt.T.Log("Disabling RepoSync deletion propagation")
	repoSync := rootSyncGitRepo.MustGet(nt.T, repoSyncPath)
	if metadata.RemoveDeletionPropagationPolicy(repoSync) {
		nt.Must(rootSyncGitRepo.Add(repoSyncPath, repoSync))
		nt.Must(rootSyncGitRepo.CommitAndPush("Disabling RepoSync deletion propagation"))
		nt.Must(nt.WatchForAllSyncs())
	}
	nt.Must(nt.Watcher.WatchObject(kinds.RepoSyncV1Beta1(), repoSync.GetName(), repoSync.GetNamespace(),
		testwatcher.WatchPredicates(
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus),
			testpredicates.HasFinalizer(metadata.ReconcilerFinalizer),
			testpredicates.MissingAnnotation(metadata.DeletionPropagationPolicyAnnotationKey),
		)))

	repoSync = &v1beta1.RepoSync{}
	nt.Must(nt.KubeClient.Get(repoSyncID.Name, repoSyncID.Namespace, repoSync))

	// Abandon the test namespace, otherwise it will block the finalizer
	namespace1Path := nomostest.StructuredNSPath(namespace1NN.Name, namespace1NN.Name)
	namespace1 := rootSyncGitRepo.MustGet(nt.T, namespace1Path)
	core.SetAnnotation(namespace1, common.LifecycleDeleteAnnotation, common.PreventDeletion)
	nt.Must(rootSyncGitRepo.Add(namespace1Path, namespace1))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding annotation to keep test namespace on removal from git"))
	nt.Must(nt.WatchForAllSyncs())

	// Delete the RootSync
	nt.Must(nt.KubeClient.Delete(rootSync))

	tg := taskgroup.New()
	tg.Go(func() error {
		// Deleting the RootSync should trigger the RootSync's finalizer to delete the RepoSync and Deployment1
		return nt.Watcher.WatchForNotFound(kinds.Deployment(), deployment1.GetName(), deployment1.GetNamespace())
	})
	tg.Go(func() error {
		// RepoSync should delete quickly without finalizing
		return nt.Watcher.WatchForNotFound(kinds.RepoSyncV1Beta1(), repoSync.GetName(), repoSync.GetNamespace())
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.ResourceGroup(), rootSync.GetName(), rootSync.GetNamespace())
	})
	tg.Go(func() error {
		// After RepoSync and Deployment1 are deleted, the RootSync should have its finalizer removed and be garbage collected
		return nt.Watcher.WatchForNotFound(kinds.RootSyncV1Beta1(), rootSync.GetName(), rootSync.GetNamespace())
	})
	nt.Must(tg.Wait())

	tg = taskgroup.New()
	tg.Go(func() error {
		// Namespace1 should NOT have been deleted, because it was abandoned by the RootSync.
		return nt.Watcher.WatchObject(kinds.Namespace(), namespace1.GetName(), namespace1.GetNamespace(),
			testwatcher.WatchPredicates(
				testpredicates.NoConfigSyncMetadata(),
				testpredicates.MissingLabel(metadata.ApplySetPartOfLabel),
			))
	})
	tg.Go(func() error {
		// Deployment2 should NOT have been deleted, because it was orphaned by the RepoSync.
		return nt.Watcher.WatchObject(kinds.Deployment(), deployment2.GetName(), deployment2.GetNamespace(),
			testwatcher.WatchPredicates(
				testpredicates.NoConfigSyncMetadata(),
			))
	})
	nt.Must(tg.Wait())
}

// TestReconcileFinalizerReconcileTimeout verifies that the reconciler finalizer
// blocks deletion and continues to track objects which fail garbage collection.
// A RootSync is created which manages a single Namespace, and a fake finalizer
// is added to that Namespace from the test. Deletion of both the Namespace and
// RootSync should be blocked until the Namespace finalizer is removed.
func TestReconcileFinalizerReconcileTimeout(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	rootSync2ID := core.RootSyncID("nested-root-sync")
	namespaceNN := types.NamespacedName{Name: "managed-ns"}
	contrivedFinalizer := "e2e-test"
	nt := nomostest.New(t, nomostesting.MultiRepos,
		ntopts.WithCentralizedControl, // This test assumes centralized control
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(rootSync2ID, ntopts.Unstructured), // Create a nested RootSync to delete mid-test
		ntopts.WithReconcileTimeout(10*time.Second),                // Reconcile expected to fail, so use a short timeout
	)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)
	rootSync2GitRepo := nt.SyncSourceGitReadWriteRepository(rootSync2ID)

	// add a Namespace to the nested RootSync
	namespace := k8sobjects.NamespaceObject(namespaceNN.Name)
	nsPath := nomostest.StructuredNSPath(namespace.GetNamespace(), namespaceNN.Name)
	nt.Must(rootSync2GitRepo.Add(nsPath, namespace))
	nt.Must(rootSync2GitRepo.CommitAndPush(fmt.Sprintf("add Namespace %s", namespaceNN.Name)))
	nt.Must(nt.WatchForAllSyncs())
	nt.T.Cleanup(func() {
		_, err := e2eretry.Retry(30*time.Second, func() error {
			namespace := &corev1.Namespace{}
			if err := nt.KubeClient.Get(namespaceNN.Name, namespaceNN.Namespace, namespace); err != nil {
				if apierrors.IsNotFound(err) { // Happy path - exit
					return nil
				}
				return err // unexpected error
			}
			if testutils.RemoveFinalizer(namespace, nomostest.ConfigSyncE2EFinalizer) {
				// The test failed to remove the finalizer. Remove to enable deletion.
				if err := nt.KubeClient.Update(namespace); err != nil {
					return err
				}
				nt.T.Log("removed finalizer in test cleanup")
			}
			return nil
		})
		if err != nil {
			nt.T.Fatal(err)
		}
	})
	// Add a fake finalizer to the namespace to block deletion
	testutils.AppendFinalizer(namespace, nomostest.ConfigSyncE2EFinalizer)
	nt.T.Logf("Add a fake finalizer named %s to Namespace %s", contrivedFinalizer, namespaceNN.Name)
	if err := nt.KubeClient.Apply(namespace); err != nil {
		nt.T.Fatal(err)
	}

	// Remove the fake finalizer
	t.Cleanup(func() {
		namespace = k8sobjects.NamespaceObject(namespaceNN.Name)
		nt.T.Logf("Remove the fake finalizer named %s from Namespace %s", contrivedFinalizer, namespaceNN.Name)
		if err := nt.KubeClient.Apply(namespace); err != nil {
			nt.T.Fatal(err)
		}
		// With the finalizer removed, the deletion should reconcile
		tg := taskgroup.New()
		tg.Go(func() error {
			return nt.Watcher.WatchForNotFound(kinds.RootSyncV1Beta1(),
				rootSync2ID.Name, rootSync2ID.Namespace)
		})
		tg.Go(func() error {
			return nt.Watcher.WatchForNotFound(kinds.Namespace(),
				namespaceNN.Name, namespaceNN.Namespace)
		})
		if err := tg.Wait(); err != nil {
			nt.T.Fatal(err)
		}
	})

	// Try to remove the nested RootSync. Deletion should be blocked by ns finalizer
	nestedRootSyncPath := fmt.Sprintf("acme/namespaces/%s/%s.yaml",
		rootSync2ID.Namespace, rootSync2ID.Name)
	nt.Must(rootSyncGitRepo.Remove(nestedRootSyncPath))
	nt.Must(rootSyncGitRepo.CommitAndPush(fmt.Sprintf("remove Namespace %s", namespaceNN.Name)))
	expectedCondition := &v1beta1.RootSyncCondition{
		Type:    v1beta1.RootSyncReconcilerFinalizerFailure,
		Status:  "True",
		Reason:  "DestroyFailure",
		Message: "Failed to delete managed resource objects",
		Errors: []v1beta1.ConfigSyncError{
			{
				Code:         applier.ApplierErrorCode,
				ErrorMessage: "KNV2009: failed to wait for Namespace, /managed-ns: reconcile timeout\n\nFor more information, see https://g.co/cloud/acm-errors#knv2009",
			},
		},
	}
	// Finalizer currently only sets condition, not sync status
	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(), rootSync2ID.Name, rootSync2ID.Namespace,
		testwatcher.WatchPredicates(testpredicates.RootSyncHasCondition(expectedCondition))))
	// Wait a fixed duration for RootSync deletion to be blocked
	time.Sleep(30 * time.Second)
	if err := nt.Validate(rootSync2ID.Name, rootSync2ID.Namespace, &v1beta1.RootSync{}); err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate(namespaceNN.Name, namespaceNN.Namespace, &corev1.Namespace{}); err != nil {
		nt.T.Fatal(err)
	}
}

func cleanupSingleLevel(nt *nomostest.NT,
	rootSyncNN,
	deployment1NN,
	namespace1NN, safetyNamespace1NN types.NamespacedName,
) {
	cleanupSyncsAndObjects(nt,
		[]client.Object{
			k8sobjects.RootSyncObjectV1Beta1(rootSyncNN.Name),
		},
		[]client.Object{
			k8sobjects.DeploymentObject(core.Name(deployment1NN.Name), core.Namespace(deployment1NN.Namespace)),
			k8sobjects.NamespaceObject(namespace1NN.Name),
			k8sobjects.NamespaceObject(safetyNamespace1NN.Name),
		})
}

func cleanupMultiLevel(nt *nomostest.NT,
	rootSyncNN, repoSyncNN,
	deployment1NN, deployment2NN,
	namespace1NN, safetyNamespace1NN, safetyNamespace2NN types.NamespacedName,
) {
	cleanupSyncsAndObjects(nt,
		[]client.Object{
			k8sobjects.RootSyncObjectV1Beta1(rootSyncNN.Name),
			k8sobjects.RepoSyncObjectV1Beta1(repoSyncNN.Namespace, repoSyncNN.Name),
		},
		[]client.Object{
			k8sobjects.DeploymentObject(core.Name(deployment1NN.Name), core.Namespace(deployment1NN.Namespace)),
			k8sobjects.DeploymentObject(core.Name(deployment2NN.Name), core.Namespace(deployment2NN.Namespace)),
			k8sobjects.NamespaceObject(namespace1NN.Name),
			k8sobjects.NamespaceObject(safetyNamespace1NN.Name),
			k8sobjects.NamespaceObject(safetyNamespace2NN.Name),
		})
}

func cleanupSyncsAndObjects(nt *nomostest.NT, syncObjs []client.Object, objs []client.Object) {
	if nt.T.Failed() && *e2e.Debug {
		nt.T.Log("Skipping test cleanup: debug enabled")
		return
	}

	nt.T.Log("Stopping webhook")
	// Stop webhook to avoid deletion prevention.
	// Webhook will be re-enabled by test setup, if the next test needs it.
	nomostest.StopWebhook(nt)

	// For the purposes of these finalizer tests, we assume the finalizer may
	// not work correctly. So we delete the deletion propagation annotation,
	// the syncs, and all the managed objects.
	for _, syncObj := range syncObjs {
		if err := deleteSyncWithOrphanPolicy(nt, syncObj); err != nil {
			nt.T.Error(err)
		}
	}
	for _, obj := range objs {
		if err := deleteObject(nt, obj); err != nil {
			nt.T.Error(err)
		}
	}

	tg := taskgroup.New()
	for _, syncObj := range syncObjs {
		o := syncObj
		tg.Go(func() error {
			return watchForNotFoundFromObject(nt, o)
		})
	}
	for _, obj := range objs {
		o := obj
		tg.Go(func() error {
			return watchForNotFoundFromObject(nt, o)
		})
	}
	if err := tg.Wait(); err != nil {
		nt.T.Error(err)
	}
}

// watchForNotFoundFromObject wraps WatchForNotFound, but allows the object
// to be fully populated, constructing a new empty typed object as needed.
func watchForNotFoundFromObject(nt *nomostest.NT, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	scheme := nt.KubeClient.Client.Scheme()
	gvk, err := kinds.Lookup(obj, scheme)
	if err != nil {
		return err
	}
	return nt.Watcher.WatchForNotFound(gvk, key.Name, key.Namespace)
}

func newEmptyTypedObject(nt *nomostest.NT, obj client.Object) (client.Object, schema.GroupVersionKind, error) {
	scheme := nt.KubeClient.Client.Scheme()
	gvk, err := kinds.Lookup(obj, scheme)
	if err != nil {
		return nil, gvk, err
	}
	cObj, err := kinds.NewClientObjectForGVK(gvk, scheme)
	if err != nil {
		return nil, gvk, err
	}
	return cObj, gvk, nil
}

func deleteSyncWithOrphanPolicy(nt *nomostest.NT, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	obj, gvk, err := newEmptyTypedObject(nt, obj)
	if err != nil {
		return err
	}

	err = nt.KubeClient.Get(key.Name, key.Namespace, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	nt.T.Log("Removing deletion propagation annotation")
	if metadata.RemoveDeletionPropagationPolicy(obj) {
		err = nt.KubeClient.Update(obj)
		if err != nil {
			return err
		}
	}

	nt.T.Logf("Deleting %s %s", gvk.Kind, key)
	err = nt.KubeClient.Delete(obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func deleteObject(nt *nomostest.NT, obj client.Object) error {
	gvk, err := kinds.Lookup(obj, nt.KubeClient.Client.Scheme())
	if err != nil {
		return err
	}

	nt.T.Logf("Deleting %s %s", gvk.Kind, client.ObjectKeyFromObject(obj))
	err = nt.KubeClient.Delete(obj, client.PropagationPolicy(metav1.DeletePropagationForeground))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// TestReconcilerFinalizer_ResourceGroupMetricsReset tests that when a ResourceGroup
// is deleted, all resource-related metrics are reset to 0.
func TestReconcilerFinalizer_ResourceGroupMetricsReset(t *testing.T) {
	nt := nomostest.New(t, nomostesting.MultiRepos,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured))
	rootSyncID := nomostest.DefaultRootSyncID
	rootSyncKey := rootSyncID.ObjectKey
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)

	deployment1NN := types.NamespacedName{Name: "helloworld-1", Namespace: testNs}
	namespace1NN := types.NamespacedName{Name: testNs}
	safetyNamespace1NN := types.NamespacedName{Name: rootSyncGitRepo.SafetyNSName}

	nt.T.Cleanup(func() {
		cleanupSingleLevel(nt,
			rootSyncKey,
			deployment1NN,
			namespace1NN, safetyNamespace1NN)
	})

	// Add namespace to RootSync
	namespace1 := k8sobjects.NamespaceObject(namespace1NN.Name)
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(namespace1NN.Name, namespace1NN.Name), namespace1))

	// Add deployment-helloworld-1 to RootSync
	deployment1Path := nomostest.StructuredNSPath(deployment1NN.Namespace, "deployment-helloworld-1")
	deployment1 := loadDeployment(nt, "../testdata/deployment-helloworld.yaml")
	deployment1.SetName(deployment1NN.Name)
	deployment1.SetNamespace(deployment1NN.Namespace)
	nt.Must(rootSyncGitRepo.Add(deployment1Path, deployment1))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding deployment helloworld-1 to RootSync"))
	nt.Must(nt.WatchForAllSyncs())
	nt.Must(nt.Watcher.WatchForCurrentStatus(kinds.Deployment(), deployment1.Name, deployment1.Namespace))

	// Wait for ResourceGroup to be reconciled and metrics to be recorded
	nt.Must(nt.Watcher.WatchObject(kinds.ResourceGroup(), rootSyncKey.Name, rootSyncKey.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus),
		)))

	nt.T.Log("Verifying resourcegroup metrics are non-zero before deletion")
	nt.Must(validateResourceGroupMetricsNonZero(nt, rootSyncKey))

	// Tail reconciler logs and print if there's an error.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go nomostest.TailReconcilerLogs(ctx, nt, nomostest.RootReconcilerObjectKey(rootSyncKey.Name))

	// Delete the RootSync
	nt.T.Log("Deleting RootSync")
	rootSync := k8sobjects.RootSyncObjectV1Beta1(rootSyncKey.Name)
	err := nt.KubeClient.Delete(rootSync)
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.Must(nt.Watcher.WatchForNotFound(kinds.ResourceGroup(), rootSyncKey.Name, rootSyncKey.Namespace))

	// Verify resourcegroup metrics are reset to 0
	nt.T.Log("Verifying resourcegroup metrics are reset to 0 after deletion")
	nt.Must(validateResourceGroupMetricsReset(nt, rootSyncKey))
}

// validateResourceGroupMetricsNonZero verifies that resourcegroup metrics are non-zero
func validateResourceGroupMetricsNonZero(nt *nomostest.NT, rgNN types.NamespacedName) error {
	return nomostest.ValidateMetrics(nt,
		resourceGroupMetricHasValueAtLeast(nt, rgmetrics.ResourceCountName, rgNN, 1),
		resourceGroupMetricHasValueAtLeast(nt, rgmetrics.ReadyResourceCountName, rgNN, 1),
		resourceGroupMetricHasValueAtLeast(nt, rgmetrics.NamespaceCountName, rgNN, 1),
	)
}

// validateResourceGroupMetricsReset verifies that all resourcegroup metrics are reset to 0
func validateResourceGroupMetricsReset(nt *nomostest.NT, rgNN types.NamespacedName) error {
	return nomostest.ValidateMetrics(nt,
		resourceGroupMetricHasValue(nt, rgmetrics.ResourceCountName, rgNN, 0),
		resourceGroupMetricHasValue(nt, rgmetrics.ReadyResourceCountName, rgNN, 0),
		resourceGroupMetricHasValue(nt, rgmetrics.NamespaceCountName, rgNN, 0),
		resourceGroupMetricHasValue(nt, rgmetrics.ClusterScopedResourceCountName, rgNN, 0),
		resourceGroupMetricHasValue(nt, rgmetrics.CRDCountName, rgNN, 0),
		resourceGroupMetricHasValue(nt, rgmetrics.KCCResourceCountName, rgNN, 0),
		resourceGroupMetricHasValue(nt, rgmetrics.PipelineErrorName, rgNN, 0),
	)
}

const (
	prometheusConfigSyncMetricPrefix = "config_sync_"
)

// resourceGroupMetricHasValue returns a MetricsPredicate that validates a resourcegroup metric has the expected value.
// If the expected value is zero, the metric must be zero or not found.
func resourceGroupMetricHasValue(nt *nomostest.NT, metricName string, rgNN types.NamespacedName, value int64) nomostest.MetricsPredicate {
	return func(ctx context.Context, v1api prometheusv1.API) error {
		fullMetricName := fmt.Sprintf("%s%s", prometheusConfigSyncMetricPrefix, metricName)
		labels := prometheusmodel.LabelSet{
			prometheusmodel.LabelName("resourcegroup"): prometheusmodel.LabelValue(rgNN.String()),
		}
		query := fmt.Sprintf("%s%s", fullMetricName, labels)
		return validateResourceGroupMetricValue(ctx, nt, v1api, query, float64(value), value == 0)
	}
}

// resourceGroupMetricHasValueAtLeast returns a MetricsPredicate that validates a resourcegroup metric has at least the expected value.
func resourceGroupMetricHasValueAtLeast(nt *nomostest.NT, metricName string, rgNN types.NamespacedName, value int64) nomostest.MetricsPredicate {
	return func(ctx context.Context, v1api prometheusv1.API) error {
		fullMetricName := fmt.Sprintf("%s%s", prometheusConfigSyncMetricPrefix, metricName)
		labels := prometheusmodel.LabelSet{
			prometheusmodel.LabelName("resourcegroup"): prometheusmodel.LabelValue(rgNN.String()),
		}
		query := fmt.Sprintf("%s%s", fullMetricName, labels)
		return validateResourceGroupMetricValueAtLeast(ctx, nt, v1api, query, float64(value))
	}
}

// validateResourceGroupMetricValue validates that a metric has the expected value.
// If allowMissing is true and the metric doesn't exist, it's considered valid (for zero values).
func validateResourceGroupMetricValue(ctx context.Context, nt *nomostest.NT, v1api prometheusv1.API, query string, value float64, allowMissing bool) error {
	response, err := metricQueryNow(ctx, nt, v1api, query)
	if err != nil {
		if allowMissing {
			return nil // Missing metric is acceptable when expecting zero
		}
		return err
	}

	switch result := response.(type) {
	case prometheusmodel.Vector:
		if len(result) == 0 {
			if allowMissing {
				return nil // No results is acceptable when expecting zero
			}
			return fmt.Errorf("no results from prometheus query: %s", query)
		}
		nt.Logger.Debugf("prometheus vector response:\n%s", result)
		for _, sample := range result {
			if sample.Value.Equal(prometheusmodel.SampleValue(value)) {
				return nil
			}
		}
		var values []prometheusmodel.SampleValue
		for _, sample := range result {
			values = append(values, sample.Value)
		}
		return fmt.Errorf("value %v not found in vector response %v for query: %s", value, values, query)
	case prometheusmodel.Matrix:
		if len(result) == 0 {
			if allowMissing {
				return nil // No results is acceptable when expecting zero
			}
			return fmt.Errorf("no results from prometheus query: %s", query)
		}
		nt.Logger.Debugf("prometheus matrix response:\n%s", result)
		for _, samples := range result {
			for _, sample := range samples.Values {
				if sample.Value.Equal(prometheusmodel.SampleValue(value)) {
					return nil
				}
			}
		}
		var values []prometheusmodel.SampleValue
		for _, samples := range result {
			for _, sample := range samples.Values {
				values = append(values, sample.Value)
			}
		}
		return fmt.Errorf("value %v not found in matrix response %v for query: %s", value, values, query)
	default:
		return fmt.Errorf("unsupported prometheus response: %T", response)
	}
}

// validateResourceGroupMetricValueAtLeast validates that a metric has at least the expected value.
func validateResourceGroupMetricValueAtLeast(ctx context.Context, nt *nomostest.NT, v1api prometheusv1.API, query string, value float64) error {
	response, err := metricQueryNow(ctx, nt, v1api, query)
	if err != nil {
		return err
	}

	switch result := response.(type) {
	case prometheusmodel.Vector:
		if len(result) == 0 {
			return fmt.Errorf("no results from prometheus query: %s", query)
		}
		nt.Logger.Debugf("prometheus vector response:\n%s", result)
		for _, sample := range result {
			if sample.Value >= prometheusmodel.SampleValue(value) {
				return nil
			}
		}
		var values []prometheusmodel.SampleValue
		for _, sample := range result {
			values = append(values, sample.Value)
		}
		return fmt.Errorf("value %v not found in vector response %v for query: %s", value, values, query)
	case prometheusmodel.Matrix:
		if len(result) == 0 {
			return fmt.Errorf("no results from prometheus query: %s", query)
		}
		nt.Logger.Debugf("prometheus matrix response:\n%s", result)
		for _, samples := range result {
			for _, sample := range samples.Values {
				if sample.Value >= prometheusmodel.SampleValue(value) {
					return nil
				}
			}
		}
		var values []prometheusmodel.SampleValue
		for _, samples := range result {
			for _, sample := range samples.Values {
				values = append(values, sample.Value)
			}
		}
		return fmt.Errorf("value %v not found in matrix response %v for query: %s", value, values, query)
	default:
		return fmt.Errorf("unsupported prometheus response: %T", response)
	}
}

// metricQueryNow performs the specified query with the default timeout.
func metricQueryNow(ctx context.Context, nt *nomostest.NT, v1api prometheusv1.API, query string) (prometheusmodel.Value, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	nt.Logger.Debugf("prometheus query: %s", query)
	response, warnings, err := v1api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		nt.T.Logf("prometheus warnings: %v", warnings)
	}

	return response, nil
}

func loadDeployment(nt *nomostest.NT, path string) *appsv1.Deployment {
	specBytes, err := os.ReadFile(path)
	if err != nil {
		nt.T.Fatalf("failed to read test file: %v", err)
	}
	obj := &appsv1.Deployment{}
	err = yaml.Unmarshal(specBytes, obj)
	if err != nil {
		nt.T.Fatalf("failed to parse test file: %v", err)
	}
	return obj
}

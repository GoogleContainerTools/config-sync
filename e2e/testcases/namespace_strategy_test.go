// Copyright 2023 Google LLC
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
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/taskgroup"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/applier"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/declared"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestNamespaceStrategy focuses on the behavior of switching between modes
// of namespaceStrategy for a single RootSync.
// - Set namespaceStrategy: explicit
// - Declare resource in "foo-implicit" Namespace but not the Namespace itself (should error)
// - Set namespaceStrategy: implicit - should create the Namespace implicitly
// - Set namespaceStrategy: explicit - Namespace still exists but is unmanaged
// - Declare namespace "foo-implicit" in git. The Namespace should be managed explicitly
// - Prune "foo-implicit" from git. Should error because cm1 depends on the Namespace
// - Prune "cm1" from git. The Namespace and ConfigMap should be successfully pruned.
func TestNamespaceStrategy(t *testing.T) {
	nt := nomostest.New(t, nomostesting.OverrideAPI,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	rootReconcilerNN := core.RootReconcilerObjectKey(rootSyncNN.Name)
	rootSync := k8sobjects.RootSyncObjectV1Alpha1(rootSyncNN.Name)
	// set the NamespaceStrategy to explicit
	nt.MustMergePatch(rootSync, `{"spec": {"override": {"namespaceStrategy": "explicit"}}}`)
	nt.Must(nt.Watcher.WatchObject(
		kinds.Deployment(), rootReconcilerNN.Name, rootReconcilerNN.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentHasEnvVar(
				reconcilermanager.Reconciler,
				reconcilermanager.NamespaceStrategy,
				string(configsync.NamespaceStrategyExplicit),
			),
		),
	))
	// add a resource for which the namespace is not declared/created
	fooNamespace := k8sobjects.NamespaceObject("foo-implicit")
	cm1 := k8sobjects.ConfigMapObject(core.Name("cm1"), core.Namespace(fooNamespace.Name))
	nt.Must(rootSyncGitRepo.Add("acme/cm1.yaml", cm1))
	nt.Must(rootSyncGitRepo.CommitAndPush("Add cm1"))

	// check for error
	nt.Must(nt.Watcher.WatchForRootSyncSyncError(rootSyncNN.Name, applier.ApplierErrorCode,
		"failed to apply ConfigMap, foo-implicit/cm1: namespaces \"foo-implicit\" not found", []v1beta1.ResourceRef{{
			SourcePath: "acme/cm1.yaml",
			Name:       "cm1",
			Namespace:  "foo-implicit",
			GVK: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
		}}))

	// switch the mode to implicit
	nt.MustMergePatch(rootSync, `{"spec": {"override": {"namespaceStrategy": "implicit"}}}`)
	// check for success
	nt.Must(nt.WatchForAllSyncs())
	// assert that implicit namespace was created
	nt.Must(nt.Validate(fooNamespace.Name, fooNamespace.Namespace, &corev1.Namespace{},
		testpredicates.HasAnnotation(common.LifecycleDeleteAnnotation, common.PreventDeletion),
		testpredicates.IsManagementEnabled(),
		testpredicates.HasAnnotation(metadata.ResourceManagerKey, string(declared.RootScope)),
	))
	// switch mode back to explicit
	nt.MustMergePatch(rootSync, `{"spec": {"override": {"namespaceStrategy": "explicit"}}}`)
	// assert that namespace is no longer managed
	nt.Must(nt.Watcher.WatchObject(kinds.Namespace(), fooNamespace.Name, fooNamespace.Namespace,
		testwatcher.WatchPredicates(
			// still has PreventDeletion
			testpredicates.HasAnnotation(common.LifecycleDeleteAnnotation, common.PreventDeletion),
			// management annotations should be removed
			testpredicates.MissingAnnotation(metadata.ManagementModeAnnotationKey),
			testpredicates.MissingAnnotation(metadata.ResourceManagerKey),
		),
	))
	// explicitly declare the namespace in git
	nt.Must(rootSyncGitRepo.Add("acme/namespace-foo.yaml", fooNamespace))
	nt.Must(rootSyncGitRepo.CommitAndPush("Explicitly manage fooNamespace"))
	nt.Must(nt.WatchForAllSyncs())
	// assert that namespace is managed
	nt.Must(nt.Watcher.WatchObject(kinds.Namespace(), fooNamespace.Name, fooNamespace.Namespace,
		testwatcher.WatchPredicates(
			// Config Sync uses a single field manager, so the PreventDeletion
			// annotation is removed.
			// Users can still declare the annotation in the explicit namespace.
			testpredicates.MissingAnnotation(common.LifecycleDeleteAnnotation),
			testpredicates.IsManagementEnabled(),
			testpredicates.HasAnnotation(metadata.ResourceManagerKey, string(declared.RootScope)),
		),
	))
	// prune the namespace
	nt.Must(rootSyncGitRepo.Remove("acme/namespace-foo.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Prune namespace-foo"))
	// check for error
	nt.Must(nt.Watcher.WatchForRootSyncSyncError(rootSyncNN.Name, applier.ApplierErrorCode,
		"skipped delete of Namespace, /foo-implicit: namespace still in use: foo-implicit", nil))
	// prune the ConfigMap
	nt.Must(rootSyncGitRepo.Remove("acme/cm1.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Prune cm1"))
	nt.Must(nt.WatchForAllSyncs())
	// all resources should be pruned
	tg := taskgroup.New()
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.Namespace(), fooNamespace.Name, fooNamespace.Namespace)
	})
	tg.Go(func() error {
		return nt.Watcher.WatchForNotFound(kinds.ConfigMap(), cm1.Name, cm1.Namespace)
	})
	if err := tg.Wait(); err != nil {
		nt.T.Fatal(err)
	}
}

// TestNamespaceStrategyMultipleRootSyncs focuses on the behavior of using
// namespaceStrategy: explicit with multiple RootSyncs.
// When using multiple RootSyncs that declare resources in the same namespace,
// the namespace should only be created if declared explicitly in a sync source.
func TestNamespaceStrategyMultipleRootSyncs(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	rootSyncAID := core.RootSyncID("sync-a")
	rootSyncXID := core.RootSyncID("sync-x")
	rootSyncYID := core.RootSyncID("sync-y")
	namespaceA := k8sobjects.NamespaceObject("namespace-a")
	nt := nomostest.New(t, nomostesting.OverrideAPI,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(rootSyncAID, ntopts.Unstructured), // will declare namespace-a explicitly
		ntopts.SyncWithGitSource(rootSyncXID, ntopts.Unstructured), // will declare resources in namespace-a, but not namespace-a itself
		ntopts.SyncWithGitSource(rootSyncYID, ntopts.Unstructured), // will declare resources in namespace-a, but not namespace-a itself
	)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)
	rootSyncAGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncAID)
	rootSyncXGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncXID)
	rootSyncYGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncYID)

	rootSyncA := nomostest.RootSyncObjectV1Beta1FromRootRepo(nt, rootSyncAID.Name)
	rootSyncX := nomostest.RootSyncObjectV1Beta1FromRootRepo(nt, rootSyncXID.Name)
	rootSyncY := nomostest.RootSyncObjectV1Beta1FromRootRepo(nt, rootSyncYID.Name)
	rootSyncA.Spec.SafeOverride().NamespaceStrategy = configsync.NamespaceStrategyExplicit
	rootSyncX.Spec.SafeOverride().NamespaceStrategy = configsync.NamespaceStrategyExplicit
	rootSyncY.Spec.SafeOverride().NamespaceStrategy = configsync.NamespaceStrategyExplicit

	// set the NamespaceStrategy to explicit
	nt.Must(rootSyncGitRepo.Add(
		fmt.Sprintf("acme/namespaces/%s/%s.yaml", configsync.ControllerNamespace, rootSyncA.Name),
		rootSyncA,
	))
	nt.Must(rootSyncGitRepo.Add(
		fmt.Sprintf("acme/namespaces/%s/%s.yaml", configsync.ControllerNamespace, rootSyncX.Name),
		rootSyncX,
	))
	nt.Must(rootSyncGitRepo.Add(
		fmt.Sprintf("acme/namespaces/%s/%s.yaml", configsync.ControllerNamespace, rootSyncY.Name),
		rootSyncY,
	))
	nt.Must(rootSyncGitRepo.CommitAndPush(
		fmt.Sprintf("Adding RootSyncs (%s, %s, %s) with namespaceStrategy=explicit",
			rootSyncA.Name, rootSyncX.Name, rootSyncY.Name),
	))
	nt.Must(nt.WatchForAllSyncs())
	// Assert that all reconcilers have NAMESPACE_STRATEGY=explicit set
	tg := taskgroup.New()
	for _, rsName := range []string{rootSyncA.Name, rootSyncX.Name, rootSyncY.Name} {
		reconcilerNN := core.RootReconcilerObjectKey(rsName)
		tg.Go(func() error {
			return nt.Watcher.WatchObject(
				kinds.Deployment(), reconcilerNN.Name, reconcilerNN.Namespace,
				testwatcher.WatchPredicates(
					testpredicates.DeploymentHasEnvVar(
						reconcilermanager.Reconciler,
						reconcilermanager.NamespaceStrategy,
						string(configsync.NamespaceStrategyExplicit),
					),
				),
			)
		})
	}
	if err := tg.Wait(); err != nil {
		nt.T.Fatal(err)
	}
	// add a resource for which the namespace is not declared/created
	cmX := k8sobjects.ConfigMapObject(core.Name("cm-x"), core.Namespace(namespaceA.Name))
	nt.Must(rootSyncXGitRepo.Add("acme/cm-x.yaml", cmX))
	nt.Must(rootSyncXGitRepo.CommitAndPush("Add cm-x"))
	cmY := k8sobjects.ConfigMapObject(core.Name("cm-y"), core.Namespace(namespaceA.Name))
	nt.Must(rootSyncYGitRepo.Add("acme/cm-y.yaml", cmY))
	nt.Must(rootSyncYGitRepo.CommitAndPush("Add cm-y"))
	// check for error
	nt.Must(nt.Watcher.WatchForRootSyncSyncError(rootSyncX.Name, applier.ApplierErrorCode,
		"failed to apply ConfigMap, namespace-a/cm-x: namespaces \"namespace-a\" not found", nil))
	nt.Must(nt.Watcher.WatchForRootSyncSyncError(rootSyncY.Name, applier.ApplierErrorCode,
		"failed to apply ConfigMap, namespace-a/cm-y: namespaces \"namespace-a\" not found", nil))
	// declare the namespace in sync-a
	nt.Must(rootSyncAGitRepo.Add("acme/namespace-a.yaml", namespaceA))
	nt.Must(rootSyncAGitRepo.CommitAndPush("Add namespace-a"))
	// check for success
	nt.Must(nt.WatchForAllSyncs())
	// assert that all resources were created
	tg = taskgroup.New()
	tg.Go(func() error {
		return nt.Watcher.WatchObject(kinds.Namespace(), namespaceA.Name, namespaceA.Namespace,
			testwatcher.WatchPredicates(
				// Users can add PreventDeletion annotation to the declared namespace
				// if they choose, but the reconciler does not add it by default.
				testpredicates.MissingAnnotation(common.LifecycleDeleteAnnotation),
				testpredicates.IsManagementEnabled(),
				testpredicates.HasAnnotation(metadata.ResourceManagerKey, declared.ResourceManager(declared.RootScope, rootSyncA.Name)),
			))
	})
	tg.Go(func() error {
		return nt.Watcher.WatchObject(kinds.ConfigMap(), cmX.Name, cmX.Namespace,
			testwatcher.WatchPredicates(
				testpredicates.IsManagementEnabled(),
				testpredicates.HasAnnotation(metadata.ResourceManagerKey, declared.ResourceManager(declared.RootScope, rootSyncX.Name)),
			))
	})
	tg.Go(func() error {
		return nt.Watcher.WatchObject(kinds.ConfigMap(), cmY.Name, cmY.Namespace,
			testwatcher.WatchPredicates(
				testpredicates.IsManagementEnabled(),
				testpredicates.HasAnnotation(metadata.ResourceManagerKey, declared.ResourceManager(declared.RootScope, rootSyncY.Name)),
			))
	})
	nt.Must(tg.Wait())
}

// TestImplicitNamespaceHandoffBetweenRootSyncs covers a RootSync that creates
// implicit Namespaces which another RootSync later declares explicitly.
//   - sync-x declares ConfigMaps in namespace-a and namespace-b, but not the
//     Namespaces, so it creates them implicitly.
//   - sync-a declares namespace-a and namespace-b (without the deletion: detach
//     annotation), takes them over, and adds a ConfigMap in namespace-a.
//   - sync-x re-parses. It no longer declares any Namespace, but still has
//     objects in them, so it must not report KNV2006 and must keep syncing.
//   - sync-x removes its ConfigMaps. It must not delete the Namespaces that
//     sync-a now owns, or anything sync-a has in them.
func TestImplicitNamespaceHandoffBetweenRootSyncs(t *testing.T) {
	rootSyncID := nomostest.DefaultRootSyncID
	rootSyncAID := core.RootSyncID("sync-a")
	rootSyncXID := core.RootSyncID("sync-x")
	nt := nomostest.New(t, nomostesting.MultiRepos,
		ntopts.SyncWithGitSource(rootSyncID, ntopts.Unstructured),
		ntopts.SyncWithGitSource(rootSyncAID, ntopts.Unstructured), // will declare the Namespaces
		ntopts.SyncWithGitSource(rootSyncXID, ntopts.Unstructured), // will only declare objects in the Namespaces
	)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncID)
	rootSyncAGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncAID)
	rootSyncXGitRepo := nt.SyncSourceGitReadWriteRepository(rootSyncXID)

	rootSyncA := nomostest.RootSyncObjectV1Beta1FromRootRepo(nt, rootSyncAID.Name)
	rootSyncX := nomostest.RootSyncObjectV1Beta1FromRootRepo(nt, rootSyncXID.Name)
	nt.Must(rootSyncGitRepo.Add(
		fmt.Sprintf("acme/namespaces/%s/%s.yaml", configsync.ControllerNamespace, rootSyncA.Name),
		rootSyncA,
	))
	nt.Must(rootSyncGitRepo.Add(
		fmt.Sprintf("acme/namespaces/%s/%s.yaml", configsync.ControllerNamespace, rootSyncX.Name),
		rootSyncX,
	))
	nt.Must(rootSyncGitRepo.CommitAndPush(
		fmt.Sprintf("Adding RootSyncs (%s, %s) with the default namespaceStrategy", rootSyncA.Name, rootSyncX.Name)))
	nt.Must(nt.WatchForAllSyncs())

	// sync-x creates both Namespaces implicitly.
	cmA := k8sobjects.ConfigMapObject(core.Name("cm-x"), core.Namespace("namespace-a"))
	cmB := k8sobjects.ConfigMapObject(core.Name("cm-x"), core.Namespace("namespace-b"))
	nt.Must(rootSyncXGitRepo.Add("acme/cm-a.yaml", cmA))
	nt.Must(rootSyncXGitRepo.Add("acme/cm-b.yaml", cmB))
	nt.Must(rootSyncXGitRepo.CommitAndPush("Add ConfigMaps in namespace-a and namespace-b"))
	nt.Must(nt.WatchForAllSyncs())

	// sync-a declares both Namespaces and takes them over. Without the
	// deletion: detach annotation, sync-x has nothing stopping it from pruning
	// the Namespaces other than releasing them from its inventory.
	namespaces := []client.Object{
		k8sobjects.NamespaceObject("namespace-a"),
		k8sobjects.NamespaceObject("namespace-b"),
	}
	for _, ns := range namespaces {
		nt.Must(rootSyncAGitRepo.Add(fmt.Sprintf("acme/%s.yaml", ns.GetName()), ns))
	}
	cmSyncA := k8sobjects.ConfigMapObject(core.Name("cm-a"), core.Namespace("namespace-a"))
	nt.Must(rootSyncAGitRepo.Add("acme/cm-sync-a.yaml", cmSyncA))
	nt.Must(rootSyncAGitRepo.CommitAndPush("Declare namespace-a and namespace-b"))
	nt.Must(nt.WatchForAllSyncs())

	// Push a new commit to sync-x so it re-parses after the handoff. Before
	// the fix, this failed with KNV2006 until the reconciler restarted.
	cmA.SetLabels(map[string]string{"handoff": "true"})
	nt.Must(rootSyncXGitRepo.Add("acme/cm-a.yaml", cmA))
	nt.Must(rootSyncXGitRepo.CommitAndPush("Update cm-x in namespace-a"))
	nt.Must(nt.WatchForAllSyncs())

	tg := taskgroup.New()
	for _, ns := range namespaces {
		tg.Go(func() error {
			return nt.Watcher.WatchObject(kinds.Namespace(), ns.GetName(), "",
				testwatcher.WatchPredicates(
					testpredicates.HasAnnotation(metadata.ResourceManagerKey, declared.ResourceManager(declared.RootScope, rootSyncA.Name)),
				))
		})
	}
	tg.Go(func() error {
		return nt.Watcher.WatchObject(kinds.ConfigMap(), cmA.Name, cmA.Namespace,
			testwatcher.WatchPredicates(
				testpredicates.HasLabel("handoff", "true"),
				testpredicates.HasAnnotation(metadata.ResourceManagerKey, declared.ResourceManager(declared.RootScope, rootSyncX.Name)),
			))
	})
	tg.Go(func() error {
		return nt.Watcher.WatchObject(kinds.ConfigMap(), cmB.Name, cmB.Namespace,
			testwatcher.WatchPredicates(
				testpredicates.HasAnnotation(metadata.ResourceManagerKey, declared.ResourceManager(declared.RootScope, rootSyncX.Name)),
			))
	})
	tg.Go(func() error {
		return nt.Watcher.WatchObject(kinds.ResourceGroup(), rootSyncX.Name, configsync.ControllerNamespace,
			testwatcher.WatchPredicates(
				testpredicates.ResourceGroupMissingObjects(namespaces),
			))
	})
	nt.Must(tg.Wait())

	// sync-x removes its ConfigMaps. The Namespaces and sync-a's ConfigMap
	// must survive.
	nt.Must(rootSyncXGitRepo.Remove("acme/cm-a.yaml"))
	nt.Must(rootSyncXGitRepo.Remove("acme/cm-b.yaml"))
	nt.Must(rootSyncXGitRepo.CommitAndPush("Remove ConfigMaps from namespace-a and namespace-b"))
	nt.Must(nt.WatchForAllSyncs())
	nt.Must(nt.Watcher.WatchForNotFound(kinds.ConfigMap(), cmA.Name, cmA.Namespace))
	nt.Must(nt.Watcher.WatchForNotFound(kinds.ConfigMap(), cmB.Name, cmB.Namespace))
	for _, ns := range namespaces {
		nt.Must(nt.Validate(ns.GetName(), "", &corev1.Namespace{}))
	}
	nt.Must(nt.Validate(cmSyncA.Name, cmSyncA.Namespace, &corev1.ConfigMap{}))
}

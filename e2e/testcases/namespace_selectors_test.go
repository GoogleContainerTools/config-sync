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
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	v1 "github.com/GoogleContainerTools/config-sync/pkg/api/configmanagement/v1"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	k8sobjects2 "github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/transform/selectors"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	bookstoreNS      = "bookstore"
	shoestoreNS      = "shoestore"
	bookstoreNSSName = "bookstore-nss"
	shoestoreNSSName = "shoestore-nss"
	bookstoreCMName  = "cm-bookstore"
	bookstoreRQName  = "rq-bookstore"
	shoestoreCMName  = "cm-shoestore"
)

var (
	selectedResourcesWithBookstoreNSSAndShoestoreNSS = []client.Object{
		k8sobjects2.ConfigMapObject(core.Namespace(bookstoreNS), core.Name(bookstoreCMName)),
		k8sobjects2.ResourceQuotaObject(core.Namespace(bookstoreNS), core.Name(bookstoreRQName)),
		k8sobjects2.ConfigMapObject(core.Namespace(shoestoreNS), core.Name(shoestoreCMName)),
	}

	unselectedResourcesWithBookstoreNSSAndShoestoreNSS = []client.Object{
		k8sobjects2.ConfigMapObject(core.Namespace(bookstoreNS), core.Name(shoestoreCMName)),
		k8sobjects2.ConfigMapObject(core.Namespace(shoestoreNS), core.Name(bookstoreCMName)),
		k8sobjects2.ResourceQuotaObject(core.Namespace(shoestoreNS), core.Name(bookstoreRQName)),
	}

	selectedResourcesWithShoestoreNSSOnly = []client.Object{
		k8sobjects2.ConfigMapObject(core.Namespace(shoestoreNS), core.Name(shoestoreCMName)),
	}

	unselectedResourcesWithShoestoreNSSOnly = []client.Object{
		k8sobjects2.ConfigMapObject(core.Namespace(bookstoreNS), core.Name(bookstoreCMName)),
		k8sobjects2.ResourceQuotaObject(core.Namespace(bookstoreNS), core.Name(bookstoreRQName)),
		k8sobjects2.ConfigMapObject(core.Namespace(bookstoreNS), core.Name(shoestoreCMName)),
		k8sobjects2.ConfigMapObject(core.Namespace(shoestoreNS), core.Name(bookstoreCMName)),
		k8sobjects2.ResourceQuotaObject(core.Namespace(shoestoreNS), core.Name(bookstoreRQName)),
	}
)

func TestNamespaceSelectorHierarchicalFormat(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Selector)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	bookstoreNSS := k8sobjects2.NamespaceSelectorObject(core.Name(bookstoreNSSName))
	bookstoreCM := k8sobjects2.ConfigMapObject(core.Name(bookstoreCMName),
		core.Annotation(metadata.NamespaceSelectorAnnotationKey, bookstoreNSSName))
	bookstoreRQ := k8sobjects2.ResourceQuotaObject(core.Name(bookstoreRQName),
		core.Annotation(metadata.NamespaceSelectorAnnotationKey, bookstoreNSSName))

	shoestoreNSS := k8sobjects2.NamespaceSelectorObject(core.Name(shoestoreNSSName))
	shoestoreCM := k8sobjects2.ConfigMapObject(core.Name(shoestoreCMName),
		core.Annotation(metadata.NamespaceSelectorAnnotationKey, shoestoreNSSName))

	nt.T.Log("Add Namespaces, NamespaceSelectors and Namespace-scoped resources")
	bookstoreNSS.Spec.Selector.MatchLabels = map[string]string{"app": bookstoreNS}
	bookstoreNSS.Spec.Mode = v1.NSSelectorDynamicMode
	bookstoreRQ.Spec.Hard = map[corev1.ResourceName]resource.Quantity{corev1.ResourcePods: resource.MustParse("1")}

	shoestoreNSS.Spec.Selector.MatchLabels = map[string]string{"app": shoestoreNS}

	nt.Must(rootSyncGitRepo.Add("acme/namespaces/namespace-selector-bookstore.yaml", bookstoreNSS))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/namespace-selector-shoestore.yaml", shoestoreNSS))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/bookstore/ns.yaml", k8sobjects2.NamespaceObject(bookstoreNS, core.Label("app", bookstoreNS))))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/shoestore/ns.yaml", k8sobjects2.NamespaceObject(shoestoreNS, core.Label("app", shoestoreNS))))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/cm-bookstore.yaml", bookstoreCM))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/rq-bookstore.yaml", bookstoreRQ))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/cm-shoestore.yaml", shoestoreCM))
	nt.Must(rootSyncGitRepo.CommitAndPush("Add Namespaces, NamespaceSelectors and Namespace-scoped resources"))

	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, selectors.InvalidSelectorErrorCode, "NamespaceSelector MUST NOT use the dynamic mode with the hierarchy source format"))

	nt.T.Log("Update NamespaceSelector to use static mode with the hierarchy format")
	bookstoreNSS.Spec.Mode = v1.NSSelectorStaticMode
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/namespace-selector-bookstore.yaml", bookstoreNSS))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update NamespaceSelector to use static mode"))

	nt.Must(nt.WatchForAllSyncs())

	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithBookstoreNSSAndShoestoreNSS,
		unselectedResourcesWithBookstoreNSSAndShoestoreNSS,
	)
}

func TestNamespaceSelectorUnstructuredFormat(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Selector,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	bookstoreNSS := k8sobjects2.NamespaceSelectorObject(core.Name(bookstoreNSSName))
	bookstoreCM := k8sobjects2.ConfigMapObject(core.Name(bookstoreCMName),
		core.Annotation(metadata.NamespaceSelectorAnnotationKey, bookstoreNSSName))
	bookstoreRQ := k8sobjects2.ResourceQuotaObject(core.Name(bookstoreRQName),
		core.Annotation(metadata.NamespaceSelectorAnnotationKey, bookstoreNSSName))

	shoestoreNSS := k8sobjects2.NamespaceSelectorObject(core.Name(shoestoreNSSName))
	shoestoreCM := k8sobjects2.ConfigMapObject(core.Name(shoestoreCMName),
		core.Annotation(metadata.NamespaceSelectorAnnotationKey, shoestoreNSSName))

	nt.T.Log("Add Namespaces, NamespaceSelectors and Namespace-scoped resources")
	bookstoreNSS.Spec.Selector.MatchLabels = map[string]string{"app": bookstoreNS}
	bookstoreRQ.Spec.Hard = map[corev1.ResourceName]resource.Quantity{corev1.ResourcePods: resource.MustParse("1")}

	shoestoreNSS.Spec.Selector.MatchLabels = map[string]string{"app": shoestoreNS}

	nt.Must(rootSyncGitRepo.Add("acme/namespace-selector-bookstore.yaml", bookstoreNSS))
	nt.Must(rootSyncGitRepo.Add("acme/namespace-selector-shoestore.yaml", shoestoreNSS))
	nt.Must(rootSyncGitRepo.Add("acme/shoestore-ns.yaml", k8sobjects2.NamespaceObject(shoestoreNS, core.Label("app", shoestoreNS))))
	nt.Must(rootSyncGitRepo.Add("acme/cm-bookstore.yaml", bookstoreCM))
	nt.Must(rootSyncGitRepo.Add("acme/rq-bookstore.yaml", bookstoreRQ))
	nt.Must(rootSyncGitRepo.Add("acme/cm-shoestore.yaml", shoestoreCM))
	nt.Must(rootSyncGitRepo.CommitAndPush("Add Namespaces, NamespaceSelectors and Namespace-scoped resources"))

	nt.Logger.Info("Only resources in shoestore are created because bookstore Namespace is not declared")
	nt.Must(nt.WatchForAllSyncs())

	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithShoestoreNSSOnly,
		unselectedResourcesWithShoestoreNSSOnly,
	)
	if err := nt.Validate(
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		&v1beta1.RootSync{},
		testpredicates.MissingAnnotation(metadata.DynamicNSSelectorEnabledAnnotationKey),
	); err != nil {
		nt.T.Fatal(err)
	}
	if err := nt.Validate(
		core.RootReconcilerName(configsync.RootSyncName),
		configsync.ControllerNamespace,
		&appsv1.Deployment{},
		testpredicates.DeploymentMissingEnvVar(reconcilermanager.Reconciler, reconcilermanager.DynamicNSSelectorEnabled),
	); err != nil {
		nt.T.Fatal(err)
	}

	nt.Logger.Info("Update NamespaceSelector to use dynamic mode")
	bookstoreNSS.Spec.Mode = v1.NSSelectorDynamicMode
	nt.Must(rootSyncGitRepo.Add("acme/namespace-selector-bookstore.yaml", bookstoreNSS))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update NamespaceSelector to use dynamic mode"))
	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(),
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.HasAnnotation(metadata.DynamicNSSelectorEnabledAnnotationKey, "true"),
		)))
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		core.RootReconcilerName(configsync.RootSyncName),
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentHasEnvVar(reconcilermanager.Reconciler, reconcilermanager.DynamicNSSelectorEnabled, "true"),
		)))

	nt.Must(nt.WatchForAllSyncs())
	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithShoestoreNSSOnly,
		unselectedResourcesWithShoestoreNSSOnly,
	)

	nt.Logger.Info("Creating a Namespace with labels, matching resources should be selected")
	bookstoreNamespace := k8sobjects2.NamespaceObject(bookstoreNS, core.Label("app", bookstoreNS))
	if err := nt.KubeClient.Create(bookstoreNamespace); err != nil {
		nt.T.Fatal(err)
	}
	t.Cleanup(func() {
		// When a Namespace is deleted, resources in the Namespace will also be deleted.
		// Those resources are dynamically selected and managed by Config Sync.
		// If the webhook is running, they cannot be deleted due to the admission-webhook.
		nomostest.StopWebhook(nt)
		if err := nt.KubeClient.Delete(bookstoreNamespace); err != nil && !apierrors.IsNotFound(err) {
			nt.T.Fatal(err)
		}
	})

	nt.Logger.Info("Watching the ResourceGroup object until new selected resources are added to the inventory")
	nt.Must(nt.Watcher.WatchObject(kinds.ResourceGroup(),
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.ResourceGroupHasObjects(selectedResourcesWithBookstoreNSSAndShoestoreNSS),
			testpredicates.ResourceGroupMissingObjects(unselectedResourcesWithBookstoreNSSAndShoestoreNSS),
		)))
	nt.Must(nt.WatchForAllSyncs())
	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithBookstoreNSSAndShoestoreNSS,
		unselectedResourcesWithBookstoreNSSAndShoestoreNSS,
	)

	nt.Logger.Info("Update NamespaceSelector to use static mode")
	bookstoreNSS.Spec.Mode = v1.NSSelectorStaticMode
	nt.Must(rootSyncGitRepo.Add("acme/namespace-selector-bookstore.yaml", bookstoreNSS))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update NamespaceSelector to use static mode"))

	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(),
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.HasAnnotation(metadata.DynamicNSSelectorEnabledAnnotationKey, "false"),
		)))
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		core.RootReconcilerName(configsync.RootSyncName),
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentMissingEnvVar(reconcilermanager.Reconciler, reconcilermanager.DynamicNSSelectorEnabled),
		)))

	nt.Logger.Info("Only resources in shoestore are created because bookstore Namespace is not selected")
	nt.Must(nt.WatchForAllSyncs())
	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithShoestoreNSSOnly,
		unselectedResourcesWithShoestoreNSSOnly,
	)

	nt.Logger.Info("Update NamespaceSelector back to use dynamic mode")
	bookstoreNSS.Spec.Mode = v1.NSSelectorDynamicMode
	nt.Must(rootSyncGitRepo.Add("acme/namespace-selector-bookstore.yaml", bookstoreNSS))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update NamespaceSelector to use dynamic mode again"))
	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(),
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.HasAnnotation(metadata.DynamicNSSelectorEnabledAnnotationKey, "true"),
		)))
	nt.Must(nt.Watcher.WatchObject(kinds.Deployment(),
		core.RootReconcilerName(configsync.RootSyncName),
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.DeploymentHasEnvVar(reconcilermanager.Reconciler, reconcilermanager.DynamicNSSelectorEnabled, "true"),
		)))

	nt.Must(nt.WatchForAllSyncs())
	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithBookstoreNSSAndShoestoreNSS,
		unselectedResourcesWithBookstoreNSSAndShoestoreNSS,
	)

	nt.Logger.Info("Update Namespace's label to make it unselected, resources should NOT be selected")
	nt.MustMergePatch(bookstoreNamespace, `{"metadata":{"labels":{"app":"other"}}}`)

	nt.Logger.Info("Watching the ResourceGroup object until unselected resources are removed from the inventory")
	nt.Must(nt.Watcher.WatchObject(kinds.ResourceGroup(),
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.ResourceGroupHasObjects(selectedResourcesWithShoestoreNSSOnly),
			testpredicates.ResourceGroupMissingObjects(unselectedResourcesWithShoestoreNSSOnly),
		)))
	nt.Must(nt.WatchForAllSyncs())
	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithShoestoreNSSOnly,
		unselectedResourcesWithShoestoreNSSOnly,
	)

	nt.Logger.Info("Update Namespace's label to make it selected again, resources should be selected")
	nt.MustMergePatch(bookstoreNamespace, fmt.Sprintf(`{"metadata":{"labels":{"app":"%s"}}}`, bookstoreNS))

	nt.Must(nt.Watcher.WatchObject(kinds.ResourceGroup(),
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.ResourceGroupHasObjects(selectedResourcesWithBookstoreNSSAndShoestoreNSS),
			testpredicates.ResourceGroupMissingObjects(unselectedResourcesWithBookstoreNSSAndShoestoreNSS),
		)))
	nt.Must(nt.WatchForAllSyncs())
	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithBookstoreNSSAndShoestoreNSS,
		unselectedResourcesWithBookstoreNSSAndShoestoreNSS,
	)

	nt.Logger.Info("Stop the admission webhook so the Namespace can be deleted")
	// When a Namespace is deleted, resources in the Namespace will also be deleted.
	// Those resources are dynamically selected and managed by Config Sync.
	// If the webhook is running, they cannot be deleted due to the admission-webhook.
	nomostest.StopWebhook(nt)
	nt.Logger.Info("Delete Namespace, resources should NOT be selected")
	if err := nt.KubeClient.Delete(bookstoreNamespace); err != nil {
		t.Fatal(err)
	}

	nt.Logger.Info("Watching the ResourceGroup object until unselected resources are removed from the inventory")
	if err := nt.Watcher.WatchObject(kinds.ResourceGroup(),
		configsync.RootSyncName,
		configsync.ControllerNamespace,
		testwatcher.WatchPredicates(
			testpredicates.ResourceGroupHasObjects(selectedResourcesWithShoestoreNSSOnly),
			testpredicates.ResourceGroupMissingObjects(unselectedResourcesWithShoestoreNSSOnly),
		)); err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(nt.WatchForAllSyncs())
	validateSelectedAndUnselectedResources(nt,
		selectedResourcesWithShoestoreNSSOnly,
		unselectedResourcesWithShoestoreNSSOnly,
	)
}

func validateSelectedAndUnselectedResources(nt *nomostest.NT, selected []client.Object, unselected []client.Object) {
	for _, o := range selected {
		unst := &unstructured.Unstructured{}
		unst.SetGroupVersionKind(o.GetObjectKind().GroupVersionKind())
		if err := nt.Validate(o.GetName(), o.GetNamespace(), unst); err != nil {
			nt.T.Fatal(err)
		}
	}
	for _, o := range unselected {
		unst := &unstructured.Unstructured{}
		unst.SetGroupVersionKind(o.GetObjectKind().GroupVersionKind())
		if err := nt.ValidateNotFound(o.GetName(), o.GetNamespace(), unst); err != nil {
			nt.T.Fatal(err)
		}
	}
}

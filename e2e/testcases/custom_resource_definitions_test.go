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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/metrics"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/nonhierarchical"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/webhook/configuration"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func mustRemoveCustomResourceWithDefinition(nt *nomostest.NT, crd client.Object) {
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	nt.Must(rootSyncGitRepo.Add("acme/cluster/anvil-crd.yaml", crd))
	nsObj := k8sobjects.NamespaceObject("foo")
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/ns.yaml", nsObj))
	anvilObj := newAnvilObject("v1", "heavy", 10)
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/anvil-v1.yaml", anvilObj))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding Anvil CRD and one Anvil CR"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	err := nt.Validate(configuration.Name, "", &admissionv1.ValidatingWebhookConfiguration{},
		hasRule("acme.com.v1.admission-webhook.configsync.gke.io"))
	if err != nil {
		nt.T.Fatal(err)
	}

	err = nt.Validate("heavy", "foo", newAnvilObject("v1", "", 0))
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, crd)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, nsObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, anvilObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}

	// This should cause an error.
	nt.Must(rootSyncGitRepo.Remove("acme/cluster/anvil-crd.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Removing Anvil CRD but leaving Anvil CR"))

	nt.Must(nt.Watcher.WatchObject(kinds.RootSyncV1Beta1(), rootSyncNN.Name, rootSyncNN.Namespace,
		testwatcher.WatchPredicates(
			testpredicates.RootSyncHasSourceError(nonhierarchical.UnsupportedCRDRemovalErrorCode, ""),
		)))

	rootSyncLabels, err := nomostest.MetricLabelsForRootSync(nt, rootSyncNN)
	if err != nil {
		nt.T.Fatal(err)
	}
	commitHash := rootSyncGitRepo.MustHash(nt.T)

	err = nomostest.ValidateMetrics(nt,
		nomostest.ReconcilerErrorMetrics(nt, rootSyncLabels, commitHash, metrics.ErrorSummary{
			Source: 1,
		}))
	if err != nil {
		nt.T.Fatal(err)
	}

	// This should fix the error.
	nt.Must(rootSyncGitRepo.Remove("acme/namespaces/foo/anvil-v1.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Removing the Anvil CR as well"))
	nt.Must(nt.WatchForAllSyncs())

	nt.MetricsExpectations.AddObjectDelete(configsync.RootSyncKind, rootSyncNN, crd)
	nt.MetricsExpectations.AddObjectDelete(configsync.RootSyncKind, rootSyncNN, anvilObj)

	// Validate reconciler error is cleared.
	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}
}

func TestMustRemoveCustomResourceWithDefinitionV1(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	mustRemoveCustomResourceWithDefinition(nt, anvilV1CRD())
}

func addAndRemoveCustomResource(nt *nomostest.NT, dir string, crd string) {
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	crdFile := filepath.Join(".", "..", "testdata", "customresources", dir, crd)
	crdContent, err := os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/anvil-crd.yaml", crdContent))
	crdObj := rootSyncGitRepo.MustGet(nt.T, "acme/cluster/anvil-crd.yaml")
	nsObj := k8sobjects.NamespaceObject("prod")
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/prod/ns.yaml", nsObj))
	anvilObj := newAnvilObject("v1", "e2e-test-anvil", 10)
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/prod/anvil.yaml", anvilObj))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding Anvil CRD and one Anvil CR"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	err = nt.Validate("e2e-test-anvil", "prod", newAnvilObject("v1", "", 10))
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, crdObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, nsObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, anvilObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}

	// Remove the CustomResource.
	nt.Must(rootSyncGitRepo.Remove("acme/namespaces/prod/anvil.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Removing Anvil CR but leaving Anvil CRD"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.ValidateNotFound("e2e-test-anvil", "prod", newAnvilObject("v1", "", 10))
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.MetricsExpectations.AddObjectDelete(configsync.RootSyncKind, rootSyncNN, anvilObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}

	// Remove the CustomResourceDefinition.
	nt.Must(rootSyncGitRepo.Remove("acme/cluster/anvil-crd.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Removing the Anvil CRD as well"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.ValidateNotFound("anvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object())
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.MetricsExpectations.AddObjectDelete(configsync.RootSyncKind, rootSyncNN, crdObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}
}

func TestAddAndRemoveCustomResourceV1(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	addAndRemoveCustomResource(nt, "v1_crds", "anvil-crd.yaml")
}

func mustRemoveUnManagedCustomResource(nt *nomostest.NT, dir string, crd string) {
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	crdFile := filepath.Join(".", "..", "testdata", "customresources", dir, crd)
	crdContent, err := os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/anvil-crd.yaml", crdContent))
	crdObj := rootSyncGitRepo.MustGet(nt.T, "acme/cluster/anvil-crd.yaml")
	nsObj := k8sobjects.NamespaceObject("prod")
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/prod/ns.yaml", nsObj))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding Anvil CRD"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, crdObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, nsObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}

	err = nt.Validate("anvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object())
	if err != nil {
		nt.T.Fatal(err)
	}

	// Apply the CustomResource.
	cr := newAnvilObject("v1", "e2e-test-anvil", 100)
	cr.SetNamespace("prod")
	err = nt.KubeClient.Create(cr)
	if err != nil {
		nt.T.Fatal(err)
	}

	// Remove the CustomResourceDefinition.
	nt.Must(rootSyncGitRepo.Remove("acme/cluster/anvil-crd.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Removing the Anvil CRD"))
	nt.Must(nt.WatchForAllSyncs())

	err = nt.ValidateNotFound("anvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object())
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.MetricsExpectations.AddObjectDelete(configsync.RootSyncKind, rootSyncNN, crdObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}
}
func TestMustRemoveUnManagedCustomResourceV1(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	mustRemoveUnManagedCustomResource(nt, "v1_crds", "anvil-crd.yaml")
}

func addUpdateRemoveClusterScopedCRD(nt *nomostest.NT, dir string, crd string) {
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	crdFile := filepath.Join(".", "..", "testdata", "customresources", dir, crd)
	crdContent, err := os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/clusteranvil-crd.yaml", crdContent))
	crdObj := rootSyncGitRepo.MustGet(nt.T, "acme/cluster/clusteranvil-crd.yaml")
	clusteranvilObj := clusteranvilCR("v1", "e2e-test-clusteranvil", 10)
	nt.Must(rootSyncGitRepo.Add("acme/cluster/clusteranvil.yaml", clusteranvilObj))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding clusterscoped Anvil CRD and CR"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	err = nt.Validate("e2e-test-clusteranvil", "", clusteranvilCR("v1", "", 10))
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, crdObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, clusteranvilObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}

	// Update the CRD from version v1 to version v2.
	crdFile = filepath.Join(".", "..", "testdata", "customresources", dir, "clusteranvil-crd-v2.yaml")
	crdContent, err = os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/clusteranvil-crd.yaml", crdContent))
	nt.Must(rootSyncGitRepo.CommitAndPush("Updating the Anvil CRD"))
	nt.Must(nt.WatchForAllSyncs())

	err = nt.Validate("clusteranvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object(), hasTwoVersions)
	if err != nil {
		nt.T.Error(err)
	}
	err = nt.Validate("e2e-test-clusteranvil", "", clusteranvilCR("v2", "", 10))
	if err != nil {
		nt.T.Error(err)
	}
	if nt.T.Failed() {
		nt.T.FailNow()
	}

	// Remove the CR and CRD.
	nt.Must(rootSyncGitRepo.Remove("acme/cluster"))
	// Add back the safety ClusterRole to pass the safety check (KNV2006).
	nt.Must(rootSyncGitRepo.AddSafetyClusterRole())
	nt.Must(rootSyncGitRepo.CommitAndPush("Removing the Anvil CRD as well"))
	nt.Must(nt.WatchForAllSyncs())
	err = nt.ValidateNotFoundOrNoMatch("e2e-test-clusteranvil", "prod", clusteranvilCR("v2", "", 10))
	if err != nil {
		nt.T.Error(err)
	}
	err = nt.ValidateNotFound("clusteranvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object())
	if err != nil {
		nt.T.Error(err)
	}
	if nt.T.Failed() {
		nt.T.FailNow()
	}
}
func TestAddUpdateRemoveClusterScopedCRDV1(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	addUpdateRemoveClusterScopedCRD(nt, "v1_crds", "clusteranvil-crd.yaml")
}

func addUpdateNamespaceScopedCRD(nt *nomostest.NT, dir string, crd string) {
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	crdFile := filepath.Join(".", "..", "testdata", "customresources", dir, crd)
	crdContent, err := os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/anvil-crd.yaml", crdContent))
	crdObj := rootSyncGitRepo.MustGet(nt.T, "acme/cluster/anvil-crd.yaml")
	anvilObj := newAnvilObject("v1", "e2e-test-anvil", 10)
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/prod/anvil.yaml", anvilObj))
	nsObj := k8sobjects.NamespaceObject("prod")
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/prod/ns.yaml", nsObj))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding namespacescoped Anvil CRD and CR"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	err = nt.Validate("e2e-test-anvil", "prod", newAnvilObject("v1", "", 10))
	if err != nil {
		nt.T.Fatal(err)
	}

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, crdObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, anvilObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, nsObj)

	err = nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	})
	if err != nil {
		nt.T.Fatal(err)
	}

	// Update the CRD from version v1 to version v2.
	crdFile = filepath.Join(".", "..", "testdata", "customresources", dir, "anvil-crd-v2.yaml")
	crdContent, err = os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/anvil-crd.yaml", crdContent))
	nt.Must(rootSyncGitRepo.CommitAndPush("Updating the Anvil CRD"))
	nt.Must(nt.WatchForAllSyncs())

	err = nt.Validate("e2e-test-anvil", "prod", newAnvilObject("v2", "", 10))
	if err != nil {
		nt.T.Fatal(err)
	}
	err = nt.Validate("anvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object(), hasTwoVersions)
	if err != nil {
		nt.T.Fatal(err)
	}

	// Update CRD and CR to only support V2
	crdFile = filepath.Join(".", "..", "testdata", "customresources", dir, "anvil-crd-only-v2.yaml")
	crdContent, err = os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/anvil-crd.yaml", crdContent))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/prod/anvil.yaml", newAnvilObject("v2", "e2e-test-anvil", 10)))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update the Anvil CRD and CR"))
	nt.Must(nt.WatchForAllSyncs())

	err = nt.Validate("anvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object(), nomostest.IsEstablished, hasTwoVersions)
	if err != nil {
		nt.T.Fatal(err)
	}

	err = nt.Validate("e2e-test-anvil", "prod", newAnvilObject("v2", "", 10))
	if err != nil {
		nt.T.Fatal(err)
	}

	// Remove CRD and CR
	nt.Must(rootSyncGitRepo.Remove("acme/cluster/anvil-crd.yaml"))
	nt.Must(rootSyncGitRepo.Remove("acme/namespaces/prod/anvil.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Remove the Anvil CRD and CR"))
	nt.Must(nt.WatchForAllSyncs())

	// Validate the CustomResource is also deleted from cluster.
	err = nt.ValidateNotFound("anvils.acme.com", "", k8sobjects.CustomResourceDefinitionV1Object())
	if err != nil {
		nt.T.Fatal(err)
	}
}
func TestAddUpdateNamespaceScopedCRDV1(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	addUpdateNamespaceScopedCRD(nt, "v1_crds", "anvil-crd.yaml")
}

func TestLargeCRD(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	for _, file := range []string{"challenges-acme-cert-manager-io.yaml", "solrclouds-solr-apache-org.yaml"} {
		crdFile := filepath.Join(".", "..", "testdata", "customresources", file)
		crdContent, err := os.ReadFile(crdFile)
		if err != nil {
			nt.T.Fatal(err)
		}
		nt.Must(rootSyncGitRepo.AddFile(fmt.Sprintf("acme/cluster/%s", file), crdContent))
	}
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding two large CRDs"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	nt.Must(nt.Watcher.WatchObject(kinds.CustomResourceDefinitionV1(), "challenges.acme.cert-manager.io", "",
		testwatcher.WatchTimeout(30*time.Second)))

	nt.Must(nt.Watcher.WatchObject(kinds.CustomResourceDefinitionV1(), "solrclouds.solr.apache.org", "",
		testwatcher.WatchTimeout(30*time.Second)))

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, rootSyncGitRepo.MustGet(nt.T, "acme/cluster/challenges-acme-cert-manager-io.yaml"))
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, rootSyncGitRepo.MustGet(nt.T, "acme/cluster/solrclouds-solr-apache-org.yaml"))

	nt.Must(nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
	}))

	// update one CRD
	crdFile := filepath.Join(".", "..", "testdata", "customresources", "challenges-acme-cert-manager-io_with_new_label.yaml")
	crdContent, err := os.ReadFile(crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}
	nt.Must(rootSyncGitRepo.AddFile("acme/cluster/challenges-acme-cert-manager-io.yaml", crdContent))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update label for one CRD"))
	nt.Must(nt.WatchForAllSyncs())

	nt.Must(nt.Watcher.WatchObject(kinds.CustomResourceDefinitionV1(), "challenges.acme.cert-manager.io", "",
		testwatcher.WatchPredicates(testpredicates.HasLabel("random-key", "random-value")),
		testwatcher.WatchTimeout(30*time.Second)))
}

func hasRule(name string) testpredicates.Predicate {
	return func(o client.Object) error {
		if o == nil {
			return testpredicates.ErrObjectNotFound
		}
		vwc, ok := o.(*admissionv1.ValidatingWebhookConfiguration)
		if !ok {
			return testpredicates.WrongTypeErr(o, &admissionv1.ValidatingWebhookConfiguration{})
		}
		for _, w := range vwc.Webhooks {
			if w.Name == name {
				return nil
			}
		}
		return fmt.Errorf("missing ValidatingWebhook %q", name)
	}
}

func hasTwoVersions(obj client.Object) error {
	if obj == nil {
		return testpredicates.ErrObjectNotFound
	}
	crd := obj.(*apiextensionsv1.CustomResourceDefinition)
	if len(crd.Spec.Versions) != 2 {
		return errors.New("the CRD should contain 2 versions")
	}
	if crd.Spec.Versions[0].Name != "v1" || crd.Spec.Versions[1].Name != "v2" {
		return errors.New("incorrect versions for CRD")
	}
	return nil
}

func clusteranvilCR(version, name string, weight int64) *unstructured.Unstructured {
	u := newAnvilObject(version, name, weight)
	gvk := u.GroupVersionKind()
	gvk.Kind = "ClusterAnvil"
	u.SetGroupVersionKind(gvk)
	return u
}

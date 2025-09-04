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
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/metrics"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/retry"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testutils"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testwatcher"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/webhook/configuration"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kstatus "sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCRDDeleteBeforeRemoveCustomResourceV1(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	crdFile := filepath.Join(".", "..", "testdata", "customresources", "v1_crds", "anvil-crd.yaml")
	clusterFile := filepath.Join(".", "..", "testdata", "customresources", "v1_crds", "clusteranvil-crd.yaml")
	nt.Must(nt.Shell.Kubectl("apply", "-f", crdFile))
	nt.Must(nt.Shell.Kubectl("apply", "-f", clusterFile))

	nt.Must(nomostest.WaitForCRDs(nt, []string{"anvils.acme.com", "clusteranvils.acme.com"}))

	nt.T.Log("Increase log level of reconciler to help debug failures")
	nt.Must(testutils.UpdateRootSyncReconcilerLogLevel(nt,
		configsync.RootSyncName, reconcilermanager.Reconciler, 5))

	nsObj := k8sobjects.NamespaceObject("foo")
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/ns.yaml", nsObj))
	anvilObj := newAnvilObject("v1", "heavy", 10)
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/anvil-v1.yaml", anvilObj))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding Anvil CR"))
	firstCommitHash := rootSyncGitRepo.MustHash(nt.T)

	nt.Must(nt.WatchForAllSyncs())

	// Reset discovery client to pick up Anvil CRD
	nt.RenewClient()

	nt.Must(nt.Validate(configuration.Name, "", &admissionv1.ValidatingWebhookConfiguration{},
		hasRule("acme.com.v1.admission-webhook.configsync.gke.io")))

	// Use Retry & Validate instead of WatchForCurrentStatus, because
	// watching would require creating API objects and updating the scheme.
	nt.Must(retry.Retry(60*time.Second, func() error {
		return nt.Validate("heavy", "foo", newAnvilObject("v1", "", 0),
			testpredicates.StatusEquals(nt.Scheme, kstatus.CurrentStatus))
	}))

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, nsObj)
	nt.MetricsExpectations.AddObjectApply(configsync.RootSyncKind, rootSyncNN, anvilObj)

	nt.Must(nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: rootSyncNN,
	}))

	// Remove CRD
	// This will garbage collect the CR too and block until both are deleted.
	nt.T.Log("Deleting Anvil CRD")
	nt.Must(nt.Shell.Kubectl("delete", "-f", crdFile))

	rootSyncLabels, err := nomostest.MetricLabelsForRootSync(nt, rootSyncNN)
	if err != nil {
		nt.T.Fatal(err)
	}

	// Resource conflict errors are recorded as status errors when the
	// remediator watches are updated after an apply succeeds, but not when
	// watches are updated before the apply attempt or from watch events handled
	// by the remediator. So we don't expect to see a resource conflict error
	// in the RootSync status until after the next apply attempt fails, which
	// won't happen until the next automatic re-sync (1hr default).
	//
	// However, we do expect the remediator to get a deletion event for each
	// Anvil object after the Anvil CRD is deleted. This can be surfaced as one
	// of the following errors:
	// - NoResourceMatchError
	// - NoKindMatchError
	// - ObjectNotFound
	// - ResourceVersionConflict
	// Which error depends on race conditions between the remediator and the
	// Kubernetes custom resource controller.
	//
	// Note: This is NOT a "management conflict", just a "resource conflict".
	// But both types of conflict both share the same metric.
	// TODO: distinguish between management conflict (unexpected manager annotation) and resource conflict (resource version change)
	nt.Must(nomostest.ValidateMetrics(nt,
		nomostest.ReconcilerErrorMetrics(nt, rootSyncLabels, firstCommitHash, metrics.ErrorSummary{
			Conflicts: 1, // at least 1
		})))

	// Reset discovery client to invalidate the cached Anvil CRD
	nt.RenewClient()

	// Modify the Anvil yaml to trigger immediate re-sync, instead of waiting
	// for automatic retry (1hr default).
	anvilObj = newAnvilObject("v1", "heavy", 100)
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/anvil-v1.yaml", anvilObj))
	nt.Must(rootSyncGitRepo.CommitAndPush("Modify Anvil CR"))
	secondCommitHash := rootSyncGitRepo.MustHash(nt.T)

	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.UnknownKindErrorCode, ""))

	rootSyncLabels, err = nomostest.MetricLabelsForRootSync(nt, rootSyncNN)
	if err != nil {
		nt.T.Fatal(err)
	}

	// Validate the source error metric was recorded.
	// No management conflict is expected for this new commit, because the
	// remediator should be paused waiting for the sync to succeed.
	nt.Must(nomostest.ValidateMetrics(nt,
		nomostest.ReconcilerErrorMetrics(nt, rootSyncLabels, secondCommitHash, metrics.ErrorSummary{
			Source: 1,
		})))

	// Remove the CR.
	// This should fix the error and the conflict.
	nt.Must(rootSyncGitRepo.Remove("acme/namespaces/foo/anvil-v1.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Removing the Anvil CR as well"))
	nt.Must(nt.WatchForAllSyncs())

	// CR wasn't added to the inventory, so it won't be deleted.
	nt.MetricsExpectations.RemoveObject(configsync.RootSyncKind, rootSyncNN, anvilObj)

	// Validate reconciler error and conflict are cleared.
	nt.Must(nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: rootSyncNN,
	}))
}

func TestSyncUpdateCustomResource(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	crdFile := filepath.Join(".", "..", "testdata", "customresources", "v1_crds", "anvil-crd.yaml")
	_, err := nt.Shell.Kubectl("apply", "-f", crdFile)
	if err != nil {
		nt.T.Fatal(err)
	}

	nt.T.Cleanup(func() {
		// Cleanup CRD
		_, err := nt.Shell.Kubectl("delete", "-f", crdFile)
		if err != nil {
			nt.T.Fatal(err)
		}

		err = nt.Watcher.WatchForNotFound(kinds.CustomResourceDefinitionV1(), "anvils.acme.com", "")
		if err != nil {
			nt.T.Fatal(err)
		}
	})

	nt.Must(nt.Watcher.WatchObject(kinds.CustomResourceDefinitionV1(), "anvils.acme.com", "",
		testwatcher.WatchPredicates(nomostest.IsEstablished),
		testwatcher.WatchTimeout(30*time.Second)))

	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/ns.yaml", k8sobjects.NamespaceObject("foo")))
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/anvil-v1.yaml", newAnvilObject("v1", "heavy", 10)))
	nt.Must(rootSyncGitRepo.CommitAndPush("Adding Anvil CR"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	err = nt.Validate("heavy", "foo", newAnvilObject("v1", "", 0), weightEqual10)
	if err != nil {
		nt.T.Fatal(err)
	}

	// Update CustomResource
	nt.Must(rootSyncGitRepo.Add("acme/namespaces/foo/anvil-v1.yaml", newAnvilObject("v1", "heavy", 100)))
	nt.Must(rootSyncGitRepo.CommitAndPush("Updating Anvil CR"))
	nt.Must(nt.WatchForAllSyncs())
	nt.RenewClient()

	err = nt.Validate("heavy", "foo", newAnvilObject("v1", "", 0), weightEqual100)
	if err != nil {
		nt.T.Fatal(err)
	}

}

func weightEqual100(obj client.Object) error {
	if obj == nil {
		return testpredicates.ErrObjectNotFound
	}
	u := obj.(*unstructured.Unstructured)
	val, _, err := unstructured.NestedInt64(u.Object, "spec", "lbs")
	if err != nil {
		return err
	}
	if val != 100 {
		return fmt.Errorf(".spec.lbs should be 100 but got %d", val)
	}
	return nil
}

func weightEqual10(obj client.Object) error {
	if obj == nil {
		return testpredicates.ErrObjectNotFound
	}
	u := obj.(*unstructured.Unstructured)
	val, _, err := unstructured.NestedInt64(u.Object, "spec", "lbs")
	if err != nil {
		return err
	}
	if val != 10 {
		return fmt.Errorf(".spec.lbs should be 10 but got %d", val)
	}
	return nil
}

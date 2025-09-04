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
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// templatesGroup is the api group for gatekeeper constraint templates
	templatesGroup = "templates.gatekeeper.sh"
)

var (
	gk = schema.GroupKind{
		Group: templatesGroup,
		Kind:  "ConstraintTemplate",
	}

	// GVK is the GVK for gatekeeper ConstraintTemplates.
	gatekeeperGVK = gk.WithVersion("v1beta1")
)

// emptyConstraintTemplate returns an empty ConstraintTemplate.
func emptyConstraintTemplate() unstructured.Unstructured {
	ct := unstructured.Unstructured{}
	ct.SetGroupVersionKind(gatekeeperGVK)
	return ct
}

func TestConstraintTemplateAndConstraintInSameCommit(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation1,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	crdName := "k8sallowedrepos.constraints.gatekeeper.sh"
	nt.T.Logf("Delete the %q CRD if needed", crdName)
	nt.MustKubectl("delete", "crd", crdName, "--ignore-not-found")

	// Simulate install of Gatekeeper with just the ConstraintTemplate CRD
	if err := nt.ApplyGatekeeperCRD("constraint-template-crd.yaml", "constrainttemplates.templates.gatekeeper.sh"); err != nil {
		nt.T.Fatalf("Failed to create ConstraintTemplate CRD: %v", err)
	}

	nt.T.Log("Adding ConstraintTemplate & Constraint in one commit")
	nt.Must(rootSyncGitRepo.Copy("../testdata/gatekeeper/constraint-template.yaml", "acme/cluster/constraint-template.yaml"))
	nt.Must(rootSyncGitRepo.Copy("../testdata/gatekeeper/constraint.yaml", "acme/cluster/constraint.yaml"))
	nt.Must(rootSyncGitRepo.CommitAndPush("Add ConstraintTemplate & Constraint"))

	// Cleanup if waiting for sync error fails.
	nt.T.Cleanup(func() {
		if nt.T.Failed() {
			// Cleanup before deleting the ConstraintTemplate CRDs to avoid resource conflict errors from the webhook.
			nt.Must(rootSyncGitRepo.Remove("acme/cluster"))
			// Add back the safety ClusterRole to pass the safety check (KNV2006).
			nt.Must(rootSyncGitRepo.AddSafetyClusterRole())
			nt.Must(rootSyncGitRepo.CommitAndPush("Reset the acme directory"))
			nt.Must(nt.WatchForAllSyncs())
		}
	})

	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.UnknownKindErrorCode,
		`No CustomResourceDefinition is defined for the type "K8sAllowedRepos.constraints.gatekeeper.sh" in the cluster`))

	// Simulate Gatekeeper's controller behavior.
	// Wait for the ConstraintTemplate to be applied, then apply the Constraint CRD.
	nt.Must(nomostest.Wait(nt.T, "ConstraintTemplate on API server", 2*time.Minute, func() error {
		ct := emptyConstraintTemplate()
		return nt.Validate("k8sallowedrepos", "", &ct)
	}))
	if err := nt.ApplyGatekeeperCRD("constraint-crd.yaml", "k8sallowedrepos.constraints.gatekeeper.sh"); err != nil {
		nt.T.Fatalf("Failed to create constraint CRD: %v", err)
	}
	// Sync should eventually succeed on retry, now that all the required CRDs exist.
	nt.Must(nt.WatchForAllSyncs())

	// Cleanup before deleting the ConstraintTemplate and Constraint CRDs to avoid resource conflict errors from the webhook.
	nt.Must(rootSyncGitRepo.Remove("acme/cluster"))
	// Add back the safety ClusterRole to pass the safety check (KNV2006).
	nt.Must(rootSyncGitRepo.AddSafetyClusterRole())
	nt.Must(rootSyncGitRepo.CommitAndPush("Reset the acme directory"))
	nt.Must(nt.WatchForAllSyncs())
}

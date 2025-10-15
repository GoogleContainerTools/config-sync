// Copyright 2025 Google LLC
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
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/syncsource"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testpredicates"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
	corev1 "k8s.io/api/core/v1"
)

// This test currently requires KinD because MutatingAdmissionPolicy is alpha
// and requires a feature gate.
func TestMutatingAdmissionPolicy(t *testing.T) {
	nt := nomostest.New(t, nomostesting.Reconciliation2,
		ntopts.SyncWithGitSource(nomostest.DefaultRootSyncID, ntopts.Unstructured),
		ntopts.RequireKind(t))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	mapFile := filepath.Join(".", "..", "..", "examples", "mutating-admission-policies", "config-sync-node-placement.yaml")
	nt.T.Cleanup(func() {
		nt.Must(nt.Shell.Kubectl("delete", "--ignore-not-found", "-f", mapFile))
		nt.Must(nomostest.Wait(nt.T, "reconciler-manager has no nodeAffinity", time.Minute, func() error {
			if err := nomostest.DeletePodByLabel(nt, "app", reconcilermanager.ManagerName, false); err != nil {
				return err
			}
			return nomostest.ValidatePodByLabel(nt, "app", reconcilermanager.ManagerName,
				testpredicates.HasExactlyNodeAffinity(&corev1.NodeAffinity{}))
		}))
	})
	nt.Must(nt.Shell.Kubectl("apply", "-f", mapFile))

	// expected nodeAffinity from the example MutatingAdmissionPolicy yaml
	exampleNodeAffinity := &corev1.NodeAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
			{
				Weight: 1,
				Preference: corev1.NodeSelectorTerm{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "another-node-label-key",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"another-node-label-value"},
						},
					},
				},
			},
		},
	}

	// bounce reconciler-manager Pod and verify the nodeAffinity is applied by MAP
	nt.Must(nomostest.ValidatePodByLabel(nt, "app", reconcilermanager.ManagerName,
		testpredicates.HasExactlyNodeAffinity(&corev1.NodeAffinity{})))
	nt.T.Log("Replacing the reconciler-manager Pod to validate nodeAffinity is added")
	// MutatingAdmissionPolicy does not surface a readiness status, so need to retry.
	nt.Must(nomostest.Wait(nt.T, "reconciler-manager has mutated nodeAffinity", time.Minute, func() error {
		if err := nomostest.DeletePodByLabel(nt, "app", reconcilermanager.ManagerName, false); err != nil {
			return err
		}
		return nomostest.ValidatePodByLabel(nt, "app", reconcilermanager.ManagerName,
			testpredicates.HasExactlyNodeAffinity(exampleNodeAffinity))
	}))

	// update the RootSync to trigger a Deployment change, verify new reconciler Pod has nodeAffinity
	rootSync := nomostest.RootSyncObjectV1Beta1FromRootRepo(nt, nomostest.DefaultRootSyncID.Name)
	rootSync.Spec.Git.Dir = "foo"
	nt.Must(nt.KubeClient.Apply(rootSync))
	nt.Must(rootSyncGitRepo.Add("foo/ns.yaml", k8sobjects.NamespaceObject("test-map-ns")))
	nt.Must(rootSyncGitRepo.CommitAndPush("add foo-ns under foo/ dir"))
	nt.Must(nt.WatchForSync(kinds.RootSyncV1Beta1(), rootSync.Name, configsync.ControllerNamespace,
		&syncsource.GitSyncSource{
			ExpectedCommit:    rootSyncGitRepo.MustHash(t),
			ExpectedDirectory: "foo",
		}))
	nt.Must(nt.Validate("test-map-ns", "", &corev1.Namespace{}))
	nt.Must(nomostest.ValidatePodByLabel(nt, "app", reconcilermanager.Reconciler,
		testpredicates.HasExactlyNodeAffinity(exampleNodeAffinity)))
}

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

package finalizer

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/api/kpt.dev/v1alpha1"
	"github.com/GoogleContainerTools/config-sync/pkg/applier"
	"github.com/GoogleContainerTools/config-sync/pkg/applier/stats"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/syncer/syncertest/fake"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/testerrors"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/cli-utils/pkg/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// decoder uses core.Scheme to parse YAML/JSON into typed objects
var scheme = core.Scheme
var decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()

var rootSync1Yaml = `
apiVersion: configsync.gke.io/v1beta1
kind: RootSync
metadata:
  name: root-sync
  namespace: config-management-system
  uid: "1"
  resourceVersion: "1"
  generation: 1
spec:
  sourceFormat: unstructured
  git:
    repo: https://github.com/config-sync-examples/crontab-crs
    branch: main
    dir: configs
    auth: none
`

func TestRootSyncFinalize(t *testing.T) {
	rootSync1 := yamlToTypedObject(t, rootSync1Yaml).(*v1beta1.RootSync)
	metadata.SetDeletionPropagationPolicy(rootSync1, metadata.DeletionPropagationPolicyForeground)
	rootSync1.SetFinalizers([]string{
		metadata.ReconcilerFinalizer,
	})

	asserter := testutil.NewAsserter(
		cmpopts.EquateErrors(),
		cmpopts.IgnoreFields(metav1.Time{}, "Time"),
	)

	testCases := []struct {
		name                       string
		rsync                      client.Object
		deletionPolicy             metadata.DeletionPropagationPolicy
		setup                      func(*fake.Client) error
		destroyErrs                []status.Error
		expectedRsyncBeforeDestroy client.Object
		expectedError              error
		expectedStopped            bool
		expectedRsyncAfterFinalize client.Object
	}{
		{
			name:           "destroy happy path",
			rsync:          rootSync1.DeepCopy(),
			deletionPolicy: metadata.DeletionPropagationPolicyForeground,
			expectedRsyncBeforeDestroy: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				obj.SetResourceVersion("2")
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
				}
				return obj
			}(),
			expectedError:   nil,
			expectedStopped: true,
			expectedRsyncAfterFinalize: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				// +1 to remove ReconcilerFinalizing condition
				// +1 to remove Finalizer
				// TODO: optimize by combining consecutive updates
				obj.SetResourceVersion("4")
				// Finalizer has been removed
				obj.SetFinalizers(nil)
				// ReconcilerFinalizing condition added and then removed
				return obj
			}(),
		},
		{
			name:           "destroy failure",
			rsync:          rootSync1.DeepCopy(),
			deletionPolicy: metadata.DeletionPropagationPolicyForeground,
			expectedRsyncBeforeDestroy: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				obj.SetResourceVersion("2")
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
				}
				return obj
			}(),
			destroyErrs: []status.Error{
				status.APIServerError(fmt.Errorf("destroy error"), "example message"),
			},
			expectedError: fmt.Errorf(
				"deleting managed objects: %w",
				status.APIServerError(fmt.Errorf("destroy error"), "example message")),
			expectedStopped: true,
			expectedRsyncAfterFinalize: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				// +1 to set ReconcilerFinalizerFailure condition
				obj.SetResourceVersion("3")
				// Finalizer NOT removed
				// ReconcilerFinalizing condition added and NOT removed
				// ReconcilerFinalizerFailure condition added
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
					{
						Type:    v1beta1.RootSyncReconcilerFinalizerFailure,
						Status:  metav1.ConditionTrue,
						Reason:  "DestroyFailure",
						Message: "Failed to delete managed resource objects",
						Errors: []v1beta1.ConfigSyncError{
							{
								Code:         "2002",
								ErrorMessage: "KNV2002: example message: APIServer error: destroy error\n\nFor more information, see https://g.co/cloud/acm-errors#knv2002",
							},
						},
					},
				}
				return obj
			}(),
		},
		{
			name:           "destroy recovery",
			deletionPolicy: metadata.DeletionPropagationPolicyForeground,
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				// +1 to set ReconcilerFinalizerFailure condition
				obj.SetResourceVersion("3")
				// Finalizer NOT removed
				// ReconcilerFinalizing condition added and NOT removed
				// ReconcilerFinalizerFailure condition added
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
					{
						Type:    v1beta1.RootSyncReconcilerFinalizerFailure,
						Status:  metav1.ConditionTrue,
						Reason:  "DestroyFailure",
						Message: "Failed to delete managed resource objects",
						Errors: []v1beta1.ConfigSyncError{
							{
								Code:         "2002",
								ErrorMessage: "KNV2002: example message: APIServer error: destroy error\n\nFor more information, see https://g.co/cloud/acm-errors#knv2002",
							},
						},
					},
				}
				return obj
			}(),
			expectedRsyncBeforeDestroy: func() client.Object {
				obj := rootSync1.DeepCopy()
				// No changes - continue deleting
				obj.SetResourceVersion("3")
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
					{
						Type:    v1beta1.RootSyncReconcilerFinalizerFailure,
						Status:  metav1.ConditionTrue,
						Reason:  "DestroyFailure",
						Message: "Failed to delete managed resource objects",
						Errors: []v1beta1.ConfigSyncError{
							{
								Code:         "2002",
								ErrorMessage: "KNV2002: example message: APIServer error: destroy error\n\nFor more information, see https://g.co/cloud/acm-errors#knv2002",
							},
						},
					},
				}
				return obj
			}(),
			destroyErrs:     nil,
			expectedError:   nil,
			expectedStopped: true,
			expectedRsyncAfterFinalize: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to remove ReconcilerFinalizing condition
				// +1 to remove ReconcilerFinalizerFailure condition
				// +1 to remove Finalizer
				// TODO: optimize by combining consecutive updates
				obj.SetResourceVersion("6")
				// Finalizer has been removed
				obj.SetFinalizers(nil)
				// ReconcilerFinalizing condition removed
				// ReconcilerFinalizerFailure condition removed
				return obj
			}(),
		},
		{
			name:           "rsync not found",
			rsync:          rootSync1.DeepCopy(),
			deletionPolicy: metadata.DeletionPropagationPolicyForeground,
			setup: func(fakeClient *fake.Client) error {
				// remove the finalizer that blocks deletion
				ctx := context.Background()
				rs := &v1beta1.RootSync{}
				rs.Name = rootSync1.Name
				rs.Namespace = rootSync1.Namespace
				if err := updateToRemoveFinalizers(ctx, fakeClient, rs); err != nil {
					return err
				}
				// delete RootSync to cause update error
				return fakeClient.Delete(ctx, rs)
			},
			expectedError: fmt.Errorf(
				"setting Finalizing condition: %w",
				fmt.Errorf(
					"failed to set ReconcilerFinalizing condition: %w",
					status.APIServerErrorWrap(
						fmt.Errorf(
							"failed to update object status: %s: %w",
							kinds.ObjectSummary(rootSync1),
							apierrors.NewNotFound(
								kinds.RootSyncResource().GroupResource(),
								"config-management-system/root-sync")),
						rootSync1.DeepCopy()),
				),
			),
			expectedStopped:            true,
			expectedRsyncAfterFinalize: nil,
		},
		{
			name:           "orphan happy path",
			rsync:          rootSync1.DeepCopy(),
			deletionPolicy: metadata.DeletionPropagationPolicyOrphan,
			expectedRsyncBeforeDestroy: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				obj.SetResourceVersion("2")
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
				}
				return obj
			}(),
			expectedError:   nil,
			expectedStopped: true,
			expectedRsyncAfterFinalize: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				// +1 to remove ReconcilerFinalizing condition
				// +1 to remove Finalizer
				// TODO: optimize by combining consecutive updates
				obj.SetResourceVersion("4")
				// Finalizer has been removed
				obj.SetFinalizers(nil)
				// ReconcilerFinalizing condition added and then removed
				return obj
			}(),
		},
		{
			name:           "orphan recovery",
			deletionPolicy: metadata.DeletionPropagationPolicyOrphan,
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to set ReconcilerFinalizing condition
				// +1 to set ReconcilerFinalizerFailure condition
				obj.SetResourceVersion("3")
				// Finalizer NOT removed
				// ReconcilerFinalizing condition added and NOT removed
				// ReconcilerFinalizerFailure condition added
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
					{
						Type:    v1beta1.RootSyncReconcilerFinalizerFailure,
						Status:  metav1.ConditionTrue,
						Reason:  "DestroyFailure",
						Message: "Failed to delete managed resource objects",
						Errors: []v1beta1.ConfigSyncError{
							{
								Code:         "2002",
								ErrorMessage: "KNV2002: example message: APIServer error: destroy error\n\nFor more information, see https://g.co/cloud/acm-errors#knv2002",
							},
						},
					},
				}
				return obj
			}(),
			expectedRsyncBeforeDestroy: func() client.Object {
				obj := rootSync1.DeepCopy()
				// No changes - continue deleting
				obj.SetResourceVersion("3")
				obj.Status.Conditions = []v1beta1.RootSyncCondition{
					{
						Type:    v1beta1.RootSyncReconcilerFinalizing,
						Status:  metav1.ConditionTrue,
						Reason:  "ResourcesDeleting",
						Message: "Deleting managed resource objects",
					},
					{
						Type:    v1beta1.RootSyncReconcilerFinalizerFailure,
						Status:  metav1.ConditionTrue,
						Reason:  "DestroyFailure",
						Message: "Failed to delete managed resource objects",
						Errors: []v1beta1.ConfigSyncError{
							{
								Code:         "2002",
								ErrorMessage: "KNV2002: example message: APIServer error: destroy error\n\nFor more information, see https://g.co/cloud/acm-errors#knv2002",
							},
						},
					},
				}
				return obj
			}(),
			destroyErrs:     nil,
			expectedError:   nil,
			expectedStopped: true,
			expectedRsyncAfterFinalize: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to remove ReconcilerFinalizing condition
				// +1 to remove ReconcilerFinalizerFailure condition
				// +1 to remove Finalizer
				// TODO: optimize by combining consecutive updates
				obj.SetResourceVersion("6")
				// Finalizer has been removed
				obj.SetFinalizers(nil)
				// ReconcilerFinalizing condition removed
				// ReconcilerFinalizerFailure condition removed
				return obj
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up a toy object referenced by the ResourceGroup
			cm := k8sobjects.ConfigMapObject(
				core.Name("cm1"), core.Namespace(tc.rsync.GetNamespace()),
				core.Annotation("a-key", "a-value"),
				core.Label("l-key", "l-value"))
			wantCM := cm.DeepCopy()
			wantCM.Generation = 1
			wantCM.ResourceVersion = "1"
			wantCM.UID = "1"

			applySetID := "apply-set-id"
			if tc.deletionPolicy == metadata.DeletionPropagationPolicyForeground {
				wantCM.SetAnnotations(map[string]string{
					metadata.OwningInventoryKey:          "inventory-id",
					metadata.ManagementModeAnnotationKey: metadata.ManagementEnabled.String(),
					metadata.SyncTokenAnnotationKey:      "commit-hash",
					metadata.GitContextKey:               "git-context",
					metadata.ResourceManagerKey:          "manager-value",
					metadata.ResourceIDKey:               "_configmap_config-management-system_cm1",
					"a-key":                              "a-value",
				})
				wantCM.SetLabels(map[string]string{
					metadata.ManagedByKey:        metadata.ManagedByValue,
					metadata.ApplySetPartOfLabel: applySetID,
					"l-key":                      "l-value",
				})
			} else { // When "Orphan" is used, all CS metadata should be removed from the CM
				wantCM.ResourceVersion = "2"
				wantCM.SetAnnotations(map[string]string{
					"a-key": "a-value",
				})
				wantCM.SetLabels(map[string]string{
					"l-key": "l-value",
				})
			}
			csm := metadata.ConfigSyncMetadata{
				ApplySetID:      "apply-set-id",
				GitContextValue: "git-context",
				ManagerValue:    "manager-value",
				SourceHash:      "commit-hash",
				InventoryID:     "inventory-id",
			}
			csm.SetConfigSyncMetadata(cm)
			// Create a ResourceGroup which references the ConfigMap
			// "Orphan" will look up the ResourceGroup to remove CS metadata from managed objects
			rg := &v1alpha1.ResourceGroup{}
			rg.Name = tc.rsync.GetName()
			rg.Namespace = tc.rsync.GetNamespace()
			rg.Spec.Resources = []v1alpha1.ObjMetadata{
				{
					Name:      cm.Name,
					Namespace: cm.Namespace,
					GroupKind: v1alpha1.GroupKind{
						Group: cm.GroupVersionKind().Group,
						Kind:  cm.GroupVersionKind().Kind,
					},
				},
			}
			metadata.SetDeletionPropagationPolicy(tc.rsync, tc.deletionPolicy)

			fakeClient := fake.NewClient(t, scheme, tc.rsync, cm, rg)
			ctx := context.Background()

			stopped := false
			continueCh := make(chan struct{})
			stopFunc := func() {
				defer close(continueCh)
				stopped = true
			}
			destroyFunc := func(context.Context) []status.Error {
				// Lookup the current RootSync
				key := client.ObjectKeyFromObject(rootSync1)
				rsync := &v1beta1.RootSync{}
				err := fakeClient.Get(context.Background(), key, rsync)
				require.NoError(t, err)
				metadata.SetDeletionPropagationPolicy(tc.expectedRsyncBeforeDestroy, tc.deletionPolicy)
				asserter.Equal(t, tc.expectedRsyncBeforeDestroy, rsync)
				// Return errors, if any
				return tc.destroyErrs
			}
			fakeDestroyer := newFakeDestroyer(tc.destroyErrs, destroyFunc)
			finalizer := &RootSyncFinalizer{
				baseFinalizer: baseFinalizer{
					Destroyer:  fakeDestroyer,
					Client:     fakeClient,
					ApplySetID: applySetID,
				},
				StopControllers:    stopFunc,
				ControllersStopped: continueCh,
			}

			if tc.setup != nil {
				err := tc.setup(fakeClient)
				require.NoError(t, err)
			}

			err := finalizer.Finalize(ctx, tc.rsync)
			testerrors.AssertEqual(t, tc.expectedError, err)

			assert.Equal(t, tc.expectedStopped, stopped)
			expectedObjs := []client.Object{rg, wantCM}
			if tc.expectedRsyncAfterFinalize != nil {
				metadata.SetDeletionPropagationPolicy(tc.expectedRsyncAfterFinalize, tc.deletionPolicy)
				expectedObjs = append(expectedObjs, tc.expectedRsyncAfterFinalize)
			}
			fakeClient.Check(t, expectedObjs...)
		})
	}
}

func TestRootSyncAddFinalizer(t *testing.T) {
	rootSync1 := yamlToTypedObject(t, rootSync1Yaml).(*v1beta1.RootSync)

	testCases := []struct {
		name            string
		rsync           client.Object
		setup           func(*fake.Client) error
		expectedError   error
		expectedUpdated bool
		expectedRsync   client.Object
	}{
		{
			name:            "add finalizer",
			rsync:           rootSync1.DeepCopy(),
			expectedError:   nil,
			expectedUpdated: true,
			expectedRsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to add Finalizer
				obj.SetResourceVersion("2")
				obj.SetFinalizers([]string{
					metadata.ReconcilerFinalizer,
				})
				return obj
			}(),
		},
		{
			name: "add finalizer again",
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				obj.SetResourceVersion("2")
				obj.SetFinalizers([]string{
					metadata.ReconcilerFinalizer,
				})
				return obj
			}(),
			expectedError:   nil,
			expectedUpdated: false,
			expectedRsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// No change
				obj.SetResourceVersion("2")
				obj.SetFinalizers([]string{
					metadata.ReconcilerFinalizer,
				})
				return obj
			}(),
		},
		{
			name: "add finalizer to list",
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				obj.SetResourceVersion("2")
				obj.SetFinalizers([]string{
					"some-other-finalizer",
				})
				return obj
			}(),
			expectedError:   nil,
			expectedUpdated: true,
			expectedRsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to add Finalizer
				obj.SetResourceVersion("3")
				obj.SetFinalizers([]string{
					"some-other-finalizer",
					metadata.ReconcilerFinalizer,
				})
				return obj
			}(),
		},
		{
			name:  "rsync not found",
			rsync: rootSync1.DeepCopy(),
			setup: func(fakeClient *fake.Client) error {
				// remove the finalizer that blocks deletion
				ctx := context.Background()
				rs := &v1beta1.RootSync{}
				rs.Name = rootSync1.Name
				rs.Namespace = rootSync1.Namespace
				if err := updateToRemoveFinalizers(ctx, fakeClient, rs); err != nil {
					return err
				}
				// delete RootSync to cause update error
				return fakeClient.Delete(ctx, rs)
			},
			expectedError: fmt.Errorf(
				"failed to add finalizer: %w",
				status.APIServerErrorWrap(
					fmt.Errorf(
						"failed to update object: %s: %w",
						kinds.ObjectSummary(rootSync1),
						apierrors.NewNotFound(
							kinds.RootSyncResource().GroupResource(),
							"config-management-system/root-sync"),
					),
					rootSync1.DeepCopy()),
			),
			expectedUpdated: false,
			expectedRsync:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClient(t, scheme, tc.rsync)
			ctx := context.Background()

			finalizer := &RootSyncFinalizer{
				baseFinalizer: baseFinalizer{
					Client: fakeClient,
				},
			}

			if tc.setup != nil {
				err := tc.setup(fakeClient)
				require.NoError(t, err)
			}

			updated, err := finalizer.AddFinalizer(ctx, tc.rsync)
			testerrors.AssertEqual(t, tc.expectedError, err)

			assert.Equal(t, tc.expectedUpdated, updated)
			var expectedObjs []client.Object
			if tc.expectedRsync != nil {
				expectedObjs = append(expectedObjs, tc.expectedRsync)
			}
			fakeClient.Check(t, expectedObjs...)
		})
	}
}

func TestRootSyncRemoveFinalizer(t *testing.T) {
	rootSync1 := yamlToTypedObject(t, rootSync1Yaml).(*v1beta1.RootSync)

	testCases := []struct {
		name            string
		rsync           client.Object
		setup           func(*fake.Client) error
		expectedError   error
		expectedUpdated bool
		expectedRsync   client.Object
	}{
		{
			name: "remove finalizer",
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				obj.SetResourceVersion("2")
				obj.SetFinalizers([]string{
					metadata.ReconcilerFinalizer,
				})
				return obj
			}(),
			expectedError:   nil,
			expectedUpdated: true,
			expectedRsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to remove Finalizer
				obj.SetResourceVersion("3")
				obj.SetFinalizers(nil)
				return obj
			}(),
		},
		{
			name: "remove finalizer again",
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to remove Finalizer
				obj.SetResourceVersion("3")
				obj.SetFinalizers(nil)
				return obj
			}(),
			expectedError:   nil,
			expectedUpdated: false,
			expectedRsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// No change
				obj.SetResourceVersion("3")
				obj.SetFinalizers(nil)
				return obj
			}(),
		},
		{
			name: "remove finalizer from list",
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				obj.SetResourceVersion("2")
				obj.SetFinalizers([]string{
					"some-other-finalizer",
					metadata.ReconcilerFinalizer,
				})
				return obj
			}(),
			expectedError:   nil,
			expectedUpdated: true,
			expectedRsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				// +1 to add Finalizer
				obj.SetResourceVersion("3")
				obj.SetFinalizers([]string{
					"some-other-finalizer",
				})
				return obj
			}(),
		},
		{
			name: "rsync not found",
			rsync: func() client.Object {
				obj := rootSync1.DeepCopy()
				obj.SetFinalizers([]string{
					metadata.ReconcilerFinalizer,
				})
				return obj
			}(),
			setup: func(fakeClient *fake.Client) error {
				// remove the finalizer that blocks deletion
				ctx := context.Background()
				rs := &v1beta1.RootSync{}
				rs.Name = rootSync1.Name
				rs.Namespace = rootSync1.Namespace
				if err := updateToRemoveFinalizers(ctx, fakeClient, rs); err != nil {
					return err
				}
				// delete RootSync to cause update error
				return fakeClient.Delete(ctx, rs)
			},
			expectedError: fmt.Errorf(
				"failed to remove finalizer: %w",
				status.APIServerErrorWrap(
					fmt.Errorf(
						"failed to update object: %s: %w",
						kinds.ObjectSummary(rootSync1),
						apierrors.NewNotFound(
							kinds.RootSyncResource().GroupResource(),
							"config-management-system/root-sync"),
					),
					rootSync1.DeepCopy()),
			),
			expectedUpdated: false,
			expectedRsync:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClient(t, scheme, tc.rsync)
			ctx := context.Background()

			finalizer := &RootSyncFinalizer{
				baseFinalizer: baseFinalizer{
					Client: fakeClient,
				},
			}

			if tc.setup != nil {
				err := tc.setup(fakeClient)
				require.NoError(t, err)
			}

			updated, err := finalizer.RemoveFinalizer(ctx, tc.rsync)
			testerrors.AssertEqual(t, tc.expectedError, err)

			assert.Equal(t, tc.expectedUpdated, updated)
			var expectedObjs []client.Object
			if tc.expectedRsync != nil {
				expectedObjs = append(expectedObjs, tc.expectedRsync)
			}
			fakeClient.Check(t, expectedObjs...)
		})
	}
}

func yamlToTypedObject(t *testing.T, yml string) client.Object {
	uObj := &unstructured.Unstructured{}
	_, _, err := decoder.Decode([]byte(yml), nil, uObj)
	if err != nil {
		t.Fatalf("error decoding yaml: %v", err)
	}
	rObj, err := kinds.ToTypedObject(uObj, scheme)
	if err != nil {
		t.Fatalf("error converting object: %v", err)
	}
	cObj, err := kinds.ObjectAsClientObject(rObj)
	if err != nil {
		t.Fatalf("error casting object: %v", err)
	}
	return cObj
}

func updateToRemoveFinalizers(ctx context.Context, fakeClient *fake.Client, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	if err := fakeClient.Get(ctx, key, obj); err != nil {
		return err
	}
	obj.SetFinalizers(nil)
	return fakeClient.Update(ctx, obj, client.FieldOwner(fake.FieldManager))
}

type fakeDestroyer struct {
	errors      []status.Error
	destroyFunc func(context.Context) []status.Error
}

var _ applier.Destroyer = &fakeDestroyer{}

func newFakeDestroyer(errs []status.Error, destroyFunc func(context.Context) []status.Error) *fakeDestroyer {
	return &fakeDestroyer{
		errors:      errs,
		destroyFunc: destroyFunc,
	}
}

func (d *fakeDestroyer) Destroy(ctx context.Context, eventHandler func(applier.Event)) (applier.ObjectStatusMap, *stats.SyncStats) {
	var errs []status.Error
	if d.destroyFunc != nil {
		errs = d.destroyFunc(ctx)
	} else {
		errs = d.errors
	}
	for _, err := range errs {
		eventHandler(applier.ErrorEvent{
			Error: err,
		})
	}
	// TODO: test ObjectStatusMap & SyncStats
	return applier.ObjectStatusMap{}, &stats.SyncStats{}
}

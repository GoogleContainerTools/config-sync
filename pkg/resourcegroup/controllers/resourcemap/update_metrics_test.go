/*
Copyright 2025 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resourcemap

import (
	"context"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/api/kpt.dev/v1alpha1"
	"github.com/GoogleContainerTools/config-sync/pkg/resourcegroup/controllers/metrics"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/testmetrics"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"k8s.io/apimachinery/pkg/types"
)

func TestResourceMapUpdateMetrics(t *testing.T) {
	ctx := context.Background()

	testcases := []struct {
		name                   string
		group                  types.NamespacedName
		resources              []v1alpha1.ObjMetadata
		isDelete               bool
		expectedMetricValue    float64
		expectedResourceGroups int
	}{
		{
			name:  "Add single resource group",
			group: types.NamespacedName{Name: "root-sync", Namespace: configsync.ControllerNamespace},
			resources: []v1alpha1.ObjMetadata{
				{
					Name:      "test-deployment",
					Namespace: "default",
					GroupKind: v1alpha1.GroupKind{
						Group: "apps",
						Kind:  "Deployment",
					},
				},
			},
			isDelete:               false,
			expectedMetricValue:    1,
			expectedResourceGroups: 1,
		},
		{
			name:  "Add second resource group",
			group: types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"},
			resources: []v1alpha1.ObjMetadata{
				{
					Name:      "test-service",
					Namespace: "bookinfo",
					GroupKind: v1alpha1.GroupKind{
						Group: "",
						Kind:  "Service",
					},
				},
			},
			isDelete:               false,
			expectedMetricValue:    2, // root-sync + repo-sync
			expectedResourceGroups: 2,
		},
		{
			name:                   "Delete first resource group",
			group:                  types.NamespacedName{Name: "root-sync", Namespace: configsync.ControllerNamespace},
			resources:              []v1alpha1.ObjMetadata{},
			isDelete:               true,
			expectedMetricValue:    1, // Only repo-sync remains
			expectedResourceGroups: 1,
		},
		{
			name:                   "Delete remaining resource group",
			group:                  types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"},
			resources:              []v1alpha1.ObjMetadata{},
			isDelete:               true,
			expectedMetricValue:    0, // No resource groups remain
			expectedResourceGroups: 0,
		},
		{
			name:  "Add resource group with multiple resources",
			group: types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"},
			resources: []v1alpha1.ObjMetadata{
				{
					Name:      "test-service",
					Namespace: "bookinfo",
					GroupKind: v1alpha1.GroupKind{
						Group: "",
						Kind:  "Service",
					},
				},
				{
					Name:      "test-deployment",
					Namespace: "bookinfo",
					GroupKind: v1alpha1.GroupKind{
						Group: "apps",
						Kind:  "Deployment",
					},
				},
			},
			isDelete:               false,
			expectedMetricValue:    1, // 1 resource group with multiple resources
			expectedResourceGroups: 1,
		},
	}

	// Create a single resource map for the entire test
	m := NewResourceMap()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Register metrics views with test exporter for this test
			exporter := testmetrics.RegisterMetrics(
				metrics.ResourceGroupTotalView,
			)

			_ = m.Reconcile(ctx, tc.group, tc.resources, tc.isDelete)

			expected := []*view.Row{
				{Data: &view.LastValueData{Value: tc.expectedMetricValue}, Tags: []tag.Tag{}},
			}

			if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
				t.Errorf("Unexpected metrics recorded: %v", diff)
			}

			if len(m.resgroupToResources) != tc.expectedResourceGroups {
				t.Errorf("Expected %d resource groups in map, got %d", tc.expectedResourceGroups, len(m.resgroupToResources))
			}
		})
	}
}

func TestResourceMapMultipleUpdates(t *testing.T) {
	ctx := context.Background()

	testcases := []struct {
		name       string
		operations []struct {
			group     types.NamespacedName
			resources []v1alpha1.ObjMetadata
			isDelete  bool
		}
		expectedMetricValue    float64
		expectedResourceGroups int
	}{
		{
			name: "Add multiple resource groups sequentially",
			operations: []struct {
				group     types.NamespacedName
				resources []v1alpha1.ObjMetadata
				isDelete  bool
			}{
				{
					group: types.NamespacedName{Name: "root-sync", Namespace: configsync.ControllerNamespace},
					resources: []v1alpha1.ObjMetadata{
						{
							Name:      "deployment-1",
							Namespace: "default",
							GroupKind: v1alpha1.GroupKind{
								Group: "apps",
								Kind:  "Deployment",
							},
						},
					},
					isDelete: false,
				},
				{
					group: types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"},
					resources: []v1alpha1.ObjMetadata{
						{
							Name:      "service-1",
							Namespace: "bookinfo",
							GroupKind: v1alpha1.GroupKind{
								Group: "",
								Kind:  "Service",
							},
						},
					},
					isDelete: false,
				},
				{
					group: types.NamespacedName{Name: "another-sync", Namespace: "default"},
					resources: []v1alpha1.ObjMetadata{
						{
							Name:      "role-1",
							Namespace: "default",
							GroupKind: v1alpha1.GroupKind{
								Group: "rbac.authorization.k8s.io",
								Kind:  "Role",
							},
						},
					},
					isDelete: false,
				},
			},
			expectedMetricValue:    3, // All three resource groups
			expectedResourceGroups: 3,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Register metrics views with test exporter
			exporter := testmetrics.RegisterMetrics(
				metrics.ResourceGroupTotalView,
			)

			// Create a new resource map
			m := NewResourceMap()

			// Execute all operations
			for _, op := range tc.operations {
				_ = m.Reconcile(ctx, op.group, op.resources, op.isDelete)
			}

			// Verify final metrics
			expected := []*view.Row{
				{Data: &view.LastValueData{Value: tc.expectedMetricValue}, Tags: []tag.Tag{}},
			}

			if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
				t.Errorf("Unexpected metrics recorded: %v", diff)
			}

			// Verify the resource map state
			if len(m.resgroupToResources) != tc.expectedResourceGroups {
				t.Errorf("Expected %d resource groups in map, got %d", tc.expectedResourceGroups, len(m.resgroupToResources))
			}
		})
	}
}

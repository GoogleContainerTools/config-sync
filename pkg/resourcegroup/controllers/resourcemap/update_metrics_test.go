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
	"k8s.io/apimachinery/pkg/types"
)

type testOperation struct {
	group     types.NamespacedName
	resources []v1alpha1.ObjMetadata
	isDelete  bool
}

func TestResourceMapUpdateMetrics(t *testing.T) {
	ctx := context.Background()

	testcases := []struct {
		name                   string
		operations             []testOperation
		expectedMetricValue    float64
		expectedResourceGroups int
	}{
		{
			name: "Add single resource group",
			operations: []testOperation{
				{
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
					isDelete: false,
				},
			},
			expectedMetricValue:    1,
			expectedResourceGroups: 1,
		},
		{
			name: "Add multiple resource groups and delete them",
			operations: []testOperation{
				{
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
					isDelete: false,
				},
				{
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
					isDelete: false,
				},
				{
					group:     types.NamespacedName{Name: "root-sync", Namespace: configsync.ControllerNamespace},
					resources: []v1alpha1.ObjMetadata{},
					isDelete:  true,
				},
				{
					group:     types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"},
					resources: []v1alpha1.ObjMetadata{},
					isDelete:  true,
				},
			},
			expectedMetricValue:    0, // All resource groups deleted
			expectedResourceGroups: 0,
		},
		{
			name: "Add resource group with multiple resources",
			operations: []testOperation{
				{
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
					isDelete: false,
				},
			},
			expectedMetricValue:    1, // 1 resource group with multiple resources
			expectedResourceGroups: 1,
		},
		{
			name: "Add and update resource group",
			operations: []testOperation{
				{
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
					isDelete: false,
				},
				{
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
						{
							Name:      "test-service",
							Namespace: "default",
							GroupKind: v1alpha1.GroupKind{
								Group: "",
								Kind:  "Service",
							},
						},
					},
					isDelete: false,
				},
			},
			expectedMetricValue:    1, // Same resource group, now with 2 resources
			expectedResourceGroups: 1,
		},
		{
			name: "Add multiple resource groups sequentially",
			operations: []testOperation{
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
			exporter := testmetrics.NewTestExporter()

			// Create a new resource map for each test case
			m := NewResourceMap()

			// Execute all operations
			for _, op := range tc.operations {
				_ = m.Reconcile(ctx, op.group, op.resources, op.isDelete)
			}

			// Verify final metrics
			expected := []testmetrics.MetricData{
				{Name: metrics.ResourceGroupTotalName, Value: tc.expectedMetricValue, Labels: map[string]string{}},
			}

			if diff := exporter.ValidateMetrics(expected); diff != "" {
				t.Errorf("Unexpected metrics recorded: %v", diff)
			}
		})
	}
}

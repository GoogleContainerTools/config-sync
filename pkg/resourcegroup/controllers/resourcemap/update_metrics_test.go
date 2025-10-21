/*
Copyright 2020 Google LLC.

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
	// Create a single resource map for the entire test
	m := NewResourceMap()
	ctx := context.Background()

	// Test 1: Add single resource group
	t.Run("Add single resource group", func(t *testing.T) {
		// Register metrics views with test exporter for this test
		exporter := testmetrics.RegisterMetrics(
			metrics.ResourceGroupTotalView,
		)

		group := types.NamespacedName{Name: "root-sync", Namespace: configsync.ControllerNamespace}
		resources := []v1alpha1.ObjMetadata{
			{
				Name:      "test-deployment",
				Namespace: "default",
				GroupKind: v1alpha1.GroupKind{
					Group: "apps",
					Kind:  "Deployment",
				},
			},
		}

		_ = m.Reconcile(ctx, group, resources, false)

		expected := []*view.Row{
			{Data: &view.LastValueData{Value: 1}, Tags: []tag.Tag{}},
		}

		if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
			t.Errorf("Unexpected metrics recorded: %v", diff)
		}

		if len(m.resgroupToResources) != 1 {
			t.Errorf("Expected 1 resource groups in map, got %d", len(m.resgroupToResources))
		}
	})

	// Test 2: Add second resource group
	t.Run("Add second resource group", func(t *testing.T) {
		// Register metrics views with test exporter for this test
		exporter := testmetrics.RegisterMetrics(
			metrics.ResourceGroupTotalView,
		)

		group2 := types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"}
		resources2 := []v1alpha1.ObjMetadata{
			{
				Name:      "test-service",
				Namespace: "bookinfo",
				GroupKind: v1alpha1.GroupKind{
					Group: "",
					Kind:  "Service",
				},
			},
		}

		_ = m.Reconcile(ctx, group2, resources2, false)

		expected := []*view.Row{
			{Data: &view.LastValueData{Value: 2}, Tags: []tag.Tag{}}, // root-sync + repo-sync
		}

		if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
			t.Errorf("Unexpected metrics recorded: %v", diff)
		}

		if len(m.resgroupToResources) != 2 {
			t.Errorf("Expected 2 resource groups in map, got %d", len(m.resgroupToResources))
		}
	})

	// Test 3: Delete first resource group
	t.Run("Delete first resource group", func(t *testing.T) {
		// Register metrics views with test exporter for this test
		exporter := testmetrics.RegisterMetrics(
			metrics.ResourceGroupTotalView,
		)

		group := types.NamespacedName{Name: "root-sync", Namespace: configsync.ControllerNamespace}
		_ = m.Reconcile(ctx, group, []v1alpha1.ObjMetadata{}, true)

		expected := []*view.Row{
			{Data: &view.LastValueData{Value: 1}, Tags: []tag.Tag{}}, // Only repo-sync remains
		}

		if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
			t.Errorf("Unexpected metrics recorded: %v", diff)
		}

		if len(m.resgroupToResources) != 1 {
			t.Errorf("Expected 1 resource groups in map, got %d", len(m.resgroupToResources))
		}
	})

	// Test 4: Delete remaining resource group
	t.Run("Delete remaining resource group", func(t *testing.T) {
		// Register metrics views with test exporter for this test
		exporter := testmetrics.RegisterMetrics(
			metrics.ResourceGroupTotalView,
		)

		group2 := types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"}
		_ = m.Reconcile(ctx, group2, []v1alpha1.ObjMetadata{}, true)

		expected := []*view.Row{
			{Data: &view.LastValueData{Value: 0}, Tags: []tag.Tag{}}, // No resource groups remain
		}

		if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
			t.Errorf("Unexpected metrics recorded: %v", diff)
		}

		if len(m.resgroupToResources) != 0 {
			t.Errorf("Expected 0 resource groups in map, got %d", len(m.resgroupToResources))
		}
	})

	// Test 5: Add resource group with multiple resources
	t.Run("Add resource group with multiple resources", func(t *testing.T) {
		// Register metrics views with test exporter for this test
		exporter := testmetrics.RegisterMetrics(
			metrics.ResourceGroupTotalView,
		)

		group2 := types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"}
		resources3 := []v1alpha1.ObjMetadata{
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
		}

		_ = m.Reconcile(ctx, group2, resources3, false)

		expected := []*view.Row{
			{Data: &view.LastValueData{Value: 1}, Tags: []tag.Tag{}}, // 1 resource group with multiple resources
		}

		if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
			t.Errorf("Unexpected metrics recorded: %v", diff)
		}

		if len(m.resgroupToResources) != 1 {
			t.Errorf("Expected 1 resource groups in map, got %d", len(m.resgroupToResources))
		}
	})
}

func TestResourceMapMultipleUpdates(t *testing.T) {
	// Register metrics views with test exporter
	exporter := testmetrics.RegisterMetrics(
		metrics.ResourceGroupTotalView,
	)

	// Create a new resource map
	m := NewResourceMap()
	ctx := context.Background()

	// Add first resource group
	group1 := types.NamespacedName{Name: "root-sync", Namespace: configsync.ControllerNamespace}
	resources1 := []v1alpha1.ObjMetadata{
		{
			Name:      "deployment-1",
			Namespace: "default",
			GroupKind: v1alpha1.GroupKind{
				Group: "apps",
				Kind:  "Deployment",
			},
		},
	}
	_ = m.Reconcile(ctx, group1, resources1, false)

	// Add second resource group
	group2 := types.NamespacedName{Name: "repo-sync", Namespace: "bookinfo"}
	resources2 := []v1alpha1.ObjMetadata{
		{
			Name:      "service-1",
			Namespace: "bookinfo",
			GroupKind: v1alpha1.GroupKind{
				Group: "",
				Kind:  "Service",
			},
		},
	}
	_ = m.Reconcile(ctx, group2, resources2, false)

	// Add third resource group
	group3 := types.NamespacedName{Name: "another-sync", Namespace: "default"}
	resources3 := []v1alpha1.ObjMetadata{
		{
			Name:      "role-1",
			Namespace: "default",
			GroupKind: v1alpha1.GroupKind{
				Group: "rbac.authorization.k8s.io",
				Kind:  "Role",
			},
		},
	}
	_ = m.Reconcile(ctx, group3, resources3, false)

	// Verify final metrics
	expected := []*view.Row{
		{Data: &view.LastValueData{Value: 3}, Tags: []tag.Tag{}}, // All three resource groups
	}

	if diff := exporter.ValidateMetrics(metrics.ResourceGroupTotalView, expected); diff != "" {
		t.Errorf("Unexpected metrics recorded: %v", diff)
	}

	// Verify the resource map state
	if len(m.resgroupToResources) != 3 {
		t.Errorf("Expected 3 resource groups in map, got %d", len(m.resgroupToResources))
	}
}

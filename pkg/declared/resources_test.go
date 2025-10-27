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

package declared

import (
	"context"
	"sort"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/metrics"
	"github.com/GoogleContainerTools/config-sync/pkg/syncer/reconcile"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/testmetrics"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	obj1 = k8sobjects.CustomResourceDefinitionV1Beta1Object()
	obj2 = k8sobjects.ResourceQuotaObject()

	testSet = []client.Object{obj1, obj2}
	nilSet  = []client.Object{nil}
)

func TestUpdateDeclared(t *testing.T) {
	dr := Resources{}
	objects := testSet
	commit := "1"
	expectedIDs := getIDs(objects)

	newObjects, err := dr.UpdateDeclared(context.Background(), objects, commit)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	for _, id := range expectedIDs {
		if _, ok := dr.declaredObjectsMap.Get(id); !ok {
			t.Errorf("ID %v not found in the declared resource", id)
		}
	}

	gotIDs := getIDs(newObjects)
	if diff := cmp.Diff(expectedIDs, gotIDs); diff != "" {
		t.Error(diff)
	}

	require.Equal(t, commit, dr.commit)
}

func TestMutateImpossible(t *testing.T) {
	wantResourceVersion := "version 1"

	dr := Resources{}
	o1 := k8sobjects.RoleObject(core.Name("foo"), core.Namespace("bar"))
	o1.SetResourceVersion(wantResourceVersion)
	o2 := asUnstructured(t, k8sobjects.RoleObject(core.Name("baz"), core.Namespace("bar")))
	o2.SetResourceVersion(wantResourceVersion)

	expectedCommit := "example"
	_, err := dr.UpdateDeclared(context.Background(), []client.Object{o1, o2}, expectedCommit)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the original resources and ensure the stored resources are preserved.
	o1.SetResourceVersion("version 1++")
	o2.SetResourceVersion("version 1++")

	got1, commit, found := dr.GetDeclared(core.IDOf(o1))
	require.Equal(t, expectedCommit, commit)
	if !found {
		t.Fatalf("got dr.Get = %v, %t, want dr.Get = obj, true", got1, found)
	}
	if diff := cmp.Diff(wantResourceVersion, got1.GetResourceVersion()); diff != "" {
		t.Error(diff)
	}
	got2, commit, found := dr.GetDeclared(core.IDOf(o2))
	require.Equal(t, expectedCommit, commit)
	if !found {
		t.Fatalf("got dr.Get = %v, %t, want dr.Get = obj, true", got2, found)
	}
	if diff := cmp.Diff(wantResourceVersion, got2.GetResourceVersion()); diff != "" {
		t.Error(diff)
	}

	// Modify the fetched resource and ensure the stored resource is preserved.
	got1.SetResourceVersion("version 2")
	got2.SetResourceVersion("version 2")

	got3, commit, found := dr.GetDeclared(core.IDOf(o1))
	require.Equal(t, expectedCommit, commit)
	if !found {
		t.Fatalf("got dr.Get = %v, %t, want dr.Get = obj, true", got3, found)
	}
	if diff := cmp.Diff(wantResourceVersion, got3.GetResourceVersion()); diff != "" {
		t.Error(diff)
	}
	got4, commit, found := dr.GetDeclared(core.IDOf(o2))
	require.Equal(t, expectedCommit, commit)
	if !found {
		t.Fatalf("got dr.Get = %v, %t, want dr.Get = obj, true", got4, found)
	}
	if diff := cmp.Diff(wantResourceVersion, got4.GetResourceVersion()); diff != "" {
		t.Error(diff)
	}
}

func asUnstructured(t *testing.T, o client.Object) *unstructured.Unstructured {
	t.Helper()
	u, err := reconcile.AsUnstructuredSanitized(o)
	if err != nil {
		t.Fatal("converting to unstructured", err)
	}
	return u
}

func TestDeclarations(t *testing.T) {
	dr := Resources{}
	expectedCommit := "example"
	objects, err := dr.UpdateDeclared(context.Background(), testSet, expectedCommit)
	if err != nil {
		t.Fatal(err)
	}

	got := dr.DeclaredUnstructureds()

	// Sort got decls to ensure determinism.
	sort.Slice(got, func(i, j int) bool {
		return core.IDOf(got[i]).String() < core.IDOf(got[j]).String()
	})

	want := []*unstructured.Unstructured{
		asUnstructured(t, obj1),
		asUnstructured(t, obj2),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}

	gotIDs := getIDs(objects)
	wantIDs := []core.ID{}
	for _, obj := range got {
		wantIDs = append(wantIDs, core.IDOf(obj))
	}
	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Error(diff)
	}
}

func TestGetDeclared(t *testing.T) {
	dr := Resources{}
	expectedCommit := "example"
	_, err := dr.UpdateDeclared(context.Background(), testSet, expectedCommit)
	if err != nil {
		t.Fatal(err)
	}

	actual, commit, found := dr.GetDeclared(core.IDOf(obj1))
	require.Equal(t, expectedCommit, commit)
	if !found {
		t.Fatal("got not found, want found")
	}
	if diff := cmp.Diff(asUnstructured(t, obj1), actual); diff != "" {
		t.Error(diff)
	}
}

func TestGVKSet(t *testing.T) {
	dr := Resources{}
	expectedCommit := "example"
	_, err := dr.UpdateDeclared(context.Background(), testSet, expectedCommit)
	if err != nil {
		t.Fatal(err)
	}

	got, commit := dr.DeclaredGVKs()
	require.Equal(t, expectedCommit, commit)
	want := map[schema.GroupVersionKind]struct{}{
		obj1.GroupVersionKind(): {},
		obj2.GroupVersionKind(): {},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestResources_InternalErrorMetricValidation(t *testing.T) {
	// Initialize metrics for this test
	exporter, err := testmetrics.NewTestExporter()
	if err != nil {
		t.Fatalf("Failed to create test exporter: %v", err)
	}
	defer exporter.ClearMetrics()
	dr := Resources{}
	if _, err := dr.UpdateDeclared(context.Background(), nilSet, "unused"); err != nil {
		t.Fatal(err)
	}

	expectedMetrics := []testmetrics.MetricData{
		{
			Name:   metrics.InternalErrorsName,
			Value:  1,
			Labels: map[string]string{"source": "parser"},
		},
	}

	if diff := exporter.ValidateMetrics(expectedMetrics); diff != "" {
		t.Error(diff)
	}
}

func getIDs(objects []client.Object) []core.ID {
	var IDs []core.ID
	for _, obj := range objects {
		IDs = append(IDs, core.IDOf(obj))
	}
	return IDs
}

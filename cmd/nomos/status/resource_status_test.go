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

package status

import (
	"sort"
	"testing"

	kptv1alpha1 "github.com/GoogleContainerTools/config-sync/pkg/api/kpt.dev/v1alpha1"
	"github.com/google/go-cmp/cmp"
)

func TestResourceStatus(t *testing.T) {
	resources := []kptv1alpha1.ResourceStatus{
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "apps",
					Kind:  "Deployment",
				},
			},
			Status: kptv1alpha1.Current,
		},
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "bookstore",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "",
					Kind:  "Service",
				},
			},
			Status: kptv1alpha1.Current,
		},
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "gamestore",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "",
					Kind:  "Service",
				},
			},
			Status: kptv1alpha1.Current,
		},
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "rbac.authorization.k8s.io",
					Kind:  "ClusterRole",
				},
			},
			Status: kptv1alpha1.Current,
		},
	}
	sort.Sort(byNamespaceAndType(resources))

	expected := []kptv1alpha1.ResourceStatus{
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "rbac.authorization.k8s.io",
					Kind:  "ClusterRole",
				},
			},
			Status: kptv1alpha1.Current,
		},
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "apps",
					Kind:  "Deployment",
				},
			},
			Status: kptv1alpha1.Current,
		},
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "bookstore",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "",
					Kind:  "Service",
				},
			},
			Status: kptv1alpha1.Current,
		},
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "test",
				Namespace: "gamestore",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "",
					Kind:  "Service",
				},
			},
			Status: kptv1alpha1.Current,
		},
	}
	if diff := cmp.Diff(expected, resources); diff != "" {
		t.Error(diff)
	}
}

func TestResourceLevelStatus(t *testing.T) {
	rg := &kptv1alpha1.ResourceGroup{
		Status: kptv1alpha1.ResourceGroupStatus{
			ResourceStatuses: []kptv1alpha1.ResourceStatus{
				{
					ObjMetadata: kptv1alpha1.ObjMetadata{
						Name:      "example",
						Namespace: "",
						GroupKind: kptv1alpha1.GroupKind{
							Group: "",
							Kind:  "ConfigMap",
						},
					},
					Status:    kptv1alpha1.Current,
					Strategy:  kptv1alpha1.Apply,
					Actuation: kptv1alpha1.ActuationSucceeded,
					Reconcile: kptv1alpha1.ReconcileSucceeded,
				},
				{
					ObjMetadata: kptv1alpha1.ObjMetadata{
						Name:      "example-2",
						Namespace: "",
						GroupKind: kptv1alpha1.GroupKind{
							Group: "",
							Kind:  "ConfigMap",
						},
					},
					Status:    kptv1alpha1.Current,
					Strategy:  kptv1alpha1.Apply,
					Actuation: kptv1alpha1.ActuationSucceeded,
					Reconcile: kptv1alpha1.ReconcileSucceeded,
					Conditions: []kptv1alpha1.Condition{
						{
							Type:   kptv1alpha1.Ownership,
							Status: kptv1alpha1.TrueConditionStatus,
						},
					},
				},
			},
		},
	}

	want := []kptv1alpha1.ResourceStatus{
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "example",
				Namespace: "",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "",
					Kind:  "ConfigMap",
				},
			},
			Status:    kptv1alpha1.Current,
			Strategy:  kptv1alpha1.Apply,
			Actuation: kptv1alpha1.ActuationSucceeded,
			Reconcile: kptv1alpha1.ReconcileSucceeded,
		},
		{
			ObjMetadata: kptv1alpha1.ObjMetadata{
				Name:      "example-2",
				Namespace: "",
				GroupKind: kptv1alpha1.GroupKind{
					Group: "",
					Kind:  "ConfigMap",
				},
			},
			Status:    "Conflict",
			Strategy:  kptv1alpha1.Apply,
			Actuation: kptv1alpha1.ActuationSucceeded,
			Reconcile: kptv1alpha1.ReconcileSucceeded,
			Conditions: []kptv1alpha1.Condition{
				{
					Type:   kptv1alpha1.Ownership,
					Status: kptv1alpha1.TrueConditionStatus,
				},
			},
		}}

	got := resourceLevelStatus(rg)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

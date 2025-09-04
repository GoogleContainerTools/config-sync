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

package validate

import (
	"errors"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/nonhierarchical"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
)

func TestIllegalKindsForHierarchical(t *testing.T) {
	testCases := []struct {
		name    string
		obj     ast.FileObject
		wantErr status.Error
	}{
		{
			name: "Non-hierarchical object passes",
			obj:  k8sobjects.ClusterSelector(),
		},
		{
			name: "Hiearchical object passes",
			obj:  k8sobjects.HierarchyConfig(),
		},
		{
			name:    "Sync object fails",
			obj:     k8sobjects.FileObject(k8sobjects.SyncObject(kinds.Role().GroupKind()), "sync.yaml"),
			wantErr: nonhierarchical.UnsupportedObjectError(k8sobjects.SyncObject(kinds.Role().GroupKind())),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := IllegalKindsForHierarchical(tc.obj)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got IllegalKindsForHierarchical() error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestIllegalKindsForUnstructured(t *testing.T) {
	testCases := []struct {
		name    string
		obj     ast.FileObject
		wantErr status.Error
	}{
		{
			name: "Non-hierarchical object passes",
			obj:  k8sobjects.ClusterSelector(),
		},
		{
			name:    "HierarchyConfig object fails",
			obj:     k8sobjects.HierarchyConfig(),
			wantErr: nonhierarchical.IllegalHierarchicalKind(k8sobjects.HierarchyConfig()),
		},
		{
			name:    "Repo object fails",
			obj:     k8sobjects.Repo(),
			wantErr: nonhierarchical.IllegalHierarchicalKind(k8sobjects.Repo()),
		},
		{
			name:    "Sync object fails",
			obj:     k8sobjects.FileObject(k8sobjects.SyncObject(kinds.Role().GroupKind()), "sync.yaml"),
			wantErr: nonhierarchical.UnsupportedObjectError(k8sobjects.SyncObject(kinds.Role().GroupKind())),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := IllegalKindsForUnstructured(tc.obj)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got IllegalKindsForUnstructured() error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

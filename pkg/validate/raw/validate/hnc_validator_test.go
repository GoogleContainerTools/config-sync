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

	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/hnc"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
)

const (
	illegalSuffixedLabel  = "unsupported" + metadata.DepthSuffix
	illegalSuffixedLabel2 = "unsupported2" + metadata.DepthSuffix
)

func TestHNCLabels(t *testing.T) {
	testCases := []struct {
		name    string
		obj     ast.FileObject
		wantErr status.Error
	}{
		{
			name: "no labels",
			obj:  k8sobjects.RoleAtPath("namespaces/hello/role.yaml"),
		},
		{
			name: "one legal label",
			obj: k8sobjects.RoleAtPath("namespaces/hello/role.yaml",
				core.Label(legalLabel, "")),
		},
		{
			name: "one illegal label",
			obj: k8sobjects.RoleAtPath("namespaces/hello/role.yaml",
				core.Label(illegalSuffixedLabel, "")),
			wantErr: hnc.IllegalDepthLabelError(k8sobjects.Role(), []string{illegalSuffixedLabel}),
		},
		{
			name: "two illegal labels",
			obj: k8sobjects.RoleAtPath("namespaces/hello/role.yaml",
				core.Label(illegalSuffixedLabel, ""),
				core.Label(illegalSuffixedLabel2, "")),
			wantErr: hnc.IllegalDepthLabelError(k8sobjects.Role(), []string{illegalSuffixedLabel, illegalSuffixedLabel2}),
		},
		{
			name: "one legal and one illegal label",
			obj: k8sobjects.RoleAtPath("namespaces/hello/role.yaml",
				core.Label(legalLabel, ""),
				core.Label(illegalSuffixedLabel, "")),
			wantErr: hnc.IllegalDepthLabelError(k8sobjects.Role(), []string{illegalSuffixedLabel}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := HNCLabels(tc.obj)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got HNCLabels() error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

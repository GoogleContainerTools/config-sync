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
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/nonhierarchical"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/syncer/syncertest"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/testerrors"
)

func TestManagementAnnotation(t *testing.T) {
	testCases := []struct {
		name string
		obj  ast.FileObject
		want status.Error
	}{
		{
			name: "no management annotation passes",
			obj:  k8sobjects.Role(),
		},
		{
			name: "disabled management passes",
			obj:  k8sobjects.Role(syncertest.ManagementDisabled),
		},
		{
			name: "enabled management fails",
			obj:  k8sobjects.Role(syncertest.ManagementEnabled),
			want: nonhierarchical.IllegalManagementAnnotationError(
				k8sobjects.Role(), metadata.ManagementEnabled.String()),
		},
		{
			name: "invalid management fails",
			obj:  k8sobjects.Role(syncertest.ManagementInvalid),
			want: nonhierarchical.IllegalManagementAnnotationError(
				k8sobjects.Role(), "invalid"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ManagementAnnotation(tc.obj)
			testerrors.AssertEqual(t, tc.want, err)
		})
	}
}

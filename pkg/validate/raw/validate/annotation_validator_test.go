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

	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/metadata"
	csmetadata "github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
)

const (
	legalAnnotation = "supported"
	cmAnnotation    = csmetadata.ConfigManagementPrefix + "unsupported"
	csAnnotation    = configsync.ConfigSyncPrefix + "unsupported"
)

func TestAnnotations(t *testing.T) {
	testCases := []struct {
		name    string
		obj     ast.FileObject
		wantErr status.MultiError
	}{
		{
			name: "no annotations",
			obj:  k8sobjects.Role(),
		},
		{
			name: "legal annotation",
			obj:  k8sobjects.Role(core.Annotation(legalAnnotation, "a")),
		},
		{
			name: "legal namespace selector annotation",
			obj:  k8sobjects.Role(core.Annotation(csmetadata.NamespaceSelectorAnnotationKey, "a")),
		},
		{
			name: "legal legacy cluster selector annotation",
			obj:  k8sobjects.Role(core.Annotation(csmetadata.LegacyClusterSelectorAnnotationKey, "a")),
		},
		{
			name: "legal inline cluster selector annotation",
			obj:  k8sobjects.Role(core.Annotation(csmetadata.ClusterNameSelectorAnnotationKey, "a")),
		},
		{
			name: "legal management annotation",
			obj:  k8sobjects.RoleBinding(core.Annotation(csmetadata.ManagementModeAnnotationKey, "a")),
		},
		{
			name:    "illegal ConfigManagement annotation",
			obj:     k8sobjects.Role(core.Annotation(cmAnnotation, "a")),
			wantErr: metadata.IllegalAnnotationDefinitionError(k8sobjects.Role(), []string{cmAnnotation}),
		},
		{
			name:    "illegal ConfigSync annotation",
			obj:     k8sobjects.RoleBinding(core.Annotation(csAnnotation, "a")),
			wantErr: metadata.IllegalAnnotationDefinitionError(k8sobjects.RoleBinding(), []string{csAnnotation}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Annotations(tc.obj)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got Annotations() error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

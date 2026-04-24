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
	legalLabel = "supported"
	cmLabel    = csmetadata.ConfigManagementPrefix + "unsupported"
	csLabel    = configsync.ConfigSyncPrefix + "unsupported2"
)

func TestLabels(t *testing.T) {
	testCases := []struct {
		name    string
		obj     ast.FileObject
		wantErr status.MultiError
	}{
		{
			name: "no labels",
			obj:  k8sobjects.Role(),
		},
		{
			name: "legal label",
			obj:  k8sobjects.Role(core.Label(legalLabel, "a")),
		},
		{
			name:    "illegal ConfigManagement label",
			obj:     k8sobjects.Role(core.Label(cmLabel, "a")),
			wantErr: metadata.IllegalLabelDefinitionError(k8sobjects.Role(), []string{cmLabel}),
		},
		{
			name:    "illegal ConfigSync label",
			obj:     k8sobjects.RoleBinding(core.Label(csLabel, "a")),
			wantErr: metadata.IllegalLabelDefinitionError(k8sobjects.RoleBinding(), []string{csLabel}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Labels(tc.obj)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got Labels() error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

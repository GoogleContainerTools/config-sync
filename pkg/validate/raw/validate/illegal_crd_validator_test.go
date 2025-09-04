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
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/nonhierarchical"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestIllegalCRD(t *testing.T) {
	testCases := []struct {
		name    string
		obj     ast.FileObject
		wantErr status.Error
	}{
		{
			name: "Anvil v1beta1 CRD",
			obj: k8sobjects.FileObject(k8sobjects.CRDV1UnstructuredForGVK(
				kinds.Anvil(), apiextensionsv1.NamespaceScoped, core.Name("crd")),
				"crd.yaml"),
		},
		{
			name: "ClusterConfig v1beta1 CRD",
			obj: k8sobjects.FileObject(k8sobjects.CRDV1beta1UnstructuredForGVK(
				kinds.ClusterConfig(), apiextensionsv1.ClusterScoped, core.Name("crd")),
				"crd.yaml"),
			wantErr: status.FakeError(nonhierarchical.UnsupportedObjectErrorCode),
		},
		{
			name: "ClusterConfig v1 CRD",
			obj: k8sobjects.FileObject(k8sobjects.CRDV1UnstructuredForGVK(
				kinds.ClusterConfig(), apiextensionsv1.ClusterScoped, core.Name("crd")),
				"crd.yaml"),
			wantErr: status.FakeError(nonhierarchical.UnsupportedObjectErrorCode),
		},
		{
			name: "RepoSync v1beta1 CRD",
			obj: k8sobjects.FileObject(k8sobjects.CRDV1beta1UnstructuredForGVK(
				kinds.RepoSyncV1Beta1(), apiextensionsv1.NamespaceScoped, core.Name("crd")),
				"crd.yaml"),
			wantErr: status.FakeError(nonhierarchical.UnsupportedObjectErrorCode),
		},
		{
			name: "RepoSync v1 CRD",
			obj: k8sobjects.FileObject(k8sobjects.CRDV1UnstructuredForGVK(
				kinds.RepoSyncV1Beta1(), apiextensionsv1.NamespaceScoped, core.Name("crd")),
				"crd.yaml"),
			wantErr: status.FakeError(nonhierarchical.UnsupportedObjectErrorCode),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := IllegalCRD(tc.obj)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got IllegalCRD() error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

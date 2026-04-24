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
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast/node"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/transform/selectors"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/syntax"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/filesystem/cmpath"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/validate/fileobjects"
)

func TestNamespaceSelector(t *testing.T) {
	testCases := []struct {
		name     string
		objs     *fileobjects.Tree
		wantErrs status.MultiError
	}{
		{
			name: "NamespaceSelector in abstract namespace",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"dev": k8sobjects.NamespaceSelectorAtPath("namespaces/sel.yaml",
						core.Name("dev")),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/hello"),
							Type:     node.Namespace,
							Objects: []ast.FileObject{
								k8sobjects.Namespace("namespaces/hello"),
							},
						},
					},
				},
			},
		},
		{
			name: "NamespaceSelector in Namespace",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"dev": k8sobjects.NamespaceSelectorAtPath("namespaces/hello/sel.yaml",
						core.Name("dev")),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/hello"),
							Type:     node.Namespace,
							Objects: []ast.FileObject{
								k8sobjects.Namespace("namespaces/hello"),
							},
						},
					},
				},
			},
			wantErrs: status.FakeMultiError(syntax.IllegalKindInNamespacesErrorCode),
		},
		{
			name: "Object references ancestor NamespaceSelector",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"dev": k8sobjects.NamespaceSelectorAtPath("namespaces/hello/sel.yaml",
						core.Name("dev")),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/hello"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/hello/world"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/hello/world"),
										k8sobjects.RoleAtPath("namespaces/hello/world/role.yaml",
											core.Annotation(metadata.NamespaceSelectorAnnotationKey, "dev")),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Object references non-ancestor NamespaceSelector",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"dev": k8sobjects.NamespaceSelectorAtPath("namespaces/goodbye/sel.yaml",
						core.Name("dev")),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/hello"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/hello/world"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/hello/world"),
										k8sobjects.RoleAtPath("namespaces/hello/world/role.yaml",
											core.Annotation(metadata.NamespaceSelectorAnnotationKey, "dev")),
									},
								},
							},
						},
						{
							Relative: cmpath.RelativeSlash("namespaces/goodbye"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/goodbye/moon"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/goodbye/moon"),
										k8sobjects.RoleAtPath("namespaces/goodbye/moon/role.yaml"),
									},
								},
							},
						},
					},
				},
			},
			wantErrs: status.FakeMultiError(selectors.ObjectHasUnknownSelectorCode),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := NamespaceSelector(tc.objs)
			if !errors.Is(errs, tc.wantErrs) {
				t.Errorf("got NamespaceSelector() error %v, want %v", errs, tc.wantErrs)
			}
		})
	}
}

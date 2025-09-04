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

package hydrate

import (
	"errors"
	"testing"

	v1 "github.com/GoogleContainerTools/config-sync/pkg/api/configmanagement/v1"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast/node"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/transform/selectors"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/filesystem/cmpath"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/validate/fileobjects"
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func nsSelector(nssName, mode string) ast.FileObject {
	mutFunc := func(o client.Object) {
		nss := o.(*v1.NamespaceSelector)
		nss.Spec.Selector.MatchLabels = map[string]string{
			"sre-support": "true",
		}
		nss.Spec.Mode = mode
	}
	return k8sobjects.NamespaceSelectorAtPath("namespaces/foo/selector.yaml",
		core.Name(nssName), mutFunc)
}

func TestNamespaceSelectors(t *testing.T) {
	testCases := []struct {
		name     string
		objs     *fileobjects.Tree
		want     *fileobjects.Tree
		wantErrs status.MultiError
	}{
		{
			name: "Object without selector is kept",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend",
											core.Label("sre-support", "false")),
										k8sobjects.RoleAtPath("namespaces/foo/role.yaml",
											core.Namespace("frontend")),
									},
								},
							},
						},
					},
				},
			},
			want: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend",
											core.Label("sre-support", "false")),
										k8sobjects.RoleAtPath("namespaces/foo/role.yaml",
											core.Namespace("frontend")),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Object outside selector dir is kept",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend",
											core.Label("sre-support", "false")),
									},
								},
							},
						},
						{
							Relative: cmpath.RelativeSlash("namespaces/bar"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/bar/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/bar/frontend"),
										k8sobjects.RoleAtPath("namespaces/bar/role.yaml",
											core.Namespace("bar")),
									},
								},
							},
						},
					},
				},
			},
			want: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend",
											core.Label("sre-support", "false")),
									},
								},
							},
						},
						{
							Relative: cmpath.RelativeSlash("namespaces/bar"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/bar/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/bar/frontend"),
										k8sobjects.RoleAtPath("namespaces/bar/role.yaml",
											core.Namespace("bar")),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Object and Namespace with labels is kept",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend",
											core.Label("sre-support", "true")),
										k8sobjects.RoleAtPath("namespaces/foo/role.yaml",
											core.Namespace("frontend"),
											core.Annotation(metadata.NamespaceSelectorAnnotationKey, "sre")),
									},
								},
							},
						},
					},
				},
			},
			want: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend",
											core.Label("sre-support", "true")),
										k8sobjects.RoleAtPath("namespaces/foo/role.yaml",
											core.Namespace("frontend"),
											core.Annotation(metadata.NamespaceSelectorAnnotationKey, "sre")),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Object with selector and Namespace without labels is not kept",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend"),
										k8sobjects.RoleAtPath("namespaces/foo/role.yaml",
											core.Namespace("frontend"),
											core.Annotation(metadata.NamespaceSelectorAnnotationKey, "sre")),
									},
								},
							},
						},
					},
				},
			},
			want: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorStaticMode),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/foo"),
							Type:     node.AbstractNamespace,
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/foo/frontend"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/foo/frontend"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Use dynamic mode in hierarchy mode",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", v1.NSSelectorDynamicMode),
				},
			},
			wantErrs: selectors.UnsupportedNamespaceSelectorModeError(nsSelector("sre", v1.NSSelectorDynamicMode)),
		},
		{
			name: "Use unknown mode in hierarchy mode",
			objs: &fileobjects.Tree{
				NamespaceSelectors: map[string]ast.FileObject{
					"sre": nsSelector("sre", "unknown"),
				},
			},
			wantErrs: selectors.UnknownNamespaceSelectorModeError(nsSelector("sre", "unknown")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := NamespaceSelectors(tc.objs)
			if !errors.Is(errs, tc.wantErrs) {
				t.Errorf("Got NamespaceSelectors() error %v, want %v", errs, tc.wantErrs)
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, tc.objs, ast.CompareFileObject); diff != "" {
					t.Error(diff)
				}
			}
		})
	}
}

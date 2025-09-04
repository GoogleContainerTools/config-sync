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

package fileobjects

import (
	"errors"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast/node"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/filesystem/cmpath"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/google/go-cmp/cmp"
)

func TestBuildTree(t *testing.T) {
	testCases := []struct {
		name     string
		from     *Scoped
		want     *Tree
		wantErrs status.MultiError
	}{
		{
			name: "almost-empty tree",
			from: &Scoped{
				Cluster: []ast.FileObject{
					k8sobjects.Repo(),
				},
			},
			want: &Tree{
				Repo:               k8sobjects.Repo(),
				NamespaceSelectors: map[string]ast.FileObject{},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash(""),
					Type:     node.AbstractNamespace,
				},
			},
		},
		{
			name: "populated tree",
			from: &Scoped{
				Cluster: []ast.FileObject{
					k8sobjects.Repo(),
					k8sobjects.HierarchyConfig(),
					k8sobjects.ClusterRole(core.Name("hello-reader")),
					k8sobjects.Namespace("namespaces/hello/world"),
					k8sobjects.Namespace("namespaces/hello/moon"),
					k8sobjects.NamespaceSelectorAtPath("namespaces/selector.yaml"),
				},
				Namespace: []ast.FileObject{
					k8sobjects.RoleAtPath("namespaces/hello/role.yaml", core.Name("writer")),
				},
				Unknown: []ast.FileObject{
					k8sobjects.AnvilAtPath("namespaces/hello/world/anvil.yaml"),
				},
			},
			want: &Tree{
				Repo: k8sobjects.Repo(),
				HierarchyConfigs: []ast.FileObject{
					k8sobjects.HierarchyConfig(),
				},
				NamespaceSelectors: map[string]ast.FileObject{
					"default-name": k8sobjects.NamespaceSelectorAtPath("namespaces/selector.yaml"),
				},
				Cluster: []ast.FileObject{
					k8sobjects.ClusterRole(core.Name("hello-reader")),
				},
				Tree: &ast.TreeNode{
					Relative: cmpath.RelativeSlash("namespaces"),
					Type:     node.AbstractNamespace,
					Children: []*ast.TreeNode{
						{
							Relative: cmpath.RelativeSlash("namespaces/hello"),
							Type:     node.AbstractNamespace,
							Objects: []ast.FileObject{
								k8sobjects.RoleAtPath("namespaces/hello/role.yaml", core.Name("writer")),
							},
							Children: []*ast.TreeNode{
								{
									Relative: cmpath.RelativeSlash("namespaces/hello/moon"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/hello/moon"),
									},
								},
								{
									Relative: cmpath.RelativeSlash("namespaces/hello/world"),
									Type:     node.Namespace,
									Objects: []ast.FileObject{
										k8sobjects.Namespace("namespaces/hello/world"),
										k8sobjects.AnvilAtPath("namespaces/hello/world/anvil.yaml"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "cluster-scoped resource in wrong directory",
			from: &Scoped{
				Cluster: []ast.FileObject{
					k8sobjects.Repo(),
					k8sobjects.ClusterRoleAtPath("namespaces/hello/cr.yaml", core.Name("hello-reader")),
					k8sobjects.Namespace("namespaces/hello"),
				},
				Namespace: []ast.FileObject{
					k8sobjects.RoleAtPath("namespaces/hello/role.yaml", core.Name("writer")),
				},
			},
			want:     nil,
			wantErrs: validation.ShouldBeInClusterError(k8sobjects.ClusterRole()),
		},
		{
			name: "namespace-scoped resource in wrong directory",
			from: &Scoped{
				Cluster: []ast.FileObject{
					k8sobjects.Repo(),
					k8sobjects.Namespace("namespaces/hello"),
				},
				Namespace: []ast.FileObject{
					k8sobjects.RoleAtPath("cluster/role.yaml", core.Name("writer")),
				},
			},
			want:     nil,
			wantErrs: validation.ShouldBeInNamespacesError(k8sobjects.Role()),
		},
		{
			name: "system resource in wrong directory",
			from: &Scoped{
				Cluster: []ast.FileObject{
					k8sobjects.Repo(),
					k8sobjects.HierarchyConfigAtPath("cluster/hc.yaml"),
					k8sobjects.Namespace("namespaces/hello"),
				},
			},
			want:     nil,
			wantErrs: validation.ShouldBeInSystemError(k8sobjects.HierarchyConfig()),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, errs := BuildTree(tc.from)
			if !errors.Is(errs, tc.wantErrs) {
				t.Errorf("Got BuildTree() error %v, want %v", errs, tc.wantErrs)
			}
			if diff := cmp.Diff(tc.want, got, ast.CompareFileObject); diff != "" {
				t.Error(diff)
			}
		})
	}
}

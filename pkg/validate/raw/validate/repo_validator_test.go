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
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/system"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/util/repo"
	"github.com/GoogleContainerTools/config-sync/pkg/validate/fileobjects"
)

const notAllowedRepoVersion = "0.0.0"

func TestRepo(t *testing.T) {
	testCases := []struct {
		name    string
		objs    *fileobjects.Raw
		wantErr status.Error
	}{
		{
			name: "Repo with current version",
			objs: &fileobjects.Raw{
				Objects: []ast.FileObject{
					k8sobjects.Repo(k8sobjects.RepoVersion(repo.CurrentVersion)),
				},
			},
		},
		{
			name: "Repo with supported old version",
			objs: &fileobjects.Raw{
				Objects: []ast.FileObject{
					k8sobjects.Repo(k8sobjects.RepoVersion(system.OldAllowedRepoVersion)),
				},
			},
		},
		{
			name: "Repo with unsupported old version",
			objs: &fileobjects.Raw{
				Objects: []ast.FileObject{
					k8sobjects.Repo(k8sobjects.RepoVersion(notAllowedRepoVersion)),
				},
			},
			wantErr: system.UnsupportedRepoSpecVersion(k8sobjects.Repo(k8sobjects.RepoVersion(notAllowedRepoVersion)), notAllowedRepoVersion),
		},
		{
			name: "Missing Repo",
			objs: &fileobjects.Raw{
				Objects: []ast.FileObject{
					k8sobjects.Role(),
					k8sobjects.RoleBinding(),
				},
			},
			wantErr: system.MissingRepoError(),
		},
		{
			name: "Multiple Repos",
			objs: &fileobjects.Raw{
				Objects: []ast.FileObject{
					k8sobjects.Repo(core.Name("first")),
					k8sobjects.Repo(core.Name("second")),
				},
			},
			wantErr: status.MultipleSingletonsError(k8sobjects.Repo(), k8sobjects.Repo()),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Repo(tc.objs)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("Got Repo() error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

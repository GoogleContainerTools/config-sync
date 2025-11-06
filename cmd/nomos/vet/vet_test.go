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

package vet

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/config-sync/cmd/nomos/flags"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/nonhierarchical"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/system"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/filesystem/cmpath"
	ft "github.com/GoogleContainerTools/config-sync/pkg/importer/filesystem/filesystemtest"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func resetFlags() {
	// Flags are global state carried over between tests.
	// Cobra lazily evaluates flags only if they are declared, so unless these
	// are reset, successive calls to Cmd.Execute aren't guaranteed to be
	// independent.
	flags.Clusters = nil
	flags.Path = flags.PathDefault
	flags.SkipAPIServer = true
	flags.SourceFormat = string(configsync.SourceFormatHierarchy)
	namespaceValue = ""
	keepOutput = false
	outPath = flags.DefaultHydrationOutput
	flags.OutputFormat = flags.OutputYAML
	flags.SkipAPIServerCheckForGroup = nil
}

func resetFlagExclusivityTestFlags() {
	flags.SkipAPIServer = false
	flags.SkipAPIServerCheckForGroup = nil
}

var examplesDir = cmpath.RelativeSlash("../../../examples")

func TestVet_Acme(t *testing.T) {
	resetFlags()
	Cmd.SilenceUsage = true

	os.Args = []string{
		"vet", // this first argument does nothing, but is required to exist.
		"--path", examplesDir.Join(cmpath.RelativeSlash("acme")).OSPath(),
	}

	err := Cmd.Execute()
	require.NoError(t, err)
}

func TestVet_AcmeSymlink(t *testing.T) {
	resetFlags()
	Cmd.SilenceUsage = true

	dir := ft.NewTestDir(t)
	symDir := dir.Root().Join(cmpath.RelativeSlash("acme-symlink"))

	absExamples, err := filepath.Abs(examplesDir.Join(cmpath.RelativeSlash("acme")).OSPath())
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(absExamples, symDir.OSPath())
	if err != nil {
		t.Fatal(err)
	}

	os.Args = []string{
		"vet", // this first argument does nothing, but is required to exist.
		"--path", symDir.OSPath(),
	}

	err = Cmd.Execute()
	require.NoError(t, err)
}

func TestVet_FooCorp(t *testing.T) {
	resetFlags()
	Cmd.SilenceUsage = true

	os.Args = []string{
		"vet", // this first argument does nothing, but is required to exist.
		"--path", examplesDir.Join(cmpath.RelativeSlash("foo-corp")).OSPath(),
	}

	err := Cmd.Execute()
	require.NoError(t, err)
}

func TestVet_MultiCluster(t *testing.T) {
	Cmd.SilenceUsage = true

	repoPath := filepath.Join(examplesDir.OSPath(), "parse-errors/cluster-specific-collision")
	absRepoPath, err := filepath.Abs(repoPath)
	require.NoError(t, err)

	tcs := []struct {
		name      string
		args      []string
		wantError error
	}{
		{
			name: "detect collision when all clusters enabled",
			wantError: clusterErrors{
				name: "prod-cluster",
				MultiError: nonhierarchical.ClusterMetadataNameCollisionError(
					kinds.ClusterRole().GroupKind(), "clusterrole",
					k8sobjects.ClusterRoleObject(core.Name("clusterrole"),
						core.Annotation(metadata.SourcePathAnnotationKey,
							filepath.Join(absRepoPath, "cluster/default_clusterrole.yaml"))),
					k8sobjects.ClusterRoleObject(core.Name("clusterrole"),
						core.Annotation(metadata.SourcePathAnnotationKey,
							filepath.Join(absRepoPath, "cluster/prod_clusterrole.yaml"))),
				),
			},
		},
		{
			name: "detect collision in prod-cluster",
			args: []string{"--clusters", "prod-cluster"},
			wantError: clusterErrors{
				name: "prod-cluster",
				MultiError: nonhierarchical.ClusterMetadataNameCollisionError(
					kinds.ClusterRole().GroupKind(), "clusterrole",
					k8sobjects.ClusterRoleObject(core.Name("clusterrole"),
						core.Annotation(metadata.SourcePathAnnotationKey,
							filepath.Join(absRepoPath, "cluster/default_clusterrole.yaml"))),
					k8sobjects.ClusterRoleObject(core.Name("clusterrole"),
						core.Annotation(metadata.SourcePathAnnotationKey,
							filepath.Join(absRepoPath, "cluster/prod_clusterrole.yaml"))),
				),
			},
		},
		{
			name: "do not detect collision in dev-cluster",
			args: []string{"--clusters", "dev-cluster"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			resetFlags()

			os.Args = append([]string{
				"vet", // this first argument does nothing, but is required to exist.
				"--path", repoPath,
			}, tc.args...)

			output := new(bytes.Buffer)
			Cmd.SetOut(output)
			Cmd.SetErr(output)

			err := Cmd.Execute()
			if tc.wantError == nil {
				require.NoError(t, err)
				require.Equal(t, "✅ No validation issues found.\n", output.String())
			} else {
				// Execute wraps the string output as a simple error.
				wantError := errors.New(tc.wantError.Error())
				require.Equal(t, wantError, err)
			}
		})
	}
}

func TestVet_FlagExclusivity(t *testing.T) {
	testCases := []struct {
		name                     string
		flags                    []string
		expectError              bool
		expectedError            string
		expectedWarning          string
		expectGroupsToBeNil      bool
		initialNoAPIServerGroups []string
	}{
		{
			name:          "Both flags used, expect error",
			flags:         []string{"--no-api-server-check", "--no-api-server-check-for-group=constraints.gatekeeper.sh"},
			expectError:   true,
			expectedError: fmt.Sprintf("cannot specify both --%s and --%s", flags.SkipAPIServerFlag, flags.NoAPIServerCheckForGroupFlag),
		},
		{
			name:        "Only --no-api-server-check used",
			flags:       []string{"--no-api-server-check"},
			expectError: false,
		},
		{
			name:                     "Only --no-api-server-check-for-group used",
			flags:                    []string{"--no-api-server-check-for-group=constraints.gatekeeper.sh"},
			expectError:              false,
			initialNoAPIServerGroups: []string{"constraints.gatekeeper.sh"},
		},
		{
			name:        "No flags used",
			flags:       []string{},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resetFlags()
			// We need a valid directory for the command to run, even though we're testing flag validation.
			tmpDir, err := os.MkdirTemp("", "nomos-vet-test")
			require.NoError(t, err)
			t.Cleanup(func() {
				err := os.RemoveAll(tmpDir)
				require.NoError(t, err)
			})

			resetFlagExclusivityTestFlags()

			// Set the flags for the current test case.
			var groups []string
			for _, f := range tc.flags {
				if f == "--no-api-server-check" {
					flags.SkipAPIServer = true
				} else if strings.HasPrefix(f, "--no-api-server-check-for-group=") {
					parts := strings.SplitN(f, "=", 2)
					groups = append(groups, strings.Split(parts[1], ",")...)
				}
			}
			flags.SkipAPIServerCheckForGroup = groups
			if tc.initialNoAPIServerGroups != nil {
				flags.SkipAPIServerCheckForGroup = tc.initialNoAPIServerGroups
			}

			output := new(bytes.Buffer)
			Cmd.SetOut(output)
			Cmd.SetErr(output)

			err = Cmd.PreRunE(Cmd, nil)

			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
			}

			if tc.expectedWarning != "" {
				require.Equal(t, tc.expectedWarning, output.String())
			}

			if tc.expectGroupsToBeNil {
				require.Nil(t, flags.SkipAPIServerCheckForGroup)
			}
		})
	}
}

func TestVet_Threshold(t *testing.T) {
	Cmd.SilenceUsage = true

	tcs := []struct {
		name      string
		objects   []ast.FileObject
		args      []string
		wantError error
	}{
		{
			name: "unstructured, no objects",
			args: []string{
				"--source-format", string(configsync.SourceFormatUnstructured),
			},
		},
		{
			name: "unstructured, 1 object",
			objects: []ast.FileObject{
				k8sobjects.FileObject(k8sobjects.DeploymentObject(), "deployment.yaml"),
			},
			args: []string{
				"--source-format", string(configsync.SourceFormatUnstructured),
			},
		},
		{
			name: "unstructured, 1001 objects, threshold unspecified",
			objects: func() []ast.FileObject {
				objs := make([]ast.FileObject, 1001)
				for i := range objs {
					name := fmt.Sprintf("deployment-%d", i)
					fileName := fmt.Sprintf("deployment-%d.yaml", i)
					objs[i] = k8sobjects.FileObject(
						k8sobjects.DeploymentObject(
							core.Name(name),
							core.Namespace("example")),
						fileName)
				}
				return objs
			}(),
			args: []string{
				"--source-format", string(configsync.SourceFormatUnstructured),
			},
		},
		{
			name: "unstructured, 1001 objects, threshold default",
			objects: func() []ast.FileObject {
				objs := make([]ast.FileObject, 1001)
				for i := range objs {
					name := fmt.Sprintf("deployment-%d", i)
					fileName := fmt.Sprintf("deployment-%d.yaml", i)
					objs[i] = k8sobjects.FileObject(
						k8sobjects.DeploymentObject(
							core.Name(name),
							core.Namespace("example")),
						fileName)
				}
				return objs
			}(),
			args: []string{
				"--source-format", string(configsync.SourceFormatUnstructured),
				"--threshold",
			},
			wantError: system.MaxObjectCountError(1000, 1001),
		},
		{
			name: "unstructured, 1001 objects, threshold 1000",
			objects: func() []ast.FileObject {
				objs := make([]ast.FileObject, 1001)
				for i := range objs {
					name := fmt.Sprintf("deployment-%d", i)
					fileName := fmt.Sprintf("deployment-%d.yaml", i)
					objs[i] = k8sobjects.FileObject(
						k8sobjects.DeploymentObject(
							core.Name(name),
							core.Namespace("example")),
						fileName)
				}
				return objs
			}(),
			args: []string{
				"--source-format", string(configsync.SourceFormatUnstructured),
				"--threshold=1000",
			},
			wantError: system.MaxObjectCountError(1000, 1001),
		},
		{
			name: "unstructured, 1001 objects, threshold 1001",
			objects: func() []ast.FileObject {
				objs := make([]ast.FileObject, 1001)
				for i := range objs {
					name := fmt.Sprintf("deployment-%d", i)
					fileName := fmt.Sprintf("deployment-%d.yaml", i)
					objs[i] = k8sobjects.FileObject(
						k8sobjects.DeploymentObject(
							core.Name(name),
							core.Namespace("example")),
						fileName)
				}
				return objs
			}(),
			args: []string{
				"--source-format", string(configsync.SourceFormatUnstructured),
				"--threshold=1001",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			resetFlags()

			tmpDirPath, err := os.MkdirTemp("", "nomos-test-")
			require.NoError(t, err)
			t.Cleanup(func() {
				err := os.RemoveAll(tmpDirPath)
				require.NoError(t, err)
			})

			// Write objects to temp dir as yaml files
			for _, fileObj := range tc.objects {
				filePath := filepath.Join(tmpDirPath, fileObj.OSPath())
				err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
				require.NoError(t, err)
				fileData, err := yaml.Marshal(fileObj.Unstructured)
				require.NoError(t, err)
				err = os.WriteFile(filePath, fileData, 0644)
				require.NoError(t, err)
			}

			os.Args = append([]string{
				"vet", // this first argument does nothing, but is required to exist.
				"--path", tmpDirPath,
			}, tc.args...)

			output := new(bytes.Buffer)
			Cmd.SetOut(output)
			Cmd.SetErr(output)

			err = Cmd.Execute()
			if tc.wantError == nil {
				require.NoError(t, err)
				require.Equal(t, "✅ No validation issues found.\n", output.String())
			} else {
				// Execute wraps the string output as a simple error.
				wantError := errors.New(tc.wantError.Error())
				require.Equal(t, wantError, err)
			}
		})
	}
}

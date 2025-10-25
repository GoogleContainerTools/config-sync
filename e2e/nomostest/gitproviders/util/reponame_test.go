// Copyright 2025 Google LLC
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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeRepoName(t *testing.T) {
	testCases := []struct {
		testName     string
		repoSuffix   string
		repoName     string
		expectedName string
	}{
		{
			testName:     "RepoSync test-ns/repo-sync",
			repoSuffix:   "project/cluster",
			repoName:     "test-ns/repo-sync",
			expectedName: "test-ns-repo-sync-project-cluster-b96b1396",
		},
		{
			testName:     "RepoSync test/ns-repo-sync should not collide with RepoSync test-ns/repo-sync",
			repoSuffix:   "project/cluster",
			repoName:     "test/ns-repo-sync",
			expectedName: "test-ns-repo-sync-project-cluster-d98dee7d",
		},
		{
			testName:     "A very long repoSuffix should be truncated",
			repoSuffix:   "kpt-config-sync-ci-main/autopilot-rapid-latest-10",
			repoName:     "config-management-system/root-sync",
			expectedName: "config-management-system-root-sync-kpt-config-sync-ci-6485bfa0",
		},
		{
			testName:     "A similar very long repoSuffix should be truncated and not collide",
			repoSuffix:   "kpt-config-sync-ci-release/autopilot-rapid-latest-10",
			repoName:     "config-management-system/root-sync",
			expectedName: "config-management-system-root-sync-kpt-config-sync-ci-8b9c3b0d",
		},
		{
			testName:     "A very long repoName should be truncated",
			repoSuffix:   "test",
			repoName:     "config-management-system/root-sync-with-a-very-long-name",
			expectedName: "config-management-system-root-sync-with-a-very-long-na-3b0dae1c",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotName := SanitizeRepoName(tc.repoSuffix, tc.repoName)
			assert.Equal(t, tc.expectedName, gotName)
		})
	}
}

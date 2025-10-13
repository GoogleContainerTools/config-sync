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
		repoPrefix   string
		repoName     string
		expectedName string
	}{
		{
			testName:     "RepoSync test-ns/repo-sync",
			repoPrefix:   "test",
			repoName:     "test-ns/repo-sync",
			expectedName: "cs-e2e-test-test-ns-repo-sync-19dcbc51",
		},
		{
			testName:     "The expected name shouldn't have a double dash in it",
			repoPrefix:   "my-test-cluster1",
			repoName:     "config-management-system/root-sync",
			expectedName: "cs-e2e-my-test-cluster1-config-management-system-root-a5af55f0",
		},
		{
			testName:     "RepoSync test/ns-repo-sync should not collide with RepoSync test-ns/repo-sync",
			repoPrefix:   "test",
			repoName:     "test/ns-repo-sync",
			expectedName: "cs-e2e-test-test-ns-repo-sync-f98ca740",
		},
		{
			testName:     "A very long repoPrefix should be truncated",
			repoPrefix:   "autopilot-rapid-latest-10",
			repoName:     "config-management-system/root-sync",
			expectedName: "cs-e2e-autopilot-rapid-latest-10-config-management-sys-0aab99c5",
		},
		{
			testName:     "A very long repoName should be truncated",
			repoPrefix:   "test",
			repoName:     "config-management-system/root-sync-with-a-very-long-name",
			expectedName: "cs-e2e-test-config-management-system-root-sync-with-a-0d0af6c0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotName := SanitizeRepoName(tc.repoPrefix, tc.repoName)
			assert.Equal(t, tc.expectedName, gotName)
			assert.LessOrEqual(t, len(gotName), defaultRepoNameMaxLen)
		})
	}
}

func TestSanitizeBitbucketRepoName(t *testing.T) {
	testCases := []struct {
		testName     string
		repoPrefix   string
		repoName     string
		expectedName string
	}{
		{
			testName:     "RepoSync test-ns/repo-sync",
			repoPrefix:   "test",
			repoName:     "test-ns/repo-sync",
			expectedName: "cs-e2e-test-test-ns-repo-sync-19dcbc51",
		},
		{
			testName:     "The expected name shouldn't have a double dash in it",
			repoPrefix:   "my-test-cluster",
			repoName:     "config-management-system/root-sync",
			expectedName: "cs-e2e-my-test-cluster-config-management-system-root-e5e2fb26",
		},
		{
			testName:     "RepoSync test/ns-repo-sync should not collide with RepoSync test-ns/repo-sync",
			repoPrefix:   "test",
			repoName:     "test/ns-repo-sync",
			expectedName: "cs-e2e-test-test-ns-repo-sync-f98ca740",
		},
		{
			testName:     "A very long repoPrefix should be truncated",
			repoPrefix:   "autopilot-rapid-latest-10",
			repoName:     "config-management-system/root-sync",
			expectedName: "cs-e2e-autopilot-rapid-latest-10-config-management-sy-0aab99c5",
		},
		{
			testName:     "A very long repoName should be truncated",
			repoPrefix:   "test",
			repoName:     "config-management-system/root-sync-with-a-very-long-name",
			expectedName: "cs-e2e-test-config-management-system-root-sync-with-a-0d0af6c0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotName := SanitizeBitbucketRepoName(tc.repoPrefix, tc.repoName)
			assert.Equal(t, tc.expectedName, gotName)
			assert.LessOrEqual(t, len(gotName), bitbucketRepoNameMaxLen)
		})
	}
}

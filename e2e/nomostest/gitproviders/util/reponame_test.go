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

func TestSanitizeGCPRepoName(t *testing.T) {
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
		{
			testName:     "Empty repoPrefix",
			repoPrefix:   "",
			repoName:     "test-ns/repo-sync",
			expectedName: "cs-e2e-test-ns-repo-sync-cf15cd55",
		}, {
			testName:     "Empty repoName",
			repoPrefix:   "test",
			repoName:     "",
			expectedName: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotName := SanitizeGCPRepoName(tc.repoPrefix, tc.repoName)
			assert.Equal(t, tc.expectedName, gotName)
			assert.LessOrEqual(t, len(gotName), defaultRepoNameMaxLen)
		})
	}
}

func TestSanitizeGitlabRepoName(t *testing.T) {
	testCases := []struct {
		testName     string
		repoSuffix   string
		repoName     string
		expectedName string
	}{
		{
			testName:     "RepoSync test-ns/repo-sync",
			repoSuffix:   "test",
			repoName:     "test-ns/repo-sync",
			expectedName: "test-ns-repo-sync-test-3b350268",
		},
		{
			testName:     "RepoSync test/ns-repo-sync should not collide with RepoSync test-ns/repo-sync",
			repoSuffix:   "test",
			repoName:     "test/ns-repo-sync",
			expectedName: "test-ns-repo-sync-test-6831d08b",
		},
		{
			testName:     "A very long repoName should be truncated",
			repoSuffix:   "test",
			repoName:     "config-management-system/root-sync-with-a-very-long-name-that-is-definitely-going-to-be-way-too-long-for-gcp-or-bitbucket-but-not-for-gitlab-because-it-has-a-much-longer-limit-so-we-need-to-make-this-string-extra-long-to-test-truncation-logic-for-gitlab-as-well-just-in-case-someone-is-feeling-verbose",
			expectedName: "config-management-system-root-sync-with-a-very-long-name-that-is-definitely-going-to-be-way-too-long-for-gcp-or-bitbucket-but-not-for-gitlab-because-it-has-a-much-longer-limit-so-we-need-to-make-this-string-extra-long-to-test-truncation-logic-for-b29fd62d",
		},
		{
			testName:     "Empty repoSuffix",
			repoSuffix:   "",
			repoName:     "test-ns/repo-sync",
			expectedName: "test-ns-repo-sync-08675d6b",
		}, {
			testName:     "Empty repoName",
			repoSuffix:   "test",
			repoName:     "",
			expectedName: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotName := SanitizeGitlabRepoName(tc.repoSuffix, tc.repoName)
			assert.Equal(t, tc.expectedName, gotName)
			assert.LessOrEqual(t, len(gotName), gitlabProjectNameMaxLength)
		})
	}
}

func TestSanitizeBitbucketRepoName(t *testing.T) {
	testCases := []struct {
		testName     string
		repoSuffix   string
		repoName     string
		expectedName string
	}{
		{
			testName:     "RepoSync test-ns/repo-sync",
			repoSuffix:   "test",
			repoName:     "test-ns/repo-sync",
			expectedName: "test-ns-repo-sync-test-3b350268",
		},
		{
			testName:     "RepoSync test/ns-repo-sync should not collide with RepoSync test-ns/repo-sync",
			repoSuffix:   "test",
			repoName:     "test/ns-repo-sync",
			expectedName: "test-ns-repo-sync-test-6831d08b",
		},
		{
			testName:     "The expected name shouldn't have a double dash in it",
			repoSuffix:   "test",
			repoName:     "config-management-system/a-new-root-sync-with-ab-123",
			expectedName: "config-management-system-a-new-root-sync-with-ab-123-da77fbd1",
		},
		{
			testName:     "A very long repoSuffix should be truncated",
			repoSuffix:   "kpt-config-sync-ci-main/autopilot-rapid-latest-10",
			repoName:     "config-management-system/root-sync",
			expectedName: "config-management-system-root-sync-kpt-config-sync-ci-6485bfa0",
		},
		{
			testName:     "A very long repoName should be truncated",
			repoSuffix:   "test",
			repoName:     "config-management-system/root-sync-with-a-very-long-name",
			expectedName: "config-management-system-root-sync-with-a-very-long-n-3b0dae1c",
		},
		{
			testName:     "Empty repoSuffix",
			repoSuffix:   "",
			repoName:     "test-ns/repo-sync",
			expectedName: "test-ns-repo-sync-08675d6b",
		}, {
			testName:     "Empty repoName",
			repoSuffix:   "test",
			repoName:     "",
			expectedName: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			gotName := SanitizeBitbucketRepoName(tc.repoSuffix, tc.repoName)
			assert.Equal(t, tc.expectedName, gotName)
			assert.LessOrEqual(t, len(gotName), bitbucketRepoNameMaxLen)
		})
	}
}

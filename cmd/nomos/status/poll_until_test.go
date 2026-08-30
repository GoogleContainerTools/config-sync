// Copyright 2026 Google LLC
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

package status

import (
	"testing"

	kptv1alpha1 "github.com/GoogleContainerTools/config-sync/pkg/api/kpt.dev/v1alpha1"
)

func TestAllSynced(t *testing.T) {
	tests := []struct {
		name   string
		states map[string]*ClusterState
		want   bool
	}{
		{
			name: "all repositories synced",
			states: map[string]*ClusterState{
				"cluster": {repos: []*RepoState{{status: syncedMsg}}},
			},
			want: true,
		},
		{
			name: "pending repository",
			states: map[string]*ClusterState{
				"cluster": {repos: []*RepoState{{status: pendingMsg}}},
			},
		},
		{
			name: "repository with non-current resource",
			states: map[string]*ClusterState{
				"cluster": {repos: []*RepoState{{
					status: syncedMsg,
					resources: []kptv1alpha1.ResourceStatus{{
						Status: kptv1alpha1.Failed,
					}},
				}}},
		},
		{
			name: "cluster error",
			states: map[string]*ClusterState{
				"cluster": {Error: "unavailable", repos: []*RepoState{{status: syncedMsg}}},
			},
		},
		{
			name: "empty state",
			states: map[string]*ClusterState{
				"cluster": {},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := allSynced(test.states); got != test.want {
				t.Fatalf("allSynced() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestValidatePollUntil(t *testing.T) {
	if err := validatePollUntil(""); err != nil {
		t.Fatalf("validatePollUntil(\"\") returned error: %v", err)
	}
	if err := validatePollUntil(pollUntilComplete); err != nil {
		t.Fatalf("validatePollUntil(%q) returned error: %v", pollUntilComplete, err)
	}
	if err := validatePollUntil("ready"); err == nil {
		t.Fatal("validatePollUntil(\"ready\") returned nil")
	}
}

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

package gvkutil

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ByGroupKind implements sort.Interface for []Pattern based on the Group field.
type ByGroupKind []Pattern

func (a ByGroupKind) Len() int      { return len(a) }
func (a ByGroupKind) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByGroupKind) Less(i, j int) bool {
	return a[i].Group < a[j].Group
}

func TestParsePatterns(t *testing.T) {
	testCases := []struct {
		name        string
		rawPatterns []string
		expected    []Pattern
		expectErr   bool
	}{
		{
			name:        "group-only patterns",
			rawPatterns: []string{"group1", "group2.example.com"},
			expected: []Pattern{
				{Group: "group1"},
				{Group: "group2.example.com"},
			},
			expectErr: false,
		},
		{
			name:        "empty input",
			rawPatterns: []string{},
			expected:    []Pattern{},
			expectErr:   false,
		},
		{
			name:        "nil input",
			rawPatterns: nil,
			expected:    []Pattern{},
			expectErr:   false,
		},
		{
			name:        "ignore empty and whitespace-only patterns",
			rawPatterns: []string{"group1", "  group2  ", "", "   "},
			expected: []Pattern{
				{Group: "group1"},
				{Group: "group2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			patterns, err := ParsePatterns(tc.rawPatterns)
			if (err != nil) != tc.expectErr {
				t.Errorf("ParsePatterns() error = %v, expectErr %v", err, tc.expectErr)
				return
			}
			sort.Sort(ByGroupKind(patterns))
			sort.Sort(ByGroupKind(tc.expected))
			if diff := cmp.Diff(tc.expected, patterns); diff != "" {
				t.Errorf("patterns mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMatches(t *testing.T) {
	patterns := []Pattern{
		{Group: "constraints.gatekeeper.sh"},
		{Group: "acme.io"},
	}

	testCases := []struct {
		name     string
		gk       schema.GroupKind
		expected bool
	}{
		{
			name:     "match group-only",
			gk:       schema.GroupKind{Group: "constraints.gatekeeper.sh", Kind: "K8sRequiredLabels"},
			expected: true,
		},
		{
			name:     "another match group-only",
			gk:       schema.GroupKind{Group: "acme.io", Kind: "Certificate"},
			expected: true,
		},
		{
			name:     "no match",
			gk:       schema.GroupKind{Group: "core", Kind: "Pod"},
			expected: false,
		},
		{
			name:     "empty group",
			gk:       schema.GroupKind{Group: "", Kind: "Namespace"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Matches(tc.gk, patterns); got != tc.expected {
				t.Errorf("Matches() = %v, want %v", got, tc.expected)
			}
		})
	}
}

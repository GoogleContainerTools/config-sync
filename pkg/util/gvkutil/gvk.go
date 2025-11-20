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
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Pattern represents a pattern to match against a GroupKind.
type Pattern struct {
	Group string
}

// ParsePatterns parses a slice of strings into a slice of Patterns.
// Each string is expected to be a group.
func ParsePatterns(rawPatterns []string) ([]Pattern, error) {
	if rawPatterns == nil {
		return []Pattern{}, nil
	}
	patterns := make([]Pattern, 0, len(rawPatterns))
	for _, p := range rawPatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		patterns = append(patterns, Pattern{Group: p})
	}
	return patterns, nil
}

// Matches checks if the given GroupKind matches any of the patterns.
func Matches(gk schema.GroupKind, patterns []Pattern) bool {
	for _, p := range patterns {
		if gk.Group == p.Group {
			return true
		}
	}
	return false
}

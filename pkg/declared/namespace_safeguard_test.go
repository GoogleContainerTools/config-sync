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

package declared

import (
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/testerrors"
	"github.com/elliotchance/orderedmap/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDontDeleteAllNamespaces(t *testing.T) {
	testCases := []struct {
		name     string
		previous []string
		current  []string
		want     status.Error
	}{
		{
			name:     "zero to zero",
			previous: []string{},
			current:  []string{},
		},
		{
			name:     "zero to one",
			previous: []string{},
			current:  []string{"foo"},
		},
		{
			name:     "zero to two",
			previous: []string{},
			current:  []string{"foo", "bar"},
		},
		{
			name:     "one to zero",
			previous: []string{},
			current:  []string{},
		},
		{
			name:     "one to one",
			previous: []string{"foo"},
			current:  []string{"foo"},
		},
		{
			name:     "one to two",
			previous: []string{"foo"},
			current:  []string{"foo", "bar"},
		},
		{
			name:     "two to zero",
			previous: []string{"foo", "bar"},
			current:  []string{},
			want:     DeleteAllNamespacesError([]string{"foo", "bar"}),
		},
		{
			name:     "two to one",
			previous: []string{"foo", "bar"},
			current:  []string{"foo"},
		},
		{
			name:     "two to two",
			previous: []string{"foo", "bar"},
			current:  []string{"foo", "bar"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			previous := orderedmap.NewOrderedMap[core.ID, *unstructured.Unstructured]()
			for _, p := range tc.previous {
				u := k8sobjects.UnstructuredObject(kinds.Namespace(), core.Name(p))
				previous.Set(core.IDOf(u), u)
			}
			current := orderedmap.NewOrderedMap[core.ID, *unstructured.Unstructured]()
			for _, c := range tc.current {
				u := k8sobjects.UnstructuredObject(kinds.Namespace(), core.Name(c))
				current.Set(core.IDOf(u), u)
			}

			got := deletesAllNamespaces(previous, current)
			testerrors.AssertEqual(t, got, tc.want)
		})
	}
}

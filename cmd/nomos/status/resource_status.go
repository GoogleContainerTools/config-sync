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

package status

import (
	"fmt"
	"strings"

	kptv1alpha1 "github.com/GoogleContainerTools/config-sync/pkg/api/kpt.dev/v1alpha1"
)

func resourceStatusToString(r kptv1alpha1.ResourceStatus) string {
	if r.Group == "" {
		return fmt.Sprintf("%s/%s", strings.ToLower(r.Kind), r.Name)
	}
	return fmt.Sprintf("%s.%s/%s", strings.ToLower(r.Kind), r.Group, r.Name)
}

// byNamespaceAndType implements sort.Interface:
// It first sorts the ResourceStatuses by namespace, then sorts them
// by type.
type byNamespaceAndType []kptv1alpha1.ResourceStatus

func (b byNamespaceAndType) Len() int {
	return len(b)
}

func (b byNamespaceAndType) Less(i, j int) bool {
	if b[i].Namespace < b[j].Namespace {
		return true
	}
	if b[i].Namespace > b[j].Namespace {
		return false
	}
	return resourceStatusToString(b[i]) < resourceStatusToString(b[j])
}

func (b byNamespaceAndType) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func resourceLevelStatus(rg *kptv1alpha1.ResourceGroup) []kptv1alpha1.ResourceStatus {
	if rg == nil {
		return nil
	}
	return checkConflict(rg.Status.ResourceStatuses)
}

func checkConflict(states []kptv1alpha1.ResourceStatus) []kptv1alpha1.ResourceStatus {
	for i, s := range states {
		for _, c := range s.Conditions {
			if c.Type == kptv1alpha1.Ownership && c.Status == kptv1alpha1.TrueConditionStatus {
				states[i].Status = "Conflict"
			}
		}
	}
	return states
}

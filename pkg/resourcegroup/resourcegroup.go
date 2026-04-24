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

package resourcegroup

import (
	"github.com/GoogleContainerTools/config-sync/pkg/api/kpt.dev/v1alpha1"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
)

// GetSourceHash returns the source hash that is defined in the
// source hash annotation.
func GetSourceHash(annotations map[string]string) string {
	if len(annotations) == 0 {
		return ""
	}
	return TruncateSourceHash(annotations[metadata.SyncTokenAnnotationKey])
}

// TruncateSourceHash truncates the provided source hash after the first 7 characters.
func TruncateSourceHash(sourceHash string) string {
	if len(sourceHash) > 7 {
		return sourceHash[0:7]
	}
	return sourceHash
}

// IsStatusDisabled returns whether the ResourceGroup has disabled status updates.
func IsStatusDisabled(resgroup *v1alpha1.ResourceGroup) bool {
	annotations := resgroup.GetAnnotations()
	if annotations == nil {
		return false
	}
	val, found := annotations[metadata.StatusModeAnnotationKey]
	return found && val == metadata.StatusDisabled.String()
}

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

package applier

import (
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/remediator/conflict"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KptManagementConflictError indicates that the passed resource is illegally
// declared in multiple repositories.
// TODO: merge with status.ManagementConflictError if cli-utils supports reporting the conflicting manager in InventoryOverlapError.
func KptManagementConflictError(resource client.Object) status.Error {
	newManager := core.GetAnnotation(resource, metadata.ResourceManagerKey)
	return status.ManagementConflictErrorBuilder.
		Sprintf("The %q reconciler detected a management conflict with another reconciler. "+
			"Remove the object from one of the sources of truth so that the object is only managed by one reconciler.",
			newManager).
		BuildWithConflictingManagers(resource, newManager, conflict.UnknownManager)
}

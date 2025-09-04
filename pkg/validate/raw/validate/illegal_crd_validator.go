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

package validate

import (
	v1 "github.com/GoogleContainerTools/config-sync/pkg/api/configmanagement/v1"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/validation/nonhierarchical"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/util/clusterconfig"
)

var illegalGroups = map[string]bool{
	v1.SchemeGroupVersion.Group:      true,
	v1beta1.SchemeGroupVersion.Group: true,
}

// IllegalCRD returns an error if the given FileObject is a CRD of a Config Sync
// type.
func IllegalCRD(obj ast.FileObject) status.Error {
	if obj.GetObjectKind().GroupVersionKind().GroupKind() != kinds.CustomResourceDefinition() {
		return nil
	}

	crd, err := clusterconfig.ToCRD(obj.Unstructured, core.Scheme)
	if err != nil {
		return err
	}
	if illegalGroups[crd.Spec.Group] {
		return nonhierarchical.UnsupportedObjectError(obj)
	}
	return nil
}

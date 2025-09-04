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

package clusterconfig

import (
	"fmt"

	v1 "github.com/GoogleContainerTools/config-sync/pkg/api/configmanagement/v1"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"github.com/GoogleContainerTools/config-sync/pkg/syncer/decode"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCRDs returns the names and CustomResourceDefinitions of the CRDs in ClusterConfig.
func GetCRDs(decoder decode.Decoder, clusterConfig *v1.ClusterConfig) ([]*apiextensionsv1.CustomResourceDefinition, status.Error) {
	if clusterConfig == nil {
		return nil, nil
	}

	gvkrs, err := decoder.DecodeResources(clusterConfig.Spec.Resources)
	if err != nil {
		return nil, status.APIServerErrorf(err, "could not deserialize CRD in %s", v1.CRDClusterConfigName)
	}

	crdMap := make(map[string]*apiextensionsv1.CustomResourceDefinition)
	for gvk, unstructureds := range gvkrs {
		if gvk.GroupKind() != kinds.CustomResourceDefinition() {
			return nil, status.APIServerErrorf(err, "%s contains non-CRD resources: %v", v1.CRDClusterConfigName, gvk)
		}
		for _, u := range unstructureds {
			crd, err := ToCRD(u, decoder.Scheme())
			if err != nil {
				return nil, err
			}
			crdMap[crd.GetName()] = crd
		}
	}

	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, crd := range crdMap {
		crds = append(crds, crd)
	}
	return crds, nil
}

// MalformedCRDErrorCode is the error code for MalformedCRDError.
const MalformedCRDErrorCode = "1065"

var malformedCRDErrorBuilder = status.NewErrorBuilder(MalformedCRDErrorCode)

// MalformedCRDError reports a malformed CRD.
func MalformedCRDError(err error, obj client.Object) status.Error {
	return malformedCRDErrorBuilder.Wrap(err).
		Sprint("malformed CustomResourceDefinition").
		BuildWithResources(obj)
}

// ToCRD converts an Unstructured object into a v1.CustomResourceDefinition
// using conversions registered with the specified scheme.
func ToCRD(o *unstructured.Unstructured, scheme *runtime.Scheme) (*apiextensionsv1.CustomResourceDefinition, status.Error) {
	obj, err := kinds.ToTypedWithVersion(o, kinds.CustomResourceDefinitionV1(), scheme)
	if err != nil {
		return nil, MalformedCRDError(err, o)
	}
	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return nil, MalformedCRDError(fmt.Errorf("unexpected type produced by converting unstructured CRD to v1 CRD: %T", obj), o)
	}
	return crd, nil
}

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

package k8sobjects

import (
	kptv1alpha1 "github.com/GoogleContainerTools/config-sync/pkg/api/kpt.dev/v1alpha1"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/kinds"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceGroupObject initializes a ResourceGroup.
func ResourceGroupObject(ns, name string, opts ...core.MetaMutator) *kptv1alpha1.ResourceGroup {
	result := &kptv1alpha1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		TypeMeta: ToTypeMeta(kinds.ResourceGroup()),
	}
	mutate(result, opts...)

	return result
}

// WithRGResourceStatuses sets the ResourceStatuses of the ResourceGroup object.
func WithRGResourceStatuses(rs ...kptv1alpha1.ResourceStatus) core.MetaMutator {
	return func(o client.Object) {
		rg := o.(*kptv1alpha1.ResourceGroup)
		rg.Status.ResourceStatuses = append(rg.Status.ResourceStatuses, rs...)
	}
}

// WithRGResources sets the Spec.Resources of the ResourceGroup object.
func WithRGResources(resources ...kptv1alpha1.ObjMetadata) core.MetaMutator {
	return func(o client.Object) {
		rg := o.(*kptv1alpha1.ResourceGroup)
		rg.Spec.Resources = append(rg.Spec.Resources, resources...)
	}
}

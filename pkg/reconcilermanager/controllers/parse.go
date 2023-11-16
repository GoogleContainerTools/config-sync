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

package controllers

import (
	"os"

	"sigs.k8s.io/yaml"

	appsv1 "k8s.io/api/apps/v1"
)

const (
	// ReconcilerTemplateConfigMapKey is the key used to specify the reconciler
	// deployment template in the "reconciler-manager-cm" ConfigMap.
	// Defined in configmap manifests/templates/reconciler-manager-configmap.yaml
	ReconcilerTemplateConfigMapKey = "deployment.yaml"

	// ReconcilerTemplateConfigMapName is the name of the ConfigMap used to
	// specify the reconciler deployment template.
	// Defined in configmap manifests/templates/reconciler-manager-configmap.yaml
	ReconcilerTemplateConfigMapName = "reconciler-manager-cm"
)

// parseDeployment parse deployment from deployment.yaml to deploy reconciler pod
// Alias to enable test mocking.
var parseDeployment = func(de *appsv1.Deployment) error {
	return parseFromDeploymentConfig(ReconcilerTemplateConfigMapKey, de)
}

func parseFromDeploymentConfig(config string, obj *appsv1.Deployment) error {
	yamlDep, err := os.ReadFile(config)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(yamlDep, obj)
}

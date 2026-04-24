/*
Copyright 2021 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kmetrics

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/klog/v2"
)

// Attribute keys for kustomize metrics
var (
	KeyFieldName              = attribute.Key("field_name")
	KeyDeprecatingField       = attribute.Key("deprecating_field")
	KeySimplificationField    = attribute.Key("simplification_field")
	KeyK8sMetadataTransformer = attribute.Key("k8s_metadata_transformer")
	KeyHelmInflator           = attribute.Key("helm_inflator")
	KeyBaseSource             = attribute.Key("base_source")
	KeyPatchField             = attribute.Key("patch_field")
	KeyTopTierField           = attribute.Key("top_tier_field")
)

var (
	// KustomizeFieldCount is the number of times a particular field is used
	KustomizeFieldCount metric.Int64Gauge

	// KustomizeDeprecatingFields is the usage of fields that may become deprecated
	KustomizeDeprecatingFields metric.Int64Gauge

	// KustomizeSimplification is the usage of simplification transformers
	KustomizeSimplification metric.Int64Gauge

	// KustomizeK8sMetadata is the usage of builtin transformers
	KustomizeK8sMetadata metric.Int64Gauge

	// KustomizeHelmMetrics is the usage of helm
	KustomizeHelmMetrics metric.Int64Gauge

	// KustomizeBaseCount is the number of remote and local bases
	KustomizeBaseCount metric.Int64Gauge

	// KustomizePatchCount is the number of patches
	KustomizePatchCount metric.Int64Gauge

	// KustomizeTopTierMetrics is the usage of high level metrics
	KustomizeTopTierMetrics metric.Int64Gauge

	// KustomizeResourceCount is the number of resources outputted by `kustomize build`
	KustomizeResourceCount metric.Int64Gauge

	// KustomizeExecutionTime is the execution time of `kustomize build`
	KustomizeExecutionTime metric.Float64Histogram
)

// InitializeOTelKustomizeMetrics initializes OpenTelemetry Kustomize metrics instruments
func InitializeOTelKustomizeMetrics() error {
	klog.V(5).Infof("METRIC DEBUG: Initializing OpenTelemetry kustomize metrics instruments")

	meter := otel.Meter("config-sync-kmetric")
	klog.V(5).Infof("METRIC DEBUG: Created kustomize meter: config-sync-kmetric")

	var err error

	// Initialize gauge instruments
	KustomizeFieldCount, err = meter.Int64Gauge(
		"kustomize_field_count",
		metric.WithDescription("The number of times a particular field is used in the kustomization files"),
	)
	if err != nil {
		return err
	}

	KustomizeDeprecatingFields, err = meter.Int64Gauge(
		"kustomize_deprecating_field_count",
		metric.WithDescription("The usage of fields that may become deprecated"),
	)
	if err != nil {
		return err
	}

	KustomizeSimplification, err = meter.Int64Gauge(
		"kustomize_simplification_adoption_count",
		metric.WithDescription("The usage of simplification transformers images, replicas, and replacements"),
	)
	if err != nil {
		return err
	}

	KustomizeK8sMetadata, err = meter.Int64Gauge(
		"kustomize_builtin_transformers",
		metric.WithDescription("The usage of builtin transformers related to kubernetes object metadata"),
	)
	if err != nil {
		return err
	}

	KustomizeHelmMetrics, err = meter.Int64Gauge(
		"kustomize_helm_inflator_count",
		metric.WithDescription("The usage of helm in kustomize, whether by the builtin fields or the custom function"),
	)
	if err != nil {
		return err
	}

	KustomizeBaseCount, err = meter.Int64Gauge(
		"kustomize_base_count",
		metric.WithDescription("The number of remote and local bases"),
	)
	if err != nil {
		return err
	}

	KustomizePatchCount, err = meter.Int64Gauge(
		"kustomize_patch_count",
		metric.WithDescription("The number of patches in the fields `patches`, `patchesStrategicMerge`, and `patchesJson6902`"),
	)
	if err != nil {
		return err
	}

	KustomizeTopTierMetrics, err = meter.Int64Gauge(
		"kustomize_ordered_top_tier_metrics",
		metric.WithDescription("Usage of Resources, Generators, SecretGenerator, ConfigMapGenerator, Transformers, and Validators"),
	)
	if err != nil {
		return err
	}

	KustomizeResourceCount, err = meter.Int64Gauge(
		"kustomize_resource_count",
		metric.WithDescription("The number of resources outputted by `kustomize build`"),
	)
	if err != nil {
		return err
	}

	// Initialize histogram instrument
	KustomizeExecutionTime, err = meter.Float64Histogram(
		"kustomize_build_latency",
		metric.WithDescription("Kustomize build latency"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		klog.V(5).ErrorS(err, "METRIC DEBUG: Failed to create KustomizeExecutionTime histogram")
		return err
	}
	klog.V(5).Infof("METRIC DEBUG: Created KustomizeExecutionTime histogram")

	klog.V(5).Infof("METRIC DEBUG: Successfully initialized all OpenTelemetry kustomize metrics instruments")
	return nil
}

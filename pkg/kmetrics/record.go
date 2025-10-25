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

package kmetrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/klog/v2"
)

// RecordKustomizeFieldCountData records all data relevant to the kustomization's field counts
func RecordKustomizeFieldCountData(ctx context.Context, fieldCountData *KustomizeFieldMetrics) {

	klog.V(5).Infof("METRIC DEBUG: Recording KustomizeFieldCountData - FieldCount: %d, DeprecationMetrics: %d, SimplMetrics: %d",
		len(fieldCountData.FieldCount), len(fieldCountData.DeprecationMetrics), len(fieldCountData.SimplMetrics))

	recordKustomizeFieldCount(ctx, fieldCountData.FieldCount)
	recordKustomizeDeprecatingFields(ctx, fieldCountData.DeprecationMetrics)
	recordKustomizeSimplification(ctx, fieldCountData.SimplMetrics)
	recordKustomizeK8sMetadata(ctx, fieldCountData.K8sMetadata)
	recordKustomizeHelmMetrics(ctx, fieldCountData.HelmMetrics)
	recordKustomizeBaseCount(ctx, fieldCountData.BaseCount)
	recordKustomizePatchCount(ctx, fieldCountData.PatchCount)
	recordKustomizeTopTierMetrics(ctx, fieldCountData.TopTierCount)
}

// RecordKustomizeResourceCount produces measurement for KustomizeResourceCount view
func RecordKustomizeResourceCount(ctx context.Context, resourceCount int) {
	klog.V(5).Infof("METRIC DEBUG: Recording KustomizeResourceCount: resourceCount=%d", resourceCount)
	KustomizeResourceCount.Record(ctx, int64(resourceCount))
}

// RecordKustomizeExecutionTime produces measurement for KustomizeExecutionTime view
func RecordKustomizeExecutionTime(ctx context.Context, executionTime float64) {
	klog.V(5).Infof("METRIC DEBUG: Recording KustomizeExecutionTime: executionTime=%.3fs", executionTime)
	KustomizeExecutionTime.Record(ctx, executionTime)
}

// recordKustomizeFieldCount produces measurement for KustomizeFieldCount view
func recordKustomizeFieldCount(ctx context.Context, fieldCount map[string]int) {
	for field, count := range fieldCount {
		attrs := []attribute.KeyValue{
			KeyFieldName.String(field),
		}
		KustomizeFieldCount.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

// recordKustomizeDeprecatingFields produces measurement for KustomizeDeprecatingMetrics view
func recordKustomizeDeprecatingFields(ctx context.Context, deprecationMetrics map[string]int) {
	for field, count := range deprecationMetrics {
		attrs := []attribute.KeyValue{
			KeyDeprecatingField.String(field),
		}
		KustomizeDeprecatingFields.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

// recordKustomizeSimplification produces measurement for KustomizeSimplification view
func recordKustomizeSimplification(ctx context.Context, simplMetrics map[string]int) {
	for field, count := range simplMetrics {
		attrs := []attribute.KeyValue{
			KeySimplificationField.String(field),
		}
		KustomizeSimplification.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

// recordKustomizeK8sMetadata produces measurement for KustomizeK8sMetadata view
func recordKustomizeK8sMetadata(ctx context.Context, k8sMetadata map[string]int) {
	for field, count := range k8sMetadata {
		attrs := []attribute.KeyValue{
			KeyK8sMetadataTransformer.String(field),
		}
		KustomizeK8sMetadata.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

// recordKustomizeHelmMetrics produces measurement for KustomizeHelmMetrics view
func recordKustomizeHelmMetrics(ctx context.Context, helmMetrics map[string]int) {
	for helmInflator, count := range helmMetrics {
		attrs := []attribute.KeyValue{
			KeyHelmInflator.String(helmInflator),
		}
		KustomizeHelmMetrics.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

// recordKustomizeBaseCount produces measurement for KustomizeBaseCount view
func recordKustomizeBaseCount(ctx context.Context, baseCount map[string]int) {
	for baseSource, count := range baseCount {
		attrs := []attribute.KeyValue{
			KeyBaseSource.String(baseSource),
		}
		KustomizeBaseCount.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

// recordKustomizePatchCount produces measurement for KustomizePatchCount view
func recordKustomizePatchCount(ctx context.Context, patchCount map[string]int) {
	for patchType, count := range patchCount {
		attrs := []attribute.KeyValue{
			KeyPatchField.String(patchType),
		}
		KustomizePatchCount.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

// recordKustomizeTopTierMetrics produces measurement for KustomizeTopTierMetrics view
func recordKustomizeTopTierMetrics(ctx context.Context, topTierCount map[string]int) {
	for field, count := range topTierCount {
		attrs := []attribute.KeyValue{
			KeyTopTierField.String(field),
		}
		KustomizeTopTierMetrics.Record(ctx, int64(count), metric.WithAttributes(attrs...))
	}
}

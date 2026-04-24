/*
Copyright 2020 Google LLC.

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

package metrics

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const (
	// RGReconcileDurationName is the name of resource group reconcile duration metric
	RGReconcileDurationName = "rg_reconcile_duration_seconds"
	// ResourceGroupTotalName is the name of resource group count metric
	ResourceGroupTotalName = "resource_group_total"
	// ResourceCountName is the name of resource count metric
	ResourceCountName = "resource_count"
	// ReadyResourceCountName is the name of ready resource count metric
	ReadyResourceCountName = "ready_resource_count"
	// KCCResourceCountName is the name of KCC resource count metric
	KCCResourceCountName = "kcc_resource_count"
	// NamespaceCountName is the name of namespace count metric
	NamespaceCountName = "resource_ns_count"
	// ClusterScopedResourceCountName is the name of cluster scoped resource count metric
	ClusterScopedResourceCountName = "cluster_scoped_resource_count"
	// CRDCountName is the name of CRD count metric
	CRDCountName = "crd_count"
	// PipelineErrorName is the name of pipeline error status metric (same as in Config Sync)
	PipelineErrorName = "pipeline_error_observed"
)

var (
	// ReconcileDuration tracks the time duration in seconds of reconciling
	// a ResourceGroup CR by the ResourceGroup controller.
	// label `reason`: the `Reason` field of the `Stalled` condition in a ResourceGroup CR.
	// reason can be: StartReconciling, FinishReconciling, ComponentFailed, ExceedTimeout.
	// This metric should be updated in the ResourceGroup controller.
	ReconcileDuration metric.Float64Histogram

	// ResourceGroupTotal tracks the total number of ResourceGroup CRs in a cluster.
	// This metric should be updated in the Root controller.
	ResourceGroupTotal metric.Int64Gauge

	// ResourceCount tracks the number of resources in a ResourceGroup CR.
	// This metric should be updated in the Root controller.
	ResourceCount metric.Int64Gauge

	// ReadyResourceCount tracks the number of resources with Current status in a ResourceGroup CR.
	// This metric should be updated in the ResourceGroup controller.
	ReadyResourceCount metric.Int64Gauge

	// KCCResourceCount tracks the number of KCC resources in a ResourceGroup CR.
	// This metric should be updated in the ResourceGroup controller.
	KCCResourceCount metric.Int64Gauge

	// NamespaceCount tracks the number of resource namespaces in a ResourceGroup CR.
	// This metric should be updated in the Root controller.
	NamespaceCount metric.Int64Gauge

	// ClusterScopedResourceCount tracks the number of cluster-scoped resources in a ResourceGroup CR.
	// This metric should be updated in the Root controller.
	ClusterScopedResourceCount metric.Int64Gauge

	// CRDCount tracks the number of CRDs in a ResourceGroup CR.
	// This metric should be updated in the Root controller.
	CRDCount metric.Int64Gauge

	// PipelineError tracks the error that happened when syncing a commit
	PipelineError metric.Int64Gauge
)

// InitializeOTelResourceGroupMetrics initializes OpenTelemetry Resource Group metrics instruments
func InitializeOTelResourceGroupMetrics() error {
	meter := otel.Meter("config-sync-resourcegroup")

	var err error

	// Initialize histogram instruments
	ReconcileDuration, err = meter.Float64Histogram(
		RGReconcileDurationName,
		metric.WithDescription("Time duration in seconds of reconciling a ResourceGroup CR by the ResourceGroup controller"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Initialize gauge instruments
	ResourceGroupTotal, err = meter.Int64Gauge(
		ResourceGroupTotalName,
		metric.WithDescription("Total number of ResourceGroup CRs in a cluster"),
	)
	if err != nil {
		return err
	}

	ResourceCount, err = meter.Int64Gauge(
		ResourceCountName,
		metric.WithDescription("The number of resources in a ResourceGroup CR"),
	)
	if err != nil {
		return err
	}

	ReadyResourceCount, err = meter.Int64Gauge(
		ReadyResourceCountName,
		metric.WithDescription("The number of resources with Current status in a ResourceGroup CR"),
	)
	if err != nil {
		return err
	}

	KCCResourceCount, err = meter.Int64Gauge(
		KCCResourceCountName,
		metric.WithDescription("The number of KCC resources in a ResourceGroup CR"),
	)
	if err != nil {
		return err
	}

	NamespaceCount, err = meter.Int64Gauge(
		NamespaceCountName,
		metric.WithDescription("The number of resource namespaces in a ResourceGroup CR"),
	)
	if err != nil {
		return err
	}

	ClusterScopedResourceCount, err = meter.Int64Gauge(
		ClusterScopedResourceCountName,
		metric.WithDescription("The number of cluster-scoped resources in a ResourceGroup CR"),
	)
	if err != nil {
		return err
	}

	CRDCount, err = meter.Int64Gauge(
		CRDCountName,
		metric.WithDescription("The number of CRDs in a ResourceGroup CR"),
	)
	if err != nil {
		return err
	}

	PipelineError, err = meter.Int64Gauge(
		PipelineErrorName,
		metric.WithDescription("A boolean value indicates if error happened at readiness stage when syncing a commit"),
	)
	if err != nil {
		return err
	}

	return nil
}

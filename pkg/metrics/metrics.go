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

package metrics

import (
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/klog/v2"
)

const (
	// APICallDurationName is the name of API duration metric
	APICallDurationName = "api_duration_seconds"
	// ReconcilerErrorsName is the name of reconciler error count metric
	ReconcilerErrorsName = "reconciler_errors"
	// PipelineErrorName is the name of pipeline error status metric.
	PipelineErrorName = "pipeline_error_observed"
	// ReconcileDurationName is the name of reconcile duration metric
	ReconcileDurationName = "reconcile_duration_seconds"
	// ParserDurationName is the name of parser duration metric
	ParserDurationName = "parser_duration_seconds"
	// LastSyncName is the name of last sync timestamp metric
	LastSyncName = "last_sync_timestamp"
	// DeclaredResourcesName is the name of declared resource count metric
	DeclaredResourcesName = "declared_resources"
	// ApplyOperationsName is the name of apply operations count metric
	ApplyOperationsName = "apply_operations_total"
	// ApplyDurationName is the name of apply duration metric
	ApplyDurationName = "apply_duration_seconds"
	// ResourceFightsName is the name of resource fight count metric
	ResourceFightsName = "resource_fights_total"
	// RemediateDurationName is the name of remediate duration metric
	RemediateDurationName = "remediate_duration_seconds"
	// LastApplyName is the name of last apply timestamp metric
	LastApplyName = "last_apply_timestamp"
	// ResourceConflictsName is the name of resource conflict count metric
	ResourceConflictsName = "resource_conflicts_total"
	// InternalErrorsName is the name of internal error count metric
	InternalErrorsName = "internal_errors_total"
)

var (
	// APICallDuration metric measures the latency of API server calls.
	APICallDuration metric.Float64Histogram

	// ReconcilerErrors metric measures the number of errors in the reconciler.
	ReconcilerErrors metric.Int64Gauge

	// PipelineError metric measures the error by components when syncing a commit.
	// Definition here must exactly match the definition in the resource-group
	// controller, or the Prometheus exporter will error. b/247516388
	// https://github.com/GoogleContainerTools/kpt-resource-group/blob/main/controllers/metrics/metrics.go#L88
	PipelineError metric.Int64Gauge

	// ReconcileDuration metric measures the latency of reconcile events.
	ReconcileDuration metric.Float64Histogram

	// ParserDuration metric measures the latency of the parse-apply-watch loop.
	ParserDuration metric.Float64Histogram

	// LastSync metric measures the timestamp of the latest Git sync.
	LastSync metric.Int64Gauge

	// DeclaredResources metric measures the number of declared resources parsed from Git.
	DeclaredResources metric.Int64Gauge

	// ApplyOperations metric measures the number of applier apply events.
	ApplyOperations metric.Int64Counter

	// ApplyDuration metric measures the latency of applier apply events.
	ApplyDuration metric.Float64Histogram

	// ResourceFights metric measures the number of resource fights.
	ResourceFights metric.Int64Counter

	// RemediateDuration metric measures the latency of remediator reconciliation events.
	RemediateDuration metric.Float64Histogram

	// LastApply metric measures the timestamp of the most recent applier apply event.
	LastApply metric.Int64Gauge

	// ResourceConflicts metric measures the number of resource conflicts.
	ResourceConflicts metric.Int64Counter

	// InternalErrors metric measures the number of unexpected internal errors triggered by defensive checks in Config Sync.
	InternalErrors metric.Int64Counter
)

var (
	// once ensures metrics are initialized only once
	once sync.Once
	// initErr stores any error that occurred during initialization
	initErr error
)

// lazyInit initializes metrics lazily using sync.Once
func lazyInit() error {
	once.Do(func() {
		initErr = initializeOTelMetrics()
	})
	return initErr
}

// InitializeOTelMetrics initializes OpenTelemetry metrics instruments
func InitializeOTelMetrics() error {
	return initializeOTelMetrics()
}

// initializeOTelMetrics contains the actual initialization logic
func initializeOTelMetrics() error {
	klog.V(5).Infof("METRIC DEBUG: Initializing OpenTelemetry metrics instruments")

	meter := otel.Meter("config-sync")
	klog.V(5).Infof("METRIC DEBUG: Created meter: config-sync")

	var err error

	// Initialize histogram instruments
	APICallDuration, err = meter.Float64Histogram(
		APICallDurationName,
		metric.WithDescription("The duration of API server calls in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		klog.V(5).ErrorS(err, "METRIC DEBUG: Failed to create APICallDuration histogram")
		return err
	}
	klog.V(5).Infof("METRIC DEBUG: Created APICallDuration histogram: %s", APICallDurationName)

	ReconcileDuration, err = meter.Float64Histogram(
		ReconcileDurationName,
		metric.WithDescription("The duration of reconcile events in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	ParserDuration, err = meter.Float64Histogram(
		ParserDurationName,
		metric.WithDescription("The duration of the parse-apply-watch loop in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	ApplyDuration, err = meter.Float64Histogram(
		ApplyDurationName,
		metric.WithDescription("The duration of applier events in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	RemediateDuration, err = meter.Float64Histogram(
		RemediateDurationName,
		metric.WithDescription("The duration of remediator reconciliation events"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Initialize gauge instruments
	ReconcilerErrors, err = meter.Int64Gauge(
		ReconcilerErrorsName,
		metric.WithDescription("The number of errors in the reconciler"),
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

	LastSync, err = meter.Int64Gauge(
		LastSyncName,
		metric.WithDescription("The timestamp of the most recent sync from Git"),
	)
	if err != nil {
		return err
	}

	DeclaredResources, err = meter.Int64Gauge(
		DeclaredResourcesName,
		metric.WithDescription("The number of declared resources parsed from Git"),
	)
	if err != nil {
		return err
	}

	LastApply, err = meter.Int64Gauge(
		LastApplyName,
		metric.WithDescription("The timestamp of the most recent applier event"),
	)
	if err != nil {
		return err
	}

	// Initialize counter instruments
	ApplyOperations, err = meter.Int64Counter(
		ApplyOperationsName,
		metric.WithDescription("The number of operations that have been performed to sync resources to source of truth"),
	)
	if err != nil {
		return err
	}

	ResourceFights, err = meter.Int64Counter(
		ResourceFightsName,
		metric.WithDescription("The number of resources that are being synced too frequently"),
	)
	if err != nil {
		return err
	}

	ResourceConflicts, err = meter.Int64Counter(
		ResourceConflictsName,
		metric.WithDescription("The number of resource conflicts resulting from a mismatch between the cached resources and cluster resources"),
	)
	if err != nil {
		return err
	}

	InternalErrors, err = meter.Int64Counter(
		InternalErrorsName,
		metric.WithDescription("The number of internal errors triggered by Config Sync"),
	)
	if err != nil {
		klog.V(5).ErrorS(err, "METRIC DEBUG: Failed to create InternalErrors counter")
		return err
	}
	klog.V(5).Infof("METRIC DEBUG: Created InternalErrors counter: %s", InternalErrorsName)

	klog.V(5).Infof("METRIC DEBUG: Successfully initialized all OpenTelemetry metrics instruments")
	return nil
}

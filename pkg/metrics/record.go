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
	"context"
	"os"
	"time"

	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/klog/v2"
)

// RecordAPICallDuration produces a measurement for the APICallDuration view.
func RecordAPICallDuration(ctx context.Context, operation, status string, startTime time.Time) {
	attrs := []attribute.KeyValue{
		KeyOperation.String(operation),
		KeyStatus.String(status),
	}
	duration := time.Since(startTime).Seconds()

	if APICallDuration == nil {
		panic("APICallDuration metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	APICallDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

	klog.V(5).Infof("METRIC DEBUG: Recorded APICallDuration - Name: %q, Value: %f, Operation: %q, Status: %q",
		APICallDurationName, duration, operation, status)
}

// RecordReconcilerErrors produces a measurement for the ReconcilerErrors view.
func RecordReconcilerErrors(ctx context.Context, component string, errs []v1beta1.ConfigSyncError) {
	if ReconcilerErrors == nil {
		panic("ReconcilerErrors metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	errorCountByClass := status.CountErrorByClass(errs)
	var supportedErrorClasses = []string{"1xxx", "2xxx", "9xxx"}

	klog.V(5).Infof("METRIC DEBUG: Recording ReconcilerErrors - Component: %q, Total errors: %d", component, len(errs))

	for _, errorclass := range supportedErrorClasses {
		var errorCount int64
		if v, ok := errorCountByClass[errorclass]; ok {
			errorCount = v
		}
		attrs := []attribute.KeyValue{
			KeyComponent.String(component),
			KeyErrorClass.String(errorclass),
		}
		ReconcilerErrors.Record(ctx, errorCount, metric.WithAttributes(attrs...))

		klog.V(5).Infof("METRIC DEBUG: Recorded ReconcilerErrors - Component: %q, ErrorClass: %q, Count: %d",
			component, errorclass, errorCount)
	}
}

// RecordPipelineError produces a measurement for the PipelineError view
func RecordPipelineError(ctx context.Context, reconcilerType, component string, errLen int) {
	if PipelineError == nil {
		panic("PipelineError metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	reconcilerName := os.Getenv(reconcilermanager.ReconcilerNameKey)
	attrs := []attribute.KeyValue{
		KeyName.String(reconcilerName),
		KeyReconcilerType.String(reconcilerType),
		KeyComponent.String(component),
	}
	var metricVal int64
	if errLen > 0 {
		metricVal = 1
	} else {
		metricVal = 0
	}
	PipelineError.Record(ctx, metricVal, metric.WithAttributes(attrs...))

	klog.V(5).Infof("METRIC DEBUG: Recorded PipelineError - ReconcilerType: %q, Component: %q, ReconcilerName: %q, ErrorLen: %d, Value: %d",
		reconcilerType, component, reconcilerName, errLen, metricVal)
}

// RecordReconcileDuration produces a measurement for the ReconcileDuration view.
func RecordReconcileDuration(ctx context.Context, status string, startTime time.Time) {
	if ReconcileDuration == nil {
		panic("ReconcileDuration metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyStatus.String(status),
	}
	duration := time.Since(startTime).Seconds()
	klog.V(5).Infof("METRIC DEBUG: Recording ReconcileDuration: status=%s, duration=%.3fs", status, duration)
	ReconcileDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
}

// RecordParserDuration produces a measurement for the ParserDuration view.
func RecordParserDuration(ctx context.Context, trigger, source, status string, startTime time.Time) {
	if ParserDuration == nil {
		panic("ParserDuration metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyStatus.String(status),
		KeyTrigger.String(trigger),
		KeyParserSource.String(source),
	}
	duration := time.Since(startTime).Seconds()
	klog.V(5).Infof("METRIC DEBUG: Recording ParserDuration: trigger=%s, source=%s, status=%s, duration=%.3fs", trigger, source, status, duration)
	ParserDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
}

// RecordLastSync produces a measurement for the LastSync view.
func RecordLastSync(ctx context.Context, status, commit string, timestamp time.Time) {
	if LastSync == nil {
		panic("LastSync metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyStatus.String(status),
		KeyCommit.String(commit),
	}
	klog.V(5).Infof("METRIC DEBUG: Recording LastSync: status=%s, commit=%s, timestamp=%d", status, commit, timestamp.Unix())
	LastSync.Record(ctx, timestamp.Unix(), metric.WithAttributes(attrs...))
}

// RecordDeclaredResources produces a measurement for the DeclaredResources view.
func RecordDeclaredResources(ctx context.Context, commit string, numResources int) {
	if DeclaredResources == nil {
		panic("DeclaredResources metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyCommit.String(commit),
	}
	klog.V(5).Infof("METRIC DEBUG: Recording DeclaredResources: commit=%s, numResources=%d", commit, numResources)
	DeclaredResources.Record(ctx, int64(numResources), metric.WithAttributes(attrs...))
}

// RecordApplyOperation produces a measurement for the ApplyOperations view.
func RecordApplyOperation(ctx context.Context, controller, operation, status string) {
	if ApplyOperations == nil {
		panic("ApplyOperations metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyOperation.String(operation),
		KeyController.String(controller),
		KeyStatus.String(status),
	}
	klog.V(5).Infof("METRIC DEBUG: Recording ApplyOperation: controller=%s, operation=%s, status=%s", controller, operation, status)
	ApplyOperations.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordApplyDuration produces measurements for the ApplyDuration and LastApplyTimestamp views.
func RecordApplyDuration(ctx context.Context, status, commit string, startTime time.Time) {
	// Fail fast if metrics are not initialized - this enforces that InitializeOTelMetrics is called
	if ApplyDuration == nil {
		panic("ApplyDuration metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}
	if LastApply == nil {
		panic("LastApply metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	if commit == "" {
		// TODO: Remove default value when otel-collector supports empty tag values correctly.
		commit = CommitNone
	}
	now := time.Now()
	attrs := []attribute.KeyValue{
		KeyStatus.String(status),
		KeyCommit.String(commit),
	}

	duration := now.Sub(startTime).Seconds()
	klog.V(5).Infof("METRIC DEBUG: Recording ApplyDuration: status=%s, commit=%s, duration=%.3fs", status, commit, duration)
	ApplyDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
	LastApply.Record(ctx, now.Unix(), metric.WithAttributes(attrs...))
}

// RecordResourceFight produces measurements for the ResourceFights view.
func RecordResourceFight(ctx context.Context, _ string) {
	if ResourceFights == nil {
		panic("ResourceFights metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	klog.V(5).Infof("METRIC DEBUG: Recording ResourceFight")
	ResourceFights.Add(ctx, 1)
}

// RecordRemediateDuration produces measurements for the RemediateDuration view.
func RecordRemediateDuration(ctx context.Context, status string, startTime time.Time) {
	if RemediateDuration == nil {
		panic("RemediateDuration metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyStatus.String(status),
	}
	duration := time.Since(startTime).Seconds()
	klog.V(5).Infof("METRIC DEBUG: Recording RemediateDuration: status=%s, duration=%.3fs", status, duration)
	RemediateDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
}

// RecordResourceConflict produces measurements for the ResourceConflicts view.
func RecordResourceConflict(ctx context.Context, commit string) {
	if ResourceConflicts == nil {
		panic("ResourceConflicts metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyCommit.String(commit),
	}
	klog.V(5).Infof("METRIC DEBUG: Recording ResourceConflict: commit=%s", commit)
	ResourceConflicts.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordInternalError produces measurements for the InternalErrors view.
func RecordInternalError(ctx context.Context, source string) {
	if InternalErrors == nil {
		panic("InternalErrors metric is not initialized! Call InitializeOTelMetrics() before recording metrics.")
	}

	attrs := []attribute.KeyValue{
		KeyInternalErrorSource.String(source),
	}
	klog.V(5).Infof("METRIC DEBUG: Recording InternalError: source=%s", source)
	InternalErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
}

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
	"context"
	"time"

	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// RecordReconcileDuration produces a measurement for the ReconcileDuration view.
func RecordReconcileDuration(ctx context.Context, stallStatus string, startTime time.Time) {
	attrs := []attribute.KeyValue{
		KeyStallReason.String(stallStatus),
	}
	duration := time.Since(startTime).Seconds()
	ReconcileDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
}

// RecordReadyResourceCount produces a measurement for the ReadyResourceCount view.
func RecordReadyResourceCount(ctx context.Context, nn types.NamespacedName, count int64) {
	attrs := []attribute.KeyValue{
		KeyResourceGroup.String(nn.String()),
	}
	ReadyResourceCount.Record(ctx, count, metric.WithAttributes(attrs...))
}

// RecordKCCResourceCount produces a measurement for the KCCResourceCount view.
func RecordKCCResourceCount(ctx context.Context, nn types.NamespacedName, count int64) {
	attrs := []attribute.KeyValue{
		KeyResourceGroup.String(nn.String()),
	}
	KCCResourceCount.Record(ctx, count, metric.WithAttributes(attrs...))
}

// RecordResourceCount produces a measurement for the ResourceCount view.
func RecordResourceCount(ctx context.Context, nn types.NamespacedName, count int64) {
	attrs := []attribute.KeyValue{
		KeyResourceGroup.String(nn.String()),
	}
	ResourceCount.Record(ctx, count, metric.WithAttributes(attrs...))
}

// RecordResourceGroupTotal produces a measurement for the ResourceGroupTotalView
func RecordResourceGroupTotal(ctx context.Context, count int64) {
	ResourceGroupTotal.Record(ctx, count)
}

// RecordNamespaceCount produces a measurement for the NamespaceCount view.
func RecordNamespaceCount(ctx context.Context, nn types.NamespacedName, count int64) {
	attrs := []attribute.KeyValue{
		KeyResourceGroup.String(nn.String()),
	}
	NamespaceCount.Record(ctx, count, metric.WithAttributes(attrs...))
}

// RecordClusterScopedResourceCount produces a measurement for ClusterScopedResourceCount view
func RecordClusterScopedResourceCount(ctx context.Context, nn types.NamespacedName, count int64) {
	attrs := []attribute.KeyValue{
		KeyResourceGroup.String(nn.String()),
	}
	ClusterScopedResourceCount.Record(ctx, count, metric.WithAttributes(attrs...))
}

// RecordCRDCount produces a measurement for RecordCRDCount view
func RecordCRDCount(ctx context.Context, nn types.NamespacedName, count int64) {
	attrs := []attribute.KeyValue{
		KeyResourceGroup.String(nn.String()),
	}
	CRDCount.Record(ctx, count, metric.WithAttributes(attrs...))
}

// RecordPipelineError produces a measurement for PipelineErrorView
func RecordPipelineError(ctx context.Context, nn types.NamespacedName, component string, hasErr bool) {
	reconcilerName, reconcilerType := ComputeReconcilerNameType(nn)
	attrs := []attribute.KeyValue{
		KeyComponent.String(component),
		KeyName.String(reconcilerName),
		KeyType.String(reconcilerType),
	}
	var metricVal int64
	if hasErr {
		metricVal = 1
	} else {
		metricVal = 0
	}
	PipelineError.Record(ctx, metricVal, metric.WithAttributes(attrs...))
	klog.Infof("Recording %s metric at component: %s, namespace: %s, reconciler: %s, sync type: %s with value %v",
		PipelineErrorName, component, nn.Namespace, reconcilerName, nn.Name, metricVal)
}

// ComputeReconcilerNameType computes the reconciler name from the ResourceGroup CR name
func ComputeReconcilerNameType(nn types.NamespacedName) (reconcilerName, reconcilerType string) {
	if nn.Namespace == configsync.ControllerNamespace {
		return core.RootReconcilerName(nn.Name), configsync.RootSyncName
	}
	return core.NsReconcilerName(nn.Namespace, nn.Name), configsync.RepoSyncName
}

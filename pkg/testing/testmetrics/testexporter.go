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

package testmetrics

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/GoogleContainerTools/config-sync/pkg/kmetrics"
	"github.com/GoogleContainerTools/config-sync/pkg/metrics"
	rgmetrics "github.com/GoogleContainerTools/config-sync/pkg/resourcegroup/controllers/metrics"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// MetricData represents a metric measurement for testing.
type MetricData struct {
	Name   string
	Value  float64
	Labels map[string]string
}

// TestExporter captures and validates OpenTelemetry metrics in tests.
type TestExporter struct {
	metrics []MetricData
	reader  sdkmetric.Reader
	mutex   sync.Mutex
}

// Mutex to protect metrics initialization
var metricsInitMutex sync.Mutex

// NewTestExporter creates a new test exporter and initializes the metrics system.
// This function resets and initializes the metrics system, then creates an exporter.
func NewTestExporter() *TestExporter {
	metricsInitMutex.Lock()
	defer metricsInitMutex.Unlock()

	reader := initOtelReaderAndProvider()
	initMetrics()

	return &TestExporter{
		metrics: make([]MetricData, 0),
		reader:  reader,
	}
}

// ResetGlobalMetrics resets the global metrics state for testing.
// This function completely resets and re-initializes the metrics system.
func ResetGlobalMetrics() {
	metricsInitMutex.Lock()
	defer metricsInitMutex.Unlock()

	_ = initOtelReaderAndProvider()
	initMetrics()
}

// CollectMetrics collects all OpenTelemetry metrics and stores them in a simple format.
func (e *TestExporter) CollectMetrics(ctx context.Context) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.reader == nil {
		return fmt.Errorf("no reader configured")
	}

	e.metrics = make([]MetricData, 0)

	var rm metricdata.ResourceMetrics
	if err := e.reader.Collect(ctx, &rm); err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, metric := range sm.Metrics {
			e.convertMetricToSimpleFormat(metric)
		}
	}

	return nil
}

// GetMetrics returns all collected metrics.
func (e *TestExporter) GetMetrics() []MetricData {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return append([]MetricData(nil), e.metrics...)
}

// ClearMetrics clears all collected metrics for this test exporter.
func (e *TestExporter) ClearMetrics() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.metrics = make([]MetricData, 0)
}

// ValidateMetrics compares collected metrics with expected values.
func (e *TestExporter) ValidateMetrics(expected []MetricData) string {
	if err := e.CollectMetrics(context.Background()); err != nil {
		return fmt.Sprintf("Failed to collect metrics: %v", err)
	}

	got := e.GetMetrics()

	if len(expected) > 0 {
		expectedMetricNames := make(map[string]bool)
		for _, expectedMetric := range expected {
			expectedMetricNames[expectedMetric.Name] = true
		}

		var filteredGot []MetricData
		for _, metric := range got {
			if expectedMetricNames[metric.Name] {
				filteredGot = append(filteredGot, metric)
			}
		}
		got = filteredGot
	}

	return diffMetrics(got, expected)
}

// convertMetricToSimpleFormat converts OpenTelemetry metrics to simple MetricData.
func (e *TestExporter) convertMetricToSimpleFormat(metric metricdata.Metrics) {
	switch data := metric.Data.(type) {
	case metricdata.Sum[int64]:
		for _, point := range data.DataPoints {
			e.addMetric(metric.Name, float64(point.Value), point.Attributes)
		}
	case metricdata.Sum[float64]:
		for _, point := range data.DataPoints {
			e.addMetric(metric.Name, point.Value, point.Attributes)
		}
	case metricdata.Gauge[int64]:
		for _, point := range data.DataPoints {
			e.addMetric(metric.Name, float64(point.Value), point.Attributes)
		}
	case metricdata.Gauge[float64]:
		for _, point := range data.DataPoints {
			e.addMetric(metric.Name, point.Value, point.Attributes)
		}
	case metricdata.Histogram[float64]:
		for _, point := range data.DataPoints {
			e.addMetric(metric.Name, point.Sum, point.Attributes)
		}
	case metricdata.Histogram[int64]:
		for _, point := range data.DataPoints {
			e.addMetric(metric.Name, float64(point.Sum), point.Attributes)
		}
	default:
		fmt.Printf("Warning: Unsupported metric type: %T\n", data)
	}
}

// addMetric adds a metric to the collection.
func (e *TestExporter) addMetric(name string, value float64, attrs attribute.Set) {
	e.metrics = append(e.metrics, MetricData{
		Name:   name,
		Value:  value,
		Labels: e.attributesToMap(attrs),
	})
}

// attributesToMap converts OpenTelemetry attributes to a simple map.
func (e *TestExporter) attributesToMap(attrs attribute.Set) map[string]string {
	result := make(map[string]string)
	iter := attrs.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		result[string(kv.Key)] = kv.Value.AsString()
	}
	return result
}

// diffMetrics compares collected metrics with expected values.
func diffMetrics(got, want []MetricData) string {
	sort.Slice(got, func(i, j int) bool {
		return got[i].Name < got[j].Name || (got[i].Name == got[j].Name && fmt.Sprintf("%v", got[i].Labels) < fmt.Sprintf("%v", got[j].Labels))
	})
	sort.Slice(want, func(i, j int) bool {
		return want[i].Name < want[j].Name || (want[i].Name == want[j].Name && fmt.Sprintf("%v", want[i].Labels) < fmt.Sprintf("%v", want[j].Labels))
	})

	if len(got) != len(want) {
		return fmt.Sprintf("Expected %d metrics, got %d", len(want), len(got))
	}

	for i := range got {
		if !cmp.Equal(got[i], want[i], cmpopts.IgnoreTypes(time.Time{})) {
			return fmt.Sprintf("Metric mismatch at index %d:\n- %+v\n+ %+v", i, want[i], got[i])
		}
	}

	return ""
}

// initOtelReaderAndProvider initializes OpenTelemetry reader and meter provider.
func initOtelReaderAndProvider() sdkmetric.Reader {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewSchemaless()),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(meterProvider)
	return reader
}

// initMetrics initializes all metric systems.
func initMetrics() {
	if err := metrics.InitializeOTelMetrics(); err != nil {
		panic(fmt.Sprintf("Failed to initialize OTel metrics: %v", err))
	}
	if err := rgmetrics.InitializeOTelResourceGroupMetrics(); err != nil {
		panic(fmt.Sprintf("Failed to initialize OTel resource group metrics: %v", err))
	}
	if err := kmetrics.InitializeOTelKustomizeMetrics(); err != nil {
		panic(fmt.Sprintf("Failed to initialize OTel kustomize metrics: %v", err))
	}
}

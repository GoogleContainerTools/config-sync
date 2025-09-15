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

// MetricData represents a simple metric measurement for testing
type MetricData struct {
	Name   string
	Value  float64
	Labels map[string]string
}

// TestExporter provides a simple way to capture and validate metrics in tests
type TestExporter struct {
	metrics []MetricData
	reader  sdkmetric.Reader
	mutex   sync.Mutex
}

// Global flag to track if metrics have been initialized for testing
var (
	metricsInitialized bool
	metricsInitMutex   sync.Mutex
)

// initializeMetricsForTesting initializes OpenTelemetry metrics for testing
func initializeMetricsForTesting() error {
	metricsInitMutex.Lock()
	defer metricsInitMutex.Unlock()

	if metricsInitialized {
		return nil // Already initialized
	}

	// Create a manual reader to capture metrics
	globalTestReader = sdkmetric.NewManualReader()

	// Set up a meter provider for testing with our reader
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewSchemaless()),
		sdkmetric.WithReader(globalTestReader),
	)
	otel.SetMeterProvider(meterProvider)

	// Call the actual metrics initialization function from the metrics package
	if err := metrics.InitializeOTelMetrics(); err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry metrics: %w", err)
	}

	// Initialize resourcegroup metrics
	if err := rgmetrics.InitializeOTelResourceGroupMetrics(); err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry resourcegroup metrics: %w", err)
	}

	// Initialize kustomize metrics
	if err := kmetrics.InitializeOTelKustomizeMetrics(); err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry kustomize metrics: %w", err)
	}

	metricsInitialized = true
	return nil
}

// CollectMetrics collects all OpenTelemetry metrics and stores them in a simple format
func (e *TestExporter) CollectMetrics() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.reader == nil {
		return fmt.Errorf("no reader configured")
	}

	// Clear existing metrics before collecting new ones
	e.metrics = make([]MetricData, 0)

	ctx := context.Background()
	var rm metricdata.ResourceMetrics
	if err := e.reader.Collect(ctx, &rm); err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Convert OpenTelemetry metrics to simple MetricData format
	for _, sm := range rm.ScopeMetrics {
		for _, metric := range sm.Metrics {
			e.convertMetricToSimpleFormat(metric)
		}
	}

	return nil
}

// convertMetricToSimpleFormat converts OpenTelemetry metrics to simple MetricData
func (e *TestExporter) convertMetricToSimpleFormat(metric metricdata.Metrics) {
	switch data := metric.Data.(type) {
	case metricdata.Sum[int64]:
		for _, point := range data.DataPoints {
			e.metrics = append(e.metrics, MetricData{
				Name:   metric.Name,
				Value:  float64(point.Value),
				Labels: e.attributesToMap(point.Attributes),
			})
		}
	case metricdata.Sum[float64]:
		for _, point := range data.DataPoints {
			e.metrics = append(e.metrics, MetricData{
				Name:   metric.Name,
				Value:  point.Value,
				Labels: e.attributesToMap(point.Attributes),
			})
		}
	case metricdata.Gauge[int64]:
		for _, point := range data.DataPoints {
			e.metrics = append(e.metrics, MetricData{
				Name:   metric.Name,
				Value:  float64(point.Value),
				Labels: e.attributesToMap(point.Attributes),
			})
		}
	case metricdata.Gauge[float64]:
		for _, point := range data.DataPoints {
			e.metrics = append(e.metrics, MetricData{
				Name:   metric.Name,
				Value:  point.Value,
				Labels: e.attributesToMap(point.Attributes),
			})
		}
	case metricdata.Histogram[float64]:
		for _, point := range data.DataPoints {
			e.metrics = append(e.metrics, MetricData{
				Name:   metric.Name,
				Value:  point.Sum,
				Labels: e.attributesToMap(point.Attributes),
			})
		}
	case metricdata.Histogram[int64]:
		for _, point := range data.DataPoints {
			e.metrics = append(e.metrics, MetricData{
				Name:   metric.Name,
				Value:  float64(point.Sum),
				Labels: e.attributesToMap(point.Attributes),
			})
		}
	// Add more cases as needed for other metric types
	default:
		fmt.Printf("Warning: Unsupported metric type: %T\n", data)
	}
}

// attributesToMap converts OpenTelemetry attributes to a simple map
func (e *TestExporter) attributesToMap(attrs attribute.Set) map[string]string {
	result := make(map[string]string)
	iter := attrs.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		result[string(kv.Key)] = kv.Value.AsString()
	}
	return result
}

// Global test reader to capture metrics
var globalTestReader sdkmetric.Reader

// NewTestExporter creates a new test exporter for capturing metrics
func NewTestExporter() *TestExporter {
	// Initialize OpenTelemetry metrics to ensure they're available for the code being tested
	if err := initializeMetricsForTesting(); err != nil {
		fmt.Printf("Warning: Failed to initialize OpenTelemetry metrics for testing: %v\n", err)
	}

	// Use the global test reader that was set up during initialization
	return &TestExporter{
		metrics: make([]MetricData, 0),
		reader:  globalTestReader,
	}
}

// GetMetrics returns all collected metrics
func (e *TestExporter) GetMetrics() []MetricData {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return append([]MetricData(nil), e.metrics...)
}

// ClearMetrics clears all collected metrics for this test exporter
func (e *TestExporter) ClearMetrics() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.metrics = make([]MetricData, 0)
}

// ResetGlobalMetrics resets the global metrics state for testing
func ResetGlobalMetrics() {
	// Create a fresh manual reader to reset the metrics collection
	globalTestReader = sdkmetric.NewManualReader()

	// Set up a fresh meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewSchemaless()),
		sdkmetric.WithReader(globalTestReader),
	)
	otel.SetMeterProvider(meterProvider)

	// Re-initialize all metrics with the new meter provider
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

// ValidateMetrics compares collected metrics with expected values
func (e *TestExporter) ValidateMetrics(expected []MetricData) string {
	// Collect current metrics
	if err := e.CollectMetrics(); err != nil {
		return fmt.Sprintf("Failed to collect metrics: %v", err)
	}

	got := e.GetMetrics()

	// Filter metrics to only include the ones we're testing for
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

// diffMetrics compares collected metrics with expected values
func diffMetrics(got, want []MetricData) string {
	// Sort both slices for consistent comparison
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

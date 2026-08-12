// Copyright 2026 Google LLC
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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestInitializeOTelKustomizeMetrics_HistogramBuckets(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewSchemaless()),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(meterProvider)

	if err := InitializeOTelKustomizeMetrics(); err != nil {
		t.Fatalf("InitializeOTelKustomizeMetrics() failed: %v", err)
	}

	ctx := t.Context()
	// Record a 350ms build duration in milliseconds
	executionTime := 350 * time.Millisecond
	RecordKustomizeExecutionTime(ctx, float64(executionTime.Milliseconds()))

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("reader.Collect() failed: %v", err)
	}

	var foundPoint *metricdata.HistogramDataPoint[float64]
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "kustomize_build_latency" {
				if hist, ok := m.Data.(metricdata.Histogram[float64]); ok && len(hist.DataPoints) > 0 {
					foundPoint = &hist.DataPoints[0]
				}
			}
		}
	}

	if foundPoint == nil {
		t.Fatalf("Histogram %q was not found in collected metrics", "kustomize_build_latency")
	}

	if diff := cmp.Diff(KustomizeBuildLatencyBounds, foundPoint.Bounds); diff != "" {
		t.Errorf("Histogram %q bucket boundaries mismatch (-want +got):\n%s", "kustomize_build_latency", diff)
	}

	// For 350ms and KustomizeBuildLatencyBounds [0, 10, 20, 40, 80, 160, 320, 640, ...]:
	// Index 7: (320, 640]  <-- 350ms must fall in this bucket
	if len(foundPoint.BucketCounts) <= 7 {
		t.Fatalf("Expected at least 8 bucket counts, got %d", len(foundPoint.BucketCounts))
	}
	if foundPoint.BucketCounts[7] != 1 {
		t.Errorf("BucketCounts[7] (interval (320, 640]) = %d, want 1. Full BucketCounts: %v",
			foundPoint.BucketCounts[7], foundPoint.BucketCounts)
	}

	// Verify the overflow (+Inf) bucket is empty. Before the fix, durations were
	// recorded in nanoseconds against millisecond-scale default buckets, so every
	// recording landed in the overflow bucket.
	overflowIdx := len(foundPoint.BucketCounts) - 1
	if foundPoint.BucketCounts[overflowIdx] != 0 {
		t.Errorf("BucketCounts[%d] (+Inf overflow bucket) = %d, want 0", overflowIdx, foundPoint.BucketCounts[overflowIdx])
	}
}

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

package metrics

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestInitializeOTelMetrics_HistogramBuckets(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewSchemaless()),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(meterProvider)

	if err := InitializeOTelMetrics(); err != nil {
		t.Fatalf("InitializeOTelMetrics() failed: %v", err)
	}

	ctx := t.Context()
	startTime := time.Now().Add(-350 * time.Millisecond)

	RecordAPICallDuration(ctx, "get", "success", startTime)
	RecordReconcileDuration(ctx, "success", startTime)
	RecordParserDuration(ctx, "git", "root", "success", startTime)
	RecordApplyDuration(ctx, "success", "abcdef", startTime)
	RecordRemediateDuration(ctx, "success", startTime)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("reader.Collect() failed: %v", err)
	}

	expectedHistograms := map[string][]float64{
		APICallDurationName:   DistributionBounds,
		ReconcileDurationName: DistributionBounds,
		ParserDurationName:    LongDistributionBounds,
		ApplyDurationName:     LongDistributionBounds,
		RemediateDurationName: DistributionBounds,
	}

	foundHistograms := make(map[string][]float64)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if hist, ok := m.Data.(metricdata.Histogram[float64]); ok {
				if len(hist.DataPoints) > 0 {
					foundHistograms[m.Name] = hist.DataPoints[0].Bounds
				}
			}
		}
	}

	for metricName, wantBounds := range expectedHistograms {
		t.Run(metricName, func(t *testing.T) {
			gotBounds, ok := foundHistograms[metricName]
			if !ok {
				t.Fatalf("Histogram %q was not found in collected metrics", metricName)
			}
			if diff := cmp.Diff(wantBounds, gotBounds); diff != "" {
				t.Errorf("Histogram %q bucket boundaries mismatch (-want +got):\n%s", metricName, diff)
			}
		})
	}
}

func TestRecordDuration_SubsecondBucketPlacement(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewSchemaless()),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(meterProvider)

	if err := InitializeOTelMetrics(); err != nil {
		t.Fatalf("InitializeOTelMetrics() failed: %v", err)
	}

	ctx := t.Context()
	// Record an exact 350ms duration (0.35s)
	startTime := time.Now().Add(-350 * time.Millisecond)
	RecordAPICallDuration(ctx, "get", "success", startTime)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("reader.Collect() failed: %v", err)
	}

	var foundPoint *metricdata.HistogramDataPoint[float64]
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == APICallDurationName {
				if hist, ok := m.Data.(metricdata.Histogram[float64]); ok && len(hist.DataPoints) > 0 {
					foundPoint = &hist.DataPoints[0]
				}
			}
		}
	}

	if foundPoint == nil {
		t.Fatalf("Metric %q was not recorded", APICallDurationName)
	}

	// For 0.35s and DistributionBounds [.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10]:
	// Index 0: <= 0.005
	// Index 1: (0.005, 0.01]
	// Index 2: (0.01, 0.025]
	// Index 3: (0.025, 0.05]
	// Index 4: (0.05, 0.1]
	// Index 5: (0.1, 0.25]
	// Index 6: (0.25, 0.5]  <-- 0.35s must fall in this sub-second bucket
	// Index 9: (2.5, 5.0]  <-- Before fix, this was where it fell (le=5)
	if len(foundPoint.BucketCounts) <= 6 {
		t.Fatalf("Expected at least 7 bucket counts, got %d", len(foundPoint.BucketCounts))
	}

	if foundPoint.BucketCounts[6] != 1 {
		t.Errorf("BucketCounts[6] (interval (0.25, 0.5]) = %d, want 1. Full BucketCounts: %v",
			foundPoint.BucketCounts[6], foundPoint.BucketCounts)
	}

	// Verify that the coarse 5-second bucket (index 9) is 0, confirming it didn't fall into the 5s bucket
	if foundPoint.BucketCounts[9] != 0 {
		t.Errorf("BucketCounts[9] (interval (2.5, 5.0]) = %d, want 0", foundPoint.BucketCounts[9])
	}
}

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

func TestInitializeOTelResourceGroupMetrics_HistogramBuckets(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewSchemaless()),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(meterProvider)

	if err := InitializeOTelResourceGroupMetrics(); err != nil {
		t.Fatalf("InitializeOTelResourceGroupMetrics() failed: %v", err)
	}

	ctx := t.Context()
	startTime := time.Now().Add(-200 * time.Millisecond)

	RecordReconcileDuration(ctx, "FinishReconciling", startTime)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("reader.Collect() failed: %v", err)
	}

	var gotBounds []float64
	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == RGReconcileDurationName {
				if hist, ok := m.Data.(metricdata.Histogram[float64]); ok && len(hist.DataPoints) > 0 {
					gotBounds = hist.DataPoints[0].Bounds
					found = true
				}
			}
		}
	}

	if !found {
		t.Fatalf("Histogram %q was not found in collected metrics", RGReconcileDurationName)
	}

	if diff := cmp.Diff(RGReconcileDurationBounds, gotBounds); diff != "" {
		t.Errorf("Histogram %q bucket boundaries mismatch (-want +got):\n%s", RGReconcileDurationName, diff)
	}
}

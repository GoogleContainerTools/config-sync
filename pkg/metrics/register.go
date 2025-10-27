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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"k8s.io/klog/v2"
)

const (
	// ShutdownTimeout is the timeout for shutting down the Otel exporter
	ShutdownTimeout = 5 * time.Second
)

// RegisterOTelExporter creates the OTLP metrics exporter.
func RegisterOTelExporter(ctx context.Context, containerName string) (*otlpmetricgrpc.Exporter, error) {

	klog.V(5).Infof("METRIC DEBUG: Registering OTLP exporter for container: %q", containerName)
	err := os.Setenv(
		"OTEL_RESOURCE_ATTRIBUTES",
		"k8s.container.name=\""+containerName+"\"")
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
	)
	if err != nil {
		klog.V(5).ErrorS(err, "METRIC DEBUG: Failed to create resource")
		return nil, err
	}

	// Create OTLP exporter
	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint("localhost:4317"),
	)
	if err != nil {
		klog.V(5).ErrorS(err, "METRIC DEBUG: Failed to create OTLP exporter")
		return nil, err
	}
	klog.V(5).Infof("METRIC DEBUG: Created OTLP exporter with endpoint: otel-collector.config-management-monitoring:4317")

	// Create meter provider
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter)),
		metric.WithResource(res),
	)
	klog.V(5).Infof("METRIC DEBUG: Created meter provider with periodic reader")

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)
	klog.V(5).Infof("METRIC DEBUG: Set global meter provider")

	err = InitializeOTelMetrics()
	if err != nil {
		return nil, err
	}

	return exporter, nil
}

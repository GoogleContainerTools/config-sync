// Copyright 2024 Google LLC
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

package metrics

import (
	"context"
	"os"
	"testing"
)

func TestRegisterOTelExporter_DisableMonitoring(t *testing.T) {
	// Save the original value and defer restoration
	original := os.Getenv("DISABLE_MONITORING")
	defer func() {
		if err := os.Setenv("DISABLE_MONITORING", original); err != nil {
			t.Errorf("failed to restore DISABLE_MONITORING env var: %v", err)
		}
	}()

	err := os.Setenv("DISABLE_MONITORING", "true")
	if err != nil {
		t.Fatalf("failed to set DISABLE_MONITORING env var: %v", err)
	}

	ctx := context.Background()
	exporter, err := RegisterOTelExporter(ctx, "test-container")
	if err != nil {
		t.Fatalf("expected no error when monitoring is disabled, got %v", err)
	}

	if exporter != nil {
		t.Fatalf("expected exporter to be nil when monitoring is disabled")
	}

	// In the real code we wrap exporter.Shutdown in 'if exporter != nil' to prevent panics.
	// We simulate that fix here to ensure we don't panic on Shutdown.
	func() {
		if exporter != nil {
			if err := exporter.Shutdown(ctx); err != nil {
				t.Errorf("expected no error on shutdown")
			}
		}
	}()
}

// Copyright 2023 Google LLC
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

package controllers

import (
	"os"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/testing/testcontroller"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/testmetrics"
	"k8s.io/klog/v2"
)

// TestMain executes the tests for this package, with optional logging.
// To see all logs, use:
// go test github.com/GoogleContainerTools/config-sync/pkg/reconcilermanager/controllers -v -args -v=5
func TestMain(m *testing.M) {
	setup := func() error {
		klog.InitFlags(nil)

		// Initialize metrics for tests
		_, err := testmetrics.NewTestExporter()
		if err != nil {
			return err
		}

		return nil
	}

	cleanup := func() error {
		return nil
	}

	os.Exit(testcontroller.RunTestSuite(m, setup, cleanup))
}

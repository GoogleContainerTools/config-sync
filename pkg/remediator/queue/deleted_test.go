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

package queue

import (
	"context"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/metrics"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/testmetrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestWasDeleted(t *testing.T) {
	testCases := []struct {
		name string
		obj  client.Object
	}{
		{
			"object with no annotations",
			k8sobjects.ConfigMapObject(),
		},
		{
			"object with an annotation",
			k8sobjects.ConfigMapObject(core.Annotation("hello", "world")),
		},
		{
			"object with explicitly empty annotations",
			k8sobjects.ConfigMapObject(core.Annotations(map[string]string{})),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// First verify that the object is not detected as deleted.
			ctx := context.Background()
			if WasDeleted(ctx, tc.obj) {
				t.Errorf("object was incorrectly detected as deleted: %v", tc.obj)
			}
			// Next mark the object as deleted and verify that it is now detected.
			deletedObj := MarkDeleted(ctx, tc.obj)
			if !WasDeleted(ctx, deletedObj) {
				t.Errorf("deleted object was not detected: %v", tc.obj)
			}
		})
	}
}

func TestDeleted_InternalErrorMetricValidation(t *testing.T) {
	exporter := testmetrics.NewTestExporter()
	ctx := context.Background()
	MarkDeleted(ctx, nil)

	expectedMetrics := []testmetrics.MetricData{
		{
			Name:   metrics.InternalErrorsName,
			Value:  1,
			Labels: map[string]string{"source": "remediator"},
		},
	}

	if diff := exporter.ValidateMetrics(expectedMetrics); diff != "" {
		t.Error(diff)
	}
}

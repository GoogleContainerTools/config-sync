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
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync/v1beta1"
	"go.opentelemetry.io/otel/attribute"
)

var (
	// KeyName groups metrics by the reconciler name. Possible values: root-reconciler, ns-reconciler-<namespace>
	// TODO b/208316928 remove this key from pipeline_error_observed metric once same metric in Resource Group Controller has this tag removed
	KeyName = attribute.Key("name")

	// KeyReconcilerType groups metrics by the reconciler type. Possible values: root, namespace.
	// TODO: replace with configsync.sync.kind resource attribute
	KeyReconcilerType = attribute.Key("reconciler")

	// KeyOperation groups metrics by their operation. Possible values: create, patch, update, delete.
	KeyOperation = attribute.Key("operation")

	// KeyController groups metrics by their controller. Possible values: applier, remediator.
	KeyController = attribute.Key("controller")

	// KeyComponent groups metrics by their component. Possible values: source, sync, rendering, readiness(from Resource Group Controller).
	KeyComponent = attribute.Key("component")

	// KeyExportedComponent groups metrics by their component.
	// The "component" metric tag overlaps with a resource tag exported by
	// resource_to_telemetry_conversion when using Prometheus. So it's renamed
	// "exported_component" when exported to Prometheus.
	// TODO: Fix this naming overlap by renaming the "component" metric tag.
	// Possible values: source, sync, rendering, readiness (from Resource Group Controller).
	KeyExportedComponent = attribute.Key("exported_component")

	// KeyErrorClass groups metrics by their error code.
	KeyErrorClass = attribute.Key("errorclass")

	// KeyStatus groups metrics by their status. Possible values: success, error.
	KeyStatus = attribute.Key("status")

	// KeyInternalErrorSource groups the InternalError metrics by their source. Possible values: parser, differ, remediator.
	KeyInternalErrorSource = attribute.Key("source")

	// KeyParserSource groups the metrics for the parser by their source. Possible values: read, parse, update.
	KeyParserSource = attribute.Key("source")

	// KeyTrigger groups metrics by their trigger. Possible values: retry, watchUpdate, managementConflict, resync, reimport.
	KeyTrigger = attribute.Key("trigger")

	// KeyCommit groups metrics by their git commit. Even though this tag has a high cardinality,
	// it is only used by the `last_sync_timestamp` and `last_apply_timestamp` metrics.
	// These are both aggregated as LastValue metrics so the number of recorded values will always be
	// at most 1 per git commit.
	KeyCommit = attribute.Key("commit")

	// KeyContainer groups metrics by their container names. Possible values: reconciler, git-sync.
	// TODO: replace with k8s.container.name resource attribute
	KeyContainer = attribute.Key("container")

	// KeyResourceType groups metrics by their resource types. Possible values: cpu, memory.
	KeyResourceType = attribute.Key("resource")
)

// The following metric tag keys are available from the otel-collector
// Prometheus exporter. They are created from resource attributes using the
// resource_to_telemetry_conversion feature.
var (
	// ResourceKeySyncKind groups metrics by the Sync kind. Possible values: RootSync, RepoSync.
	ResourceKeySyncKind = attribute.Key("configsync_sync_kind")

	// ResourceKeySyncName groups metrics by the Sync name.
	ResourceKeySyncName = attribute.Key("configsync_sync_name")

	// ResourceKeySyncNamespace groups metrics by the Sync namespace.
	ResourceKeySyncNamespace = attribute.Key("configsync_sync_namespace")

	// ResourceKeySyncGeneration groups metrics by the Sync metadata.generation.
	// This allows matching metrics to a specific Sync configuration.
	ResourceKeySyncGeneration = attribute.Key("configsync_sync_generation")

	// ResourceKeyDeploymentName groups metrics by k8s deployment name.
	ResourceKeyDeploymentName = attribute.Key("k8s_deployment_name")

	// ResourceKeyPodName groups metrics by k8s pod name.
	ResourceKeyPodName = attribute.Key("k8s_pod_name")
)

const (
	// StatusSuccess is the string value for the status key indicating success
	StatusSuccess = "success"
	// StatusError is the string value for the status key indicating failure/errors
	StatusError = "error"
	// CommitNone is the string value for the commit key indicating that no
	// commit has been synced.
	CommitNone = "NONE"
	// ApplierController is the string value for the applier controller in the multi-repo mode
	ApplierController = "applier"
	// RemediatorController is the string value for the remediator controller in the multi-repo mode
	RemediatorController = "remediator"
)

// StatusTagKey returns a string representation of the error, if it exists, otherwise success.
func StatusTagKey(err error) string {
	if err == nil {
		return StatusSuccess
	}
	return StatusError
}

// StatusTagValueFromSummary returns error if the summary indicates at least 1
// error, otherwise success.
func StatusTagValueFromSummary(summary *v1beta1.ErrorSummary) string {
	if summary.TotalCount == 0 {
		return StatusSuccess
	}
	return StatusError
}

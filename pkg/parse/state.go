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

package parse

import (
	"time"

	"github.com/GoogleContainerTools/config-sync/pkg/importer/filesystem/cmpath"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
)

const invalidSyncPath = ""

// sourceState contains all state read from the mounted source repo.
type sourceState struct {
	// spec is the source specification as read from the FileSource.
	// This cache avoids re-generating the spec every time the status is updated.
	spec SourceSpec
	// commit is the commit read from the source of truth.
	commit string
	// syncPath is the absolute path to the sync directory that includes the configurations.
	syncPath cmpath.Absolute
	// files is the list of all observed files in the sync directory (recursively).
	files []cmpath.Absolute
}

// ReconcilerState is the current state of the Reconciler, including progress
// indicators and in-memory cache for each of the reconcile stages:
// - Fetch
// - Render/Hydrate
// - Read
// - Parse/Validate
// - Update
//
// ReconcilerState also includes a cache of the RSync spec and status
// (ReconcilerStatus).
//
// TODO: break up cacheForCommit into phase-based caches
type ReconcilerState struct {
	// checkpoint tracks the sourcePath the last time reconciling succeeded or
	// failed. Reconciling includes multiple stages: fetch, render, read, parse,
	// and update.
	checkpoint checkpoint

	// status contains fields that map to RSync status fields.
	status *ReconcilerStatus

	// source tracks the state of the source repo.
	// This field is only set after the reconciler successfully reads all the source files.
	source *sourceState

	// cache tracks the progress made by the reconciler for a source commit.
	cache cacheForCommit

	syncErrorCache *SyncErrorCache

	// lastFullSyncTime is the last time a full reconciler attempt was started.
	lastFullSyncTime metav1.Time
}

type checkpoint struct {
	// syncPath caches the sync path that was last successfully applied.
	// Set to the empty string if the last attempt failed.
	syncPath cmpath.Absolute
	// lastUpdateTime is the last time the checkpoint was updated.
	// AKA: Last successful sync.
	// TODO: Surface this timestamp in the RSync status API
	lastUpdateTime metav1.Time
	// lastTransitionTime is the last time the checkpoint was updated with a new sourcePath.
	// AKA: First successful sync with this sourcePath (repo + branch + source commit + syncDir).
	// TODO: Surface this timestamp in the RSync status API
	lastTransitionTime metav1.Time
}

// updateCheckpoint records the last known source path, updates the
// timestamps, and logs the message.
func (s *ReconcilerState) updateCheckpoint(c clock.Clock, newSyncPath cmpath.Absolute) {
	now := nowMeta(c)
	// Check for transition
	transitioned := false
	if s.checkpoint.syncPath != newSyncPath {
		transitioned = true
		s.checkpoint.syncPath = newSyncPath
		s.checkpoint.lastTransitionTime = now
	}
	// Record when the checkpoint was last updated
	s.checkpoint.lastUpdateTime = now
	if newSyncPath == invalidSyncPath {
		klog.Info("Reconciler checkpoint invalidated")
	} else if transitioned {
		klog.Infof("Reconciler checkpoint updated with new sync path: %s", newSyncPath)
	} else {
		klog.Infof("Reconciler checkpoint updated with existing sync path: %s", newSyncPath)
	}
}

// RecordSyncSuccess is called after a successful sync. It records the last
// known source path, as well as timestamps for last updated and last
// transitioned.
func (s *ReconcilerState) RecordSyncSuccess(c clock.Clock) {
	klog.Info("Sync successful")
	s.updateCheckpoint(c, s.source.syncPath)
	s.cache.needToRetry = false
}

// RecordRenderInProgress is called when waiting for rendering status. It resets
// the cacheForCommit, which tells the next reconcile attempt to re-parse from
// source.
func (s *ReconcilerState) RecordRenderInProgress() {
	klog.Info("Rendering in progress")
	// TODO: track render status lastUpdateTime & lastTransitionTime
	// TODO: update parserResultUpToDate() to trigger parsing when syncPath changes, to avoid needing to reset the cache to trigger parsing when render is successful
	s.cache = cacheForCommit{}
}

// RecordFailure is called when a sync attempt errors. It invalidates the
// checkpoint, requests a retry, and logs the errors. Does not reset the
// cacheForCommit.
func (s *ReconcilerState) RecordFailure(c clock.Clock, errs status.MultiError) {
	if status.AllTransientErrors(errs) {
		klog.Infof("Reconcile attempt failed with transient error(s): %v", status.FormatSingleLine(errs))
	} else {
		klog.Errorf("Reconcile attempt failed: %v", status.FormatSingleLine(errs))
	}
	s.updateCheckpoint(c, invalidSyncPath)
	s.cache.needToRetry = true
}

// RecordReadSuccess is called when read succeeds after a source change is
// detected. It resets the cacheForCommit, which skips re-reading, but forces
// re-parsing.
func (s *ReconcilerState) RecordReadSuccess(source *sourceState) {
	klog.Info("Read successful")
	// TODO: track read status lastUpdateTime & lastTransitionTime
	// TODO: update parserResultUpToDate() to trigger parsing when syncPath changes, to avoid needing to reset the cache to trigger parsing when read is successful
	s.source = source
	s.cache = cacheForCommit{}
}

// RecordReadFailure is called when read errors after a source change is
// detected. It resets the cacheForCommit, forces re-reading and re-parsing.
func (s *ReconcilerState) RecordReadFailure() {
	klog.Info("Read attempt failed")
	// TODO: track read status lastUpdateTime & lastTransitionTime
	s.cache = cacheForCommit{}
}

// IsFullSyncRequired returns true if the specified period has elapsed since the
// last time SetLastFullSyncTime was called.
func (s *ReconcilerState) IsFullSyncRequired(now metav1.Time, fullSyncPeriod time.Duration) bool {
	return s.lastFullSyncTime.IsZero() || now.After(s.lastFullSyncTime.Add(fullSyncPeriod))
}

// RecordFullSyncStart is called when a full sync attempt starts. It resets the
// cacheForCommit, which tells the parser to re-parse from source, but keeps the
// last known read status, to skip re-read on next sync attempt.
// Retry request will not be reset, to avoid resetting the backoff retries.
func (s *ReconcilerState) RecordFullSyncStart(now metav1.Time) {
	needToRetry := s.cache.needToRetry
	s.cache = cacheForCommit{}
	s.cache.needToRetry = needToRetry
	s.lastFullSyncTime = now
}

// SyncErrors returns all the sync errors, including remediator errors,
// validation errors, applier errors, and watch update errors.
func (s *ReconcilerState) SyncErrors() status.MultiError {
	return s.syncErrorCache.Errors()
}

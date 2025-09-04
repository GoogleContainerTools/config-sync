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

package ntopts

import (
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/syncsource"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/metadata"
	rbacv1 "k8s.io/api/rbac/v1"
)

// MultiRepo configures the NT for use with multi-repo tests.
// If NonRootRepos is non-empty, the test is assumed to be running in
// multi-repo mode.
type MultiRepo struct {
	// SyncSources is the set of RootSyncs and RepoSync and their source config
	// for this test.
	SyncSources syncsource.Set

	// Control indicates options for configuring Namespace Repos.
	Control RepoControl

	// ReconcileTimeout sets spec.override.reconcileTimeout on each R*Sync
	// Default: 5m.
	ReconcileTimeout *time.Duration

	// RequireOCIProvider will enable GitProvider Login & Logout.
	RequireOCIProvider bool

	// RequireHelmProvider will enable HelmProvider Login & Logout.
	RequireHelmProvider bool

	// RequireLocalGitProvider will skip the test if run with a GitProvider type other than local.
	RequireLocalGitProvider bool

	// RequireLocalOCIProvider will skip the test if run with a OCIProvider type other than local.
	RequireLocalOCIProvider bool

	// RequireLocalHelmProvider will skip the test if run with a HelmProvider type other than local.
	RequireLocalHelmProvider bool

	// RepoSyncPermissions will grant a list of PolicyRules to NS reconcilers
	RepoSyncPermissions []rbacv1.PolicyRule

	// DeletionPropagationPolicy sets the deletion propagation policy for all RSyncs
	DeletionPropagationPolicy *metadata.DeletionPropagationPolicy
}

// GitSourceOption mutates a GitSyncSource
type GitSourceOption func(*syncsource.GitSyncSource)

// Unstructured mutates a GitSyncSource to use the unstructured SourceFormat.
func Unstructured(source *syncsource.GitSyncSource) {
	source.SourceFormat = configsync.SourceFormatUnstructured
}

// SyncWithGitSource tells the test case that a RootSync or RepoSync should be
// applied that points to an empty Git Repository.
// TODO: Add another option that allows specifying an existing Repository
func SyncWithGitSource(id core.ID, gitOpts ...GitSourceOption) func(opt *New) {
	return func(opt *New) {
		source := &syncsource.GitSyncSource{}
		// Modify the default source with user specified options
		for _, gitOpt := range gitOpts {
			gitOpt(source)
		}
		// Register the RSync with the specified source
		opt.SyncSources[id] = source
	}
}

// RequireOCIProvider will enable OCI repo client login and logout
func RequireOCIProvider(opt *New) {
	opt.RequireOCIProvider = true
}

// RequireHelmProvider will enable Helm repo client login and logout
func RequireHelmProvider(opt *New) {
	opt.RequireHelmProvider = true
}

// RequireLocalGitProvider will skip the test with non-local GitProvider types
func RequireLocalGitProvider(opt *New) {
	opt.RequireLocalGitProvider = true
}

// RequireLocalOCIProvider will skip the test with non-local OCIProvider types.
// RequireLocalOCIProvider implies RequireOCIProvider.
func RequireLocalOCIProvider(opt *New) {
	opt.RequireLocalOCIProvider = true
	opt.RequireOCIProvider = true
}

// RequireLocalHelmProvider will skip the test with non-local HelmProvider types.
// RequireLocalHelmProvider implies RequireHelmProvider.
func RequireLocalHelmProvider(opt *New) {
	opt.RequireLocalHelmProvider = true
	opt.RequireHelmProvider = true
}

// WithDelegatedControl will specify the Delegated Control Pattern.
func WithDelegatedControl(opt *New) {
	opt.Control = DelegatedControl
}

// WithCentralizedControl will specify the Central Control Pattern.
func WithCentralizedControl(opt *New) {
	opt.Control = CentralControl
}

// RepoControl indicates the type of control for Namespace repos.
type RepoControl string

const (
	// DelegatedControl indicates the central admin only declares the Namespace
	// in the Root Repo and delegates declaration of RepoSync to the app operator.
	DelegatedControl = "Delegated"
	// CentralControl indicates the central admin only declares the Namespace
	// in the Root Repo and delegates declaration of RepoSync to the app operator.
	CentralControl = "Central"
)

// WithReconcileTimeout tells the test case to override the default reconcile
// timeout on all RootSyncs and RepoSyncs by default.
func WithReconcileTimeout(timeout time.Duration) func(opt *New) {
	return func(opt *New) {
		timeoutCopy := timeout
		opt.ReconcileTimeout = &timeoutCopy
	}
}

// WithoutReconcileTimeout tells the test case not to override the default
// reconcile timeout on all RootSyncs and RepoSyncs by default.
func WithoutReconcileTimeout(opt *New) {
	opt.ReconcileTimeout = nil
}

// RepoSyncPermissions specifies PolicyRule(s) to grant NS reconcilers
func RepoSyncPermissions(policy ...rbacv1.PolicyRule) Opt {
	return func(opt *New) {
		opt.RepoSyncPermissions = append(opt.RepoSyncPermissions, policy...)
	}
}

// WithDeletionPropagationPolicy specifies the DeletionPropagationPolicy
func WithDeletionPropagationPolicy(policy metadata.DeletionPropagationPolicy) Opt {
	return func(opt *New) {
		opt.DeletionPropagationPolicy = &policy
	}
}

// WithoutDeletionPropagationPolicy specifies no DeletionPropagationPolicy
// No annotation will be added to RSyncs, causing the default policy to be used ("Orphan")
func WithoutDeletionPropagationPolicy() Opt {
	return func(opt *New) {
		opt.DeletionPropagationPolicy = nil
	}
}

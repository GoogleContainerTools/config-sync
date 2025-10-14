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

package e2e

import (
	"fmt"
	"testing"

	"github.com/GoogleContainerTools/config-sync/e2e/nomostest"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/gitproviders"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/metrics"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/ntopts"
	nomostesting "github.com/GoogleContainerTools/config-sync/e2e/nomostest/testing"
	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
)

func TestInvalidRootSyncBranchStatus(t *testing.T) {
	nt := nomostest.New(t, nomostesting.SyncSourceGit)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	// Update RootSync to invalid branch name
	nomostest.SetRootSyncGitBranch(nt, configsync.RootSyncName, "invalid-branch")

	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.SourceErrorCode, ""))

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	rootSyncLabels, err := nomostest.MetricLabelsForRootSync(nt, rootSyncNN)
	if err != nil {
		nt.T.Fatal(err)
	}
	commitHash := rootSyncGitRepo.MustHash(nt.T)

	nt.Must(nomostest.ValidateMetrics(nt,
		nomostest.ReconcilerErrorMetrics(nt, rootSyncLabels, commitHash, metrics.ErrorSummary{
			Source: 1,
		})))

	// Update RootSync to valid branch name
	nomostest.SetRootSyncGitBranch(nt, configsync.RootSyncName, gitproviders.MainBranch)

	nt.Must(nt.WatchForAllSyncs())

	nt.Must(nomostest.ValidateStandardMetrics(nt))
}

func TestInvalidRootSyncRevisionStatus(t *testing.T) {
	nt := nomostest.New(t, nomostesting.SyncSourceGit)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)

	// Update RootSync to invalid branch name
	nomostest.SetRootSyncGitRevision(nt, configsync.RootSyncName, "invalid-branch")

	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.SourceErrorCode, ""))

	rootSyncNN := nomostest.RootSyncNN(configsync.RootSyncName)
	rootSyncLabels, err := nomostest.MetricLabelsForRootSync(nt, rootSyncNN)
	if err != nil {
		nt.T.Fatal(err)
	}
	commitHash := rootSyncGitRepo.MustHash(nt.T)

	nt.Must(nomostest.ValidateMetrics(nt,
		nomostest.ReconcilerErrorMetrics(nt, rootSyncLabels, commitHash, metrics.ErrorSummary{
			Source: 1,
		})))

	// Update RootSync to valid branch name
	nomostest.SetRootSyncGitRevision(nt, configsync.RootSyncName, gitproviders.MainBranch)

	nt.Must(nt.WatchForAllSyncs())

	nt.Must(nomostest.ValidateStandardMetrics(nt))
}

func TestInvalidRepoSyncBranchStatus(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, namespaceRepo)
	nt := nomostest.New(t, nomostesting.SyncSourceGit,
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)

	repoSync := nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, repoSyncKey)
	repoSync.Spec.Branch = "invalid-branch"
	repoSync.Spec.Revision = ""
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(namespaceRepo, repoSync.Name), repoSync))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update RepoSync to invalid branch name"))

	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(namespaceRepo, configsync.RepoSyncName, status.SourceErrorCode, ""))

	nt.Must(nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
		// RepoSync already included in the default resource count and operations
	}))

	repoSyncLabels, err := nomostest.MetricLabelsForRepoSync(nt, repoSyncKey)
	if err != nil {
		nt.T.Fatal(err)
	}
	// TODO: Fix commit to be UNKNOWN (b/361182373)
	commitHash := repoSyncGitRepo.MustHash(nt.T)

	nt.Must(nomostest.ValidateMetrics(nt,
		// Source error prevents apply, so don't wait for a sync with the current commit.
		nomostest.ReconcilerErrorMetrics(nt, repoSyncLabels, commitHash, metrics.ErrorSummary{
			Source: 1,
		})))

	repoSync.Spec.Branch = gitproviders.MainBranch
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(namespaceRepo, repoSync.Name), repoSync))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update RepoSync to valid branch name"))

	// Ensure RepoSync's active branch is checked out, so the correct commit is used for validation.
	nt.Must(repoSyncGitRepo.CheckoutBranch(gitproviders.MainBranch))

	nt.Must(nt.WatchForAllSyncs())

	nt.Must(nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
		// RepoSync already included in the default resource count and operations
	}))

	nt.Must(nomostest.ValidateStandardMetricsForRepoSync(nt, metrics.Summary{
		Sync:        repoSyncKey,
		ObjectCount: 0, // no additional managed objects
	}))
}

func TestInvalidRepoSyncRevisionStatus(t *testing.T) {
	repoSyncID := core.RepoSyncID(configsync.RepoSyncName, namespaceRepo)
	nt := nomostest.New(t, nomostesting.SyncSourceGit,
		ntopts.SyncWithGitSource(repoSyncID))
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	repoSyncKey := repoSyncID.ObjectKey
	repoSyncGitRepo := nt.SyncSourceGitReadWriteRepository(repoSyncID)

	repoSync := nomostest.RepoSyncObjectV1Beta1FromNonRootRepo(nt, repoSyncKey)
	repoSync.Spec.Revision = "invalid-branch"
	repoSync.Spec.Branch = ""
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(namespaceRepo, repoSync.Name), repoSync))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update RepoSync to invalid branch name"))

	nt.Must(nt.Watcher.WatchForRepoSyncSourceError(namespaceRepo, configsync.RepoSyncName, status.SourceErrorCode, ""))

	nt.Must(nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
		// RepoSync already included in the default resource count and operations
	}))

	repoSyncLabels, err := nomostest.MetricLabelsForRepoSync(nt, repoSyncKey)
	if err != nil {
		nt.T.Fatal(err)
	}
	// TODO: Fix commit to be UNKNOWN (b/361182373)
	commitHash := repoSyncGitRepo.MustHash(nt.T)

	nt.Must(nomostest.ValidateMetrics(nt,
		// Source error prevents apply, so don't wait for a sync with the current commit.
		nomostest.ReconcilerErrorMetrics(nt, repoSyncLabels, commitHash, metrics.ErrorSummary{
			Source: 1,
		})))

	repoSync.Spec.Revision = gitproviders.MainBranch
	nt.Must(rootSyncGitRepo.Add(nomostest.StructuredNSPath(namespaceRepo, repoSync.Name), repoSync))
	nt.Must(rootSyncGitRepo.CommitAndPush("Update RepoSync to valid branch name"))

	// Ensure RepoSync's active branch is checked out, so the correct commit is used for validation.
	nt.Must(repoSyncGitRepo.CheckoutBranch(gitproviders.MainBranch))

	nt.Must(nt.WatchForAllSyncs())

	nt.Must(nomostest.ValidateStandardMetricsForRootSync(nt, metrics.Summary{
		Sync: nomostest.RootSyncNN(configsync.RootSyncName),
		// RepoSync already included in the default resource count and operations
	}))

	nt.Must(nomostest.ValidateStandardMetricsForRepoSync(nt, metrics.Summary{
		Sync:        repoSyncKey,
		ObjectCount: 0, // no additional managed objects
	}))
}

func TestSyncFailureAfterSuccessfulSyncs(t *testing.T) {
	nt := nomostest.New(t, nomostesting.SyncSourceGit)
	rootSyncGitRepo := nt.SyncSourceGitReadWriteRepository(nomostest.DefaultRootSyncID)
	nt.T.Cleanup(func() {
		nt.T.Log("Resetting all RootSync branches to main")
		nt.Must(rootSyncGitRepo.CheckoutBranch(gitproviders.MainBranch))
		nomostest.SetRootSyncGitBranch(nt, configsync.RootSyncName, gitproviders.MainBranch)
		nt.Must(nt.WatchForAllSyncs())
	})

	// Add audit namespace.
	auditNS := "audit"
	// The test will delete the branch later, but the main branch can't be deleted
	// on some Git providers (e.g. Bitbucket), so using a develop branch.
	devBranch := "develop"
	nt.Must(rootSyncGitRepo.CreateBranch(devBranch))
	nt.Must(rootSyncGitRepo.CheckoutBranch(devBranch))
	nt.Must(rootSyncGitRepo.Add(
		fmt.Sprintf("acme/namespaces/%s/ns.yaml", auditNS),
		k8sobjects.NamespaceObject(auditNS)))
	nt.Must(rootSyncGitRepo.CommitAndPushBranch("add namespace to acme directory", devBranch))

	// Update RootSync to sync from the dev branch
	nomostest.SetRootSyncGitBranch(nt, configsync.RootSyncName, devBranch)
	nt.Must(nt.WatchForAllSyncs())

	// Validate namespace 'acme' created.
	nt.Must(nt.Validate(auditNS, "", k8sobjects.NamespaceObject(auditNS)))

	// Make the sync fail by invalidating the source repo.
	nt.Must(rootSyncGitRepo.RenameBranch(devBranch, "invalid-branch"))
	nt.Must(nt.Watcher.WatchForRootSyncSourceError(configsync.RootSyncName, status.SourceErrorCode, ""))

	// Change the remote branch name back to the original name.
	nt.Must(rootSyncGitRepo.RenameBranch("invalid-branch", devBranch))
	nt.Must(nt.WatchForAllSyncs())
}

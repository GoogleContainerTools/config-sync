# Poll until status is complete

* Author(s): ahmadalguydi
* Approver:
* Status: provisional

## Summary

Add `--poll-until=complete` to `nomos status` so automation can wait for every
reachable RootSync and RepoSync to finish successfully. The command prints the
initial status immediately, then refreshes it until all repositories report
`SYNCED` or the caller's external timeout terminates the process.

## Motivation

Issue [#2006](https://github.com/GoogleContainerTools/config-sync/issues/2006)
requests a bounded way to wait for eventual reconciliation without requiring
scripts to parse repeated status output.

## Design Overview

The existing `--poll` loop is extended with an optional completion predicate.
The predicate is true only when every discovered cluster is reachable, has at
least one RootSync or RepoSync, and every repository is `SYNCED` without
reported errors. A stalled, pending, reconciling, unavailable, or errored
repository keeps polling. The new flag accepts only `complete`; invalid values
fail before any cluster request.

`--poll-until=complete` uses a five-second interval when `--poll` is omitted.
An explicit `--poll` interval continues to take precedence. Existing commands
without `--poll-until` retain their current one-shot or indefinite polling
behavior.

## User Guide

Run a status check once as before:

```shell
nomos status
```

Wait for all repositories to become synchronized, refreshing every 10 seconds:

```shell
timeout 2m nomos status --poll=10s --poll-until=complete
```

When all repositories are `SYNCED`, the command exits successfully after
printing that status. If the external timeout expires first, the process is
terminated and the caller can treat that as a reconciliation timeout.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Polling can increase API traffic. | Require an explicit completion mode and use a conservative five-second default; callers can choose a longer interval. |
| A cluster with no sync objects could be mistaken for completion. | Treat empty or unavailable cluster states as incomplete. |
| Existing users depend on indefinite `--poll`. | Preserve the existing loop unless `--poll-until` is provided. |

## Test Plan

Unit tests cover complete, pending, errored, and empty cluster states, plus
validation of the supported flag value. Existing status tests continue to
exercise rendering and cluster-state collection.

## Open Issues/Questions

### Should more completion states be supported?

Resolution: Not yet resolved. The first implementation intentionally supports
only `complete`, matching issue #2006 and leaving room for future predicates.

## Alternatives Considered

### Shell-side output parsing

Parsing repeated `nomos status` output is fragile and requires callers to know
the output format. A structured completion predicate keeps the behavior inside
the CLI while preserving its human-readable output.

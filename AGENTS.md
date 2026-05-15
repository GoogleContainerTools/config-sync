# AGENTS.md

Guidance for AI coding agents working in this repository. Human contributors should also read [docs/contributing.md](docs/contributing.md), [docs/development.md](docs/development.md), and [docs/testing.md](docs/testing.md), which are the source of truth.

## Project summary

Config Sync synchronizes configurations and policies from a source of truth (Git repository, OCI image, or Helm chart) to a Kubernetes cluster, ensuring consistency at scale. It simplifies and automates configuration management across single or multi-cluster environments, managing both cluster-scoped and namespace-scoped resources.

**Key Technologies & Architecture:**
- **Language & Frameworks:** Go, Kubernetes, Kubebuilder / Controller Runtime, Kustomize.
- **Core Custom Resources:** `RootSync` (cluster-scoped synchronization) and `RepoSync` (namespace-scoped synchronization).
- **Reconciler Manager:** Controller component that manages the lifecycle of individual reconciler deployments.
- **Reconcilers:** Specialized controllers spawned for each sync target that watch the upstream source and apply manifests to the cluster.
- **Admission Webhook:** Validates configuration and state changes for Config Sync custom resources.
- **Nomos CLI:** Command-line interface for inspecting status, troubleshooting, and interacting with Config Sync deployments.

## Repository layout

| Path | What lives there |
| --- | --- |
| [cmd/](cmd/) | Main entrypoints for controllers (`reconciler-manager`, `reconciler`, `admission-webhook`) and the `nomos` CLI. |
| [pkg/](pkg/) | Core implementation logic: reconcilers, git/OCI sync logic, manifest parsers, validation logic, and webhook handlers. |
| [manifests/](manifests/) | Base Kubernetes manifests and Kustomize configurations used to deploy Config Sync components. |
| [e2e/](e2e/) | End-to-end test framework and test suites (Go-driven, runnable against kind or GKE). |
| [docs/](docs/) | Documentation for usage, installation, contributing, development, and testing. |
| [examples/](examples/) | Sample Git repositories and configurations used for tutorials and verification. |
| [scripts/](scripts/) | Shell scripts for CI, verification, and auxiliary build tasks. |
| [build/](build/) | Build setups, container entrypoints, base Dockerfiles, and build dependencies. |

When in doubt about ownership or review requirements, check the nearest `OWNERS` file.

## Build, test, lint

All primary build and testing workflows run through the [Makefile](Makefile). Prefer using `make` targets over invoking native tools directly to maintain consistency with CI.

- `make build-cli` — Compiles the `nomos` CLI binary.
- `make build-manifests` — Generates deployment manifests into `.output/staging/oss`.
- `make build-images` — Builds required Docker container images for Config Sync controllers.
- `make config-sync-manifest` — Builds images and manifests from source in a single step.
- `make test` — Runs fast Go unit tests.
- `make test-e2e-kind` — Runs end-to-end validation against a local kind cluster (requires pre-installing kind via `make install-kind`).

For detailed testing parameters (including running targeted E2E test regexes or reusing test cluster environments), consult [docs/testing.md](docs/testing.md).

## Agent Skills

Shared agent skills and custom automations for working in this repository should be organized within an `.agents/` directory following emerging repository standards.
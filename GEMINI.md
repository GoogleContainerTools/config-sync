# Config Sync GEMINI.md

## Project Overview

This directory contains the source code for Config Sync, a tool designed to enable cluster operators and platform administrators to deploy consistent configurations and policies across multiple Kubernetes clusters. It is part of Google Kubernetes Engine (GKE) but can also be installed as open source.

Config Sync synchronizes configurations from a Git repository (or OCI images, Helm charts) to a Kubernetes cluster. It manages both cluster-scoped and namespace-scoped resources.

**Key Technologies:**

*   **Go:** The primary programming language.
*   **Kubernetes:** The target platform for configuration deployment.
*   **Kubebuilder/Controller Runtime:** Framework for building Kubernetes controllers.
*   **Kustomize:** Used for Kubernetes manifest management.
*   **Docker:** Used for packaging and running components.
*   **Make:** Used as the primary build and task runner.

**Architecture:**

Config Sync typically runs as a set of controllers within a Kubernetes cluster. Key components include:

*   **Reconciler Manager:** Manages the lifecycle of reconcilers.
*   **Reconcilers:** Separate controllers for root-sync (cluster-scoped) and repo-sync (namespace-scoped) that watch the source (e.g., Git) and apply changes to the cluster.
*   **Admission Webhook:** Provides validation for Config Sync CRDs.
*   **Nomos CLI:** A command-line tool for interacting with Config Sync, checking status, and troubleshooting.

Configurations are declared using custom resources like `RootSync` and `RepoSync`.

## Building and Running

**Prerequisites:**

*   Go (version specified in `go.mod`)
*   Git
*   Make
*   Docker
*   gcloud CLI
*   gsutil

**Common Commands (from Makefile):**

*   **Build everything and generate manifests:**
    ```bash
    make config-sync-manifest
    ```
    Artifacts are placed in `.output/staging/oss`.

*   **Build Docker images:**
    ```bash
    make build-images
    ```

*   **Push Docker images (defaults to GCR in current `gcloud` project):**
    ```bash
    make push-images
    ```
    (Variables like `REGISTRY` and `IMAGE_TAG` can be overridden).

*   **Build the `nomos` CLI:**
    ```bash
    make build-cli
    ```
    Binary will be in `.output/go/bin/linux_amd64/nomos`.

*   **Run all tests:**
    ```bash
    make test
    ```

*   **Run unit tests:**
    ```bash
    make test-unit
    ```

*   **Run end-to-end tests:**
    ```bash
    make test-e2e
    ```
    (Requires a Kubernetes cluster, often set up using `kind`).

*   **Deploy to current `kubectl` context:**
    ```bash
    # Build and deploy
    make run-oss

    # Apply from existing artifacts
    kubectl apply -f .output/staging/oss
    ```

*   **Format Go code:**
    ```bash
    make fmt-go
    ```

*   **Clean build artifacts:**
    ```bash
    make clean
    ```

## Development Conventions

*   **Code Style:** Standard Go formatting enforced by `gofmt` and `goimports` (use `make fmt-go`). Linting is done with `golangci-lint` (config in `.golangci.yaml`, run via `make lint-go`).
*   **Licensing:** Apache 2.0. License headers are checked and added using `addlicense` (run via `make license-headers` or `make lint-license-headers`).
*   **Dependencies:** Managed with Go Modules (`go.mod` and `go.sum`). Vendored dependencies are in the `vendor` directory.
*   **Testing:** Unit tests are co-located with the code in `_test.go` files. End-to-end tests are in the `e2e` directory.
*   **Build Environment:** Many `make` targets run inside a Docker container defined by `build/buildenv` to ensure a consistent environment.
*   **Manifests:** Base manifests are in the `manifests` directory, often processed with Kustomize.

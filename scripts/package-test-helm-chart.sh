#!/bin/bash
# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HELM_CHARTS_BUCKET="gs://kpt-config-sync-ci-test-helm-charts"
CHART_NAME="simple-pause"
CHART_SRC_DIR="${REPO_ROOT}/e2e/testdata/helm-charts/${CHART_NAME}"

# Create a temporary directory to avoid creating artifacts in the source tree.
TMP_DIR=$(mktemp -d)

function cleanup() {
    rm -rf -- "$TMP_DIR"
}

# Ensures the temporary directory is cleaned up on exit
trap cleanup EXIT

# Copy chart source to the temporary directory.
cp -R "${CHART_SRC_DIR}" "${TMP_DIR}/"
cd "${TMP_DIR}"

echo "Packaging Helm chart in ${TMP_DIR}"
helm package ./${CHART_NAME}/*

echo "Indexing Helm repository in ${TMP_DIR}"
helm repo index .

echo "Uploading charts and index to ${HELM_CHARTS_BUCKET}"
gcloud storage cp -n ./*.tgz "${HELM_CHARTS_BUCKET}/"
gcloud storage cp ./index.yaml "${HELM_CHARTS_BUCKET}/"
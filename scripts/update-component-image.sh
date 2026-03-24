#!/bin/bash
# 
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "${REPO_ROOT}"

UPDATE_TYPE=${UPDATE_TYPE:-latest-build}
COMPONENT=${1:-${COMPONENT:-}}

if [ -z "${COMPONENT}" ]; then
  echo "Usage: $0 <git-sync|otelcontribcol|debian-base|kustomize|helm>" >&2
  exit 1
fi

case "${COMPONENT}" in
  git-sync)
    VAR_NAME="GIT_SYNC_VERSION"
    IMAGE_PATH="gcr.io/config-management-release/git-sync"
    FILTER_LATEST_VERSION="NOT tags:no-new-use-public-image*"
    GREP_PATTERN_LATEST_VERSION="__linux_amd64"
    STRIP_BASE_PATTERN="-gke.*__linux_amd64"
    QUERY_TYPE="gcloud"
    ;;
  otelcontribcol)
    VAR_NAME="OTELCONTRIBCOL_VERSION"
    IMAGE_PATH="gcr.io/config-management-release/otelcontribcol"
    FILTER_LATEST_VERSION="NOT tags:no-new-use-public-image*"
    GREP_PATTERN_LATEST_VERSION="v"
    STRIP_BASE_PATTERN="-gke.*"
    QUERY_TYPE="gcloud"
    ;;
  debian-base)
    VAR_NAME="DEBIAN_BASE_IMAGE"
    IMAGE_PATH="gcr.io/gke-release/debian-base"
    FILTER_LATEST_VERSION="tags:bookworm*"
    GREP_PATTERN_LATEST_VERSION="^bookworm"
    STRIP_BASE_PATTERN="-gke\.[0-9]+$"
    QUERY_TYPE="gcloud"
    ;;
  kustomize)
    VAR_NAME="KUSTOMIZE_VERSION"
    GCS_PATH="gs://config-management-release/config-sync/kustomize/tag/"
    STRIP_BASE_PATTERN="-gke.*"
    QUERY_TYPE="gsutil"
    ;;
  helm)
    VAR_NAME="HELM_VERSION"
    GCS_PATH="gs://config-management-release/config-sync/helm/tag/"
    STRIP_BASE_PATTERN="-gke.*"
    QUERY_TYPE="gsutil"
    ;;
  *)
    echo "Unknown component: ${COMPONENT}" >&2
    exit 1
    ;;
esac

set +e
# Get the current tag from the Makefile
if [ "${COMPONENT}" == "debian-base" ]; then
  CURRENT_TAG=$(grep "${VAR_NAME} :=" Makefile | sed -E 's/.*://; s/ .*//')
else
  CURRENT_TAG=$(grep "^${VAR_NAME} :=" Makefile | sed 's/.*:= //')
fi

if [ -z "${CURRENT_TAG}" ]; then
  echo "Failed to find ${VAR_NAME} in Makefile" >&2
  exit 1
fi
set -e

if [[ "${UPDATE_TYPE}" == "latest-version" ]]; then
  if [ "${QUERY_TYPE}" == "gcloud" ]; then
    FILTER="${FILTER_LATEST_VERSION}"
    GREP_PATTERN="${GREP_PATTERN_LATEST_VERSION}"
  else
    # For GCS, we just look for all tags and pick the latest semver-ish one
    FILTER="*"
    GREP_PATTERN="^v"
  fi
elif [[ "${UPDATE_TYPE}" == "latest-build" ]]; then
  if [[ "${COMPONENT}" == "debian-base" ]]; then
    BASE_VERSION=$(echo "${CURRENT_TAG}" | sed -E "s/${STRIP_BASE_PATTERN}//")
  else
    BASE_VERSION=${CURRENT_TAG%${STRIP_BASE_PATTERN}}
  fi
  FILTER="tags:${BASE_VERSION}*"
  GREP_PATTERN="^${BASE_VERSION}"
  if [ "${QUERY_TYPE}" == "gsutil" ]; then
    FILTER="${BASE_VERSION%-gke.*}*"
    GREP_PATTERN="^${BASE_VERSION%-gke.*}"
  fi
else
  echo "Invalid UPDATE_TYPE=\"${UPDATE_TYPE}\". Must be 'latest-version' or 'latest-build'." >&2
  exit 1
fi

echo "Fetching ${UPDATE_TYPE//-/ } tag for ${COMPONENT}"
if [ "${QUERY_TYPE}" == "gcloud" ]; then
  LATEST_TAG=$(gcloud container images list-tags "${IMAGE_PATH}" \
    --filter="${FILTER}" \
    --format="value(tags)" | tr ',' '\n' | grep "${GREP_PATTERN}" | sort -V | tail -n 1)
else
  # Query GCS and strip path/trailing slash
  LATEST_TAG=$(gsutil ls -d "${GCS_PATH}${FILTER}/" | sed "s|${GCS_PATH}||; s|/||" | grep "${GREP_PATTERN}" | sort -V | tail -n 1)
fi

if [ -z "${LATEST_TAG}" ]; then
  echo "Failed to find latest tag for ${COMPONENT}" >&2
  exit 1
fi

if [ "$LATEST_TAG" == "$CURRENT_TAG" ]; then
  echo "${VAR_NAME} is already up to date ($LATEST_TAG)."
  exit 0
fi

echo "Updating ${VAR_NAME} from $CURRENT_TAG to $LATEST_TAG"
if [ "${COMPONENT}" == "debian-base" ]; then
  sed -i "s|${VAR_NAME} := ${IMAGE_PATH}:.*|${VAR_NAME} := ${IMAGE_PATH}:$LATEST_TAG|" Makefile
else
  sed -i "s|^${VAR_NAME} := .*|${VAR_NAME} := ${LATEST_TAG}|" Makefile
fi

# Keep recommended CLI versions in pkg/hydrate/tool_util.go in sync with image tags.
TOOL_UTIL="${REPO_ROOT}/pkg/hydrate/tool_util.go"
case "${COMPONENT}" in
  helm)
    sed -i "s|HelmVersion = \".*\"|HelmVersion = \"${LATEST_TAG}\"|" "${TOOL_UTIL}"
    ;;
  kustomize)
    sed -i "s|KustomizeVersion = \".*\"|KustomizeVersion = \"${LATEST_TAG}\"|" "${TOOL_UTIL}"
    ;;
esac

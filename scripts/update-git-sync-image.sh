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

set +e
# Get the current tag from the Makefile
CURRENT_TAG=$(grep "^GIT_SYNC_VERSION :=" Makefile | sed 's/.*:= //')
if [ -z "${CURRENT_TAG}" ]; then
  echo "Failed to find GIT_SYNC_VERSION" >&2
  exit 1
fi
set -e

if [[ "${UPDATE_TYPE}" == "latest-version" ]]; then
  FILTER="NOT tags:no-new-use-public-image*"
  GREP_PATTERN="__linux_amd64"
elif [[ "${UPDATE_TYPE}" == "latest-build" ]]; then
  # Strip the -gke.N__linux_amd64 suffix to get the base version
  BASE_VERSION=${CURRENT_TAG%-gke.*__linux_amd64}
  FILTER="tags:${BASE_VERSION}*"
  GREP_PATTERN="^${BASE_VERSION}"
else
  echo "Usage: $0 [latest-version|latest-build]" >&2
  exit 1
fi

echo "Fetching ${UPDATE_TYPE//-/ } tag for git-sync"
LATEST_TAG=$(gcloud container images list-tags gcr.io/config-management-release/git-sync \
  --filter="${FILTER}" \
  --format="value(tags)" | tr ',' '\n' | grep "${GREP_PATTERN}" | sort -V | tail -n 1)

if [ -z "${LATEST_TAG}" ]; then
  echo "Failed to find latest tag for git-sync" >&2
  exit 1
fi

if [ "$LATEST_TAG" == "$CURRENT_TAG" ]; then
  echo "GIT_SYNC_VERSION is already up to date ($LATEST_TAG)."
  exit 0
fi

echo "Updating GIT_SYNC_VERSION from $CURRENT_TAG to $LATEST_TAG"
sed -i "s|^GIT_SYNC_VERSION := .*|GIT_SYNC_VERSION := ${LATEST_TAG}|" Makefile
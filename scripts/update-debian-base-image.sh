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

####################################################################################################
# FUNCTION DECLARATIONS
#
# It's crucial that error messages in these functions be sent to stderr with 1>&2.
# Otherwise, they will not bubble up to the make target that calls this script.

err() {
  local msg="$1"
  echo "${msg}" 1>&2
  exit 1
}

####################################################################################################

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "${REPO_ROOT}"

UPDATE_TYPE=${UPDATE_TYPE:-latest-build}

# Get the current tag from the Makefile
CURRENT_TAG=$(grep "DEBIAN_BASE_IMAGE :=" Makefile | sed -E 's/.*://; s/ .*//')

if [[ "${UPDATE_TYPE}" == "latest-version" ]]; then
  FILTER="tags:bookworm*"
  GREP_PATTERN="^bookworm"
elif [[ "${UPDATE_TYPE}" == "latest-build" ]]; then
  # Strip the -gke.N suffix to get the base version
  BASE_VERSION=$(echo "${CURRENT_TAG}" | sed -E 's/-gke\.[0-9]+$//')
  FILTER="tags:${BASE_VERSION}*"
  GREP_PATTERN="^${BASE_VERSION}"
else
  echo "Usage: $0 [latest-version|latest-build]" >&2
  exit 1
fi

# Get the latest tag
LATEST_TAG=$(gcloud container images list-tags gcr.io/gke-release/debian-base --filter="${FILTER}" --format="value(tags)" | tr ',' '\n' | grep "${GREP_PATTERN}" | sort -V | tail -n 1)

if [ -z "$LATEST_TAG" ]; then
  echo "Failed to find latest tag for debian-base" >&2
  exit 1
fi

if [ "$LATEST_TAG" == "$CURRENT_TAG" ]; then
  echo "DEBIAN_BASE_IMAGE is already up to date ($LATEST_TAG)."
  exit 0
fi

echo "Updating DEBIAN_BASE_IMAGE from $CURRENT_TAG to $LATEST_TAG"
sed -i "s|DEBIAN_BASE_IMAGE := gcr.io/gke-release/debian-base:.*|DEBIAN_BASE_IMAGE := gcr.io/gke-release/debian-base:$LATEST_TAG|" Makefile

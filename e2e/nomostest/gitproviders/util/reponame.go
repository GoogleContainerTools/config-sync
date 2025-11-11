// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	defaultRepoNameMaxLen      = 63
	bitbucketRepoNameMaxLen    = 62
	gitlabProjectNameMaxLength = 256
	repoNameHashLen            = 8
)

// SanitizeGCPRepoName replaces all slashes with hyphens, and truncates the name.
// repo name may contain between 3 and 63 lowercase letters, digits and hyphens.
// This is used by CSR and SSM which are per-project resources
// The repo name will be of the form cs-e2e-<repoPrefix>-<name>-<hash>
func SanitizeGCPRepoName(repoPrefix, name string) string {
	if name == "" {
		return name // Requires at least the base repo name
	}
	fullName := "cs-e2e"

	if repoPrefix != "" {
		fullName += "-" + repoPrefix
	}
	fullName += "-" + name
	hashStr := hashName(fullName)

	return sanitize(fullName, hashStr, defaultRepoNameMaxLen)
}

// SanitizeBitbucketRepoName replaces all slashes with hyphens, and truncates the name for Bitbucket.
// repo name may contain between 3 and 62 lowercase letters, digits and hyphens.
// The repo name will be of the form <name>-<repoSuffix>-<hash>
func SanitizeBitbucketRepoName(repoSuffix, name string) string {
	if name == "" {
		return name // Requires at least the base repo name
	}

	fullName := name
	if repoSuffix != "" {
		fullName += "-" + repoSuffix
	}
	hashStr := hashName(fullName)

	return sanitize(fullName, hashStr, bitbucketRepoNameMaxLen)
}

// SanitizeGitlabRepoName replaces all slashes with hyphens, and truncates the name for Gitlab.
// repo name may contain between 3 and 256 lowercase letters, digits and hyphens.
// The repo name will be of the form <name>-<repoSuffix>-<hash>
func SanitizeGitlabRepoName(repoSuffix, name string) string {
	if name == "" {
		return name // Requires at least the base repo name
	}

	fullName := name
	if repoSuffix != "" {
		fullName += "-" + repoSuffix
	}
	hashStr := hashName(fullName)

	return sanitize(fullName, hashStr, gitlabProjectNameMaxLength)
}

func hashName(fullName string) string {
	hashBytes := sha1.Sum([]byte(fullName))
	return hex.EncodeToString(hashBytes[:])[:repoNameHashLen]
}

func sanitize(fullName, hashStr string, maxLen int) string {
	if len(fullName) > maxLen-1-repoNameHashLen {
		fullName = fullName[:maxLen-1-repoNameHashLen]
	}
	sanitizedName := strings.ReplaceAll(fullName, "/", "-")
	sanitizedName = strings.TrimSuffix(sanitizedName, "-") // Avoids double dash before the hash.

	return fmt.Sprintf("%s-%s", sanitizedName, hashStr)
}

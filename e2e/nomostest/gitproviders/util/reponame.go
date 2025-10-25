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
	defaultRepoNameMaxLen   = 63
	bitbucketRepoNameMaxLen = 62
	repoNameHashLen         = 8
)

// SanitizeRepoName replaces all slashes with hyphens, and truncates the name.
// repo name may contain between 3 and 63 lowercase letters, digits and hyphens.
func SanitizeRepoName(repoPrefix, name string) string {
	return sanitize(repoPrefix, name, defaultRepoNameMaxLen)
}

// SanitizeBitbucketRepoName replaces all slashes with hyphens, and truncates the name for Bitbucket.
// repo name may contain between 3 and 62 lowercase letters, digits and hyphens.
func SanitizeBitbucketRepoName(repoPrefix, name string) string {
	return sanitize(repoPrefix, name, bitbucketRepoNameMaxLen)
}

func hashName(fullName string) string {
	hashBytes := sha1.Sum([]byte(fullName))
	return hex.EncodeToString(hashBytes[:])[:repoNameHashLen]
}

func sanitize(repoPrefix, name string, maxLen int) string {
	fullName := "cs-e2e-" + repoPrefix + "-" + name
	hashStr := hashName(fullName)

	if len(fullName) > maxLen-1-repoNameHashLen {
		fullName = fullName[:maxLen-1-repoNameHashLen]
	}
	sanitizedName := strings.ReplaceAll(fullName, "/", "-")
	sanitizedName = strings.TrimSuffix(sanitizedName, "-") // Avoids double dash before the hash.

	return fmt.Sprintf("%s-%s", sanitizedName, hashStr)
}

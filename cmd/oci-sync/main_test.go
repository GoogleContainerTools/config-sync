// Copyright 2025 Google LLC
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

package main

import (
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/api/configsync"
	"github.com/GoogleContainerTools/config-sync/pkg/auth"
	"github.com/GoogleContainerTools/config-sync/pkg/oci"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/assert"
)

func TestGetAuthenticator(t *testing.T) {
	username := "username"
	password := "password"
	flUsername = &username
	flPassword = &password

	testCases := []struct {
		name     string
		auth     configsync.AuthType
		expected authn.Authenticator
		hasError bool
	}{
		{
			name:     "none",
			auth:     configsync.AuthNone,
			expected: authn.Anonymous,
		},
		{
			name: "token",
			auth: configsync.AuthToken,
			expected: &authn.Basic{
				Username: username,
				Password: password,
			},
		},
		{
			name: "k8sserviceaccount",
			auth: configsync.AuthK8sServiceAccount,
			expected: &oci.CredentialAuthenticator{
				CredentialProvider: &auth.CachingCredentialProvider{
					Scopes: auth.OCISourceScopes(),
				},
			},
		},
		{
			name: "gcenode",
			auth: configsync.AuthGCENode,
			expected: &oci.CredentialAuthenticator{
				CredentialProvider: &auth.CachingCredentialProvider{
					Scopes: auth.OCISourceScopes(),
				},
			},
		},
		{
			name: "gcpserviceaccount",
			auth: configsync.AuthGCPServiceAccount,
			expected: &oci.CredentialAuthenticator{
				CredentialProvider: &auth.CachingCredentialProvider{
					Scopes: auth.OCISourceScopes(),
				},
			},
		},
		{
			name:     "invalid auth",
			auth:     "invalid",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auth, err := getAuthenticator(tc.auth)
			assert.Equal(t, tc.hasError, err != nil)
			assert.Equal(t, tc.expected, auth)
		})
	}
}

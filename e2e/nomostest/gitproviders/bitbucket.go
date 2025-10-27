// Copyright 2022 Google LLC
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

package gitproviders

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/gitproviders/util"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testlogger"
)

const (
	bitbucketProject = "CSCI"

	// PrivateSSHKey is secret name of the private SSH key stored in the Cloud Secret Manager.
	PrivateSSHKey = "config-sync-ci-ssh-private-key"
)

// BitbucketClient is the client that calls the Bitbucket REST APIs.
type BitbucketClient struct {
	oauthKey     string
	oauthSecret  string
	refreshToken string
	logger       *testlogger.TestLogger
	workspace    string
	// repoSuffix is used to avoid overlap
	repoSuffix string
	httpClient *http.Client
}

// newBitbucketClient instantiates a new Bitbucket client.
func newBitbucketClient(repoSuffix string, logger *testlogger.TestLogger) (*BitbucketClient, error) {
	if *e2e.BitbucketWorkspace == "" {
		return nil, errors.New("bitbucket workspace cannot be empty; set with -bitbucket-workspace flag or E2E_BITBUCKET_WORKSPACE env var")
	}
	client := &BitbucketClient{
		logger:     logger,
		workspace:  *e2e.BitbucketWorkspace,
		repoSuffix: repoSuffix,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	var err error
	if client.oauthKey, err = FetchCloudSecret("bitbucket-oauth-key"); err != nil {
		return client, err
	}
	if client.oauthSecret, err = FetchCloudSecret("bitbucket-oauth-secret"); err != nil {
		return client, err
	}
	if client.refreshToken, err = FetchCloudSecret("bitbucket-refresh-token"); err != nil {
		return client, err
	}
	return client, nil
}

func (b *BitbucketClient) fullName(name string) string {
	return util.SanitizeBitbucketRepoName(b.repoSuffix, name)
}

// Type returns the provider type.
func (b *BitbucketClient) Type() string {
	return e2e.Bitbucket
}

// RemoteURL returns the Git URL for the Bitbucket repository.
// name refers to the repo name in the format of <NAMESPACE>/<NAME> of RootSync|RepoSync.
func (b *BitbucketClient) RemoteURL(name string) (string, error) {
	return b.SyncURL(name), nil
}

// SyncURL returns a URL for Config Sync to sync from.
// name refers to the repo name in the format of <NAMESPACE>/<NAME> of RootSync|RepoSync.
// The Bitbucket Rest API doesn't allow slash in the repository name, so slash has to be replaced with dash in the name.
func (b *BitbucketClient) SyncURL(name string) string {
	return fmt.Sprintf("git@bitbucket.org:%s/%s", b.workspace, strings.ReplaceAll(name, "/", "-"))
}

// CreateRepository calls the POST API to create a remote repository on Bitbucket.
// The remote repo name is unique with a prefix of the local name.
func (b *BitbucketClient) CreateRepository(name string) (string, error) {
	fullName := b.fullName(name)
	repoURL := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s", b.workspace, fullName)

	// Create a remote repository on demand with a random localName.
	accessToken, err := b.refreshAccessToken()
	if err != nil {
		return "", err
	}

	// Check if repository already exists
	resp, err := b.sendRequest(http.MethodGet, repoURL, accessToken, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusOK {
		return fullName, nil
	}

	payload := map[string]any{
		"scm": "git",
		"project": map[string]string{
			"key": bitbucketProject,
		},
		"is_private": true,
	}

	// Marshal the data into JSON format
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Create new Bitbucket repository
	resp, err = b.sendRequest(http.MethodPost, repoURL, accessToken, bytes.NewReader(jsonPayload))

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error as just printing it might be missed.
			b.logger.Infof("failed to close response body: %v\n", closeErr)
		}
	}()

	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create repository: status %d: %s", resp.StatusCode, string(body))
	}

	return fullName, nil
}

// sendRequest sends an HTTP request to the Bitbucket API.
func (b *BitbucketClient) sendRequest(method, url, accessToken string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	return resp, nil
}

// DeleteRepositories is a no-op because Bitbucket repo names are determined by the
// test cluster name and RSync namespace and name, so they can be reset and reused
// across test runs
func (b *BitbucketClient) DeleteRepositories(_ ...string) error {
	b.logger.Info("[BITBUCKET] Skip deletion of repos")
	return nil
}

// DeleteObsoleteRepos is a no-op because Bitbucket repo names are determined by the
// test cluster name and RSync namespace and name, so it can be reused if it
// failed to be deleted after the test.
func (b *BitbucketClient) DeleteObsoleteRepos() error {
	return nil
}

func (b *BitbucketClient) refreshAccessToken() (string, error) {
	tokenURL := "https://bitbucket.org/site/oauth2/access_token"
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", b.refreshToken)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token refresh request: %w", err)
	}

	req.SetBasicAuth(b.oauthKey, b.oauthSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending token refresh request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			b.logger.Infof("failed to close response body: %v\n", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token refresh response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh token failed with status %d", resp.StatusCode)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(body, &output); err != nil {
		return "", fmt.Errorf("unmarshalling refresh token response: %w", err)
	}

	accessToken, ok := output["access_token"]
	if !ok {
		return "", fmt.Errorf("no access_token in response")
	}

	accessTokenStr, ok := accessToken.(string)
	if !ok {
		return "", fmt.Errorf("access_token is not a string: %T", accessToken)
	}

	return accessTokenStr, nil
}

// FetchCloudSecret fetches secret from Google Cloud Secret Manager.
func FetchCloudSecret(name string) (string, error) {
	if *e2e.GCPProject == "" {
		return "", fmt.Errorf("gcp-project must be set to fetch cloud secret")
	}
	out, err := exec.Command("gcloud", "secrets", "versions",
		"access", "latest", "--project", *e2e.GCPProject, "--secret", name).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", string(out), err)
	}
	return string(out), nil
}

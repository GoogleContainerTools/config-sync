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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/GoogleContainerTools/config-sync/e2e"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/gitproviders/util"
	"github.com/GoogleContainerTools/config-sync/e2e/nomostest/testlogger"
)

const (
	groupID              = 15698791
	groupName            = "configsync"
	gitlabRequestTimeout = 10 * time.Second
)

// GitlabClient is the client that will call Gitlab REST APIs.
type GitlabClient struct {
	privateToken string
	logger       *testlogger.TestLogger
	// repoSuffix is used to avoid overlap
	repoSuffix string
	httpClient *http.Client
}

// newGitlabClient instantiates a new GitlabClient.
func newGitlabClient(repoSuffix string, logger *testlogger.TestLogger) (*GitlabClient, error) {
	client := &GitlabClient{
		logger:     logger,
		repoSuffix: repoSuffix,
		httpClient: &http.Client{},
	}

	var err error

	if client.privateToken, err = FetchCloudSecret("gitlab-private-token"); err != nil {
		return client, err
	}
	return client, nil
}

func (g *GitlabClient) fullName(name string) string {
	return util.SanitizeGitlabRepoName(g.repoSuffix, name)
}

// Type returns the git provider type
func (g *GitlabClient) Type() string {
	return e2e.GitLab
}

// RemoteURL returns the Git URL for the Gitlab project repository.
func (g *GitlabClient) RemoteURL(name string) (string, error) {
	return g.SyncURL(name), nil
}

// SyncURL returns a URL for Config Sync to sync from.
func (g *GitlabClient) SyncURL(name string) string {
	return fmt.Sprintf("git@gitlab.com:%s/%s.git", groupName, name)
}

// CreateRepository calls the POST API to create a project/repository on Gitlab.
// The remote repo name is unique with a prefix of the local name.
func (g *GitlabClient) CreateRepository(name string) (string, error) {
	fullName := g.fullName(name)
	repoURL := fmt.Sprintf("https://gitlab.com/api/v4/projects?search=%s", fullName)

	// Check if the repository already exists
	getRequestCtx, getRequestCancel := context.WithTimeout(context.Background(), gitlabRequestTimeout)
	defer getRequestCancel()
	resp, err := g.sendRequest(getRequestCtx, http.MethodGet, repoURL, nil)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error reading response body: %w", err)
		}

		var output []map[string]interface{}
		if err := json.Unmarshal(body, &output); err != nil {
			return "", fmt.Errorf("unmarshalling project search response: %w", err)
		}

		// the assumption is that our project name is unique, so we'll get exactly 1 result
		if len(output) > 0 {
			return fullName, nil

		}
	} else {
		return "", fmt.Errorf("failed to check if repository exists: status %d", resp.StatusCode)
	}

	// Creates repository
	repoURL = fmt.Sprintf("https://gitlab.com/api/v4/projects?name=%s&namespace_id=%d&initialize_with_readme=true", fullName, groupID)
	postRequestCtx, postRequestCancel := context.WithTimeout(context.Background(), gitlabRequestTimeout)
	resp, err = g.sendRequest(postRequestCtx, http.MethodPost, repoURL, nil)

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			g.logger.Infof("failed to close response body: %v\n", closeErr)
		}
		postRequestCancel()
	}()
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create repository: status %d: %s", resp.StatusCode, string(body))
	}

	return fullName, nil
}

// DeleteRepositories is a no-op because Gitlab repo names are determined by the
// test cluster name and RSync namespace and name, so they can be reset and reused
// across test runs
func (g *GitlabClient) DeleteRepositories(_ ...string) error {
	g.logger.Info("[Gitlab] Skip deletion of repos")
	return nil
}

// DeleteObsoleteRepos is a no-op because Gitlab repo names are determined by the
// test cluster name and RSync namespace and name, so it can be reused if it
// failed to be deleted after the test.
func (g *GitlabClient) DeleteObsoleteRepos() error {
	return nil
}

// sendRequest sends an HTTP request to the Gitlab API.
func (g *GitlabClient) sendRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Add("PRIVATE-TOKEN", g.privateToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	return resp, nil
}

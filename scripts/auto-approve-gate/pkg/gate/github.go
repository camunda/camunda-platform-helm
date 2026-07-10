// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type GitHubClient struct {
	Token      string
	Repository string
	HTTPClient *http.Client
	baseURL    string
}

func (c *GitHubClient) apiBaseURL() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return "https://api.github.com"
}

func NewGitHubClientFromEnv() (*GitHubClient, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN or GH_TOKEN environment variable is required")
	}

	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY environment variable is required")
	}

	return &GitHubClient{
		Token:      token,
		Repository: repo,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

type pullRequestResponse struct {
	ChangedFiles int `json:"changed_files"`
}

type pullRequestFileResponse struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
}

func (c *GitHubClient) GetPullRequest(pr int) (PRMeta, error) {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d", c.apiBaseURL(), c.Repository, pr)
	var resp pullRequestResponse
	if err := c.getJSON(url, &resp); err != nil {
		return PRMeta{}, err
	}
	return PRMeta{ChangedFiles: resp.ChangedFiles}, nil
}

func (c *GitHubClient) ListPullRequestFiles(pr int) ([]PRFile, error) {
	var all []PRFile
	page := 1
	for {
		url := fmt.Sprintf("%s/repos/%s/pulls/%d/files?per_page=100&page=%d",
			c.apiBaseURL(), c.Repository, pr, page)
		var batch []pullRequestFileResponse
		if err := c.getJSON(url, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, f := range batch {
			all = append(all, PRFile{
				Filename:         f.Filename,
				PreviousFilename: f.PreviousFilename,
			})
		}
		page++
	}
	return all, nil
}

func (c *GitHubClient) getJSON(url string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

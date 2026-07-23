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
	"bytes"
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
	Head         struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type pullRequestFileResponse struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
}

type Review struct {
	ID        int64
	UserLogin string
	CommitID  string
	State     string
}

type reviewResponse struct {
	ID   int64 `json:"id"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	CommitID string `json:"commit_id"`
	State    string `json:"state"`
}

func (c *GitHubClient) GetPullRequest(pr int) (PRMeta, error) {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d", c.apiBaseURL(), c.Repository, pr)
	var resp pullRequestResponse
	if err := c.getJSON(url, &resp); err != nil {
		return PRMeta{}, err
	}
	return PRMeta{ChangedFiles: resp.ChangedFiles}, nil
}

func (c *GitHubClient) GetPullRequestHeadSHA(pr int) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d", c.apiBaseURL(), c.Repository, pr)
	var resp pullRequestResponse
	if err := c.getJSON(url, &resp); err != nil {
		return "", err
	}
	return resp.Head.SHA, nil
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

func (c *GitHubClient) ListReviews(pr int) ([]Review, error) {
	var all []Review
	page := 1
	for {
		url := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews?per_page=100&page=%d",
			c.apiBaseURL(), c.Repository, pr, page)
		var batch []reviewResponse
		if err := c.getJSON(url, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, r := range batch {
			all = append(all, Review{
				ID:        r.ID,
				UserLogin: r.User.Login,
				CommitID:  r.CommitID,
				State:     r.State,
			})
		}
		page++
	}
	return all, nil
}

func (c *GitHubClient) CreateReview(pr int, commitID, event, body string) error {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews", c.apiBaseURL(), c.Repository, pr)
	reqBody := map[string]string{
		"commit_id": commitID,
		"event":     event,
		"body":      body,
	}
	return c.doJSON(http.MethodPost, url, reqBody, nil)
}

func (c *GitHubClient) DismissReview(pr int, reviewID int64, message string) error {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews/%d/dismissals", c.apiBaseURL(), c.Repository, pr, reviewID)
	reqBody := map[string]string{
		"message": message,
		"event":   "DISMISS",
	}
	return c.doJSON(http.MethodPut, url, reqBody, nil)
}

func (c *GitHubClient) getJSON(url string, dest any) error {
	return c.doJSON(http.MethodGet, url, nil, dest)
}

func (c *GitHubClient) doJSON(method, url string, body any, dest any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "token "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(respBody))
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

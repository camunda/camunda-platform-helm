// Copyright 2026 Camunda Services GmbH
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

package retagger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const ghAPIBase = "https://api.github.com"

// RealGitHubClient implements GitHubClient against the GitHub REST API using
// Bearer token auth. The Do field is injectable so tests can exercise all code
// paths without a real network.
type RealGitHubClient struct {
	Token string
	Do    func(*http.Request) (*http.Response, error)
}

// NewGitHubClient returns a RealGitHubClient reading the token from GITHUB_TOKEN.
func NewGitHubClient() *RealGitHubClient {
	c := &RealGitHubClient{Token: os.Getenv("GITHUB_TOKEN")}
	c.Do = http.DefaultClient.Do
	return c
}

func (c *RealGitHubClient) newRequest(method, url string, body []byte) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *RealGitHubClient) doJSON(req *http.Request, out any) (int, error) {
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		io.Copy(io.Discard, resp.Body)
		return http.StatusNotFound, nil
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(b))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	} else {
		io.Copy(io.Discard, resp.Body)
	}
	return resp.StatusCode, nil
}

type ghTagRef struct {
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

// CommitSHA returns the commit SHA the tag currently points to, or "" if the
// tag does not exist. Annotated tags (type "tag") are resolved to their
// underlying commit via a second API call.
func (c *RealGitHubClient) CommitSHA(repo, tag string) (string, error) {
	req, err := c.newRequest(http.MethodGet,
		fmt.Sprintf("%s/repos/%s/git/refs/tags/%s", ghAPIBase, repo, tag), nil)
	if err != nil {
		return "", err
	}
	var ref ghTagRef
	code, err := c.doJSON(req, &ref)
	if err != nil {
		return "", err
	}
	if code == http.StatusNotFound {
		return "", nil
	}
	if ref.Object.Type != "tag" {
		return ref.Object.SHA, nil
	}
	// Annotated tag: resolve tag object → commit SHA.
	req2, err := c.newRequest(http.MethodGet,
		fmt.Sprintf("%s/repos/%s/git/tags/%s", ghAPIBase, repo, ref.Object.SHA), nil)
	if err != nil {
		return "", err
	}
	var obj ghTagRef
	if _, err := c.doJSON(req2, &obj); err != nil {
		return "", fmt.Errorf("resolve annotated tag %s: %w", tag, err)
	}
	return obj.Object.SHA, nil
}

// MoveTag force-updates tag to point at sha.
func (c *RealGitHubClient) MoveTag(repo, tag, sha string) error {
	body, _ := json.Marshal(map[string]any{"sha": sha, "force": true})
	req, err := c.newRequest(http.MethodPatch,
		fmt.Sprintf("%s/repos/%s/git/refs/tags/%s", ghAPIBase, repo, tag), body)
	if err != nil {
		return err
	}
	_, err = c.doJSON(req, nil)
	return err
}

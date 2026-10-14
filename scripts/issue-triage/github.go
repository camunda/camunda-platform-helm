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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type githubClient struct {
	http              *http.Client
	base, token, repo string
	members           map[string]bool
}

type apiError struct {
	status int
	path   string
}

func (e *apiError) Error() string { return fmt.Sprintf("GitHub %s returned HTTP %d", e.path, e.status) }

func (c *githubClient) request(method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub %s request failed: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{resp.StatusCode, path}
	}
	if output != nil {
		return json.NewDecoder(resp.Body).Decode(output)
	}
	return nil
}

type author struct{ Login, Type string }
type label struct{ Name string }
type issue struct {
	Number             int
	Title, Body, State string
	HTMLURL            string `json:"html_url"`
	User               author
	Labels             []label
	PullRequest        json.RawMessage `json:"pull_request,omitempty"`
}

func (i issue) has(name string) bool {
	for _, l := range i.Labels {
		if strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}
func (i issue) prefixed(prefix string) bool {
	for _, l := range i.Labels {
		if strings.HasPrefix(strings.ToLower(l.Name), prefix) {
			return true
		}
	}
	return false
}
func (i issue) isPullRequest() bool {
	return len(i.PullRequest) > 0 && string(i.PullRequest) != "null"
}

func (i issue) assessable() bool {
	return i.State == "open" && !i.has("TRIAGE:COMPLETED") && !i.has("triage:manual")
}
func (c *githubClient) issuePath(number int) string {
	return "/repos/" + c.repo + "/issues/" + strconv.Itoa(number)
}
func (c *githubClient) getIssue(number int) (issue, error) {
	var i issue
	err := c.request("GET", c.issuePath(number), nil, &i)
	if err == nil && (i.Number != number || i.User.Login == "" || i.isPullRequest()) {
		err = fmt.Errorf("invalid issue #%d", number)
	}
	return i, err
}

func (c *githubClient) internal(i issue) (bool, error) {
	if i.User.Type == "Bot" {
		return false, nil
	}
	if i.User.Login == "" {
		return false, fmt.Errorf("issue creator is missing")
	}
	if c.members == nil {
		members := map[string]bool{}
		owner := strings.Split(c.repo, "/")[0]
		for _, team := range []string{"distribution", "infrastructure-experience"} {
			for page := 1; ; page++ {
				var users []author
				path := fmt.Sprintf("/orgs/%s/teams/%s/members?per_page=100&page=%d", owner, team, page)
				if err := c.request("GET", path, nil, &users); err != nil {
					return false, fmt.Errorf("resolve team %s (requires Members: read): %w", team, err)
				}
				if page == 1 && len(users) == 0 {
					return false, fmt.Errorf("team %s has no visible members", team)
				}
				for _, user := range users {
					if user.Login == "" {
						return false, fmt.Errorf("team %s returned an invalid member", team)
					}
					members[strings.ToLower(user.Login)] = true
				}
				if len(users) < 100 {
					break
				}
			}
		}
		c.members = members
	}
	return c.members[strings.ToLower(i.User.Login)], nil
}

func (c *githubClient) addLabels(number int, labels ...string) error {
	if len(labels) == 0 {
		return nil
	}
	return c.request("POST", c.issuePath(number)+"/labels", map[string]any{"labels": labels}, nil)
}
func (c *githubClient) removeLabel(number int, name string) error {
	return c.request("DELETE", c.issuePath(number)+"/labels/"+url.PathEscape(name), nil, nil)
}
func (c *githubClient) ensureLabel(name string) error {
	path := "/repos/" + c.repo + "/labels/" + url.PathEscape(name)
	err := c.request("GET", path, nil, nil)
	if err == nil {
		return nil
	}
	if e, ok := err.(*apiError); !ok || e.status != 404 {
		return err
	}
	err = c.request("POST", "/repos/"+c.repo+"/labels", map[string]string{"name": name, "color": "ededed", "description": "Issue intake delivery state"}, nil)
	if e, ok := err.(*apiError); ok && e.status == 422 {
		return c.request("GET", path, nil, nil)
	}
	return err
}
func (c *githubClient) comment(number int, body string) error {
	return c.request("POST", c.issuePath(number)+"/comments", map[string]string{"body": body}, nil)
}

func (c *githubClient) graphql(query string, variables map[string]any, output any) error {
	var result struct {
		Data   json.RawMessage
		Errors []struct{ Message string }
	}
	if err := c.request("POST", "/graphql", map[string]any{"query": query, "variables": variables}, &result); err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("GraphQL: %s", result.Errors[0].Message)
	}
	if len(result.Data) == 0 || string(result.Data) == "null" {
		return fmt.Errorf("GraphQL returned no data")
	}
	if output == nil {
		return nil
	}
	return json.Unmarshal(result.Data, output)
}

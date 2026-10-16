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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"scripts/camunda-core/pkg/ghactions"
)

type client struct {
	http              *http.Client
	base, token, repo string
}

func (c client) get(path string, result any) error {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

func (c client) internal(number int) (bool, error) {
	var issue struct {
		User        struct{ Login, Type string }
		PullRequest json.RawMessage `json:"pull_request"`
	}
	if err := c.get(fmt.Sprintf("/repos/%s/issues/%d", c.repo, number), &issue); err != nil {
		return false, err
	}
	if issue.User.Login == "" || len(issue.PullRequest) > 0 && string(issue.PullRequest) != "null" {
		return false, fmt.Errorf("invalid issue creator or issue is a pull request")
	}
	if issue.User.Type == "Bot" {
		return false, nil
	}
	internal := false
	for _, team := range []string{"distribution", "infrastructure-experience"} {
		for page := 1; ; page++ {
			var members []struct{ Login string }
			path := fmt.Sprintf("/orgs/%s/teams/%s/members?per_page=100&page=%d", strings.Split(c.repo, "/")[0], team, page)
			if err := c.get(path, &members); err != nil {
				return false, err
			}
			if page == 1 && len(members) == 0 {
				return false, fmt.Errorf("team %s has no visible members", team)
			}
			for _, member := range members {
				if member.Login == "" {
					return false, fmt.Errorf("team %s returned invalid member", team)
				}
				internal = internal || strings.EqualFold(member.Login, issue.User.Login)
			}
			if len(members) < 100 {
				break
			}
		}
	}
	return internal, nil
}

func run() error {
	repo, token := os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GH_TOKEN")
	number, err := strconv.Atoi(os.Getenv("ISSUE_NUMBER"))
	if err != nil || number <= 0 || token == "" || !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repo) {
		return fmt.Errorf("GH_TOKEN, GITHUB_REPOSITORY (owner/repo), and positive ISSUE_NUMBER are required")
	}
	base := os.Getenv("GITHUB_API_URL")
	if base == "" {
		base = "https://api.github.com"
	}
	c := client{http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, base: strings.TrimRight(base, "/"), token: token, repo: repo}
	internal, err := c.internal(number)
	if err != nil {
		return err
	}
	if os.Getenv("GITHUB_OUTPUT") != "" || os.Getenv("GITHUB_ACTIONS") == "true" {
		if err := ghactions.NewGitHubOutput().Set("internal", strconv.FormatBool(internal)); err != nil {
			return err
		}
	}
	fmt.Println(internal)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

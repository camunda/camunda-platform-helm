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
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func appendFile(path, text string) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, text)
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if summaryErr := appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), "\nIssue triage failed: "+err.Error()+"\n"); summaryErr != nil {
			fmt.Fprintln(os.Stderr, summaryErr)
		}
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: issue-triage select|classify|route|triage")
	}
	repo := os.Getenv("GITHUB_REPOSITORY")
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repo) {
		return fmt.Errorf("GITHUB_REPOSITORY must be owner/repo")
	}
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		return fmt.Errorf("GH_TOKEN is required")
	}
	base := os.Getenv("GITHUB_API_URL")
	if base == "" {
		base = "https://api.github.com"
	}
	httpClient := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	c := &githubClient{http: httpClient, base: strings.TrimRight(base, "/"), token: token, repo: repo}
	project := 33
	if v := os.Getenv("PROJECT_NUMBER"); v != "" {
		var err error
		project, err = strconv.Atoi(v)
		if err != nil || project <= 0 {
			return fmt.Errorf("invalid PROJECT_NUMBER")
		}
	}
	number := 0
	if v := os.Getenv("ISSUE_NUMBER"); v != "" {
		var err error
		number, err = strconv.Atoi(v)
		if err != nil || number <= 0 {
			return fmt.Errorf("invalid ISSUE_NUMBER")
		}
	}
	if os.Args[1] == "select" {
		numbers := []int{number}
		if number == 0 {
			var err error
			numbers, err = c.candidates(project)
			if err != nil {
				return err
			}
		}
		b, err := json.Marshal(numbers)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return appendFile(os.Getenv("GITHUB_OUTPUT"), fmt.Sprintf("issues=%s\nhas_issues=%t\n", b, len(numbers) > 0))
	}
	if number == 0 {
		return fmt.Errorf("ISSUE_NUMBER is required")
	}
	if os.Args[1] == "triage" {
		g := &gleanClient{http: httpClient, endpoint: "https://camunda-be.glean.com/rest/api/v1/chat", token: os.Getenv("GLEAN_API_TOKEN")}
		var summary strings.Builder
		t := &triager{github: c, assess: g.assess, slack: &slackClient{http: httpClient, webhook: os.Getenv("SLACK_WEBHOOK")}, summary: &summary, project: project}
		err := t.run(number)
		fmt.Print(summary.String())
		if writeErr := appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), summary.String()); writeErr != nil {
			return fmt.Errorf("summary: %w (triage: %v)", writeErr, err)
		}
		return err
	}
	i, err := c.getIssue(number)
	if err != nil {
		return err
	}
	internal, err := c.internal(i)
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "classify":
		fmt.Printf("Issue #%d internal=%t\n", number, internal)
		return appendFile(os.Getenv("GITHUB_OUTPUT"), fmt.Sprintf("internal=%t\n", internal))
	case "route":
		if internal {
			return c.backlog(number, project, true)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

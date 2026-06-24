// Copyright 2025 Camunda Services GmbH
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
	"os/exec"
	"strings"
)

type ghCLI struct {
	repo string
}

func newGHCLI(repo string) *ghCLI { return &ghCLI{repo: repo} }

func (c *ghCLI) run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh %s: %v: %s", strings.Join(args, " "), err,
			strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *ghCLI) FindRun(workflow, sha, event string) (string, error) {
	out, err := c.run("run", "list",
		"--repo", c.repo,
		"--workflow", workflow,
		"--commit", sha,
		"--event", event,
		"--limit", "1",
		"--json", "databaseId",
		"--jq", ".[0].databaseId // empty")
	if err != nil {
		return "", err
	}
	return out, nil
}

func (c *ghCLI) RunURL(runID string) (string, error) {
	return c.run("run", "view", runID,
		"--repo", c.repo,
		"--json", "url",
		"--jq", ".url")
}

func (c *ghCLI) RunAttempt(runID string) (int, error) {
	out, err := c.run("api",
		fmt.Sprintf("repos/%s/actions/runs/%s", c.repo, runID),
		"--jq", ".run_attempt")
	if err != nil {
		return 0, err
	}
	var n int
	if err := json.Unmarshal([]byte(out), &n); err != nil {
		return 0, fmt.Errorf("parse run_attempt %q: %v", out, err)
	}
	return n, nil
}

func (c *ghCLI) AttemptStatus(runID string, attempt int) (string, error) {
	return c.run("run", "view", runID,
		"--repo", c.repo,
		"--attempt", fmt.Sprintf("%d", attempt),
		"--json", "status",
		"--jq", ".status")
}

func (c *ghCLI) AttemptConclusion(runID string, attempt int) (string, error) {
	return c.run("run", "view", runID,
		"--repo", c.repo,
		"--attempt", fmt.Sprintf("%d", attempt),
		"--json", "conclusion",
		"--jq", ".conclusion")
}

func (c *ghCLI) RerunFailed(runID string) error {
	_, err := c.run("run", "rerun", runID,
		"--repo", c.repo,
		"--failed")
	return err
}

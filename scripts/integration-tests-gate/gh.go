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
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ghCLI struct {
	repo    string
	timeout time.Duration
}

type workflowRun struct {
	DatabaseID int64  `json:"databaseId"`
	Conclusion string `json:"conclusion"`
}

func newGHCLI(repo string, timeout time.Duration) *ghCLI {
	return &ghCLI{repo: repo, timeout: timeout}
}

func (c *ghCLI) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("gh %s: timed out after %s",
				strings.Join(args, " "), c.timeout)
		}
		return "", fmt.Errorf("gh %s: %v: %s",
			strings.Join(args, " "), err,
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
		"--limit", "10",
		"--json", "databaseId,conclusion")
	if err != nil {
		return "", err
	}
	var runs []workflowRun
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		return "", fmt.Errorf("parse workflow runs: %w", err)
	}
	return selectWorkflowRun(runs), nil
}

func selectWorkflowRun(runs []workflowRun) string {
	for _, run := range runs {
		if run.Conclusion != "cancelled" {
			return strconv.FormatInt(run.DatabaseID, 10)
		}
	}
	if len(runs) > 0 {
		return strconv.FormatInt(runs[0].DatabaseID, 10)
	}
	return ""
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
	n, err := strconv.Atoi(out)
	if err != nil {
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

func (c *ghCLI) AttemptJobConclusions(runID string, attempt int) ([]string, error) {
	out, err := c.run("api", "--paginate",
		fmt.Sprintf("repos/%s/actions/runs/%s/attempts/%d/jobs?per_page=100", c.repo, runID, attempt),
		"--jq", ".jobs[].conclusion")
	if err != nil {
		return nil, err
	}
	var conclusions []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			conclusions = append(conclusions, line)
		}
	}
	return conclusions, nil
}

func (c *ghCLI) Rerun(runID string) error {
	_, err := c.run("run", "rerun", runID,
		"--repo", c.repo)
	if err != nil && strings.Contains(err.Error(), "already running") {
		return ErrRerunAlreadyRunning
	}
	return err
}

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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowEntryPointsUseCreator(t *testing.T) {
	for _, event := range []string{"issues", "workflow_dispatch", "workflow_call"} {
		for _, actor := range []string{"outside", "distro", "github-actions[bot]"} {
			t.Run(event+"/"+actor, func(t *testing.T) {
				f, _ := newFixture(t)
				f.issue.User.Login = "infraex"
				t.Setenv("GITHUB_EVENT_NAME", event)
				t.Setenv("GITHUB_ACTOR", actor)
				t.Setenv("GITHUB_REPOSITORY", "camunda/repo")
				t.Setenv("GITHUB_API_URL", f.server.URL)
				t.Setenv("GH_TOKEN", "fixture-token")
				t.Setenv("ISSUE_NUMBER", "42")
				t.Setenv("PROJECT_NUMBER", "33")
				output := filepath.Join(t.TempDir(), "output")
				t.Setenv("GITHUB_OUTPUT", output)
				args := os.Args
				t.Cleanup(func() { os.Args = args })
				os.Args = []string{"issue-triage", "select"}
				if err := run(); err != nil {
					t.Fatal(err)
				}
				os.Args[1] = "classify"
				if err := run(); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), "issues=[42]\nhas_issues=true\n") || !strings.Contains(string(data), "internal=true\n") {
					t.Fatalf("unexpected outputs: %s", data)
				}
				os.Args[1] = "route"
				if err := run(); err != nil {
					t.Fatal(err)
				}
				if f.status != "📋 Backlog" {
					t.Fatal("internal issue not routed")
				}
			})
		}
	}
}

func TestInvalidIssueInput(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "camunda/repo")
	t.Setenv("GH_TOKEN", "fixture-token")
	t.Setenv("PROJECT_NUMBER", "33")
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"issue-triage", "select"}
	for _, input := range []string{"-1", "0", "42; arbitrary-command", "42 43", "not-a-number"} {
		t.Setenv("ISSUE_NUMBER", input)
		if err := run(); err == nil {
			t.Fatalf("accepted invalid issue input %q", input)
		}
	}
}

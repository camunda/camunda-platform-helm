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
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWorkflowBacklogRouting(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Fatal("workflow regression tests require jq:", err)
	}
	data, err := os.ReadFile("../../.github/workflows/triage-ai-issue.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	start := strings.Index(workflow, "            # Apply labels")
	end := strings.Index(workflow, "            # Send Slack notification")
	if start < 0 || end <= start {
		t.Fatal("triage workflow block not found")
	}
	block := workflow[start:end]
	for _, tc := range []struct {
		name, status                                        string
		internal, missingItem, missingOption, mutationError bool
		wantMove, wantComplete, wantSlack                   bool
	}{
		{name: "internal Inbox", status: "📥 Inbox", internal: true, wantMove: true, wantComplete: true},
		{name: "internal unset", internal: true, wantMove: true, wantComplete: true},
		{name: "internal Backlog", status: "📋 Backlog", internal: true, wantComplete: true},
		{name: "internal in progress", status: "In Progress", internal: true, wantComplete: true},
		{name: "internal missing item", internal: true, missingItem: true},
		{name: "internal missing option", internal: true, missingOption: true},
		{name: "internal mutation failure", internal: true, mutationError: true, wantMove: true},
		{name: "external Inbox", status: "📥 Inbox", wantMove: true, wantComplete: true, wantSlack: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := []any{}
			if !tc.missingItem {
				items = append(items, map[string]any{"id": "item", "project": map[string]any{"number": 33}, "fieldValueByName": map[string]string{"name": tc.status}})
			}
			options := []any{}
			if !tc.missingOption {
				options = append(options, map[string]string{"id": "backlog", "name": "📋 Backlog"})
			}
			fixture, err := json.Marshal(map[string]any{"data": map[string]any{
				"repository":   map[string]any{"issue": map[string]any{"projectItems": map[string]any{"nodes": items}}},
				"organization": map[string]any{"projectV2": map[string]any{"id": "project", "field": map[string]any{"id": "status", "options": options}}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			script := `set -euo pipefail
gh() {
  if [[ "$1" == issue && "$2" == view ]]; then
    printf 'severity/low\nlikelihood/low\n'
  elif [[ "$1" == issue && "$2" == edit ]]; then
    printf 'EDIT %s\n' "$*" >&2
  elif [[ "$*" == *'query=mutation('* ]]; then
    printf 'MOVE\n' >&2
    [[ "$MUTATION_ERROR" == false ]]
  elif [[ "$1" == api && "$2" == graphql ]]; then
    printf '%s\n' "$FIXTURE"
  else
    return 99
  fi
}
OWNER=camunda REPO=team-distribution PROJECT_NUMBER=33
SEVERITY=critical LIKELIHOOD=high KIND=chore MISSING_INFO=false
for ISSUE_NUMBER in 42; do
` + block + "\nprintf 'SLACK_PATH\\n'\ndone\n"
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "FIXTURE="+string(fixture), "INTERNAL="+boolString(tc.internal), "MUTATION_ERROR="+boolString(tc.mutationError))
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.mutationError {
				t.Fatalf("error=%v output=%s", err, output)
			}
			text := string(output)
			for marker, want := range map[string]bool{"MOVE\n": tc.wantMove, "--add-label TRIAGE:COMPLETED": tc.wantComplete, "SLACK_PATH": tc.wantSlack} {
				if strings.Contains(text, marker) != want {
					t.Errorf("marker %q want=%t output=%s", marker, want, text)
				}
			}
			if tc.internal && strings.Contains(text, "--add-label severity/critical") {
				t.Error("existing priority overwritten")
			}
			if tc.internal && tc.wantMove && tc.wantComplete && strings.Index(text, "MOVE\n") > strings.Index(text, "--add-label TRIAGE:COMPLETED") {
				t.Error("marked complete before routing")
			}
		})
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

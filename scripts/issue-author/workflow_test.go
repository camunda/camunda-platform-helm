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
	"path/filepath"
	"strings"
	"testing"
)

func TestInitialRoutingCondition(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/triage-assign-urgency-to-issue.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	start := strings.Index(workflow, "      - name: Fetch current status for initial routing")
	if start < 0 {
		t.Fatal("initial route missing")
	}
	end := strings.Index(workflow[start:], "      - name: Route new internal issue to Backlog") + start
	block := strings.SplitN(workflow[start:end], "        run: |\n", 2)[1]
	step := workflow[end:]
	if !strings.Contains(step, "content_id: ${{ steps.current_status.outputs.content_id }}") || !strings.Contains(step, "value: 📋 Backlog") || !strings.Contains(step, "if: steps.current_status.outputs.route == 'true'") {
		t.Fatal("incorrect project action wiring")
	}
	if !strings.Contains(workflow, "if: inputs.route_new_internal && !inputs.dry_run") || !strings.Contains(workflow[start:end], "if: steps.author.outputs.internal == 'true'") {
		t.Fatal("initial routing must exclude dry runs and external authors")
	}
	for _, tc := range []struct {
		name, status                                string
		missingProject, closed, apiError, wantRoute bool
	}{
		{name: "unset", wantRoute: true},
		{name: "inbox", status: "📥 Inbox", wantRoute: true},
		{name: "moved to progress after original fetch", status: "In Progress"},
		{name: "already backlog", status: "📋 Backlog"},
		{name: "removed from project", missingProject: true},
		{name: "closed after original fetch", status: "📥 Inbox", closed: true},
		{name: "API failure", apiError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := "OPEN"
			if tc.closed {
				state = "CLOSED"
			}
			project := 33
			if tc.missingProject {
				project = 99
			}
			fixture, err := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "issue", "state": state, "projectItems": map[string]any{"nodes": []any{map[string]any{
					"project": map[string]int{"number": project}, "fieldValueByName": map[string]string{"name": tc.status},
				}}},
			}}}})
			if err != nil {
				t.Fatal(err)
			}
			script := `gh() {
  [[ "$*" == *'fieldValueByName(name:"Status")'* ]] || return 99
  if [[ "$API_ERROR" == true ]]; then printf '{"errors":[{"message":"failed"}]}'; return 1; fi
  printf '%s\n' "$FIXTURE"
}
` + block
			outputPath := filepath.Join(t.TempDir(), "output")
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "FIXTURE="+string(fixture), "API_ERROR="+boolString(tc.apiError), "GITHUB_REPOSITORY=camunda/repo", "ISSUE_NUMBER=42", "PROJECT_NUMBER=33", "CURRENT_STATUS=📥 Inbox", "GITHUB_OUTPUT="+outputPath)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.apiError {
				t.Fatalf("%v: %s", err, output)
			}
			outputs, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(outputs), "route=true") != tc.wantRoute {
				t.Fatalf("outputs=%s wantRoute=%t", outputs, tc.wantRoute)
			}
			if !tc.apiError && !strings.Contains(string(outputs), "content_id=issue") {
				t.Fatalf("missing fresh issue ID: %s", outputs)
			}
		})
	}
	if strings.Index(workflow, "      - name: Classify issue creator for routing") < strings.Index(workflow, "      - name: Update Urgency") {
		t.Fatal("classification blocks urgency updates")
	}
}

func TestUrgencySerialization(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/triage-assign-urgency-to-issue.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	if strings.Contains(workflow, "\nconcurrency:") {
		t.Fatal("workflow cancellation can discard initial routing")
	}
	jobs := strings.SplitN(workflow, "\n  route-new-internal:", 2)
	if len(jobs) != 2 || strings.Contains(jobs[1], "needs:") {
		t.Fatal("initial routing must run independently of urgency reconciliation")
	}
	concurrency := "    concurrency:\n      group: update-urgency-${{ inputs.issue_number || github.event.inputs.issue_number || github.event.issue.number }}\n      cancel-in-progress: false"
	if !strings.Contains(jobs[0], concurrency) {
		t.Fatal("all urgency entry points must share a non-cancelling job concurrency group")
	}
	fetch := strings.Index(jobs[0], "      - name: Fetch issue and project data")
	write := strings.Index(jobs[0], "      - name: Update Urgency")
	if fetch < strings.Index(jobs[0], concurrency) || write <= fetch || !strings.Contains(jobs[0][fetch:write], "labels(first:100){ nodes { name } }") {
		t.Fatal("labels must be fetched within the serialized job before the urgency write")
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

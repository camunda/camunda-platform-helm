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
		existingKind                                        bool
		wantMove, wantComplete, wantSlack                   bool
	}{
		{name: "internal Inbox", status: "📥 Inbox", internal: true, wantMove: true, wantComplete: true},
		{name: "internal existing kind", status: "📥 Inbox", internal: true, existingKind: true, wantMove: true, wantComplete: true},
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
    if [[ "$EXISTING_KIND" == true ]]; then printf 'kind/bug\n'; fi
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
			cmd.Env = append(os.Environ(), "FIXTURE="+string(fixture), "INTERNAL="+boolString(tc.internal), "MUTATION_ERROR="+boolString(tc.mutationError), "EXISTING_KIND="+boolString(tc.existingKind))
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.mutationError {
				t.Fatalf("error=%v output=%s", err, output)
			}
			text := string(output)
			if strings.Contains(text, "--add-label kind/chore") == tc.existingKind {
				t.Error("missing kind not filled or existing kind overwritten")
			}
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

func TestInternalInvalidAssessment(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/triage-ai-issue.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	start, end := strings.Index(workflow, "            # Validate AI JSON response"), strings.Index(workflow, "            # Parse AI response")
	if start < 0 || end <= start {
		t.Fatal("assessment validation block missing")
	}
	script := `set -euo pipefail
gh() { printf 'GH %s\n' "$*"; }
INTERNAL=true AI_JSON=invalid AI_TEXT=invalid OWNER=camunda REPO=repo
for ISSUE_NUMBER in 42 43; do
` + workflow[start:end] + "\nprintf 'UNEXPECTED_SUCCESS\\n'\ndone\nprintf 'CONTINUED\\n'\n"
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "GITHUB_STEP_SUMMARY="+filepath.Join(t.TempDir(), "summary"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	text := string(output)
	if strings.Count(text, "--add-label triage:manual") != 2 || strings.Contains(text, "issue comment") || strings.Contains(text, "UNEXPECTED_SUCCESS") || !strings.Contains(text, "CONTINUED") {
		t.Fatalf("invalid internal failure handling: %s", text)
	}
}

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

func TestUncertainAssessmentSuppression(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/triage-ai-issue.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	start := strings.Index(workflow, "            # Check confidence threshold")
	end := strings.Index(workflow[start:], "            # Prepare severity emoji") + start
	if start < 0 || end <= start {
		t.Fatal("assessment gates missing")
	}
	block := workflow[start:end]
	block = block[:strings.LastIndex(block, `            if [[ "$INTERNAL" != "true" ]]; then`)]
	for _, tc := range []struct {
		name, confidence, severity         string
		internal, wantManual, wantContinue bool
	}{
		{"internal low", "0.4", "high", true, true, true},
		{"internal unknown", "0.9", "unknown", true, true, true},
		{"internal accepted", "0.9", "high", true, false, false},
		{"external low", "0.4", "high", false, false, true},
		{"external unknown", "0.9", "unknown", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := "set -euo pipefail\ngh() { printf 'GH %s\\n' \"$*\"; }\nOWNER=camunda REPO=repo CONFIDENCE_THRESHOLD=0.7\nfor ISSUE_NUMBER in 42; do\n" + block + "\nprintf 'ACCEPTED\\n'\ndone\n"
			cmd := exec.Command("bash", "-c", script)
			cmd.Env = append(os.Environ(), "INTERNAL="+boolString(tc.internal), "CONFIDENCE="+tc.confidence, "SEVERITY="+tc.severity)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, output)
			}
			text := string(output)
			if strings.Contains(text, "--add-label triage:manual") != tc.wantManual || strings.Contains(text, "ACCEPTED") == tc.wantContinue || strings.Contains(text, "issue comment") {
				t.Fatalf("unexpected handling: %s", text)
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

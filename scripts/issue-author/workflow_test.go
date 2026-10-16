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
	start := strings.Index(workflow, "      - name: Route new internal issue to Backlog")
	if start < 0 {
		t.Fatal("initial route missing")
	}
	step := workflow[start : strings.Index(workflow[start:], "      - name: Move to Ready")+start]
	condition := strings.Split(strings.Split(step, "if: >-\n")[1], "        uses:")[0]
	if !strings.Contains(step, "content_id: ${{ fromJSON(steps.fetch_data.outputs.issue_data).id }}") || !strings.Contains(step, "value: 📋 Backlog") {
		t.Fatal("incorrect project action wiring")
	}
	for _, tc := range []struct {
		name, status                          string
		initial, internal, project, dry, want bool
	}{
		{"empty", "", true, true, true, false, true},
		{"inbox", "📥 Inbox", true, true, true, false, true},
		{"progress", "In Progress", true, true, true, false, false},
		{"backlog", "📋 Backlog", true, true, true, false, false},
		{"external", "📥 Inbox", true, false, true, false, false},
		{"ordinary run", "📥 Inbox", false, true, true, false, false},
		{"no project", "📥 Inbox", true, true, false, false, false},
		{"dry run", "📥 Inbox", true, true, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr := strings.NewReplacer("inputs.route_new_internal", "'"+boolString(tc.initial)+"' == 'true'", "env.DRY_RUN", "'"+boolString(tc.dry)+"'", "steps.author.outputs.internal", "'"+boolString(tc.internal)+"'", "steps.validate_project.outputs.in_target_project", "'"+boolString(tc.project)+"'", "steps.validate_project.outputs.current_status", "'"+tc.status+"'").Replace(condition)
			cmd := exec.Command("bash", "-c", "if [[ "+expr+" ]]; then printf yes; else printf no; fi")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, output)
			}
			if (string(output) == "yes") != tc.want {
				t.Fatalf("route=%s want=%t", output, tc.want)
			}
		})
	}
	if strings.Index(workflow, "      - name: Classify issue creator for routing") < strings.Index(workflow, "      - name: Update Urgency") {
		t.Fatal("classification blocks urgency updates")
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

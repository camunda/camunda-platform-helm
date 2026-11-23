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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const accepted = `{"work_type":"TASK","kind":"chore","severity":"critical","likelihood":"high","confidence":0.9,"rationale":"r"}`

type fake struct {
	issue, answer, labels, project string
	internal, moveFails            bool
	calls                          [][]string
	mu                             sync.Mutex
	bodies                         map[string][]byte
}

func (f *fake) gh(args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	call := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(call, "api /repos/"):
		return []byte(f.issue), nil
	case strings.HasPrefix(call, "issue view"):
		return []byte(f.labels), nil
	case strings.HasPrefix(call, "api graphql -f query=mutation(") && f.moveFails:
		return nil, errors.New("mutation failed")
	case strings.HasPrefix(call, "api graphql -f query=query("):
		return []byte(f.project), nil
	}
	return nil, nil
}

func (f *fake) log() string {
	var b strings.Builder
	for _, args := range f.calls {
		b.WriteString(strings.Join(args, " ") + "\n")
	}
	return b.String()
}

func (f *fake) body(path string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[path]
}

func (f *fake) triage(t *testing.T) *triage {
	t.Helper()
	f.bodies = map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		f.mu.Lock()
		f.bodies[r.URL.Path] = body
		f.mu.Unlock()
		switch {
		case r.URL.Path == "/glean":
			if r.Header.Get("Authorization") != "Bearer glean-token" || r.Header.Get("Content-Type") != "application/json" {
				t.Error("missing Glean request headers")
			}
			fmt.Fprint(w, f.answer)
		case strings.Contains(r.URL.Path, "/issues/"):
			fmt.Fprint(w, `{"user":{"login":"author","type":"User"}}`)
		case strings.Contains(r.URL.Path, "/distribution/") && f.internal:
			fmt.Fprint(w, `[{"login":"author"}]`)
		case strings.Contains(r.URL.Path, "/members"):
			fmt.Fprint(w, `[{"login":"someone"}]`)
		}
	}))
	t.Cleanup(server.Close)
	return &triage{
		gh:         f.gh,
		author:     client{http: server.Client(), base: server.URL, token: "test", repo: "camunda/camunda-platform-helm"},
		http:       server.Client(),
		gleanURL:   server.URL + "/glean",
		gleanToken: "glean-token",
		slackURL:   server.URL + "/slack",
		owner:      "camunda",
		repo:       "camunda-platform-helm",
		serverURL:  "https://github.com",
		summary:    filepath.Join(t.TempDir(), "summary"),
		project:    33,
		threshold:  0.7,
	}
}

func gleanAnswer(text string) string {
	data, _ := json.Marshal(map[string]any{"messages": []any{map[string]any{"author": "GLEAN_AI", "messageType": "CONTENT", "fragments": []any{map[string]string{"text": text}}}}})
	return string(data)
}

func issueJSON(state string, labels ...string) string {
	names := make([]map[string]string, len(labels))
	for i, label := range labels {
		names[i] = map[string]string{"name": label}
	}
	data, _ := json.Marshal(map[string]any{"title": "Chart fails", "body": "", "state": state, "user": map[string]string{"login": "author"}, "labels": names})
	return string(data)
}

func projectData(status string, item, option bool) string {
	items, options := []any{}, []any{}
	if item {
		items = append(items, map[string]any{"id": "item", "project": map[string]any{"number": 33}, "fieldValueByName": map[string]string{"name": status}})
	}
	if option {
		options = append(options, map[string]string{"id": "backlog", "name": backlog})
	}
	data, _ := json.Marshal(map[string]any{"data": map[string]any{
		"repository":   map[string]any{"issue": map[string]any{"projectItems": map[string]any{"nodes": items}}},
		"organization": map[string]any{"projectV2": map[string]any{"id": "project", "field": map[string]any{"id": "status", "options": options}}},
	}})
	return string(data)
}

func read(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func sameJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("%v: %s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestTriageMatchesBash(t *testing.T) {
	f := &fake{issue: read(t, "issue.json"), answer: read(t, "glean-response.json"), project: projectData(inbox, true, true)}
	if err := f.triage(t).process("42"); err != nil {
		t.Fatal(err)
	}
	sameJSON(t, f.body("/glean"), read(t, "glean-request.json"))
	sameJSON(t, f.body("/slack"), read(t, "slack.json"))
	var posted string
	for _, args := range f.calls {
		if args[0] == "issue" && args[1] == "comment" {
			posted = args[len(args)-1]
		}
	}
	if want := read(t, "comment.md"); posted != want {
		t.Errorf("comment\n%s\nwant\n%s", posted, want)
	}
	for _, want := range []string{
		"issue edit 42 --repo camunda/camunda-platform-helm --add-label severity/high --add-label likelihood/mid --add-label TRIAGE:COMPLETED\n",
		"--add-label kind/bug\n",
		"--add-label BLOCKER/INFO\n",
		"-f itemId=item -f fieldId=status -f optionId=backlog\n",
	} {
		if !strings.Contains(f.log(), want) {
			t.Errorf("missing %q in\n%s", want, f.log())
		}
	}
}

func TestGleanResponses(t *testing.T) {
	quoted, err := json.Marshal(accepted)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, response string
		wantTriage     bool
	}{
		{"content message without fragments", `{"messages":[{"author":"GLEAN_AI","messageType":"CONTENT"},{"author":"GLEAN_AI","messageType":"CONTENT","fragments":[{"citation":{}},{"text":` + string(quoted) + `}]}]}`, true},
		{"null fragments", `{"messages":[{"author":"GLEAN_AI","messageType":"CONTENT","fragments":null}]}`, false},
		{"no messages", `{"error":"timeout"}`, false},
		{"JSON string", `"busy"`, false},
		{"not JSON", `<html>bad gateway</html>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fake{issue: issueJSON("open"), answer: tc.response, project: projectData(inbox, true, true)}
			if err := f.triage(t).process("42"); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(f.log(), "--add-label TRIAGE:COMPLETED") != tc.wantTriage || !tc.wantTriage && len(f.calls) != 1 {
				t.Errorf("calls:\n%s", f.log())
			}
		})
	}
}

func TestAssessmentParsing(t *testing.T) {
	defaults := assessment{workType: "unknown", severity: "unknown", likelihood: "low", confidence: "0.0", rationale: "No rationale provided", missingInfo: "false"}
	withSeverity, withSimilar := defaults, defaults
	withSeverity.severity, withSeverity.confidence = "mid", "0.75"
	withSimilar.workType, withSimilar.similar, withSimilar.missingInfo = "TASK", "#1, 2, ", "true"
	for _, tc := range []struct {
		name, text string
		want       assessment
		valid      bool
	}{
		{"fenced block", "Result:\n```json\n{\"severity\":\"mid\",\"confidence\":0.75}\n```\nDone.", withSeverity, true},
		{"object in prose", `The answer is {"work_type":"TASK","kind":"","similar_issues":["#1",2,null],"missing_info":"true\n"} as requested`, withSimilar, true},
		{"false and null fields", `{"work_type":false,"severity":null,"similar_issues":"#1","missing_info":false}`, defaults, true},
		{"empty fenced block", "```json\n```\n{}", defaults, true},
		{"no object", "I cannot assess this issue.", assessment{}, false},
		{"JSON that is not an object", "true", assessment{}, false},
		{"null", "null", assessment{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, valid := parseAssessment(tc.text)
			if got != tc.want || valid != tc.valid {
				t.Fatalf("got %+v valid=%t", got, valid)
			}
		})
	}
}

func TestUncertainAssessments(t *testing.T) {
	for _, tc := range []struct {
		name, confidence, severity         string
		internal, wantManual, wantAccepted bool
	}{
		{"internal low", "0.4", "high", true, true, false},
		{"internal unknown", "0.9", "unknown", true, true, false},
		{"internal missing confidence", "null", "high", true, true, false},
		{"internal accepted", "0.9", "high", true, false, true},
		{"external low", "0.4", "high", false, false, false},
		{"external unknown", "0.9", "unknown", false, false, false},
		{"external at threshold", "0.7", "mid", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fake{issue: issueJSON("open"), answer: gleanAnswer(`{"severity":"` + tc.severity + `","confidence":` + tc.confidence + `}`), project: projectData(inbox, true, true), internal: tc.internal}
			tr := f.triage(t)
			if err := tr.process("42"); err != nil {
				t.Fatal(err)
			}
			text := f.log()
			if strings.Contains(text, "--add-label triage:manual") != tc.wantManual || strings.Contains(text, "api graphql") != tc.wantAccepted || strings.Contains(text, "issue comment") != (tc.wantAccepted && !tc.internal) {
				t.Fatalf("unexpected handling:\n%s", text)
			}
			if summary, err := os.ReadFile(tr.summary); err != nil || !strings.Contains(string(summary), "### Issue #42 — suggested priority\nSeverity: "+tc.severity) {
				t.Fatalf("summary %q: %v", summary, err)
			}
		})
	}
}

func TestInvalidAssessment(t *testing.T) {
	for name, internal := range map[string]bool{"internal": true, "external": false} {
		t.Run(name, func(t *testing.T) {
			f := &fake{issue: issueJSON("open"), answer: gleanAnswer("I cannot assess this issue."), internal: internal}
			tr := f.triage(t)
			for _, number := range []string{"42", "43"} {
				if err := tr.process(number); err != nil {
					t.Fatal(err)
				}
			}
			text := f.log()
			summary, _ := os.ReadFile(tr.summary)
			if strings.Count(text, "--add-label triage:manual") != 2 || strings.Count(text, "manual triage required") != map[bool]int{true: 0, false: 2}[internal] || strings.Contains(string(summary), "Issue #43: assessment failed; manual review needed.\n") != internal {
				t.Fatalf("invalid failure handling:\n%s\nsummary %q", text, summary)
			}
		})
	}
}

func TestBacklogRouting(t *testing.T) {
	for _, tc := range []struct {
		name, status                                        string
		internal, missingItem, missingOption, mutationError bool
		existingKind, slackFails                            bool
		wantMove, wantComplete, wantSlack                   bool
	}{
		{name: "internal Inbox", status: inbox, internal: true, wantMove: true, wantComplete: true},
		{name: "internal existing kind", status: inbox, internal: true, existingKind: true, wantMove: true, wantComplete: true},
		{name: "internal unset", internal: true, wantMove: true, wantComplete: true},
		{name: "internal Backlog", status: backlog, internal: true, wantComplete: true},
		{name: "internal in progress", status: "In Progress", internal: true, wantComplete: true},
		{name: "internal missing item", internal: true, missingItem: true},
		{name: "internal missing option", internal: true, missingOption: true},
		{name: "internal mutation failure", internal: true, mutationError: true, wantMove: true},
		{name: "external Inbox", status: inbox, wantMove: true, wantComplete: true, wantSlack: true},
		{name: "external missing item", missingItem: true, wantComplete: true, wantSlack: true},
		{name: "external Slack failure", status: inbox, slackFails: true, wantMove: true, wantComplete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			labels := "severity/low\nlikelihood/low\n"
			if tc.existingKind {
				labels += "kind/bug\n"
			}
			f := &fake{issue: issueJSON("open"), answer: gleanAnswer(accepted), labels: labels, project: projectData(tc.status, !tc.missingItem, !tc.missingOption), internal: tc.internal, moveFails: tc.mutationError}
			tr := f.triage(t)
			if tc.slackFails {
				tr.slackURL = ""
			}
			if err := tr.process("42"); (err != nil) != tc.mutationError {
				t.Fatalf("error=%v calls:\n%s", err, f.log())
			}
			text := f.log()
			if strings.Contains(text, "--add-label kind/chore") == tc.existingKind {
				t.Error("missing kind not filled or existing kind overwritten")
			}
			for marker, want := range map[string]bool{"query=mutation(": tc.wantMove, "--add-label TRIAGE:COMPLETED": tc.wantComplete} {
				if strings.Contains(text, marker) != want {
					t.Errorf("marker %q want=%t calls:\n%s", marker, want, text)
				}
			}
			if (f.body("/slack") != nil) != tc.wantSlack {
				t.Errorf("Slack notification want=%t", tc.wantSlack)
			}
			if tc.internal && strings.Contains(text, "--add-label severity/critical") {
				t.Error("existing priority overwritten")
			}
			if tc.internal && tc.wantMove && tc.wantComplete && strings.Index(text, "query=mutation(") > strings.Index(text, "--add-label TRIAGE:COMPLETED") {
				t.Error("marked complete before routing")
			}
		})
	}
}

func TestFeatureRequestRouting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		labels []string
		wantPM bool
	}{
		{"external feature request", nil, true},
		{"kind/internal label", []string{"kind/internal"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fake{issue: issueJSON("open", tc.labels...), answer: gleanAnswer(`{"kind":"feature-request","severity":"mid","likelihood":"low","confidence":0.8}`), project: projectData(inbox, true, true)}
			if err := f.triage(t).process("42"); err != nil {
				t.Fatal(err)
			}
			text := f.log()
			routed := strings.Contains(text, "--add-label kind/feature-request --add-label triage:manual\n") && strings.Contains(text, "--add-assignee mlowe-camunda\n")
			if routed != tc.wantPM || strings.Contains(text, "TRIAGE:COMPLETED") == tc.wantPM || (f.body("/slack") != nil) == tc.wantPM {
				t.Fatalf("unexpected routing:\n%s", text)
			}
		})
	}
}

func TestSkippedIssues(t *testing.T) {
	for name, data := range map[string]string{
		"closed":           issueJSON("closed"),
		"triage completed": issueJSON("open", "area/ci", "triage:completed"),
		"triage manual":    issueJSON("open", "triage:manual"),
	} {
		t.Run(name, func(t *testing.T) {
			f := &fake{issue: data}
			if err := f.triage(t).process("42"); err != nil || len(f.calls) != 1 || len(f.bodies) != 0 {
				t.Fatalf("error=%v calls:\n%s", err, f.log())
			}
		})
	}
}

func TestClassificationOnly(t *testing.T) {
	f := &fake{issue: issueJSON("closed"), internal: true}
	tr := f.triage(t)
	tr.classificationOnly = true
	if err := tr.process("42"); err != nil {
		t.Fatal(err)
	}
	summary, err := os.ReadFile(tr.summary)
	if err != nil || string(summary) != "App-token classification: issue #42 internal=true\n" || len(f.calls) != 1 || f.body("/glean") != nil {
		t.Fatalf("summary %q error=%v calls:\n%s", summary, err, f.log())
	}
}

func TestRunTriageEnvironment(t *testing.T) {
	for name, value := range map[string]string{"OWNER": "camunda", "REPO": "repo", "GH_TOKEN": "fixture", "PROJECT_NUMBER": "33", "CONFIDENCE_THRESHOLD": "0.7", "ISSUE_NUMBERS": ""} {
		t.Setenv(name, value)
	}
	if err := runTriage(); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"PROJECT_NUMBER": "x", "CONFIDENCE_THRESHOLD": "", "REPO": "a/b", "GH_TOKEN": ""} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, value)
			if runTriage() == nil {
				t.Fatalf("%s=%q accepted", name, value)
			}
		})
	}
}

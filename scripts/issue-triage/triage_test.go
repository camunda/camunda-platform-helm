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
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixture struct {
	mu                                           sync.Mutex
	issue                                        issue
	status                                       string
	inProject                                    bool
	comments, assignments, mutations, slackCalls int
	slackCode, memberCode                        int
	receiptFailure                               bool
	slackTexts                                   []string
	server                                       *httptest.Server
}

func newFixture(t *testing.T) (*fixture, *githubClient) {
	t.Helper()
	f := &fixture{
		issue:  issue{Number: 42, Title: "Test issue <!channel>", Body: "Investigate", State: "open", HTMLURL: "https://github.com/camunda/repo/issues/42", User: author{Login: "outside", Type: "User"}},
		status: "📥 Inbox", inProject: true, slackCode: 200, memberCode: 200,
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f, &githubClient{http: f.server.Client(), base: f.server.URL, token: "test-token", repo: "camunda/repo"}
}

func (f *fixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	write := func(v any) {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			panic(err)
		}
	}
	decode := func(v any) {
		if err := json.NewDecoder(r.Body).Decode(v); err != nil {
			panic(err)
		}
	}
	path := r.URL.Path
	switch {
	case path == "/slack":
		f.slackCalls++
		var payload struct {
			Blocks []struct{ Text struct{ Type, Text string } }
		}
		decode(&payload)
		if payload.Blocks[0].Text.Type != "plain_text" {
			panic("untrusted title must use plain_text")
		}
		f.slackTexts = append(f.slackTexts, payload.Blocks[0].Text.Text)
		w.WriteHeader(f.slackCode)
		if f.slackCode == 200 {
			fmt.Fprint(w, "ok")
		}
	case strings.Contains(path, "/teams/"):
		w.WriteHeader(f.memberCode)
		if f.memberCode != 200 {
			return
		}
		if strings.Contains(path, "/distribution/") {
			write([]author{{Login: "distro"}})
		} else {
			write([]author{{Login: "infraex"}})
		}
	case path == "/repos/camunda/repo/issues/42" && r.Method == "GET":
		write(f.issue)
	case path == "/repos/camunda/repo/issues" && r.Method == "GET":
		write([]issue{f.issue})
	case strings.HasPrefix(path, "/repos/camunda/repo/labels/") && r.Method == "GET":
		write(map[string]string{"name": "exists"})
	case path == "/repos/camunda/repo/issues/42/labels" && r.Method == "POST":
		var body struct{ Labels []string }
		decode(&body)
		if f.receiptFailure && slices.Contains(body.Labels, slackNotified) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, name := range body.Labels {
			if !f.issue.has(name) {
				f.issue.Labels = append(f.issue.Labels, label{name})
			}
		}
		write(f.issue.Labels)
	case strings.HasPrefix(path, "/repos/camunda/repo/issues/42/labels/") && r.Method == "DELETE":
		name := strings.TrimPrefix(path, "/repos/camunda/repo/issues/42/labels/")
		f.issue.Labels = slices.DeleteFunc(f.issue.Labels, func(l label) bool { return l.Name == name })
		write(f.issue.Labels)
	case strings.HasSuffix(path, "/comments"):
		f.comments++
		write(map[string]int{"id": 1})
	case strings.HasSuffix(path, "/assignees"):
		f.assignments++
		write(map[string]int{"id": 1})
	case path == "/graphql":
		var body struct {
			Query     string
			Variables map[string]any
		}
		decode(&body)
		switch {
		case strings.Contains(body.Query, "mutation"):
			f.mutations++
			f.status = "📋 Backlog"
			write(map[string]any{"data": map[string]any{"updateProjectV2ItemFieldValue": map[string]any{"projectV2Item": map[string]string{"id": "item"}}}})
		case strings.Contains(body.Query, "projectItems"):
			nodes := []any{}
			if f.inProject {
				nodes = append(nodes, map[string]any{"id": "item", "project": map[string]any{"id": "project", "number": 33}, "fieldValues": map[string]any{"nodes": []any{map[string]any{"name": f.status, "field": map[string]string{"name": "Status"}}}}})
			}
			write(map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{"projectItems": map[string]any{"nodes": nodes, "pageInfo": map[string]bool{"hasNextPage": false}}}}}})
		default:
			write(map[string]any{"data": map[string]any{"node": map[string]any{"field": map[string]any{"id": "status", "options": []any{map[string]string{"id": "backlog", "name": "📋 Backlog"}}}}}})
		}
	default:
		panic("unexpected request: " + r.Method + " " + path)
	}
}

func goodAssessment() assessment {
	return assessment{WorkType: "TASK", Kind: "chore", Severity: "critical", Likelihood: "high", Confidence: .9, Rationale: "Triage guideline assessment"}
}

func TestInternalPrioritization(t *testing.T) {
	for _, login := range []string{"distro", "infraex", "DiStRo"} {
		for _, status := range []string{"", "📥 Inbox", "📋 Backlog", "⏭️ Ready", "In Progress", "✅ Done"} {
			t.Run(login+"/"+status, func(t *testing.T) {
				f, c := newFixture(t)
				f.issue.User.Login, f.status = login, status
				f.issue.Labels = []label{{"severity/low"}, {"kind/bug"}, {"impact/high"}, {"when/now"}, {"urgency-locked"}}
				var summary strings.Builder
				calls := 0
				tr := &triager{github: c, project: 33, summary: &summary, assess: func(string, issue) (assessment, error) { calls++; return goodAssessment(), nil }}
				if err := tr.run(42); err != nil {
					t.Fatal(err)
				}
				if !f.issue.has("severity/low") || f.issue.has("severity/critical") || !f.issue.has("likelihood/high") || !f.issue.has("kind/bug") || f.issue.has("kind/chore") || !f.issue.has("TRIAGE:COMPLETED") {
					t.Fatalf("unexpected labels: %+v", f.issue.Labels)
				}
				if f.comments != 0 || f.assignments != 0 || f.slackCalls != 0 {
					t.Fatal("internal assessment created discussion noise")
				}
				wantStatus := status
				if status == "" || status == "📥 Inbox" {
					wantStatus = "📋 Backlog"
				}
				if f.status != wantStatus {
					t.Fatalf("status %q, want %q", f.status, wantStatus)
				}
				if !strings.Contains(summary.String(), "Triage guideline assessment") {
					t.Fatal("missing rationale")
				}
				if err := tr.run(42); err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatal("reassessed completed issue")
				}
			})
		}
	}
}

func TestExternalReporting(t *testing.T) {
	for _, test := range []struct {
		name                  string
		a                     assessment
		err                   error
		manual                bool
		wantText              string
		comments, assignments int
	}{
		{"success", goodAssessment(), nil, false, "AI assessment complete", 1, 0},
		{"low confidence", func() assessment { a := goodAssessment(); a.Confidence = .4; return a }(), nil, false, "Manual assessment needed", 0, 0},
		{"unknown severity", func() assessment { a := goodAssessment(); a.Severity = "unknown"; return a }(), nil, false, "Manual assessment needed", 0, 0},
		{"failure", assessment{}, errors.New("assessment unavailable"), false, "Manual attention required", 1, 0},
		{"PM", func() assessment { a := goodAssessment(); a.Kind = "feature-request"; return a }(), nil, false, "routed to PM", 1, 1},
		{"manual", assessment{}, nil, true, "Manual triage required", 0, 0},
	} {
		for _, userType := range []string{"User", "Bot"} {
			t.Run(test.name+"/"+userType, func(t *testing.T) {
				f, c := newFixture(t)
				f.issue.User.Type = userType
				if test.manual {
					f.issue.Labels = []label{{"triage:manual"}}
				}
				tr := &triager{github: c, project: 33, summary: io.Discard, slack: &slackClient{http: f.server.Client(), webhook: f.server.URL + "/slack"}, assess: func(string, issue) (assessment, error) { return test.a, test.err }}
				err := tr.run(42)
				if (err != nil) != (test.err != nil) {
					t.Fatalf("error = %v", err)
				}
				if f.slackCalls != 1 || !f.issue.has(slackNotified) || f.issue.has(slackPending) || f.issue.has(slackRetry) {
					t.Fatalf("delivery state calls=%d labels=%v", f.slackCalls, f.issue.Labels)
				}
				if !strings.Contains(f.slackTexts[0], test.wantText) {
					t.Fatalf("Slack text: %s", f.slackTexts[0])
				}
				if f.comments != test.comments || f.assignments != test.assignments {
					t.Fatalf("comments=%d assignments=%d", f.comments, f.assignments)
				}
				if err := tr.run(42); err != nil {
					t.Fatal(err)
				}
				if f.slackCalls != 1 {
					t.Fatal("duplicate intake notification")
				}
			})
		}
	}
}

func TestInternalFailureAndFeatureRequest(t *testing.T) {
	for _, kind := range []string{"failure", "low confidence", "feature-request"} {
		t.Run(kind, func(t *testing.T) {
			f, c := newFixture(t)
			f.issue.User.Login = "infraex"
			tr := &triager{github: c, project: 33, summary: io.Discard, assess: func(string, issue) (assessment, error) {
				a := goodAssessment()
				switch kind {
				case "failure":
					return a, errors.New("unavailable")
				case "low confidence":
					a.Confidence = .2
				case "feature-request":
					a.Kind = kind
				}
				return a, nil
			}}
			err := tr.run(42)
			if (err != nil) != (kind == "failure") {
				t.Fatal(err)
			}
			if f.comments != 0 || f.assignments != 0 || f.slackCalls != 0 || f.issue.has("triage:manual") {
				t.Fatal("internal assessment created discussion side effects")
			}
			if kind != "feature-request" && len(f.issue.Labels) != 0 {
				t.Fatal("failed assessment applied labels")
			}
		})
	}
}

func TestMembershipFailureHasNoSideEffects(t *testing.T) {
	for _, code := range []int{401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			f, c := newFixture(t)
			f.memberCode = code
			tr := &triager{github: c, summary: io.Discard, assess: func(string, issue) (assessment, error) {
				t.Fatal("assessment must not start")
				return assessment{}, nil
			}}
			if err := tr.run(42); err == nil {
				t.Fatal("expected membership failure")
			}
			if f.comments+f.assignments+f.mutations+f.slackCalls != 0 || len(f.issue.Labels) != 0 {
				t.Fatal("side effects on classification failure")
			}
		})
	}
}

func TestDeliveryRetries(t *testing.T) {
	f, c := newFixture(t)
	s := &slackClient{http: f.server.Client(), webhook: f.server.URL + "/slack"}
	f.slackCode = 429
	if err := s.notify(c, f.issue, "Pending assessment"); err == nil {
		t.Fatal("expected rejection")
	}
	if f.issue.has(slackPending) || f.issue.has(slackNotified) || !f.issue.has(slackRetry) {
		t.Fatalf("rejection state: %v", f.issue.Labels)
	}
	f.slackCode = 200
	if err := s.notify(c, f.issue, "Retry"); err != nil {
		t.Fatal(err)
	}
	if err := s.notify(c, f.issue, "Duplicate"); err != nil {
		t.Fatal(err)
	}
	if f.slackCalls != 2 {
		t.Fatalf("calls=%d", f.slackCalls)
	}
}

func TestAmbiguousDeliveryNeedsReconciliation(t *testing.T) {
	f, c := newFixture(t)
	f.slackCode = 500
	s := &slackClient{http: f.server.Client(), webhook: f.server.URL + "/slack"}
	if err := s.notify(c, f.issue, "Intake"); err == nil {
		t.Fatal("expected ambiguous delivery error")
	}
	if !f.issue.has(slackPending) {
		t.Fatal("missing pending marker")
	}
	f.slackCode = 200
	if err := s.notify(c, f.issue, "Retry"); err == nil {
		t.Fatal("must require reconciliation")
	}
	if f.slackCalls != 1 {
		t.Fatal("duplicate attempt after ambiguous delivery")
	}
}

func TestSlackTimeoutDoesNotLeakWebhook(t *testing.T) {
	f, c := newFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(30 * time.Millisecond); fmt.Fprint(w, "ok") }))
	defer server.Close()
	s := &slackClient{http: &http.Client{Timeout: time.Millisecond}, webhook: server.URL + "/secret-webhook"}
	err := s.notify(c, f.issue, "Intake")
	if err == nil || strings.Contains(err.Error(), "secret-webhook") {
		t.Fatalf("unsafe error: %v", err)
	}
	if !f.issue.has(slackPending) {
		t.Fatal("timeout lost pending marker")
	}
}

func TestBulkSelection(t *testing.T) {
	for _, test := range []struct {
		name, login, status string
		labels              []label
		want                bool
	}{
		{"internal backlog", "distro", "📋 Backlog", nil, true},
		{"internal done", "infraex", "📋 Backlog", []label{{"TRIAGE:COMPLETED"}}, false},
		{"external inbox", "other-camunda-team", "📥 Inbox", nil, true},
		{"manual inbox", "outside", "📥 Inbox", []label{{"triage:manual"}}, true},
		{"retry outside inbox", "outside", "⏭️ Ready", []label{{"TRIAGE:COMPLETED"}, {slackRetry}}, true},
		{"old assessed", "outside", "📋 Backlog", []label{{"TRIAGE:COMPLETED"}}, false},
		{"already notified", "outside", "⏭️ Ready", []label{{"TRIAGE:COMPLETED"}, {slackNotified}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, c := newFixture(t)
			f.issue.User.Login, f.status, f.issue.Labels = test.login, test.status, test.labels
			got, err := c.candidates(33)
			if err != nil {
				t.Fatal(err)
			}
			if (len(got) == 1) != test.want {
				t.Fatalf("candidates=%v", got)
			}
		})
	}
}

func TestBacklogAfterProjectInsertion(t *testing.T) {
	f, c := newFixture(t)
	f.issue.User.Login, f.inProject = "distro", false
	tr := &triager{github: c, project: 33, summary: io.Discard, assess: func(string, issue) (assessment, error) { return goodAssessment(), nil }}
	if err := tr.run(42); err != nil {
		t.Fatal(err)
	}
	if f.mutations != 0 || !f.issue.has("likelihood/high") {
		t.Fatal("assessment should succeed before project insertion")
	}
	f.inProject = true
	if err := c.backlog(42, 33, true); err != nil {
		t.Fatal(err)
	}
	if f.status != "📋 Backlog" {
		t.Fatal("missing Backlog routing after insertion")
	}
}

func TestClosedIssueHasNoSideEffects(t *testing.T) {
	f, c := newFixture(t)
	f.issue.State = "closed"
	tr := &triager{github: c, summary: io.Discard, assess: func(string, issue) (assessment, error) {
		t.Fatal("closed issue assessed")
		return assessment{}, nil
	}}
	if err := tr.run(42); err != nil {
		t.Fatal(err)
	}
	if f.comments+f.assignments+f.mutations+f.slackCalls != 0 || len(f.issue.Labels) != 0 {
		t.Fatal("closed issue mutated")
	}
}

func TestRefreshPreservesConcurrentPriorityEdits(t *testing.T) {
	f, c := newFixture(t)
	f.issue.User.Login = "distro"
	tr := &triager{github: c, project: 33, summary: io.Discard, assess: func(string, issue) (assessment, error) {
		f.mu.Lock()
		f.issue.Labels = []label{{"severity/mid"}, {"likelihood/low"}}
		f.status = "In Progress"
		f.mu.Unlock()
		return goodAssessment(), nil
	}}
	if err := tr.run(42); err != nil {
		t.Fatal(err)
	}
	if f.issue.has("severity/critical") || f.issue.has("likelihood/high") || f.status != "In Progress" {
		t.Fatal("concurrent human edits were overwritten")
	}
}

func TestDeliveredMessageWithFailedReceiptIsNotResent(t *testing.T) {
	f, c := newFixture(t)
	f.receiptFailure = true
	s := &slackClient{http: f.server.Client(), webhook: f.server.URL + "/slack"}
	if err := s.notify(c, f.issue, "Intake"); err == nil {
		t.Fatal("expected receipt failure")
	}
	if !f.issue.has(slackPending) || f.issue.has(slackNotified) {
		t.Fatal("lost ambiguous delivery state")
	}
	f.receiptFailure = false
	if err := s.notify(c, f.issue, "Retry"); err == nil {
		t.Fatal("must reconcile receipt before retry")
	}
	if f.slackCalls != 1 {
		t.Fatal("delivered message was sent twice")
	}
}

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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMembershipPaginationAndCache(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		users := []author{{Login: "infraex"}}
		if strings.Contains(r.URL.Path, "/distribution/") {
			if r.URL.Query().Get("page") == "1" {
				users = make([]author, 100)
				for n := range users {
					users[n].Login = fmt.Sprint("member", n)
				}
			} else {
				users = []author{{Login: "last-page"}}
			}
		}
		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Error(err)
		}
	}))
	defer s.Close()
	c := &githubClient{http: s.Client(), base: s.URL, repo: "camunda/repo"}
	for _, login := range []string{"last-page", "infraex", "MEMBER0"} {
		internal, err := c.internal(issue{User: author{Login: login, Type: "User"}})
		if err != nil || !internal {
			t.Fatalf("%s internal=%t err=%v", login, internal, err)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("expected paginated requests once, got %d", calls.Load())
	}
	internal, err := c.internal(issue{User: author{Login: "infraex", Type: "Bot"}})
	if err != nil || internal {
		t.Fatal("bots must remain reportable")
	}
}

func TestEmptyTeamIsNotExternal(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "[]") }))
	defer s.Close()
	c := &githubClient{http: s.Client(), base: s.URL, repo: "camunda/repo"}
	if _, err := c.internal(issue{User: author{Login: "someone"}}); err == nil {
		t.Fatal("empty team must not silently classify everybody as external")
	}
	if c.members != nil {
		t.Fatal("failed lookup must not populate cache")
	}
}

func TestParseAssessment(t *testing.T) {
	body, err := json.Marshal(goodAssessment())
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{string(body), "```json\n" + string(body) + "\n```", "Assessment:\n" + string(body)} {
		if _, err := parseAssessment(text); err != nil {
			t.Fatal(err)
		}
	}
	for _, text := range []string{"", "{}", "{invalid}", strings.Replace(string(body), "critical", "invented", 1), strings.Replace(string(body), "0.9", "2", 1)} {
		if _, err := parseAssessment(text); err == nil {
			t.Fatalf("accepted invalid response: %s", text)
		}
	}
}

func TestGleanResponse(t *testing.T) {
	body, err := json.Marshal(goodAssessment())
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer glean-token" {
			t.Error("missing token")
		}
		var request struct {
			Messages []struct{ Fragments []struct{ Text string } }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		prompt := request.Messages[0].Fragments[0].Text
		if !strings.Contains(prompt, "Issue #42") || !strings.Contains(prompt, "Distro Bug Triage") {
			t.Error("missing issue/guideline context")
		}
		response := map[string]any{"messages": []any{map[string]any{"author": "GLEAN_AI", "messageType": "CONTENT", "fragments": []any{map[string]string{"text": string(body[:20])}, map[string]string{"text": string(body[20:])}}}}}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer s.Close()
	g := &gleanClient{http: s.Client(), endpoint: s.URL, token: "glean-token"}
	a, err := g.assess("camunda/repo", issue{Number: 42})
	if err != nil || a.Severity != "critical" {
		t.Fatalf("assessment=%+v err=%v", a, err)
	}
}

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
	"testing"
)

func TestCreatorClassification(t *testing.T) {
	for _, tc := range []struct {
		name, login, kind    string
		code                 int
		empty, want, wantErr bool
	}{
		{"distribution", "DISTRO", "User", 200, false, true, false},
		{"infraex", "infraex", "User", 200, false, true, false},
		{"other Camunda team", "other-team", "User", 200, false, false, false},
		{"outside", "external", "User", 200, false, false, false},
		{"bot", "distro", "Bot", 403, false, false, false},
		{"forbidden", "external", "User", 403, false, false, true},
		{"missing team", "external", "User", 404, false, false, true},
		{"rate limited", "external", "User", 429, false, false, true},
		{"server error", "external", "User", 500, false, false, true},
		{"empty team", "external", "User", 200, true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITHUB_ACTOR", "unrelated-triggering-actor")
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Error("classification must be read-only")
				}
				if r.Header.Get("Authorization") != "Bearer test" {
					t.Error("missing authorization")
				}
				if strings.Contains(r.URL.Path, "/issues/") {
					fmt.Fprintf(w, `{"user":{"login":%q,"type":%q}}`, tc.login, tc.kind)
					return
				}
				if tc.kind == "Bot" {
					t.Error("bot classification must not require membership")
				}
				w.WriteHeader(tc.code)
				if tc.code != 200 {
					return
				}
				if tc.empty {
					fmt.Fprint(w, `[]`)
					return
				}
				if strings.Contains(r.URL.Path, "/distribution/") {
					fmt.Fprint(w, `[{"login":"distro"}]`)
				} else {
					fmt.Fprint(w, `[{"login":"infraex"}]`)
				}
			}))
			defer s.Close()
			c := client{http: s.Client(), base: s.URL, token: "test", repo: "camunda/repo"}
			got, err := c.internal(42)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("internal=%t error=%v", got, err)
			}
		})
	}
}

func TestTeamPagination(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/issues/") {
			fmt.Fprint(w, `{"user":{"login":"last-page","type":"User"}}`)
			return
		}
		if strings.Contains(r.URL.Path, "/distribution/") && r.URL.Query().Get("page") == "1" {
			members := make([]map[string]string, 100)
			for n := range members {
				members[n] = map[string]string{"login": fmt.Sprint("member", n)}
			}
			if err := json.NewEncoder(w).Encode(members); err != nil {
				t.Error(err)
			}
			return
		}
		if strings.Contains(r.URL.Path, "/infrastructure-experience/") {
			fmt.Fprint(w, `[{"login":"infraex"}]`)
			return
		}
		fmt.Fprint(w, `[{"login":"last-page"}]`)
	}))
	defer s.Close()
	c := client{http: s.Client(), base: s.URL, repo: "camunda/repo"}
	got, err := c.internal(42)
	if err != nil || !got {
		t.Fatalf("internal=%t error=%v", got, err)
	}
}

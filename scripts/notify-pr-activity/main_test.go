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
	"testing"
	"time"
)

func setEnv(t *testing.T, kvs map[string]string) {
	t.Helper()
	for k, v := range kvs {
		t.Setenv(k, v)
	}
}

func baseEnv(action string) map[string]string {
	return map[string]string{
		"GH_ACTION": action,
		"PR_REPO":   "camunda-platform-helm",
		"PR_URL":    "https://github.com/camunda/camunda-platform-helm/pull/1",
		"PR_NUMBER": "1",
		"PR_TITLE":  "feat: something",
	}
}

// TestBuildMessage_Opened checks that opened PRs produce a non-empty message.
func TestBuildMessage_Opened(t *testing.T) {
	setEnv(t, baseEnv("opened"))
	t.Setenv("PR_REVIEWERS_JSON", "[]")
	msg := buildMessage()
	if msg == "" {
		t.Fatal("expected non-empty message for opened PR")
	}
}

// TestBuildMessage_ClosedMerged checks that merged PRs produce a non-empty message.
func TestBuildMessage_ClosedMerged(t *testing.T) {
	env := baseEnv("closed")
	env["PR_MERGED"] = "true"
	env["PR_CREATED_AT"] = "2025-01-01T10:00:00Z"
	env["PR_MERGED_AT"] = "2025-01-01T14:00:00Z"
	setEnv(t, env)
	msg := buildMessage()
	if msg == "" {
		t.Fatal("expected non-empty message for merged PR")
	}
}

// TestBuildMessage_ClosedUnmerged_Empty checks that closed-without-merge PRs produce no message.
func TestBuildMessage_ClosedUnmerged_Empty(t *testing.T) {
	env := baseEnv("closed")
	env["PR_MERGED"] = "false"
	setEnv(t, env)
	msg := buildMessage()
	if msg != "" {
		t.Fatalf("expected empty message for unmerged closed PR, got: %q", msg)
	}
}

// TestHasLabel_Present checks that hasLabel returns true when label is present.
func TestHasLabel_Present(t *testing.T) {
	t.Setenv("PR_LABELS_JSON", `[{"name":"upgrade:major"},{"name":"dependencies"}]`)
	if !hasLabel("upgrade:major") {
		t.Fatal("expected hasLabel to return true for upgrade:major")
	}
}

// TestHasLabel_Absent checks that hasLabel returns false when label is absent.
func TestHasLabel_Absent(t *testing.T) {
	t.Setenv("PR_LABELS_JSON", `[{"name":"dependencies"}]`)
	if hasLabel("upgrade:major") {
		t.Fatal("expected hasLabel to return false when label absent")
	}
}

// TestHasLabel_Empty checks that hasLabel returns false for an empty label list.
func TestHasLabel_Empty(t *testing.T) {
	t.Setenv("PR_LABELS_JSON", `[]`)
	if hasLabel("upgrade:major") {
		t.Fatal("expected hasLabel to return false for empty labels")
	}
}

// TestFormatDuration verifies the duration formatting helper.
func TestFormatDuration(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		addHours int
		want     string
	}{
		{0, "< 1h"},
		{3, "3h"},
		{25, "1d 1h"},
		{48, "2d"},
	}
	for _, c := range cases {
		got := formatDuration(base, base.Add(time.Duration(c.addHours)*time.Hour))
		if got != c.want {
			t.Errorf("addHours=%d: got %q, want %q", c.addHours, got, c.want)
		}
	}
}

// TestParseReviewers checks reviewer resolution with and without a Slack user map.
func TestParseReviewers(t *testing.T) {
	userMap := map[string]string{"alice": "U123"}
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"mapped and fallback", `[{"login":"alice"},{"login":"bob"}]`, "<@U123>, @bob"},
		{"all fallback", `[{"login":"bob"}]`, "@bob"},
		{"empty", `[]`, ""},
	}
	for _, c := range cases {
		if got := parseReviewers(c.raw, userMap); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestSlackMention checks mapped resolution and the @login fallback.
func TestSlackMention(t *testing.T) {
	userMap := map[string]string{"alice": "U123", "empty": ""}
	cases := []struct {
		login string
		want  string
	}{
		{"alice", "<@U123>"},
		{"bob", "@bob"},
		{"empty", "@empty"},
	}
	for _, c := range cases {
		if got := slackMention(c.login, userMap); got != c.want {
			t.Errorf("login=%s: got %q, want %q", c.login, got, c.want)
		}
	}
}

// TestLoadUserMap_Missing returns an empty map for an absent file.
func TestLoadUserMap_Missing(t *testing.T) {
	if m := loadUserMap("does-not-exist.json"); len(m) != 0 {
		t.Errorf("expected empty map for missing file, got %v", m)
	}
}

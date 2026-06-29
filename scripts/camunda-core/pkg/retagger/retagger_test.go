// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retagger

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeChart creates a minimal Chart.yaml under root/charts/<dir>/.
func writeChart(t *testing.T, root, dir, version, appVersion string) {
	t.Helper()
	chartDir := filepath.Join(root, "charts", dir)
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", chartDir, err)
	}
	content := fmt.Sprintf("version: %q\nappVersion: %q\n", version, appVersion)
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write Chart.yaml: %v", err)
	}
}

func TestListCharts_SkipsPreRelease(t *testing.T) {
	root := t.TempDir()
	writeChart(t, root, "camunda-platform-8.7", "12.12.1", "8.7.x")
	writeChart(t, root, "camunda-platform-8.8", "13.0.0-alpha1", "8.8.x")
	writeChart(t, root, "camunda-platform-8.10", "13.5.2", "8.10.x")

	charts, err := ListCharts(root)
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	if len(charts) != 2 {
		t.Fatalf("want 2 charts (non-pre-release), got %d: %v", len(charts), charts)
	}
	tags := make(map[string]bool)
	for _, c := range charts {
		tags[c.TagName] = true
	}
	if !tags["camunda-platform-8.7-12.12.1"] {
		t.Error("missing camunda-platform-8.7-12.12.1")
	}
	if !tags["camunda-platform-8.10-13.5.2"] {
		t.Error("missing camunda-platform-8.10-13.5.2")
	}
}

func TestListCharts_StripsDotX(t *testing.T) {
	root := t.TempDir()
	writeChart(t, root, "camunda-platform-8.10", "13.5.2", "8.10.x")

	charts, err := ListCharts(root)
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	if len(charts) != 1 {
		t.Fatalf("want 1 chart, got %d", len(charts))
	}
	// appVersion "8.10.x" must be stripped to "8.10" in the tag name.
	want := "camunda-platform-8.10-13.5.2"
	if charts[0].TagName != want {
		t.Errorf("TagName = %q, want %q", charts[0].TagName, want)
	}
}

func TestListCharts_EmptyRoot(t *testing.T) {
	root := t.TempDir()
	charts, err := ListCharts(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charts) != 0 {
		t.Errorf("want 0 charts, got %d", len(charts))
	}
}

// mockClient implements GitHubClient with configurable responses.
type mockClient struct {
	shaFn  func(repo, tag string) (string, error)
	moveFn func(repo, tag, sha string) error
	moved  []string // tags that were moved
}

func (m *mockClient) CommitSHA(repo, tag string) (string, error) {
	return m.shaFn(repo, tag)
}

func (m *mockClient) MoveTag(repo, tag, sha string) error {
	m.moved = append(m.moved, tag)
	if m.moveFn != nil {
		return m.moveFn(repo, tag, sha)
	}
	return nil
}

func mustListCharts(t *testing.T) (string, []Chart) {
	t.Helper()
	root := t.TempDir()
	writeChart(t, root, "camunda-platform-8.7", "12.12.1", "8.7.x")
	charts, err := ListCharts(root)
	if err != nil {
		t.Fatalf("ListCharts: %v", err)
	}
	return root, charts
}

func TestRun_AlreadyCorrect(t *testing.T) {
	root, _ := mustListCharts(t)
	const target = "abc1234abc1234abc1234abc1234abc1234abc1234"
	mc := &mockClient{shaFn: func(_, _ string) (string, error) { return target, nil }}

	results, err := Run(root, "owner/repo", target, mc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Moved {
		t.Error("expected Moved=false when tag already points to target SHA")
	}
	if results[0].Reason != "already correct" {
		t.Errorf("Reason = %q, want %q", results[0].Reason, "already correct")
	}
	if len(mc.moved) != 0 {
		t.Errorf("MoveTag was called unexpectedly: %v", mc.moved)
	}
}

func TestRun_TagMissing(t *testing.T) {
	root, _ := mustListCharts(t)
	mc := &mockClient{shaFn: func(_, _ string) (string, error) { return "", nil }}

	results, err := Run(root, "owner/repo", "newsha", mc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Moved {
		t.Error("expected Moved=false when tag does not exist")
	}
	if !strings.Contains(results[0].Reason, "does not exist") {
		t.Errorf("Reason = %q, expected to contain 'does not exist'", results[0].Reason)
	}
	if len(mc.moved) != 0 {
		t.Errorf("MoveTag was called unexpectedly: %v", mc.moved)
	}
}

func TestRun_MovesStaleTag(t *testing.T) {
	root, _ := mustListCharts(t)
	const (
		staleSHA  = "aaaaaaaabbbbbbbbccccccccddddddddeeeeeeee"
		targetSHA = "1111111122222222333333334444444455555555"
	)
	mc := &mockClient{shaFn: func(_, _ string) (string, error) { return staleSHA, nil }}

	results, err := Run(root, "owner/repo", targetSHA, mc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if !results[0].Moved {
		t.Error("expected Moved=true when tag points to stale SHA")
	}
	if len(mc.moved) != 1 || mc.moved[0] != results[0].TagName {
		t.Errorf("MoveTag called with wrong tags: %v", mc.moved)
	}
	// Reason should contain short SHAs.
	if !strings.Contains(results[0].Reason, "aaaaaaa") {
		t.Errorf("Reason should contain short stale SHA: %q", results[0].Reason)
	}
	if !strings.Contains(results[0].Reason, "1111111") {
		t.Errorf("Reason should contain short target SHA: %q", results[0].Reason)
	}
}

func TestRun_MoveError(t *testing.T) {
	root, _ := mustListCharts(t)
	mc := &mockClient{
		shaFn:  func(_, _ string) (string, error) { return "stalesha", nil },
		moveFn: func(_, _, _ string) error { return fmt.Errorf("API error") },
	}
	_, err := Run(root, "owner/repo", "targetsha", mc)
	if err == nil {
		t.Fatal("expected error when MoveTag fails")
	}
}

// Tests for RealGitHubClient using httptest.

func newTestClient(handler http.HandlerFunc) (*RealGitHubClient, *httptest.Server) {
	srv := httptest.NewServer(handler)
	c := &RealGitHubClient{Token: "test-token"}
	c.Do = func(req *http.Request) (*http.Response, error) {
		// Rewrite API base to test server.
		req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
		req.URL.Scheme = "http"
		return http.DefaultClient.Do(req)
	}
	return c, srv
}

func TestGitHubClient_CommitSHA_LightweightTag(t *testing.T) {
	const (
		tag    = "camunda-platform-8.7-12.12.1"
		commit = "abc1234abc1234abc1234abc1234abc1234abc123"
	)
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "refs/tags") {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"object": map[string]any{"sha": commit, "type": "commit"},
		})
	})
	defer srv.Close()

	got, err := c.CommitSHA("owner/repo", tag)
	if err != nil {
		t.Fatalf("CommitSHA: %v", err)
	}
	if got != commit {
		t.Errorf("CommitSHA = %q, want %q", got, commit)
	}
}

func TestGitHubClient_CommitSHA_AnnotatedTag(t *testing.T) {
	const (
		tag       = "camunda-platform-8.7-12.12.1"
		tagObjSHA = "tagobj111tagobj111tagobj111tagobj111tagobj1"
		commitSHA = "abc1234abc1234abc1234abc1234abc1234abc123"
	)
	callCount := 0
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "refs/tags") {
			// First call: return annotated tag object ref.
			json.NewEncoder(w).Encode(map[string]any{
				"object": map[string]any{"sha": tagObjSHA, "type": "tag"},
			})
			return
		}
		// Second call: resolve tag object → commit.
		json.NewEncoder(w).Encode(map[string]any{
			"object": map[string]any{"sha": commitSHA, "type": "commit"},
		})
	})
	defer srv.Close()

	got, err := c.CommitSHA("owner/repo", tag)
	if err != nil {
		t.Fatalf("CommitSHA: %v", err)
	}
	if got != commitSHA {
		t.Errorf("CommitSHA = %q, want %q", got, commitSHA)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for annotated tag, got %d", callCount)
	}
}

func TestGitHubClient_CommitSHA_TagNotFound(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	defer srv.Close()

	got, err := c.CommitSHA("owner/repo", "no-such-tag")
	if err != nil {
		t.Fatalf("CommitSHA: %v", err)
	}
	if got != "" {
		t.Errorf("CommitSHA = %q, want empty string for missing tag", got)
	}
}

func TestGitHubClient_MoveTag(t *testing.T) {
	var capturedBody []byte
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "want PATCH", http.StatusMethodNotAllowed)
			return
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	const sha = "abc1234abc1234abc1234abc1234abc1234abc123"
	if err := c.MoveTag("owner/repo", "my-tag", sha); err != nil {
		t.Fatalf("MoveTag: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body["sha"] != sha {
		t.Errorf("body sha = %v, want %q", body["sha"], sha)
	}
	if body["force"] != true {
		t.Errorf("body force = %v, want true", body["force"])
	}
}

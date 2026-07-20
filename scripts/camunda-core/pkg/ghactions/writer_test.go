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

package ghactions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetSingleLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	w := &Writer{Path: path}
	if err := w.Set("CHART_PATH", "charts/camunda-platform-8.10"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := readAll(t, path)
	if got != "CHART_PATH=charts/camunda-platform-8.10\n" {
		t.Errorf("got %q", got)
	}
}

func TestSetMultilineUsesRandomDelimiter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	w := &Writer{Path: path}
	// A value that contains the delimiter previously hardcoded would corrupt
	// the file; the random delimiter must not appear inside the value.
	value := "line1\n__EOF__\nline3"
	if err := w.Set("KEY", value); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := readAll(t, path)
	if !strings.HasPrefix(got, "KEY<<ghadelimiter_") {
		t.Fatalf("expected random heredoc delimiter, got %q", got)
	}
	delim := strings.TrimSuffix(strings.TrimPrefix(strings.SplitN(got, "\n", 2)[0], "KEY<<"), "")
	if strings.Contains(value, delim) {
		t.Errorf("delimiter %q must not occur in value", delim)
	}
	// The value must round-trip verbatim between the two delimiter lines.
	if !strings.Contains(got, "\n"+value+"\n"+delim+"\n") {
		t.Errorf("value not framed by delimiter\n%q", got)
	}
}

func TestSetNoPathFailsInGitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	w := &Writer{}
	if err := w.Set("SECRET", "value"); err == nil {
		t.Fatal("expected error when no target file is configured in GitHub Actions")
	}
}

func TestSetNoPathFallsBackToStdoutLocally(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	w := &Writer{}
	if err := w.Set("KEY", "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
}

func TestRandomDelimiterUnique(t *testing.T) {
	a, err := randomDelimiter()
	if err != nil {
		t.Fatalf("randomDelimiter: %v", err)
	}
	b, err := randomDelimiter()
	if err != nil {
		t.Fatalf("randomDelimiter: %v", err)
	}
	if a == b {
		t.Errorf("expected distinct delimiters, got %q twice", a)
	}
}

func readAll(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

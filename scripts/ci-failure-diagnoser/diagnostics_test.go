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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadDiagnostics_ResolvesPodLogs(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "ns-foo", "20260309T120000Z")
	logsDir := filepath.Join(runDir, "logs")

	summary := `{
  "namespace": "ns-foo",
  "collectedAt": "2026-03-09T12:00:00Z",
  "podLogTailLines": 500,
  "pods": "NAME READY STATUS\nzeebe-0 0/1 CrashLoopBackOff",
  "events": "Warning Failed image pull",
  "podLogs": [
    {"pod": "zeebe-0", "file": "zeebe-0.log"},
    {"pod": "operate-0", "file": "operate-0.log"}
  ],
  "testOutputLast200": "venom: scenario failed"
}`
	writeFile(t, filepath.Join(runDir, "summary.json"), summary)
	writeFile(t, filepath.Join(logsDir, "zeebe-0.log"), "panic: boom\n")
	writeFile(t, filepath.Join(logsDir, "operate-0.log"), "ConnectException to elasticsearch:9200\n")

	d, err := LoadDiagnostics(root)
	if err != nil {
		t.Fatalf("LoadDiagnostics: %v", err)
	}
	if d.Namespace != "ns-foo" {
		t.Errorf("Namespace = %q, want %q", d.Namespace, "ns-foo")
	}
	if !strings.Contains(d.Pods, "CrashLoopBackOff") {
		t.Errorf("Pods missing crash status: %q", d.Pods)
	}
	if len(d.PodLogs) != 2 {
		t.Fatalf("PodLogs = %d, want 2", len(d.PodLogs))
	}
	if !strings.Contains(d.PodLogs[0].Body, "panic: boom") {
		t.Errorf("first pod log body wrong: %q", d.PodLogs[0].Body)
	}
}

func TestLoadDiagnostics_PicksMostRecentRun(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "ns-foo", "20260309T100000Z")
	newer := filepath.Join(root, "ns-foo", "20260309T120000Z")
	writeFile(t, filepath.Join(older, "summary.json"), `{"namespace":"old","collectedAt":"old","podLogTailLines":1}`)
	writeFile(t, filepath.Join(newer, "summary.json"), `{"namespace":"new","collectedAt":"new","podLogTailLines":1}`)

	d, err := LoadDiagnostics(root)
	if err != nil {
		t.Fatalf("LoadDiagnostics: %v", err)
	}
	if d.Namespace != "new" {
		t.Errorf("expected newest run, got namespace=%q", d.Namespace)
	}
}

func TestLoadDiagnostics_DirectRunDir(t *testing.T) {
	runDir := t.TempDir()
	writeFile(t, filepath.Join(runDir, "summary.json"), `{"namespace":"direct","collectedAt":"now","podLogTailLines":1}`)

	d, err := LoadDiagnostics(runDir)
	if err != nil {
		t.Fatalf("LoadDiagnostics: %v", err)
	}
	if d.Namespace != "direct" {
		t.Errorf("Namespace = %q, want %q", d.Namespace, "direct")
	}
}

func TestLoadDiagnostics_NoSummary(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadDiagnostics(root); err == nil {
		t.Fatal("expected error for missing summary.json")
	}
}

func TestLoadDiagnostics_BadLogFileRecorded(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "ns", "20260309T120000Z")
	writeFile(t, filepath.Join(runDir, "summary.json"), `{
  "namespace": "ns",
  "collectedAt": "now",
  "podLogTailLines": 1,
  "podLogs": [{"pod":"p","file":"missing.log"}]
}`)

	d, err := LoadDiagnostics(root)
	if err != nil {
		t.Fatalf("LoadDiagnostics: %v", err)
	}
	if len(d.PodLogs) != 0 {
		t.Errorf("expected no successful pod logs, got %d", len(d.PodLogs))
	}
	if len(d.CollectErrors) == 0 {
		t.Error("expected a collection error for missing log file")
	}
}

func TestTrimLogs_TailBiasAndLineBoundary(t *testing.T) {
	d := &Diagnostics{PodLogs: []PodLog{
		{Pod: "p1", Body: "head1\nhead2\nhead3\nimportant: actual error here\nfinal\n"},
		{Pod: "p2", Body: "small body\n"},
	}}
	// Cap large enough to include the important line plus the final line, but
	// small enough that the early head* lines are dropped.
	d.TrimLogs(40)

	if !strings.Contains(d.PodLogs[0].Body, "important: actual error here") {
		t.Errorf("trim dropped tail: %q", d.PodLogs[0].Body)
	}
	if strings.Contains(d.PodLogs[0].Body, "head1") {
		t.Errorf("trim should drop head: %q", d.PodLogs[0].Body)
	}
	if !strings.HasPrefix(d.PodLogs[0].Body, "...[truncated]...") {
		t.Errorf("trim should mark truncation; got %q", d.PodLogs[0].Body)
	}
	if d.PodLogs[1].Body != "small body\n" {
		t.Errorf("small body should pass through unchanged, got %q", d.PodLogs[1].Body)
	}
}

func TestTrimLogs_AggressiveCapMayDropToLastLine(t *testing.T) {
	// When the cap is smaller than a single tail line, the line-boundary cut
	// keeps only the last whole line. This is intentional: prefer well-formed
	// output to mid-line bytes.
	d := &Diagnostics{PodLogs: []PodLog{
		{Pod: "p", Body: "head1\nhead2\nhead3\nimportant: actual error here\nfinal\n"},
	}}
	d.TrimLogs(20)
	if !strings.Contains(d.PodLogs[0].Body, "final") {
		t.Errorf("expected final line to survive, got %q", d.PodLogs[0].Body)
	}
}

func TestTrimLogs_ZeroIsNoop(t *testing.T) {
	d := &Diagnostics{PodLogs: []PodLog{{Pod: "p", Body: "abcdefghij"}}}
	d.TrimLogs(0)
	if d.PodLogs[0].Body != "abcdefghij" {
		t.Errorf("expected no-op, got %q", d.PodLogs[0].Body)
	}
}

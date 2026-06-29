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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/camunda-core/pkg/retagger"
)

func TestRunRetagRelease_MissingFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no flags", []string{}},
		{"missing sha", []string{"--repo", "owner/repo"}},
		{"missing repo", []string{"--sha", "abc1234"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := runRetagRelease(tc.args); err == nil {
				t.Fatal("expected error for missing required flags")
			}
		})
	}
}

func TestWriteRetagSummary_WritesStepSummary(t *testing.T) {
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	results := []retagger.Result{
		{TagName: "camunda-platform-8.7-12.12.1", Moved: true, Reason: "moved from aaa to bbb"},
		{TagName: "camunda-platform-8.10-13.5.2", Moved: false, Reason: "already correct"},
	}
	if err := writeRetagSummary(results, "abc1234abc1234abc1234abc1234abc1234abc123"); err != nil {
		t.Fatalf("writeRetagSummary: %v", err)
	}

	content, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	got := string(content)
	if !strings.Contains(got, "camunda-platform-8.7-12.12.1") {
		t.Error("summary missing moved tag")
	}
	if !strings.Contains(got, "camunda-platform-8.10-13.5.2") {
		t.Error("summary missing skipped tag")
	}
	if !strings.Contains(got, "abc1234") {
		t.Error("summary missing short SHA")
	}
}

func TestWriteRetagSummary_NoSummaryPath(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	// Should not fail when GITHUB_STEP_SUMMARY is unset.
	if err := writeRetagSummary(nil, "sha"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

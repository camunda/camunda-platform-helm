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

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/e2erun"
)

func TestCIE2ERunWithoutLegsSucceeds(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "out"))
	command := newCIE2ERunCommand()
	command.SetArgs([]string{"--repo-root", t.TempDir(), "--chart-dir", "camunda-platform-8.10", "--namespace", "ns", "--legs", "[]"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestReportE2ERunWritesOutputsAndFailsOnBlocking(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out")
	summaryPath := filepath.Join(dir, "summary")
	t.Setenv("GITHUB_OUTPUT", outputPath)
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	for _, tc := range []struct {
		name              string
		blocking, wantErr bool
		wantOutput        string
	}{
		{"non-blocking failure passes", false, false, "blocking-failed=false\nnon-blocking-failed=true\n"},
		{"blocking failure fails", true, true, "blocking-failed=true\nnon-blocking-failed=false\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(outputPath)
			err := reportE2ERun(e2erun.Result{Legs: []e2erun.LegResult{
				{Leg: e2erun.Leg{ID: "ok", Blocking: true}},
				{Leg: e2erun.Leg{ID: "bad", Blocking: tc.blocking}, Err: errors.New("boom")},
			}})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %t", err, tc.wantErr)
			}
			got, readErr := os.ReadFile(outputPath)
			if readErr != nil {
				t.Fatalf("read output: %v", readErr)
			}
			if string(got) != tc.wantOutput {
				t.Fatalf("output = %q, want %q", got, tc.wantOutput)
			}
		})
	}

	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	if !strings.Contains(string(summary), "| `bad` |") {
		t.Fatalf("summary missing failed leg: %s", summary)
	}
}

func TestCIE2ERunPlanWritesCountAndEnforcesMax(t *testing.T) {
	legs := `[{"suite":"smoke","blocking":true,"shard_index":1,"shard_total":1},{"suite":"full","blocking":false,"shard_index":1,"shard_total":1}]`
	base := []string{"--repo-root", t.TempDir(), "--chart-dir", "camunda-platform-8.10", "--namespace", "ns", "--legs", legs, "--plan"}

	outputPath := filepath.Join(t.TempDir(), "out")
	t.Setenv("GITHUB_OUTPUT", outputPath)
	command := newCIE2ERunCommand()
	command.SetArgs(append(base, "--max-legs", "2"))
	if err := command.Execute(); err != nil {
		t.Fatalf("plan: %v", err)
	}
	got, _ := os.ReadFile(outputPath)
	if string(got) != "count=2\n" {
		t.Fatalf("output = %q", got)
	}

	command = newCIE2ERunCommand()
	command.SetArgs(append(base, "--max-legs", "1"))
	if err := command.Execute(); err == nil {
		t.Fatal("expected an error when more legs are planned than --max-legs")
	}
}

func TestCIE2ERunReportFailsWhenBlockingLegDidNotRun(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", filepath.Join(t.TempDir(), "out"))
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	artifacts := t.TempDir()
	command := newCIE2ERunCommand()
	command.SetArgs([]string{"--repo-root", t.TempDir(), "--chart-dir", "camunda-platform-8.10", "--namespace", "ns",
		"--legs", `[{"suite":"smoke","blocking":true,"shard_index":1,"shard_total":1}]`, "--artifacts-dir", artifacts, "--report"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected report to fail for a blocking leg without a result")
	}
}

func TestCIE2ERunRejectsOutOfRangeLegIndex(t *testing.T) {
	command := newCIE2ERunCommand()
	command.SetArgs([]string{"--repo-root", t.TempDir(), "--chart-dir", "camunda-platform-8.10", "--namespace", "ns", "--legs", "[]", "--leg-index", "0"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected out-of-range error")
	}
}

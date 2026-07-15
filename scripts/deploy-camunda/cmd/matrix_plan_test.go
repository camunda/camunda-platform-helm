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
	"path/filepath"
	"strings"
	"testing"
)

func TestMatrixPlanWritesGitHubOutputs(t *testing.T) {
	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "github_output")
	t.Setenv("GITHUB_OUTPUT", outputPath)

	command := newMatrixPlanCommand()
	command.SetArgs([]string{
		"--repo-root", repoRoot,
		"--active-versions", "8.10,8.9",
		"--manual-trigger", "8.10",
		"--tier", "1",
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute matrix plan: %v", err)
	}

	output := readFile(t, outputPath)
	if !strings.Contains(output, `matrix={"include":[`) {
		t.Errorf("GITHUB_OUTPUT missing matrix output:\n%s", output)
	}
	if !strings.Contains(output, `camunda-versions=["8.10"]`) {
		t.Errorf("GITHUB_OUTPUT missing camunda-versions output:\n%s", output)
	}
}

func TestMatrixPlanRejectsInvalidTier(t *testing.T) {
	command := newMatrixPlanCommand()
	command.SetArgs([]string{
		"--repo-root", t.TempDir(),
		"--active-versions", "8.10",
		"--tier", "not-a-number",
	})
	command.SilenceErrors = true
	command.SilenceUsage = true
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --tier") {
		t.Fatalf("error = %v, want invalid --tier error", err)
	}
}

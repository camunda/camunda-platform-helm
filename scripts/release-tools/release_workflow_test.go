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

	"gopkg.in/yaml.v3"
)

type sourceWorkflow struct {
	Jobs map[string]struct {
		Outputs map[string]string `yaml:"outputs"`
		Steps   []struct {
			Name string `yaml:"name"`
			Run  string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func sourceTestWorkflow(t *testing.T, file string) sourceWorkflow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", file))
	if err != nil {
		t.Fatal(err)
	}
	var workflow sourceWorkflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func (w sourceWorkflow) stepRun(t *testing.T, job, name string) string {
	t.Helper()
	for _, step := range w.Jobs[job].Steps {
		if step.Name == name {
			return step.Run
		}
	}
	t.Fatalf("job %q has no step %q", job, name)
	return ""
}

func TestReleaseWorkflowsPassArtifactSources(t *testing.T) {
	public := sourceTestWorkflow(t, "chart-release-public.yaml")
	if public.Jobs["release"].Outputs["source-commit"] != "${{ steps.parse.outputs.commit_sha }}" ||
		public.Jobs["release"].Outputs["previous-source-commit"] != "${{ steps.history.outputs.previous_commit_sha }}" {
		t.Error("the release job must export both artifact-source commits")
	}
	for _, tc := range []struct {
		file, job, step string
		args            []string
	}{
		{"chart-release-public.yaml", "release", "Run Chart Releaser - Tagging/Uploading",
			[]string{`--commit "${{ steps.parse.outputs.commit_sha }}"`}},
		{"chart-release-public.yaml", "post-release", "Label PRs with version",
			[]string{`sourceSHA="${{ needs.release.outputs.source-commit }}"`, `previousSourceSHA="${{ needs.release.outputs.previous-source-commit }}"`}},
	} {
		run := sourceTestWorkflow(t, tc.file).stepRun(t, tc.job, tc.step)
		for _, arg := range tc.args {
			if !strings.Contains(run, arg) {
				t.Errorf("%s: step %q lacks %s", tc.file, tc.step, arg)
			}
		}
	}
}

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
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type sourceWorkflowStep struct {
	Name string            `yaml:"name"`
	ID   string            `yaml:"id"`
	Run  string            `yaml:"run"`
	Uses string            `yaml:"uses"`
	If   string            `yaml:"if"`
	Env  map[string]string `yaml:"env"`
	With struct {
		Tools      string `yaml:"tools"`
		FetchDepth *int   `yaml:"fetch-depth"`
	} `yaml:"with"`
}

type sourceWorkflow struct {
	Jobs map[string]struct {
		Outputs map[string]string    `yaml:"outputs"`
		Steps   []sourceWorkflowStep `yaml:"steps"`
	} `yaml:"jobs"`
}

func sourceTestWorkflow(t *testing.T, file string) sourceWorkflow {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("../..", ".github/workflows", file))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var workflow sourceWorkflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func sourceWorkflowStepNamed(t *testing.T, steps []sourceWorkflowStep, name string) (sourceWorkflowStep, int) {
	t.Helper()
	for i, step := range steps {
		if step.Name == name {
			return step, i
		}
	}
	t.Fatalf("missing workflow step %q", name)
	return sourceWorkflowStep{}, -1
}

func TestPublicReleaseWorkflowSourceContract(t *testing.T) {
	workflow := sourceTestWorkflow(t, "chart-release-public.yaml")
	release := workflow.Jobs["release"]
	prepare, prepareAt := sourceWorkflowStepNamed(t, release.Steps, "Prepare and validate release artifact")
	upload, uploadAt := sourceWorkflowStepNamed(t, release.Steps, "Run Chart Releaser - Tagging/Uploading")
	verify, verifyAt := sourceWorkflowStepNamed(t, release.Steps, "Verify published artifact provenance")
	_, indexAt := sourceWorkflowStepNamed(t, release.Steps, "Run Chart Releaser - Indexing")
	_, signAt := sourceWorkflowStepNamed(t, release.Steps, "Sign Helm chart with Cosign")
	if !(prepareAt < uploadAt && uploadAt < verifyAt && verifyAt < indexAt && verifyAt < signAt) {
		t.Fatal("provenance must be validated before publishing and verified before indexing/signing")
	}
	if prepare.ID != "parse" || !strings.Contains(prepare.Run, "release-source prepare") || !strings.Contains(prepare.Run, `--dry-run="${DRY_RUN}"`) {
		t.Fatal("public preflight must invoke the tested source resolver for normal and dry runs")
	}
	if prepare.Env["RC_TAG"] != "${{ inputs.rc-tag }}" || prepare.Env["HARBOR_REGISTRY_PASSWORD"] == "" || prepare.Env["GH_TOKEN"] == "" {
		t.Fatal("preflight must receive the selected tag and its scoped read credentials")
	}
	if !strings.Contains(upload.Run, `--commit "${{ steps.parse.outputs.commit_sha }}"`) || !strings.Contains(upload.Run, "--skip-existing=false") {
		t.Fatal("upload must use the verified artifact source and reject a competing release")
	}
	if !strings.Contains(verify.Run, `--source "${{ steps.parse.outputs.commit_sha }}"`) || !strings.Contains(verify.Run, `steps.parse.outputs.package_file`) {
		t.Fatal("post-upload verification must check the same source and package")
	}
	for _, step := range []sourceWorkflowStep{upload, verify} {
		if step.If != "${{ !inputs.dry-run }}" {
			t.Fatalf("%s must be disabled in dry runs", step.Name)
		}
	}
	if release.Outputs["source-commit"] != "${{ steps.parse.outputs.commit_sha }}" ||
		release.Outputs["previous-source-commit"] != "${{ steps.parse.outputs.previous_commit_sha }}" {
		t.Fatal("both source boundaries must cross the release/post-release job boundary")
	}
	label, _ := sourceWorkflowStepNamed(t, workflow.Jobs["post-release"].Steps, "Label PRs with version")
	for _, arg := range []string{
		`sourceSHA="${{ needs.release.outputs.source-commit }}"`,
		`previousSourceSHA="${{ needs.release.outputs.previous-source-commit }}"`,
		`chartVersion="${{ needs.release.outputs.version }}"`,
	} {
		if !strings.Contains(label.Run, arg) {
			t.Fatalf("label step is missing %s", arg)
		}
	}
	checkout, _ := sourceWorkflowStepNamed(t, release.Steps, "Checkout")
	if checkout.With.FetchDepth == nil || *checkout.With.FetchDepth != 0 {
		t.Fatal("source and ancestry validation requires complete Git history")
	}
}

func TestReleaseHistoryWorkflowConsumers(t *testing.T) {
	for _, tc := range []struct {
		file, job, releaseStep, notesStep, source string
	}{
		{"chart-build-dev.yaml", "build", "Compute release version (release-please dry-run)", "Generate RELEASE-NOTES.md", `"${GITHUB_SHA}"`},
		{"chart-promote-rc.yaml", "promote-rc", "Run release-please", "Generate release files on the release-please PR", `"${{ steps.parse.outputs.sha }}"`},
	} {
		t.Run(tc.file, func(t *testing.T) {
			workflow := sourceTestWorkflow(t, tc.file)
			job, ok := workflow.Jobs[tc.job]
			if !ok {
				t.Fatalf("missing job %s", tc.job)
			}
			release, _ := sourceWorkflowStepNamed(t, job.Steps, tc.releaseStep)
			notes, _ := sourceWorkflowStepNamed(t, job.Steps, tc.notesStep)
			for _, step := range []sourceWorkflowStep{release, notes} {
				if !strings.Contains(step.Run, "--source "+tc.source) || step.Env["HARBOR_REGISTRY_PASSWORD"] == "" {
					t.Fatalf("%s must receive the explicit source and Harbor credentials", step.Name)
				}
			}
			if !strings.Contains(release.Run, "/tmp/release-tools release-pr") {
				t.Fatal("release-please history must use the source-bound SDK entrypoint")
			}
			if !strings.Contains(notes.Run, "release-notes --main") {
				t.Fatal("notes must use the source-bound generator")
			}
			if tc.job == "promote-rc" && !strings.Contains(notes.Run, `CHART_VERSION="${chart_version}" yq -i '.version = strenv(CHART_VERSION)'`) {
				t.Fatal("promotion rehearsal must generate the selected package version, not the source's old version")
			}
		})
	}
}

func TestReleaseWorkflowSDKAndStepReferences(t *testing.T) {
	stepRef := regexp.MustCompile(`steps\.([A-Za-z0-9_-]+)\.`)
	for _, file := range []string{"chart-build-dev.yaml", "chart-promote-rc.yaml", "chart-release-public.yaml", "test-release-tooling.yaml"} {
		t.Run(file, func(t *testing.T) {
			workflow := sourceTestWorkflow(t, file)
			installedSDK := false
			for _, job := range workflow.Jobs {
				ids := map[string]bool{}
				node := false
				for _, step := range job.Steps {
					ids[step.ID] = true
					node = node || slices.Contains(strings.Fields(step.With.Tools), "nodejs")
				}
				for _, step := range job.Steps {
					if strings.Contains(step.Run, "npm install -g release-please@17.1.3") {
						installedSDK = true
						if !node {
							t.Fatal("the SDK consumer must install the pinned Node tool")
						}
					}
					values := []string{step.Run}
					for _, value := range step.Env {
						values = append(values, value)
					}
					for _, value := range values {
						for _, match := range stepRef.FindAllStringSubmatch(value, -1) {
							if !ids[match[1]] {
								t.Fatalf("step %s refers to nonexistent step %s", step.Name, match[1])
							}
						}
					}
				}
			}
			if file != "chart-release-public.yaml" && !installedSDK {
				t.Fatal("release-please SDK must be pinned consistently in producers and contract tests")
			}
		})
	}
}

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
	"os/exec"
	"path/filepath"
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

func TestPublicReleaseRejectsMismatchedPackageVersion(t *testing.T) {
	workflow := sourceTestWorkflow(t, "chart-release-public.yaml")
	history, _ := sourceWorkflowStepNamed(t, workflow.Jobs["release"].Steps, "Resolve previous release source")
	t.Setenv("PACKAGE_VERSION", "14.11.0")
	t.Setenv("RC_VERSION", "14.11.1")
	output, err := exec.Command("bash", "-c", history.Run).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "::error::The pulled package version does not match the selected RC") {
		t.Fatalf("mismatched package was not rejected: %v\n%s", err, output)
	}
}

func TestPublicReleaseWorkflowSourceContract(t *testing.T) {
	workflow := sourceTestWorkflow(t, "chart-release-public.yaml")
	release := workflow.Jobs["release"]
	source, sourceAt := sourceWorkflowStepNamed(t, release.Steps, "Resolve RC source")
	pull, pullAt := sourceWorkflowStepNamed(t, release.Steps, "Pull immutable RC package")
	_, metadataAt := sourceWorkflowStepNamed(t, release.Steps, "Extract metadata from package")
	history, historyAt := sourceWorkflowStepNamed(t, release.Steps, "Resolve previous release source")
	upload, uploadAt := sourceWorkflowStepNamed(t, release.Steps, "Run Chart Releaser - Tagging/Uploading")
	_, indexAt := sourceWorkflowStepNamed(t, release.Steps, "Run Chart Releaser - Indexing")
	if !(sourceAt < pullAt && pullAt < metadataAt && metadataAt < historyAt && historyAt < uploadAt && uploadAt < indexAt) {
		t.Fatal("artifact source, immutable pull and previous source must precede publication")
	}
	if source.ID != "parse" || !strings.Contains(source.Run, "harbor-tag source") || !strings.Contains(source.Run, `--dry-run="${DRY_RUN}"`) ||
		!strings.Contains(source.Run, `git rev-parse --verify "${source_ref}^{commit}"`) ||
		!strings.Contains(source.Run, `git merge-base --is-ancestor "${source_sha}" origin/main`) {
		t.Fatal("the workflow must resolve and validate the recorded source commit")
	}
	if source.Env["RC_TAG"] != "${{ inputs.rc-tag }}" || source.Env["HARBOR_REGISTRY_USER"] == "" || source.Env["HARBOR_REGISTRY_PASSWORD"] == "" {
		t.Fatal("source resolution requires the selected tag and Harbor credentials")
	}
	if pull.Env["ARTIFACT_DIGEST"] != "${{ steps.parse.outputs.artifact_digest }}" ||
		!strings.Contains(pull.Run, `camunda-platform@${ARTIFACT_DIGEST}`) || !strings.Contains(pull.Run, "--destination .cr-release-packages") {
		t.Fatal("the uploaded package must be pulled by the resolved immutable digest")
	}
	if history.ID != "history" || history.Env["SOURCE_SHA"] != "${{ steps.parse.outputs.commit_sha }}" ||
		history.Env["RC_VERSION"] != "${{ steps.parse.outputs.version }}" || history.Env["PACKAGE_VERSION"] != "${{ steps.metadata.outputs.version }}" ||
		history.Env["GH_TOKEN"] == "" ||
		!strings.Contains(history.Run, "release-history") || !strings.Contains(history.Run, `git merge-base --is-ancestor "${previous_sha}" "${SOURCE_SHA}"`) {
		t.Fatal("previous-source selection requires package identity, read credentials and an explicit source bound")
	}
	for _, key := range []string{"HARBOR_REGISTRY_USER", "HARBOR_REGISTRY_PASSWORD"} {
		if _, exists := history.Env[key]; exists {
			t.Fatal("published history must not depend on Harbor credentials or retention")
		}
	}
	if !strings.Contains(upload.Run, `--commit "${{ steps.parse.outputs.commit_sha }}"`) || !strings.Contains(upload.Run, "--skip-existing=false") {
		t.Fatal("upload must use the verified artifact source and reject a competing release")
	}
	if upload.If != "${{ !inputs.dry-run }}" {
		t.Fatal("upload must be disabled in dry runs")
	}
	if release.Outputs["source-commit"] != "${{ steps.parse.outputs.commit_sha }}" ||
		release.Outputs["previous-source-commit"] != "${{ steps.history.outputs.previous_commit_sha }}" {
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

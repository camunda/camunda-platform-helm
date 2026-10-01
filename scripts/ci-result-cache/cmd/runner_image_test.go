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

package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/ci-result-cache/pkg/hash"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestResolveRunnerImage(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, testCase := range []struct {
		name       string
		output     string
		fail       bool
		missingCLI bool
		noToken    bool
		cancelled  bool
		wantImage  bool
	}{
		{name: "authenticated resolution", output: digest + "\n", wantImage: true},
		{name: "registry failure", fail: true},
		{name: "missing oras", missingCLI: true},
		{name: "missing credentials", noToken: true},
		{name: "cancelled lookup", cancelled: true},
		{name: "empty output"},
		{name: "mutable tag", output: "latest\n"},
		{name: "malformed digest", output: "sha256:abcd\n"},
		{name: "extra output", output: digest + "\nimage=injected\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			binDir := t.TempDir()
			t.Setenv("PATH", binDir)
			t.Setenv("GITHUB_TOKEN", "test-token")
			t.Setenv("GITHUB_ACTOR", "test-user")
			if testCase.noToken {
				t.Setenv("GITHUB_TOKEN", "")
			}
			script := "#!/bin/sh\n" +
				"test \"$1\" = resolve || exit 2\n" +
				"test \"$2\" = --username || exit 2\n" +
				"test \"$3\" = test-user || exit 2\n" +
				"test \"$4\" = --password-stdin || exit 2\n" +
				"test \"$5\" = " + hash.PlaywrightRunnerImage + ":latest || exit 2\n" +
				"test \"$#\" = 5 || exit 2\n" +
				"read -r token\ntest \"$token\" = test-token || exit 2\n" +
				"printf '%s' '" + testCase.output + "'\n"
			if testCase.fail {
				script += "exit 1\n"
			}
			if !testCase.missingCLI {
				if err := os.WriteFile(filepath.Join(binDir, "oras"), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			command := &cobra.Command{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if testCase.cancelled {
				cancel()
			}
			command.SetContext(ctx)
			var output, warnings bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&warnings)
			if err := runResolveRunnerImage(command, nil); err != nil {
				t.Fatal(err)
			}
			if testCase.wantImage {
				if output.String() != "image="+hash.PlaywrightRunnerImage+"@"+digest+"\n" || warnings.Len() != 0 {
					t.Fatalf("unexpected resolution output %q, warnings %q", output.String(), warnings.String())
				}
			} else if output.Len() != 0 || !strings.Contains(warnings.String(), "result cache is off") {
				t.Fatalf("failure must disable caching: output %q, warnings %q", output.String(), warnings.String())
			}
			if strings.Contains(output.String()+warnings.String(), "test-token") {
				t.Fatal("resolver exposed the token")
			}
		})
	}
}

func TestRunnerImageSurvivesRetag(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTOR", "test-user")
	t.Setenv("TEST_DIGEST", "sha256:"+strings.Repeat("a", 64))
	if err := os.WriteFile(filepath.Join(binDir, "oras"), []byte("#!/bin/sh\nprintf '%s\\n' \"$TEST_DIGEST\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	image, err := resolveRunnerImage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := t.TempDir()
	lookupHash, err := hash.Compute(repoRoot, []string{"8.10"}, "0.0.1", image)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_DIGEST", "sha256:"+strings.Repeat("b", 64))
	recordedHash, err := hash.Compute(repoRoot, []string{"8.10"}, "0.0.1", image)
	if err != nil {
		t.Fatal(err)
	}
	if recordedHash != lookupHash {
		t.Fatal("recording must retain the image selected at lookup")
	}
	nextImage, err := resolveRunnerImage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	nextHash, err := hash.Compute(repoRoot, []string{"8.10"}, "0.0.1", nextImage)
	if err != nil {
		t.Fatal(err)
	}
	if nextHash == recordedHash {
		t.Fatal("a later run must miss the cache after a runner retag")
	}
}

func TestCacheCommandsRequireRunnerImage(t *testing.T) {
	for _, command := range []*cobra.Command{annotateMatrixCmd, checkCmd, recordCmd} {
		flag := command.Flags().Lookup("playwright-runner-image")
		if flag == nil || len(flag.Annotations[cobra.BashCompOneRequiredFlag]) == 0 || flag.Annotations[cobra.BashCompOneRequiredFlag][0] != "true" {
			t.Errorf("%s must require --playwright-runner-image", command.Name())
		}
	}
}

func TestRunnerImageWorkflowContract(t *testing.T) {
	t.Parallel()
	type workflow struct {
		On struct {
			WorkflowCall struct {
				Inputs map[string]struct {
					Type     string
					Default  string
					Required bool
				}
			} `yaml:"workflow_call"`
		}
		Permissions map[string]string
		Jobs        map[string]struct {
			Uses      string
			If        string
			With      map[string]string
			Outputs   map[string]string
			Container struct {
				Image string
			}
			Steps []struct {
				ID   string
				Run  string
				If   string
				Uses string
				With map[string]string
				Env  map[string]string
			}
		}
	}
	readWorkflow := func(name string) workflow {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		var result workflow
		if err := yaml.Unmarshal(contents, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	const input = "playwright-runner-image"
	const resolvedImage = "${{ steps.resolve-runner-image.outputs.image }}"
	const passedImage = "${{ inputs.playwright-runner-image }}"
	files := []string{"test-chart-version.yaml", "test-chart-version-template.yaml", "test-integration-template.yaml", "test-integration-runner.yaml"}
	resolutionCount := 0
	for index, file := range files {
		contents := readWorkflow(file)
		if index > 0 {
			declaration := contents.On.WorkflowCall.Inputs[input]
			if declaration.Type != "string" || declaration.Required || declaration.Default != "" {
				t.Errorf("%s must accept an optional empty runner image", file)
			}
		}
		if index < len(files)-1 {
			calls := 0
			for _, job := range contents.Jobs {
				if job.Uses != "./.github/workflows/"+files[index+1] {
					continue
				}
				calls++
				want := passedImage
				if index == 0 {
					want = "${{ needs.init.outputs.playwright-runner-image }}"
				}
				if job.With[input] != want {
					t.Errorf("%s must forward the same runner image", file)
				}
			}
			if calls != 1 {
				t.Errorf("%s: found %d workflow handoffs, want 1", file, calls)
			}
		}
		for _, job := range contents.Jobs {
			for _, step := range job.Steps {
				if strings.Contains(step.Run, " resolve-runner-image") {
					resolutionCount++
					if file != files[0] || step.ID != "resolve-runner-image" || step.Env["GITHUB_TOKEN"] != "${{ github.token }}" {
						t.Errorf("runner resolution must use the init job's scoped GitHub token")
					}
				}
			}
		}
	}
	if resolutionCount != 1 {
		t.Errorf("runner must be resolved exactly once, got %d resolutions", resolutionCount)
	}
	entry := readWorkflow(files[0])
	if entry.Permissions["packages"] != "read" || entry.Jobs["init"].Outputs[input] != resolvedImage {
		t.Fatal("init must expose its resolved image and have packages:read")
	}
	annotationFound, orasFound, resolutionSeen := false, false, false
	for _, step := range entry.Jobs["init"].Steps {
		if step.Uses == "./.github/actions/install-tool-versions" && step.With["tools"] == "oras" {
			orasFound = true
		}
		if step.ID == "resolve-runner-image" {
			resolutionSeen = true
			if !orasFound {
				t.Error("ORAS must be installed before resolution")
			}
		}
		if step.ID == "annotate-cache" {
			annotationFound = true
			if !resolutionSeen || !strings.Contains(step.If, "&& steps.resolve-runner-image.outputs.image != ''") || !strings.Contains(step.Run, "--playwright-runner-image \""+resolvedImage+"\"") {
				t.Error("cache annotation must require and hash the resolved image")
			}
		}
	}
	if !annotationFound {
		t.Error("cache annotation step is missing")
	}
	runner := readWorkflow(files[3])
	wantContainer := "${{ inputs.playwright-runner-image || '" + hash.PlaywrightRunnerImage + ":latest' }}"
	for _, jobID := range []string{"playwright-e2e-tests-after-install", "playwright-e2e-tests-after-upgrade", "e2e-topology-smoke", "shadow-e2e-full-suite"} {
		if runner.Jobs[jobID].Container.Image != wantContainer {
			t.Errorf("%s must execute the same pinned image, with latest only for uncached callers", jobID)
		}
	}
	record := runner.Jobs["record-result"]
	if !strings.Contains(record.If, "&&\n  inputs.playwright-runner-image != ''") {
		t.Error("recording must be disabled without a resolved runner image")
	}
	recordFound := false
	for _, step := range record.Steps {
		if strings.Contains(step.Run, "ci-result-cache record") {
			recordFound = true
			if !strings.Contains(step.Run, "--playwright-runner-image \""+passedImage+"\"") {
				t.Error("recording must hash the executed image")
			}
		}
	}
	if !recordFound {
		t.Error("cache recording step is missing")
	}
}

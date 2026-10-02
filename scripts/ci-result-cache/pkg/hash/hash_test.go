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

package hash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSuiteVersion = "0.0.1"
const testRunnerImage = "ghcr.io/camunda/team-distribution/playwright-runner@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestCompute_RunnerImage(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	baseline, err := Compute(repoRoot, []string{"8.10"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name      string
		image     string
		wantError bool
		wantSame  bool
	}{
		{name: "unchanged", image: testRunnerImage, wantSame: true},
		{name: "changed", image: strings.ReplaceAll(testRunnerImage, "sha256:aaaa", "sha256:bbbb")},
		{name: "missing", wantError: true},
		{name: "mutable tag", image: "ghcr.io/camunda/team-distribution/playwright-runner:latest", wantError: true},
		{name: "short digest", image: "ghcr.io/camunda/team-distribution/playwright-runner@sha256:abcd", wantError: true},
		{name: "invalid digest", image: strings.ReplaceAll(testRunnerImage, "sha256:aaaa", "sha256:zzzz"), wantError: true},
		{name: "wrong image", image: strings.ReplaceAll(testRunnerImage, "playwright-runner", "ci-runner"), wantError: true},
		{name: "newline", image: testRunnerImage + "\n", wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			result, err := Compute(repoRoot, []string{"8.10"}, testSuiteVersion, testCase.image)
			if testCase.wantError {
				if err == nil || result != "" {
					t.Fatalf("expected no hash and an error, got %q, %v", result, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if (result == baseline) != testCase.wantSame {
				t.Fatalf("hash equality = %t, want %t", result == baseline, testCase.wantSame)
			}
		})
	}
}

func TestCompute_DeterministicHash(t *testing.T) {
	// Create a temporary directory structure mimicking the repo.
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	deployDir := filepath.Join(tmpDir, "scripts", "deploy-camunda")

	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(deployDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write some test files.
	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\nversion: 1.0.0\n")
	writeFile(t, filepath.Join(chartDir, "values.yaml"), "key: value\n")
	writeFile(t, filepath.Join(deployDir, "main.go"), "package main\n")

	// Compute hash twice — should be identical.
	hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected deterministic hash, got %s and %s", hash1, hash2)
	}

	if len(hash1) != 64 { // SHA-256 hex length
		t.Errorf("expected 64-char hex hash, got %d chars: %s", len(hash1), hash1)
	}
}

func TestCompute_ChangesAffectHash(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\nversion: 1.0.0\n")

	hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	// Modify a file.
	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\nversion: 2.0.0\n")

	hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change when file content changes")
	}
}

func TestCompute_NewFileChangesHash(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\n")

	hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	// Add a new file.
	writeFile(t, filepath.Join(chartDir, "values.yaml"), "newKey: newValue\n")

	hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change when a new file is added")
	}
}

func TestCompute_DifferentVersionsDifferentHashes(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir89 := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	chartDir810 := filepath.Join(tmpDir, "charts", "camunda-platform-8.10")

	if err := os.MkdirAll(chartDir89, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(chartDir810, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(chartDir89, "Chart.yaml"), "version: 8.9\n")
	writeFile(t, filepath.Join(chartDir810, "Chart.yaml"), "version: 8.10\n")

	hash89, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("Compute 8.9: %v", err)
	}

	hash810, err := Compute(tmpDir, []string{"8.10"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("Compute 8.10: %v", err)
	}

	if hash89 == hash810 {
		t.Error("expected different hashes for different chart versions")
	}
}

func TestCompute_WorkflowFilesIncluded(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\n")

	hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	// Add a workflow file.
	workflowDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(workflowDir, "test-integration-runner.yaml"), "name: runner\n")

	hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change when a workflow file is added")
	}
}

func TestCompute_DeployCamundaAndCorePackagesIncluded(t *testing.T) {
	for _, pkg := range []string{
		filepath.Join("scripts", "deploy-camunda"),
		filepath.Join("scripts", "camunda-core"),
	} {
		t.Run(pkg, func(t *testing.T) {
			tmpDir := t.TempDir()
			chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
			if err := os.MkdirAll(chartDir, 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\n")

			pkgDir := filepath.Join(tmpDir, pkg, "pkg", "deployer")
			if err := os.MkdirAll(pkgDir, 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(pkgDir, "helm.go"), "package deployer\n")

			hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
			if err != nil {
				t.Fatalf("first Compute: %v", err)
			}

			// Modify a file in the shared Go package.
			writeFile(t, filepath.Join(pkgDir, "helm.go"), "package deployer\n// changed\n")

			hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
			if err != nil {
				t.Fatalf("second Compute: %v", err)
			}

			if hash1 == hash2 {
				t.Errorf("expected hash to change when a file under %s changes", pkg)
			}
		})
	}
}

func TestCompute_E2EExecutionScriptsIncluded(t *testing.T) {
	for _, relPath := range []string{
		filepath.Join("scripts", "run-e2e-tests.sh"),
		filepath.Join("scripts", "render-e2e-env.sh"),
		filepath.Join("scripts", "base_playwright_script.sh"),
		filepath.Join("test", "e2e", "playwright.base.config.ts"),
		filepath.Join("test", "e2e", "playwright.api-projects.ts"),
	} {
		t.Run(relPath, func(t *testing.T) {
			tmpDir := t.TempDir()
			chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
			if err := os.MkdirAll(chartDir, 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\n")

			scriptPath := filepath.Join(tmpDir, relPath)
			if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, scriptPath, "#!/usr/bin/env bash\n")

			hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
			if err != nil {
				t.Fatalf("first Compute: %v", err)
			}

			writeFile(t, scriptPath, "#!/usr/bin/env bash\n# changed\n")

			hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
			if err != nil {
				t.Fatalf("second Compute: %v", err)
			}

			if hash1 == hash2 {
				t.Errorf("expected hash to change when %s changes", relPath)
			}
		})
	}
}

func TestCompute_PlaywrightE2ETestsActionIncluded(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\n")

	hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	actionDir := filepath.Join(tmpDir, ".github", "actions", "playwright-e2e-tests")
	if err := os.MkdirAll(actionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(actionDir, "action.yaml"), "name: playwright-e2e-tests\n")

	hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change when playwright-e2e-tests/action.yaml is added")
	}
}

func TestCompute_MissingChartDirNoError(t *testing.T) {
	tmpDir := t.TempDir()

	// No chart directory exists — should not error, just hash nothing.
	hash1, err := Compute(tmpDir, []string{"8.99"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("Compute should not error for missing chart dir: %v", err)
	}

	if hash1 == "" {
		t.Error("expected a non-empty hash even when no files exist")
	}
}

func TestCompute_SkipsHiddenDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	hiddenDir := filepath.Join(chartDir, ".git")

	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\n")
	writeFile(t, filepath.Join(hiddenDir, "config"), "gitconfig\n")

	hash1, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	// Modify the hidden file — hash should NOT change.
	writeFile(t, filepath.Join(hiddenDir, "config"), "modified\n")

	hash2, err := Compute(tmpDir, []string{"8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if hash1 != hash2 {
		t.Error("expected hash to be unchanged when hidden directory files change")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompute_EveryChartVersionIncluded(t *testing.T) {
	tmpDir := t.TempDir()
	parentChartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.10")
	releaseChartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.9")
	for _, dir := range []string{parentChartDir, releaseChartDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "Chart.yaml"), "name: test\n")
	}

	hash1, err := Compute(tmpDir, []string{"8.10", "8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	writeFile(t, filepath.Join(releaseChartDir, "Chart.yaml"), "name: changed\n")

	hash2, err := Compute(tmpDir, []string{"8.10", "8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change when a non-parent chart version's directory changes")
	}
}

func TestCompute_ChartVersionOrderAndDuplicatesIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	for _, version := range []string{"8.10", "8.9"} {
		dir := filepath.Join(tmpDir, "charts", "camunda-platform-"+version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "Chart.yaml"), "version: "+version+"\n")
	}

	hash1, err := Compute(tmpDir, []string{"8.9", "8.10"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	hash2, err := Compute(tmpDir, []string{"8.10", "8.9", "8.9"}, testSuiteVersion, testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected chart version order and duplicates not to affect the hash, got %s and %s", hash1, hash2)
	}
}

func TestCompute_E2ESuiteVersionChangesHash(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "camunda-platform-8.10")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\n")

	hash1, err := Compute(tmpDir, []string{"8.10"}, "0.0.1250", testRunnerImage)
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	hash2, err := Compute(tmpDir, []string{"8.10"}, "0.0.1251", testRunnerImage)
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change when the e2e test suite version changes")
	}
}

func TestCompute_RequiresChartVersionsAndSuiteVersion(t *testing.T) {
	tmpDir := t.TempDir()

	if _, err := Compute(tmpDir, nil, testSuiteVersion, testRunnerImage); err == nil {
		t.Error("expected an error when no chart version is given")
	}
	if _, err := Compute(tmpDir, []string{""}, testSuiteVersion, testRunnerImage); err == nil {
		t.Error("expected an error when only empty chart versions are given")
	}
	if _, err := Compute(tmpDir, []string{"8.10"}, "", testRunnerImage); err == nil {
		t.Error("expected an error when no e2e test suite version is given")
	}
}

func TestChartVersionsFromCSV(t *testing.T) {
	got := ChartVersionsFromCSV(" 8.10, 8.9,,8.8 ")
	want := []string{"8.10", "8.9", "8.8"}

	if len(got) != len(want) {
		t.Fatalf("ChartVersionsFromCSV = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ChartVersionsFromCSV = %v, want %v", got, want)
		}
	}
}

package hash

import (
	"os"
	"path/filepath"
	"testing"
)

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
	hash1, err := Compute(tmpDir, "8.9")
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	hash2, err := Compute(tmpDir, "8.9")
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

	hash1, err := Compute(tmpDir, "8.9")
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	// Modify a file.
	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "name: test\nversion: 2.0.0\n")

	hash2, err := Compute(tmpDir, "8.9")
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

	hash1, err := Compute(tmpDir, "8.9")
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	// Add a new file.
	writeFile(t, filepath.Join(chartDir, "values.yaml"), "newKey: newValue\n")

	hash2, err := Compute(tmpDir, "8.9")
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

	hash89, err := Compute(tmpDir, "8.9")
	if err != nil {
		t.Fatalf("Compute 8.9: %v", err)
	}

	hash810, err := Compute(tmpDir, "8.10")
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

	hash1, err := Compute(tmpDir, "8.9")
	if err != nil {
		t.Fatalf("first Compute: %v", err)
	}

	// Add a workflow file.
	workflowDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(workflowDir, "test-integration-runner.yaml"), "name: runner\n")

	hash2, err := Compute(tmpDir, "8.9")
	if err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected hash to change when a workflow file is added")
	}
}

func TestCompute_DeployerAndCorePackagesIncluded(t *testing.T) {
	for _, pkg := range []string{
		filepath.Join("scripts", "camunda-deployer"),
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

			hash1, err := Compute(tmpDir, "8.9")
			if err != nil {
				t.Fatalf("first Compute: %v", err)
			}

			// Modify a file in the shared Go package.
			writeFile(t, filepath.Join(pkgDir, "helm.go"), "package deployer\n// changed\n")

			hash2, err := Compute(tmpDir, "8.9")
			if err != nil {
				t.Fatalf("second Compute: %v", err)
			}

			if hash1 == hash2 {
				t.Errorf("expected hash to change when a file under %s changes", pkg)
			}
		})
	}
}

func TestCompute_MissingChartDirNoError(t *testing.T) {
	tmpDir := t.TempDir()

	// No chart directory exists — should not error, just hash nothing.
	hash1, err := Compute(tmpDir, "8.99")
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

	hash1, err := Compute(tmpDir, "8.9")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	// Modify the hidden file — hash should NOT change.
	writeFile(t, filepath.Join(hiddenDir, "config"), "modified\n")

	hash2, err := Compute(tmpDir, "8.9")
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

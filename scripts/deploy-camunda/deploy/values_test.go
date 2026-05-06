package deploy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestProcessCommonValues_FindsSiblingCommonDir guards against a regression of
// the silent-skip bug that killed OpenSearch nightlies in May 2026.
//
// scenarios/chart-full-setup is the scenario; scenarios/common is its sibling.
// The previous implementation went one directory too high, computing the
// non-existent test/integration/common, which made os.Stat fail and silently
// drop every common values file (including the one that sets
// CAMUNDA_SYSTEM_UPGRADE_ENABLEVERSIONCHECK=false on orchestration). On
// SNAPSHOT installs that kept the orchestration StatefulSet NotReady and the
// ingress returned 502s.
func TestProcessCommonValues_FindsSiblingCommonDir(t *testing.T) {
	tmp := t.TempDir()

	// Mirror the real layout: <root>/scenarios/{chart-full-setup, common}/
	scenariosDir := filepath.Join(tmp, "scenarios")
	scenarioPath := filepath.Join(scenariosDir, "chart-full-setup")
	commonDir := filepath.Join(scenariosDir, "common")

	if err := os.MkdirAll(scenarioPath, 0o755); err != nil {
		t.Fatalf("setup scenario dir: %v", err)
	}
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatalf("setup common dir: %v", err)
	}

	// values-integration-test.yaml is in the predefined CommonValuesFiles list
	// (deployer.CommonValuesFiles); processCommonValues must pick it up.
	srcFile := filepath.Join(commonDir, "values-integration-test.yaml")
	const srcBody = `orchestration:
  env:
    - name: CAMUNDA_SYSTEM_UPGRADE_ENABLEVERSIONCHECK
      value: "false"
`
	if err := os.WriteFile(srcFile, []byte(srcBody), 0o644); err != nil {
		t.Fatalf("write source common values file: %v", err)
	}

	outputDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("setup output dir: %v", err)
	}

	files, err := processCommonValues(context.Background(), scenarioPath, outputDir, "", "", nil)
	if err != nil {
		t.Fatalf("processCommonValues returned error: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("expected at least one processed common values file, got 0")
	}

	found := false
	for _, f := range files {
		if filepath.Base(f) == "values-integration-test.yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected values-integration-test.yaml to be picked up; got %v", files)
	}
}

// TestProcessCommonValues_RealChartLayout exercises the fix against the actual
// chart layout in this repo, so a regression of the path-resolution bug shows
// up immediately on `go test ./deploy/...` instead of waiting for a nightly.
func TestProcessCommonValues_RealChartLayout(t *testing.T) {
	// Walk up from the test working directory to the repo root by looking for
	// a sentinel that's only at the repo root (charts/ + a known chart dir).
	// Test cwd is scripts/deploy-camunda/deploy, so the root is 3 levels up.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(repoRoot, "charts", "camunda-platform-8.9")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			break // reached filesystem root
		}
		repoRoot = parent
	}

	scenarioPath := filepath.Join(repoRoot, "charts", "camunda-platform-8.9",
		"test", "integration", "scenarios", "chart-full-setup")
	if _, err := os.Stat(scenarioPath); err != nil {
		t.Skipf("chart-full-setup scenario not found at %s — skipping", scenarioPath)
	}

	outputDir := t.TempDir()

	files, err := processCommonValues(context.Background(), scenarioPath, outputDir, "", "gke", nil)
	if err != nil {
		t.Fatalf("processCommonValues returned error: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("expected common values files for chart-full-setup; got 0 (path-resolution regression?)")
	}

	hasIntegrationTest := false
	for _, f := range files {
		base := filepath.Base(f)
		if base == "values-integration-test.yaml" {
			hasIntegrationTest = true
			break
		}
	}
	if !hasIntegrationTest {
		t.Fatalf("expected values-integration-test.yaml in processed common files, got %v", files)
	}
}

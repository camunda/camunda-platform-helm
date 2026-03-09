package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := fn()

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return strings.TrimSpace(string(buf[:n])), runErr
}

func TestRunPrepareValues_LegacyPath(t *testing.T) {
	scenarioDir := t.TempDir()
	outputDir := t.TempDir()

	writeTempFile(t, scenarioDir, "values-integration-test-ingress-gateway-keycloak.yaml", `global:
  image:
    tag: "8.10.0"
elasticsearch:
  enabled: true
`)

	pv := &prepareValuesFlags{
		scenarioPath: scenarioDir,
		scenario:     "gateway-keycloak",
		outputDir:    outputDir,
		logLevel:     "error",
	}

	stdout, err := captureStdout(t, func() error { return runPrepareValues(pv) })
	if err != nil {
		t.Fatalf("runPrepareValues (legacy) failed: %v", err)
	}

	if !strings.HasPrefix(stdout, outputDir) {
		t.Errorf("expected output path in %q, got %q", outputDir, stdout)
	}

	data, readErr := os.ReadFile(stdout)
	if readErr != nil {
		t.Fatalf("failed to read output file %q: %v", stdout, readErr)
	}
	if !strings.Contains(string(data), `tag: "8.10.0"`) {
		t.Errorf("output file missing expected content, got:\n%s", string(data))
	}
}

func TestRunPrepareValues_LayeredPath(t *testing.T) {
	scenarioDir := t.TempDir()
	outputDir := t.TempDir()

	writeTempFile(t, scenarioDir, "values/base.yaml", `global:
  image:
    tag: "8.9.0"
elasticsearch:
  enabled: true
`)

	pv := &prepareValuesFlags{
		scenarioPath: scenarioDir,
		scenario:     "chart-full-setup",
		identity:     "keycloak",
		persistence:  "elasticsearch",
		platform:     "gke",
		outputDir:    outputDir,
		logLevel:     "error",
	}

	stdout, err := captureStdout(t, func() error { return runPrepareValues(pv) })
	if err != nil {
		t.Fatalf("runPrepareValues (layered) failed: %v", err)
	}

	if stdout == "" {
		t.Fatal("expected output path on stdout, got empty")
	}

	if _, statErr := os.Stat(stdout); statErr != nil {
		t.Fatalf("output file %q does not exist: %v", stdout, statErr)
	}

	data, readErr := os.ReadFile(stdout)
	if readErr != nil {
		t.Fatalf("failed to read output file: %v", readErr)
	}
	if !strings.Contains(string(data), `tag: "8.9.0"`) {
		t.Errorf("output file missing expected content, got:\n%s", string(data))
	}
}

func TestRunPrepareValues_MissingScenarioPath(t *testing.T) {
	pv := &prepareValuesFlags{
		logLevel: "error",
	}

	err := runPrepareValues(pv)
	if err == nil {
		t.Fatal("expected error when no scenario path provided")
	}
	if !strings.Contains(err.Error(), "must be provided") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunPrepareValues_NonexistentScenarioDir(t *testing.T) {
	pv := &prepareValuesFlags{
		scenarioPath: "/nonexistent/path/that/does/not/exist",
		logLevel:     "error",
	}

	err := runPrepareValues(pv)
	if err == nil {
		t.Fatal("expected error for nonexistent scenario dir")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunPrepareValues_LegacyMissingFile(t *testing.T) {
	scenarioDir := t.TempDir()

	pv := &prepareValuesFlags{
		scenarioPath: scenarioDir,
		scenario:     "nonexistent-scenario",
		outputDir:    t.TempDir(),
		logLevel:     "error",
	}

	err := runPrepareValues(pv)
	if err == nil {
		t.Fatal("expected error for missing legacy file")
	}
	if !strings.Contains(err.Error(), "no layered values and no legacy values file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunPrepareValues_ChartPathDerivation(t *testing.T) {
	chartDir := t.TempDir()
	scenarioDir := filepath.Join(chartDir, "test", "integration", "scenarios", "chart-full-setup")

	writeTempFile(t, scenarioDir, "values-integration-test-ingress-chart-full-setup.yaml", `global:
  image:
    tag: "8.10.0"
`)

	outputDir := t.TempDir()

	pv := &prepareValuesFlags{
		chartPath: chartDir,
		scenario:  "chart-full-setup",
		outputDir: outputDir,
		logLevel:  "error",
	}

	stdout, err := captureStdout(t, func() error { return runPrepareValues(pv) })
	if err != nil {
		t.Fatalf("runPrepareValues (chart-path derivation) failed: %v", err)
	}

	if !strings.HasPrefix(stdout, outputDir) {
		t.Errorf("expected output in %q, got %q", outputDir, stdout)
	}
}

func TestRunPrepareValues_LegacyWithPlaceholders(t *testing.T) {
	scenarioDir := t.TempDir()
	outputDir := t.TempDir()

	writeTempFile(t, scenarioDir, "values-integration-test-ingress-my-scenario.yaml", `global:
  image:
    tag: "$MY_IMAGE_TAG"
`)

	t.Setenv("MY_IMAGE_TAG", "8.10.42")

	pv := &prepareValuesFlags{
		scenarioPath: scenarioDir,
		scenario:     "my-scenario",
		outputDir:    outputDir,
		logLevel:     "error",
	}

	stdout, err := captureStdout(t, func() error { return runPrepareValues(pv) })
	if err != nil {
		t.Fatalf("runPrepareValues (legacy with placeholders) failed: %v", err)
	}

	data, readErr := os.ReadFile(stdout)
	if readErr != nil {
		t.Fatalf("failed to read output: %v", readErr)
	}
	if !strings.Contains(string(data), "8.10.42") {
		t.Errorf("placeholder not substituted, got:\n%s", string(data))
	}
}

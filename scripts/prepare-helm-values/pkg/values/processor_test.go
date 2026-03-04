package values

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessImageTags_NoFileSpecified(t *testing.T) {
	opts := Options{
		ImageTagsFile: "",
	}

	output, err := ProcessImageTags(opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if output != "" {
		t.Fatalf("expected empty output, got: %s", output)
	}
}

func TestProcessImageTags_FileDoesNotExist(t *testing.T) {
	opts := Options{
		ImageTagsFile: "/nonexistent/path/values-image-tags.yaml",
	}

	output, err := ProcessImageTags(opts)
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got: %v", err)
	}
	if output != "" {
		t.Fatalf("expected empty output for non-existent file, got: %s", output)
	}
}

func TestProcessImageTags_SubstitutesPlaceholders(t *testing.T) {
	// Create temp file with placeholders
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values-image-tags.yaml")

	content := `orchestration:
  image:
    tag: "$E2E_TESTS_ORCHESTRATION_IMAGE_TAG"
console:
  image:
    tag: "$E2E_TESTS_CONSOLE_IMAGE_TAG"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Set values via valuesConfig JSON
	opts := Options{
		ImageTagsFile: inputFile,
		ValuesConfig:  `{"E2E_TESTS_ORCHESTRATION_IMAGE_TAG": "8.9.0-test", "E2E_TESTS_CONSOLE_IMAGE_TAG": "console-1.0"}`,
	}

	output, err := ProcessImageTags(opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedOutput := "/tmp/values-image-tags-processed.yaml"
	if output != expectedOutput {
		t.Fatalf("expected output path %s, got: %s", expectedOutput, output)
	}

	// Read the output file and verify substitution
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	resultStr := string(result)
	if !contains(resultStr, "tag: \"8.9.0-test\"") {
		t.Errorf("expected orchestration tag to be substituted, got: %s", resultStr)
	}
	if !contains(resultStr, "tag: \"console-1.0\"") {
		t.Errorf("expected console tag to be substituted, got: %s", resultStr)
	}

	// Cleanup
	os.Remove(output)
}

func TestProcessImageTags_PreservesUnmatchedPlaceholders(t *testing.T) {
	// Create temp file with placeholders
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values-image-tags.yaml")

	content := `orchestration:
  image:
    tag: "$E2E_TESTS_ORCHESTRATION_IMAGE_TAG"
console:
  image:
    tag: "$E2E_TESTS_CONSOLE_IMAGE_TAG"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Only provide one of the values
	opts := Options{
		ImageTagsFile: inputFile,
		ValuesConfig:  `{"E2E_TESTS_ORCHESTRATION_IMAGE_TAG": "8.9.0-test"}`,
	}

	output, err := ProcessImageTags(opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Read the output file and verify
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	resultStr := string(result)
	if !contains(resultStr, "tag: \"8.9.0-test\"") {
		t.Errorf("expected orchestration tag to be substituted, got: %s", resultStr)
	}
	// Unmatched placeholder should be preserved
	if !contains(resultStr, "$E2E_TESTS_CONSOLE_IMAGE_TAG") {
		t.Errorf("expected console tag placeholder to be preserved, got: %s", resultStr)
	}

	// Cleanup
	os.Remove(output)
}

func TestProcessImageTags_UsesEnvVars(t *testing.T) {
	// Create temp file with placeholders
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values-image-tags.yaml")

	content := `identity:
  image:
    tag: "$E2E_TESTS_IDENTITY_IMAGE_TAG"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Set env var
	os.Setenv("E2E_TESTS_IDENTITY_IMAGE_TAG", "identity-env-value")
	defer os.Unsetenv("E2E_TESTS_IDENTITY_IMAGE_TAG")

	opts := Options{
		ImageTagsFile: inputFile,
		ValuesConfig:  "{}",
	}

	output, err := ProcessImageTags(opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Read the output file and verify
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	resultStr := string(result)
	if !contains(resultStr, "tag: \"identity-env-value\"") {
		t.Errorf("expected identity tag from env var, got: %s", resultStr)
	}

	// Cleanup
	os.Remove(output)
}

func TestProcessImageTags_ValuesConfigOverridesEnv(t *testing.T) {
	// Create temp file with placeholders
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values-image-tags.yaml")

	content := `connectors:
  image:
    tag: "$E2E_TESTS_CONNECTORS_IMAGE_TAG"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Set env var
	os.Setenv("E2E_TESTS_CONNECTORS_IMAGE_TAG", "env-value")
	defer os.Unsetenv("E2E_TESTS_CONNECTORS_IMAGE_TAG")

	// valuesConfig should take precedence
	opts := Options{
		ImageTagsFile: inputFile,
		ValuesConfig:  `{"E2E_TESTS_CONNECTORS_IMAGE_TAG": "config-value"}`,
	}

	output, err := ProcessImageTags(opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Read the output file and verify
	result, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	resultStr := string(result)
	if !contains(resultStr, "tag: \"config-value\"") {
		t.Errorf("expected config value to override env, got: %s", resultStr)
	}

	// Cleanup
	os.Remove(output)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Tests for values.Process with EnvOverrides
// ---------------------------------------------------------------------------

// TestProcess_EnvOverridesIsolation verifies that when EnvOverrides is set,
// the process environment is NOT consulted for placeholder substitution.
func TestProcess_EnvOverridesIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values.yaml")

	content := `host: "$MY_HOST"
port: "$MY_PORT"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Set process env — these should NOT be used when EnvOverrides is provided.
	os.Setenv("MY_HOST", "process-env-host")
	os.Setenv("MY_PORT", "9999")
	defer os.Unsetenv("MY_HOST")
	defer os.Unsetenv("MY_PORT")

	outputDir := t.TempDir()
	opts := Options{
		OutputDir: outputDir,
		EnvOverrides: map[string]string{
			"MY_HOST": "override-host",
			"MY_PORT": "8080",
		},
	}

	outputPath, resultContent, err := Process(inputFile, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify values came from EnvOverrides, not process env.
	if !contains(resultContent, "host: \"override-host\"") {
		t.Errorf("expected MY_HOST from EnvOverrides, got: %s", resultContent)
	}
	if !contains(resultContent, "port: \"8080\"") {
		t.Errorf("expected MY_PORT from EnvOverrides, got: %s", resultContent)
	}

	// Verify file was written.
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(written) != resultContent {
		t.Errorf("written content doesn't match returned content")
	}
}

// TestProcess_EnvOverridesUnsetVarIsMissing verifies that a placeholder
// not present in EnvOverrides is treated as missing (even if it exists
// in the process environment).
func TestProcess_EnvOverridesUnsetVarIsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values.yaml")

	content := `secret: "$MY_SECRET"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// MY_SECRET exists in process env but not in EnvOverrides.
	os.Setenv("MY_SECRET", "from-process")
	defer os.Unsetenv("MY_SECRET")

	opts := Options{
		OutputDir:    t.TempDir(),
		EnvOverrides: map[string]string{}, // empty — no fallback to os.LookupEnv
	}

	_, _, err := Process(inputFile, opts)
	if err == nil {
		t.Fatal("expected MissingEnvError when placeholder not in EnvOverrides, got nil")
	}
	isMissing, names := IsMissingEnv(err)
	if !isMissing {
		t.Fatalf("expected MissingEnvError, got: %T: %v", err, err)
	}
	if len(names) != 1 || names[0] != "MY_SECRET" {
		t.Errorf("expected missing [MY_SECRET], got: %v", names)
	}
}

// TestProcess_ConfigEnvOverridesEnvOverrides verifies precedence:
// configEnv (from ValuesConfig JSON) > EnvOverrides > (os.LookupEnv skipped).
func TestProcess_ConfigEnvOverridesEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values.yaml")

	content := `tag: "$IMAGE_TAG"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	opts := Options{
		OutputDir:    t.TempDir(),
		ValuesConfig: `{"IMAGE_TAG": "from-config"}`,
		EnvOverrides: map[string]string{
			"IMAGE_TAG": "from-override",
		},
	}

	_, resultContent, err := Process(inputFile, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// configEnv (ValuesConfig) should win over EnvOverrides.
	if !contains(resultContent, "tag: \"from-config\"") {
		t.Errorf("expected configEnv to take precedence, got: %s", resultContent)
	}
}

// TestProcess_NilEnvOverridesFallsBackToProcessEnv verifies backward
// compatibility: when EnvOverrides is nil (the default), os.LookupEnv
// is used as before.
func TestProcess_NilEnvOverridesFallsBackToProcessEnv(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "values.yaml")

	content := `url: "$BACKWARD_COMPAT_URL"
`
	if err := os.WriteFile(inputFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	os.Setenv("BACKWARD_COMPAT_URL", "https://example.com")
	defer os.Unsetenv("BACKWARD_COMPAT_URL")

	opts := Options{
		OutputDir:    t.TempDir(),
		EnvOverrides: nil, // explicitly nil — should fall back to os.LookupEnv
	}

	_, resultContent, err := Process(inputFile, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !contains(resultContent, "url: \"https://example.com\"") {
		t.Errorf("expected process env value, got: %s", resultContent)
	}
}

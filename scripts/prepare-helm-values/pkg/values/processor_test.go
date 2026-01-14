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


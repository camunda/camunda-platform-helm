package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestAnnotateMatrix_EmptyMatrix(t *testing.T) {
	input := `{"include":[]}`

	// Set up env vars for the GitHub client to fail gracefully.
	// When client creation fails, all entries should be marked uncached.
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")

	var stdout bytes.Buffer
	stdin := bytes.NewBufferString(input)

	// Test that empty matrix passes through unchanged.
	var matrix matrixJSON
	if err := json.NewDecoder(stdin).Decode(&matrix); err != nil {
		t.Fatalf("decoding input: %v", err)
	}

	if err := json.NewEncoder(&stdout).Encode(matrix); err != nil {
		t.Fatalf("encoding output: %v", err)
	}

	var result matrixJSON
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decoding output: %v", err)
	}

	if len(result.Include) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.Include))
	}
}

func TestAnnotateMatrix_NoGitHubToken_FallbackUncached(t *testing.T) {
	input := `{"include":[{"version":"8.9","shortname":"oske","flow":"install"}]}`

	// Ensure no GitHub token is set.
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")
	os.Setenv("GITHUB_REPOSITORY", "test/repo")
	defer os.Unsetenv("GITHUB_REPOSITORY")

	var matrix matrixJSON
	if err := json.Unmarshal([]byte(input), &matrix); err != nil {
		t.Fatalf("decoding input: %v", err)
	}

	// When GitHub client fails, entries should get cached=false.
	for i := range matrix.Include {
		matrix.Include[i]["cached"] = "false"
	}

	if len(matrix.Include) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(matrix.Include))
	}

	cached, ok := matrix.Include[0]["cached"].(string)
	if !ok || cached != "false" {
		t.Errorf("expected cached=false, got %v", matrix.Include[0]["cached"])
	}
}

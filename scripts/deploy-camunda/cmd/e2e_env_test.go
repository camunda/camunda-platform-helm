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
	"strings"
	"testing"
)

func TestMergeEnvOverridesReplacesExistingKey(t *testing.T) {
	content := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\nKEYCLOAK_URL=https://orcha.example.com\n"
	overrides := map[string]string{
		"KEYCLOAK_URL": "https://mgmt.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\nKEYCLOAK_URL=https://mgmt.example.com\n"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

func TestDecodeSecretValueRoundTrip(t *testing.T) {
	// "s3cr3t" base64 == "czNjcjN0", with surrounding whitespace kubectl may emit.
	got, err := decodeSecretValue("  czNjcjN0\n")
	if err != nil {
		t.Fatalf("decodeSecretValue() unexpected error: %v", err)
	}
	if got != "s3cr3t" {
		t.Fatalf("decodeSecretValue() = %q, want %q", got, "s3cr3t")
	}
}

func TestDecodeSecretValueEmptyStringSucceeds(t *testing.T) {
	got, err := decodeSecretValue("")
	if err != nil {
		t.Fatalf("decodeSecretValue() unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("decodeSecretValue() = %q, want empty string", got)
	}
}

func TestDecodeSecretValueRejectsInvalidBase64(t *testing.T) {
	if _, err := decodeSecretValue("not!base64!"); err == nil {
		t.Fatal("decodeSecretValue() expected error on invalid base64, got nil")
	}
}

func TestMergeEnvOverridesAppendsMissingKeysSorted(t *testing.T) {
	content := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\n"
	overrides := map[string]string{
		"OAUTH_URL":           "https://mgmt.example.com/token",
		"MANAGEMENT_BASE_URL": "https://mgmt.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\nMANAGEMENT_BASE_URL=https://mgmt.example.com\nOAUTH_URL=https://mgmt.example.com/token\n"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

func TestMergeEnvOverridesPreservesNoTrailingNewline(t *testing.T) {
	content := "PLAYWRIGHT_BASE_URL=https://orcha.example.com"
	overrides := map[string]string{
		"PLAYWRIGHT_BASE_URL": "https://mgmt.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "PLAYWRIGHT_BASE_URL=https://mgmt.example.com"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

func TestE2EEnvMergeFailsOnMissingRenderScript(t *testing.T) {
	cmd := newE2EEnvMergeCommand()
	cmd.SetArgs([]string{
		"--orchestration-namespace", "matrix-810-mns-orcha",
		"--management-namespace", "matrix-810-mns-mgmt",
		"--absolute-chart-path", "/workspace/charts/camunda-platform-8.10",
		"--render-script", "/nonexistent/render-e2e-env.sh",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --render-script points at a non-existent path")
	}
	if !strings.Contains(err.Error(), "render script failed") {
		t.Fatalf("expected error to mention render script failure, got: %v", err)
	}
}

func TestMergeEnvOverridesIgnoresLinesWithoutEquals(t *testing.T) {
	content := "# a comment\n\nPLAYWRIGHT_BASE_URL=https://orcha.example.com\n"
	overrides := map[string]string{
		"PLAYWRIGHT_BASE_URL": "https://mgmt.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "# a comment\n\nPLAYWRIGHT_BASE_URL=https://mgmt.example.com\n"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

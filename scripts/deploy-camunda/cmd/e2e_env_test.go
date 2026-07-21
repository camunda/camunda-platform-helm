package cmd

import "testing"

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

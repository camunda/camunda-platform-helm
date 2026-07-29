package matrix

import (
	"os"
	"path/filepath"
	"testing"
)

func postgresEntry(chartPath, version string) Entry {
	return Entry{
		Version: version, ChartPath: chartPath, Scenario: "test", Shortname: version, Flow: "install",
		Dependencies: []ChartDependency{{EnvVars: []string{"RDBMS_POSTGRESQL_USERNAME", "RDBMS_POSTGRESQL_PASSWORD"}}},
	}
}

func TestResolvePostgresCredentialsUsesEffectiveValues(t *testing.T) {
	chartPath := t.TempDir()
	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("RDBMS_POSTGRESQL_USERNAME=explicit\nRDBMS_POSTGRESQL_PASSWORD=explicit-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	username, password, err := ResolvePostgresCredentials([]Entry{postgresEntry(chartPath, "8.10")}, RunOptions{
		GeneratePostgresCredentials: true, EnvFile: envFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if username != "explicit" || password != "explicit-password" {
		t.Fatalf("credentials = %q/%q", username, password)
	}
}

func TestResolvePostgresCredentialsRejectsPerVersionMismatch(t *testing.T) {
	chartPath := t.TempDir()
	first := filepath.Join(t.TempDir(), "first.env")
	second := filepath.Join(t.TempDir(), "second.env")
	if err := os.WriteFile(first, []byte("RDBMS_POSTGRESQL_USERNAME=camunda\nRDBMS_POSTGRESQL_PASSWORD=first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("RDBMS_POSTGRESQL_USERNAME=camunda\nRDBMS_POSTGRESQL_PASSWORD=second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := ResolvePostgresCredentials([]Entry{postgresEntry(chartPath, "8.9"), postgresEntry(chartPath, "8.10")}, RunOptions{
		GeneratePostgresCredentials: true, EnvFiles: map[string]string{"8.9": first, "8.10": second},
	})
	if err == nil {
		t.Fatal("expected inconsistent password failure")
	}
}

func TestResolvePostgresCredentialsRejectsPartialPair(t *testing.T) {
	chartPath := t.TempDir()
	envFile := filepath.Join(t.TempDir(), "partial.env")
	if err := os.WriteFile(envFile, []byte("RDBMS_POSTGRESQL_USERNAME=camunda\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := ResolvePostgresCredentials([]Entry{postgresEntry(chartPath, "8.10")}, RunOptions{GeneratePostgresCredentials: true, EnvFile: envFile})
	if err == nil {
		t.Fatal("expected partial credential pair failure")
	}
}

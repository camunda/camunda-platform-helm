package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
)

func TestProcessCompanionChartsSubstitutesValuesFilePlaceholders(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "postgresql.yaml")
	if err := os.WriteFile(sourceFile, []byte("auth:\n  username: \"$RDBMS_POSTGRESQL_USERNAME\"\n  password: \"$RDBMS_POSTGRESQL_PASSWORD\"\n"), 0o644); err != nil {
		t.Fatalf("write source values: %v", err)
	}

	charts := []config.CompanionChart{{
		ChartRef:    "charts/internal-postgresql",
		ReleaseName: "postgresql",
		ValuesFile:  sourceFile,
	}}

	processed, err := processCompanionCharts(context.Background(), charts, tmpDir, "", map[string]string{
		"RDBMS_POSTGRESQL_USERNAME": "app",
		"RDBMS_POSTGRESQL_PASSWORD": "ci-secret",
	})
	if err != nil {
		t.Fatalf("processCompanionCharts returned error: %v", err)
	}

	if len(processed) != 1 {
		t.Fatalf("processed charts length = %d, want 1", len(processed))
	}
	if processed[0].ValuesFile == sourceFile {
		t.Fatalf("processed values file should be written to a temp path, got source path %q", sourceFile)
	}

	content, err := os.ReadFile(processed[0].ValuesFile)
	if err != nil {
		t.Fatalf("read processed values: %v", err)
	}
	got := string(content)
	for _, want := range []string{"username: \"app\"", "password: \"ci-secret\""} {
		if !strings.Contains(got, want) {
			t.Errorf("processed values missing %q:\n%s", want, got)
		}
	}

	original, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("read original values: %v", err)
	}
	if !strings.Contains(string(original), "$RDBMS_POSTGRESQL_PASSWORD") {
		t.Fatalf("source values file was modified:\n%s", original)
	}
}

func TestProcessCompanionChartsKeepsChartsWithoutValuesFile(t *testing.T) {
	charts := []config.CompanionChart{{
		ChartRef:    "opensearch/opensearch",
		ReleaseName: "opensearch",
	}}

	processed, err := processCompanionCharts(context.Background(), charts, t.TempDir(), "", nil)
	if err != nil {
		t.Fatalf("processCompanionCharts returned error: %v", err)
	}
	if len(processed) != 1 {
		t.Fatalf("processed charts length = %d, want 1", len(processed))
	}
	if processed[0].ValuesFile != "" {
		t.Fatalf("ValuesFile = %q, want empty", processed[0].ValuesFile)
	}
}

func TestProcessCompanionChartsSkipsUnrelatedShellVariables(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "elasticsearch.yaml")
	if err := os.WriteFile(sourceFile, []byte("extraInitContainers:\n  - command:\n      - sh\n      - -c\n      - |\n        echo \"waiting ($n/$max)\"\n"), 0o644); err != nil {
		t.Fatalf("write source values: %v", err)
	}

	charts := []config.CompanionChart{{
		ChartRef:    "elasticsearch/elasticsearch",
		ReleaseName: "elasticsearch",
		ValuesFile:  sourceFile,
	}}

	processed, err := processCompanionCharts(context.Background(), charts, tmpDir, "", nil)
	if err != nil {
		t.Fatalf("processCompanionCharts returned error: %v", err)
	}
	if processed[0].ValuesFile != sourceFile {
		t.Fatalf("ValuesFile = %q, want original source path %q", processed[0].ValuesFile, sourceFile)
	}
}

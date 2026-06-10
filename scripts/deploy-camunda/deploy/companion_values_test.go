package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
)

func TestProcessCompanionChartsSubstitutesAllowlistedVars(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "companion.yaml")
	// A non-PostgreSQL variable name proves substitution is generic and not
	// tied to the old $RDBMS_POSTGRESQL_ prefix. Both $VAR and ${VAR} forms.
	if err := os.WriteFile(sourceFile, []byte("auth:\n  token: \"$MY_CUSTOM_TOKEN\"\n  alt: \"${MY_CUSTOM_TOKEN}\"\n"), 0o644); err != nil {
		t.Fatalf("write source values: %v", err)
	}

	charts := []config.CompanionChart{{
		ChartRef:    "charts/internal-postgresql",
		ReleaseName: "postgresql",
		ValuesFile:  sourceFile,
		EnvVars:     []string{"MY_CUSTOM_TOKEN"},
	}}

	processed, err := processCompanionCharts(context.Background(), charts, tmpDir, "", map[string]string{
		"MY_CUSTOM_TOKEN": "secret123",
	})
	if err != nil {
		t.Fatalf("processCompanionCharts returned error: %v", err)
	}

	if len(processed) != 1 {
		t.Fatalf("processed charts length = %d, want 1", len(processed))
	}
	if processed[0].ValuesFile == sourceFile {
		t.Fatalf("processed values file should be written to a separate path, got source path %q", sourceFile)
	}

	content, err := os.ReadFile(processed[0].ValuesFile)
	if err != nil {
		t.Fatalf("read processed values: %v", err)
	}
	got := string(content)
	for _, want := range []string{"token: \"secret123\"", "alt: \"secret123\""} {
		if !strings.Contains(got, want) {
			t.Errorf("processed values missing %q:\n%s", want, got)
		}
	}

	original, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("read original values: %v", err)
	}
	if !strings.Contains(string(original), "$MY_CUSTOM_TOKEN") {
		t.Fatalf("source values file was modified:\n%s", original)
	}
}

func TestProcessCompanionChartsMissingVarErrors(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "postgresql.yaml")
	if err := os.WriteFile(sourceFile, []byte("auth:\n  username: \"$RDBMS_POSTGRESQL_USERNAME\"\n"), 0o644); err != nil {
		t.Fatalf("write source values: %v", err)
	}

	charts := []config.CompanionChart{{
		ChartRef:    "charts/internal-postgresql",
		ReleaseName: "postgresql",
		ValuesFile:  sourceFile,
		EnvVars:     []string{"RDBMS_POSTGRESQL_USERNAME", "RDBMS_POSTGRESQL_PASSWORD"},
	}}

	// Only USERNAME is present; PASSWORD is declared but missing from the env map.
	_, err := processCompanionCharts(context.Background(), charts, tmpDir, "", map[string]string{
		"RDBMS_POSTGRESQL_USERNAME": "app",
	})
	if err == nil {
		t.Fatal("expected error for missing declared variable, got nil")
	}
	if !strings.Contains(err.Error(), "RDBMS_POSTGRESQL_PASSWORD") {
		t.Errorf("error should name the missing variable, got: %v", err)
	}
}

func TestProcessCompanionChartsKeepsChartsWithoutValuesFile(t *testing.T) {
	charts := []config.CompanionChart{{
		ChartRef:    "opensearch/opensearch",
		ReleaseName: "opensearch",
		EnvVars:     []string{"SOME_VAR"},
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

func TestProcessCompanionChartsPassesThroughWithoutEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "elasticsearch.yaml")
	// Contains shell variables; with no EnvVars allowlist the file must be used
	// verbatim (no processing, no copy).
	if err := os.WriteFile(sourceFile, []byte("extraInitContainers:\n  - command:\n      - sh\n      - -c\n      - |\n        echo \"waiting ($n/$max)\"\n"), 0o644); err != nil {
		t.Fatalf("write source values: %v", err)
	}

	charts := []config.CompanionChart{{
		ChartRef:    "elastic/elasticsearch",
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

func TestSubstituteCompanionEnvVarsLeavesUndeclaredTokensIntact(t *testing.T) {
	content := strings.Join([]string{
		`echo "waiting ($n/$max)"`,
		`user: "$RDBMS_POSTGRESQL_USERNAME"`,
		`alt: "${RDBMS_POSTGRESQL_USERNAME}"`,
		`extra: "$RDBMS_POSTGRESQL_USERNAME_SUFFIX"`,
	}, "\n")

	got, err := substituteCompanionEnvVars(content, []string{"RDBMS_POSTGRESQL_USERNAME"}, map[string]string{
		"RDBMS_POSTGRESQL_USERNAME": "app",
	})
	if err != nil {
		t.Fatalf("substituteCompanionEnvVars returned error: %v", err)
	}

	// Declared variable is substituted in both forms.
	for _, want := range []string{`user: "app"`, `alt: "app"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing substituted %q:\n%s", want, got)
		}
	}
	// Undeclared shell variables are left intact.
	if !strings.Contains(got, "($n/$max)") {
		t.Errorf("shell vars $n/$max were modified:\n%s", got)
	}
	// A longer name with the allowlisted name as a prefix is NOT partially replaced.
	if !strings.Contains(got, `extra: "$RDBMS_POSTGRESQL_USERNAME_SUFFIX"`) {
		t.Errorf("prefix-shadowed token was wrongly substituted:\n%s", got)
	}
}

func TestSubstituteCompanionEnvVarsMissingVar(t *testing.T) {
	_, err := substituteCompanionEnvVars(`token: "$A_TOKEN"`, []string{"A_TOKEN", "B_TOKEN"}, map[string]string{
		"A_TOKEN": "x",
	})
	if err == nil {
		t.Fatal("expected error for missing variable, got nil")
	}
	if !strings.Contains(err.Error(), "B_TOKEN") {
		t.Errorf("error should name missing B_TOKEN, got: %v", err)
	}
}

func TestProcessCompanionChartsSharedBasenameNoCollision(t *testing.T) {
	tmpDir := t.TempDir()
	// Cross-product collision: release "alpha-beta" + "x.yaml" and release
	// "alpha" + "beta-x.yaml" both flatten to "alpha-beta-x.yaml" under a
	// release-name-only prefix. The loop-index prefix must keep them distinct.
	dirA := filepath.Join(tmpDir, "a")
	dirB := filepath.Join(tmpDir, "b")
	for _, d := range []string{dirA, dirB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	fileA := filepath.Join(dirA, "x.yaml")
	fileB := filepath.Join(dirB, "beta-x.yaml")
	if err := os.WriteFile(fileA, []byte("token: \"$TOK\"\n"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("token: \"$TOK\"\n"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	charts := []config.CompanionChart{
		{ChartRef: "chart-a", ReleaseName: "alpha-beta", ValuesFile: fileA, EnvVars: []string{"TOK"}},
		{ChartRef: "chart-b", ReleaseName: "alpha", ValuesFile: fileB, EnvVars: []string{"TOK"}},
	}
	processed, err := processCompanionCharts(context.Background(), charts, tmpDir, "", map[string]string{
		"TOK": "v1",
	})
	if err != nil {
		t.Fatalf("processCompanionCharts returned error: %v", err)
	}
	if processed[0].ValuesFile == processed[1].ValuesFile {
		t.Fatalf("processed paths collided: %q", processed[0].ValuesFile)
	}
	for _, p := range processed {
		content, err := os.ReadFile(p.ValuesFile)
		if err != nil {
			t.Fatalf("read %q: %v", p.ValuesFile, err)
		}
		if !strings.Contains(string(content), `token: "v1"`) {
			t.Errorf("%q not substituted:\n%s", p.ValuesFile, content)
		}
	}
}

func TestSubstituteCompanionEnvVarsEmptyAllowlistNoOp(t *testing.T) {
	content := `echo "($n/$max)"`
	got, err := substituteCompanionEnvVars(content, nil, nil)
	if err != nil {
		t.Fatalf("substituteCompanionEnvVars returned error: %v", err)
	}
	if got != content {
		t.Errorf("empty allowlist should be a no-op, got: %s", got)
	}
}

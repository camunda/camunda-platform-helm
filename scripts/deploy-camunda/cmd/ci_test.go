package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pins the flag→struct wiring of "ci test-type-vars": string flags compared
// against "true" (composite-action argument shape), the env passthroughs, and
// the root PersistentPreRunE skip (ci needs no chart/namespace config). The
// computation itself is covered in camunda-core/pkg/ciworkflow.
func TestCITestTypeVarsWiring(t *testing.T) {
	repo := t.TempDir()
	chartDir := "camunda-platform-8.10"
	writeFile(t, filepath.Join(repo, "charts", chartDir, "test", "ci-test-config.yaml"),
		"unit:\n  enabled: true\n  matrix:\n    - name: core\n      packages: test/unit/camunda\n")
	writeFile(t, filepath.Join(repo, "charts", chartDir, "values-digest.yaml"), "{}\n")

	envFile := filepath.Join(t.TempDir(), "github_env")
	outFile := filepath.Join(t.TempDir(), "github_output")
	t.Setenv("GITHUB_ENV", envFile)
	t.Setenv("GITHUB_OUTPUT", outFile)
	t.Setenv("GITHUB_WORKSPACE", "/workspace")
	t.Setenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", "s3cret")
	t.Chdir(repo)

	root := NewRootCommand()
	root.AddCommand(newCICommand())
	root.SetArgs([]string{
		"ci", "test-type-vars",
		"--chart-dir", chartDir,
		"--flow", "install",
		"--upgrade-step", "false",
		"--values-enterprise", "true",
		"--values-digest", "true",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	env := readFile(t, envFile)
	for _, want := range []string{
		"CHART_PATH=charts/camunda-platform-8.10\n",
		"ABSOLUTE_TEST_CHART_DIR=/workspace/charts/camunda-platform-8.10\n",
		"TEST_HELM_EXTRA_ARGS=--values ../../../../charts/camunda-platform-8.10/values-enterprise.yaml\n",
		"TEST_HELM_DIGEST_VALUES=../../../../charts/camunda-platform-8.10/values-digest.yaml\n",
		"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET=s3cret\n",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("GITHUB_ENV missing %q; got:\n%s", want, env)
		}
	}

	out := readFile(t, outFile)
	for _, want := range []string{
		"unit-enabled=true\n",
		`unit-matrix=[{"name":"core","packages":"test/unit/camunda"}]` + "\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GITHUB_OUTPUT missing %q; got:\n%s", want, out)
		}
	}
}

func TestCITestTypeVarsRequiresChartDir(t *testing.T) {
	cmd := newCITestTypeVarsCommand()
	cmd.SetArgs([]string{"--flow", "install"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --chart-dir is missing")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

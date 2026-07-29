package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkflowVarsWiring(t *testing.T) {
	repo := t.TempDir()
	chartDir := "camunda-platform-8.10"
	writeFile(t, filepath.Join(repo, ".github", "config", "infra.yaml"), `gke:
  ingress-hostname-base: ci.example.com
  namespace-prefix: camunda
eks:
  ingress-hostname-base: ci.aws.example.com
  namespace-prefix: distribution
  cluster-name: camunda-ci-eks
  aws-profile: distribution
postgresql:
  jdbc-host: postgresql.example.com
  jdbc-port: "5432"
teleport:
  proxy: teleport.example.com:443
`)
	writeFile(t, filepath.Join(repo, "charts", "camunda-platform-8.9", "Chart.yaml"), "version: 14.0.0\n")
	chartPath := filepath.Join(repo, "charts", chartDir, "Chart.yaml")
	writeFile(t, chartPath, "apiVersion: v2\nname: camunda-platform\nversion: 15.0.0\n")

	envFile := filepath.Join(t.TempDir(), "github_env")
	outFile := filepath.Join(t.TempDir(), "github_output")
	t.Setenv("GITHUB_ENV", envFile)
	t.Setenv("GITHUB_OUTPUT", outFile)
	t.Setenv("FLOW", "install")
	t.Chdir(repo)

	root := NewRootCommand()
	root.AddCommand(newCICommand())
	root.SetArgs([]string{
		"ci", "workflow-vars",
		"--platform", "GKE",
		"--setup-flow", "install",
		"--deployment-ttl", "1h",
		"--identifier-base", "6598",
		"--chart-dir", chartDir,
		"--pr-number", "1234",
		"--run-id", "16400000001",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	env := readFile(t, envFile)
	for _, want := range []string{
		"INFRA_INGRESS_HOSTNAME_BASE=ci.example.com\n",
		"INFRA_CLUSTER_NAME=camunda-ci-eks\n",
		"POSTGRESQL_JDBC_URL=jdbc:postgresql://postgresql.example.com:5432\n",
		"PLATFORM=gke\n",
		"GITHUB_WORKFLOW_JOB_ID=b73169\n",
		"GITHUB_WORKFLOW_RUN_ID=16400000001\n",
		"TEST_NAMESPACE=camunda-pr-6598\n",
		"TEST_CAMUNDA_HELM_DIR_ALPHA=camunda-platform-8.10\n",
		"FLOW=install\n",
		"KEYCLOAK_REALM=b73169-realm\n",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("GITHUB_ENV missing %q; got:\n%s", want, env)
		}
	}

	out := readFile(t, outFile)
	for _, want := range []string{
		"namespace=camunda-pr-6598\n",
		"identifier=gke-6598\n",
		"ingress-host=gke-6598.ci.example.com\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GITHUB_OUTPUT missing %q; got:\n%s", want, out)
		}
	}

	chart := readFile(t, chartPath)
	if !strings.Contains(chart, "version: 0.0.0-ci-snapshot-8.10\n") {
		t.Errorf("Chart.yaml version was not stamped:\n%s", chart)
	}
}

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

func TestCIWorkflowVarsRequiresRunID(t *testing.T) {
	cmd := newCIWorkflowVarsCommand()
	cmd.SetArgs([]string{"--platform", "gke", "--chart-dir", "camunda-platform-8.10"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), `required flag(s) "run-id" not set`) {
		t.Fatalf("error = %v, want missing run-id error", err)
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

package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
)

func TestPresence(t *testing.T) {
	envMap := map[string]string{"SET": "v", "EMPTY": ""}
	present, missing := presence(envMap, []string{"SET", "EMPTY", "ABSENT"})
	if len(present) != 1 || present[0] != "SET" {
		t.Errorf("present = %v, want [SET]", present)
	}
	if len(missing) != 2 || missing[0] != "EMPTY" || missing[1] != "ABSENT" {
		t.Errorf("missing = %v, want [EMPTY ABSENT]", missing)
	}
}

func TestFirstNonEmptyEnv(t *testing.T) {
	envMap := map[string]string{"B": "fromB", "C": "fromC"}
	if got := firstNonEmptyEnv(envMap, "flag", "A", "B"); got != "flag" {
		t.Errorf("flag should win, got %q", got)
	}
	if got := firstNonEmptyEnv(envMap, "", "A", "B", "C"); got != "fromB" {
		t.Errorf("first non-empty env should win, got %q", got)
	}
	if got := firstNonEmptyEnv(envMap, "", "A", "Z"); got != "" {
		t.Errorf("none set should be empty, got %q", got)
	}
}

func TestCheckVaultMapping(t *testing.T) {
	tests := []struct {
		name    string
		mapping string
		envMap  map[string]string
		want    CheckStatus
		missing int
	}{
		{"no mapping", "", nil, StatusOK, 0},
		{"all set", "ci/path A;ci/path B;", map[string]string{"A": "1", "B": "2"}, StatusOK, 0},
		{"some missing", "ci/path A;ci/path B;", map[string]string{"A": "1"}, StatusFail, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &config.RuntimeFlags{}
			flags.Secrets.VaultSecretMapping = tt.mapping
			c := checkVaultMapping(flags, tt.envMap)
			if c.Status != tt.want {
				t.Errorf("status = %q, want %q (detail: %s)", c.Status, tt.want, c.Detail)
			}
			if len(c.Missing) != tt.missing {
				t.Errorf("missing = %v, want %d entries", c.Missing, tt.missing)
			}
		})
	}
}

func TestCheckDockerCredentials(t *testing.T) {
	// Not required → warn; required + missing → fail; present → ok.
	cases := []struct {
		name     string
		flags    config.DockerFlags
		envMap   map[string]string
		wantHead CheckStatus // status of the Harbor check (always first)
	}{
		{"absent not required", config.DockerFlags{}, nil, StatusWarn},
		{"absent but required", config.DockerFlags{EnsureDockerRegistry: true}, nil, StatusFail},
		{"present via flags", config.DockerFlags{DockerUsername: "u", DockerPassword: "p", EnsureDockerRegistry: true}, nil, StatusOK},
		{"present via env", config.DockerFlags{EnsureDockerRegistry: true}, map[string]string{"HARBOR_USERNAME": "u", "HARBOR_PASSWORD": "p"}, StatusOK},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			flags := &config.RuntimeFlags{Docker: tt.flags}
			checks := checkDockerCredentials(flags, tt.envMap)
			if len(checks) == 0 {
				t.Fatal("expected at least the Harbor check")
			}
			if checks[0].Status != tt.wantHead {
				t.Errorf("Harbor status = %q, want %q (detail: %s)", checks[0].Status, tt.wantHead, checks[0].Detail)
			}
		})
	}
}

func TestCheckScenarioEnv(t *testing.T) {
	dir := t.TempDir()
	// Legacy single-file scenario fixture (prefix matches scenarios.ValuesFilePrefix).
	scenarioFile := filepath.Join(dir, "values-integration-test-ingress-doctortest.yaml")
	if err := os.WriteFile(scenarioFile, []byte("license:\n  key: ${DOCTOR_LICENSE}\nfoo: $DOCTOR_FOO\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := func() *config.RuntimeFlags {
		f := &config.RuntimeFlags{}
		f.Chart.ChartPath = dir // non-empty so the scan runs
		f.Deployment.ScenarioPath = dir
		f.Deployment.Scenario = "doctortest"
		return f
	}

	t.Run("no chart configured is OK", func(t *testing.T) {
		c := checkScenarioEnv(&config.RuntimeFlags{}, nil)
		if c.Status != StatusOK {
			t.Errorf("status = %q, want ok", c.Status)
		}
	})

	t.Run("all placeholders set", func(t *testing.T) {
		c := checkScenarioEnv(base(), map[string]string{"DOCTOR_LICENSE": "x", "DOCTOR_FOO": "y"})
		if c.Status != StatusOK {
			t.Errorf("status = %q, want ok (detail: %s)", c.Status, c.Detail)
		}
	})

	t.Run("missing placeholders fail", func(t *testing.T) {
		c := checkScenarioEnv(base(), map[string]string{"DOCTOR_FOO": "y"})
		if c.Status != StatusFail {
			t.Fatalf("status = %q, want fail (detail: %s)", c.Status, c.Detail)
		}
		if len(c.Missing) != 1 || c.Missing[0] != "DOCTOR_LICENSE" {
			t.Errorf("missing = %v, want [DOCTOR_LICENSE]", c.Missing)
		}
	})

	t.Run("unresolvable scenario warns", func(t *testing.T) {
		f := base()
		f.Deployment.Scenario = "does-not-exist"
		c := checkScenarioEnv(f, nil)
		if c.Status != StatusWarn {
			t.Errorf("status = %q, want warn (detail: %s)", c.Status, c.Detail)
		}
	})
}

// TestCheckScenarioEnvScansLayeredFiles guards the fix for placeholders that
// live only in a non-base layer (identity/persistence/feature). The deploy
// composes the full layered set, so a $VAR in values/persistence/elasticsearch.yaml
// (e.g. $VENOM_CLIENT_ID) must be detected by the preflight; scanning only the
// top-level scenario file made it false-negative and let the deploy fail later
// in prepareScenarioValues.
func TestCheckScenarioEnvScansLayeredFiles(t *testing.T) {
	dir := t.TempDir()
	valuesDir := filepath.Join(dir, "values")
	persistenceDir := filepath.Join(valuesDir, "persistence")
	if err := os.MkdirAll(persistenceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// base.yaml makes the dir "layered"; the placeholder lives only in the
	// persistence layer, never in base.
	if err := os.WriteFile(filepath.Join(valuesDir, "base.yaml"), []byte("global:\n  test: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(persistenceDir, "elasticsearch.yaml"), []byte("foo:\n  bar: $LAYERED_ONLY_VAR\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	flags := func() *config.RuntimeFlags {
		f := &config.RuntimeFlags{}
		f.Chart.ChartPath = dir
		f.Deployment.ScenarioPath = dir
		f.Deployment.Scenario = "elasticsearch"
		// Selection drives ResolvePaths to include the persistence layer, exactly
		// as the matrix populates it per entry.
		f.Selection.Identity = "keycloak"
		f.Selection.Persistence = "elasticsearch"
		f.Selection.TestPlatform = "gke"
		return f
	}

	t.Run("layered placeholder flagged when unset", func(t *testing.T) {
		c := checkScenarioEnv(flags(), map[string]string{})
		if c.Status != StatusFail {
			t.Fatalf("status = %q, want fail (detail: %s)", c.Status, c.Detail)
		}
		if len(c.Missing) != 1 || c.Missing[0] != "LAYERED_ONLY_VAR" {
			t.Errorf("missing = %v, want [LAYERED_ONLY_VAR]", c.Missing)
		}
	})

	t.Run("layered placeholder satisfied when set", func(t *testing.T) {
		c := checkScenarioEnv(flags(), map[string]string{"LAYERED_ONLY_VAR": "x"})
		if c.Status != StatusOK {
			t.Errorf("status = %q, want ok (detail: %s)", c.Status, c.Detail)
		}
	})
}

// TestBuildScenarioEnvSeedsKeycloakClientIDs guards the local↔CI parity defaults
// that mirror test-integration-runner.yaml's workflow env (VENOM_CLIENT_ID=venom,
// CONNECTORS_CLIENT_ID=connectors). Without them, keycloak elasticsearch
// scenarios fail locally on the $VENOM_CLIENT_ID/$CONNECTORS_CLIENT_ID
// placeholders. flags.ExtraEnv (used by OIDC/Entra) must still override.
func TestBuildScenarioEnvSeedsKeycloakClientIDs(t *testing.T) {
	ctx := &ScenarioContext{}
	t.Run("seeds keycloak defaults", func(t *testing.T) {
		env, err := buildScenarioEnv(ctx, &config.RuntimeFlags{})
		if err != nil {
			t.Fatal(err)
		}
		if env["VENOM_CLIENT_ID"] != "venom" {
			t.Errorf("VENOM_CLIENT_ID = %q, want venom", env["VENOM_CLIENT_ID"])
		}
		if env["CONNECTORS_CLIENT_ID"] != "connectors" {
			t.Errorf("CONNECTORS_CLIENT_ID = %q, want connectors", env["CONNECTORS_CLIENT_ID"])
		}
	})

	t.Run("ExtraEnv (OIDC) overrides the seed", func(t *testing.T) {
		f := &config.RuntimeFlags{ExtraEnv: map[string]string{"VENOM_CLIENT_ID": "entra-guid"}}
		env, err := buildScenarioEnv(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		if env["VENOM_CLIENT_ID"] != "entra-guid" {
			t.Errorf("VENOM_CLIENT_ID = %q, want entra-guid (ExtraEnv must win)", env["VENOM_CLIENT_ID"])
		}
	})
}

func TestCheckCompanionEnv(t *testing.T) {
	withCharts := func(charts ...config.CompanionChart) *config.RuntimeFlags {
		return &config.RuntimeFlags{CompanionCharts: charts}
	}
	pg := config.CompanionChart{
		ReleaseName: "postgresql",
		ValuesFile:  "/repo/test/integration/companion-values/postgresql.yaml",
		EnvVars:     []string{"RDBMS_POSTGRESQL_USERNAME", "RDBMS_POSTGRESQL_PASSWORD"},
	}

	t.Run("no companion charts is OK", func(t *testing.T) {
		if c := checkCompanionEnv(withCharts(), nil); c.Status != StatusOK {
			t.Errorf("status = %q, want ok", c.Status)
		}
	})

	t.Run("chart without env vars is OK", func(t *testing.T) {
		c := checkCompanionEnv(withCharts(config.CompanionChart{ReleaseName: "os", ValuesFile: "/x.yaml"}), nil)
		if c.Status != StatusOK {
			t.Errorf("status = %q, want ok", c.Status)
		}
	})

	t.Run("missing companion vars fail", func(t *testing.T) {
		c := checkCompanionEnv(withCharts(pg), map[string]string{"RDBMS_POSTGRESQL_USERNAME": "u"})
		if c.Status != StatusFail {
			t.Fatalf("status = %q, want fail (detail: %s)", c.Status, c.Detail)
		}
		if len(c.Missing) != 1 || c.Missing[0] != "RDBMS_POSTGRESQL_PASSWORD" {
			t.Errorf("missing = %v, want [RDBMS_POSTGRESQL_PASSWORD]", c.Missing)
		}
	})

	t.Run("present (even empty) is OK, matching substitution semantics", func(t *testing.T) {
		// substituteCompanionEnvVars treats an existing-but-empty key as present.
		c := checkCompanionEnv(withCharts(pg), map[string]string{
			"RDBMS_POSTGRESQL_USERNAME": "u",
			"RDBMS_POSTGRESQL_PASSWORD": "",
		})
		if c.Status != StatusOK {
			t.Errorf("status = %q, want ok (empty value should count as present)", c.Status)
		}
	})
}

func TestScenarioDeployEnvResolvesComputedVars(t *testing.T) {
	dir := t.TempDir()
	// Scenario values file that references a deploy-computed var.
	scenarioFile := filepath.Join(dir, "values-integration-test-ingress-hosttest.yaml")
	if err := os.WriteFile(scenarioFile, []byte("ingress:\n  host: ${CAMUNDA_HOSTNAME}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := func() *config.RuntimeFlags {
		f := &config.RuntimeFlags{}
		f.Chart.ChartPath = dir
		f.Deployment.ScenarioPath = dir
		f.Deployment.Scenario = "hosttest"
		f.Deployment.Scenarios = []string{"hosttest"}
		return f
	}

	t.Run("CAMUNDA_HOSTNAME satisfied when ingress resolvable", func(t *testing.T) {
		f := base()
		f.Ingress.IngressSubdomain = "matrix-810-eske-inst-gke"
		f.Ingress.IngressBaseDomain = "ci.distro.ultrawombat.com"

		// scenarioDeployEnv must include the computed CAMUNDA_HOSTNAME.
		env := scenarioDeployEnv(f, effectiveEnv(f))
		if env["CAMUNDA_HOSTNAME"] == "" {
			t.Fatalf("CAMUNDA_HOSTNAME not computed; env keys present? %v", env["CAMUNDA_HOSTNAME"])
		}
		// And the scenario check (which Preflight feeds deployEnv) must pass.
		if c := checkScenarioEnv(f, env); c.Status != StatusOK {
			t.Errorf("scenario env status = %q, want ok (detail: %s)", c.Status, c.Detail)
		}
	})

	t.Run("CAMUNDA_HOSTNAME still flagged when ingress unresolvable", func(t *testing.T) {
		f := base() // no ingress flags → ResolveIngressHostname == ""
		env := scenarioDeployEnv(f, effectiveEnv(f))
		if env["CAMUNDA_HOSTNAME"] != "" {
			t.Fatalf("CAMUNDA_HOSTNAME should be empty without ingress flags, got %q", env["CAMUNDA_HOSTNAME"])
		}
		c := checkScenarioEnv(f, env)
		if c.Status != StatusFail || len(c.Missing) != 1 || c.Missing[0] != "CAMUNDA_HOSTNAME" {
			t.Errorf("want fail with [CAMUNDA_HOSTNAME], got status=%q missing=%v", c.Status, c.Missing)
		}
	})
}

func TestReportOKAndMissingEnv(t *testing.T) {
	r := &Report{Checks: []Check{
		{Name: "a", Status: StatusOK},
		{Name: "b", Status: StatusWarn},
		{Name: "c", Status: StatusFail, Missing: []string{"Z", "A"}},
		{Name: "d", Status: StatusFail, Missing: []string{"A"}},
	}}
	if r.OK() {
		t.Error("OK() should be false when a check failed")
	}
	got := r.MissingEnv()
	if len(got) != 2 || got[0] != "A" || got[1] != "Z" {
		t.Errorf("MissingEnv() = %v, want sorted union [A Z]", got)
	}

	clean := &Report{Checks: []Check{{Name: "a", Status: StatusOK}, {Name: "b", Status: StatusWarn}}}
	if !clean.OK() {
		t.Error("OK() should be true with only ok/warn checks")
	}
}

func TestRunFailFastPreflight(t *testing.T) {
	// A vault mapping with an unset var makes the preflight fail.
	failing := func() *config.RuntimeFlags {
		f := &config.RuntimeFlags{}
		f.Test.KubeContext = "test-ctx" // OK without reachability probe
		f.Secrets.VaultSecretMapping = "ci/path DEFINITELY_UNSET_VAR_XYZ;"
		return f
	}

	t.Run("skip bypasses entirely", func(t *testing.T) {
		f := failing()
		f.SkipPreflight = true
		if err := runFailFastPreflight(context.Background(), f); err != nil {
			t.Errorf("SkipPreflight should bypass, got %v", err)
		}
	})

	t.Run("non-interactive fails fast", func(t *testing.T) {
		f := failing()
		f.Interactive = false
		if err := runFailFastPreflight(context.Background(), f); err == nil {
			t.Error("expected fail-fast error for missing vault var")
		}
	})

	t.Run("interactive downgrades to warning", func(t *testing.T) {
		f := failing()
		f.Interactive = true
		if err := runFailFastPreflight(context.Background(), f); err != nil {
			t.Errorf("interactive should not hard-fail, got %v", err)
		}
	})
}

func TestResolveMissingInteractively(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	// Scripted stdin. env.Prompt creates a fresh bufio.Reader per call, which
	// buffers ahead on piped (non-TTY) input — so multi-line piping only feeds
	// the first prompt reliably. That's a non-interactive artifact (real TTY use
	// is line-buffered and feeds each prompt); the test validates the
	// persist-and-set mechanism with a single missing var.
	stdinPath := filepath.Join(dir, "stdin")
	if err := os.WriteFile(stdinPath, []byte("supersecret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(stdinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	oldStdin := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = oldStdin
		os.Unsetenv("RDBMS_POSTGRESQL_PASSWORD")
	})

	report := &Report{Checks: []Check{{
		Name:    "companion env vars",
		Status:  StatusFail,
		Missing: []string{"RDBMS_POSTGRESQL_PASSWORD"},
	}}}
	flags := &config.RuntimeFlags{}
	flags.EnvFile = envFile

	n, err := ResolveMissingInteractively(context.Background(), report, flags)
	if err != nil {
		t.Fatalf("ResolveMissingInteractively: %v", err)
	}
	if n != 1 {
		t.Errorf("resolved = %d, want 1", n)
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf(".env not written: %v", err)
	}
	if !strings.Contains(string(data), "RDBMS_POSTGRESQL_PASSWORD") {
		t.Errorf(".env missing RDBMS_POSTGRESQL_PASSWORD; content:\n%s", string(data))
	}
	if os.Getenv("RDBMS_POSTGRESQL_PASSWORD") == "" {
		t.Error("process env RDBMS_POSTGRESQL_PASSWORD not set")
	}
}

func TestPreflightSmoke(t *testing.T) {
	flags := &config.RuntimeFlags{}
	flags.Test.KubeContext = "test-ctx" // avoids shelling out to kubectl current-context
	r := Preflight(context.Background(), flags, PreflightOptions{
		ConfigPath:           "/tmp/x.yaml",
		ConfigFound:          true,
		SkipKubeReachability: true,
	})
	if r == nil || len(r.Checks) == 0 {
		t.Fatal("expected a populated report")
	}
	if !r.OK() {
		t.Errorf("clean smoke run should be OK; checks: %+v", r.Checks)
	}
}

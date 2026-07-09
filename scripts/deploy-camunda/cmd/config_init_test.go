package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigInitWizard(t *testing.T) {
	// Hermetic + fast: no kubectl on PATH so kube probes fail immediately rather
	// than waiting on a network timeout.
	t.Setenv("PATH", "")

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".deploy-camunda.yaml")

	// Point the shared config path at our temp file and restore afterward.
	prev := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prev })

	// Scripted answers: name, platform, kube-context(default), ingress(default),
	// repo-root, then "no" to docker creds and "no" to test secrets.
	input := strings.Join([]string{
		"testprofile",
		"eks",
		"", // kube context → default (empty, no kubectl)
		"", // ingress base domain → default
		"/tmp/repo",
		"n", // docker creds
		"n", // test secrets
		"n", // RDBMS dev credentials
	}, "\n") + "\n"

	cmd := newInitCommand()
	cmd.SetIn(strings.NewReader(input))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--env-file", filepath.Join(tmp, ".env")})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init wizard returned error: %v\noutput:\n%s", err, out.String())
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	var data map[string]any
	if err := yaml.Unmarshal(raw, &data); err != nil {
		t.Fatalf("config file is not valid YAML: %v", err)
	}

	if data["current"] != "testprofile" {
		t.Errorf("current = %v, want testprofile", data["current"])
	}
	deployments, ok := data["deployments"].(map[string]any)
	if !ok {
		t.Fatalf("deployments map missing: %#v", data)
	}
	profile, ok := deployments["testprofile"].(map[string]any)
	if !ok {
		t.Fatalf("testprofile missing: %#v", deployments)
	}
	if profile["platform"] != "eks" {
		t.Errorf("platform = %v, want eks", profile["platform"])
	}
	if profile["repoRoot"] != "/tmp/repo" {
		t.Errorf("repoRoot = %v, want /tmp/repo", profile["repoRoot"])
	}

	// The doctor-after-init checklist must reach the cobra output writer (not
	// bare os.Stdout), otherwise it is invisible to callers capturing output.
	// The "config file" check line is rendered by report.Render into `out`.
	if got := out.String(); !strings.Contains(got, "config file") {
		t.Errorf("doctor checklist not captured in command output; got:\n%s", got)
	}
}

func TestConfigInitScaffoldsRDBMS(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".deploy-camunda.yaml")
	envPath := filepath.Join(tmp, ".env")
	prev := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prev })

	input := strings.Join([]string{
		"local", // profile name
		"gke",   // platform
		"",      // kube context
		"",      // ingress base domain
		tmp,     // repo root
		"n",     // docker creds
		"n",     // test secrets
		"y",     // RDBMS dev credentials → scaffold
	}, "\n") + "\n"

	cmd := newInitCommand()
	cmd.SetIn(strings.NewReader(input))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--env-file", envPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init wizard error: %v\n%s", err, out.String())
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf(".env not written: %v", err)
	}
	got := string(data)
	for _, key := range []string{"RDBMS_POSTGRESQL_USERNAME", "RDBMS_POSTGRESQL_PASSWORD"} {
		if !strings.Contains(got, key) {
			t.Errorf(".env missing %s; content:\n%s", key, got)
		}
	}
}

func TestConfigInitNonInteractiveRequiresConfig(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".deploy-camunda.yaml")
	prev := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prev })

	cmd := newInitCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--non-interactive"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --non-interactive and no config exists")
	}
}

func TestConfigInitFromExample(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".camunda-deploy.yaml")
	prev := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prev })

	cmd := newInitCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--from-example", "getting-started"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--from-example returned error: %v\n%s", err, out.String())
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if !strings.Contains(string(raw), "Getting Started") {
		t.Errorf("written config missing banner from getting-started template; content:\n%s", string(raw))
	}
	if !strings.Contains(out.String(), "Wrote starter config") {
		t.Errorf("expected user-facing 'Wrote starter config' message; got:\n%s", out.String())
	}
}

func TestConfigInitFromExampleRefusesToOverwrite(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".camunda-deploy.yaml")
	if err := os.WriteFile(cfgPath, []byte("existing: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prev })

	cmd := newInitCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--from-example", "getting-started"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when writing over existing config without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected error to mention --force; got: %v", err)
	}
	raw, _ := os.ReadFile(cfgPath)
	if string(raw) != "existing: true\n" {
		t.Errorf("original config was clobbered without --force; new content:\n%s", string(raw))
	}
}

func TestConfigInitFromExampleForceOverwrite(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".camunda-deploy.yaml")
	if err := os.WriteFile(cfgPath, []byte("existing: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prev })

	cmd := newInitCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--from-example", "getting-started", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--force overwrite failed: %v", err)
	}
	raw, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(raw), "existing: true") {
		t.Errorf("--force did not overwrite existing config; content:\n%s", string(raw))
	}
}

func TestConfigInitFromExampleUnknownName(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".camunda-deploy.yaml")
	prev := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prev })

	cmd := newInitCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--from-example", "does-not-exist"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown example name")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should name the missing example; got: %v", err)
	}
}

func TestConfigInitListExamples(t *testing.T) {
	t.Setenv("PATH", "")
	cmd := newInitCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--list-examples"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--list-examples returned error: %v", err)
	}
	if !strings.Contains(out.String(), "getting-started") {
		t.Errorf("--list-examples did not include getting-started; output:\n%s", out.String())
	}
}

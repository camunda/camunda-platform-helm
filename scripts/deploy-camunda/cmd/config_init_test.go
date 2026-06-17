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
	cfgPath := filepath.Join(tmp, ".camunda-deploy.yaml")

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
	cfgPath := filepath.Join(tmp, ".camunda-deploy.yaml")
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
	cfgPath := filepath.Join(tmp, ".camunda-deploy.yaml")
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

package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"scripts/deploy-camunda/config"
)

func TestEnvProvenance(t *testing.T) {
	// A process var that .env will override, and one only in the process.
	t.Setenv("PROV_OVERRIDDEN", "from-process")
	t.Setenv("PROV_PROCESS_ONLY", "p")

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("PROV_OVERRIDDEN=from-dotenv\nPROV_DOTENV_ONLY=d\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	flags := &config.RuntimeFlags{
		EnvFile:  envFile,
		ExtraEnv: map[string]string{"PROV_EXTRA": "e", "PROV_OVERRIDDEN": "from-extra"},
	}

	got := map[string]EnvVar{}
	for _, e := range EnvProvenance(flags) {
		got[e.Name] = e
	}

	cases := []struct {
		name       string
		wantValue  string
		wantOrigin string
	}{
		{"PROV_PROCESS_ONLY", "p", "process-env"},
		{"PROV_DOTENV_ONLY", "d", ".env (" + envFile + ")"},
		{"PROV_EXTRA", "e", "extra-env"},
		// extra-env is the last layer, so it wins over both process and .env.
		{"PROV_OVERRIDDEN", "from-extra", "extra-env"},
	}
	for _, c := range cases {
		e, ok := got[c.name]
		if !ok {
			t.Errorf("%s missing from provenance", c.name)
			continue
		}
		if e.Value != c.wantValue {
			t.Errorf("%s value = %q, want %q", c.name, e.Value, c.wantValue)
		}
		if e.Origin != c.wantOrigin {
			t.Errorf("%s origin = %q, want %q", c.name, e.Origin, c.wantOrigin)
		}
	}
}

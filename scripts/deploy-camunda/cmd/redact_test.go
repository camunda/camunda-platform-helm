package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const playwrightResultsFixture = `{
  "suites": [
    {
      "title": "smoke-tests.spec.js",
      "specs": [
        {
          "title": "logs in to Operate",
          "ok": false,
          "tests": [
            {
              "results": [
                {
                  "status": "failed",
                  "error": {
                    "message": "expect(received).toBe(expected)\n\nExpected: 200\nReceived: 401",
                    "stack": "at smoke-tests.spec.js:42:19"
                  },
                  "stdout": [
                    "request headers: authorization: Bearer eyJhbGciOiJIUzI1NiJ9.cGF5bG9hZHBheWxvYWQ.c2lnbmF0dXJl",
                    "ZEEBE_CLIENT_SECRET=super-secret-value"
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}
`

func TestRunRedactStreamRedactsCredentialsAndKeepsDiagnostics(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	if err := runRedact("", "", strings.NewReader(playwrightResultsFixture), &out); err != nil {
		t.Fatalf("runRedact returned error: %v", err)
	}
	got := out.String()

	for _, secret := range []string{
		"super-secret-value",
		"eyJhbGciOiJIUzI1NiJ9.cGF5bG9hZHBheWxvYWQ.c2lnbmF0dXJl",
	} {
		if strings.Contains(got, secret) {
			t.Errorf("output still contains %q:\n%s", secret, got)
		}
	}

	// The whole point of retaining this artifact is the triage signal, so the
	// test identity and assertion detail must survive redaction.
	for _, keep := range []string{
		"smoke-tests.spec.js",
		"logs in to Operate",
		"expect(received).toBe(expected)",
		"Expected: 200",
		"Received: 401",
		"at smoke-tests.spec.js:42:19",
		`"status": "failed"`,
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("output dropped triage detail %q:\n%s", keep, got)
		}
	}
}

// The artifact is only useful to triage tooling if it still parses, so redaction
// must not swallow the quotes that delimit the values it rewrites.
func TestRunRedactKeepsOutputValidJSON(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	if err := runRedact("", "", strings.NewReader(playwrightResultsFixture), &out); err != nil {
		t.Fatalf("runRedact returned error: %v", err)
	}

	var parsed any
	if err := json.Unmarshal([]byte(out.String()), &parsed); err != nil {
		t.Fatalf("redacted output is not valid JSON: %v\n%s", err, out.String())
	}
}

func TestRunRedactWritesOwnerOnlyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "nested", "playwright-results.json")

	if err := runRedact("", outPath, strings.NewReader(playwrightResultsFixture), nil); err != nil {
		t.Fatalf("runRedact returned error: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("output mode = %#o, want 0600", perm)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(body), "super-secret-value") {
		t.Errorf("written file still contains the secret:\n%s", body)
	}
}

func TestRunRedactInPlaceRewrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	if err := os.WriteFile(path, []byte(playwrightResultsFixture), 0o600); err != nil {
		t.Fatalf("seed input: %v", err)
	}

	if err := runRedact(path, path, nil, nil); err != nil {
		t.Fatalf("runRedact returned error: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(body), "super-secret-value") {
		t.Errorf("in-place rewrite kept the secret:\n%s", body)
	}
	if !strings.Contains(string(body), "logs in to Operate") {
		t.Errorf("in-place rewrite lost triage detail:\n%s", body)
	}
}

func TestRunRedactRefusesSymlinkOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("UNTOUCHED=1\n"), 0o600); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	err := runRedact("", link, strings.NewReader(playwrightResultsFixture), nil)
	if err == nil {
		t.Fatal("expected an error when --out is a symbolic link")
	}
	if !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("unexpected error: %v", err)
	}

	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(body) != "UNTOUCHED=1\n" {
		t.Errorf("symlink target was modified: %q", body)
	}
}

func TestRunRedactReportsMissingInput(t *testing.T) {
	t.Parallel()

	err := runRedact(filepath.Join(t.TempDir(), "absent.json"), "", nil, nil)
	if err == nil {
		t.Fatal("expected an error for a missing --in file")
	}
	if !strings.Contains(err.Error(), "absent.json") {
		t.Errorf("error should name the missing file, got: %v", err)
	}
}

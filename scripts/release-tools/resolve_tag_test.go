package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runResolveTagCapture runs runResolveTag with GITHUB_OUTPUT pointed at a temp
// file and os.Stdout redirected, returning the parsed $GITHUB_OUTPUT key/values
// and the captured stdout.
func runResolveTagCapture(t *testing.T, args []string) (map[string]string, string, error) {
	t.Helper()
	dir := t.TempDir()
	ghoPath := filepath.Join(dir, "gho.txt")
	t.Setenv("GITHUB_OUTPUT", ghoPath)

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := runResolveTag(args)

	w.Close()
	os.Stdout = origStdout
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stdout := string(buf[:n])

	out := map[string]string{}
	if data, ferr := os.ReadFile(ghoPath); ferr == nil {
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if ok {
				out[k] = v
			}
		}
	}
	return out, strings.TrimSpace(stdout), runErr
}

func writeTags(t *testing.T, names ...string) string {
	t.Helper()
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = `{"name":"` + n + `"}`
	}
	p := filepath.Join(t.TempDir(), "tags.json")
	if err := os.WriteFile(p, []byte("["+strings.Join(parts, ",")+"]"), 0o644); err != nil {
		t.Fatalf("write tags: %v", err)
	}
	return p
}

func TestResolveTagDevRolling(t *testing.T) {
	tags := writeTags(t, "13-dev-latest", "13.4.0-dev-abc1234", "13-rc-latest", "13.4.0-rc")
	out, stdout, err := runResolveTagCapture(t, []string{"--kind", "dev", "--input-tag", "13-dev-latest", "--tags-file", tags})
	if err != nil {
		t.Fatalf("runResolveTag: %v", err)
	}
	want := map[string]string{
		"resolved_tag": "13.4.0-dev-abc1234", "version": "13.4.0",
		"chart_major": "13", "rc_tag": "13.4.0-rc", "rc_latest_tag": "13-rc-latest",
	}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("output %s=%q want %q", k, out[k], v)
		}
	}
	if _, emitted := out["sha"]; emitted {
		t.Error("sha must NOT be emitted to $GITHUB_OUTPUT (workflow expands+emits it)")
	}
	if stdout != "abc1234" {
		t.Errorf("stdout sha = %q want abc1234", stdout)
	}
}

func TestResolveTagDevConcreteNoTagsFile(t *testing.T) {
	out, stdout, err := runResolveTagCapture(t, []string{"--kind", "dev", "--input-tag", "14.0.0-alpha2-dev-deadbeef"})
	if err != nil {
		t.Fatalf("runResolveTag: %v", err)
	}
	if out["resolved_tag"] != "14.0.0-alpha2-dev-deadbeef" || out["version"] != "14.0.0-alpha2" ||
		out["chart_major"] != "14" || out["rc_tag"] != "14.0.0-alpha2-rc" || out["rc_latest_tag"] != "14-rc-latest" {
		t.Errorf("unexpected dev output: %v", out)
	}
	if stdout != "deadbeef" {
		t.Errorf("stdout sha = %q want deadbeef", stdout)
	}
}

func TestResolveTagRcWithDevTag(t *testing.T) {
	tags := writeTags(t, "13.4.0-rc", "13-rc-latest", "13.4.0-dev-abc1234", "13-dev-latest")
	out, _, err := runResolveTagCapture(t, []string{"--kind", "rc", "--input-tag", "13-rc-latest", "--tags-file", tags})
	if err != nil {
		t.Fatalf("runResolveTag: %v", err)
	}
	if out["resolved_tag"] != "13.4.0-rc" || out["version"] != "13.4.0" ||
		out["dev_tag"] != "13.4.0-dev-abc1234" || out["commit_sha"] != "abc1234" {
		t.Errorf("unexpected rc output: %v", out)
	}
	// rc must NOT emit dev-only keys.
	for _, k := range []string{"chart_major", "rc_tag", "rc_latest_tag"} {
		if _, ok := out[k]; ok {
			t.Errorf("rc emitted unexpected key %q", k)
		}
	}
}

func TestResolveTagRcNoDevTag(t *testing.T) {
	tags := writeTags(t, "14.0.0-alpha2-rc", "14-rc-latest")
	out, _, err := runResolveTagCapture(t, []string{"--kind", "rc", "--input-tag", "14.0.0-alpha2-rc", "--tags-file", tags})
	if err != nil {
		t.Fatalf("runResolveTag: %v", err)
	}
	// dev_tag/commit_sha emitted but empty (matches bash else-branch).
	if v, ok := out["dev_tag"]; !ok || v != "" {
		t.Errorf("dev_tag = %q (present=%v) want empty+present", v, ok)
	}
	if v, ok := out["commit_sha"]; !ok || v != "" {
		t.Errorf("commit_sha = %q (present=%v) want empty+present", v, ok)
	}
}

func TestResolveTagErrors(t *testing.T) {
	// rolling without tags-file
	if _, _, err := runResolveTagCapture(t, []string{"--kind", "dev", "--input-tag", "13-dev-latest"}); err == nil {
		t.Error("expected error for rolling dev tag without --tags-file")
	}
	// invalid concrete format
	if _, _, err := runResolveTagCapture(t, []string{"--kind", "dev", "--input-tag", "not-a-tag"}); err == nil {
		t.Error("expected error for invalid dev tag")
	}
	// bad kind
	if _, _, err := runResolveTagCapture(t, []string{"--kind", "bogus", "--input-tag", "x"}); err == nil {
		t.Error("expected error for invalid --kind")
	}
}

package env

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// helper: write content to a temp .env file and return its path.
func writeTempEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTempEnv: %v", err)
	}
	return p
}

// helper: read file contents as string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// extractKey
// ---------------------------------------------------------------------------

func TestExtractKey(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		// Basic
		{"FOO=bar", "FOO"},
		{"MY_VAR=hello world", "MY_VAR"},
		// With export prefix
		{"export FOO=bar", "FOO"},
		{"export  FOO=bar", "FOO"},
		// Quoted values
		{`FOO="bar baz"`, "FOO"},
		{`FOO='bar baz'`, "FOO"},
		// Leading whitespace
		{"  FOO=bar", "FOO"},
		{"\tFOO=bar", "FOO"},
		{"  export FOO=bar", "FOO"},
		// Comments and blanks
		{"# comment", ""},
		{"  # indented comment", ""},
		{"", ""},
		{"  ", ""},
		// No equals
		{"FOO", ""},
		// Starts with equals
		{"=value", ""},
	}

	for _, tt := range tests {
		got := extractKey(tt.line)
		if got != tt.want {
			t.Errorf("extractKey(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// quoteValue
// ---------------------------------------------------------------------------

func TestQuoteValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"abc123", "abc123"},
		{"", `""`},
		{"has space", `"has space"`},
		{"has\ttab", "\"has\ttab\""},
		{`has"quote`, `"has\"quote"`},
		{`has\backslash`, `"has\\backslash"`},
		{"has#hash", `"has#hash"`},
		{"has$dollar", `"has$dollar"`},
	}
	for _, tt := range tests {
		got := quoteValue(tt.input)
		if got != tt.want {
			t.Errorf("quoteValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// rewriteLine
// ---------------------------------------------------------------------------

func TestRewriteLine(t *testing.T) {
	tests := []struct {
		line   string
		key    string
		newVal string
		want   string
	}{
		// Simple replacement
		{"FOO=old", "FOO", "new", "FOO=new"},
		// With export prefix
		{"export FOO=old", "FOO", "new", "export FOO=new"},
		// Leading whitespace preserved
		{"  FOO=old", "FOO", "new", "  FOO=new"},
		// Value needing quoting
		{"FOO=old", "FOO", "has space", `FOO="has space"`},
		// Quoted old value replaced
		{`FOO="old value"`, "FOO", "new", "FOO=new"},
		// Export + leading whitespace
		{"  export FOO=old", "FOO", "new", "  export FOO=new"},
	}
	for _, tt := range tests {
		got := rewriteLine(tt.line, tt.key, tt.newVal)
		if got != tt.want {
			t.Errorf("rewriteLine(%q, %q, %q) = %q, want %q",
				tt.line, tt.key, tt.newVal, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Append — single key
// ---------------------------------------------------------------------------

func TestAppend_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")

	if err := Append(p, "NEW_KEY", "new_value"); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "NEW_KEY=new_value") {
		t.Errorf("expected NEW_KEY=new_value in:\n%s", got)
	}
}

func TestAppend_UpdatesExistingKey(t *testing.T) {
	p := writeTempEnv(t, "FOO=old\nBAR=keep\n")

	if err := Append(p, "FOO", "new"); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "FOO=new") {
		t.Errorf("expected FOO=new, got:\n%s", got)
	}
	if !strings.Contains(got, "BAR=keep") {
		t.Errorf("expected BAR=keep preserved, got:\n%s", got)
	}
}

func TestAppend_AppendsNewKey(t *testing.T) {
	p := writeTempEnv(t, "EXISTING=value\n")

	if err := Append(p, "NEW_KEY", "new_value"); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got := readFile(t, p)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %q", len(lines), got)
	}
	// EXISTING should still be first.
	if !strings.HasPrefix(lines[0], "EXISTING=") {
		t.Errorf("first line should start with EXISTING=, got %q", lines[0])
	}
}

// ---------------------------------------------------------------------------
// AppendMultiple — format preservation
// ---------------------------------------------------------------------------

func TestAppendMultiple_PreservesComments(t *testing.T) {
	original := "# This is a comment\nFOO=bar\n# Another comment\nBAZ=qux\n"
	p := writeTempEnv(t, original)

	if err := AppendMultiple(p, map[string]string{"FOO": "updated"}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "# This is a comment") {
		t.Errorf("first comment lost:\n%s", got)
	}
	if !strings.Contains(got, "# Another comment") {
		t.Errorf("second comment lost:\n%s", got)
	}
	if !strings.Contains(got, "FOO=updated") {
		t.Errorf("expected FOO=updated:\n%s", got)
	}
	if !strings.Contains(got, "BAZ=qux") {
		t.Errorf("expected BAZ=qux preserved:\n%s", got)
	}
}

func TestAppendMultiple_PreservesKeyOrder(t *testing.T) {
	original := "ZEBRA=1\nAPPLE=2\nMIDDLE=3\n"
	p := writeTempEnv(t, original)

	if err := AppendMultiple(p, map[string]string{"APPLE": "updated"}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	// Order must be preserved: ZEBRA, APPLE, MIDDLE (NOT sorted alphabetically).
	if !strings.HasPrefix(lines[0], "ZEBRA=") {
		t.Errorf("line 0: expected ZEBRA=, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "APPLE=") {
		t.Errorf("line 1: expected APPLE=, got %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "MIDDLE=") {
		t.Errorf("line 2: expected MIDDLE=, got %q", lines[2])
	}
}

func TestAppendMultiple_PreservesExportPrefix(t *testing.T) {
	original := "export MY_VAR=old\n"
	p := writeTempEnv(t, original)

	if err := AppendMultiple(p, map[string]string{"MY_VAR": "new"}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "export MY_VAR=new") {
		t.Errorf("expected 'export MY_VAR=new', got:\n%s", got)
	}
}

func TestAppendMultiple_PreservesLeadingWhitespace(t *testing.T) {
	original := "  INDENTED=old\n\tTABBED=old\n"
	p := writeTempEnv(t, original)

	updates := map[string]string{
		"INDENTED": "new1",
		"TABBED":   "new2",
	}
	if err := AppendMultiple(p, updates); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "  INDENTED=new1") {
		t.Errorf("expected leading spaces preserved for INDENTED:\n%s", got)
	}
	if !strings.Contains(got, "\tTABBED=new2") {
		t.Errorf("expected leading tab preserved for TABBED:\n%s", got)
	}
}

func TestAppendMultiple_MixUpdateAndAppend(t *testing.T) {
	original := "EXISTING=old\n"
	p := writeTempEnv(t, original)

	updates := map[string]string{
		"EXISTING":  "updated",
		"BRAND_NEW": "fresh",
	}
	if err := AppendMultiple(p, updates); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "EXISTING=updated") {
		t.Errorf("expected EXISTING=updated:\n%s", got)
	}
	if !strings.Contains(got, "BRAND_NEW=fresh") {
		t.Errorf("expected BRAND_NEW=fresh:\n%s", got)
	}
	// EXISTING should come before BRAND_NEW (original order, new keys appended).
	existingIdx := strings.Index(got, "EXISTING=")
	brandNewIdx := strings.Index(got, "BRAND_NEW=")
	if existingIdx >= brandNewIdx {
		t.Errorf("expected EXISTING before BRAND_NEW in output:\n%s", got)
	}
}

func TestAppendMultiple_EmptyUpdatesNoOp(t *testing.T) {
	original := "FOO=bar\n"
	p := writeTempEnv(t, original)

	if err := AppendMultiple(p, map[string]string{}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	if got != original {
		t.Errorf("expected file unchanged, got:\n%s", got)
	}
}

func TestAppendMultiple_PreservesPasswordsWithLeadingZeros(t *testing.T) {
	// This was the critical bug: godotenv.Write would strip leading zeros
	// via strconv.Atoi, turning "01234" into 1234.
	original := `PASSWORD="01234"
OTHER_KEY=value
`
	p := writeTempEnv(t, original)

	if err := AppendMultiple(p, map[string]string{"OTHER_KEY": "updated"}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	// The PASSWORD line should be completely untouched.
	if !strings.Contains(got, `PASSWORD="01234"`) {
		t.Errorf("PASSWORD with leading zeros was corrupted:\n%s", got)
	}
}

func TestAppendMultiple_PreservesBlankLines(t *testing.T) {
	original := "FOO=1\n\nBAR=2\n\n# section\nBAZ=3\n"
	p := writeTempEnv(t, original)

	if err := AppendMultiple(p, map[string]string{"BAR": "updated"}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	// Should still contain blank lines.
	if !strings.Contains(got, "\n\n") {
		t.Errorf("blank lines were removed:\n%q", got)
	}
	if !strings.Contains(got, "# section") {
		t.Errorf("comment lost:\n%s", got)
	}
}

func TestAppendMultiple_ValueWithSpecialChars(t *testing.T) {
	p := writeTempEnv(t, "")

	val := `pass word#with$pecial"chars`
	if err := AppendMultiple(p, map[string]string{"SECRET": val}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	// Should be quoted and escaped.
	if !strings.Contains(got, "SECRET=") {
		t.Errorf("expected SECRET= in output:\n%s", got)
	}
	// The value should be double-quoted with internal quotes escaped.
	if !strings.Contains(got, `\"`) {
		t.Errorf("expected escaped double-quote in output:\n%s", got)
	}
}

func TestAppendMultiple_FileWithNoTrailingNewline(t *testing.T) {
	// File does not end with \n — ensure we handle this gracefully.
	original := "FOO=bar"
	p := writeTempEnv(t, original)

	if err := AppendMultiple(p, map[string]string{"NEW": "val"}); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)
	if !strings.Contains(got, "FOO=bar") {
		t.Errorf("expected FOO=bar preserved:\n%s", got)
	}
	if !strings.Contains(got, "NEW=val") {
		t.Errorf("expected NEW=val appended:\n%s", got)
	}
	// Should end with a newline.
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected trailing newline, got:\n%q", got)
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestAppendMultiple_ConcurrentSafety(t *testing.T) {
	p := writeTempEnv(t, "BASE=original\n")

	var wg sync.WaitGroup
	n := 20
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			key := "KEY_" + strings.Repeat("A", i%5)
			// Use a unique-ish key per goroutine group.
			key = key + "_" + string(rune('A'+i%26))
			_ = AppendMultiple(p, map[string]string{key: "value"})
		}(i)
	}
	wg.Wait()

	got := readFile(t, p)
	// BASE should still be present.
	if !strings.Contains(got, "BASE=original") {
		t.Errorf("BASE=original was lost after concurrent writes:\n%s", got)
	}
	// File should be valid (no corruption, ends with newline).
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("file doesn't end with newline after concurrent writes:\n%q", got)
	}
}

// ---------------------------------------------------------------------------
// Full round-trip: simulates the generateTestSecrets pattern
// ---------------------------------------------------------------------------

func TestAppendMultiple_GenerateTestSecretsPattern(t *testing.T) {
	// Simulates a realistic .env file that generateTestSecrets would write to.
	original := `# Camunda Platform configuration
CAMUNDA_VERSION=8.9
CHART_PATH=./charts/camunda-platform

# Deployment settings
export NAMESPACE=camunda-test
CLUSTER_NAME=my-cluster

# These will be auto-generated:
# DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=
`
	p := writeTempEnv(t, original)

	secrets := map[string]string{
		"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD":  "aB1cD2eF3gH4",
		"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD": "xY5zW6vU7tS8",
		"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD":  "mN9oP0qR1sT2",
		"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET":      "kL3jH4gF5dS6",
	}

	if err := AppendMultiple(p, secrets); err != nil {
		t.Fatalf("AppendMultiple: %v", err)
	}

	got := readFile(t, p)

	// All original content preserved.
	if !strings.Contains(got, "# Camunda Platform configuration") {
		t.Error("header comment lost")
	}
	if !strings.Contains(got, "CAMUNDA_VERSION=8.9") {
		t.Error("CAMUNDA_VERSION lost")
	}
	if !strings.Contains(got, "export NAMESPACE=camunda-test") {
		t.Error("export NAMESPACE lost")
	}
	if !strings.Contains(got, "CLUSTER_NAME=my-cluster") {
		t.Error("CLUSTER_NAME lost")
	}

	// All 4 secrets present.
	for k, v := range secrets {
		if !strings.Contains(got, k+"="+v) {
			t.Errorf("missing %s=%s in:\n%s", k, v, got)
		}
	}

	// Key order: original keys appear before the new secrets.
	camundaIdx := strings.Index(got, "CAMUNDA_VERSION=")
	firstPwdIdx := strings.Index(got, "DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD=")
	if camundaIdx >= firstPwdIdx {
		t.Error("CAMUNDA_VERSION should appear before secret keys")
	}
}

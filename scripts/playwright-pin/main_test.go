package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePkg(t *testing.T, root, chart, version string) string {
	t.Helper()
	dir := filepath.Join(root, "charts", "camunda-platform-"+chart, "test", "e2e")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "package.json")
	body := `{"devDependencies":{"@playwright/test":"` + version + `"}}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestRun_AgreesAcrossCharts(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, "8.9", "1.61.0")
	src := writePkg(t, root, "8.10", "1.61.0")

	var buf bytes.Buffer
	require.NoError(t, run(root, "", &buf))

	got := buf.String()
	assert.Contains(t, got, "playwright-version=1.61.0\n")
	assert.Contains(t, got, "playwright-source="+src+"\n")
}

func TestRun_DisagreementShowsFileAndVersion(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, "8.9", "1.60.0")
	writePkg(t, root, "8.10", "1.61.0")

	err := run(root, "", &bytes.Buffer{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "camunda-platform-8.9")
	assert.Contains(t, msg, "1.60.0")
	assert.Contains(t, msg, "camunda-platform-8.10")
	assert.Contains(t, msg, "1.61.0")
}

func TestRun_RejectsRange(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, "8.10", "^1.61.0")

	err := run(root, "", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exact version")
}

func TestRun_RejectsMissing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "charts", "camunda-platform-8.10", "test", "e2e")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644))

	err := run(root, "", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not declare @playwright/test")
}

func TestRun_NoCharts(t *testing.T) {
	err := run(t.TempDir(), "", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no charts/")
}

func TestRun_FallsBackToDependencies(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "charts", "camunda-platform-8.10", "test", "e2e")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := `{"dependencies":{"@playwright/test":"1.61.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644))

	var buf bytes.Buffer
	require.NoError(t, run(root, "", &buf))
	assert.Contains(t, buf.String(), "playwright-version=1.61.0")
}

func TestRun_CopiesChosenSource(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, "8.9", "1.61.0")
	writePkg(t, root, "8.10", "1.61.0")
	dst := filepath.Join(root, "out", "e2e-package.json")

	require.NoError(t, run(root, dst, &bytes.Buffer{}))
	b, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"@playwright/test":"1.61.0"`)
}

func TestChooseHighestVersion_NumericOrder(t *testing.T) {
	files := []string{
		"/x/charts/camunda-platform-8.10/test/e2e/package.json",
		"/x/charts/camunda-platform-8.9/test/e2e/package.json",
		"/x/charts/camunda-platform-8.8/test/e2e/package.json",
	}
	got := chooseHighestVersion(files)
	assert.True(t, strings.Contains(got, "camunda-platform-8.10"), "want 8.10 highest, got %s", got)
}

func TestChooseHighestVersion_IgnoresRepoDirPrefix(t *testing.T) {
	// Path includes the repo dir `camunda-platform-helm`, which shares a prefix
	// with the chart dirs. The selector must skip it.
	files := []string{
		"/u/work/camunda-platform-helm/charts/camunda-platform-8.10/test/e2e/package.json",
		"/u/work/camunda-platform-helm/charts/camunda-platform-8.9/test/e2e/package.json",
	}
	got := chooseHighestVersion(files)
	assert.True(t, strings.Contains(got, "camunda-platform-8.10/test"), "want 8.10 highest, got %s", got)
}

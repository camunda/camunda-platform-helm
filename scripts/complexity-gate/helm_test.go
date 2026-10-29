// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestHelmBudgetComparison(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name           string
		value, budget  int
		changed, fails bool
	}{
		{"unchanged debt", 12, 10, false, false},
		{"changed debt", 12, 10, true, true},
		{"still above budget", 11, 10, true, true},
		{"new above budget", 11, 10, true, true},
		{"new at budget", 10, 10, true, false},
		{"edited below budget", 9, 10, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.fails, helmExceedsBudget(test.value, test.budget, test.changed))
		})
	}
}

func helmHelperSource(name string, branches int) string {
	return fmt.Sprintf(`{{define %q}}`, name) +
		strings.Repeat(`{{if .x}}old{{end}}`, branches) + "{{end}}"
}

func newHelmRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	chart := "charts/camunda-platform-fixture"
	writeFile(t, root, chart+"/Chart.yaml", "apiVersion: v2\nname: fixture\nversion: 0.1.0\n")
	writeFile(t, root, chart+"/templates/resource.yaml", "kind: ConfigMap\n")
	writeFile(t, root, chart+"/templates/_helpers.tpl", helmHelperSource("large", maxHelmBranches+2))
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "--", "charts")
	gitRun(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "-qm", "test: initial templates")
	return root, chart
}

func TestHelmSnapshots(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		edit func(*testing.T, string, string)
		fail string
	}{
		{"unchanged debt", func(*testing.T, string, string) {}, ""},
		{"message edit", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl",
				strings.ReplaceAll(helmHelperSource("large", maxHelmBranches+2), "old", "new"))
		}, "large: branches"},
		{"same-count condition edit", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl",
				strings.ReplaceAll(helmHelperSource("large", maxHelmBranches+2), ".x", ".y"))
		}, "large: branches"},
		{"partial improvement", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl", helmHelperSource("large", maxHelmBranches+1))
		}, "large: branches"},
		{"edited within budget", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl", helmHelperSource("large", maxHelmBranches))
		}, ""},
		{"comments only", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl",
				strings.ReplaceAll(helmHelperSource("large", maxHelmBranches+2), "{{if", "{{/* note */}}{{if"))
		}, ""},
		{"expression formatting", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl",
				strings.ReplaceAll(helmHelperSource("large", maxHelmBranches+2), "{{if .x}}", "{{ if   .x }}"))
		}, ""},
		{"unrelated helper added", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl",
				helmHelperSource("large", maxHelmBranches+2)+helmHelperSource("small", 1))
		}, ""},
		{"staged regression", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/_helpers.tpl", helmHelperSource("large", maxHelmBranches+3))
			gitRun(t, root, "add", "--", chart)
		}, "large: branches"},
		{"untracked regression", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/new.tpl", helmHelperSource("new", maxHelmBranches+1))
		}, "new: branches new ->"},
		{"new root regression", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/new.yaml", strings.Repeat("{{if .x}}x{{end}}", maxHelmBranches+1))
		}, "<root>: branches new ->"},
		{"moved helper", func(t *testing.T, root, chart string) {
			require.NoError(t, os.Rename(filepath.Join(root, chart, "templates/_helpers.tpl"),
				filepath.Join(root, chart, "templates/moved.tpl")))
		}, ""},
		{"deleted helper", func(t *testing.T, root, chart string) {
			require.NoError(t, os.Remove(filepath.Join(root, chart, "templates/_helpers.tpl")))
		}, ""},
		{"malformed candidate", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/bad.tpl", "{{if .x}}")
		}, "unexpected EOF"},
		{"new nesting", func(t *testing.T, root, chart string) {
			writeFile(t, root, chart+"/templates/deep.tpl",
				strings.Repeat("{{if .x}}", maxHelmDepth+1)+"x"+strings.Repeat("{{end}}", maxHelmDepth+1))
		}, "depth new ->"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root, chart := newHelmRepo(t)
			test.edit(t, root, chart)
			var out bytes.Buffer
			err := checkHelm(root, "HEAD", []string{filepath.Join(root, chart)}, &out)
			if test.fail != "" {
				require.Error(t, err)
				assert.Contains(t, out.String()+err.Error(), test.fail)
				assert.NotContains(t, out.String(), "no over-budget")
			} else {
				require.NoError(t, err)
				assert.Contains(t, out.String(), "no over-budget new or edited bodies")
				assert.NotContains(t, out.String(), "large")
			}
		})
	}
}

func TestHelmBaselineErrorsAndInventory(t *testing.T) {
	t.Parallel()
	root, chart := newHelmRepo(t)
	var out bytes.Buffer
	require.ErrorContains(t, checkHelm(root, "missing-ref", []string{filepath.Join(root, chart)}, &out), "merge-base")
	assert.Empty(t, out.String())
	require.NoError(t, reportHelmCharts(root, []string{filepath.Join(root, chart)}, &out))
	assert.Contains(t, out.String(), fmt.Sprintf("large branches=%d depth=1", maxHelmBranches+2))
	writeFile(t, root, chart+"/templates/bad.tpl", "{{if .x}}")
	gitRun(t, root, "add", "--", chart)
	gitRun(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "-qm", "test: malformed baseline")
	require.NoError(t, os.Remove(filepath.Join(root, chart, "templates/bad.tpl")))
	require.ErrorContains(t, checkHelm(root, "HEAD", []string{filepath.Join(root, chart)}, &out), "baseline")
}

func TestHelmExecutableHelp(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	helmHelp(&out)
	assert.Contains(t, out.String(), fmt.Sprintf("branches <= %d; control depth <= %d", maxHelmBranches, maxHelmDepth))
	assert.Contains(t, out.String(), "Untouched legacy debt passes")
	require.ErrorContains(t, checkHelm(t.TempDir(), "HEAD", nil, &out), "at least one chart path")
}

func TestHelmMakeEntryPoint(t *testing.T) {
	t.Parallel()
	root, chart := newHelmRepo(t)
	content, err := os.ReadFile("../../Makefile")
	require.NoError(t, err)
	writeFile(t, root, "Makefile", string(content))
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, source := range append(files, "go.mod", "go.sum") {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		content, err := os.ReadFile(source)
		require.NoError(t, err)
		writeFile(t, root, filepath.Join("scripts/complexity-gate", source), string(content))
	}
	bin := filepath.Join(root, "bin")
	writeFile(t, root, "bin/helm", "#!/bin/sh\nprintf 'HELM_LINT_REACHED\\n'\n")
	require.NoError(t, os.Chmod(filepath.Join(bin, "helm"), 0o755))
	writeFile(t, root, "bin/complexity-gate", "#!/bin/sh\nprintf 'UNRESOLVED_GO_BINARY\\n' >&2\nexit 127\n")
	require.NoError(t, os.Chmod(filepath.Join(bin, "complexity-gate"), 0o755))
	invoke := func(target, charts string, args ...string) (string, error) {
		cmd := exec.Command("make", append([]string{"--no-print-directory", target, "chartPath=" + charts, "lintBase=HEAD"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOBIN="+filepath.Join(root, "installed-bin"), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"HELM_TEMPLATE_BASE_REF=")
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	output, err := invoke("helm.template-complexity", chart, "templateCheckArgs=-help")
	require.NoError(t, err, output)
	assert.Contains(t, output, fmt.Sprintf("branches <= %d; control depth <= %d", maxHelmBranches, maxHelmDepth))
	output, err = invoke("helm.lint", chart)
	require.NoError(t, err, output)
	assert.Contains(t, output, "HELM_LINT_REACHED")
	writeFile(t, root, chart+"/templates/_helpers.tpl", helmHelperSource("large", maxHelmBranches+3))
	output, err = invoke("helm.template-complexity", chart)
	require.Error(t, err)
	assert.Contains(t, output, "large: branches")
	assert.NotContains(t, output, "HELM_LINT_REACHED")
	output, err = invoke("helm.lint", chart)
	require.Error(t, err)
	assert.Contains(t, output, "large: branches")
	assert.NotContains(t, output, "HELM_LINT_REACHED")

	other := "charts/camunda-platform-second"
	writeFile(t, root, other+"/Chart.yaml", "apiVersion: v2\nname: second\nversion: 0.1.0\n")
	writeFile(t, root, other+"/templates/resource.yaml", "kind: ConfigMap\n")
	writeFile(t, root, "bin/find", fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%s' '%s'\n", chart, other))
	require.NoError(t, os.Chmod(filepath.Join(bin, "find"), 0o755))
	output, err = invoke("helm.lint-all", "charts/camunda-platform-*")
	require.Error(t, err)
	assert.Contains(t, output, "large: branches")
	assert.NotContains(t, output, "HELM_LINT_REACHED")
}

func TestHelmGlobalLintWiring(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile("../../.github/workflows/repo-lint-global.yaml")
	require.NoError(t, err)
	var workflow struct {
		On   map[string]any `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Run  string            `yaml:"run"`
				Uses string            `yaml:"uses"`
				Env  map[string]string `yaml:"env"`
				With map[string]any    `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(content, &workflow))
	assert.Contains(t, workflow.On, "pull_request")
	assert.Contains(t, workflow.On, "merge_group")
	require.Contains(t, workflow.Jobs, "template-complexity")
	var invokes, fullHistory bool
	for _, step := range workflow.Jobs["template-complexity"].Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") {
			fullHistory = step.With["fetch-depth"] == 0
		}
		if step.Run == "make helm.template-complexity" {
			invokes = true
			assert.Contains(t, step.Env["HELM_TEMPLATE_BASE_REF"], "pull_request.base.sha")
			assert.Contains(t, step.Env["HELM_TEMPLATE_BASE_REF"], "merge_group.base_sha")
		}
	}
	assert.True(t, invokes, "global lint must call the shared Make target")
	assert.True(t, fullHistory, "the merge-base comparison requires Git history")
}

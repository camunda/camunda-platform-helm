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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func branchyScript(branches int) string {
	var b strings.Builder
	for i := range branches {
		fmt.Fprintf(&b, "if [ \"$X\" = %d ]; then echo %d; fi\n", i, i)
	}
	return b.String()
}

func longScript(lines int) string {
	return strings.Repeat("echo step\n", lines)
}

func step(name, script string) string {
	indented := "          " + strings.ReplaceAll(strings.TrimSuffix(script, "\n"), "\n", "\n          ")
	return fmt.Sprintf("      - name: %s\n        run: |\n%s\n", name, indented)
}

func workflow(steps ...string) string {
	return "on: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n" + strings.Join(steps, "")
}

func action(name, script string) string {
	return "runs:\n  using: composite\n  steps:\n" + step(name, script)
}

func TestRunAllReportsRunBlocks(t *testing.T) {
	root := newRepo(t, map[string]string{
		".github/workflows/a.yaml": workflow(
			step("Small", "echo ok"),
			step("Branchy", branchyScript(10)),
			step("Long", longScript(101)),
			"      - uses: actions/checkout@v6\n",
		),
		".github/actions/setup/action.yml": action("vars", branchyScript(11)),
		".github/workflows/README.md":      "run: not yaml\n",
		"other/workflow.yaml":              workflow(step("Elsewhere", branchyScript(20))),
	})

	var out bytes.Buffer
	err := run(root, "main", true, &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), ".github/workflows/a.yaml:10: jobs/build/steps/Branchy: cyclomatic 11 > 10")
	assert.Contains(t, out.String(), ".github/workflows/a.yaml:22: jobs/build/steps/Long: 101 lines > 100")
	assert.Contains(t, out.String(), ".github/actions/setup/action.yml:5: runs/steps/vars: cyclomatic 12 > 10")
	assert.NotContains(t, out.String(), "Small")
	assert.NotContains(t, out.String(), "Elsewhere")
	assert.Contains(t, err.Error(), "3 of 4 checked run block(s) above cyclomatic 10, 100 lines")
}

func TestRunAllCountsFoldedRunBlocksBySourceLines(t *testing.T) {
	folded := strings.Replace(step("Folded", longScript(101)), "run: |", "run: >", 1)
	root := newRepo(t, map[string]string{".github/workflows/a.yaml": workflow(folded, step("After", longScript(100)))})

	var out bytes.Buffer
	require.Error(t, run(root, "main", true, &out))
	assert.Contains(t, out.String(), "jobs/build/steps/Folded: 101 lines > 100")
	assert.NotContains(t, out.String(), "After")
}

func TestRunAllPassesRunBlocksAtTheLimits(t *testing.T) {
	root := newRepo(t, map[string]string{
		".github/workflows/a.yaml": workflow(step("Branchy", branchyScript(9)), step("Long", longScript(100))),
	})

	var out bytes.Buffer
	require.NoError(t, run(root, "main", true, &out))
	assert.Contains(t, out.String(), "complexity-gate: 2 run block(s) checked, none above cyclomatic 10, 100 lines")
}

func TestRunChecksAddedAndGrownRunBlocks(t *testing.T) {
	baseWorkflow := workflow(step("Small", branchyScript(1)), step("Branchy", branchyScript(12)), step("Long", longScript(101)))

	cases := []struct {
		name       string
		edit       map[string]string
		rename     [2]string
		wantErr    bool
		wantOut    []string
		notWantOut []string
	}{
		{
			name:    "no change",
			wantOut: []string{"0 run block(s) checked"},
		},
		{
			name:    "change below the limits passes even when the file holds over-limit blocks",
			edit:    map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(2)), step("Branchy", branchyScript(12)), step("Long", longScript(101)))},
			wantOut: []string{"3 run block(s) checked, none added or grown above cyclomatic 10, 100 lines"},
		},
		{
			name:    "edit inside an over-limit block that adds no branch passes",
			edit:    map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(1)), step("Branchy", strings.ReplaceAll(branchyScript(12), "echo", "printf")), step("Long", longScript(101)))},
			wantOut: []string{"none added or grown above"},
		},
		{
			name:    "shrinking an over-limit block passes",
			edit:    map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(1)), step("Branchy", branchyScript(11)), step("Long", longScript(101)))},
			wantOut: []string{"none added or grown above"},
		},
		{
			name:    "branch added to an over-limit block fails",
			edit:    map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(1)), step("Branchy", branchyScript(13)), step("Long", longScript(101)))},
			wantErr: true,
			wantOut: []string{".github/workflows/a.yaml:10: jobs/build/steps/Branchy: cyclomatic 14 > 10"},
		},
		{
			name:    "line added to an over-length block fails",
			edit:    map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(1)), step("Branchy", branchyScript(12)), step("Long", longScript(102)))},
			wantErr: true,
			wantOut: []string{"jobs/build/steps/Long: 102 lines > 100"},
		},
		{
			name:    "new over-limit step fails",
			edit:    map[string]string{".github/workflows/a.yaml": baseWorkflow + step("Added", branchyScript(10))},
			wantErr: true,
			wantOut: []string{"jobs/build/steps/Added: cyclomatic 11 > 10"},
		},
		{
			name:    "new untracked workflow with an over-limit block fails",
			edit:    map[string]string{".github/actions/new/action.yaml": action("Fresh", longScript(101))},
			wantErr: true,
			wantOut: []string{".github/actions/new/action.yaml:5: runs/steps/Fresh: 101 lines > 100"},
		},
		{
			name:    "renamed workflow keeps its over-limit blocks known",
			rename:  [2]string{".github/workflows/a.yaml", ".github/workflows/b.yaml"},
			wantOut: []string{"3 run block(s) checked, none added or grown above"},
		},
		{
			name:    "line added to an over-cyclomatic block fails",
			edit:    map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(1)), step("Branchy", branchyScript(12)+"echo done\n"), step("Long", longScript(101)))},
			wantErr: true,
			wantOut: []string{"jobs/build/steps/Branchy: cyclomatic 13 > 10, up from cyclomatic 13, 12 lines"},
		},
		{
			name:    "renamed step with an unchanged over-limit block passes",
			edit:    map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(1)), step("Renamed", branchyScript(12)), step("Long", longScript(101)))},
			wantOut: []string{"none added or grown above"},
		},
		{
			name:    "copy of an over-limit block in a new step fails",
			edit:    map[string]string{".github/workflows/a.yaml": baseWorkflow + step("Copy", branchyScript(12))},
			wantErr: true,
			wantOut: []string{"jobs/build/steps/Copy: cyclomatic 13 > 10"},
		},
		{
			name:       "renamed step and a copy of its over-limit block fail once",
			edit:       map[string]string{".github/workflows/a.yaml": workflow(step("Small", branchyScript(1)), step("Renamed", branchyScript(12)), step("Long", longScript(101)), step("Copy", branchyScript(12)))},
			wantErr:    true,
			wantOut:    []string{"jobs/build/steps/Copy: cyclomatic 13 > 10"},
			notWantOut: []string{"steps/Renamed"},
		},
		{
			name:    "workflow moved without a rename that git detects keeps its over-limit blocks known",
			edit:    map[string]string{".github/workflows/a.yaml": "", ".github/workflows/moved.yaml": baseWorkflow},
			wantOut: []string{"3 run block(s) checked, none added or grown above"},
		},
		{
			name:       "deleted workflow copied into two new workflows fails once",
			edit:       map[string]string{".github/workflows/a.yaml": "", ".github/workflows/b.yaml": baseWorkflow, ".github/workflows/c.yaml": baseWorkflow},
			wantErr:    true,
			wantOut:    []string{".github/workflows/c.yaml:10: jobs/build/steps/Branchy: cyclomatic 13 > 10", ".github/workflows/c.yaml:24: jobs/build/steps/Long: 101 lines > 100"},
			notWantOut: []string{"b.yaml"},
		},
		{
			name: "deleted workflow passes",
			edit: map[string]string{".github/workflows/a.yaml": ""},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRepo(t, map[string]string{".github/workflows/a.yaml": baseWorkflow})
			if tc.rename[0] != "" {
				gitRun(t, root, "mv", tc.rename[0], tc.rename[1])
			}
			for rel, content := range tc.edit {
				if content == "" {
					gitRun(t, root, "rm", "-q", rel)
					continue
				}
				writeFile(t, root, rel, content)
			}

			var out bytes.Buffer
			err := run(root, "main", false, &out)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "run block(s) added or grown above cyclomatic 10, 100 lines")
			} else {
				require.NoError(t, err)
			}
			for _, want := range tc.wantOut {
				assert.Contains(t, out.String(), want)
			}
			for _, notWant := range tc.notWantOut {
				assert.NotContains(t, out.String(), notWant)
			}
		})
	}
}

func TestRunMatchesStepsThatShareAName(t *testing.T) {
	baseWorkflow := workflow(step("Same", branchyScript(14)), step("Same", branchyScript(11)))

	cases := []struct {
		name    string
		edit    string
		wantErr bool
		wantOut string
	}{
		{"unchanged reorder passes", workflow(step("Same", branchyScript(11)), step("Same", branchyScript(14))), false, "none added or grown above"},
		{"shrunk block after a reorder passes", workflow(step("Same", branchyScript(11)), step("Same", branchyScript(13))), false, "none added or grown above"},
		{"grown block after a reorder fails", workflow(step("Same", branchyScript(11)), step("Same", branchyScript(15))), true, "jobs/build/steps/Same: cyclomatic 16 > 10, up from cyclomatic 15, 14 lines"},
		{"extra copy of an unchanged block fails", workflow(step("Same", branchyScript(14)), step("Same", branchyScript(11)), step("Same", branchyScript(11))), true, ".github/workflows/a.yaml:36: jobs/build/steps/Same: cyclomatic 12 > 10\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRepo(t, map[string]string{".github/workflows/a.yaml": baseWorkflow})
			writeFile(t, root, ".github/workflows/a.yaml", tc.edit)

			var out bytes.Buffer
			err := run(root, "main", false, &out)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Contains(t, out.String(), tc.wantOut)
		})
	}
}

func TestRunAllFailsOnUnparsableRunBlock(t *testing.T) {
	root := newRepo(t, map[string]string{".github/workflows/a.yaml": workflow(step("Broken", "if true; then echo"))})

	var out bytes.Buffer
	err := run(root, "main", true, &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".github/workflows/a.yaml: run block at line 7")
}

func TestRunAllChecksUntrackedRunBlocks(t *testing.T) {
	root := newRepo(t, map[string]string{".github/workflows/a.yaml": workflow(step("Small", "echo ok"))})
	writeFile(t, root, ".github/actions/new/action.yaml", action("Long", longScript(101)))

	var out bytes.Buffer
	err := run(root, "main", true, &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), ".github/actions/new/action.yaml:5: runs/steps/Long: 101 lines > 100")
	assert.Contains(t, err.Error(), "1 of 2 checked run block(s)")
}

func TestParseRunBlocksReadsOnlyStepRunKeys(t *testing.T) {
	blocks, err := parseRunBlocks([]byte("on: push\n" +
		"defaults:\n  run:\n    shell: bash\n" +
		"jobs:\n  build:\n    env:\n      run: \"if [\"\n    steps:\n" +
		"      - name: Named\n        id: ignored\n        run: echo a\n" +
		"      - id: only-id\n        run: echo b\n" +
		"      - run: echo c\n" +
		"      - uses: actions/example@v1\n        with:\n          run: \"if [\"\n",
	))
	require.NoError(t, err)
	assert.Equal(t, []runBlock{
		{step: "jobs/build/steps/Named", script: "echo a", line: 12, cyclomatic: 1, lines: 1},
		{step: "jobs/build/steps/only-id", script: "echo b", line: 14, cyclomatic: 1, lines: 1},
		{step: "jobs/build/steps/2", script: "echo c", line: 15, cyclomatic: 1, lines: 1},
	}, blocks)
}

func TestBashCyclomatic(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   int
	}{
		{"straight line", "echo a\necho b", 1},
		{"if elif else", "if a; then b; elif c; then d; else e; fi", 3},
		{"case items", "case $x in a) b ;; c) d ;; *) e ;; esac", 4},
		{"loops", "for i in 1 2; do a; done\nwhile a; do b; done\nuntil a; do b; done", 4},
		{"command lists", "a && b || c", 3},
		{"test expressions", "[[ -n $a && -n $b || -n $c ]]", 3},
		{"arithmetic commands", "(( a >= 0 && a <= 9 || b ))", 3},
		{"arithmetic expansions", "echo $(( a && b || c ))", 3},
		{"workflow expressions", `if [ "${{ inputs.flag }}" = "true" ]; then echo "${{ github.sha }}"; fi`, 2},
		{"multiline workflow expression", "echo \"${{ format('{0}',\n  inputs.flag) }}\"", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := measureRun(&yaml.Node{Kind: yaml.ScalarNode, Value: tc.script})
			require.NoError(t, err)
			assert.Equal(t, tc.want, b.cyclomatic)
		})
	}
}

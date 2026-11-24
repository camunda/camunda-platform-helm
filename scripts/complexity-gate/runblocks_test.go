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
)

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

func TestRunShellchecksCompositeActionRunBlocks(t *testing.T) {
	root := newRepo(t, map[string]string{".github/actions/clean/action.yaml": action("Clean", `echo "ok"`)})
	writeFile(t, root, ".github/actions/new/action.yaml", action("Unquoted", `if [ -n "${{ inputs.dir }}" ]; then cd $TARGET_DIR || exit 1; fi`))
	writeFile(t, root, ".github/workflows/w.yaml", workflow(step("Unquoted", "cd $TARGET_DIR || exit 1")))

	var out bytes.Buffer
	err := run(root, &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), ".github/actions/new/action.yaml:6: runs/steps/Unquoted: note: Double quote to prevent globbing and word splitting. [SC2086]")
	assert.NotContains(t, out.String(), "SC2157")
	assert.NotContains(t, out.String(), "w.yaml")
	assert.NotContains(t, out.String(), "clean/action.yaml")
	assert.Contains(t, err.Error(), "shellcheck reports 1 finding(s) in 2 checked composite action run block(s)")
}

func TestRunShellchecksInlineCompositeActionRunValue(t *testing.T) {
	root := newRepo(t, map[string]string{".github/actions/a/action.yaml": action("Clean", `echo "ok"`)})
	writeFile(t, root, ".github/actions/a/action.yaml", "runs:\n  using: composite\n  steps:\n    - name: Inline\n      run: cd $TARGET_DIR || exit 1\n")

	var out bytes.Buffer
	require.Error(t, run(root, &out))
	assert.Contains(t, out.String(), ".github/actions/a/action.yaml:5: runs/steps/Inline:")
}

func TestRunShellchecksMaskedExpressionsAtTheirSourceLines(t *testing.T) {
	root := newRepo(t, map[string]string{".github/actions/a/action.yaml": action("Clean", `echo "ok"`)})
	writeFile(t, root, ".github/actions/a/action.yaml", action("Masked", "echo '${{ github.sha }}' '$HOME'\necho \"${{\n  inputs.dir\n}}\"\ncd $TARGET_DIR || exit 1"))

	var out bytes.Buffer
	err := run(root, &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), ".github/actions/a/action.yaml:6: runs/steps/Masked: note: Expressions don't expand in single quotes")
	assert.Contains(t, out.String(), ".github/actions/a/action.yaml:10: runs/steps/Masked: note: Double quote to prevent globbing")
	assert.Contains(t, err.Error(), "shellcheck reports 2 finding(s)")
}

func TestRunShellchecksEveryCompositeActionRunBlock(t *testing.T) {
	root := newRepo(t, map[string]string{
		".github/actions/a/action.yaml": action("Clean", `echo "ok"`),
		".github/actions/b/action.yaml": action("Unquoted", "cd $TARGET_DIR || exit 1"),
	})

	var out bytes.Buffer
	err := run(root, &out)

	require.Error(t, err)
	assert.Contains(t, out.String(), ".github/actions/b/action.yaml:6: runs/steps/Unquoted:")
	assert.Contains(t, err.Error(), "1 finding(s) in 2 checked composite action run block(s)")
}

func TestRunPassesCompositeActionRunBlocksWithoutFinding(t *testing.T) {
	root := newRepo(t, map[string]string{".github/actions/a/action.yaml": action("Clean", `echo "ok"`)})

	var out bytes.Buffer
	require.NoError(t, run(root, &out))
	assert.Contains(t, out.String(), "1 composite action run block(s) checked, no shellcheck finding")
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
		{step: "jobs/build/steps/Named", script: "echo a", firstLine: 12},
		{step: "jobs/build/steps/only-id", script: "echo b", firstLine: 14},
		{step: "jobs/build/steps/2", script: "echo c", firstLine: 15},
	}, blocks)
}

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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeRunsPreviousMinorWhenChartContainsAllowlist(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	for _, optedIn := range []bool{true, false} {
		name := "without allowlist"
		if optedIn {
			name = "with allowlist"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			chart := filepath.Join(dir, "chart")
			bin := filepath.Join(dir, "bin")
			files := map[string]string{
				filepath.Join(chart, "values.schema.json"): "{}",
				filepath.Join(chart, "values.yaml"):        "{}",
				filepath.Join(bin, "readme-generator"):     "#!/bin/bash\nfor arg; do output=\"$arg\"; done\nprintf '{}' > \"$output\"\n",
				filepath.Join(bin, "go"):                   "#!/bin/bash\nprintf '%s\\n' \"$*\"\n",
			}
			if optedIn {
				files[filepath.Join(chart, "test/unit/deprecation/allowlist.yaml")] = "{}"
			}
			for path, content := range files {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
				require.NoError(t, os.WriteFile(path, []byte(content), 0o700))
			}
			relative, err := filepath.Rel(root, chart)
			require.NoError(t, err)
			cmd := exec.Command("make", "helm.schema-validate-values", "chartPath="+relative)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TMPDIR="+dir)

			output, err := cmd.CombinedOutput()

			require.NoError(t, err, "%s", output)
			if optedIn {
				require.Contains(t, string(output), "--previous-minor")
			} else {
				require.NotContains(t, string(output), "--previous-minor")
			}
		})
	}
}

func TestMakeInstallsReadmeGeneratorFromSharedPin(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("make", "-n", "install.readme-generator", "readmeGeneratorVersion=99.0.0")
	cmd.Dir = "../.."

	output, err := cmd.CombinedOutput()

	require.NoError(t, err, "%s", output)
	require.Contains(t, string(output), "npm install -g @bitnami/readme-generator-for-helm@99.0.0")
}

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

func TestUpgradeCLIWhenPreviousDefaultsContainRejectedKeys(t *testing.T) {
	t.Parallel()
	for _, covered := range []bool{true, false} {
		name := "uncovered fails"
		if covered {
			name = "covered passes"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			previous := filepath.Join(dir, "camunda-platform-8.9")
			current := filepath.Join(dir, "camunda-platform-8.10")
			files := map[string]string{
				filepath.Join(previous, "values.schema.json"):                  `{}`,
				filepath.Join(previous, "values.yaml"):                         "old: true\nremoved: true\nexception: true\n",
				filepath.Join(previous, "values-enterprise.yaml"):              "old: false\n",
				filepath.Join(current, "Chart.yaml"):                           "apiVersion: v2\nname: test\n",
				filepath.Join(current, "values.schema.json"):                   `{"type":"object","properties":{}}`,
				filepath.Join(current, "templates/common/constraints.tpl"):     `{{ include "camundaPlatform.keyRemoved" (dict "condition" true "oldName" "removed") }}`,
				filepath.Join(current, "test/unit/deprecation/allowlist.yaml"): `exception: test exception`,
			}
			if covered {
				files[filepath.Join(current, "templates/common/constraints.tpl")] += `{{ include "camundaPlatform.keyDeprecated" (dict "condition" true "oldName" "old" "migration" "new") }}`
			}
			for path, contents := range files {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
				require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
			}
			binary, err := os.Executable()
			require.NoError(t, err)
			cmd := exec.Command(binary, "-test.run=^TestUpgradeCLIProcess$", "--", "--schema", filepath.Join(current, "values.schema.json"), "--chart-dir", current, "--previous-minor")
			cmd.Env = append(os.Environ(), "VALIDATE_VALUES_SCHEMA_PROCESS=1")

			output, err := cmd.CombinedOutput()

			if covered {
				require.NoError(t, err, "%s", output)
				require.Contains(t, string(output), "deprecated=1 removed=1 allowlisted=1 uncovered=0")
			} else {
				var exit *exec.ExitError
				require.ErrorAs(t, err, &exit)
				require.Equal(t, 1, exit.ExitCode())
				require.Contains(t, string(output), "deprecated=0 removed=1 allowlisted=1 uncovered=1")
			}
			require.Contains(t, string(output), "values-enterprise.yaml")
			data, err := os.ReadFile(filepath.Join(current, "values.schema.json"))
			require.NoError(t, err)
			require.JSONEq(t, `{"type":"object","properties":{}}`, string(data))
		})
	}
}

func TestUpgradeCLIProcess(t *testing.T) {
	if os.Getenv("VALIDATE_VALUES_SCHEMA_PROCESS") != "1" {
		return
	}
	for index, arg := range os.Args {
		if arg == "--" {
			os.Args = append(os.Args[:1], os.Args[index+1:]...)
			break
		}
	}
	main()
	os.Exit(0)
}

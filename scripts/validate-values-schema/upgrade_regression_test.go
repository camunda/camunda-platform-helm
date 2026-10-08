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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpgradeCoversArrayItemRemovalWhenRegistrationIsUnindexed(t *testing.T) {
	t.Parallel()
	coverage, err := parseUpgradeCoverage(`{{ include "camundaPlatform.keyRemoved" (dict "condition" true "oldName" "env.oldField") }}`, "")
	require.NoError(t, err)
	schema := strictify(mustJSON(t, `{"type":"object","properties":{"env":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`))
	values := mustYAMLMap(t, `{"env":[{"name":"first"},{"name":"second","oldField":true}]}`)
	var output bytes.Buffer

	failures := coverage.report(&output, "previous/values.yaml", coverage.findUnknownKeys(schema, values))

	require.Zero(t, failures, "%s", output.String())
	require.Contains(t, output.String(), "removed: env[1].oldField")
}

func TestUpgradeClassifiesArrayPathsWhenCoverageIsNested(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, key, registered, allowed string
		want                           coverageClass
	}{
		{"nested indexes", "env[12].children[3].oldField", "env.children.oldField", "", removed},
		{"indexed registration", "env[1].oldField", "env[1].oldField", "", removed},
		{"wildcard", "env[1].oldField", "env.*", "", removed},
		{"allowlist", "env[1].oldField", "env.*", "env.oldField", allowlisted},
		{"uncovered sibling", "env[1].other", "env.oldField", "", uncovered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			coverage := upgradeCoverage{keys: map[string]coverageClass{tc.registered: removed}, allowlist: map[string]string{}}
			if tc.allowed != "" {
				coverage.allowlist[tc.allowed] = "exception"
			}

			class := coverage.classify(tc.key)

			require.Equal(t, tc.want, class)
		})
	}
}

func TestUpgradeCLIRejectsMissingSchemaWhenPreviousChartExists(t *testing.T) {
	t.Parallel()
	for _, exists := range []bool{true, false} {
		name := "absent previous chart skips"
		if exists {
			name = "existing previous chart requires schema"
		}
		t.Run(name, func(t *testing.T) {
			current := filepath.Join(t.TempDir(), "camunda-platform-99.10")
			previous, err := previousMinorChart(current)
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(current, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(current, "Chart.yaml"), []byte("apiVersion: v2\nname: test\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(current, "values.schema.json"), []byte(`{}`), 0o600))
			if exists {
				require.NoError(t, os.MkdirAll(previous, 0o700))
			}
			binary, err := os.Executable()
			require.NoError(t, err)
			cmd := exec.Command(binary, "-test.run=^TestUpgradeCLIProcess$", "--", "--schema", filepath.Join(current, "values.schema.json"), "--chart-dir", current, "--previous-minor")
			cmd.Env = append(os.Environ(), "VALIDATE_VALUES_SCHEMA_PROCESS=1")

			output, err := cmd.CombinedOutput()

			if exists {
				var exit *exec.ExitError
				require.ErrorAs(t, err, &exit, "%s", output)
				require.Equal(t, 2, exit.ExitCode())
				require.Contains(t, string(output), "values.schema.json")
			} else {
				require.NoError(t, err, "%s", output)
				require.NotContains(t, string(output), "Upgrade coverage")
			}
		})
	}
}

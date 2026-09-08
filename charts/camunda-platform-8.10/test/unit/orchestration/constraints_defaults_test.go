// Copyright 2022 Camunda Services GmbH
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

package orchestration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The zoned-mode guard in templates/common/constraints.tpl treats orchestration.clusterSize and
// orchestration.replicationFactor as untouched while they still hold the chart default, because
// Helm cannot distinguish a supplied value from a default. That default is written there as a
// literal: .Chart carries Chart.yaml only, and .Files excludes values.yaml and values.schema.json,
// so nothing exposes it at render time. This test is what keeps the two in step — change a default
// without changing the guard and it fails here instead of quietly turning the guard into a no-op.
const zonedModeGuardChartDefault = "3"

func TestZonedModeGuardMatchesChartDefaults(t *testing.T) {
	t.Parallel()

	valuesPath, err := filepath.Abs("../../../values.yaml")
	require.NoError(t, err)

	raw, err := os.ReadFile(valuesPath)
	require.NoError(t, err)

	var values struct {
		Orchestration struct {
			ClusterSize       string `yaml:"clusterSize"`
			ReplicationFactor string `yaml:"replicationFactor"`
		} `yaml:"orchestration"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &values))

	for _, tc := range []struct {
		key    string
		actual string
	}{
		{key: "orchestration.clusterSize", actual: values.Orchestration.ClusterSize},
		{key: "orchestration.replicationFactor", actual: values.Orchestration.ReplicationFactor},
	} {
		require.Equalf(t, zonedModeGuardChartDefault, tc.actual,
			"%s default is %q but the zoned-mode guard in templates/common/constraints.tpl compares against %q; "+
				"update the literal in that guard and this constant together",
			tc.key, tc.actual, zonedModeGuardChartDefault)
	}
}

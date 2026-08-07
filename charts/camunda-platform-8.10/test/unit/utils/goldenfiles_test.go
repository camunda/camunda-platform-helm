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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoldenValuesClonesInput(t *testing.T) {
	t.Parallel()

	input := map[string]string{"provided": "original"}
	values := goldenValues(input)

	require.Equal(t, map[string]string{"provided": "original"}, input)
	require.Equal(t, map[string]string{
		"connectors.security.authentication.oidc.secret.existingSecret":       "camunda-credentials",
		"connectors.security.authentication.oidc.secret.existingSecretKey":    "client-secret",
		"global.identity.auth.console.existingSecret.name":                    "camunda-credentials",
		"global.identity.auth.optimize.secret.existingSecret":                 "camunda-credentials",
		"global.identity.auth.optimize.secret.existingSecretKey":              "identity-optimize-client-token",
		"orchestration.data.secondaryStorage.type":                            "elasticsearch",
		"orchestration.security.authentication.oidc.secret.existingSecret":    "camunda-credentials",
		"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
		"provided": "original",
	}, values)

	values["provided"] = "changed"
	require.Equal(t, "original", input["provided"])
}

func TestGoldenValuesPreservesStorageType(t *testing.T) {
	t.Parallel()

	values := goldenValues(map[string]string{
		"orchestration.data.secondaryStorage.type": "opensearch",
	})

	require.Equal(t, "opensearch", values["orchestration.data.secondaryStorage.type"])
}

func TestGoldenValuesDoesNotOverrideStorageSelectors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{name: "no secondary storage", key: "global.noSecondaryStorage", value: "true"},
		{name: "RDBMS", key: "orchestration.exporters.rdbms.enabled", value: "true"},
		{name: "Optimize Elasticsearch", key: "optimize.database.elasticsearch.enabled", value: "true"},
		{name: "Optimize OpenSearch", key: "optimize.database.opensearch.enabled", value: "true"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			values := goldenValues(map[string]string{testCase.key: testCase.value})

			require.NotContains(t, values, "orchestration.data.secondaryStorage.type")
		})
	}
}

func TestGoldenValuesDefaultsStorageWhenSelectorsAreDisabled(t *testing.T) {
	t.Parallel()

	values := goldenValues(map[string]string{
		"global.noSecondaryStorage":               "false",
		"orchestration.exporters.rdbms.enabled":   "false",
		"optimize.database.elasticsearch.enabled": "false",
		"optimize.database.opensearch.enabled":    "false",
	})

	require.Equal(t, "elasticsearch", values["orchestration.data.secondaryStorage.type"])
}

func TestGoldenIgnoredLinesClonesInput(t *testing.T) {
	t.Parallel()

	input := make([]string, 1, 4)
	input[0] = "caller-pattern"
	ignoredLines := goldenIgnoredLines(input)

	require.Equal(t, []string{"caller-pattern"}, input)
	require.Equal(t, []string{"caller-pattern", `\s+helm.sh/chart:\s+.*`}, ignoredLines)

	ignoredLines[0] = "changed"
	require.Equal(t, "caller-pattern", input[0])
}

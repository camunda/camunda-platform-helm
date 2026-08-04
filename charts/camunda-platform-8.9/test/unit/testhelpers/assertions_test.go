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

package testhelpers

import (
	"flag"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ = flag.Bool("update-golden", false, "accepted for chart-wide golden updates")

func TestDeclarativeAssertions(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateTestCase(TestCase{Expected: map[string]string{"metadata.name": "test"}}))
	require.NoError(t, validateTestCase(TestCase{Unexpected: []string{"metadata.annotations.optional"}}))
	require.ErrorContains(t, validateTestCase(TestCase{}), "must declare an assertion")
	require.ErrorContains(t, validateTestCase(TestCase{Expected: map[string]string{"ERROR": ""}}), "must not be empty")

	objects, err := decodeRenderedObjects(`
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    checksum/ca-bundle: abc123
spec:
  emptyString: ""
  nullValue: null
  emptyMap: {}
  env:
    - name: TEST_VALUE
      value: present
---
apiVersion: v1
kind: Service
metadata:
  name: test
`)
	require.NoError(t, err)

	resolved, err := resolveScalarPath(objects, "spec.env[?(@.name=='TEST_VALUE')].value")
	require.NoError(t, err)
	require.Equal(t, "present", resolved)
	resolved, err = resolveScalarPath(objects, "metadata.annotations.checksum/ca-bundle")
	require.NoError(t, err)
	require.Equal(t, "abc123", resolved)
	resolved, err = resolveScalarPath(objects, "spec.emptyString")
	require.NoError(t, err)
	require.Equal(t, "", resolved)
	resolved, err = resolveScalarPath(objects, "spec.nullValue")
	require.NoError(t, err)
	require.Equal(t, "null", resolved)

	matches, err := findPathMatches(objects, "spec.missing")
	require.NoError(t, err)
	require.Empty(t, matches)
	_, err = resolveScalarPath(objects, "kind")
	require.ErrorContains(t, err, "resolved to 2 values")
	_, err = resolveScalarPath(objects, "spec.emptyMap")
	require.ErrorContains(t, err, "must resolve to a scalar")
}

func TestSetupHelmOptionsUsesCurrentStorageFixtures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		values           map[string]string
		expectedSetValue string
	}{
		{name: "defaults to Elasticsearch", expectedSetValue: "elasticsearch"},
		{
			name: "derives OpenSearch from Optimize",
			values: map[string]string{
				"optimize.database.opensearch.enabled": "true",
			},
			expectedSetValue: "opensearch",
		},
		{
			name: "prefers Elasticsearch when both Optimize selectors are enabled",
			values: map[string]string{
				"optimize.database.elasticsearch.enabled": "true",
				"optimize.database.opensearch.enabled":    "true",
			},
			expectedSetValue: "elasticsearch",
		},
		{
			name: "preserves explicit current type",
			values: map[string]string{
				"orchestration.data.secondaryStorage.type": "rdbms",
			},
			expectedSetValue: "rdbms",
		},
		{
			name: "does not override explicit compatibility selector",
			values: map[string]string{
				"global.elasticsearch.enabled": "false",
			},
		},
		{
			name: "does not override no secondary storage",
			values: map[string]string{
				"global.noSecondaryStorage": "true",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			original := maps.Clone(testCase.values)

			options, err := setupHelmOptions("test", testCase.values, nil, nil)
			require.NoError(t, err)
			require.Equal(t, original, testCase.values)
			if testCase.expectedSetValue == "" {
				require.NotContains(t, options.SetValues, "orchestration.data.secondaryStorage.type")
			} else {
				require.Equal(t, testCase.expectedSetValue, options.SetValues["orchestration.data.secondaryStorage.type"])
			}
			require.NotContains(t, options.SetValues, "elasticsearch.enabled")
		})
	}
}

func TestSetupHelmOptionsHonorsValuesFileStorageSelection(t *testing.T) {
	t.Parallel()

	valuesFile := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(valuesFile, []byte(`orchestration:
  data:
    secondaryStorage:
      type: opensearch
`), 0o600))

	options, err := setupHelmOptions("test", nil, []string{valuesFile}, nil)
	require.NoError(t, err)
	require.NotContains(t, options.SetValues, "orchestration.data.secondaryStorage.type")
	require.NotContains(t, options.SetValues, "global.elasticsearch.enabled")
}

func TestSetupHelmOptionsPreservesValuesFileStoragePrecedence(t *testing.T) {
	t.Parallel()

	valuesFile := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(valuesFile, []byte(`optimize:
  database:
    elasticsearch:
      enabled: true
    opensearch:
      enabled: true
`), 0o600))

	options, err := setupHelmOptions("test", nil, []string{valuesFile}, nil)
	require.NoError(t, err)
	require.Equal(t, "elasticsearch", options.SetValues["orchestration.data.secondaryStorage.type"])
	require.NotContains(t, options.SetValues, "global.elasticsearch.enabled")
}

func TestValidateStorageFixtureRenderArgs(t *testing.T) {
	t.Parallel()

	require.ErrorContains(
		t,
		validateStorageFixtureRenderArgs([]string{"--set", "optimize.database.elasticsearch.enabled=true"}),
		"optimize.database.elasticsearch.enabled must be provided through TestCase.Values",
	)
	require.ErrorContains(
		t,
		validateStorageFixtureRenderArgs([]string{"-fstorage.yaml"}),
		"values files must be provided through TestCase.ValuesFiles",
	)
	require.NoError(t, validateStorageFixtureRenderArgs([]string{
		"--set-string", "orchestration.env[0].value=true",
	}))
}

func TestSetupHelmOptionsHonorsLayeredNullOverrides(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		baseYAML    string
		overlayYAML string
	}{
		{
			name: "clears explicit storage type",
			baseYAML: `orchestration:
  data:
    secondaryStorage:
      type: opensearch
`,
			overlayYAML: `orchestration:
  data:
    secondaryStorage:
      type: null
`,
		},
		{
			name: "clears legacy selector",
			baseYAML: `global:
  elasticsearch:
    enabled: false
`,
			overlayYAML: `global:
  elasticsearch:
    enabled: null
`,
		},
		{
			name: "clears no-secondary-storage parent",
			baseYAML: `global:
  noSecondaryStorage: true
`,
			overlayYAML: `global: null
`,
		},
		{
			name: "clears RDBMS exporter parent",
			baseYAML: `orchestration:
  exporters:
    rdbms:
      enabled: true
`,
			overlayYAML: `orchestration:
  exporters: null
`,
		},
		{
			name: "clears Optimize OpenSearch selection",
			baseYAML: `optimize:
  database:
    opensearch:
      enabled: true
`,
			overlayYAML: `optimize:
  database:
    opensearch:
      enabled: null
`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tempDir := t.TempDir()
			baseFile := filepath.Join(tempDir, "base.yaml")
			overlayFile := filepath.Join(tempDir, "overlay.yaml")
			require.NoError(t, os.WriteFile(baseFile, []byte(testCase.baseYAML), 0o600))
			require.NoError(t, os.WriteFile(overlayFile, []byte(testCase.overlayYAML), 0o600))

			options, err := setupHelmOptions("test", nil, []string{baseFile, overlayFile}, nil)
			require.NoError(t, err)
			require.Equal(t, "elasticsearch", options.SetValues["orchestration.data.secondaryStorage.type"])
		})
	}
}

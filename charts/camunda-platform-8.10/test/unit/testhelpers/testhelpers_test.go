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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ = flag.Bool("update-golden", false, "accepted for chart-wide golden updates")

func TestValidateTestCase(t *testing.T) {
	t.Parallel()

	validCases := []TestCase{
		{Verifier: func(*testing.T, string, error) {}},
		{Expected: map[string]string{"ERROR": "render failed"}},
		{Expected: map[string]string{"metadata.name": "test"}},
		{Unexpected: []string{"metadata.annotations.optional"}},
		{
			ExpectedObject: &struct{}{},
			ObjectAsserter: func(*testing.T, any) {},
		},
	}
	for _, testCase := range validCases {
		require.NoError(t, validateTestCase(testCase))
	}

	invalidCases := []struct {
		name        string
		testCase    TestCase
		expectedErr string
	}{
		{
			name:        "assertion-free",
			testCase:    TestCase{},
			expectedErr: "must declare an assertion",
		},
		{
			name: "empty error assertion",
			testCase: TestCase{
				Expected: map[string]string{"ERROR": "  "},
			},
			expectedErr: "ERROR assertion must not be empty",
		},
		{
			name: "verifier and declarative",
			testCase: TestCase{
				Verifier: func(*testing.T, string, error) {},
				Expected: map[string]string{"metadata.name": "test"},
			},
			expectedErr: "Verifier cannot be combined",
		},
		{
			name: "verifier and object",
			testCase: TestCase{
				Verifier:       func(*testing.T, string, error) {},
				ExpectedObject: &struct{}{},
				ObjectAsserter: func(*testing.T, any) {},
			},
			expectedErr: "Verifier cannot be combined",
		},
		{
			name: "error and output",
			testCase: TestCase{
				Expected: map[string]string{
					"ERROR":         "render failed",
					"metadata.name": "test",
				},
			},
			expectedErr: "ERROR assertion cannot be combined",
		},
		{
			name: "error and absence",
			testCase: TestCase{
				Expected:   map[string]string{"ERROR": "render failed"},
				Unexpected: []string{"metadata.name"},
			},
			expectedErr: "ERROR assertion cannot be combined",
		},
		{
			name: "declarative and object",
			testCase: TestCase{
				Expected:       map[string]string{"metadata.name": "test"},
				ExpectedObject: &struct{}{},
				ObjectAsserter: func(*testing.T, any) {},
			},
			expectedErr: "declarative assertions cannot be combined",
		},
		{
			name: "object without asserter",
			testCase: TestCase{
				ExpectedObject: &struct{}{},
			},
			expectedErr: "ExpectedObject and ObjectAsserter must be set together",
		},
		{
			name: "asserter without object",
			testCase: TestCase{
				ObjectAsserter: func(*testing.T, any) {},
			},
			expectedErr: "ExpectedObject and ObjectAsserter must be set together",
		},
	}

	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateTestCase(testCase.testCase)
			require.ErrorContains(t, err, testCase.expectedErr)
		})
	}
}

func TestRenderedPathResolution(t *testing.T) {
	t.Parallel()

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
  emptyList: []
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
	require.Len(t, objects, 2)

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

	_, err = resolveScalarPath(objects, "spec.missing")
	require.ErrorContains(t, err, "resolved to 0 values")

	_, err = resolveScalarPath(objects, "kind")
	require.ErrorContains(t, err, "resolved to 2 values")

	_, err = resolveScalarPath(objects, "spec.emptyMap")
	require.ErrorContains(t, err, "must resolve to a scalar")

	_, err = resolveScalarPath(objects, "spec.emptyList")
	require.ErrorContains(t, err, "must resolve to a scalar")

	_, err = resolveScalarPath(objects, "spec.env[")
	require.Error(t, err)
}

func TestRenderedPathAbsenceDistinguishesPresentStates(t *testing.T) {
	t.Parallel()

	objects, err := decodeRenderedObjects(`
apiVersion: v1
kind: ConfigMap
data:
  nullValue: null
  emptyString: ""
  emptyMap: {}
  emptyList: []
`)
	require.NoError(t, err)

	for _, path := range []string{"data.nullValue", "data.emptyString", "data.emptyMap", "data.emptyList"} {
		matches, err := findPathMatches(objects, path)
		require.NoError(t, err)
		require.Len(t, matches, 1, path)
	}

	matches, err := findPathMatches(objects, "data.missing")
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestSetupHelmOptionsClonesValues(t *testing.T) {
	t.Parallel()

	values := map[string]string{"provided": "original"}
	options, err := setupHelmOptions("test", values, nil, nil)
	require.NoError(t, err)

	require.Equal(t, map[string]string{"provided": "original"}, values)
	require.Equal(t, map[string]string{
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"provided": "original",
	}, options.SetValues)

	options.SetValues["provided"] = "changed"
	require.Equal(t, "original", values["provided"])
}

func TestSetupHelmOptionsUsesSupportedStorageDefaults(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name:     "defaults to Elasticsearch",
			expected: "elasticsearch",
		},
		{
			name: "derives OpenSearch from Optimize",
			values: map[string]string{
				"optimize.database.opensearch.enabled": "true",
			},
			expected: "opensearch",
		},
		{
			name: "prefers Elasticsearch when both Optimize selectors are enabled",
			values: map[string]string{
				"optimize.database.elasticsearch.enabled": "true",
				"optimize.database.opensearch.enabled":    "true",
			},
			expected: "elasticsearch",
		},
		{
			name: "preserves an explicit type",
			values: map[string]string{
				"orchestration.data.secondaryStorage.type": "rdbms",
			},
			expected: "rdbms",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			options, err := setupHelmOptions("test", testCase.values, nil, nil)
			require.NoError(t, err)

			expectedValues := make(map[string]string, len(testCase.values)+1)
			for key, value := range testCase.values {
				expectedValues[key] = value
			}
			expectedValues["orchestration.data.secondaryStorage.type"] = testCase.expected
			require.Equal(t, expectedValues, options.SetValues)
		})
	}
}

func TestSetupHelmOptionsHonorsValuesFileStorageSelection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		valuesYAML       string
		expectedSetValue string
	}{
		{
			name: "explicit OpenSearch type",
			valuesYAML: `orchestration:
  data:
    secondaryStorage:
      type: opensearch
`,
		},
		{
			name: "explicit empty type",
			valuesYAML: `orchestration:
  data:
    secondaryStorage:
      type: ""
`,
		},
		{
			name: "no secondary storage",
			valuesYAML: `global:
  noSecondaryStorage: true
`,
		},
		{
			name: "RDBMS exporter",
			valuesYAML: `orchestration:
  exporters:
    rdbms:
      enabled: true
`,
		},
		{
			name: "Optimize OpenSearch",
			valuesYAML: `optimize:
  database:
    opensearch:
      enabled: true
`,
			expectedSetValue: "opensearch",
		},
		{
			name: "Optimize Elasticsearch takes precedence over OpenSearch",
			valuesYAML: `optimize:
  database:
    elasticsearch:
      enabled: true
    opensearch:
      enabled: true
`,
			expectedSetValue: "elasticsearch",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			valuesFile := filepath.Join(t.TempDir(), "values.yaml")
			require.NoError(t, os.WriteFile(valuesFile, []byte(testCase.valuesYAML), 0o600))

			options, err := setupHelmOptions("test", nil, []string{valuesFile}, nil)
			require.NoError(t, err)
			if testCase.expectedSetValue == "" {
				require.NotContains(t, options.SetValues, "orchestration.data.secondaryStorage.type")
			} else {
				require.Equal(t, testCase.expectedSetValue, options.SetValues["orchestration.data.secondaryStorage.type"])
			}
		})
	}
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

func TestValidateStorageFixtureRenderArgs(t *testing.T) {
	t.Parallel()

	rejectedArgs := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "set no secondary storage",
			args:        []string{"--set", "global.noSecondaryStorage=true"},
			expectedErr: "global.noSecondaryStorage must be provided through TestCase.Values",
		},
		{
			name:        "inline set storage type",
			args:        []string{"--set=orchestration.data.secondaryStorage.type=rdbms"},
			expectedErr: "orchestration.data.secondaryStorage.type must be provided through TestCase.Values",
		},
		{
			name:        "set string RDBMS exporter",
			args:        []string{"--set-string", "orchestration.exporters.rdbms.enabled=true"},
			expectedErr: "orchestration.exporters.rdbms.enabled must be provided through TestCase.Values",
		},
		{
			name:        "combined Optimize OpenSearch set",
			args:        []string{"--set", "identity.enabled=true,optimize.database.opensearch.enabled=true"},
			expectedErr: "optimize.database.opensearch.enabled must be provided through TestCase.Values",
		},
		{
			name:        "combined Optimize Elasticsearch set",
			args:        []string{"--set", "identity.enabled=true,optimize.database.elasticsearch.enabled=true"},
			expectedErr: "optimize.database.elasticsearch.enabled must be provided through TestCase.Values",
		},
		{
			name:        "set JSON secondary storage parent",
			args:        []string{"--set-json", `orchestration.data.secondaryStorage={"type":"opensearch"}`},
			expectedErr: "orchestration.data.secondaryStorage must be provided through TestCase.Values",
		},
		{
			name:        "inline set JSON Optimize OpenSearch parent",
			args:        []string{`--set-json=optimize.database.opensearch={"enabled":true}`},
			expectedErr: "optimize.database.opensearch must be provided through TestCase.Values",
		},
		{
			name:        "set JSON orchestration ancestor",
			args:        []string{"--set-json", `orchestration={"data":{"secondaryStorage":{"type":"opensearch"}}}`},
			expectedErr: "orchestration must be provided through TestCase.Values",
		},
		{
			name:        "inline set JSON Optimize ancestor",
			args:        []string{`--set-json=optimize={"database":{"opensearch":{"enabled":true}}}`},
			expectedErr: "optimize must be provided through TestCase.Values",
		},
		{
			name:        "values file",
			args:        []string{"--values", "storage.yaml"},
			expectedErr: "values files must be provided through TestCase.ValuesFiles",
		},
		{
			name:        "compact values file",
			args:        []string{"-fstorage.yaml"},
			expectedErr: "values files must be provided through TestCase.ValuesFiles",
		},
	}

	for _, testCase := range rejectedArgs {
		t.Run(testCase.name, func(t *testing.T) {
			require.ErrorContains(t, validateStorageFixtureRenderArgs(testCase.args), testCase.expectedErr)
		})
	}

	require.NoError(t, validateStorageFixtureRenderArgs([]string{
		"--set-string", "orchestration.env[0].value=true",
	}))
}

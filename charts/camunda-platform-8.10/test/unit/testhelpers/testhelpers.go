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

// Package testhelpers provides utilities for testing Helm charts.
// To enable verbose logging, set the VERBOSE_TEST_LOGGING environment variable to "true".
// Example: VERBOSE_TEST_LOGGING=true go test ./...
package testhelpers

import (
	"fmt"
	"io"
	"maps"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/jsonpath"
)

type CaseTemplate struct {
	Templates []string
}

// TestCase represents a single test scenario for Helm chart testing.
// It encapsulates all the necessary configuration and validation logic for a test.
type TestCase struct {
	// Ignores a test case
	Skip bool

	// Name is the descriptive name of the test case, used for identification in test output
	Name string

	// HelmOptionsExtraArgs contains additional arguments to pass to the Helm command
	// The key spaecifies the Helm command (e.g., "install", "upgrade"), and the value is a slice of arguments
	// This allows customizing the Helm command behavior for specific test cases
	HelmOptionsExtraArgs map[string][]string

	// RenderTemplateExtraArgs contains additional arguments for template rendering
	// These are passed to the template rendering process
	RenderTemplateExtraArgs []string

	// When provided, this function is called to get the templates to render. This overrides the
	// templates set in the test suite
	CaseTemplates *CaseTemplate

	// When provided, this function is called to get the templates to render. This overrides the
	// templates set in the test suite
	Template string

	// Values represents the Helm chart values to set for this test case
	// These are equivalent to values passed with --set flag in Helm CLI
	Values map[string]string

	// ValuesFiles contains paths to values files to use for this test case
	// Use this for values containing Go template syntax (e.g., {{ .Release.Name }})
	// as these cannot be passed via --set flag
	ValuesFiles []string

	// Expected contains key-value pairs that should be present in the rendered output
	// For error tests, it should contain an "ERROR" key with the expected error message
	// For ConfigMap tests, keys can be direct data keys or dot-notation paths into application.yaml
	Expected map[string]string

	// Unexpected contains paths that must NOT be present in the rendered output
	Unexpected []string

	// Verifier is a custom function for complex validation scenarios
	// When provided, it overrides the default validation logic
	// It receives the rendered output and any error that occurred during rendering
	Verifier func(t *testing.T, output string, err error)

	ExpectedObject any

	ObjectAsserter func(t *testing.T, obj any)
}

func validateTestCase(tc TestCase) error {
	expectedError, expectsError := tc.Expected["ERROR"]
	hasDeclarativeAssertions := len(tc.Unexpected) > 0 || len(tc.Expected) > 0 && (!expectsError || len(tc.Expected) > 1)
	hasObject := tc.ExpectedObject != nil
	hasObjectAsserter := tc.ObjectAsserter != nil
	hasObjectAssertions := hasObject || hasObjectAsserter

	if tc.Verifier != nil && (len(tc.Expected) > 0 || len(tc.Unexpected) > 0 || hasObjectAssertions) {
		return fmt.Errorf("Verifier cannot be combined with declarative, error, or object assertions")
	}
	if expectsError && (len(tc.Expected) > 1 || len(tc.Unexpected) > 0 || hasObjectAssertions) {
		return fmt.Errorf("ERROR assertion cannot be combined with output or object assertions")
	}
	if expectsError && strings.TrimSpace(expectedError) == "" {
		return fmt.Errorf("ERROR assertion must not be empty")
	}
	if hasObject != hasObjectAsserter {
		return fmt.Errorf("ExpectedObject and ObjectAsserter must be set together")
	}
	if hasDeclarativeAssertions && hasObjectAssertions {
		return fmt.Errorf("declarative assertions cannot be combined with object assertions")
	}
	if tc.Verifier == nil && !expectsError && !hasDeclarativeAssertions && !hasObjectAssertions {
		return fmt.Errorf("test case must declare an assertion")
	}

	return nil
}

// quietLogger returns a logger that only logs errors
func quietLogger() *logger.Logger {
	// Check if verbose logging is enabled via environment variable
	if os.Getenv("VERBOSE_TEST_LOGGING") == "true" {
		return logger.Default
	}
	// Create a logger that discards all output
	return logger.Discard
}

type storageFixtureState struct {
	typeSet            bool
	noSecondaryStorage bool
	rdbmsEnabled       bool
	openSearchEnabled  bool
}

func mergeStorageFixtureValues(base, overlay map[string]any) {
	for key, overlayValue := range overlay {
		if overlayValue == nil {
			delete(base, key)
			continue
		}

		overlayMap, overlayIsMap := overlayValue.(map[string]any)
		baseMap, baseIsMap := base[key].(map[string]any)
		if overlayIsMap && baseIsMap {
			mergeStorageFixtureValues(baseMap, overlayMap)
			continue
		}
		base[key] = overlayValue
	}
}

func storageFixtureValue(values map[string]any, path ...string) (any, bool) {
	var current any = values
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapping[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func storageFixtureBool(values map[string]any, path ...string) (bool, bool, error) {
	rawValue, ok := storageFixtureValue(values, path...)
	if !ok {
		return false, false, nil
	}
	value, ok := rawValue.(bool)
	if !ok {
		return false, false, fmt.Errorf("storage fixture value %s must be a boolean", strings.Join(path, "."))
	}
	return value, true, nil
}

func loadStorageFixtureState(valuesFiles []string, values map[string]string) (storageFixtureState, error) {
	state := storageFixtureState{}
	mergedValues := make(map[string]any)
	for _, valuesFile := range valuesFiles {
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			return state, fmt.Errorf("read values file %s: %w", valuesFile, err)
		}

		var fileValues map[string]any
		if err := yaml.Unmarshal(data, &fileValues); err != nil {
			return state, fmt.Errorf("parse values file %s: %w", valuesFile, err)
		}
		mergeStorageFixtureValues(mergedValues, fileValues)
	}

	if storageType, ok := storageFixtureValue(mergedValues, "orchestration", "data", "secondaryStorage", "type"); ok {
		if _, ok := storageType.(string); !ok {
			return state, fmt.Errorf("storage fixture value orchestration.data.secondaryStorage.type must be a string")
		}
		state.typeSet = true
	}
	var err error
	if state.noSecondaryStorage, _, err = storageFixtureBool(mergedValues, "global", "noSecondaryStorage"); err != nil {
		return state, err
	}
	if state.rdbmsEnabled, _, err = storageFixtureBool(mergedValues, "orchestration", "exporters", "rdbms", "enabled"); err != nil {
		return state, err
	}
	if state.openSearchEnabled, _, err = storageFixtureBool(mergedValues, "optimize", "database", "opensearch", "enabled"); err != nil {
		return state, err
	}

	if _, ok := values["orchestration.data.secondaryStorage.type"]; ok {
		state.typeSet = true
	}
	if value, ok := values["global.noSecondaryStorage"]; ok {
		state.noSecondaryStorage = strings.EqualFold(value, "true")
	}
	if value, ok := values["orchestration.exporters.rdbms.enabled"]; ok {
		state.rdbmsEnabled = strings.EqualFold(value, "true")
	}
	if value, ok := values["optimize.database.opensearch.enabled"]; ok {
		state.openSearchEnabled = strings.EqualFold(value, "true")
	}

	return state, nil
}

func setupHelmOptions(namespace string, values map[string]string, valuesFiles []string, helmOptionsExtraArgs map[string][]string) (*helm.Options, error) {
	values = maps.Clone(values)
	if values == nil {
		values = make(map[string]string)
	}
	storageState, err := loadStorageFixtureState(valuesFiles, values)
	if err != nil {
		return nil, err
	}
	if !storageState.typeSet && !storageState.noSecondaryStorage && !storageState.rdbmsEnabled {
		secondaryStorageType := "elasticsearch"
		if storageState.openSearchEnabled {
			secondaryStorageType = "opensearch"
		}
		values["orchestration.data.secondaryStorage.type"] = secondaryStorageType
	}

	options := &helm.Options{
		SetValues:      values,
		ValuesFiles:    valuesFiles,
		KubectlOptions: k8s.NewKubectlOptions("", "", namespace),
		Logger:         quietLogger(), // Use quiet logger to reduce verbosity
		ExtraArgs:      helmOptionsExtraArgs,
	}
	return options, nil
}

func validateStorageFixtureRenderArgs(renderTemplateExtraArgs []string) error {
	storageKeys := []string{
		"global.noSecondaryStorage",
		"orchestration.data.secondaryStorage.type",
		"orchestration.exporters.rdbms.enabled",
		"optimize.database.opensearch.enabled",
		"global",
		"orchestration.data.secondaryStorage",
		"orchestration.data",
		"orchestration.exporters.rdbms",
		"orchestration.exporters",
		"orchestration",
		"optimize.database.opensearch",
		"optimize.database",
		"optimize",
	}
	for _, argument := range renderTemplateExtraArgs {
		if argument == "--values" || argument == "-f" || strings.HasPrefix(argument, "--values=") || strings.HasPrefix(argument, "-f=") {
			return fmt.Errorf("values files must be provided through TestCase.ValuesFiles, not RenderTemplateExtraArgs")
		}
		for _, key := range storageKeys {
			if strings.Contains(argument, key+"=") {
				return fmt.Errorf("storage fixture value %s must be provided through TestCase.Values, not RenderTemplateExtraArgs", key)
			}
		}
	}
	return nil
}

func renderTemplateE(t *testing.T, chartPath, release string, namespace string, templates []string, values map[string]string, valuesFiles []string, extraArgs map[string][]string, renderTemplateExtraArgs []string) (string, error) {
	if err := validateStorageFixtureRenderArgs(renderTemplateExtraArgs); err != nil {
		return "", err
	}
	options, err := setupHelmOptions(namespace, values, valuesFiles, extraArgs)
	if err != nil {
		return "", err
	}

	output, err := helm.RenderTemplateE(t, options, chartPath, release, templates, renderTemplateExtraArgs...)
	return output, err
}

func RunTestCasesE(t *testing.T, chartPath, release, namespace string, templates []string, testCases []TestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(tct *testing.T) {
			if tc.Skip {
				tct.Skipf("Skipping test case: %s", tc.Name)
			}
			defer func() {
				if r := recover(); r != nil {
					tct.Errorf("Panic in test case %q: %v", tc.Name, r)
				}
			}()
			runTestCaseE(tct, chartPath, release, namespace, templates, tc)
		})
	}
}

func runTestCaseE(t *testing.T, chartPath, release, namespace string, templates []string, tc TestCase) {
	require.NoError(t, validateTestCase(tc), "invalid test case %q", tc.Name)

	var caseTemplates []string
	if tc.Template != "" {
		caseTemplates = []string{tc.Template}
	} else if tc.CaseTemplates != nil {
		caseTemplates = tc.CaseTemplates.Templates
	} else {
		caseTemplates = templates
	}
	output, err := renderTemplateE(t, chartPath, release, namespace, caseTemplates, tc.Values, tc.ValuesFiles, tc.HelmOptionsExtraArgs, tc.RenderTemplateExtraArgs)
	if err != nil {
		t.Logf("Error during rendering: %v", err)
	}
	if tc.Verifier != nil {
		tc.Verifier(t, output, err)
		return
	}

	if expectedErr, ok := tc.Expected["ERROR"]; ok {
		require.ErrorContains(t, err, expectedErr)
		return
	}
	if err != nil {
		t.Fatalf("Unexpected error during rendering: %v", err)
	}

	if len(tc.Expected) > 0 || len(tc.Unexpected) > 0 {
		verifyRenderedPaths(t, output, tc.Expected, tc.Unexpected)
		return
	}

	if tc.ExpectedObject != nil {
		helm.UnmarshalK8SYaml(t, output, tc.ExpectedObject)
		tc.ObjectAsserter(t, tc.ExpectedObject)
	}
}

type pathMatch struct {
	objectIndex int
	value       reflect.Value
}

func decodeRenderedObjects(output string) ([]any, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)
	objects := []any{}
	for {
		var object any
		if err := decoder.Decode(&object); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode rendered object: %w", err)
		}
		if object == nil {
			continue
		}
		objectValue := reflect.ValueOf(object)
		if objectValue.Kind() == reflect.Map && objectValue.Len() == 0 {
			continue
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func findPathMatches(objects []any, path string) ([]pathMatch, error) {
	query := jsonpath.New(path).AllowMissingKeys(true)
	if err := query.Parse("{." + path + "}"); err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}

	matches := []pathMatch{}
	for objectIndex, object := range objects {
		results, err := query.FindResults(object)
		if err != nil {
			return nil, fmt.Errorf("evaluate path %q in rendered object %d: %w", path, objectIndex, err)
		}
		for _, result := range results {
			for _, value := range result {
				matches = append(matches, pathMatch{objectIndex: objectIndex, value: value})
			}
		}
	}
	return matches, nil
}

func resolveScalarPath(objects []any, path string) (string, error) {
	matches, err := findPathMatches(objects, path)
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("path %q resolved to %d values across %d rendered objects; expected exactly one", path, len(matches), len(objects))
	}

	value := matches[0].value
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return "null", nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return "null", nil
	}

	switch value.Kind() {
	case reflect.Bool,
		reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.String,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprint(value.Interface()), nil
	default:
		return "", fmt.Errorf("path %q must resolve to a scalar, got %s", path, value.Kind())
	}
}

func verifyRenderedPaths(t *testing.T, output string, expected map[string]string, unexpected []string) {
	t.Helper()

	objects, err := decodeRenderedObjects(output)
	require.NoError(t, err)

	expectedPaths := make([]string, 0, len(expected))
	for path := range expected {
		expectedPaths = append(expectedPaths, path)
	}
	sort.Strings(expectedPaths)
	for _, path := range expectedPaths {
		actual, err := resolveScalarPath(objects, path)
		require.NoError(t, err)
		require.Equal(t, expected[path], actual, "path %q", path)
	}

	unexpectedPaths := append([]string(nil), unexpected...)
	sort.Strings(unexpectedPaths)
	for _, path := range unexpectedPaths {
		matches, err := findPathMatches(objects, path)
		require.NoError(t, err)
		require.Empty(t, matches, "path %q must be structurally absent", path)
	}
}

// renderTemplate renders the specified Helm templates into a Kubernetes ConfigMap
func renderTemplate(t *testing.T, chartPath, release string, namespace string, templates []string, values map[string]string, valuesFiles []string) corev1.ConfigMap {
	options, err := setupHelmOptions(namespace, values, valuesFiles, nil)
	require.NoError(t, err)

	output := helm.RenderTemplate(t, options, chartPath, release, templates)
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(t, output, &configmap)
	return configmap
}

// RunTestCases executes multiple test cases using the provided Helm chart and ConfigMap validation
func RunTestCases(t *testing.T, chartPath, release, namespace string, templates []string, testCases []TestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(tct *testing.T) {
			configmap := renderTemplate(tct, chartPath, release, namespace, templates, tc.Values, tc.ValuesFiles)
			verifyConfigMap(tct, tc.Name, configmap, tc.Expected, tc.Unexpected)
		})
	}
}

// verifyConfigMap checks whether the generated ConfigMap contains the expected key-value pairs
// and does not contain any of the unexpected keys
func verifyConfigMap(t *testing.T, testCase string, configmap corev1.ConfigMap, expectedValues map[string]string, unexpectedKeys []string) {
	for keyPath, expectedValue := range expectedValues {
		var actualValue string
		if strings.HasPrefix(keyPath, "configmapApplication.") {
			var configmapApplication map[string]any
			err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
			require.NoError(t, err)
			actualValue = getConfigMapFieldValue(configmapApplication, strings.Split(keyPath, ".")[1:])
		} else {
			actualValue = strings.TrimSpace(configmap.Data[keyPath])
		}
		require.Equal(t, expectedValue, actualValue, "Test case '%s': Expected key '%s' to have value '%s', but got '%s'", testCase, keyPath, expectedValue, actualValue)
	}
	for _, keyPath := range unexpectedKeys {
		_, found := configmap.Data[keyPath]
		require.False(t, found, "Test case '%s': key '%s' should NOT be present in the ConfigMap", testCase, keyPath)
	}
}

// getConfigMapFieldValue function traverses a nested map structure based on a given key path.
// It handles maps with both interface{} and string keys, converting them as necessary to retrieve the desired value.
// If the key is not found or the final value is not a string, the function returns an empty string.
// TODO: Replace this code with some library, we should not have such logic in the test code.
func getConfigMapFieldValue(configmapApplication map[string]any, keyPath []string) string {
	var current any = configmapApplication

	for _, key := range keyPath {
		if nestedMap, ok := current.(map[any]any); ok {
			// Convert map[interface{}]any to map[string]any
			stringMap := make(map[string]any)
			for k, v := range nestedMap {
				if strKey, isString := k.(string); isString {
					stringMap[strKey] = v
				}
			}
			// Move to the next level in the map
			current = stringMap[key]
		} else if nestedMap, ok := current.(map[string]any); ok {
			// If the current level is already a map with string keys, move to the next level
			current = nestedMap[key]
		} else {
			// If the key is not found or current is not a map, return an empty string
			return ""
		}
	}

	// Return string if possible, otherwise attempt to convert to string
	switch v := current.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		// Unsupported type
		return ""
	}
}

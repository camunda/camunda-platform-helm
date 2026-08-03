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
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/jsonpath"
)

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

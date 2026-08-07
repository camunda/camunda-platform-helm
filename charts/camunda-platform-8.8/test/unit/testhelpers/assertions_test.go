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

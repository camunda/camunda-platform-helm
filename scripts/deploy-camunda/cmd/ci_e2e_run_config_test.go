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

package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteE2ERunConfig(t *testing.T) {
	var out bytes.Buffer
	err := writeE2ERunConfig(&out, []string{
		"--namespace", "test namespace",
		"--playwright-project", "full-suite-v1",
		"--file-pattern", "tasklist/**/*[ab].spec.js",
		"--is-rba", "true",
		"--mcp-gateway-enabled", "false",
		"--trace", "retain-on-failure",
	})
	require.NoError(t, err)

	assert.Contains(t, out.String(), "PLAYWRIGHT_PROJECT='full-suite-v1'")
	assert.Contains(t, out.String(), "FILE_PATTERN='tasklist/**/*[ab].spec.js'")
	assert.Contains(t, out.String(), "IS_RBA_OVERRIDE='true'")
	assert.Contains(t, out.String(), "MCP_GATEWAY_ENABLED_OVERRIDE='false'")
	assert.Contains(t, out.String(), "REQUIRE_SM_810_TEST_SUITE_OVERRIDE='true'")
	assert.Contains(t, out.String(), "set -- '--namespace' 'test namespace' '--trace' 'retain-on-failure'")
}

func TestWriteE2ERunConfigDoesNotRequireSM810SuiteForAuth0(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, writeE2ERunConfig(&out, []string{"--playwright-project", "auth0-smoke"}))
	assert.Contains(t, out.String(), "REQUIRE_SM_810_TEST_SUITE_OVERRIDE='false'")
}

func TestWriteE2ERunConfigQuotesShellValues(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, writeE2ERunConfig(&out, []string{"--file-pattern", "it's a test"}))
	assert.Contains(t, out.String(), `FILE_PATTERN='it'"'"'s a test'`)
}

func TestWriteE2ERunConfigRequiresValues(t *testing.T) {
	err := writeE2ERunConfig(&bytes.Buffer{}, []string{"--is-rba"})
	require.EqualError(t, err, "flag --is-rba needs an argument")
}

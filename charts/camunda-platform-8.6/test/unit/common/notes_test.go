// Copyright Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	_ "camunda-platform/test/unit/utils"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotesTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	output, err := exec.Command("helm", "install", "credential-output-test", chartPath,
		"--dry-run=client",
		"--set", "identity.firstUser.password=credential-output-canary-do-not-print",
		"--set", "orchestration.data.secondaryStorage.type=elasticsearch",
		"--set", "global.elasticsearch.enabled=true",
		"--set", "global.elasticsearch.external=true",
		"--set", "global.elasticsearch.url.host=elasticsearch",
	).CombinedOutput()
	require.NoError(t, err, string(output))

	_, notes, found := strings.Cut(string(output), "\nNOTES:\n")
	require.True(t, found)
	require.Contains(t, notes, "intentionally omitted")
	require.NotContains(t, notes, "credential-output-canary-do-not-print")
}

func TestNotesKeycloakEmudhraWarning(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	testCases := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "DefaultKeycloak25Warns",
			expected: true,
		},
		{
			name:     "EnterpriseKeycloak25Warns",
			args:     []string{"-f", filepath.Join(chartPath, "values-enterprise.yaml")},
			expected: true,
		},
		{
			name:     "Keycloak23Warns",
			args:     []string{"--set", "identityKeycloak.image.tag=23.0.7"},
			expected: true,
		},
		{
			name:     "Keycloak24Warns",
			args:     []string{"--set", "identityKeycloak.image.tag=24.0.5"},
			expected: true,
		},
		{
			name: "Keycloak26DoesNotWarn",
			args: []string{"--set", "identityKeycloak.image.tag=quay-26.8.1"},
		},
		{
			name: "NonCamundaRepositoryDoesNotWarn",
			args: []string{"--set", "identityKeycloak.image.repository=bitnami/keycloak"},
		},
		{
			name: "DisabledKeycloakDoesNotWarn",
			args: []string{
				"--set", "identityKeycloak.enabled=false",
				"--set", "global.identity.keycloak.url.protocol=https",
				"--set", "global.identity.keycloak.url.host=keycloak.example.com",
				"--set", "global.identity.keycloak.url.port=443",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"install", "keycloak-emudhra-test", chartPath, "--dry-run=client"}, tc.args...)
			output, err := exec.Command("helm", args...).CombinedOutput()
			require.NoError(t, err, string(output))

			_, notes, found := strings.Cut(string(output), "\nNOTES:\n")
			require.True(t, found)
			if tc.expected {
				require.Contains(t, notes, "eMudhra CA compromise")
				require.Contains(t, notes, "#notice-80")
			} else {
				require.NotContains(t, notes, "#notice-80")
			}
		})
	}
}

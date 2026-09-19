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

package camunda

import (
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

	testCases := []struct {
		name        string
		values      []string
		expected    string
		notExpected string
	}{
		{
			name:        "inline secret",
			values:      []string{"identity.firstUser.secret.inlineSecret=credential-output-canary-do-not-print"},
			expected:    "configured via `identity.firstUser.secret.inlineSecret`",
			notExpected: "credential-output-canary-do-not-print",
		},
		{
			name:     "complete secret reference",
			values:   []string{"identity.firstUser.secret.existingSecret=first-user", "identity.firstUser.secret.existingSecretKey=password"},
			expected: "stored in Kubernetes Secret \"first-user\" under key \"password\"",
		},
		{
			name:     "empty configuration",
			expected: "No password is configured",
		},
		{
			name:        "incomplete secret reference",
			values:      []string{"identity.firstUser.secret.existingSecret=first-user"},
			expected:    "No password is configured",
			notExpected: "stored in Kubernetes Secret",
		},
		{
			name:        "round-robin across more than one zone",
			values:      []string{"orchestration.partitioning.numberOfZones=2", "orchestration.partitioning.zoneIndex=0"},
			expected:    "zones: 2",
			notExpected: "regions: 2",
		},
		{
			name: "zone-aware reports the declared zone count",
			values: []string{
				"orchestration.partitioning.scheme=zone-aware",
				"orchestration.partitioning.zone=zone-a",
				"orchestration.partitioning.zones[0].name=zone-a",
				"orchestration.partitioning.zones[0].numberOfBrokers=1",
				"orchestration.partitioning.zones[0].numberOfReplicas=1",
				"orchestration.partitioning.zones[0].priority=1",
				"orchestration.partitioning.zones[1].name=zone-b",
				"orchestration.partitioning.zones[1].numberOfBrokers=1",
				"orchestration.partitioning.zones[1].numberOfReplicas=1",
				"orchestration.partitioning.zones[1].priority=2",
			},
			expected: "zones: 2",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			args := []string{
				"install", "credential-output-test", chartPath,
				"--dry-run=client",
				"--set", "orchestration.data.secondaryStorage.type=elasticsearch",
			}
			for _, value := range testCase.values {
				args = append(args, "--set", value)
			}

			output, err := exec.Command("helm", args...).CombinedOutput()
			require.NoError(t, err, string(output))

			_, notes, found := strings.Cut(string(output), "\nNOTES:\n")
			require.True(t, found)
			require.Contains(t, notes, testCase.expected)
			if testCase.notExpected != "" {
				require.NotContains(t, notes, testCase.notExpected)
			}
		})
	}
}

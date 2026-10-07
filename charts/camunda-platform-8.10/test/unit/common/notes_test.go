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

func TestAuthIssuerNotesTemplate(t *testing.T) {
	t.Parallel()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	for _, releaseInfo := range []string{"true", "false"} {
		t.Run("release-info-"+releaseInfo, func(t *testing.T) {
			output, err := exec.Command("helm", "install", "issuer-notes-test", chartPath,
				"--dry-run=client",
				"--set", "orchestration.data.secondaryStorage.type=elasticsearch",
				"--set", "global.createReleaseInfo="+releaseInfo,
				"--set", "global.identity.auth.enabled=true",
				"--set", "global.identity.auth.type=GENERIC",
				"--set", "global.identity.auth.publicIssuerUrl=",
			).CombinedOutput()
			require.NoError(t, err, string(output))
			_, notes, found := strings.Cut(string(output), "\nNOTES:\n")
			require.True(t, found)
			require.Equal(t, 1, strings.Count(notes, "no shared authentication issuer or issuer backend URL resolves"))
			require.Equal(t, releaseInfo == "true", strings.Contains(notes, "## Release information"))
		})
	}
}

func TestNotesTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	testCases := []struct {
		name         string
		values       []string
		expected     string
		notExpected  string
		expectedURLs []string
	}{
		{
			name: "custom HTTP ingress port",
			values: []string{
				"global.ingress.enabled=true", "global.host=camunda.example.com",
				"global.ingress.publicPorts.http=8080", "global.ingress.publicPorts.https=8443",
				"identity.enabled=true", "identity.contextPath=/identity",
				"camundaHub.enabled=true", "camundaHub.contextPath=/modeler",
				"camundaHub.restapi.mail.fromAddress=test@example.com",
				"orchestration.ingress.grpc.enabled=true", "orchestration.ingress.grpc.host=grpc.example.com",
			},
			expected: "- Camunda REST API: http://camunda.example.com:8080",
			expectedURLs: []string{
				"- Identity: http://camunda.example.com:8080/identity",
				"Ingress URLs use global.ingress.publicPorts. Configure ingress-controller listeners and host port forwarding separately.",
				"Explicit full URLs and authentication redirect URLs are not rewritten.",
				"- Camunda Hub: http://camunda.example.com:8080/modeler",
				"- Camunda Hub WebSockets: http://camunda.example.com:8080/modeler-ws",
				"- Camunda gRPC API: http://grpc.example.com:8080",
			},
		},
		{
			name: "custom HTTPS ingress port",
			values: []string{
				"global.ingress.enabled=true", "global.host=camunda.example.com", "global.ingress.tls.enabled=true",
				"global.ingress.publicPorts.http=8080", "global.ingress.publicPorts.https=8443",
				"identity.enabled=true", "identity.contextPath=/identity",
				"camundaHub.enabled=true", "camundaHub.contextPath=/modeler",
				"camundaHub.restapi.mail.fromAddress=test@example.com",
				"orchestration.ingress.grpc.enabled=true", "orchestration.ingress.grpc.host=grpc.example.com",
				"orchestration.ingress.grpc.tls.enabled=true",
			},
			expected: "- Camunda REST API: https://camunda.example.com:8443",
			expectedURLs: []string{
				"- Identity: https://camunda.example.com:8443/identity",
				"Ingress URLs use global.ingress.publicPorts. Configure ingress-controller listeners and host port forwarding separately.",
				"Explicit full URLs and authentication redirect URLs are not rewritten.",
				"- Camunda Hub: https://camunda.example.com:8443/modeler",
				"- Camunda Hub WebSockets: https://camunda.example.com:8443/modeler-ws",
				"- Camunda gRPC API: https://grpc.example.com:8443",
			},
		},
		{
			name: "ingress protocol override",
			values: []string{
				"global.ingress.enabled=true", "global.host=camunda.example.com",
				"global.ingress.tls.enabled=false", "global.ingress.protocol=https",
				"identity.enabled=true", "identity.contextPath=/identity",
				"camundaHub.enabled=true", "camundaHub.contextPath=/modeler",
				"camundaHub.restapi.mail.fromAddress=test@example.com",
			},
			expected:    "- Camunda REST API: https://camunda.example.com",
			notExpected: "- Camunda REST API: http://camunda.example.com",
			expectedURLs: []string{
				"- Identity: https://camunda.example.com/identity",
				"- Camunda Hub: https://camunda.example.com/modeler",
				"- Camunda Hub WebSockets: https://camunda.example.com/modeler-ws",
			},
		},
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
			values:      []string{"orchestration.partitioning.numberOfZones=3", "orchestration.partitioning.zoneIndex=0"},
			expected:    "zones: 3",
			notExpected: "regions: 3",
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
			for _, expectedURL := range testCase.expectedURLs {
				require.Contains(t, notes, expectedURL)
			}
			if testCase.notExpected != "" {
				require.NotContains(t, notes, testCase.notExpected)
			}
		})
	}
}

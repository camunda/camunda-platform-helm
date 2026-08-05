// Copyright 2022 Camunda Services GmbH
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

package camunda

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ReleaseInfoGatewayTest verifies the release-info external URLs resolve to
// global.host (not localhost) when the Gateway API is used instead of Ingress.
type ReleaseInfoGatewayTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestReleaseInfoGateway(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ReleaseInfoGatewayTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func (s *ReleaseInfoGatewayTest) TestExternalURLsUseGlobalHost() {
	testCases := []testhelpers.TestCase{
		{
			Name: "GatewayTLSExternalURLsUseGlobalHostAndCustomPort",
			Values: map[string]string{
				"global.ingress.enabled":                   "false",
				"global.gateway.enabled":                   "true",
				"global.gateway.tls.enabled":               "true",
				"global.gateway.tls.port":                  "8443",
				"global.gateway.tls.secretName":            "camunda-tls",
				"global.host":                              "camunda.example.com",
				"global.identity.keycloak.internal":        "true",
				"identity.contextPath":                     "/identity",
				"identity.enabled":                         "true",
				"optimize.contextPath":                     "/optimize",
				"optimize.enabled":                         "true",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.gateway.grpc.enabled":       "true",
				"webModeler.enabled":                       "true",
				"webModeler.contextPath":                   "/modeler",
				"webModeler.restapi.mail.fromAddress":      "test@example.com",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "url: https://camunda.example.com:8443/identity")
				require.Contains(t, output, "url: https://camunda.example.com:8443/modeler")
				require.Contains(t, output, "url: https://camunda.example.com:8443/operate")
				require.Contains(t, output, "url: https://camunda.example.com:8443/optimize")
				require.Contains(t, output, "url: https://camunda.example.com:8443/tasklist")
				require.Contains(t, output, "grpc: https://grpc-camunda.example.com:8443")
				require.Contains(t, output, "http: https://camunda.example.com:8443")
				require.Contains(t, output, "https://camunda.example.com:8443/auth")
				require.NotContains(t, output, "http://localhost:8080")
				require.NotContains(t, output, "http://localhost:26500")
				require.NotContains(t, output, "http://localhost:8083")
			},
		},
		{
			Name: "GatewayWithoutGRPCRouteUsesLocalEndpoint",
			Values: map[string]string{
				"global.ingress.enabled":        "false",
				"global.gateway.enabled":        "true",
				"global.gateway.tls.enabled":    "true",
				"global.gateway.tls.secretName": "camunda-tls",
				"global.host":                   "camunda.example.com",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "grpc: http://localhost:26500")
				require.NotContains(t, output, "grpc: https://grpc-camunda.example.com")
			},
		},
		{
			Name: "PlaintextGatewayUsesCustomPortAndIgnoresInactiveIngressTLS",
			Values: map[string]string{
				"global.ingress.enabled":            "false",
				"global.ingress.tls.enabled":        "true",
				"global.gateway.enabled":            "true",
				"global.gateway.port":               "8080",
				"global.gateway.tls.enabled":        "false",
				"global.host":                       "camunda.example.com",
				"global.identity.keycloak.internal": "true",
				"identity.enabled":                  "true",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "url: http://camunda.example.com:8080/operate")
				require.Contains(t, output, "http://camunda.example.com:8080/auth")
				require.NotContains(t, output, "https://camunda.example.com/auth")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

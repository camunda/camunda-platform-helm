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

package identity

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HTTPRouteTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestHTTPRouteTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &HTTPRouteTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/identity/httproute.yaml"},
	})
}

func (s *HTTPRouteTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestHTTPRouteNotRenderedWhenGatewayDisabled",
			Values: map[string]string{
				"global.gateway.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: HTTPRoute")
			},
		},
		{
			Name: "TestKeycloakHTTPRouteRendered",
			Values: map[string]string{
				"global.gateway.enabled":   "true",
				"global.gateway.hostname":  "camunda.example.com",
				"identityKeycloak.enabled": "true",
				"identity.enabled":         "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: HTTPRoute")
				require.Contains(t, output, "name: keycloak")
				require.Contains(t, output, "\"camunda.example.com\"")
			},
		},
		{
			Name: "TestIdentityHTTPRouteRendered",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.gateway.hostname": "camunda.example.com",
				"identity.enabled":        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: HTTPRoute")
				require.Contains(t, output, "name: identity")
				require.Contains(t, output, "\"camunda.example.com\"")
				require.Contains(t, output, "port: 80")
			},
		},
		{
			Name: "TestKeycloakHTTPRouteNotRenderedWhenKeycloakDisabled",
			Values: map[string]string{
				"global.gateway.enabled":   "true",
				"global.gateway.hostname":  "camunda.example.com",
				"identityKeycloak.enabled": "false",
				"identity.enabled":         "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "name: keycloak")
				// Identity HTTPRoute should still be present
				require.Contains(t, output, "name: identity")
			},
		},
		{
			Name: "TestIdentityHTTPRouteNotRenderedWhenIdentityDisabled",
			Values: map[string]string{
				"global.gateway.enabled":   "true",
				"global.gateway.hostname":  "camunda.example.com",
				"identity.enabled":         "false",
				"identityKeycloak.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "name: identity")
			},
		},
		{
			Name: "TestHTTPRouteWithTLSSectionName",
			Values: map[string]string{
				"global.gateway.enabled":     "true",
				"global.gateway.hostname":    "camunda.example.com",
				"global.gateway.tls.enabled": "true",
				"identity.enabled":           "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "sectionName: https")
			},
		},
		{
			Name: "TestIdentityHTTPRouteWithContextPath",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.gateway.hostname": "camunda.example.com",
				"identity.enabled":        "true",
				"identity.contextPath":    "/identity",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "value: /identity")
			},
		},
		{
			Name: "TestHTTPRouteWithAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":                 "true",
				"global.gateway.hostname":                "camunda.example.com",
				"identity.enabled":                       "true",
				"global.annotations.global-key":          "global-value",
				"global.gateway.annotations.gateway-key": "gateway-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "global-key: global-value")
				require.Contains(t, output, "gateway-key: gateway-value")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

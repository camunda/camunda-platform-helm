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

package optimize

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
		templates: []string{"templates/optimize/httproute.yaml"},
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
			Name: "TestHTTPRouteNotRenderedWhenOptimizeDisabled",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.host": "camunda.example.com",
				"optimize.enabled":        "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: HTTPRoute")
			},
		},
		{
			Name: "TestHTTPRouteRendered",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.host": "camunda.example.com",
				"optimize.enabled":        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: HTTPRoute")
				require.Contains(t, output, "name: optimize")
				require.Contains(t, output, "sectionName: http")
				require.Contains(t, output, "\"camunda.example.com\"")
				require.Contains(t, output, "port: 80")
			},
		},
		{
			Name: "TestHTTPRouteWithTLSSectionName",
			Values: map[string]string{
				"global.gateway.enabled":     "true",
				"global.host":    "camunda.example.com",
				"global.gateway.tls.enabled": "true",
				"optimize.enabled":           "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "sectionName: https")
			},
		},
		{
			Name: "TestHTTPRouteWithContextPath",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.host": "camunda.example.com",
				"optimize.enabled":        "true",
				"optimize.contextPath":    "/optimize",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "value: /optimize")
			},
		},
		{
			Name: "TestHTTPRouteWithAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":                 "true",
				"global.host":                "camunda.example.com",
				"optimize.enabled":                       "true",
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

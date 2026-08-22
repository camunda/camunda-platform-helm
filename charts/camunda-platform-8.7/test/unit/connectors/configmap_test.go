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

package connectors

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type ConfigMapTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigMapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigMapTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/connectors/configmap.yaml"},
	})
}

func (s *ConfigMapTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerSetContextPath",
			Values: map[string]string{
				"connectors.enabled":     "true",
				"connectors.contextPath": "/connectors",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication ConnectorsConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Equal("/connectors", configmapApplication.Server.Servlet.ContextPath)
			},
		}, {
			Name: "TestContainerConfigMapSetInboundModeCredentials",
			Values: map[string]string{
				"connectors.enabled":           "true",
				"connectors.inbound.mode":      "credentials",
				"global.identity.auth.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication ConnectorsConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Empty(configmapApplication.Camunda.Connector.Polling.Enabled)
				s.Require().Empty(configmapApplication.Camunda.Connector.WebHook.Enabled)
				s.Require().Empty(configmapApplication.Operate.Client.KeycloakTokenURL)
				s.Require().Empty(configmapApplication.Operate.Client.ClientId)

				s.Require().Equal("http://camunda-platform-test-zeebe-gateway:26500", configmapApplication.Camunda.Client.Zeebe.GRPCAddress)
				s.Require().Equal("http://camunda-platform-test-operate:80", configmapApplication.Operate.Client.BaseURL)
				s.Require().Equal("connectors", configmapApplication.Operate.Client.Username)
			},
		}, {
			Name: "TestContainerConfigMapSetInboundModeDisabled",
			Values: map[string]string{
				"connectors.enabled":      "true",
				"connectors.inbound.mode": "disabled",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication ConnectorsConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Empty(configmapApplication.Operate.Client.KeycloakTokenURL)
				s.Require().Empty(configmapApplication.Operate.Client.BaseURL)
				s.Require().Empty(configmapApplication.Operate.Client.Username)
				s.Require().Empty(configmapApplication.Operate.Client.ClientId)

				s.Require().Equal("http://camunda-platform-test-zeebe-gateway:26500", configmapApplication.Camunda.Client.Zeebe.GRPCAddress)
				s.Require().Equal("false", configmapApplication.Camunda.Connector.Polling.Enabled)
				s.Require().Equal("false", configmapApplication.Camunda.Connector.WebHook.Enabled)
			},
		}, {
			Name: "TestContainerConfigMapSetInboundModeOauthIdentity",
			Values: map[string]string{
				"connectors.enabled":           "true",
				"connectors.inbound.mode":      "oauth",
				"global.identity.auth.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication ConnectorsConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Empty(configmapApplication.Camunda.Connector.Polling.Enabled)
				s.Require().Empty(configmapApplication.Camunda.Connector.WebHook.Enabled)
				s.Require().Empty(configmapApplication.Operate.Client.Username)

				s.Require().Equal("http://camunda-platform-test-zeebe-gateway:26500", configmapApplication.Camunda.Client.Zeebe.GRPCAddress)
				s.Require().Equal("http://camunda-platform-test-operate:80", configmapApplication.Operate.Client.BaseURL)
				s.Require().Equal("operate-api", configmapApplication.Camunda.Identity.Audience)
				s.Require().Equal("connectors", configmapApplication.Camunda.Identity.ClientId)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

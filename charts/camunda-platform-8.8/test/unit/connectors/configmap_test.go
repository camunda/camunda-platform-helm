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

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Equal("/connectors", configmapApplication.Server.Servlet.ContextPath)
			},
		},
	}

	s.T().Skip("Skipping until 8.8 reenables these")
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// // TODO: Refactor the tests to work with the new Connectors config.
// func (s *configMapTemplateTest) TestContainerConfigMapSetInboundModeCredentials() {
// 	// given
// 	options := &helm.Options{
// 		SetValues: map[string]string{
// 			"connectors.enabled":           "true",
// 			"connectors.inbound.mode":      "credentials",
// 			"global.identity.auth.enabled": "false",
// 		},
// 		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
// 	}

// 	// when
// 	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
// 	var configmap corev1.ConfigMap
// 	var configmapApplication ConnectorsConfigYAML
// 	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

// 	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
// 	if err != nil {
// 		s.Fail("Failed to unmarshal yaml. error=", err)
// 	}

// 	// then
// 	s.Require().Empty(configmapApplication.Camunda.Connector.Polling.Enabled)
// 	s.Require().Empty(configmapApplication.Camunda.Connector.WebHook.Enabled)
// 	s.Require().Empty(configmapApplication.Camunda.Operate.Client.KeycloakTokenURL)
// 	s.Require().Empty(configmapApplication.Camunda.Operate.Client.ClientId)

// 	s.Require().Equal("camunda-platform-test-zeebe:26500", configmapApplication.Zeebe.Client.Broker.GatewayAddress)
// 	s.Require().Equal("true", configmapApplication.Zeebe.Client.Security.Plaintext)
// 	s.Require().Equal("http://camunda-platform-test-zeebe:80/v1", configmapApplication.Camunda.Operate.Client.Url)
// 	s.Require().Equal("connectors", configmapApplication.Camunda.Operate.Client.Username)
// }

// func (s *configMapTemplateTest) TestContainerConfigMapSetInboundModeDisabled() {
// 	// given
// 	options := &helm.Options{
// 		SetValues: map[string]string{
// 			"connectors.enabled":      "true",
// 			"connectors.inbound.mode": "disabled",
// 		},
// 		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
// 	}

// 	// when
// 	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
// 	var configmap corev1.ConfigMap
// 	var configmapApplication ConnectorsConfigYAML
// 	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

// 	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
// 	if err != nil {
// 		s.Fail("Failed to unmarshal yaml. error=", err)
// 	}

// 	// then
// 	s.Require().Empty(configmapApplication.Camunda.Operate.Client.KeycloakTokenURL)
// 	s.Require().Empty(configmapApplication.Camunda.Operate.Client.Url)
// 	s.Require().Empty(configmapApplication.Camunda.Operate.Client.Username)
// 	s.Require().Empty(configmapApplication.Camunda.Operate.Client.ClientId)

// 	s.Require().Equal("camunda-platform-orchestration-gateway:26500", configmapApplication.Zeebe.Client.Broker.GatewayAddress)
// 	s.Require().Equal("true", configmapApplication.Zeebe.Client.Security.Plaintext)
// 	s.Require().Equal("false", configmapApplication.Camunda.Connector.Polling.Enabled)
// 	s.Require().Equal("false", configmapApplication.Camunda.Connector.WebHook.Enabled)
// }

// func (s *configMapTemplateTest) TestContainerConfigMapSetInboundModeOauthIdentity() {
// 	// given
// 	options := &helm.Options{
// 		SetValues: map[string]string{
// 			"connectors.enabled":           "true",
// 			"connectors.inbound.mode":      "oauth",
// 			"global.identity.auth.enabled": "true",
// 		},
// 		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
// 	}

// 	// when
// 	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
// 	var configmap corev1.ConfigMap
// 	var configmapApplication ConnectorsConfigYAML
// 	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

// 	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
// 	if err != nil {
// 		s.Fail("Failed to unmarshal yaml. error=", err)
// 	}

// 	// then
// 	s.Require().Empty(configmapApplication.Camunda.Connector.Polling.Enabled)
// 	s.Require().Empty(configmapApplication.Camunda.Connector.WebHook.Enabled)
// 	s.Require().Empty(configmapApplication.Camunda.Operate.Client.Username)

// 	s.Require().Equal("camunda-platform-test-zeebe:26500/v1", configmapApplication.Zeebe.Client.Broker.GatewayAddress)
// 	s.Require().Equal("true", configmapApplication.Zeebe.Client.Security.Plaintext)
// 	s.Require().Equal("http://camunda-platform-test-operate:80", configmapApplication.Camunda.Operate.Client.Url)
// 	s.Require().Equal("operate-api", configmapApplication.Camunda.Identity.Audience)
// 	s.Require().Equal("connectors", configmapApplication.Camunda.Identity.ClientId)
// }

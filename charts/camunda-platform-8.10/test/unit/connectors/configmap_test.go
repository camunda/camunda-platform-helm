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
			Expected: map[string]string{
				"configmapApplication.server.servlet.context-path": "/connectors",
			},
		},
		{
			Name: "TestConnectorsOIDCTokenScope",
			Values: map[string]string{
				"connectors.enabled":                                               "true",
				"connectors.security.authentication.method":                        "oidc",
				"connectors.security.authentication.oidc.clientId":                 "test-client-id",
				"connectors.security.authentication.oidc.tokenScope":               "test-client-id/.default",
				"connectors.security.authentication.oidc.secret.existingSecret":    "test-secret",
				"connectors.security.authentication.oidc.secret.existingSecretKey": "client-secret",
				"orchestration.security.authentication.oidc.secret.existingSecret": "test-orch-secret",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.client.auth.client-id": "test-client-id",
				"configmapApplication.camunda.client.auth.scope":     "test-client-id/.default",
			},
		},
		{
			Name: "TestConnectorsOIDCWithoutTokenScope",
			Values: map[string]string{
				"connectors.enabled":                                               "true",
				"connectors.security.authentication.method":                        "oidc",
				"connectors.security.authentication.oidc.clientId":                 "test-client-id",
				"connectors.security.authentication.oidc.secret.existingSecret":    "test-secret",
				"connectors.security.authentication.oidc.secret.existingSecretKey": "client-secret",
				"orchestration.security.authentication.oidc.secret.existingSecret": "test-orch-secret",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.client.auth.client-id": "test-client-id",
				"configmapApplication.camunda.client.auth.scope":     "",
			},
		},
		{
			Name: "TestContainerSetAuthMethodGlobally",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"connectors.contextPath":                "/connectors",
				"global.security.authentication.method": "oidc",
				"global.identity.auth.tokenUrl":         "http://camunda-keycloak/auth/realms/camunda-platform/protocol/openid-connect/token",
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
				s.Require().Equal("http://camunda-keycloak/auth/realms/camunda-platform/protocol/openid-connect/token", configmapApplication.Camunda.Client.Auth.TokenUrl)
			},
		},
		{
			Name: "TestContainerSetAuthMethodConnectors",
			Values: map[string]string{
				"connectors.enabled":                        "true",
				"connectors.contextPath":                    "/connectors",
				"connectors.security.authentication.method": "oidc",
				"global.identity.auth.tokenUrl":             "http://camunda-keycloak/auth/realms/camunda-platform/protocol/openid-connect/token",
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
				s.Require().Equal("http://camunda-keycloak/auth/realms/camunda-platform/protocol/openid-connect/token", configmapApplication.Camunda.Client.Auth.TokenUrl)
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapTemplateTest) TestExtraConfigurationSpringImport() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestExtraConfigWithSpringImportDefault",
			Values: map[string]string{
				"connectors.enabled":                       "true",
				"connectors.extraConfiguration[0].file":    "custom-spring.yaml",
				"connectors.extraConfiguration[0].content": "some: config",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// spring.config.import should include the file
				s.Require().Contains(applicationYaml, "optional:file:/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				// File content should be in ConfigMap
				s.Require().Contains(configmap.Data["custom-spring.yaml"], "some: config",
					"File content should be present in ConfigMap")
			},
		},
		{
			Name: "TestExtraConfigWithSpringImportFalse",
			Values: map[string]string{
				"connectors.enabled":                            "true",
				"connectors.extraConfiguration[0].file":         "log4j2-spring.xml",
				"connectors.extraConfiguration[0].springImport": "false",
				"connectors.extraConfiguration[0].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// spring.config.import should NOT include the file
				s.Require().NotContains(applicationYaml, "log4j2-spring.xml",
					"File with springImport: false should not be in spring.config.import")
				// spring.config.import block should not be rendered at all
				s.Require().NotContains(applicationYaml, "config:",
					"spring.config.import block should not be rendered when all entries have springImport: false")
				// File content should still be in ConfigMap
				s.Require().Contains(configmap.Data["log4j2-spring.xml"], "<Configuration/>",
					"File content should be present in ConfigMap even with springImport: false")
			},
		},
		{
			Name: "TestExtraConfigMixedSpringImport",
			Values: map[string]string{
				"connectors.enabled":                            "true",
				"connectors.extraConfiguration[0].file":         "custom-spring.yaml",
				"connectors.extraConfiguration[0].content":      "some: config",
				"connectors.extraConfiguration[1].file":         "log4j2-spring.xml",
				"connectors.extraConfiguration[1].springImport": "false",
				"connectors.extraConfiguration[1].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// Only custom-spring.yaml should be in spring.config.import
				s.Require().Contains(applicationYaml, "optional:file:/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				s.Require().NotContains(applicationYaml, "log4j2-spring.xml",
					"File with springImport: false should not be in spring.config.import")
				// Both files should be in ConfigMap
				s.Require().Contains(configmap.Data["custom-spring.yaml"], "some: config",
					"First file content should be present in ConfigMap")
				s.Require().Contains(configmap.Data["log4j2-spring.xml"], "<Configuration/>",
					"Second file content should be present in ConfigMap even with springImport: false")
			},
		},
	}

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

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

package optimize

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
		templates: []string{"templates/optimize/configmap.yaml"},
	})
}

func (s *ConfigMapTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:                 "TestContainerShouldAddContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":     "true",
				"optimize.enabled":     "true",
				"optimize.contextPath": "/optimize",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Equal("/optimize", configmapApplication.Container.ContextPath)
			},
		}, {
			Name:                 "TestCustomZeebeName",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                        "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
				"optimize.database.elasticsearch.prefix":  "custom-prefix",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Equal("custom-prefix", configmapApplication.Zeebe.Name)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapTemplateTest) TestDatabaseOverrides() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestElasticsearchPrefixFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                        "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
				"optimize.database.elasticsearch.prefix":  "component-prefix",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				s.Require().Equal("component-prefix", configmapApplication.Zeebe.Name)
			},
		},
		{
			Name: "TestElasticsearchPortFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                         "true",
				"optimize.enabled":                         "true",
				"optimize.database.elasticsearch.enabled":  "true",
				"optimize.database.elasticsearch.url.port": "9201",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				s.Require().Equal(9201, configmapApplication.Es.Connection.Nodes[0].HttpPort)
			},
		},
		{
			Name: "TestElasticsearchExternalSecurityUsernameFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                              "true",
				"optimize.enabled":                              "true",
				"optimize.database.elasticsearch.enabled":       "true",
				"optimize.database.elasticsearch.external":      "true",
				"optimize.database.elasticsearch.auth.username": "optimize-es-user",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				s.Require().Equal("optimize-es-user", configmapApplication.Es.Security.Username)
			},
		},
		{
			Name: "TestElasticsearchSslEnabledWhenProtocolHttpsFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                             "true",
				"optimize.enabled":                             "true",
				"optimize.database.elasticsearch.enabled":      "true",
				"optimize.database.elasticsearch.external":     "true",
				"optimize.database.elasticsearch.url.protocol": "https",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				s.Require().Equal("true", configmapApplication.Es.Security.Ssl.Enabled)
			},
		},
		{
			Name: "TestElasticsearchNoSecuritySectionWhenNotExternal",
			Values: map[string]string{
				"identity.enabled":                        "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				s.Require().Empty(configmapApplication.Es.Security.Username)
			},
		},
		{
			Name: "TestOpensearchPrefixFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"optimize.enabled":                      "true",
				"optimize.database.opensearch.enabled":  "true",
				"optimize.database.opensearch.url.host": "opensearch-host",
				"optimize.database.opensearch.prefix":   "os-component-prefix",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				s.Require().Equal("os-component-prefix", configmapApplication.Zeebe.Name)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapTemplateTest) TestExtraConfigurationSpringImport() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestExtraConfigWithSpringImportDefault",
			Values: map[string]string{
				"identity.enabled":                       "true",
				"optimize.enabled":                       "true",
				"optimize.extraConfiguration[0].file":    "custom-spring.yaml",
				"optimize.extraConfiguration[0].content": "some: config",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationCcsmYaml := configmap.Data["application-ccsm.yaml"]
				// spring.config.import should include the file
				s.Require().Contains(applicationCcsmYaml, "optional:file:/optimize/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				// File content should be in ConfigMap
				s.Require().Contains(configmap.Data["custom-spring.yaml"], "some: config",
					"File content should be present in ConfigMap")
			},
		},
		{
			Name: "TestExtraConfigWithSpringImportFalse",
			Values: map[string]string{
				"identity.enabled":                            "true",
				"optimize.enabled":                            "true",
				"optimize.extraConfiguration[0].file":         "log4j2-spring.xml",
				"optimize.extraConfiguration[0].springImport": "false",
				"optimize.extraConfiguration[0].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationCcsmYaml := configmap.Data["application-ccsm.yaml"]
				// spring.config.import should NOT include the file
				s.Require().NotContains(applicationCcsmYaml, "log4j2-spring.xml",
					"File with springImport: false should not be in spring.config.import")
				// spring.config.import block should not be rendered
				s.Require().NotContains(applicationCcsmYaml, "config:",
					"spring.config.import block should not be rendered when all entries have springImport: false")
				// File content should still be in ConfigMap
				s.Require().Contains(configmap.Data["log4j2-spring.xml"], "<Configuration/>",
					"File content should be present in ConfigMap even with springImport: false")
			},
		},
		{
			Name: "TestExtraConfigMixedSpringImport",
			Values: map[string]string{
				"identity.enabled":                            "true",
				"optimize.enabled":                            "true",
				"optimize.extraConfiguration[0].file":         "custom-spring.yaml",
				"optimize.extraConfiguration[0].content":      "some: config",
				"optimize.extraConfiguration[1].file":         "log4j2-spring.xml",
				"optimize.extraConfiguration[1].springImport": "false",
				"optimize.extraConfiguration[1].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationCcsmYaml := configmap.Data["application-ccsm.yaml"]
				// Only custom-spring.yaml should be in spring.config.import
				s.Require().Contains(applicationCcsmYaml, "optional:file:/optimize/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				s.Require().NotContains(applicationCcsmYaml, "log4j2-spring.xml",
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

func (s *ConfigMapTemplateTest) TestOptimizeNativeConfigHonorsExtraConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestPartitionCountAndIndexNameResolvedFromExtraConfiguration",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-extraconfig.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				envConfig := configmap.Data["environment-config.yaml"]
				s.Require().Contains(envConfig, "partitionCount: 7",
					"partitionCount should be resolved from extraConfiguration")
				s.Require().Contains(envConfig, "name: \"gated-index\"",
					"index name should be resolved from extraConfiguration")
			},
		},
		{
			Name:        "TestDeprecatedPartitionCountUsedWithoutExtraConfiguration",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-deprecated.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				s.Require().Contains(configmap.Data["environment-config.yaml"], "partitionCount: 9",
					"deprecated optimize.partitionCount should still apply without extraConfiguration")
			},
		},
		{
			Name:        "TestLargeNumberRendersAsIntegerNotScientificNotation",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-nonscalar.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				envConfig := configmap.Data["environment-config.yaml"]
				s.Require().Contains(envConfig, "partitionCount: 10000000",
					"a whole-number float must render as an integer")
				s.Require().NotContains(envConfig, "1e+07",
					"a large number must not render in scientific notation")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapTemplateTest) TestCamundaSecurityConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestOidcConfigurationIsRendered",
			Values: map[string]string{
				"identity.enabled":                          "true",
				"optimize.enabled":                          "true",
				"global.identity.auth.enabled":              "true",
				"global.identity.auth.optimize.redirectUrl": "https://camunda.example.com/optimize",
				"global.identity.auth.issuer":               "https://camunda.example.com/auth/realms/camunda-platform",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				authConfig := configmap.Data["application-ccsm.yaml"]
				s.Require().NotContains(authConfig, "csl:",
					"Optimize defaults to the CSL chains, so the chart no longer opts in explicitly")
				s.Require().Contains(authConfig, `method: "oidc"`)
				s.Require().Contains(authConfig, `client-id: "optimize"`)
				s.Require().Contains(authConfig, "client-secret: ${VALUES_OPTIMIZE_CLIENT_SECRET:}")
				s.Require().Contains(authConfig, `issuer-uri: "https://camunda.example.com/auth/realms/camunda-platform"`)
				s.Require().Contains(authConfig, `redirect-uri: "https://camunda.example.com/optimize/api/authentication/callback"`)
			},
		},
		{
			Name: "TestKeycloakSetupUsesBackChannelEndpointsInsteadOfDiscovery",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"optimize.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.identity.auth.type":             "KEYCLOAK",
				"global.identity.auth.publicIssuerUrl":  "https://camunda.example.com/auth/realms/camunda-platform",
				"global.identity.auth.issuerBackendUrl": "http://keycloak:80/auth/realms/camunda-platform",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				authConfig := configmap.Data["application-ccsm.yaml"]
				s.Require().NotContains(authConfig, "issuer-uri:",
					"issuer-uri makes CSL run OIDC discovery over the public URL at startup, which breaks wherever the JVM truststore is replaced for a self-signed database")
				s.Require().Contains(authConfig, `authorization-uri: "https://camunda.example.com/auth/realms/camunda-platform/protocol/openid-connect/auth"`)
				s.Require().Contains(authConfig, `jwk-set-uri: "http://keycloak:80/auth/realms/camunda-platform/protocol/openid-connect/certs"`)
				s.Require().Contains(authConfig, `token-uri: "http://keycloak:80/auth/realms/camunda-platform/protocol/openid-connect/token"`)
			},
		},
		{
			Name: "TestAudiencesCoverLoginClientApiAndHub",
			Values: map[string]string{
				"identity.enabled":             "true",
				"optimize.enabled":             "true",
				"global.identity.auth.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				authConfig := configmap.Data["application-ccsm.yaml"]
				s.Require().Contains(authConfig, "audiences:\n          - \"optimize\"\n          - \"optimize-api\"\n          - \"web-modeler-api\"")
			},
		},
		{
			Name: "TestCamundaHubAudienceOverridesWebModeler",
			Values: map[string]string{
				"identity.enabled":                                  "true",
				"optimize.enabled":                                  "true",
				"global.identity.auth.enabled":                      "true",
				"global.identity.auth.camundaHub.clientApiAudience": "camunda-hub-api",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				authConfig := configmap.Data["application-ccsm.yaml"]
				s.Require().Contains(authConfig, `- "camunda-hub-api"`)
				s.Require().NotContains(authConfig, `- "web-modeler-api"`)
			},
		},
		{
			Name: "TestBothSecurityConfigShapesAreRendered",
			Values: map[string]string{
				"identity.enabled":             "true",
				"optimize.enabled":             "true",
				"global.identity.auth.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				// Optimize picks its stack from optimize.security.csl.enabled, whose default differs
				// per image and which an operator can still set to false through 8.10. Rendering both
				// shapes keeps the chart working either way, and lets the escape hatch be the flag
				// alone rather than a chart change.
				s.Require().Contains(configmap.Data["application-ccsm.yaml"], `method: "oidc"`,
					"the CSL chains read camunda.security.*")

				envConfig := configmap.Data["environment-config.yaml"]
				s.Require().Contains(envConfig, "redirectRootUrl:",
					"the legacy chains read environment-config.yaml, which extraConfiguration cannot reach")
				s.Require().Contains(envConfig, `audience: "optimize-api"`)
				s.Require().Contains(envConfig, "jwtSetUri:")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

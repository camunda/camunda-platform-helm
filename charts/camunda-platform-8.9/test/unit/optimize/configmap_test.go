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
				"identity.enabled":            "true",
				"optimize.enabled":            "true",
				"global.elasticsearch.prefix": "custom-prefix",
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
			Name: "TestElasticsearchPrefixOverriddenByOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                       "true",
				"optimize.enabled":                       "true",
				"global.elasticsearch.prefix":            "global-prefix",
				"optimize.database.elasticsearch.prefix": "optimize-prefix",
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

				s.Require().Equal("optimize-prefix", configmapApplication.Zeebe.Name)
			},
		},
		{
			Name: "TestElasticsearchPrefixFallsBackToGlobal",
			Values: map[string]string{
				"identity.enabled":            "true",
				"optimize.enabled":            "true",
				"global.elasticsearch.prefix": "global-prefix",
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

				s.Require().Equal("global-prefix", configmapApplication.Zeebe.Name)
			},
		},
		{
			Name: "TestElasticsearchPortOverriddenInConfigMap",
			Values: map[string]string{
				"identity.enabled":                         "true",
				"optimize.enabled":                         "true",
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
			Name: "TestElasticsearchPortFallsBackToGlobalInConfigMap",
			Values: map[string]string{
				"identity.enabled":              "true",
				"optimize.enabled":              "true",
				"global.elasticsearch.url.port": "9300",
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

				s.Require().Equal(9300, configmapApplication.Es.Connection.Nodes[0].HttpPort)
			},
		},
		{
			Name: "TestElasticsearchExternalSecurityUsernameFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                              "true",
				"optimize.enabled":                              "true",
				"global.elasticsearch.external":                 "true",
				"global.elasticsearch.auth.username":            "global-es-user",
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
			Name: "TestElasticsearchExternalSecurityUsernameFallsBackToGlobal",
			Values: map[string]string{
				"identity.enabled":                   "true",
				"optimize.enabled":                   "true",
				"global.elasticsearch.external":      "true",
				"global.elasticsearch.auth.username": "global-es-user",
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

				s.Require().Equal("global-es-user", configmapApplication.Es.Security.Username)
			},
		},
		{
			Name: "TestElasticsearchSslEnabledWhenProtocolHttpsFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                             "true",
				"optimize.enabled":                             "true",
				"global.elasticsearch.external":                "true",
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
				"identity.enabled": "true",
				"optimize.enabled": "true",
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
			Name: "TestOpensearchPrefixFallsBackThroughChain",
			Values: map[string]string{
				"identity.enabled":             "true",
				"optimize.enabled":             "true",
				"global.elasticsearch.enabled": "false",
				"elasticsearch.enabled":        "false",
				"global.opensearch.enabled":    "true",
				"global.opensearch.url.host":   "opensearch-host",
				"global.opensearch.prefix":     "os-prefix",
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

				s.Require().Equal("os-prefix", configmapApplication.Zeebe.Name)
			},
		},
		{
			Name: "TestOpensearchPrefixOverriddenByOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                    "true",
				"optimize.enabled":                    "true",
				"global.elasticsearch.enabled":        "false",
				"elasticsearch.enabled":               "false",
				"global.opensearch.enabled":           "true",
				"global.opensearch.url.host":          "opensearch-host",
				"optimize.database.opensearch.prefix": "my-optimize-os-prefix",
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

				s.Require().Equal("my-optimize-os-prefix", configmapApplication.Zeebe.Name)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

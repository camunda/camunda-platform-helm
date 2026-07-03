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
		{
			Name:                 "TestOpenSearchNodeRenderedInEsSection",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"optimize.enabled":             "true",
				"elasticsearch.enabled":        "false",
				"global.elasticsearch.enabled": "false",
				"global.opensearch.enabled":    "true",
				"global.opensearch.url.host":   "my-opensearch",
				"global.opensearch.url.port":   "9200",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication OptimizeConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["environment-config.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				s.Require().Len(configmapApplication.Es.Connection.Nodes, 1)
				s.Require().Equal("my-opensearch", configmapApplication.Es.Connection.Nodes[0].Host)
				s.Require().Equal(9200, configmapApplication.Es.Connection.Nodes[0].HttpPort)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

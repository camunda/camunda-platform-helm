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

package zeebe

import (
	"camunda-platform/test/unit/camunda"
	"camunda-platform/test/unit/testhelpers"
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type configmapTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigmapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &configmapTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/zeebe/configmap.yaml"},
	})
}

func TestGoldenConfigmapWithLog4j2(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "configmap-log4j2",
		Templates:      []string{"templates/zeebe/configmap.yaml"},
		SetValues:      map[string]string{"zeebe.log4j2": "<xml>\n</xml>"},
	})
}

func (s *configmapTemplateTest) TestContainerShouldContainExporterClassPerDefault() {
	// given
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication camunda.ZeebeApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)
	helm.UnmarshalK8SYaml(s.T(), configmap.Data["application.yaml"], &configmapApplication)

	// then
	s.Require().Equal("io.camunda.zeebe.exporter.ElasticsearchExporter", configmapApplication.Zeebe.Broker.Exporters.Elasticsearch.ClassName)
}

func (s *configmapTemplateTest) TestRequestBodySizeConfiguresBrokerMessageSize() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestRequestBodySizeConfiguresBrokerMessageSize",
			Values: map[string]string{
				"global.config.requestBodySize": "50MB",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				applicationYaml := configmap.Data["application.yaml"]

				require.NotContains(t, applicationYaml, "maxMessageSize: \"50MB\"")
				require.Equal(t, 0, strings.Count(applicationYaml, "maxMessageSize:"))
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *configmapTemplateTest) TestZeebeMaxMessageSizeUsesEngineDefault() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestZeebeMaxMessageSizeUsesEngineDefaultWithDefaultValues",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				applicationYaml := configmap.Data["application.yaml"]

				// the chart must never render a Zeebe message-size key, keeping the engine's 4MB default
				require.Equal(t, 0, strings.Count(applicationYaml, "maxMessageSize:"))
			},
		},
		{
			Name: "TestZeebeMaxMessageSizeUsesEngineDefaultWithRequestBodySizeOverride",
			Values: map[string]string{
				"global.config.requestBodySize": "50MB",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				applicationYaml := configmap.Data["application.yaml"]

				// requestBodySize must not leak into a Zeebe message-size key
				require.Equal(t, 0, strings.Count(applicationYaml, "maxMessageSize:"))
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

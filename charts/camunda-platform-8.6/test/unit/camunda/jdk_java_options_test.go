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
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

const jdkJavaOptionsConfigMap = "camunda-platform-test-jdk-java-options"

func TestGoldenJdkJavaOptionsConfigMap(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda",
		GoldenFileName: "configmap-jdk-java-options",
		Templates:      []string{"templates/camunda/configmap-jdk-java-options.yaml"},
	})
}

type JdkJavaOptionsTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestJdkJavaOptionsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &JdkJavaOptionsTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func requireJdkJavaOptionsEnvFrom(t *testing.T, output string, err error) {
	require.NoError(t, err)

	var statefulSet appsv1.StatefulSet
	helm.UnmarshalK8SYaml(t, output, &statefulSet)

	var configMapRefs []string
	for _, envFrom := range statefulSet.Spec.Template.Spec.Containers[0].EnvFrom {
		if envFrom.ConfigMapRef != nil {
			configMapRefs = append(configMapRefs, envFrom.ConfigMapRef.Name)
		}
	}
	require.Contains(t, configMapRefs, jdkJavaOptionsConfigMap)
}

func (s *JdkJavaOptionsTemplateTest) TestDifferentValuesInputs() {
	overriddenExtraEnvVars := map[string]string{
		"identityKeycloak.extraEnvVars[0].name":   "FOO",
		"identityKeycloak.extraEnvVars[0].value":  "bar",
		"elasticsearch.extraEnvVars[0].name":      "FOO",
		"elasticsearch.extraEnvVars[0].value":     "bar",
		"elasticsearch.extraEnvVarsSecret":        "user-secret",
		"elasticsearch.data.replicaCount":         "1",
		"elasticsearch.coordinating.replicaCount": "1",
		"elasticsearch.ingest.enabled":            "true",
		"elasticsearch.ingest.replicaCount":       "1",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:                    "TestKeycloakKeepsJdkJavaOptionsWithOverriddenExtraEnvVars",
			Values:                  overriddenExtraEnvVars,
			RenderTemplateExtraArgs: []string{"--show-only", "charts/identityKeycloak/templates/statefulset.yaml"},
			Verifier:                requireJdkJavaOptionsEnvFrom,
		},
	}
	for _, role := range []string{"master", "data", "coordinating", "ingest"} {
		testCases = append(testCases, testhelpers.TestCase{
			Name:                    "TestElasticsearch" + strings.ToUpper(role[:1]) + role[1:] + "KeepsJdkJavaOptionsWithOverriddenExtraEnvVars",
			Values:                  overriddenExtraEnvVars,
			RenderTemplateExtraArgs: []string{"--show-only", "charts/elasticsearch/templates/" + role + "/statefulset.yaml"},
			Verifier:                requireJdkJavaOptionsEnvFrom,
		})
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, []string{}, testCases)
}

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

package web_modeler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type CamundaHubShimTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestCamundaHubShimTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &CamundaHubShimTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func (s *CamundaHubShimTemplateTest) TestEnablementMatrix() {
	testCases := []struct {
		name                string
		values              map[string]string
		expectWebModeler    bool
		expectConsoleFlag   string
		expectConsoleEnvVar bool
	}{
		{
			name: "HubDisabledLegacyUnset",
			values: map[string]string{
				"camundaHub.enabled": "false",
			},
		},
		{
			name: "HubDisabledLegacyFalse",
			values: map[string]string{
				"camundaHub.enabled":           "false",
				"webModeler.enabled":           "false",
				"console.enabled":              "false",
				"global.elasticsearch.enabled": "true",
			},
		},
		{
			name: "HubDisabledWebModelerTrueConsoleFalse",
			values: map[string]string{
				"camundaHub.enabled":                  "false",
				"webModeler.enabled":                  "true",
				"console.enabled":                     "false",
				"webModeler.restapi.mail.fromAddress": "example@example.com",
			},
			expectWebModeler:    true,
			expectConsoleFlag:   "false",
			expectConsoleEnvVar: true,
		},
		{
			name: "HubDisabledWebModelerFalseConsoleTrue",
			values: map[string]string{
				"camundaHub.enabled": "false",
				"webModeler.enabled": "false",
				"console.enabled":    "true",
			},
		},
		{
			name: "HubDisabledBothLegacyTrue",
			values: map[string]string{
				"camundaHub.enabled":                  "false",
				"webModeler.enabled":                  "true",
				"console.enabled":                     "true",
				"webModeler.restapi.mail.fromAddress": "example@example.com",
			},
			expectWebModeler:    true,
			expectConsoleFlag:   "true",
			expectConsoleEnvVar: true,
		},
		{
			name: "HubEnabledLegacyUnset",
			values: map[string]string{
				"camundaHub.enabled": "true",
			},
			expectWebModeler:    true,
			expectConsoleFlag:   "true",
			expectConsoleEnvVar: true,
		},
		{
			name: "HubEnabledLegacyFalse",
			values: map[string]string{
				"camundaHub.enabled": "true",
				"webModeler.enabled": "false",
				"console.enabled":    "false",
			},
			expectWebModeler:    true,
			expectConsoleFlag:   "true",
			expectConsoleEnvVar: true,
		},
		{
			name: "HubEnabledWebModelerTrueConsoleFalse",
			values: map[string]string{
				"camundaHub.enabled": "true",
				"webModeler.enabled": "true",
				"console.enabled":    "false",
			},
			expectWebModeler:    true,
			expectConsoleFlag:   "true",
			expectConsoleEnvVar: true,
		},
		{
			name: "HubEnabledWebModelerFalseConsoleTrue",
			values: map[string]string{
				"camundaHub.enabled": "true",
				"webModeler.enabled": "false",
				"console.enabled":    "true",
			},
			expectWebModeler:    true,
			expectConsoleFlag:   "true",
			expectConsoleEnvVar: true,
		},
		{
			name: "HubEnabledBothLegacyTrue",
			values: map[string]string{
				"camundaHub.enabled": "true",
				"webModeler.enabled": "true",
				"console.enabled":    "true",
			},
			expectWebModeler:    true,
			expectConsoleFlag:   "true",
			expectConsoleEnvVar: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			output, err := s.renderWebModelerRestAPI(tc.values)

			if !tc.expectWebModeler {
				if err != nil {
					s.Require().ErrorContains(err, "could not find template templates/web-modeler/deployment-restapi.yaml in chart")
					return
				}
				s.Require().Empty(strings.TrimSpace(output))
				return
			}

			s.Require().NoError(err)

			deployment := s.unmarshalDeployment(output)
			s.Require().Equal(s.release+"-web-modeler-restapi", deployment.Name)
			consoleEnv := envVarByName(deployment.Spec.Template.Spec.Containers[0].Env, "CAMUNDA_MODELER_FEATURE_CONSOLE_ENABLED")
			if tc.expectConsoleEnvVar {
				s.Require().NotNil(consoleEnv)
				s.Require().Equal(tc.expectConsoleFlag, consoleEnv.Value)
			} else {
				s.Require().Nil(consoleEnv)
			}
		})
	}
}

func (s *CamundaHubShimTemplateTest) TestDoubleEnableRendersSingleWebModelerWorkloadSet() {
	values := map[string]string{
		"camundaHub.enabled":                  "true",
		"webModeler.enabled":                  "true",
		"console.enabled":                     "true",
		"webModeler.restapi.mail.fromAddress": "example@example.com",
	}
	output, err := s.renderWebModeler(values, []string{
		"templates/web-modeler/deployment-restapi.yaml",
		"templates/web-modeler/deployment-websockets.yaml",
		"templates/web-modeler/service-restapi.yaml",
		"templates/web-modeler/service-websockets.yaml",
		"templates/web-modeler/serviceaccount.yaml",
	})
	s.Require().NoError(err)

	s.Require().Equal(map[string]int{
		"Deployment/" + s.release + "-web-modeler-restapi":    1,
		"Deployment/" + s.release + "-web-modeler-websockets": 1,
		"Service/" + s.release + "-web-modeler-restapi":       1,
		"Service/" + s.release + "-web-modeler-websockets":    1,
		"ServiceAccount/" + s.release + "-web-modeler":        1,
	}, objectCounts(output))
}

func (s *CamundaHubShimTemplateTest) TestCamundaHubValuesTakePrecedenceOverLegacyValues() {
	values := map[string]string{
		"camundaHub.enabled":                             "true",
		"camundaHub.webModeler.restapi.replicas":         "3",
		"camundaHub.webModeler.restapi.mail.fromAddress": "hub@example.com",
		"webModeler.enabled":                             "true",
		"webModeler.restapi.replicas":                    "1",
		"webModeler.restapi.mail.fromAddress":            "legacy@example.com",
	}
	output, err := s.renderWebModelerRestAPI(values)
	s.Require().NoError(err)

	deployment := s.unmarshalDeployment(output)
	s.Require().Equal(int32(3), *deployment.Spec.Replicas)
}

func (s *CamundaHubShimTemplateTest) TestLegacyOnlyValuesStillApply() {
	values := map[string]string{
		"camundaHub.enabled":                  "false",
		"webModeler.enabled":                  "true",
		"webModeler.restapi.replicas":         "2",
		"webModeler.restapi.mail.fromAddress": "legacy@example.com",
	}
	output, err := s.renderWebModelerRestAPI(values)
	s.Require().NoError(err)

	deployment := s.unmarshalDeployment(output)
	s.Require().Equal(int32(2), *deployment.Spec.Replicas)
}

func (s *CamundaHubShimTemplateTest) TestFalsyCamundaHubOverrideDoesNotOverrideTruthyLegacyValue() {
	values := map[string]string{
		"camundaHub.enabled": "true",
		"camundaHub.webModeler.restapi.readinessProbe.enabled": "false",
		"webModeler.enabled":                        "true",
		"webModeler.restapi.readinessProbe.enabled": "true",
		"webModeler.restapi.mail.fromAddress":       "example@example.com",
	}
	output, err := s.renderWebModelerRestAPI(values)
	s.Require().NoError(err)

	deployment := s.unmarshalDeployment(output)
	s.Require().NotNil(deployment.Spec.Template.Spec.Containers[0].ReadinessProbe)
}

func (s *CamundaHubShimTemplateTest) TestResourceNamesStableBetweenLegacyAndCamundaHubValues() {
	legacyValues := map[string]string{
		"camundaHub.enabled":                  "false",
		"webModeler.enabled":                  "true",
		"webModeler.restapi.mail.fromAddress": "example@example.com",
	}
	hubValues := map[string]string{
		"camundaHub.enabled": "true",
	}
	templates := []string{
		"templates/web-modeler/deployment-restapi.yaml",
		"templates/web-modeler/deployment-websockets.yaml",
		"templates/web-modeler/service-restapi.yaml",
		"templates/web-modeler/service-websockets.yaml",
		"templates/web-modeler/serviceaccount.yaml",
	}

	legacyOutput, err := s.renderWebModeler(legacyValues, templates)
	s.Require().NoError(err)
	hubOutput, err := s.renderWebModeler(hubValues, templates)
	s.Require().NoError(err)

	s.Require().Equal(objectNamesByKind(legacyOutput), objectNamesByKind(hubOutput))
}

func (s *CamundaHubShimTemplateTest) TestOIDCClientIDPreserved() {
	for _, camundaHubEnabled := range []string{"false", "true"} {
		s.Run("camundaHub.enabled="+camundaHubEnabled, func() {
			values := map[string]string{
				"camundaHub.enabled":           camundaHubEnabled,
				"webModeler.enabled":           "true",
				"global.identity.auth.enabled": "true",
				"global.elasticsearch.enabled": "true",
			}
			config := s.renderRestAPIConfigMap(values)
			s.Require().Equal("web-modeler", config.Camunda.Modeler.OAuth2.ClientId)
		})
	}
}

func (s *CamundaHubShimTemplateTest) TestConsoleFeatureFlagTracksShimHelper() {
	testCases := []struct {
		name     string
		values   map[string]string
		expected string
	}{
		{
			name: "LegacyConsoleEnabled",
			values: map[string]string{
				"camundaHub.enabled": "false",
				"webModeler.enabled": "true",
				"console.enabled":    "true",
			},
			expected: "true",
		},
		{
			name: "CamundaHubEnabled",
			values: map[string]string{
				"camundaHub.enabled": "true",
				"webModeler.enabled": "false",
				"console.enabled":    "false",
			},
			expected: "true",
		},
		{
			name: "ConsoleDisabled",
			values: map[string]string{
				"camundaHub.enabled": "false",
				"webModeler.enabled": "true",
				"console.enabled":    "false",
			},
			expected: "false",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			output, err := s.renderWebModelerRestAPI(tc.values)
			s.Require().NoError(err)
			deployment := s.unmarshalDeployment(output)
			consoleEnv := envVarByName(deployment.Spec.Template.Spec.Containers[0].Env, "CAMUNDA_MODELER_FEATURE_CONSOLE_ENABLED")
			s.Require().NotNil(consoleEnv)
			s.Require().Equal(tc.expected, consoleEnv.Value)
		})
	}
}

func (s *CamundaHubShimTemplateTest) TestRemovedConsoleEnvVarsAreNotRendered() {
	values := map[string]string{
		"camundaHub.enabled":                  "true",
		"webModeler.enabled":                  "true",
		"console.enabled":                     "true",
		"webModeler.restapi.mail.fromAddress": "example@example.com",
	}
	output, err := s.renderWebModelerRestAPI(values)
	s.Require().NoError(err)
	s.Require().NotContains(output, "CAMUNDA_CONSOLE_")
}

func (s *CamundaHubShimTemplateTest) renderWebModelerRestAPI(values map[string]string) (string, error) {
	return s.renderWebModeler(values, []string{"templates/web-modeler/deployment-restapi.yaml"})
}

func (s *CamundaHubShimTemplateTest) renderWebModeler(values map[string]string, templates []string) (string, error) {
	for key, value := range map[string]string{
		"global.elasticsearch.enabled":        "true",
		"identity.enabled":                    "true",
		"webModeler.restapi.mail.fromAddress": "example@example.com",
	} {
		if _, ok := values[key]; !ok {
			values[key] = value
		}
	}
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	return helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, templates)
}

func (s *CamundaHubShimTemplateTest) unmarshalDeployment(output string) appsv1.Deployment {
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)
	return deployment
}

func (s *CamundaHubShimTemplateTest) renderRestAPIConfigMap(values map[string]string) WebModelerRestAPIApplicationYAML {
	for key, value := range requiredValues {
		if _, ok := values[key]; !ok {
			values[key] = value
		}
	}
	if _, ok := values["identity.enabled"]; !ok {
		values["identity.enabled"] = "true"
	}
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, []string{"templates/web-modeler/configmap-restapi.yaml"})
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	var application WebModelerRestAPIApplicationYAML
	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &application)
	s.Require().NoError(err)
	return application
}

func envVarByName(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range envVars {
		if envVars[i].Name == name {
			return &envVars[i]
		}
	}
	return nil
}

func objectCounts(output string) map[string]int {
	counts := map[string]int{}
	for _, document := range strings.Split(output, "---") {
		object := parseObject(document)
		if object.Kind == "" || object.Metadata.Name == "" {
			continue
		}
		counts[object.Kind+"/"+object.Metadata.Name]++
	}
	return counts
}

func objectNamesByKind(output string) map[string][]string {
	names := map[string][]string{}
	for _, document := range strings.Split(output, "---") {
		object := parseObject(document)
		if object.Kind == "" || object.Metadata.Name == "" {
			continue
		}
		names[object.Kind] = append(names[object.Kind], object.Metadata.Name)
	}
	return names
}

func parseObject(document string) struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
} {
	var object struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	}
	_ = yaml.Unmarshal([]byte(document), &object)
	return object
}

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
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

func TestCamundaHubUpgradePhases(t *testing.T) {
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	testCases := []struct {
		name               string
		phase              string
		restapiReplicas    int32
		websocketsReplicas int32
		strategy           appsv1.DeploymentStrategyType
	}{
		{
			name:               "normal",
			phase:              "normal",
			restapiReplicas:    3,
			websocketsReplicas: 1,
			strategy:           appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			name:               "quiesce",
			phase:              "quiesce",
			restapiReplicas:    0,
			websocketsReplicas: 0,
			strategy:           appsv1.RollingUpdateDeploymentStrategyType,
		},
		{
			name:               "migrate",
			phase:              "migrate",
			restapiReplicas:    1,
			websocketsReplicas: 0,
			strategy:           appsv1.RollingUpdateDeploymentStrategyType,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			values := map[string]string{
				"camundaHub.enabled":                  "true",
				"camundaHub.upgrade.phase":            testCase.phase,
				"camundaHub.restapi.replicas":         "3",
				"camundaHub.restapi.mail.fromAddress": "example@example.com",
				"global.elasticsearch.enabled":        "true",
				"identity.enabled":                    "true",
			}

			output := helm.RenderTemplate(t, &helm.Options{SetValues: values}, chartPath, "camunda-platform-test", []string{
				"templates/web-modeler/deployment-restapi.yaml",
				"templates/web-modeler/deployment-websockets.yaml",
				"templates/web-modeler/service-restapi.yaml",
				"templates/web-modeler/service-websockets.yaml",
			})
			deployments, services := decodeUpgradePhaseResources(t, output)
			require.Contains(t, deployments, "camunda-platform-test-web-modeler-restapi")
			require.Contains(t, deployments, "camunda-platform-test-web-modeler-websockets")
			restapiDeployment := deployments["camunda-platform-test-web-modeler-restapi"]
			require.Equal(t, testCase.restapiReplicas, *restapiDeployment.Spec.Replicas)
			require.Equal(t, testCase.strategy, restapiDeployment.Spec.Strategy.Type)
			if testCase.phase == "migrate" {
				require.Zero(t, restapiDeployment.Spec.Strategy.RollingUpdate.MaxSurge.IntValue())
				require.Equal(t, "100%", restapiDeployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal)
			}
			require.Equal(t, testCase.phase, restapiDeployment.Spec.Template.Labels["camunda.io/upgrade-phase"])

			websocketsDeployment := deployments["camunda-platform-test-web-modeler-websockets"]
			require.Equal(t, testCase.websocketsReplicas, *websocketsDeployment.Spec.Replicas)
			require.Equal(t, testCase.phase, websocketsDeployment.Spec.Template.Labels["camunda.io/upgrade-phase"])

			for _, service := range services {
				require.Equal(t, "normal", service.Spec.Selector["camunda.io/upgrade-phase"])
			}
			require.Len(t, deployments, 2)
			require.Len(t, services, 2)
		})
	}
}

func decodeUpgradePhaseResources(t *testing.T, output string) (map[string]appsv1.Deployment, map[string]corev1.Service) {
	t.Helper()
	deployments := map[string]appsv1.Deployment{}
	services := map[string]corev1.Service{}
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)
	for {
		var object map[string]any
		err := decoder.Decode(&object)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if len(object) == 0 {
			continue
		}
		data, err := json.Marshal(object)
		require.NoError(t, err)

		switch object["kind"] {
		case "Deployment":
			var deployment appsv1.Deployment
			require.NoError(t, json.Unmarshal(data, &deployment))
			deployments[deployment.Name] = deployment
		case "Service":
			var service corev1.Service
			require.NoError(t, json.Unmarshal(data, &service))
			services[service.Name] = service
		}
	}
	return deployments, services
}

func TestCamundaHubUpgradePhaseLabelCannotBeOverridden(t *testing.T) {
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	for _, label := range []struct {
		name     string
		labelKey string
		template string
	}{
		{name: "restapi", labelKey: "camundaHub.restapi.podLabels.camunda\\.io/upgrade-phase", template: "templates/web-modeler/deployment-restapi.yaml"},
		{name: "websockets", labelKey: "camundaHub.websockets.podLabels.camunda\\.io/upgrade-phase", template: "templates/web-modeler/deployment-websockets.yaml"},
		{name: "global labels", labelKey: "global.labels.camunda\\.io/upgrade-phase", template: "templates/web-modeler/deployment-restapi.yaml"},
		{name: "global common labels", labelKey: "global.commonLabels.camunda\\.io/upgrade-phase", template: "templates/web-modeler/deployment-restapi.yaml"},
	} {
		label := label
		t.Run(label.name, func(t *testing.T) {
			_, err := helm.RenderTemplateE(t, &helm.Options{SetValues: map[string]string{
				"camundaHub.enabled":                  "true",
				"camundaHub.upgrade.phase":            "migrate",
				"camundaHub.restapi.mail.fromAddress": "example@example.com",
				"global.elasticsearch.enabled":        "true",
				"identity.enabled":                    "true",
				label.labelKey:                        "normal",
			}}, chartPath, "camunda-platform-test", []string{label.template})

			require.ErrorContains(t, err, "camunda.io/upgrade-phase is reserved")
		})
	}
}

func TestCamundaHubUpgradePhaseRendersGitOpsWarning(t *testing.T) {
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	for _, phase := range []string{"quiesce", "migrate"} {
		phase := phase
		t.Run(phase, func(t *testing.T) {
			output := helm.RenderTemplate(t, &helm.Options{SetValues: map[string]string{
				"camundaHub.upgrade.phase": phase,
				"orchestration.enabled":    "false",
			}}, chartPath, "camunda-platform-test", []string{"templates/common/configmap-warnings.yaml"})
			require.Contains(t, strings.ToLower(output), phase)
			require.Contains(t, output, "8.9 to 8.10 database migration")
		})
	}
}

func TestCamundaHubUpgradePhaseSupportsLegacyEnablement(t *testing.T) {
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	output := helm.RenderTemplate(t, &helm.Options{SetValues: map[string]string{
		"camundaHub.upgrade.phase":            "migrate",
		"global.elasticsearch.enabled":        "true",
		"identity.enabled":                    "true",
		"webModeler.enabled":                  "true",
		"webModeler.restapi.mail.fromAddress": "example@example.com",
	}}, chartPath, "camunda-platform-test", []string{"templates/web-modeler/deployment-restapi.yaml"})

	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, output, &deployment)
	require.Equal(t, int32(1), *deployment.Spec.Replicas)
	require.Equal(t, appsv1.RollingUpdateDeploymentStrategyType, deployment.Spec.Strategy.Type)
}

func TestCamundaHubUpgradePhaseRejectsInvalidValue(t *testing.T) {
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	_, err = helm.RenderTemplateE(t, &helm.Options{SetValues: map[string]string{
		"camundaHub.enabled":                  "true",
		"camundaHub.upgrade.phase":            "invalid",
		"global.elasticsearch.enabled":        "true",
		"identity.enabled":                    "true",
		"webModeler.restapi.mail.fromAddress": "example@example.com",
	}}, chartPath, "camunda-platform-test", []string{"templates/web-modeler/deployment-restapi.yaml"})

	require.ErrorContains(t, err, "value must be one of 'normal', 'quiesce', 'migrate'")
}

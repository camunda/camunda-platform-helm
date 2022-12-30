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

package web_modeler

import (
	corev1 "k8s.io/api/core/v1"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

type webappDeploymentTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestWebappDeploymentTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &webappDeploymentTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/web-modeler/templates/deployment-webapp.yaml"},
	})
}

func (s *webappDeploymentTemplateTest) TestContainerShouldSetCorrectKeycloakServiceUrl() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"web-modeler.enabled":                   "true",
			"global.identity.keycloak.url.protocol": "http",
			"global.identity.keycloak.url.host":     "keycloak",
			"global.identity.keycloak.url.port":     "80",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env,
		corev1.EnvVar{
			Name:  "KEYCLOAK_JWKS_URL",
			Value: "http://keycloak:80/auth/realms/camunda-platform/protocol/openid-connect/certs",
		})
}

func (s *webappDeploymentTemplateTest) TestContainerShouldSetCorrectIdentityServiceUrlWithFullnameOverride() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"web-modeler.enabled":              "true",
			"global.identity.fullnameOverride": "custom-identity-fullname",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env, corev1.EnvVar{Name: "IDENTITY_BASE_URL", Value: "http://custom-identity-fullname:80"})
}

func (s *webappDeploymentTemplateTest) TestContainerShouldSetCorrectIdentityServiceUrlWithNameOverride() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"web-modeler.enabled":          "true",
			"global.identity.nameOverride": "custom-identity",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env,
		corev1.EnvVar{Name: "IDENTITY_BASE_URL", Value: "http://camunda-platform-test-custom-identity:80"})
}

func (s *webappDeploymentTemplateTest) TestContainerShouldSetCorrectIdentityServiceUrlWitCustomPort() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"web-modeler.enabled":          "true",
			"global.identity.service.port": "8888",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env,
		corev1.EnvVar{Name: "IDENTITY_BASE_URL", Value: "http://camunda-platform-test-identity:8888"})
}

func (s *webappDeploymentTemplateTest) TestContainerShouldSetCorrectClientPusherConfigurationWithIngressTlsEnabled() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"web-modeler.enabled":                        "true",
			"web-modeler.ingress.enabled":                "true",
			"web-modeler.ingress.websockets.host":        "modeler-ws.example.com",
			"web-modeler.ingress.websockets.tls.enabled": "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env, corev1.EnvVar{Name: "CLIENT_PUSHER_HOST", Value: "modeler-ws.example.com"})
	s.Require().Contains(env, corev1.EnvVar{Name: "CLIENT_PUSHER_PORT", Value: "443"})
	s.Require().Contains(env, corev1.EnvVar{Name: "CLIENT_PUSHER_FORCE_TLS", Value: "true"})
}

func (s *webappDeploymentTemplateTest) TestContainerShouldSetCorrectClientPusherConfigurationWithIngressTlsDisabled() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"web-modeler.enabled":                        "true",
			"web-modeler.ingress.enabled":                "true",
			"web-modeler.ingress.websockets.host":        "modeler-ws.example.com",
			"web-modeler.ingress.websockets.tls.enabled": "false",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env, corev1.EnvVar{Name: "CLIENT_PUSHER_HOST", Value: "modeler-ws.example.com"})
	s.Require().Contains(env, corev1.EnvVar{Name: "CLIENT_PUSHER_PORT", Value: "80"})
	s.Require().Contains(env, corev1.EnvVar{Name: "CLIENT_PUSHER_FORCE_TLS", Value: "false"})
}

func (s *webappDeploymentTemplateTest) TestContainerShouldSetServerHttpsOnly() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"web-modeler.enabled":                         "true",
			"global.identity.auth.webModeler.redirectUrl": "https://modeler.example.com",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_HTTPS_ONLY", Value: "true"})
}

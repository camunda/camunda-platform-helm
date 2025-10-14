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
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

type WebappDeploymentTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestWebappDeploymentTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &WebappDeploymentTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/web-modeler/deployment-webapp.yaml"},
	})
}

func (s *WebappDeploymentTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerStartupProbe",
			Values: map[string]string{
				"identity.enabled":                         "true",
				"webModeler.enabled":                       "true",
				"webModeler.restapi.mail.fromAddress":      "example@example.com",
				"webModeler.webapp.startupProbe.enabled":   "true",
				"webModeler.webapp.startupProbe.probePath": "/healthz",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0].StartupProbe

				s.Require().Equal("/healthz", probe.HTTPGet.Path)
				s.Require().Equal("http-management", probe.HTTPGet.Port.StrVal)
			},
		}, {
			Name: "TestContainerReadinessProbe",
			Values: map[string]string{
				"identity.enabled":                           "true",
				"webModeler.enabled":                         "true",
				"webModeler.restapi.mail.fromAddress":        "example@example.com",
				"webModeler.webapp.readinessProbe.enabled":   "true",
				"webModeler.webapp.readinessProbe.probePath": "/healthz",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe

				s.Require().Equal("/healthz", probe.HTTPGet.Path)
				s.Require().Equal("http-management", probe.HTTPGet.Port.StrVal)
			},
		}, {
			Name: "TestContainerLivenessProbe",
			Values: map[string]string{
				"identity.enabled":                          "true",
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "example@example.com",
				"webModeler.webapp.livenessProbe.enabled":   "true",
				"webModeler.webapp.livenessProbe.probePath": "/healthz",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0].LivenessProbe

				s.Require().Equal("/healthz", probe.HTTPGet.Path)
				s.Require().Equal("http-management", probe.HTTPGet.Port.StrVal)
			},
		}, {
			Name:                 "TestContainerProbesWithContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                           "true",
				"webModeler.enabled":                         "true",
				"webModeler.restapi.mail.fromAddress":        "example@example.com",
				"webModeler.contextPath":                     "/test",
				"webModeler.webapp.startupProbe.enabled":     "true",
				"webModeler.webapp.startupProbe.probePath":   "/start",
				"webModeler.webapp.readinessProbe.enabled":   "true",
				"webModeler.webapp.readinessProbe.probePath": "/ready",
				"webModeler.webapp.livenessProbe.enabled":    "true",
				"webModeler.webapp.livenessProbe.probePath":  "/live",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0]

				s.Require().Equal("/start", probe.StartupProbe.HTTPGet.Path)
				s.Require().Equal("/ready", probe.ReadinessProbe.HTTPGet.Path)
				s.Require().Equal("/live", probe.LivenessProbe.HTTPGet.Path)
			},
		}, {
			// Web-Modeler WebApp doesn't support contextPath for health endpoints.
			Name: "TestContainerSetSidecar",
			Values: map[string]string{
				"identity.enabled":                                     "true",
				"webModeler.enabled":                                   "true",
				"webModeler.restapi.mail.fromAddress":                  "example@example.com",
				"webModeler.webapp.sidecars[0].name":                   "nginx",
				"webModeler.webapp.sidecars[0].image":                  "nginx:latest",
				"webModeler.webapp.sidecars[0].ports[0].containerPort": "80",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				podContainers := deployment.Spec.Template.Spec.Containers
				expectedContainer := corev1.Container{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				}

				s.Require().Contains(podContainers, expectedContainer)
			},
		}, {
			Name: "TestContainerSetInitContainer",
			Values: map[string]string{
				"identity.enabled":                                           "true",
				"webModeler.enabled":                                         "true",
				"webModeler.restapi.mail.fromAddress":                        "example@example.com",
				"webModeler.webapp.initContainers[0].name":                   "nginx",
				"webModeler.webapp.initContainers[0].image":                  "nginx:latest",
				"webModeler.webapp.initContainers[0].ports[0].containerPort": "80",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				podContainers := deployment.Spec.Template.Spec.InitContainers
				expectedContainer := corev1.Container{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				}

				s.Require().Contains(podContainers, expectedContainer)
			},
		}, {
			Name: "TestSetDnsPolicyAndDnsConfig",
			Values: map[string]string{
				"identity.enabled":                           "true",
				"webModeler.enabled":                         "true",
				"webModeler.restapi.mail.fromAddress":        "example@example.com",
				"webModeler.webapp.dnsPolicy":                "ClusterFirst",
				"webModeler.webapp.dnsConfig.nameservers[0]": "8.8.8.8",
				"webModeler.webapp.dnsConfig.searches[0]":    "example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				// Check if dnsPolicy is set
				require.NotEmpty(s.T(), deployment.Spec.Template.Spec.DNSPolicy, "dnsPolicy should not be empty")

				// Check if dnsConfig is set
				require.NotNil(s.T(), deployment.Spec.Template.Spec.DNSConfig, "dnsConfig should not be nil")

				expectedDNSConfig := &corev1.PodDNSConfig{
					Nameservers: []string{"8.8.8.8"},
					Searches:    []string{"example.com"},
				}

				require.Equal(s.T(), expectedDNSConfig, deployment.Spec.Template.Spec.DNSConfig, "dnsConfig should match the expected configuration")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

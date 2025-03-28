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
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

type restapiDeploymentTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestRestapiDeploymentTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &restapiDeploymentTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/web-modeler/deployment-restapi.yaml"},
	})
}

func (s *restapiDeploymentTemplateTest) TestContainerExternalDatabasePasswordSecretRefForGivenPassword() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                           "true",
			"webModeler.restapi.mail.fromAddress":          "example@example.com",
			"postgresql.enabled":                           "false",
			"webModeler.restapi.externalDatabase.url":      "jdbc:postgresql://postgres.example.com:65432/modeler-database",
			"webModeler.restapi.externalDatabase.user":     "modeler-user",
			"webModeler.restapi.externalDatabase.password": "modeler-password",
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
			Name: "SPRING_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "camunda-platform-test-web-modeler-restapi"},
					Key:                  "database-password",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerExternalDatabasePasswordSecretRefForExistingSecretAndDefaultKey() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                      "true",
			"webModeler.restapi.mail.fromAddress":                     "example@example.com",
			"postgresql.enabled":                                      "false",
			"webModeler.restapi.externalDatabase.url":                 "jdbc:postgresql://postgres.example.com:65432/modeler-database",
			"webModeler.restapi.externalDatabase.user":                "modeler-user",
			"webModeler.restapi.externalDatabase.existingSecret.name": "my-secret",
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
			Name: "SPRING_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
					Key:                  "database-password",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerExternalDatabasePasswordSecretRefForExistingSecretAndCustomKey() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                            "true",
			"webModeler.restapi.mail.fromAddress":                           "example@example.com",
			"postgresql.enabled":                                            "false",
			"webModeler.restapi.externalDatabase.url":                       "jdbc:postgresql://postgres.example.com:65432/modeler-database",
			"webModeler.restapi.externalDatabase.user":                      "modeler-user",
			"webModeler.restapi.externalDatabase.existingSecret.name":       "my-secret",
			"webModeler.restapi.externalDatabase.existingSecretPasswordKey": "my-database-password-key",
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
			Name: "SPRING_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
					Key:                  "my-database-password-key",
				},
			},
		})
}
func (s *restapiDeploymentTemplateTest) TestContainerExternalDatabasePasswordExplicitlyDefinedStillReferencesInternalSecret() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                 "true",
			"webModeler.restapi.mail.fromAddress":                "example@example.com",
			"postgresql.enabled":                                 "false",
			"webModeler.restapi.externalDatabase.url":            "jdbc:postgresql://postgres.example.com:65432/modeler-database",
			"webModeler.restapi.externalDatabase.user":           "modeler-user",
			"webModeler.restapi.externalDatabase.existingSecret": "password1234",
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
			Name: "SPRING_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "camunda-platform-test-web-modeler-restapi"},
					Key:                  "database-password",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerInternalDatabasePasswordSecretRefForExistingSecretAndDefaultKey() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"postgresql.enabled":                  "true",
			"postgresql.auth.existingSecret":      "my-secret",
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
			Name: "SPRING_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
					Key:                  "password",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerInternalDatabasePasswordSecretRefForExistingSecretAndCustomKey() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                         "true",
			"webModeler.restapi.mail.fromAddress":        "example@example.com",
			"postgresql.enabled":                         "true",
			"postgresql.auth.existingSecret":             "my-secret",
			"postgresql.auth.secretKeys.userPasswordKey": "my-database-password-key",
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
			Name: "SPRING_DATASOURCE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
					Key:                  "my-database-password-key",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerSmtpPasswordSecretRefForGivenPassword() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                   "true",
			"webModeler.restapi.mail.fromAddress":  "example@example.com",
			"webModeler.restapi.mail.smtpUser":     "modeler-user",
			"webModeler.restapi.mail.smtpPassword": "modeler-password",
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
			Name: "RESTAPI_MAIL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "camunda-platform-test-web-modeler-restapi"},
					Key:                  "smtp-password",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerSmtpPasswordSecretRefForExistingSecretAndDefaultKey() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                          "true",
			"webModeler.restapi.mail.fromAddress":         "example@example.com",
			"webModeler.restapi.mail.smtpUser":            "modeler-user",
			"webModeler.restapi.mail.existingSecret.name": "my-secret",
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
			Name: "RESTAPI_MAIL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
					Key:                  "smtp-password",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerSmtpPasswordSecretRefForExistingSecretAndCustomKey() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                "true",
			"webModeler.restapi.mail.fromAddress":               "example@example.com",
			"webModeler.restapi.mail.smtpUser":                  "modeler-user",
			"webModeler.restapi.mail.existingSecret.name":       "my-secret",
			"webModeler.restapi.mail.existingSecretPasswordKey": "my-smtp-password-key",
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
			Name: "RESTAPI_MAIL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
					Key:                  "my-smtp-password-key",
				},
			},
		})
}

func (s *restapiDeploymentTemplateTest) TestContainerStartupProbe() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                        "true",
			"webModeler.restapi.mail.fromAddress":       "example@example.com",
			"webModeler.restapi.startupProbe.enabled":   "true",
			"webModeler.restapi.startupProbe.probePath": "/healthz",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	probe := deployment.Spec.Template.Spec.Containers[0].StartupProbe

	s.Require().Equal("/healthz", probe.HTTPGet.Path)
	s.Require().Equal("http-management", probe.HTTPGet.Port.StrVal)
}

func (s *restapiDeploymentTemplateTest) TestContainerReadinessProbe() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                          "true",
			"webModeler.restapi.mail.fromAddress":         "example@example.com",
			"webModeler.restapi.readinessProbe.enabled":   "true",
			"webModeler.restapi.readinessProbe.probePath": "/healthz",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	probe := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe

	s.Require().Equal("/healthz", probe.HTTPGet.Path)
	s.Require().Equal("http-management", probe.HTTPGet.Port.StrVal)
}

func (s *restapiDeploymentTemplateTest) TestContainerLivenessProbe() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                         "true",
			"webModeler.restapi.mail.fromAddress":        "example@example.com",
			"webModeler.restapi.livenessProbe.enabled":   "true",
			"webModeler.restapi.livenessProbe.probePath": "/healthz",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	probe := deployment.Spec.Template.Spec.Containers[0].LivenessProbe

	s.Require().Equal("/healthz", probe.HTTPGet.Path)
	s.Require().Equal("http-management", probe.HTTPGet.Port.StrVal)
}

// Web-Modeler REST API doesn't use contextPath for health endpoints.
func (s *restapiDeploymentTemplateTest) TestContainerProbesWithContextPath() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                          "true",
			"webModeler.restapi.mail.fromAddress":         "example@example.com",
			"webModeler.contextPath":                      "/test",
			"webModeler.restapi.startupProbe.enabled":     "true",
			"webModeler.restapi.startupProbe.probePath":   "/start",
			"webModeler.restapi.readinessProbe.enabled":   "true",
			"webModeler.restapi.readinessProbe.probePath": "/ready",
			"webModeler.restapi.livenessProbe.enabled":    "true",
			"webModeler.restapi.livenessProbe.probePath":  "/live",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	probe := deployment.Spec.Template.Spec.Containers[0]

	s.Require().Equal("/start", probe.StartupProbe.HTTPGet.Path)
	s.Require().Equal("/ready", probe.ReadinessProbe.HTTPGet.Path)
	s.Require().Equal("/live", probe.LivenessProbe.HTTPGet.Path)
}

func (s *restapiDeploymentTemplateTest) TestContainerSetSidecar() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                    "true",
			"webModeler.restapi.mail.fromAddress":                   "example@example.com",
			"webModeler.restapi.sidecars[0].name":                   "nginx",
			"webModeler.restapi.sidecars[0].image":                  "nginx:latest",
			"webModeler.restapi.sidecars[0].ports[0].containerPort": "80",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
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
}

func (s *restapiDeploymentTemplateTest) TestContainerSetInitContainer() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                          "true",
			"webModeler.restapi.mail.fromAddress":                         "example@example.com",
			"webModeler.restapi.initContainers[0].name":                   "nginx",
			"webModeler.restapi.initContainers[0].image":                  "nginx:latest",
			"webModeler.restapi.initContainers[0].ports[0].containerPort": "80",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
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
}
func (s *restapiDeploymentTemplateTest) TestSetDnsPolicyAndDnsConfig() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                          "true",
			"webModeler.restapi.mail.fromAddress":         "example@example.com",
			"webModeler.restapi.dnsPolicy":                "ClusterFirst",
			"webModeler.restapi.dnsConfig.nameservers[0]": "8.8.8.8",
			"webModeler.restapi.dnsConfig.searches[0]":    "example.com",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
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
}

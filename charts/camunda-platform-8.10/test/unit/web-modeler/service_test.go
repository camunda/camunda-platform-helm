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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
)

type ServiceTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	component string
	templates []string
}

func TestServiceTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	components := []string{"restapi", "websockets"}

	for _, component := range components {
		suite.Run(t, &ServiceTest{
			chartPath: chartPath,
			release:   "camunda-platform-test",
			namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
			component: component,
			templates: []string{"templates/web-modeler/service-" + component + ".yaml"},
		})
	}
}

func (s *ServiceTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerSetGlobalAnnotations",
			Values: map[string]string{
				"identity.enabled":   "true",
				"webModeler.enabled": "true",
				"camundaHub.webModeler.restapi.mail.fromAddress": "example@example.com",
				"global.annotations.foo":                         "bar",
				"global.elasticsearch.enabled":                   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(s.T(), output, &service)

				// then
				s.Require().Equal("bar", service.ObjectMeta.Annotations["foo"])
			},
		}, {
			Name: "TestContainerServiceAnnotations",
			Values: map[string]string{
				"identity.enabled":   "true",
				"webModeler.enabled": "true",
				"camundaHub.webModeler.restapi.mail.fromAddress":                    "example@example.com",
				"camundaHub.webModeler." + s.component + ".service.annotations.foo": "bar",
				"global.elasticsearch.enabled":                                      "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(s.T(), output, &service)

				// then
				s.Require().Equal("bar", service.ObjectMeta.Annotations["foo"])
			},
		}, {
			Name: "TestAppProtocolsDefaultEmpty",
			Values: map[string]string{
				"identity.enabled":   "true",
				"webModeler.enabled": "true",
				"camundaHub.webModeler.restapi.mail.fromAddress": "example@example.com",
				"global.elasticsearch.enabled":                   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(s.T(), output, &service)

				// then
				for _, port := range service.Spec.Ports {
					s.Require().Empty(port.AppProtocol, "port %q should have no appProtocol by default", port.Name)
				}
			},
		}, {
			Name: "TestAppProtocolsSetsOnlyTargetedPort",
			Values: map[string]string{
				"identity.enabled":   "true",
				"webModeler.enabled": "true",
				"camundaHub.webModeler.restapi.mail.fromAddress":                      "example@example.com",
				"camundaHub.webModeler." + s.component + ".service.appProtocols.http": "http",
				"global.elasticsearch.enabled":                                        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(s.T(), output, &service)

				// then
				for _, port := range service.Spec.Ports {
					if port.Name == "http" {
						s.Require().NotNil(port.AppProtocol)
						s.Require().Equal("http", *port.AppProtocol)
					} else {
						s.Require().Empty(port.AppProtocol, "port %q should have no appProtocol", port.Name)
					}
				}
			},
		}, {
			Name: "TestAppProtocolsUnknownPortNameFails",
			Values: map[string]string{
				"identity.enabled":   "true",
				"webModeler.enabled": "true",
				"camundaHub.webModeler.restapi.mail.fromAddress":                       "example@example.com",
				"camundaHub.webModeler." + s.component + ".service.appProtocols.htttp": "http",
				"global.elasticsearch.enabled":                                         "true",
			},
			Expected: map[string]string{
				"ERROR": "unknown port name",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ServiceTest) TestLegacyServiceAccountEnabledOverrideDoesNotBreakDeploymentReference() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"identity.enabled":                               "true",
			"webModeler.enabled":                             "true",
			"webModeler.serviceAccount.enabled":              "false",
			"camundaHub.webModeler.serviceAccount.enabled":   "true",
			"camundaHub.webModeler.restapi.mail.fromAddress": "example@example.com",
			"global.elasticsearch.enabled":                   "true",
		},
	}
	templates := []string{
		"templates/web-modeler/serviceaccount.yaml",
		"templates/web-modeler/deployment-" + s.component + ".yaml",
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, templates)

	var serviceAccount coreV1.ServiceAccount
	var deployment appsv1.Deployment
	for _, object := range strings.Split(output, "---") {
		if strings.Contains(object, "kind: ServiceAccount") {
			helm.UnmarshalK8SYaml(s.T(), object, &serviceAccount)
		}
		if strings.Contains(object, "kind: Deployment") {
			helm.UnmarshalK8SYaml(s.T(), object, &deployment)
		}
	}

	// then
	s.Require().NotEmpty(serviceAccount.Name)
	s.Require().Equal(serviceAccount.Name, deployment.Spec.Template.Spec.ServiceAccountName)
}

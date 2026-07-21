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

package identity

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ServiceTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestServiceTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ServiceTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/identity/service.yaml"},
	})
}

func (s *ServiceTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Skip: true,
			Name: "TestContainerSetGlobalAnnotations",
			Values: map[string]string{
				"identity.enabled":       "true",
				"global.annotations.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(t, output, &service)

				// then
				s.Require().Equal("bar", service.ObjectMeta.Annotations["foo"])
			},
		}, {
			Skip: true,
			Name: "TestContainerServiceAnnotations",
			Values: map[string]string{
				"identity.enabled":                 "true",
				"identity.service.annotations.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(t, output, &service)

				// then
				s.Require().Equal("bar", service.ObjectMeta.Annotations["foo"])
			},
		}, {
			Name: "TestAppProtocolsDefaultEmpty",
			Values: map[string]string{
				"identity.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(t, output, &service)

				// then
				for _, port := range service.Spec.Ports {
					s.Require().Empty(port.AppProtocol, "port %q should have no appProtocol by default", port.Name)
				}
			},
		}, {
			Name: "TestAppProtocolsSetsOnlyTargetedPort",
			Values: map[string]string{
				"identity.enabled":                   "true",
				"identity.service.appProtocols.http": "http",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(t, output, &service)

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
				"identity.enabled":                    "true",
				"identity.service.appProtocols.htttp": "http",
			},
			Expected: map[string]string{
				"ERROR": "is not a valid appProtocols key",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

type KeycloakServiceTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestKeycloakServiceTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &KeycloakServiceTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/identity/keycloak-service.yaml"},
	})
}

func (s *KeycloakServiceTest) TestKeycloakDifferentServiceValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestKeycloakExternalService",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.keycloak.internal":     "true",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.prod.svc.cluster.local",
				"global.identity.keycloak.url.port":     "8443",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var service coreV1.Service
				helm.UnmarshalK8SYaml(t, output, &service)

				// then
				s.Require().Equal(coreV1.ServiceType("ExternalName"), service.Spec.Type)
				s.Require().Equal("keycloak.prod.svc.cluster.local", service.Spec.ExternalName)
				s.Require().Len(service.Spec.Ports, 1)
				s.Require().Equal("http", service.Spec.Ports[0].Name)
				s.Require().Equal(int32(8443), service.Spec.Ports[0].Port)
				s.Require().Equal(intstr.FromInt32(8443), service.Spec.Ports[0].TargetPort)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

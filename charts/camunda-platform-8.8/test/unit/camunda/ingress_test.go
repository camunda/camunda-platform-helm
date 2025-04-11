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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

type IngressTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
	extraArgs []string
}

func TestIngressTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &IngressTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/camunda/ingress-http.yaml"},
	})
}

func (s *IngressTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestIngressEnabledAndKeycloakChartProxyForwardingEnabled",
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: []string{"--show-only", "charts/identityKeycloak/templates/statefulset.yaml"},
			},
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.identity.auth.connectors.existingSecret.name": "foo",
				"global.identity.auth.core.existingSecret.name":       "bar",
				"global.ingress.tls.enabled":                          "true",
				"identity.contextPath":                                "/identity",
				"identity.enabled":                                    "true",
				"identityKeycloak.enabled":                            "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env,
					corev1.EnvVar{
						Name:  "KEYCLOAK_PROXY_ADDRESS_FORWARDING",
						Value: "true",
					})
			},
		},
		{
			Name:                 "TestIngressEnabledAndKeycloakChartProxyForwardingEnabled",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: nil,
			},
			RenderTemplateExtraArgs: []string{"--show-only", "charts/identityKeycloak/templates/statefulset.yaml"},
			Values: map[string]string{
				"global.ingress.tls.enabled": "true",
				"identity.contextPath":       "/identity",
				"identity.enabled":           "/true",
				"identityKeycloak.enabled":   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env,
					corev1.EnvVar{
						Name:  "KEYCLOAK_PROXY_ADDRESS_FORWARDING",
						Value: "true",
					})
			},
		},
		{
			Name:                 "TestIngressEnabledWithKeycloakCustomContextPathIngress",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled":               "true",
				"global.identity.keycloak.contextPath": "/custom",
				"identityKeycloak.enabled":             "true",
				"identityKeycloak.httpRelativePath":    "/custom",
				"identity.contextPath":                 "/identity",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(s.T(), output, &ingress)

				// then
				path := ingress.Spec.Rules[0].HTTP.Paths[0]
				s.Require().Equal("/custom/", path.Path)
				s.Require().Equal("camunda-platform-test-keycloak", path.Backend.Service.Name)
			},
		},
		{
			Name:                 "TestIngressEnabledWithKeycloakCustomContextPathSts",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: nil,
			},
			RenderTemplateExtraArgs: []string{"--show-only", "charts/identityKeycloak/templates/statefulset.yaml"},
			Values: map[string]string{
				"global.ingress.enabled":               "true",
				"global.identity.keycloak.contextPath": "/custom",
				"identityKeycloak.enabled":             "true",
				"identityKeycloak.httpRelativePath":    "/custom",
				"identity.contextPath":                 "/identity",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env,
					corev1.EnvVar{
						Name:  "KEYCLOAK_HTTP_RELATIVE_PATH",
						Value: "/custom",
					})
			},
		},
		{
			Name:                 "TestIngressWithKeycloakChartIsDisabled",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled": "true",
				"identity.contextPath":   "/identity",
				// Disable Identity Keycloak chart.
				"identityKeycloak.enabled": "false",
				// Set vars to use existing Keycloak.
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.prod.svc.cluster.local",
				"global.identity.keycloak.url.port":     "8443",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				// TODO: Instead of using plain text search, unmarshal the output in an ingress struct and assert the values.
				s.Require().NotContains(output, "keycloak")
				s.Require().NotContains(output, "path: /auth")
				s.Require().NotContains(output, "number: 8443")
			},
		},
		{
			Name:                 "TestIngressWithContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled":              "true",
				"identity.contextPath":                "/identity",
				"optimize.contextPath":                "/optimize",
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "example@example.com",
				"webModeler.contextPath":              "/modeler",
				"core.contextPath":                    "/core",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().Contains(output, "kind: Ingress")
				s.Require().Contains(output, "path: /auth")
				s.Require().Contains(output, "path: /identity")
				s.Require().Contains(output, "path: /optimize")
				s.Require().Contains(output, "path: /modeler")
				s.Require().Contains(output, "path: /modeler-ws")
				s.Require().Contains(output, "path: /core")
			},
		},
		{
			Name:                 "TestIngressComponentWithNoContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled":              "true",
				"identity.contextPath":                "",
				"optimize.contextPath":                "",
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "example@example.com",
				"webModeler.contextPath":              "",
				"core.contextPath":                    "",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().NotContains(output, "name: camunda-platform-test-identity")
				s.Require().NotContains(output, "name: camunda-platform-test-optimize")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-webapp")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-websockets")
				s.Require().NotContains(output, "name: camunda-platform-test-core")
			},
		},
		{
			Name:                 "TestIngressComponentDisabled",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled": "true",
				"optimize.enabled":       "false",
				"webModeler.enabled":     "false",
				"core.enabled":           "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().NotContains(output, "name: camunda-platform-test-identity")
				s.Require().NotContains(output, "name: camunda-platform-test-optimize")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-webapp")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-websockets")
				s.Require().NotContains(output, "name: camunda-platform-test-core")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

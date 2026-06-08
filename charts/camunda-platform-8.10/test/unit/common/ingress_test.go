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
		templates: []string{"templates/common/ingress-http.yaml"},
	})
}

func (s *IngressTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:                 "TestIngressWithKeycloakChartIsDisabled",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled": "true",
				"identity.contextPath":   "/identity",
				// Disable Identity Keycloak chart.
				// Set vars to use existing Keycloak.
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.prod.svc.cluster.local",
				"global.identity.keycloak.url.port":     "8443",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				// TODO: Instead of using plain text search, unmarshal the output in an ingress struct and assert the values.
				require.NotContains(t, output, "keycloak")
				require.NotContains(t, output, "path: /auth")
				require.NotContains(t, output, "number: 8443")
			},
		},
		{
			Skip:                 true,
			Name:                 "TestIngressWithContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled":              "true",
				"identity.contextPath":                "/identity",
				"optimize.contextPath":                "/optimize",
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "example@example.com",
				"webModeler.contextPath":              "/modeler",
				"orchestration.contextPath":           "/orchestration",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().Contains(output, "kind: Ingress")
				s.Require().Contains(output, "path: /auth")
				s.Require().Contains(output, "path: /identity")
				s.Require().Contains(output, "path: /optimize")
				s.Require().Contains(output, "path: /modeler")
				s.Require().Contains(output, "path: /modeler-ws")
				s.Require().Contains(output, "path: /orchestration")
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
				"orchestration.contextPath":           "",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().NotContains(output, "name: camunda-platform-test-identity")
				s.Require().NotContains(output, "name: camunda-platform-test-optimize")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-restapi")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-websockets")
				s.Require().NotContains(output, "name: camunda-platform-test-zeebe")
			},
		},
		{
			Name:                 "TestIngressComponentDisabled",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled": "true",
				"optimize.enabled":       "false",
				"webModeler.enabled":     "false",
				"orchestration.enabled":  "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().NotContains(output, "name: camunda-platform-test-identity")
				s.Require().NotContains(output, "name: camunda-platform-test-optimize")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-restapi")
				s.Require().NotContains(output, "name: camunda-platform-test-web-modeler-websockets")
				s.Require().NotContains(output, "name: camunda-platform-test-zeebe")
			},
		},
		{
			Name:                 "TestIngressExternal",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled":  "true",
				"global.ingress.external": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				require.NotContains(t, output, "kind: Ingress")
			},
		},
		{
			Name:                 "TestIngressWithGlobalLabels",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled":                        "true",
				"global.ingress.labels.test-label":              "test-value",
				"global.ingress.labels.external-dns":            "enabled",
				"global.ingress.labels.nginx\\.ingress\\.class": "public",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// then
				s.Require().Equal("test-value", ingress.Labels["test-label"])
				s.Require().Equal("enabled", ingress.Labels["external-dns"])
				s.Require().Equal("public", ingress.Labels["nginx.ingress.class"])
			},
		},
		{
			Name:                 "TestIngressWithoutLabels",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// then - should only have default chart labels, not custom labels
				s.Require().NotContains(ingress.Labels, "test-label")
				s.Require().NotContains(ingress.Labels, "external-dns")
				// But should still have standard chart labels
				s.Require().Contains(ingress.Labels, "app")
				s.Require().Contains(ingress.Labels, "app.kubernetes.io/name")
			},
		},
		{
			Name:                 "TestHttpIngressLabelMergeOverwrite",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.ingress.enabled":          "true",
				"global.commonLabels.app":         "common-override",
				"global.commonLabels.environment": "common-env",
				"global.ingress.labels.app":       "ingress-override",
				"global.ingress.labels.team":      "ingress-team",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// then - ingress labels should override common labels for same keys
				s.Require().Equal("ingress-override", ingress.Labels["app"], "ingress labels should override common labels for same key")
				// and common labels should be present when no ingress label conflicts
				s.Require().Equal("common-env", ingress.Labels["environment"], "common labels should be present when not overridden")
				// and ingress-specific labels should be present
				s.Require().Equal("ingress-team", ingress.Labels["team"], "ingress labels should be present")
				// standard chart labels should still be there for non-conflicting keys
				s.Require().Contains(ingress.Labels, "app.kubernetes.io/name")
			},
		},
		{
			Name:                 "TestIngressHostWithTemplatingAndTLS",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			ValuesFiles:          []string{filepath.Join(s.chartPath, "test/unit/common/testdata/values-templated-ingress-host-tls.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// then - verify templating is evaluated in both rules and TLS hosts
				s.Require().Equal("camunda-platform-test.example.com", ingress.Spec.Rules[0].Host)
				s.Require().Equal("camunda-platform-test.example.com", ingress.Spec.TLS[0].Hosts[0])
				s.Require().Equal("tls-secret", ingress.Spec.TLS[0].SecretName)
			},
		},
		{
			Name: "TestHttpIngressOmitsOrchestrationPathWithServerTLS",
			Values: map[string]string{
				"global.ingress.enabled":    "true",
				"orchestration.enabled":     "true",
				"orchestration.contextPath": "/orchestration",
				"orchestration.env[0].name": "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "orchestration.env[0].value=true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				require.NotContains(t, output, "path: /orchestration")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

type OrchestrationHttpIngressTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
	extraArgs []string
}

func TestOrchestrationHttpIngressTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &OrchestrationHttpIngressTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/ingress-orchestration-http.yaml"},
	})
}

func (s *OrchestrationHttpIngressTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestOrchestrationHttpIngressWithServerTLS",
			Values: map[string]string{
				"global.ingress.enabled":                     "true",
				"global.host":                                "camunda.example.com",
				"global.ingress.tls.enabled":                 "true",
				"global.ingress.annotations.test-annotation": "test-value",
				"orchestration.enabled":                      "true",
				"orchestration.contextPath":                  "/orchestration",
				"orchestration.env[0].name":                  "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "orchestration.env[0].value=true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				require.Equal(t, "camunda-platform-test-orchestration-http", ingress.Name)
				require.Equal(t, "HTTPS", ingress.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
				require.Equal(t, "test-value", ingress.Annotations["test-annotation"])
				require.Equal(t, "camunda.example.com", ingress.Spec.Rules[0].Host)
				require.Equal(t, "/orchestration", ingress.Spec.Rules[0].HTTP.Paths[0].Path)
				require.Equal(t, "camunda-platform-test-zeebe-gateway", ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
				require.Equal(t, int32(8080), ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
				require.Equal(t, "camunda.example.com", ingress.Spec.TLS[0].Hosts[0])
			},
		},
		{
			Name: "TestOrchestrationHttpIngressDisabledWithoutServerTLS",
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: nil,
			},
			Values: map[string]string{
				"global.ingress.enabled":    "true",
				"orchestration.enabled":     "true",
				"orchestration.contextPath": "/orchestration",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				require.NotContains(t, output, "name: camunda-platform-test-orchestration-http")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

type GrpcIngressTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
	extraArgs []string
}

func TestGrpcIngressTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &GrpcIngressTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/ingress-grpc.yaml"},
	})
}

func (s *GrpcIngressTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:                 "TestGrpcIngressWithLabels",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.enabled":                          "true",
				"orchestration.ingress.grpc.enabled":             "true",
				"orchestration.ingress.grpc.labels.test-label":   "grpc-test-value",
				"orchestration.ingress.grpc.labels.external-dns": "grpc-enabled",
				"orchestration.ingress.grpc.labels.grpc-service": "zeebe-gateway",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// then
				s.Require().Equal("grpc-test-value", ingress.Labels["test-label"])
				s.Require().Equal("grpc-enabled", ingress.Labels["external-dns"])
				s.Require().Equal("zeebe-gateway", ingress.Labels["grpc-service"])
			},
		},
		{
			Name:                 "TestGrpcIngressWithoutLabels",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.enabled":              "true",
				"orchestration.ingress.grpc.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// then - should only have default chart labels, not custom labels
				s.Require().NotContains(ingress.Labels, "test-label")
				s.Require().NotContains(ingress.Labels, "external-dns")
				// But should still have standard chart labels
				s.Require().Contains(ingress.Labels, "app")
				s.Require().Contains(ingress.Labels, "app.kubernetes.io/name")
			},
		},
		{
			Name:                 "TestGrpcIngressDisabled",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.enabled":                        "true",
				"orchestration.ingress.grpc.enabled":           "false",
				"orchestration.ingress.grpc.labels.test-label": "should-not-appear",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then - no ingress should be rendered
				require.NotContains(t, output, "kind: Ingress")
			},
		},
		{
			Name:                 "TestGrpcIngressExternal",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.enabled":                        "true",
				"orchestration.ingress.grpc.enabled":           "true",
				"orchestration.ingress.grpc.external":          "true",
				"orchestration.ingress.grpc.labels.test-label": "should-not-appear",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then - no ingress should be rendered when external is true
				require.NotContains(t, output, "kind: Ingress")
			},
		},
		{
			Name:                 "TestGrpcIngressLabelMergeOverwrite",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.enabled":                      "true",
				"orchestration.ingress.grpc.enabled":         "true",
				"global.commonLabels.app":                    "common-grpc-override",
				"global.commonLabels.environment":            "common-grpc-env",
				"orchestration.ingress.grpc.labels.app":      "grpc-specific-override",
				"orchestration.ingress.grpc.labels.protocol": "grpc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// then - grpc-specific labels should override common labels for same keys
				s.Require().Equal("grpc-specific-override", ingress.Labels["app"], "grpc labels should override common labels for same key")
				// and common labels should be present when no grpc label conflicts
				s.Require().Equal("common-grpc-env", ingress.Labels["environment"], "common labels should be present when not overridden")
				// and grpc-specific labels should be present
				s.Require().Equal("grpc", ingress.Labels["protocol"], "grpc-specific labels should be present")
				// standard chart labels should still be there for non-conflicting keys
				s.Require().Contains(ingress.Labels, "app.kubernetes.io/name")
			},
		},
		{
			Name:                 "TestGrpcIngressUsesSecureBackendProtocolWhenOrchestrationGrpcTlsIsEnabled",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			ValuesFiles:          []string{filepath.Join(s.chartPath, "test/unit/common/testdata/values-orchestration-grpc-tls.yaml")},
			Values: map[string]string{
				"orchestration.enabled":                              "true",
				"orchestration.ingress.grpc.enabled":                 "true",
				"orchestration.ingress.grpc.annotations.custom\\.io": "kept",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				s.Require().Equal("GRPCS", ingress.Annotations["nginx.ingress.kubernetes.io/backend-protocol"])
				s.Require().Equal("kept", ingress.Annotations["custom.io"])
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

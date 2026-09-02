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

package camunda

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	netv1 "k8s.io/api/networking/v1"
)

const (
	backendProtocolAnnotation = "nginx.ingress.kubernetes.io/backend-protocol"

	orchestrationTLSFixtures = "testdata/values-orchestration-tls-"
)

func orchestrationTLSFixture(name string) []string {
	return []string{orchestrationTLSFixtures + name + ".yaml"}
}

// OrchestrationTLSDetectionTest covers the four values the chart derives from
// Orchestration TLS state, across every config source the derivation reads.
type OrchestrationTLSDetectionTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestOrchestrationTLSDetection(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &OrchestrationTLSDetectionTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

// TestConnectorsEndpointSchemes asserts the Connectors client endpoints, which
// are derived from the same helpers as the Ingress backend protocols.
func (s *OrchestrationTLSDetectionTest) TestConnectorsEndpointSchemes() {
	const (
		restTLS   = "rest-address: https://camunda-platform-test-zeebe-gateway:8080"
		restPlain = "rest-address: http://camunda-platform-test-zeebe-gateway:8080"
		grpcTLS   = "grpc-address: https://camunda-platform-test-zeebe-gateway:26500"
		grpcPlain = "grpc-address: http://camunda-platform-test-zeebe-gateway:26500"
	)

	secure := func(t *testing.T, output string, err error) {
		require.NoError(t, err)
		require.Contains(t, output, restTLS)
		require.Contains(t, output, grpcTLS)
	}
	plaintext := func(t *testing.T, output string, err error) {
		require.NoError(t, err)
		require.Contains(t, output, restPlain)
		require.Contains(t, output, grpcPlain)
	}

	testCases := []testhelpers.TestCase{
		{
			Name:        "TestTLSFromConfigurationYieldsSecureSchemes",
			ValuesFiles: orchestrationTLSFixture("configuration"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    secure,
		},
		{
			Name:        "TestTLSFromExtraConfigurationYieldsSecureSchemes",
			ValuesFiles: orchestrationTLSFixture("extra-configuration"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    secure,
		},
		{
			Name:        "TestEnvFalseOverridesConfigurationTrue",
			ValuesFiles: orchestrationTLSFixture("configuration-env-false"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    plaintext,
		},
		{
			Name:        "TestExtraConfigurationOverridesConfiguration",
			ValuesFiles: orchestrationTLSFixture("extra-configuration-overrides"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    plaintext,
		},
		{
			Name:        "TestDottedKeyFormIsNotDetected",
			ValuesFiles: orchestrationTLSFixture("dotted-key"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    plaintext,
		},
		{
			// Spring applies every document of a multi-document source, so TLS
			// declared outside the first one is still the effective state.
			Name:        "TestTLSInLaterDocumentYieldsSecureSchemes",
			ValuesFiles: orchestrationTLSFixture("later-document"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    secure,
		},
		{
			// Later documents override earlier ones for the keys they set.
			Name:        "TestLaterDocumentOverridesEarlierTLS",
			ValuesFiles: orchestrationTLSFixture("later-document-overrides"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    plaintext,
		},
		{
			// A spring.config.activate condition is a runtime decision, so the
			// chart must not derive a secure transport from it.
			Name:        "TestProfileActivatedTLSIsNotDetected",
			ValuesFiles: orchestrationTLSFixture("profile-activated"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    plaintext,
		},
		{
			Name:        "TestSpringImportFalseIsNotDetected",
			ValuesFiles: orchestrationTLSFixture("spring-import-false"),
			Values:      map[string]string{"connectors.enabled": "true"},
			Verifier:    plaintext,
		},
	}

	testhelpers.RunTestCasesE(
		s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/connectors/configmap.yaml"}, testCases,
	)
}

// TestWebModelerEndpointSchemes pins the Hub gRPC scheme, the fourth derived value.
func (s *OrchestrationTLSDetectionTest) TestWebModelerEndpointSchemes() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestTLSFromConfigurationYieldsGrpcsScheme",
			ValuesFiles: orchestrationTLSFixture("configuration"),
			Values: map[string]string{
				"camundaHub.enabled":                           "true",
				"webModeler.restapi.mail.fromAddress":          "noreply@example.com",
				"identity.enabled":                             "true",
				"orchestration.security.authentication.method": "oidc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "grpcs://camunda-platform-test-zeebe-gateway")
				require.NotContains(t, output, "grpc://camunda-platform-test-zeebe-gateway")
			},
		},
		{
			Name:        "TestPlaintextYieldsGrpcScheme",
			ValuesFiles: orchestrationTLSFixture("spring-import-false"),
			Values: map[string]string{
				"camundaHub.enabled":                           "true",
				"webModeler.restapi.mail.fromAddress":          "noreply@example.com",
				"identity.enabled":                             "true",
				"orchestration.security.authentication.method": "oidc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "grpcs://camunda-platform-test-zeebe-gateway")
				require.Contains(t, output, "grpc://camunda-platform-test-zeebe-gateway")
			},
		},
	}

	testhelpers.RunTestCasesE(
		s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/web-modeler/configmap-restapi.yaml"}, testCases,
	)
}

// TestGRPCIngressBackendProtocol pins GRPC vs GRPCS selection.
func (s *OrchestrationTLSDetectionTest) TestGRPCIngressBackendProtocol() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestConfigurationTLSYieldsGRPCS",
			ValuesFiles: orchestrationTLSFixture("configuration"),
			Values: map[string]string{
				"orchestration.ingress.grpc.enabled": "true",
				"orchestration.ingress.grpc.host":    "grpc.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				require.Equal(t, "GRPCS", ingress.Annotations[backendProtocolAnnotation])
			},
		},
		{
			Name:        "TestTLSInLaterDocumentYieldsGRPCS",
			ValuesFiles: orchestrationTLSFixture("later-document"),
			Values: map[string]string{
				"orchestration.ingress.grpc.enabled": "true",
				"orchestration.ingress.grpc.host":    "grpc.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				require.Equal(t, "GRPCS", ingress.Annotations[backendProtocolAnnotation])
			},
		},
		{
			Name:        "TestProfileActivatedTLSYieldsGRPC",
			ValuesFiles: orchestrationTLSFixture("profile-activated"),
			Values: map[string]string{
				"orchestration.ingress.grpc.enabled": "true",
				"orchestration.ingress.grpc.host":    "grpc.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				require.Equal(t, "GRPC", ingress.Annotations[backendProtocolAnnotation])
			},
		},
		{
			Name:        "TestDottedKeyFormYieldsGRPC",
			ValuesFiles: orchestrationTLSFixture("dotted-key"),
			Values: map[string]string{
				"orchestration.ingress.grpc.enabled": "true",
				"orchestration.ingress.grpc.host":    "grpc.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				require.Equal(t, "GRPC", ingress.Annotations[backendProtocolAnnotation])
			},
		},
	}

	testhelpers.RunTestCasesE(
		s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/common/ingress-grpc.yaml"}, testCases,
	)
}

// TestSplitIngressInheritance is the §6.1b regression guard. Phase 2 widened the
// population that renders the split /orchestration Ingress, so the annotation
// and label inheritance it relies on needs real coverage.
func (s *OrchestrationTLSDetectionTest) TestSplitIngressInheritance() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestConflictingBackendProtocolStillYieldsHTTPS",
			ValuesFiles: orchestrationTLSFixture("ingress-inheritance"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				// The chart's protocol wins over the operator's conflicting value.
				require.Equal(t, "HTTPS", ingress.Annotations[backendProtocolAnnotation])
				// Unrelated operator annotations survive alongside it.
				require.Equal(t, "keep-me", ingress.Annotations["customer-annotation"])
				// Templated annotation values render, matching the shared Ingress.
				require.Equal(t, "camunda-platform-test-suffix", ingress.Annotations["templated-annotation"])
				// Operator labels inherit alongside the chart's own.
				require.Equal(t, "keep-me", ingress.Labels["customer-label"])
				require.Equal(t, "camunda-platform", ingress.Labels["app.kubernetes.io/name"])
			},
		},
	}

	testhelpers.RunTestCasesE(
		s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/common/ingress-orchestration-http.yaml"}, testCases,
	)
}

// TestOrchestrationPathIsNeverDuplicated asserts the complementary conditions at
// ingress-orchestration-http.yaml and ingress-http.yaml stay complementary: the
// /orchestration route appears on exactly one Ingress in both TLS states.
func (s *OrchestrationTLSDetectionTest) TestOrchestrationPathIsNeverDuplicated() {
	// helm --show-only errors instead of emitting nothing when a template renders
	// empty, which is the expected shape for whichever Ingress is inactive.
	countPaths := func(t *testing.T, fixture, template string) int {
		options := &helm.Options{
			ValuesFiles:    orchestrationTLSFixture(fixture),
			KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		}

		output, err := helm.RenderTemplateE(t, options, s.chartPath, s.release, []string{template})
		if err != nil {
			require.Contains(t, err.Error(), "could not find template",
				"unexpected render failure for %s", template)
			return 0
		}
		return strings.Count(output, "path: /orchestration")
	}

	for _, tc := range []struct{ name, fixture string }{
		{"TLSEnabled", "ingress-inheritance"},
		{"TLSDisabled", "ingress-plaintext"},
	} {
		s.Run(tc.name, func() {
			shared := countPaths(s.T(), tc.fixture, "templates/common/ingress-http.yaml")
			split := countPaths(s.T(), tc.fixture, "templates/common/ingress-orchestration-http.yaml")
			require.Equal(s.T(), 1, shared+split,
				"the /orchestration route must appear on exactly one Ingress (shared=%d split=%d)", shared, split)
		})
	}
}

// TestDetectionWarnings covers the four detection diagnostics. They exist because
// the derivation cannot read every form Spring accepts, and a wrong derivation
// installs cleanly and then fails at connection time.
func (s *OrchestrationTLSDetectionTest) TestDetectionWarnings() {
	testCases := []testhelpers.TestCase{
		{
			// W1: a dotted key the nested-key walk cannot see.
			Name:        "TestDottedKeyFormWarns",
			ValuesFiles: orchestrationTLSFixture("dotted-key"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "through the dotted key 'server.ssl.enabled'")
				require.Contains(t, output, "global.tls.orchestration.rest.enabled: true")
			},
		},
		{
			// W1 must stay quiet once the key is in a form the chart does read.
			Name:        "TestNestedKeyFormDoesNotWarn",
			ValuesFiles: orchestrationTLSFixture("configuration"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "through the dotted key")
			},
		},
		{
			// W2: a valueFrom-sourced toggle is unresolvable at render time.
			Name: "TestValueFromToggleWarns",
			Values: map[string]string{
				"orchestration.env[0].name":                           "SERVER_SSL_ENABLED",
				"orchestration.env[0].valueFrom.configMapKeyRef.name": "tls-config",
				"orchestration.env[0].valueFrom.configMapKeyRef.key":  "ssl-enabled",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "orchestration.env sets SERVER_SSL_ENABLED from a valueFrom reference")
			},
		},
		{
			// W2 is precise: a literal toggle carries no render-time ambiguity.
			Name: "TestLiteralToggleDoesNotWarn",
			Values: map[string]string{
				"global.tls.orchestration.rest.enabled":                    "true",
				"global.tls.orchestration.rest.cert.secret.existingSecret": "rest-tls",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "from a valueFrom reference")
			},
		},
		{
			// W3: a spring.config.activate-conditioned document sets the toggle,
			// so whether Spring applies it is unknown while templating.
			Name:        "TestProfileActivatedTLSWarns",
			ValuesFiles: orchestrationTLSFixture("profile-activated"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "inside a spring.config.activate-conditioned YAML document")
				require.Contains(t, output, "derives plaintext for Orchestration REST")
				require.Contains(t, output, "derives plaintext for Orchestration gRPC")
			},
		},
		{
			// W3 must stay quiet for a multi-document source the chart now reads
			// in full: unconditional documents are resolved, not unresolved.
			Name:        "TestTLSInLaterDocumentDoesNotWarn",
			ValuesFiles: orchestrationTLSFixture("later-document"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "spring.config.activate-conditioned YAML document")
			},
		},
		{
			// W3 is precise: a multi-document source that never mentions TLS must
			// not draw a TLS warning at all.
			// helm --show-only errors instead of emitting nothing when the
			// warnings ConfigMap renders empty, which is the shape of "no
			// warning at all".
			Name:        "TestMultiDocumentWithoutTLSDoesNotWarn",
			ValuesFiles: orchestrationTLSFixture("multi-document-plain"),
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "could not find template")
			},
		},
		{
			// W4: the split Ingress overrides an inherited backend-protocol.
			Name:        "TestOverriddenBackendProtocolWarns",
			ValuesFiles: orchestrationTLSFixture("ingress-inheritance"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "nginx.ingress.kubernetes.io/backend-protocol: HTTP, but Orchestration REST TLS is enabled")
			},
		},
	}

	testhelpers.RunTestCasesE(
		s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/common/configmap-warnings.yaml"}, testCases,
	)
}

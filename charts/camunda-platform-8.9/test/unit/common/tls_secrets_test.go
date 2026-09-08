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
	"maps"
	"strconv"
	"testing"

	"camunda-platform/test/unit/testhelpers"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type tlsSecretsTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func (s *tlsSecretsTest) SetupTest() {
	s.chartPath = "../../../"
	s.release = "test-release"
	s.namespace = "test-namespace"
	s.templates = []string{"templates"}
}

// Elasticsearch TLS Tests
func (s *tlsSecretsTest) TestElasticsearchTLSLegacyPattern() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "elasticsearch legacy TLS secret pattern",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                   "true",
				"global.elasticsearch.enabled":            "true",
				"global.elasticsearch.external":           "true",
				"global.elasticsearch.tls.enabled":        "true",
				"global.elasticsearch.tls.existingSecret": "legacy-es-tls-secret",
			},
			Expected: map[string]string{
				// Volume should use legacy secret name
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "legacy-es-tls-secret",
				// SubPath should default to externaldb.jks for legacy pattern
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath": "externaldb.jks",
				// MountPath should use default key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/externaldb.jks",
				// JAVA_TOOL_OPTIONS should include trustStore with default key
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value": "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestElasticsearchTLSNewPatternCustomKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "elasticsearch new TLS secret pattern with custom key",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                             "true",
				"global.elasticsearch.enabled":                      "true",
				"global.elasticsearch.external":                     "true",
				"global.elasticsearch.tls.enabled":                  "true",
				"global.elasticsearch.tls.secret.existingSecret":    "new-es-tls-abc123",
				"global.elasticsearch.tls.secret.existingSecretKey": "custom-truststore.jks",
			},
			Expected: map[string]string{
				// Volume should use new secret name
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-es-tls-abc123",
				// SubPath should use custom key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath": "custom-truststore.jks",
				// MountPath should use custom key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/custom-truststore.jks",
				// JAVA_TOOL_OPTIONS should include trustStore with custom key
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value": "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/custom-truststore.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestElasticsearchTLSNewPatternDefaultKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "elasticsearch new TLS secret pattern with default key",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                          "true",
				"global.elasticsearch.enabled":                   "true",
				"global.elasticsearch.external":                  "true",
				"global.elasticsearch.tls.enabled":               "true",
				"global.elasticsearch.tls.secret.existingSecret": "es-secret-def456",
			},
			Expected: map[string]string{
				// Volume should use new secret name
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "es-secret-def456",
				// SubPath should use default from values.yaml (externaldb.jks)
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath": "externaldb.jks",
				// MountPath should use default key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/externaldb.jks",
				// JAVA_TOOL_OPTIONS should include trustStore with default key
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value": "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestElasticsearchTLSBothPatterns() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "elasticsearch both TLS patterns - new takes precedence",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                             "true",
				"global.elasticsearch.enabled":                      "true",
				"global.elasticsearch.external":                     "true",
				"global.elasticsearch.tls.enabled":                  "true",
				"global.elasticsearch.tls.existingSecret":           "legacy-es-secret",
				"global.elasticsearch.tls.secret.existingSecret":    "new-es-secret-xyz",
				"global.elasticsearch.tls.secret.existingSecretKey": "new-cert.jks",
			},
			Expected: map[string]string{
				// Volume should use new secret name (not legacy)
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-es-secret-xyz",
				// SubPath should use new key (not default)
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath": "new-cert.jks",
				// MountPath should use new key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/new-cert.jks",
				// JAVA_TOOL_OPTIONS should use new key
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value": "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/new-cert.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// OpenSearch TLS Tests
func (s *tlsSecretsTest) TestOpenSearchTLSLegacyPattern() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "opensearch legacy TLS secret pattern",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                "true",
				"global.opensearch.enabled":            "true",
				"global.opensearch.external":           "true",
				"global.opensearch.tls.enabled":        "true",
				"global.opensearch.tls.existingSecret": "legacy-os-tls-ghi789",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName":            "legacy-os-tls-ghi789",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/externaldb.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOpenSearchTLSNewPatternCustomKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "opensearch new TLS secret pattern with custom key",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                          "true",
				"global.opensearch.enabled":                      "true",
				"global.opensearch.external":                     "true",
				"global.opensearch.tls.enabled":                  "true",
				"global.opensearch.tls.secret.existingSecret":    "os-certificates-jkl012",
				"global.opensearch.tls.secret.existingSecretKey": "ca-bundle-prod.jks",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName":            "os-certificates-jkl012",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "ca-bundle-prod.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/ca-bundle-prod.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/ca-bundle-prod.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOpenSearchTLSNewPatternDefaultKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "opensearch new TLS secret pattern with default key",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                       "true",
				"global.opensearch.enabled":                   "true",
				"global.opensearch.external":                  "true",
				"global.opensearch.tls.enabled":               "true",
				"global.opensearch.tls.secret.existingSecret": "os-secret-mno345",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName":            "os-secret-mno345",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/externaldb.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOpenSearchTLSBothPatterns() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "opensearch both TLS patterns - new takes precedence",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                          "true",
				"global.opensearch.enabled":                      "true",
				"global.opensearch.external":                     "true",
				"global.opensearch.tls.enabled":                  "true",
				"global.opensearch.tls.existingSecret":           "legacy-os-old",
				"global.opensearch.tls.secret.existingSecret":    "new-os-pqr678",
				"global.opensearch.tls.secret.existingSecretKey": "production.jks",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName":            "new-os-pqr678",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "production.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/usr/local/camunda/certificates/production.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError -Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/production.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// Console TLS Tests
func (s *tlsSecretsTest) TestConsoleTLSLegacyPattern() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "console legacy TLS secret pattern",
			Template: "templates/console/deployment.yaml",
			Values: map[string]string{
				"console.enabled":              "true",
				"console.contextPath":          "/",
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
				"console.tls.enabled":          "true",
				"console.tls.existingSecret":   "legacy-console-tls-stu901",
				"console.tls.certKeyFilename":  "server-ca.crt",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='console-certificates')].secret.secretName":            "legacy-console-tls-stu901",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value":               "/usr/local/console/certificates/server-ca.crt",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='console-certificates')].mountPath": "/usr/local/console/certificates",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestConsoleTLSNewPattern() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "console new TLS secret pattern",
			Template: "templates/console/deployment.yaml",
			Values: map[string]string{
				"console.enabled":                   "true",
				"console.contextPath":               "/",
				"identity.enabled":                  "true",
				"global.identity.auth.enabled":      "true",
				"console.tls.enabled":               "true",
				"console.tls.secret.existingSecret": "new-console-certs-vwx234",
				"console.tls.certKeyFilename":       "custom-root-ca.pem",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='console-certificates')].secret.secretName":            "new-console-certs-vwx234",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value":               "/usr/local/console/certificates/custom-root-ca.pem",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='console-certificates')].mountPath": "/usr/local/console/certificates",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestConsoleTLSBothPatterns() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "console both TLS patterns - new takes precedence",
			Template: "templates/console/deployment.yaml",
			Values: map[string]string{
				"console.enabled":                   "true",
				"console.contextPath":               "/",
				"identity.enabled":                  "true",
				"global.identity.auth.enabled":      "true",
				"console.tls.enabled":               "true",
				"console.tls.existingSecret":        "legacy-console-old",
				"console.tls.secret.existingSecret": "new-console-bcd890",
				"console.tls.certKeyFilename":       "ca-cert.pem",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='console-certificates')].secret.secretName":            "new-console-bcd890",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value":               "/usr/local/console/certificates/ca-cert.pem",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='console-certificates')].mountPath": "/usr/local/console/certificates",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// Disabled State Tests
func (s *tlsSecretsTest) TestElasticsearchTLSEnabledNoSecret() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "elasticsearch TLS enabled but no secret provided",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":            "true",
				"global.elasticsearch.enabled":     "true",
				"global.elasticsearch.external":    "true",
				"global.elasticsearch.tls.enabled": "true",
			},
			Expected: map[string]string{
				"kind": "StatefulSet",
			},
			Unexpected: []string{"spec.template.spec.volumes[?(@.name=='keystore')]"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestConsoleTLSEnabledNoSecret() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "console TLS enabled but no secret provided",
			Template: "templates/console/deployment.yaml",
			Values: map[string]string{
				"console.enabled":              "true",
				"console.contextPath":          "/",
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
				"console.tls.enabled":          "true",
				"console.tls.certKeyFilename":  "ca.crt",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value": "/usr/local/console/certificates/ca.crt",
			},
			Unexpected: []string{"spec.template.spec.volumes[?(@.name=='console-certificates')]"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestConsoleTLSDisabled() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "console TLS disabled - env var still rendered",
			Template: "templates/console/deployment.yaml",
			Values: map[string]string{
				"console.enabled":              "true",
				"console.contextPath":          "/",
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
				"console.tls.enabled":          "false",
				"console.tls.certKeyFilename":  "ca.crt",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value": "/usr/local/console/certificates/ca.crt",
			},
			Unexpected: []string{"spec.template.spec.volumes[?(@.name=='console-certificates')]"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// Optimize TLS Tests (uses same global ES/OS config)
func (s *tlsSecretsTest) TestOptimizeElasticsearchTLSLegacyPattern() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize elasticsearch legacy TLS secret pattern",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                        "true",
				"identity.enabled":                        "true",
				"global.identity.auth.enabled":            "true",
				"global.elasticsearch.enabled":            "true",
				"global.elasticsearch.external":           "true",
				"global.elasticsearch.tls.enabled":        "true",
				"global.elasticsearch.tls.existingSecret": "legacy-optimize-es-tls",
			},
			Expected: map[string]string{
				// Volume should use legacy secret name
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "legacy-optimize-es-tls",
				// Init container: SubPath should default to externaldb.jks
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath": "externaldb.jks",
				// Init container: MountPath should use default key
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				// Init container: JAVA_TOOL_OPTIONS should include trustStore with default key
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value": "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
				// Main container: SubPath should default to externaldb.jks
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath": "externaldb.jks",
				// Main container: MountPath should use default key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				// Main container: JAVA_TOOL_OPTIONS should include trustStore with default key
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value": "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOptimizeElasticsearchTLSNewPatternCustomKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize elasticsearch new TLS secret pattern with custom key",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                                  "true",
				"identity.enabled":                                  "true",
				"global.identity.auth.enabled":                      "true",
				"global.elasticsearch.enabled":                      "true",
				"global.elasticsearch.external":                     "true",
				"global.elasticsearch.tls.enabled":                  "true",
				"global.elasticsearch.tls.secret.existingSecret":    "new-optimize-es-uvw345",
				"global.elasticsearch.tls.secret.existingSecretKey": "optimize-truststore.jks",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-optimize-es-uvw345",
				// Init container with custom key
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath":   "optimize-truststore.jks",
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/optimize-truststore.jks",
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/optimize-truststore.jks",
				// Main container with custom key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "optimize-truststore.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/optimize-truststore.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/optimize-truststore.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOptimizeOpenSearchTLSNewPatternCustomKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize opensearch new TLS secret pattern with custom key",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"identity.enabled":                               "true",
				"global.identity.auth.enabled":                   "true",
				"global.opensearch.enabled":                      "true",
				"global.opensearch.external":                     "true",
				"global.opensearch.tls.enabled":                  "true",
				"global.opensearch.tls.secret.existingSecret":    "new-optimize-os-xyz678",
				"global.opensearch.tls.secret.existingSecretKey": "opensearch-certs.jks",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-optimize-os-xyz678",
				// Init container with custom key
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath":   "opensearch-certs.jks",
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/opensearch-certs.jks",
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/opensearch-certs.jks",
				// Main container with custom key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "opensearch-certs.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/opensearch-certs.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/opensearch-certs.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOptimizeElasticsearchTLSNewPatternDefaultKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize elasticsearch new TLS secret pattern with default key",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"identity.enabled":                               "true",
				"global.identity.auth.enabled":                   "true",
				"global.elasticsearch.enabled":                   "true",
				"global.elasticsearch.external":                  "true",
				"global.elasticsearch.tls.enabled":               "true",
				"global.elasticsearch.tls.secret.existingSecret": "new-optimize-es-default-key",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-optimize-es-default-key",
				// Init container with default key (externaldb.jks)
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
				// Main container with default key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOptimizeElasticsearchTLSBothPatterns() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize elasticsearch both TLS patterns - new takes precedence",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                                  "true",
				"identity.enabled":                                  "true",
				"global.identity.auth.enabled":                      "true",
				"global.elasticsearch.enabled":                      "true",
				"global.elasticsearch.external":                     "true",
				"global.elasticsearch.tls.enabled":                  "true",
				"global.elasticsearch.tls.existingSecret":           "legacy-optimize-es-both",
				"global.elasticsearch.tls.secret.existingSecret":    "new-optimize-es-both",
				"global.elasticsearch.tls.secret.existingSecretKey": "new-pattern.jks",
			},
			Expected: map[string]string{
				// Should use NEW pattern secret name
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-optimize-es-both",
				// Init container with NEW pattern custom key
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath":   "new-pattern.jks",
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/new-pattern.jks",
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/new-pattern.jks",
				// Main container with NEW pattern custom key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "new-pattern.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/new-pattern.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/new-pattern.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOptimizeOpenSearchTLSLegacyPattern() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize opensearch legacy TLS secret pattern",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                     "true",
				"identity.enabled":                     "true",
				"global.identity.auth.enabled":         "true",
				"global.opensearch.enabled":            "true",
				"global.opensearch.external":           "true",
				"global.opensearch.tls.enabled":        "true",
				"global.opensearch.tls.existingSecret": "legacy-optimize-os-tls",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "legacy-optimize-os-tls",
				// Init container with default key
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
				// Main container with default key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOptimizeOpenSearchTLSNewPatternDefaultKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize opensearch new TLS secret pattern with default key",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                            "true",
				"identity.enabled":                            "true",
				"global.identity.auth.enabled":                "true",
				"global.opensearch.enabled":                   "true",
				"global.opensearch.external":                  "true",
				"global.opensearch.tls.enabled":               "true",
				"global.opensearch.tls.secret.existingSecret": "new-optimize-os-default-key",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-optimize-os-default-key",
				// Init container with default key
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
				// Main container with default key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "externaldb.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/externaldb.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/externaldb.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestOptimizeOpenSearchTLSBothPatterns() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "optimize opensearch both TLS patterns - new takes precedence",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"identity.enabled":                               "true",
				"global.identity.auth.enabled":                   "true",
				"global.opensearch.enabled":                      "true",
				"global.opensearch.external":                     "true",
				"global.opensearch.tls.enabled":                  "true",
				"global.opensearch.tls.existingSecret":           "legacy-optimize-os-both",
				"global.opensearch.tls.secret.existingSecret":    "new-optimize-os-both",
				"global.opensearch.tls.secret.existingSecretKey": "new-os-pattern.jks",
			},
			Expected: map[string]string{
				// Should use NEW pattern secret name
				"spec.template.spec.volumes[?(@.name=='keystore')].secret.secretName": "new-optimize-os-both",
				// Init container with NEW pattern custom key
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].subPath":   "new-os-pattern.jks",
				"spec.template.spec.initContainers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/new-os-pattern.jks",
				"spec.template.spec.initContainers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/new-os-pattern.jks",
				// Main container with NEW pattern custom key
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].subPath":   "new-os-pattern.jks",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='keystore')].mountPath": "/optimize/certificates/new-os-pattern.jks",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":     "-Djavax.net.ssl.trustStore=/optimize/certificates/new-os-pattern.jks",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// global.tls.caBundle tests

func (s *tlsSecretsTest) TestCaBundleOrchestration() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle injects SSL_CERT_FILE + NODE_EXTRA_CA_CERTS env, volume, and mount into orchestration",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                        "true",
				"global.tls.caBundle.secret.existingSecret":    "my-ca-bundle",
				"global.tls.caBundle.secret.existingSecretKey": "ca.crt",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":            "my-ca-bundle",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='ca-bundle')].mountPath": "/etc/camunda/tls",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":          "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value":    "/etc/camunda/tls/ca.crt",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleWebModelerWebsockets() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle injects NODE_EXTRA_CA_CERTS into web-modeler websockets",
			Template: "templates/web-modeler/deployment-websockets.yaml",
			Values: map[string]string{
				"webModeler.enabled":                           "true",
				"webModeler.restapi.mail.fromAddress":          "test@example.com",
				"identity.enabled":                             "true",
				"global.tls.caBundle.secret.existingSecret":    "my-ca-bundle",
				"global.tls.caBundle.secret.existingSecretKey": "ca.crt",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":         "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value": "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":       "/etc/camunda/tls/ca.crt",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleInitContainerUsesComponentImage() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "init container reuses the component's own image (registry + tag), not a pinned JRE image",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"orchestration.image.tag":                   "t1",
				"global.image.registry":                     "reg.test",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				// init container image must equal the main container image
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].image": "reg.test/camunda/camunda:t1",
				"spec.template.spec.containers[0].image":                                          "reg.test/camunda/camunda:t1",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleInitContainerImageOverrideVerbatim() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "explicit caBundle.image override is used verbatim and NOT prefixed with global.image.registry",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.image.registry":                     "reg.test",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.image":                 "custom.io/myjre:1",
			},
			Expected: map[string]string{
				// verbatim — must NOT become reg.test/custom.io/myjre:1
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].image": "custom.io/myjre:1",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleInitContainerSecurityContext() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "init container pins runAsUser by default",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsUser":    "1000",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsNonRoot": "true",
			},
		},
		{
			Name:     "OpenShift adaptSecurityContext=force drops runAsUser from the init container",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"global.tls.caBundle.secret.existingSecret":           "my-ca-bundle",
				"global.compatibility.openshift.adaptSecurityContext": "force",
			},
			Expected: map[string]string{
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].name": "ca-bundle-truststore-init",
			},
			Unexpected: []string{
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsUser",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsGroup",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleChecksumAnnotation() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle + autoRollout stamps a checksum/ca-bundle pod annotation",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.autoRollout":           "true",
			},
			Expected: map[string]string{
				// lookup is empty under `helm template`, so the value is the stable
				// sha256 of an empty object — presence is what we assert here.
				"spec.template.metadata.annotations.checksum/ca-bundle": "12ae32cb1ec02d01eda3581b127c1fee3b0dc53572ed6baf239721a03d82e126",
			},
		},
		{
			Name:     "no checksum/ca-bundle annotation when caBundle is set but autoRollout is off (default)",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"kind": "StatefulSet",
			},
			Unexpected: []string{"spec.template.metadata.annotations.checksum/ca-bundle"},
		},
		{
			Name:     "no checksum/ca-bundle annotation when caBundle is unset",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled": "true",
			},
			Expected: map[string]string{
				"kind": "StatefulSet",
			},
			Unexpected: []string{"spec.template.metadata.annotations.checksum/ca-bundle"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleChecksumAnnotationWebModeler() {
	const sentinel = "12ae32cb1ec02d01eda3581b127c1fee3b0dc53572ed6baf239721a03d82e126"
	testCases := []testhelpers.TestCase{
		{
			Name:     "web-modeler restapi gets checksum/ca-bundle even with no user podAnnotations (restructured block)",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values: map[string]string{
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "test@example.com",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.autoRollout":           "true",
			},
			Expected: map[string]string{
				"spec.template.metadata.annotations.checksum/ca-bundle": sentinel,
			},
		},
		{
			Name:     "web-modeler restapi keeps caBundle checksum alongside user podAnnotations",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values: map[string]string{
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "test@example.com",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.autoRollout":           "true",
				"webModeler.restapi.podAnnotations.my-anno": "v1",
			},
			Expected: map[string]string{
				"spec.template.metadata.annotations.checksum/ca-bundle": sentinel,
				"spec.template.metadata.annotations.my-anno":            "v1",
			},
		},
		{
			Name:     "web-modeler restapi has no checksum annotation when caBundle is set but autoRollout is off (no empty annotations block)",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values: map[string]string{
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "test@example.com",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"kind": "Deployment",
			},
			Unexpected: []string{"spec.template.metadata.annotations.checksum/ca-bundle"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleConsole() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle injects SSL_CERT_FILE + NODE_EXTRA_CA_CERTS into Console (Node.js trusts custom CA via NODE_EXTRA_CA_CERTS)",
			Template: "templates/console/deployment.yaml",
			Values: map[string]string{
				"console.enabled":                           "true",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":         "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":       "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value": "/etc/camunda/tls/ca.crt",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleConnectors() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires connectors: volume, SSL_CERT_FILE, NODE_EXTRA_CA_CERTS, truststore JAVA_TOOL_OPTIONS, and init container",
			Template: "templates/connectors/deployment.yaml",
			Values: map[string]string{
				"connectors.enabled":                        "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                                                                "my-ca-bundle",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='ca-bundle')].mountPath":                                                     "/etc/camunda/tls",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                                                              "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value":                                                        "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":                                                          "-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleIdentity() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires identity: volume, SSL_CERT_FILE, truststore JAVA_TOOL_OPTIONS, and init container",
			Template: "templates/identity/deployment.yaml",
			Values: map[string]string{
				"identity.enabled":                          "true",
				"global.security.authentication.method":     "oidc",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                                                                "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                                                              "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":                                                          "-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleOptimize() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires optimize: volume, SSL_CERT_FILE, truststore mount, and init container",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                       "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                     "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestInitContainersRendering() {
	workloads := []struct {
		name          string
		template      string
		initKey       string
		containerName string
		values        map[string]string
	}{
		{
			name: "connectors", template: "templates/connectors/deployment.yaml",
			initKey: "connectors.initContainers", containerName: "connectors",
			values: map[string]string{"connectors.enabled": "true"},
		},
		{
			name: "identity", template: "templates/identity/deployment.yaml",
			initKey: "identity.initContainers", containerName: "camunda-platform",
		},
		{
			name: "orchestration", template: "templates/orchestration/statefulset.yaml",
			initKey: "orchestration.initContainers", containerName: "orchestration",
			values: map[string]string{"orchestration.enabled": "true"},
		},
		{
			name: "orchestration-legacy", template: "templates/orchestration/statefulset.yaml",
			initKey: "orchestration.extraInitContainers", containerName: "orchestration",
			values: map[string]string{"orchestration.enabled": "true"},
		},
		{
			name: "optimize", template: "templates/optimize/deployment.yaml",
			initKey: "optimize.initContainers", containerName: "optimize",
			values: map[string]string{"optimize.enabled": "true"},
		},
		{
			name: "optimize-migration", template: "templates/optimize/deployment.yaml",
			initKey: "optimize.initContainers", containerName: "optimize",
			values: map[string]string{"optimize.enabled": "true", "optimize.migration.enabled": "true"},
		},
		{
			name: "restapi", template: "templates/web-modeler/deployment-restapi.yaml",
			initKey: "webModeler.restapi.initContainers", containerName: "web-modeler-restapi",
			values: map[string]string{
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "test@example.com",
			},
		},
	}

	var testCases []testhelpers.TestCase
	for _, workload := range workloads {
		for _, scenario := range []struct {
			name     string
			caBundle bool
			custom   bool
		}{
			{name: "empty"},
			{name: "ca-only", caBundle: true},
			{name: "custom-only", custom: true},
			{name: "ca-and-custom", caBundle: true, custom: true},
		} {
			values := map[string]string{
				"identity.enabled":                          "true",
				"global.noSecondaryStorage":                 "false",
				"global.tls.caBundle.secret.existingSecret": "",
				"optimize.migration.enabled":                "false",
			}
			maps.Copy(values, workload.values)
			var expectedNames []string
			if scenario.caBundle {
				values["global.tls.caBundle.secret.existingSecret"] = "test-ca-bundle"
				expectedNames = append(expectedNames, "ca-bundle-truststore-init")
			}
			customContainer := corev1.Container{
				Name: "custom-" + s.release, Image: "busybox:1.36",
				Command: []string{"sh", "-c", "echo ready"},
			}
			if scenario.custom {
				values[workload.initKey+"[0].name"] = "custom-{{ .Release.Name }}"
				values[workload.initKey+"[0].image"] = customContainer.Image
				for index, command := range customContainer.Command {
					values[workload.initKey+"[0].command["+strconv.Itoa(index)+"]"] = command
				}
				if workload.initKey == "orchestration.initContainers" {
					values["orchestration.extraInitContainers[0].name"] = "legacy-must-not-render"
					values["orchestration.extraInitContainers[0].image"] = "busybox:1.36"
				}
				expectedNames = append(expectedNames, customContainer.Name)
			}
			if values["optimize.migration.enabled"] == "true" {
				expectedNames = append(expectedNames, "migration")
			}
			testCase := testhelpers.TestCase{
				Name: workload.name + "/" + scenario.name, Template: workload.template, Values: values,
			}
			if len(expectedNames) == 0 {
				testCase.Expected = map[string]string{
					"spec.template.spec.containers[0].name": workload.containerName,
				}
				testCase.Unexpected = []string{"spec.template.spec.initContainers"}
			} else {
				testCase.Verifier = func(t *testing.T, output string, err error) {
					require.NoError(t, err)
					var resource struct {
						Spec struct {
							Template corev1.PodTemplateSpec
						}
					}
					helm.UnmarshalK8SYaml(t, output, &resource)
					podSpec := resource.Spec.Template.Spec
					require.NotEmpty(t, podSpec.Containers)
					require.Equal(t, workload.containerName, podSpec.Containers[0].Name)
					var actualNames []string
					for _, container := range podSpec.InitContainers {
						actualNames = append(actualNames, container.Name)
					}
					require.Equal(t, expectedNames, actualNames)
					if scenario.custom {
						require.Contains(t, podSpec.InitContainers, customContainer)
					}
				}
			}
			testCases = append(testCases, testCase)
		}
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func TestTLSSecretsTestSuite(t *testing.T) {
	suite.Run(t, new(tlsSecretsTest))
}

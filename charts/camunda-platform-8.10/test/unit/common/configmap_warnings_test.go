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
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type ConfigMapWarningsTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigMapWarningsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigMapWarningsTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/configmap-warnings.yaml"},
	})
}

func (s *ConfigMapWarningsTemplateTest) TestUnresolvedAuthIssuer() {
	const issuerWarning = "no shared authentication issuer or issuer backend URL resolves"
	testCases := []testhelpers.TestCase{}
	for _, scenario := range []struct {
		name    string
		values  map[string]string
		args    []string
		warn    bool
		issuer  string
		backend string
	}{
		{name: "MissingGenericProvider", warn: true},
		{name: "MissingKeycloakProvider", values: map[string]string{"global.identity.auth.type": "KEYCLOAK"}, warn: true},
		{name: "AuthDisabled", values: map[string]string{"global.identity.auth.enabled": "false"}},
		{name: "ExplicitIssuer", values: map[string]string{"global.identity.auth.issuer": "https://issuer.example.com"}, issuer: "https://issuer.example.com"},
		{name: "PublicIssuerFallback", values: map[string]string{"global.identity.auth.publicIssuerUrl": "https://issuer.example.com"}, issuer: "https://issuer.example.com"},
		{name: "ExplicitBackend", values: map[string]string{"global.identity.auth.issuerBackendUrl": "https://issuer.example.com"}, backend: "https://issuer.example.com"},
		{name: "ExplicitKeycloakBackendWithoutHost", values: map[string]string{"global.identity.auth.type": "KEYCLOAK", "global.identity.auth.issuerBackendUrl": "https://issuer.example.com"}, backend: "https://issuer.example.com"},
		{
			name: "KeycloakHostTemplateResolvesEmpty",
			values: map[string]string{
				"identity.enabled":                      "false",
				"global.identity.auth.type":             "KEYCLOAK",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.port":     "443",
			},
			args:    []string{"--set-json", `global.identity.keycloak.url.host="{{ print \"\" }}"`},
			warn:    true,
			backend: "https://:443/auth/realms/camunda-platform",
		},
		{
			name: "TemplatedKeycloakHost",
			values: map[string]string{
				"identity.enabled":                      "false",
				"global.identity.auth.type":             "KEYCLOAK",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.port":     "443",
			},
			args:    []string{"--set-json", `global.identity.keycloak.url.host="{{ .Release.Name }}.example.com"`},
			backend: "https://" + s.release + ".example.com:443/auth/realms/camunda-platform",
		},
		{
			name: "ExternalKeycloak",
			values: map[string]string{
				"global.identity.auth.type":             "KEYCLOAK",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.example.com",
				"global.identity.keycloak.url.port":     "443",
			},
			backend: "https://keycloak.example.com:443/auth/realms/camunda-platform",
		},
		{
			name: "KeycloakURLWithoutProtocol",
			values: map[string]string{
				"identity.enabled":                  "false",
				"global.identity.auth.type":         "KEYCLOAK",
				"global.identity.keycloak.url.host": "keycloak.example.com",
				"global.identity.keycloak.url.port": "443",
			},
			warn:    true,
			backend: "://keycloak.example.com:443/auth/realms/camunda-platform",
		},
		{
			name: "KeycloakURLWithoutPort",
			values: map[string]string{
				"identity.enabled":                      "false",
				"global.identity.auth.type":             "KEYCLOAK",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.example.com",
			},
			warn:    true,
			backend: "https://keycloak.example.com:/auth/realms/camunda-platform",
		},
		{
			name: "GenericCannotUseKeycloakHost",
			values: map[string]string{
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.example.com",
				"global.identity.keycloak.url.port":     "443",
			},
			warn: true,
		},
		{
			name: "TemplatesResolveEmpty",
			args: []string{"--set-json", `global.identity.auth.issuer="{{ print \"\" }}"`, "--set-json", `global.identity.auth.issuerBackendUrl="{{ print \"\" }}"`},
			warn: true,
		},
		{
			name:   "TemplatedIssuer",
			args:   []string{"--set-json", `global.identity.auth.issuer="https://{{ .Release.Name }}.example.com"`},
			issuer: "https://" + s.release + ".example.com",
		},
		{
			name:    "TemplatedBackend",
			args:    []string{"--set-json", `global.identity.auth.issuerBackendUrl="https://{{ .Release.Name }}.example.com"`},
			backend: "https://" + s.release + ".example.com",
		},
	} {
		values := map[string]string{
			"identity.enabled":                         "true",
			"optimize.enabled":                         "false",
			"global.identity.auth.enabled":             "true",
			"global.identity.auth.type":                "GENERIC",
			"global.identity.auth.publicIssuerUrl":     "",
			"orchestration.data.secondaryStorage.type": "elasticsearch",
		}
		for key, value := range scenario.values {
			values[key] = value
		}
		testCases = append(testCases, testhelpers.TestCase{
			Name:                    scenario.name,
			Values:                  values,
			RenderTemplateExtraArgs: scenario.args,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				configMaps := map[string]corev1.ConfigMap{}
				decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)
				for {
					var resource corev1.ConfigMap
					err := decoder.Decode(&resource)
					if err == io.EOF {
						break
					}
					s.Require().NoError(err)
					if resource.Kind == "ConfigMap" {
						configMaps[resource.Name] = resource
					}
				}
				s.Require().Contains(configMaps, s.release+"-zeebe-configuration")
				warnings := configMaps[s.release+"-warnings"].Data["warnings"]
				s.Require().Equal(scenario.warn, strings.Contains(warnings, issuerWarning))
				if scenario.warn {
					s.Require().Contains(warnings, "global.identity.auth.issuer")
					s.Require().Contains(warnings, "global.identity.auth.issuerBackendUrl")
					s.Require().Contains(warnings, "global.identity.keycloak.url")
				}
				if values["global.identity.auth.enabled"] == "true" {
					s.Require().Contains(configMaps, s.release+"-identity-env-vars")
					identity := configMaps[s.release+"-identity-env-vars"]
					s.Require().Equal(scenario.issuer, identity.Data["CAMUNDA_IDENTITY_ISSUER"])
					s.Require().Equal(scenario.backend, identity.Data["CAMUNDA_IDENTITY_ISSUER_BACKEND_URL"])
				}
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestUnresolvedOrchestrationOIDCEndpoints() {
	const endpointWarning = "The Orchestration Cluster uses OIDC without an issuer URI, and these endpoints do not render as absolute URLs: "
	testCases := []testhelpers.TestCase{}
	for _, scenario := range []struct {
		name       string
		values     map[string]string
		unresolved string
	}{
		{name: "GenericBackendOnly", values: map[string]string{"global.identity.auth.issuerBackendUrl": "https://idp.example.com"}, unresolved: "authorization-uri, jwk-set-uri, token-uri"},
		{name: "MicrosoftBackendOnly", values: map[string]string{"global.identity.auth.type": "MICROSOFT", "global.identity.auth.issuerBackendUrl": "https://idp.example.com"}, unresolved: "authorization-uri, jwk-set-uri, token-uri"},
		{name: "KeycloakPublicIssuerOnly", values: map[string]string{"global.identity.auth.type": "KEYCLOAK", "global.identity.auth.publicIssuerUrl": "https://kc.example.com/auth/realms/camunda-platform"}, unresolved: "jwk-set-uri, token-uri"},
		{
			name: "KeycloakURLOnly",
			values: map[string]string{
				"global.identity.auth.type":             "KEYCLOAK",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "kc.example.com",
				"global.identity.keycloak.url.port":     "443",
			},
			unresolved: "authorization-uri",
		},
		{
			name: "ExternalKeycloak",
			values: map[string]string{
				"global.identity.auth.type":             "KEYCLOAK",
				"global.identity.auth.publicIssuerUrl":  "https://kc.example.com/auth/realms/camunda-platform",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "kc.example.com",
				"global.identity.keycloak.url.port":     "443",
			},
		},
		{name: "GlobalIssuer", values: map[string]string{"global.identity.auth.issuer": "https://idp.example.com"}},
		{name: "OrchestrationIssuer", values: map[string]string{"orchestration.security.authentication.oidc.issuer": "https://idp.example.com"}},
		{
			name: "GlobalEndpoints",
			values: map[string]string{
				"global.identity.auth.authUrl":  "https://idp.example.com/auth",
				"global.identity.auth.jwksUrl":  "https://idp.example.com/certs",
				"global.identity.auth.tokenUrl": "https://idp.example.com/token",
			},
		},
		{
			name: "OrchestrationEndpoints",
			values: map[string]string{
				"orchestration.security.authentication.oidc.authUrl":  "https://idp.example.com/auth",
				"orchestration.security.authentication.oidc.jwksUrl":  "https://idp.example.com/certs",
				"orchestration.security.authentication.oidc.tokenUrl": "https://idp.example.com/token",
			},
		},
		{name: "OrchestrationBasicAuth", values: map[string]string{"orchestration.security.authentication.method": "basic", "global.identity.auth.issuerBackendUrl": "https://idp.example.com"}},
	} {
		values := map[string]string{
			"identity.enabled":                         "true",
			"optimize.enabled":                         "false",
			"global.identity.auth.enabled":             "true",
			"global.identity.auth.type":                "GENERIC",
			"global.identity.auth.publicIssuerUrl":     "",
			"global.security.authentication.method":    "oidc",
			"orchestration.data.secondaryStorage.type": "elasticsearch",
		}
		for key, value := range scenario.values {
			values[key] = value
		}
		testCases = append(testCases, testhelpers.TestCase{
			Name:   scenario.name,
			Values: values,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				configMaps := map[string]corev1.ConfigMap{}
				decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)
				for {
					var resource corev1.ConfigMap
					err := decoder.Decode(&resource)
					if err == io.EOF {
						break
					}
					s.Require().NoError(err)
					if resource.Kind == "ConfigMap" {
						configMaps[resource.Name] = resource
					}
				}
				s.Require().Contains(configMaps, s.release+"-zeebe-configuration")
				warnings := configMaps[s.release+"-warnings"].Data["warnings"]
				if scenario.unresolved == "" {
					s.Require().NotContains(warnings, endpointWarning)
				} else {
					s.Require().Contains(warnings, endpointWarning+scenario.unresolved+".")
				}
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestWarningsConfigMapRendersWhenWarningPresent",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                      "elasticsearch",
				"identity.enabled":                                              "true",
				"global.identity.auth.enabled":                                  "true",
				"global.security.authentication.method":                         "oidc",
				"connectors.security.authentication.oidc.secret.existingSecret": "foo",
				"global.identity.auth.issuerBackendUrl":                         "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":          "warning",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				s.Require().Contains(configmap.Data["warnings"],
					"the Camunda Helm chart will no longer automatically generate passwords for the Identity component")
			},
		},
		{
			Name: "TestHistoryDeprecationWarningsNameAllKeysAndRemovalVersion",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":        "elasticsearch",
				"orchestration.history.elsRolloverDateFormat":     "yyyy-MM",
				"orchestration.history.rolloverInterval":          "2d",
				"orchestration.history.rolloverBatchSize":         "321",
				"orchestration.history.waitPeriodBeforeArchiving": "3h",
				"orchestration.history.delayBetweenRuns":          "4000",
				"orchestration.history.maxDelayBetweenRuns":       "12000",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				warnings := configmap.Data["warnings"]
				for _, key := range []string{
					"orchestration.history.elsRolloverDateFormat",
					"orchestration.history.rolloverInterval",
					"orchestration.history.rolloverBatchSize",
					"orchestration.history.waitPeriodBeforeArchiving",
					"orchestration.history.delayBetweenRuns",
					"orchestration.history.maxDelayBetweenRuns",
				} {
					s.Require().Contains(warnings, key)
				}
				s.Require().Contains(warnings, "orchestration.extraConfiguration")
				s.Require().Contains(warnings, "chart v16 (Camunda 8.11)")
			},
		},
		{
			Name: "TestWarningsAreNotSeparatedByBlankLines",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.history.rolloverInterval":   "2d",
				"orchestration.history.rolloverBatchSize":  "321",
				"orchestration.history.delayBetweenRuns":   "4000",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				lines := strings.Split(configmap.Data["warnings"], "\n")
				s.Require().GreaterOrEqual(len(lines), 3)
				for _, line := range lines {
					s.Require().NotEmpty(strings.TrimSpace(line))
				}
			},
		},
		{
			Name: "TestWarningsConfigMapAbsentWhenNoWarnings",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// With no active warnings the helper renders nothing, so --show-only finds no manifest.
				s.Require().Error(err)
				s.Require().NotContains(output, "kind: ConfigMap")
			},
		},
		{
			Name: "TestJavaToolOptionsWarningNamesCompatibleJavaOpts",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":  "elasticsearch",
				"global.tls.caBundle.secret.existingSecret": "camunda-ca-bundle",
				"orchestration.env[0].name":                 "JAVA_TOOL_OPTIONS",
				"orchestration.env[0].value":                "-Xmx1g",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"],
					"Orchestration and Optimize can set their 'javaOpts' values instead")
				s.Require().Contains(configmap.Data["warnings"],
					"webModeler.restapi.javaOpts feeds JAVA_OPTIONS, not JAVA_TOOL_OPTIONS")
				s.Require().NotContains(configmap.Data["warnings"],
					"web-modeler restapi) can set that instead")
			},
		},
		{
			Name: "TestEmptyCamundaHubEnvIgnoresLegacyJavaToolOptions",
			Values: map[string]string{
				"camundaHub.enabled":                                   "true",
				"camundaHub.restapi.mail.fromAddress":                  "example@example.com",
				"global.testDeprecationFlags.existingSecretsMustBeSet": "warning",
				"global.tls.caBundle.secret.existingSecret":            "camunda-ca-bundle",
				"identity.enabled":                                     "true",
				"orchestration.data.secondaryStorage.type":             "elasticsearch",
				"webModeler.restapi.env[0].name":                       "JAVA_TOOL_OPTIONS",
				"webModeler.restapi.env[0].value":                      "-Xmx1g",
			},
			RenderTemplateExtraArgs: []string{"--set-json", "camundaHub.restapi.env=[]"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				// Positive anchor: warning evaluation ran for this release.
				s.Require().Contains(configmap.Data["warnings"],
					"webModeler.restapi.pusher.secret.existingSecret")
				s.Require().NotContains(configmap.Data["warnings"],
					"webModeler.restapi.env sets JAVA_TOOL_OPTIONS directly")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestOptimizeCaBundlePlaintextWarning() {
	warningAnchorValues := map[string]string{
		"identity.enabled":                                              "true",
		"global.identity.auth.enabled":                                  "true",
		"global.security.authentication.method":                         "oidc",
		"connectors.security.authentication.oidc.secret.existingSecret": "foo",
		"global.identity.auth.issuerBackendUrl":                         "http://keycloak:80/auth/realms/camunda-platform",
		"global.testDeprecationFlags.existingSecretsMustBeSet":          "warning",
		"global.tls.caBundle.secret.existingSecret":                     "ca-bundle",
	}

	verifyWarning := func(warning string, expected bool) func(t *testing.T, output string, err error) {
		return func(t *testing.T, output string, err error) {
			s.Require().NoError(err)
			var configmap corev1.ConfigMap
			helm.UnmarshalK8SYaml(t, output, &configmap)
			s.Require().Contains(configmap.Data["warnings"], "DEPRECATION NOTICE")
			if expected {
				s.Require().Contains(configmap.Data["warnings"], warning)
			} else {
				s.Require().NotContains(configmap.Data["warnings"], warning)
			}
		}
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "Enabled Optimize Elasticsearch with HTTP warns",
			Values: mergeMaps(warningAnchorValues, map[string]string{
				"orchestration.data.secondaryStorage.type":     "elasticsearch",
				"optimize.database.elasticsearch.enabled":      "true",
				"optimize.database.elasticsearch.url.protocol": "http",
			}),
			Verifier: verifyWarning("optimize.database.elasticsearch.url.protocol is plaintext 'http'", true),
		},
		{
			Name: "Disabled Optimize Elasticsearch with HTTP does not warn",
			Values: mergeMaps(warningAnchorValues, map[string]string{
				"orchestration.data.secondaryStorage.type":     "opensearch",
				"optimize.database.elasticsearch.enabled":      "false",
				"optimize.database.elasticsearch.url.protocol": "http",
			}),
			Verifier: verifyWarning("optimize.database.elasticsearch.url.protocol is plaintext 'http'", false),
		},
		{
			Name: "Enabled Optimize OpenSearch with HTTP warns",
			Values: mergeMaps(warningAnchorValues, map[string]string{
				"orchestration.data.secondaryStorage.type":  "opensearch",
				"optimize.database.opensearch.enabled":      "true",
				"optimize.database.opensearch.url.protocol": "http",
				"optimize.database.opensearch.url.host":     "opensearch.example.com",
			}),
			Verifier: verifyWarning("optimize.database.opensearch.url.protocol is plaintext 'http'", true),
		},
		{
			Name: "Disabled Optimize OpenSearch with HTTP does not warn",
			Values: mergeMaps(warningAnchorValues, map[string]string{
				"orchestration.data.secondaryStorage.type":  "elasticsearch",
				"optimize.database.opensearch.enabled":      "false",
				"optimize.database.opensearch.url.protocol": "http",
			}),
			Verifier: verifyWarning("optimize.database.opensearch.url.protocol is plaintext 'http'", false),
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestLegacyExporterTruststoreConflict() {
	conflictingTruststores := map[string]string{
		"optimize.enabled":                                                            "true",
		"optimize.database.elasticsearch.enabled":                                     "false",
		"optimize.database.opensearch.enabled":                                        "true",
		"optimize.database.opensearch.url.host":                                       "optimize-host",
		"optimize.database.opensearch.tls.secret.existingSecret":                      "optimize-tls-secret",
		"optimize.database.opensearch.tls.secret.existingSecretKey":                   "optimize-ca.jks",
		"orchestration.data.secondaryStorage.type":                                    "opensearch",
		"orchestration.data.secondaryStorage.opensearch.url":                          "https://secondary-host:9443",
		"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecret":    "secondary-tls-secret",
		"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecretKey": "secondary-ca.jks",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "Legacy exporter and secondary storage truststores conflict",
			Values: conflictingTruststores,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "TRUSTSTORE CONFLICT")
			},
		},
		{
			Name: "Shared truststore secret does not warn",
			Values: mergeMaps(conflictingTruststores, map[string]string{
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecret":    "optimize-tls-secret",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecretKey": "optimize-ca.jks",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().NotContains(configmap.Data["warnings"], "TRUSTSTORE CONFLICT")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func mergeMaps(base map[string]string, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

func (s *ConfigMapWarningsTemplateTest) TestConsoleConfigKeysWarningRendersInConfigMap() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestConsoleNonEnabledKeyTriggersConsolidationWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"console.someUnusedKey":                    "someValue",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				s.Require().Contains(configmap.Data["warnings"],
					"console.* configuration keys have no effect in 8.10")
				s.Require().Contains(configmap.Data["warnings"],
					"consolidated into Camunda Hub")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestConsoleEnabledOnlyKeepsConsolidationWarningSilent() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestConsoleEnabledAloneDoesNotTriggerConsolidationWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"console.enabled":                          "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				s.Require().Contains(configmap.Data["warnings"],
					`DEPRECATION: "console.enabled" is deprecated and will be removed in chart v16 (Camunda 8.11).`)
				s.Require().NotContains(configmap.Data["warnings"],
					"console.* configuration keys have no effect in 8.10")
			},
		},
		{
			Name: "TestConsoleEnabledWithRemovedOverrideGivesConsistentGuidance",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"console.enabled":                          "true",
				"console.nodeEnv":                          "someValue",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings,
					`DEPRECATION: "console.enabled" is deprecated and will be removed in chart v16 (Camunda 8.11).`)
				s.Require().Contains(warnings,
					"console.* configuration keys have no effect in 8.10")
				s.Require().NotContains(warnings,
					`Any console-specific overrides should use the top-level "console.*" keys.`)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestGlobalIdentityAuthConsoleDeprecationWarningRendersInConfigMap() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestGlobalIdentityAuthConsoleKeyTriggersDeprecationWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.identity.auth.console.clientId":    "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				s.Require().Contains(configmap.Data["warnings"],
					`DEPRECATION: "global.identity.auth.console.*" is no longer used in Camunda 8.10.`)
				s.Require().Contains(configmap.Data["warnings"],
					"this key has no replacement")
				s.Require().NotContains(configmap.Data["warnings"],
					"global.identity.auth.camundaHub.webModeler.*")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestWebModelerRestapiLegacyEnvOverrideWarning() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestLegacyPusherAndMailEnvOverridesTriggerUpgradeWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":    "elasticsearch",
				"webModeler.enabled":                          "true",
				"webModeler.restapi.mail.fromAddress":         "example@example.com",
				"identity.enabled":                            "true",
				"webModeler.restapi.env[0].name":              "RESTAPI_PUSHER_APP_ID",
				"webModeler.restapi.env[0].value":             "custom",
				"webModeler.restapi.env[1].name":              "RESTAPI_PUSHER_KEY",
				"webModeler.restapi.env[1].value":             "custom",
				"webModeler.restapi.env[2].name":              "RESTAPI_PUSHER_SECRET",
				"webModeler.restapi.env[2].value":             "custom",
				"webModeler.restapi.env[3].name":              "RESTAPI_MAIL_PASSWORD",
				"webModeler.restapi.env[3].value":             "custom",
				"webModeler.restapi.mail.secret.inlineSecret": "smtp-password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				for oldName, newName := range map[string]string{
					"RESTAPI_PUSHER_APP_ID": "CAMUNDA_HUB_PUSHER_APPID",
					"RESTAPI_PUSHER_KEY":    "CAMUNDA_HUB_PUSHER_KEY",
					"RESTAPI_PUSHER_SECRET": "CAMUNDA_HUB_PUSHER_SECRET",
					"RESTAPI_MAIL_PASSWORD": "SPRING_MAIL_PASSWORD",
				} {
					s.Require().Contains(configmap.Data["warnings"], fmt.Sprintf(
						"[camunda][warning] restapi.env sets %q, "+
							"which is ignored because the chart now sets %q. "+
							"Rename the override to %q, otherwise the chart-managed value is used instead of yours.",
						oldName, newName, newName))
				}
			},
		},
		{
			Name: "TestLegacyMailPasswordWithoutMailSecretIsOnlyDeprecated",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"webModeler.enabled":                       "true",
				"webModeler.restapi.mail.fromAddress":      "example@example.com",
				"identity.enabled":                         "true",
				"webModeler.restapi.env[0].name":           "RESTAPI_MAIL_PASSWORD",
				"webModeler.restapi.env[0].value":          "custom",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"],
					`[camunda][warning] restapi.env sets the deprecated "RESTAPI_MAIL_PASSWORD". Rename it to "SPRING_MAIL_PASSWORD"`)
				s.Require().NotContains(configmap.Data["warnings"], "which is ignored because the chart now sets \"SPRING_MAIL_PASSWORD\"")
			},
		},
		{
			Name: "TestLegacyEnvOverrideUnderCamundaHubTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"camundaHub.enabled":                       "true",
				"camundaHub.restapi.mail.fromAddress":      "example@example.com",
				"identity.enabled":                         "true",
				"camundaHub.restapi.env[0].name":           "RESTAPI_PUSHER_KEY",
				"camundaHub.restapi.env[0].value":          "custom",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"],
					`[camunda][warning] restapi.env sets "RESTAPI_PUSHER_KEY", which is ignored because the chart now sets "CAMUNDA_HUB_PUSHER_KEY".`)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestNginxCompatAnnotationsDeprecationWarning() {
	const warning = "global.compatibility.nginx.renderAnnotations is enabled"

	testCases := []testhelpers.TestCase{
		{
			Name: "TestShimOnWithAnIngressWarns",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.ingress.enabled":                   "true",
				"global.host":                              "camunda.example.com",
				"orchestration.contextPath":                "/",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, warning)
				s.Require().Contains(warnings, "removed in the next major")
			},
		},
		{
			Name: "TestShimOffDoesNotWarn",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":     "elasticsearch",
				"global.ingress.enabled":                       "true",
				"global.compatibility.nginx.renderAnnotations": "false",
				"global.host":                           "camunda.example.com",
				"orchestration.contextPath":             "/",
				"global.identity.auth.console.clientId": "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:        "TestGrpcOnlyReleaseWithEveryGrpcKeySetDoesNotWarn",
			ValuesFiles: []string{"testdata/values-nginx-compat-grpc-keys-set.yaml"},
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.ingress.grpc.host":          "zeebe.example.com",
				"global.identity.auth.console.clientId":    "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:        "TestHttpOnlyReleaseWithEveryHttpKeySetDoesNotWarn",
			ValuesFiles: []string{"testdata/values-nginx-compat-http-keys-set.yaml"},
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.host":                           "camunda.example.com",
				"orchestration.contextPath":             "/",
				"global.identity.auth.console.clientId": "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "TestIngressEnabledWithoutHttpPathsDoesNotWarn",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.ingress.enabled":                   "true",
				"global.host":                              "camunda.example.com",
				"global.identity.auth.console.clientId":    "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:        "TestTemplatedKeysCoveringTheLegacySetDoNotWarn",
			ValuesFiles: []string{"testdata/values-grpc-annotations-templated-keys-complete.yaml"},
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.identity.auth.console.clientId":    "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "TestGrpcRouteWithShimActiveWarns",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.ingress.grpc.enabled":       "true",
				"orchestration.ingress.grpc.host":          "zeebe.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "TestShimOnWithoutAnyIngressDoesNotWarn",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.identity.auth.console.clientId":    "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestIngressUpstreamTLSControllerWarning() {
	const warning = "Only ingress-nginx reads that annotation"

	restTLS := map[string]string{
		"orchestration.data.secondaryStorage.type":                 "elasticsearch",
		"global.ingress.enabled":                                   "true",
		"global.host":                                              "camunda.example.com",
		"orchestration.contextPath":                                "/",
		"global.tls.orchestration.rest.enabled":                    "true",
		"global.tls.orchestration.rest.cert.secret.existingSecret": "orchestration-ks",
	}
	withClassName := func(class string) map[string]string {
		values := map[string]string{"global.ingress.className": class}
		for k, v := range restTLS {
			values[k] = v
		}
		return values
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestNonNginxClassWithUpstreamTLSNamesTheComponentAndClass",
			Values: withClassName("contour"),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, warning)
				s.Require().Contains(warnings, "Upstream TLS is enabled for the Orchestration REST server")
				s.Require().Contains(warnings, `global.ingress.className is "contour"`)
				s.Require().Contains(warnings, "projectcontour.io/upstream-protocol.tls")
			},
		},
		{
			Name:        "TestUpstreamTLSFromEnvNamesTheComponentNotTheFlag",
			ValuesFiles: []string{"testdata/values-orchestration-rest-tls-via-env.yaml"},
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                 "elasticsearch",
				"global.ingress.enabled":                                   "true",
				"global.ingress.className":                                 "contour",
				"global.host":                                              "camunda.example.com",
				"global.tls.orchestration.rest.cert.secret.existingSecret": "orchestration-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, warning)
				s.Require().Contains(warnings, "Upstream TLS is enabled for the Orchestration REST server")
				s.Require().NotContains(warnings, "global.tls.orchestration.rest enabled",
					"TLS came from the env var here, so the warning must not attribute it to the flag")
			},
		},
		{
			Name:   "TestNginxClassWithUpstreamTLSDoesNotWarn",
			Values: withClassName("nginx"),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, warning)
			},
		},
		{
			Name: "TestGRPCIngressIsCheckedOnItsOwnClassAndGate",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                 "elasticsearch",
				"orchestration.ingress.grpc.enabled":                       "true",
				"orchestration.ingress.grpc.className":                     "contour",
				"orchestration.ingress.grpc.host":                          "zeebe.example.com",
				"global.tls.orchestration.grpc.enabled":                    "true",
				"global.tls.orchestration.grpc.cert.secret.existingSecret": "orchestration-crt",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, warning)
				s.Require().Contains(warnings, "projectcontour.io/upstream-protocol.h2")
			},
		},
		{
			Name: "TestConnectorsRenderedRouteWithUpstreamTLSNamesConnectors",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":         "elasticsearch",
				"global.ingress.enabled":                           "true",
				"global.ingress.className":                         "contour",
				"global.host":                                      "camunda.example.com",
				"connectors.enabled":                               "true",
				"connectors.contextPath":                           "/connectors",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, warning)
				s.Require().Contains(warnings, "Upstream TLS is enabled for Connectors")
			},
		},
		{
			Name: "TestOptimizeRenderedRouteWithUpstreamTLSNamesOptimize",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":       "elasticsearch",
				"global.ingress.enabled":                         "true",
				"global.ingress.className":                       "contour",
				"global.host":                                    "camunda.example.com",
				"optimize.enabled":                               "true",
				"optimize.contextPath":                           "/optimize",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, warning)
				s.Require().Contains(warnings, "Upstream TLS is enabled for Optimize")
			},
		},
		{
			Name: "TestUpstreamTLSOnAComponentWithNoRouteDoesNotWarn",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":       "elasticsearch",
				"global.ingress.enabled":                         "true",
				"global.ingress.className":                       "contour",
				"global.host":                                    "camunda.example.com",
				"optimize.enabled":                               "false",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"global.identity.auth.console.clientId":          "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "TestNonNginxClassWithoutUpstreamTLSDoesNotWarn",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.ingress.enabled":                   "true",
				"global.ingress.className":                 "contour",
				"global.host":                              "camunda.example.com",
				"orchestration.contextPath":                "/",
				"global.identity.auth.console.clientId":    "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "TestUpstreamTLSWithoutAnyIngressDoesNotWarn",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                 "elasticsearch",
				"global.tls.orchestration.rest.enabled":                    "true",
				"global.tls.orchestration.rest.cert.secret.existingSecret": "orchestration-ks",
				"global.identity.auth.console.clientId":                    "some-console-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "global.identity.auth.console")
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestMigrationDisruptionBudgetWarning() {
	zonedMigrationValues := func() map[string]string {
		return map[string]string{
			"orchestration.data.secondaryStorage.type":             "elasticsearch",
			"orchestration.profiles.broker":                        "true",
			"orchestration.partitioning.scheme":                    "zone-aware",
			"orchestration.partitioning.zone":                      "zone-a",
			"orchestration.partitioning.zones[0].name":             "zone-a",
			"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
			"orchestration.partitioning.zones[0].numberOfReplicas": "1",
			"orchestration.partitioning.zones[0].priority":         "100",
			"orchestration.partitioning.keepUnzonedBrokers":        "true",
		}
	}
	const warning = "covered by one PodDisruptionBudget each"

	retainedWithBudget := zonedMigrationValues()
	retainedWithBudget["orchestration.podDisruptionBudget.enabled"] = "true"

	retainedWithoutBudget := zonedMigrationValues()
	retainedWithoutBudget["orchestration.podDisruptionBudget.enabled"] = "false"

	budgetWithoutRetention := zonedMigrationValues()
	budgetWithoutRetention["orchestration.partitioning.keepUnzonedBrokers"] = "false"
	budgetWithoutRetention["orchestration.podDisruptionBudget.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestRetentionWithADisruptionBudgetWarns",
			Values: retainedWithBudget,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:   "TestRetentionWithoutADisruptionBudgetDoesNotWarn",
			Values: retainedWithoutBudget,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:   "TestADisruptionBudgetWithoutRetentionDoesNotWarn",
			Values: budgetWithoutRetention,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestFailureDomainContactPointsWarning() {
	const warning = "This deployment spans more than one failure domain"

	values := func(keepUnzonedBrokers bool, contactPointsSet bool) map[string]string {
		result := map[string]string{
			"orchestration.data.secondaryStorage.type": "elasticsearch",
			"global.identity.auth.console.clientId":    "warning-anchor",
		}
		if keepUnzonedBrokers {
			result["orchestration.profiles.broker"] = "true"
			result["orchestration.partitioning.scheme"] = "zone-aware"
			result["orchestration.partitioning.zone"] = "zone-a"
			result["orchestration.partitioning.zones[0].name"] = "zone-a"
			result["orchestration.partitioning.zones[0].numberOfBrokers"] = "1"
			result["orchestration.partitioning.zones[0].numberOfReplicas"] = "1"
			result["orchestration.partitioning.zones[0].priority"] = "100"
			result["orchestration.partitioning.zones[1].name"] = "zone-b"
			result["orchestration.partitioning.zones[1].numberOfBrokers"] = "1"
			result["orchestration.partitioning.zones[1].numberOfReplicas"] = "1"
			result["orchestration.partitioning.zones[1].priority"] = "50"
			result["orchestration.partitioning.keepUnzonedBrokers"] = fmt.Sprint(keepUnzonedBrokers)
		} else {
			result["orchestration.partitioning.numberOfZones"] = "3"
			result["orchestration.partitioning.zoneIndex"] = "0"
		}
		if contactPointsSet {
			result["orchestration.env[0].name"] = `\{\{ printf "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS" \}\}`
			result["orchestration.env[0].value"] = "camunda-zeebe-0.camunda-zeebe.default.svc.cluster.local:26502"
		}
		return result
	}

	verifyWarning := func(expected bool) func(t *testing.T, output string, err error) {
		return func(t *testing.T, output string, err error) {
			s.Require().NoError(err)
			var configmap corev1.ConfigMap
			helm.UnmarshalK8SYaml(s.T(), output, &configmap)
			if expected {
				s.Require().Contains(configmap.Data["warnings"], warning)
			} else {
				s.Require().NotContains(configmap.Data["warnings"], warning)
			}
		}
	}

	testCases := []testhelpers.TestCase{
		{Name: "Round-robin without contact points warns", Values: values(false, false), Verifier: verifyWarning(true)},
		{Name: "Round-robin with contact points does not warn", Values: values(false, true), Verifier: verifyWarning(false)},
		{Name: "Migration without contact points warns", Values: values(true, false), Verifier: verifyWarning(true)},
		{Name: "Migration with contact points does not warn", Values: values(true, true), Verifier: verifyWarning(false)},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestZonedFullConfigurationWarning() {
	zonedValues := func() map[string]string {
		return map[string]string{
			"orchestration.data.secondaryStorage.type":             "elasticsearch",
			"orchestration.profiles.broker":                        "true",
			"orchestration.partitioning.scheme":                    "zone-aware",
			"orchestration.partitioning.zone":                      "zone-a",
			"orchestration.partitioning.zones[0].name":             "zone-a",
			"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
			"orchestration.partitioning.zones[0].numberOfReplicas": "1",
			"orchestration.partitioning.zones[0].priority":         "100",
		}
	}
	const warning = "replaces the whole generated application.yaml"

	fullConfiguration := zonedValues()
	fullConfiguration["orchestration.configuration"] = "camunda: {}"

	extraConfiguration := zonedValues()
	extraConfiguration["orchestration.extraConfiguration[0].file"] = "application-extra.yaml"
	extraConfiguration["orchestration.extraConfiguration[0].content"] = "camunda: {}"

	numberedFullConfiguration := map[string]string{
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"orchestration.profiles.broker":            "true",
		"orchestration.configuration":              "camunda: {}",
		"global.identity.auth.console.clientId":    "some-console-client",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestZonedFullConfigurationWarns",
			Values: fullConfiguration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:   "TestZonedExtraConfigurationDoesNotWarn",
			Values: extraConfiguration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:   "TestNumberedFullConfigurationDoesNotWarn",
			Values: numberedFullConfiguration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().NotContains(configmap.Data["warnings"], warning)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestContactPointWarningHonorsOperatorDeclarations() {
	const warning = "chart cannot generate the broker bootstrap list"

	values := func() map[string]string {
		return map[string]string{
			"orchestration.data.secondaryStorage.type":             "elasticsearch",
			"orchestration.profiles.broker":                        "true",
			"orchestration.partitioning.scheme":                    "zone-aware",
			"orchestration.partitioning.zone":                      "zone-a",
			"orchestration.partitioning.zones[0].name":             "zone-a",
			"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
			"orchestration.partitioning.zones[0].numberOfReplicas": "1",
			"orchestration.partitioning.zones[0].priority":         "100",
			"orchestration.partitioning.zones[1].name":             "zone-b",
			"orchestration.partitioning.zones[1].numberOfBrokers":  "1",
			"orchestration.partitioning.zones[1].numberOfReplicas": "1",
			"orchestration.partitioning.zones[1].priority":         "50",
		}
	}

	modernDeclaration := values()
	modernDeclaration["orchestration.envFrom[0].configMapRef.name"] = "cluster-environment"
	modernDeclaration["orchestration.envFromProvides[0]"] = "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS"

	legacyDeclaration := values()
	legacyDeclaration["orchestration.envFrom[0].secretRef.name"] = "cluster-environment"
	legacyDeclaration["orchestration.envFromProvides[0]"] = "ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS"

	directEnvDeclaration := values()
	directEnvDeclaration["orchestration.env[0].name"] = `\{\{ printf "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS" \}\}`
	directEnvDeclaration["orchestration.env[0].value"] = "zone-a.example:26502"

	extraConfigurationDeclaration := values()
	extraConfigurationDeclaration["orchestration.extraConfiguration[0].file"] = "cluster.yaml"
	extraConfigurationDeclaration["orchestration.extraConfiguration[0].content"] = "camunda:\n  cluster:\n    initial-contact-points: zone-a.example:26502\n"

	configurationDeclaration := values()
	configurationDeclaration["orchestration.configuration"] = "camunda:\n  cluster:\n    initial-contact-points: zone-a.example:26502\n"

	testCases := []testhelpers.TestCase{
		{
			Name:   "Missing contact point declaration warns",
			Values: values(),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name:   "Modern envFrom contact point declaration suppresses warning",
			Values: modernDeclaration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, warning)
			},
		},
		{
			Name:   "Legacy envFrom contact point declaration suppresses warning",
			Values: legacyDeclaration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, warning)
			},
		},
		{
			Name:   "Templated direct env contact point declaration suppresses warning",
			Values: directEnvDeclaration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, warning)
			},
		},
		{
			Name:   "Extra configuration contact point declaration suppresses warning",
			Values: extraConfigurationDeclaration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, warning)
			},
		},
		{
			Name:   "Complete configuration suppresses warning",
			Values: configurationDeclaration,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, warning)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestPvcAccessModesReadWriteOncePodWarning() {
	testCases := []testhelpers.TestCase{
		{
			Name: "ReadWriteOncePodTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.pvcAccessModes[0]":          "ReadWriteOncePod",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"],
					"orchestration.pvcAccessModes is set to ReadWriteOncePod")
			},
		},
		{
			Name: "DefaultReadWriteOnceDoesNotTriggerWarning",
			Values: map[string]string{
				// Another warning must stay active so the ConfigMap still renders (it is omitted
				// entirely when no warnings are present, see TestWarningsConfigMapAbsentWhenNoWarnings).
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.history.rolloverInterval":   "2d",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().NotContains(configmap.Data["warnings"], "pvcAccessModes")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestNoSecondaryStorageTypeConflictWarning() {
	const warning = "global.noSecondaryStorage=true conflicts with orchestration.data.secondaryStorage.type"
	testCases := []testhelpers.TestCase{}
	for _, scenario := range []struct {
		name               string
		noSecondaryStorage string
		storageType        string
		warn               bool
	}{
		{name: "ElasticsearchWarns", noSecondaryStorage: "true", storageType: "elasticsearch", warn: true},
		{name: "OpensearchWarns", noSecondaryStorage: "true", storageType: "opensearch", warn: true},
		{name: "RdbmsWarns", noSecondaryStorage: "true", storageType: "rdbms", warn: true},
		{name: "NoneDoesNotWarn", noSecondaryStorage: "true", storageType: "none"},
		{name: "UnsetTypeDoesNotWarn", noSecondaryStorage: "true"},
		{name: "SecondaryStorageEnabledDoesNotWarn", noSecondaryStorage: "false", storageType: "elasticsearch"},
	} {
		testCases = append(testCases, testhelpers.TestCase{
			Name: "TestNoSecondaryStorageTypeConflict" + scenario.name,
			Values: map[string]string{
				"global.noSecondaryStorage":                scenario.noSecondaryStorage,
				"orchestration.data.secondaryStorage.type": scenario.storageType,
				"orchestration.pvcAccessModes[0]":          "ReadWriteOncePod",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, "orchestration.pvcAccessModes is set to ReadWriteOncePod")
				if scenario.warn {
					s.Require().Contains(warnings, warning+"="+scenario.storageType+":")
				} else {
					s.Require().NotContains(warnings, warning)
				}
			},
		})
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestDualRegionReplicationFactorWarning() {
	const warningSuffix = "is 2; a dual-region cluster needs a replication factor of 4"

	noWarning := func(t *testing.T, output string, err error) {
		s.Require().NoError(err)
		s.Require().NotContains(output, "orchestration.replicationFactor is")
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "NumberOfZonesTwoWithReplicationFactorNotFourTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.partitioning.numberOfZones": "2",
				"orchestration.partitioning.zoneIndex":     "0",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"],
					"orchestration.replicationFactor is 3 but orchestration.partitioning.numberOfZones "+warningSuffix)
			},
		},
		{
			Name: "DeprecatedRegionsTwoWithReplicationFactorNotFourTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"],
					"orchestration.replicationFactor is 3 but global.multiregion.regions "+warningSuffix)
			},
		},
		{
			Name: "ReplicationFactorFourDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.partitioning.numberOfZones": "2",
				"orchestration.partitioning.zoneIndex":     "0",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "orchestration.clusterSize=4",
				"--set-string", "orchestration.replicationFactor=4",
			},
			Verifier: noWarning,
		},
		{
			Name: "ZoneAwareSchemeDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":             "elasticsearch",
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "zone-a",
				"orchestration.partitioning.zones[0].name":             "zone-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.partitioning.zones[1].name":             "zone-b",
				"orchestration.partitioning.zones[1].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[1].numberOfReplicas": "1",
				"orchestration.partitioning.zones[1].priority":         "50",
			},
			Verifier: noWarning,
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestDefaultRolesMappingRulesDeprecationWarning() {
	const adminWarning = `DEPRECATION: The Helm values file key "orchestration.security.initialization.defaultRoles.admin.mappingRules" is deprecated and will be removed in chart v16 (Camunda 8.11). Configure this via "orchestration.extraConfiguration" instead.`
	const connectorsWarning = `DEPRECATION: The Helm values file key "orchestration.security.initialization.defaultRoles.connectors.mappingRules" is deprecated and will be removed in chart v16 (Camunda 8.11). Configure this via "orchestration.extraConfiguration" instead.`

	testCases := []testhelpers.TestCase{
		{
			Name: "TestDefaultRolesMappingRulesSetTriggersDeprecationWarnings",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                      "elasticsearch",
				"orchestration.security.initialization.defaultRoles.admin.mappingRules[0]":      "admin-rule",
				"orchestration.security.initialization.defaultRoles.connectors.mappingRules[0]": "connectors-rule",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				require.Contains(t, configmap.Data["warnings"], adminWarning)
				require.Contains(t, configmap.Data["warnings"], connectorsWarning)
			},
		},
		{
			Name: "TestCustomDefaultRoleMappingRulesTriggersDeprecationWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                  "elasticsearch",
				"orchestration.security.initialization.defaultRoles.custom.mappingRules[0]": "custom-rule",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				require.Contains(t, configmap.Data["warnings"], `"orchestration.security.initialization.defaultRoles.custom.mappingRules" is deprecated`)
				require.NotContains(t, configmap.Data["warnings"], "defaultRoles.admin.mappingRules")
			},
		},
		{
			Name: "TestDefaultRolesMappingRulesUnsetDoesNotTriggerDeprecationWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"console.enabled":                          "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				require.NotContains(t, configmap.Data["warnings"], "defaultRoles")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

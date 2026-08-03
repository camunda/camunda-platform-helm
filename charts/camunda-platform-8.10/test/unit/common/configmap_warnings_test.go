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
	corev1 "k8s.io/api/core/v1"
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

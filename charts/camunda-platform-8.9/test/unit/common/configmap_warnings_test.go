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
				"elasticsearch.enabled":                                         "true",
				"global.elasticsearch.enabled":                                  "true",
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
				s.Require().Contains(configmap.Data["warnings"],
					"The following Bitnami-based subcharts are deprecated and will be removed in Camunda 8.10: [elasticsearch].")
			},
		},
		{
			Name: "TestWarningsConfigMapAbsentWhenNoWarnings",
			// Both ES flags off avoid the legacy-option deprecation warning (the test helper
			// otherwise defaults them to true); the new secondaryStorage key satisfies the
			// storage-type constraint.
			Values: map[string]string{
				"elasticsearch.enabled":                    "false",
				"global.elasticsearch.enabled":             "false",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// With no active warnings the helper renders nothing, so --show-only finds no manifest.
				s.Require().Error(err)
				s.Require().NotContains(output, "kind: ConfigMap")
			},
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

func (s *ConfigMapWarningsTemplateTest) TestBundledKeycloakCveWarning() {
	baseValues := map[string]string{
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"identity.enabled":                         "true",
		"identityKeycloak.enabled":                 "true",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestAffectedVersionWarns",
			Values: baseValues,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "CVE-2026-18963")
			},
		},
		{
			Name: "TestBitnamiRevisionSuffixIsParsed",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "26.3.3-debian-12-r0-2026-08-27-001",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "CVE-2026-18963")
			},
		},
		{
			Name: "TestFixedVersionDoesNotWarn",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "26.7.2",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestUnparseableTagDoesNotWarn",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "latest",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestDisabledKeycloakDoesNotWarn",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.enabled": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NotContains(output, "CVE-2026-18963")
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

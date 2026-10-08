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
		{name: "MissingKeycloakProvider", values: map[string]string{"global.identity.auth.type": "KEYCLOAK"}, warn: true, backend: "http://:/auth/realms/camunda-platform"},
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
			backend: "%!s(<nil>)://keycloak.example.com:443/auth/realms/camunda-platform",
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
			backend: "https://keycloak.example.com:<nil>/auth/realms/camunda-platform",
		},
		{
			name: "BundledKeycloak",
			values: map[string]string{
				"global.identity.auth.type":    "KEYCLOAK",
				"identityKeycloak.enabled":     "true",
				"global.identity.keycloak.url": "null",
			},
			backend: "http://" + s.release + "-keycloak/auth/realms/camunda-platform",
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
			"identityKeycloak.enabled":                 "false",
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
			Name: "TestWarningsAreNotSeparatedByBlankLines",
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
				lines := strings.Split(configmap.Data["warnings"], "\n")
				s.Require().GreaterOrEqual(len(lines), 3)
				for _, line := range lines {
					s.Require().NotEmpty(strings.TrimSpace(line))
				}
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
			Name: "TestPrefixedFrozenTagWarns",
			// The upstream publish workflow also tags the frozen build as "bitnami-<version>".
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "bitnami-26.3.3",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "CVE-2026-18963")
			},
		},
		{
			Name: "TestMovingFrozenAliasWarns",
			// "bitnami-26" carries no version, but it resolves to the frozen 26.3.3 build.
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "bitnami-26",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, "CVE-2026-18963")
				s.Require().Contains(warnings, `uses the moving tag "bitnami-26"`)
				s.Require().Contains(warnings, "frozen on the discontinued bitnamilegacy base")
			},
		},
		{
			Name: "TestLatestBitnamiAliasWarns",
			// The frozen line also moves under "bitnami-latest" and "latest-bitnami".
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "latest-bitnami",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "CVE-2026-18963")
			},
		},
		{
			Name: "TestMaintainedQuayAliasDoesNotWarn",
			// The "quay-*" tags track upstream Keycloak and are still maintained.
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "quay-26",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				// A no-warning render produces no manifest, which --show-only reports as a
				// missing template; any other error means the render broke for an unrelated
				// reason and must not pass as "no warning".
				if err != nil {
					s.Require().Contains(err.Error(), "could not find template")
				}
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestOverriddenRepositoryOmitsFrozenLineClaim",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.repository": "acme/keycloak",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Require().Contains(warnings, "CVE-2026-18963")
				s.Require().NotContains(warnings, "frozen on the discontinued bitnamilegacy base")
			},
		},
		{
			Name: "TestLastAffectedVersionWarns",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "26.7.1",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "CVE-2026-18963")
			},
		},
		{
			Name: "TestFixedVersionWithBitnamiSuffixDoesNotWarn",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "26.7.2-debian-12-r0",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				// A no-warning render produces no manifest, which --show-only reports as a
				// missing template; any other error means the render broke for an unrelated
				// reason and must not pass as "no warning".
				if err != nil {
					s.Require().Contains(err.Error(), "could not find template")
				}
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestFixedVersionDoesNotWarn",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "26.7.2",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				// A no-warning render produces no manifest, which --show-only reports as a
				// missing template; any other error means the render broke for an unrelated
				// reason and must not pass as "no warning".
				if err != nil {
					s.Require().Contains(err.Error(), "could not find template")
				}
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestMaintainedLatestTagDoesNotWarn",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.image.tag": "latest",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				// A no-warning render produces no manifest, which --show-only reports as a
				// missing template; any other error means the render broke for an unrelated
				// reason and must not pass as "no warning".
				if err != nil {
					s.Require().Contains(err.Error(), "could not find template")
				}
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestDisabledKeycloakDoesNotWarn",
			Values: mergeMaps(baseValues, map[string]string{
				"identityKeycloak.enabled": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				// A no-warning render produces no manifest, which --show-only reports as a
				// missing template; any other error means the render broke for an unrelated
				// reason and must not pass as "no warning".
				if err != nil {
					s.Require().Contains(err.Error(), "could not find template")
				}
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
			// Another warning must stay active so the ConfigMap still renders (it is omitted
			// entirely when no warnings are present, see TestWarningsConfigMapAbsentWhenNoWarnings).
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"elasticsearch.enabled":                    "true",
				"global.elasticsearch.enabled":             "true",
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

func (s *ConfigMapWarningsTemplateTest) TestMultiregionClusterSizeDivisibilityWarning() {
	const warning = "orchestration.clusterSize is 5 but global.multiregion.regions is 2, so the regions deploy 4 brokers while every broker expects 5"

	testCases := []testhelpers.TestCase{
		{
			Name: "ClusterSizeTheRegionsDoNotDivideTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=5"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "ClusterSizeTheRegionsDivideDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"elasticsearch.enabled":                    "true",
				"global.elasticsearch.enabled":             "true",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().NotContains(configmap.Data["warnings"], "orchestration.clusterSize is")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestMultiregionReplicationFactorWarning() {
	const warning = "orchestration.replicationFactor is 3 but global.multiregion.regions is 2; a dual-region cluster needs a replication factor of 4"

	testCases := []testhelpers.TestCase{
		{
			Name: "DualRegionReplicationFactorNotFourTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "DualRegionReplicationFactorFourDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.replicationFactor=4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "orchestration.replicationFactor is")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestWebModelerRecreateWithoutExistingClaimWarning() {
	const warning = "webModeler.persistence.deploymentStrategy=Recreate gives no benefit without webModeler.persistence.existingClaim"

	noWarning := func(t *testing.T, output string, err error) {
		s.Require().NoError(err)
		var configmap corev1.ConfigMap
		helm.UnmarshalK8SYaml(s.T(), output, &configmap)
		s.Require().NotContains(configmap.Data["warnings"], "webModeler.persistence.deploymentStrategy")
	}
	testCases := []testhelpers.TestCase{
		{
			Name: "RecreateWithChartManagedPersistenceTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":  "elasticsearch",
				"identity.enabled":                          "true",
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "example@example.com",
				"webModeler.persistence.enabled":            "true",
				"webModeler.persistence.deploymentStrategy": "Recreate",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], warning)
			},
		},
		{
			Name: "RecreateWithExistingClaimDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":  "elasticsearch",
				"elasticsearch.enabled":                     "true",
				"global.elasticsearch.enabled":              "true",
				"identity.enabled":                          "true",
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "example@example.com",
				"webModeler.persistence.enabled":            "true",
				"webModeler.persistence.existingClaim":      "my-existing-pvc",
				"webModeler.persistence.deploymentStrategy": "Recreate",
			},
			Verifier: noWarning,
		},
		{
			Name: "RollingUpdateWithChartManagedPersistenceDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"elasticsearch.enabled":                    "true",
				"global.elasticsearch.enabled":             "true",
				"identity.enabled":                         "true",
				"webModeler.enabled":                       "true",
				"webModeler.restapi.mail.fromAddress":      "example@example.com",
				"webModeler.persistence.enabled":           "true",
			},
			Verifier: noWarning,
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

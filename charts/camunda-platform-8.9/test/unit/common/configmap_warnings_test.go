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
		{
			name: "BundledKeycloak",
			values: map[string]string{
				"global.identity.auth.type":            "KEYCLOAK",
				"global.identity.auth.publicIssuerUrl": "http://localhost:18080/auth/realms/camunda-platform",
				"identityKeycloak.enabled":             "true",
				"global.identity.keycloak.url":         "null",
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
			"identityKeycloak.enabled":                 "false",
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

func (s *ConfigMapWarningsTemplateTest) TestRDBMSURLWarning() {
	const rdbmsWarning = "The Orchestration Cluster uses RDBMS secondary storage, but no JDBC URL is set"
	const jdbcURL = "jdbc:postgresql://pg:5432/camunda"
	extraConfiguration := func(file, content string) []string {
		return []string{"--set-json", `orchestration.extraConfiguration=[{"file":"` + file + `","content":"` + content + `"}]`}
	}
	configuration := func(content string) []string {
		return []string{"--set-json", `orchestration.configuration="camunda:\n  data:\n    secondary-storage:\n      type: rdbms` + content + `"`}
	}
	testCases := []testhelpers.TestCase{}
	for _, scenario := range []struct {
		name   string
		values map[string]string
		args   []string
		warn   bool
	}{
		{name: "NoURL", warn: true},
		{name: "ExporterFlagOnly", values: map[string]string{"orchestration.data.secondaryStorage.type": "", "orchestration.exporters.rdbms.enabled": "true"}, warn: true},
		{name: "UnrelatedEnv", values: map[string]string{"orchestration.env[0].name": "FOO", "orchestration.env[0].value": "bar"}, warn: true},
		{name: "RDBMSConfigWithoutURL", args: extraConfiguration("rdbms.yaml", `camunda.data.secondary-storage.rdbms.username: camunda`), warn: true},
		{name: "NonSpringExtraConfiguration", args: []string{"--set-json", `orchestration.extraConfiguration=[{"file":"rdbms.yaml","content":"camunda.data.secondary-storage.rdbms.url: ` + jdbcURL + `","springImport":false}]`}, warn: true},
		{name: "ConfigurationReplacesValuesURL", values: map[string]string{"orchestration.data.secondaryStorage.rdbms.url": jdbcURL}, args: configuration(""), warn: true},
		{name: "ConfigurationSkipsExtraConfigurationImport", args: append(configuration(""), extraConfiguration("rdbms.yaml", `camunda.data.secondary-storage.rdbms.url: `+jdbcURL)...), warn: true},
		{name: "ConfigurationImportsExtraConfiguration", args: append(configuration(`\nspring:\n  config:\n    import: optional:file:/usr/local/camunda/config/rdbms.yaml`), extraConfiguration("rdbms.yaml", `camunda.data.secondary-storage.rdbms.url: `+jdbcURL)...)},
		{name: "ValuesURL", values: map[string]string{"orchestration.data.secondaryStorage.rdbms.url": jdbcURL}},
		{name: "EnvURL", values: map[string]string{"orchestration.env[0].name": "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_URL", "orchestration.env[0].value": jdbcURL}},
		{name: "SpringApplicationJSON", args: []string{"--set-json", `orchestration.env=[{"name":"SPRING_APPLICATION_JSON","value":"{\"camunda\":{\"data\":{\"secondary-storage\":{\"rdbms\":{\"url\":\"` + jdbcURL + `\"}}}}}"}]`}},
		{name: "EnvFrom", values: map[string]string{"orchestration.envFrom[0].secretRef.name": "rdbms-connection"}},
		{name: "JavaOpts", values: map[string]string{"orchestration.javaOpts": "-Dcamunda.data.secondary-storage.rdbms.url=" + jdbcURL}},
		{name: "NestedExtraConfiguration", args: extraConfiguration("rdbms.yaml", `camunda:\n  data:\n    secondary-storage:\n      rdbms:\n        url: `+jdbcURL)},
		{name: "PropertiesExtraConfiguration", args: extraConfiguration("rdbms.properties", `camunda.data.secondary-storage.rdbms.url=`+jdbcURL)},
		{name: "Configuration", args: configuration(`\n      rdbms:\n        url: ` + jdbcURL)},
		{name: "ElasticsearchWithExporterFlag", values: map[string]string{"orchestration.data.secondaryStorage.type": "elasticsearch", "orchestration.exporters.rdbms.enabled": "true"}},
		{name: "OrchestrationDisabled", values: map[string]string{"orchestration.enabled": "false"}},
	} {
		values := map[string]string{"orchestration.data.secondaryStorage.type": "rdbms"}
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
				s.Require().NotEmpty(configMaps)
				warnings := configMaps[s.release+"-warnings"].Data["warnings"]
				s.Require().Equal(scenario.warn, strings.Contains(warnings, rdbmsWarning))
				if scenario.warn {
					s.Require().Contains(warnings, "orchestration.data.secondaryStorage.rdbms.url")
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
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "ReplicationFactorFourFromEnvDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.env[0].name":                "CAMUNDA_CLUSTER_REPLICATIONFACTOR",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.env[0].value=4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "LegacyEnvFourStillWarnsOnValuesKey",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.env[0].name":                "ZEEBE_BROKER_CLUSTER_REPLICATIONFACTOR",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.env[0].value=4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "orchestration.replicationFactor is 3 but")
			},
		},
		{
			Name: "ReplicationFactorFromEnvValueFromDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":         "elasticsearch",
				"global.multiregion.regions":                       "2",
				"orchestration.env[0].name":                        "CAMUNDA_CLUSTER_REPLICATIONFACTOR",
				"orchestration.env[0].valueFrom.secretKeyRef.name": "cluster",
				"orchestration.env[0].valueFrom.secretKeyRef.key":  "rf",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "ReplicationFactorFourInValuesOverriddenByEnvTriggersWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.env[0].name":                "CAMUNDA_CLUSTER_REPLICATIONFACTOR",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.env[0].value=3", "--set-string", "orchestration.replicationFactor=4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "the CAMUNDA_CLUSTER_REPLICATIONFACTOR entry in orchestration.env is 3 but")
				s.Require().Contains(configmap.Data["warnings"], "Set the CAMUNDA_CLUSTER_REPLICATIONFACTOR entry in orchestration.env to 4.")
			},
		},
		{
			Name: "CustomConfigurationDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.configuration=camunda: {}"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "TemplatedEnvNameDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.env=[{"name":"{{ \"CAMUNDA_CLUSTER_REPLICATIONFACTOR\" }}","value":"4"}]`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "ReplicationFactorFromExtraConfigurationDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.extraConfiguration[0].content=camunda.cluster.replication-factor: 4"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "SnakeCaseEnvReplicationFactorDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.env=[{"name":"CAMUNDA_CLUSTER_REPLICATION_FACTOR","value":"4"}]`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "JavaOptsReplicationFactorDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.javaOpts="-Dcamunda.cluster.replication-factor=4"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "LegacyEnvLosesToNewKeyEnv",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.env=[{"name":"CAMUNDA_CLUSTER_REPLICATIONFACTOR","value":"4"},{"name":"ZEEBE_BROKER_CLUSTER_REPLICATIONFACTOR","value":"3"}]`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "LegacyEnvDoesNotOverrideValuesKey",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.replicationFactor=4", "--set-json", `orchestration.env=[{"name":"ZEEBE_BROKER_CLUSTER_REPLICATIONFACTOR","value":"3"}]`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "NestedExtraConfigurationReplicationFactorDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.extraConfiguration[0].content="camunda:\n  cluster:\n    replication-factor: 4\n"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "SecondDocumentExtraConfigurationDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.extraConfiguration[0].content="foo: bar\n---\ncamunda:\n  cluster:\n    replication-factor: 4\n"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "MixedCamelCaseExtraConfigurationDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.extraConfiguration[0].content="camunda.cluster:\n  replicationFactor: 4\n"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "FlatCamelCaseExtraConfigurationDoesNotTriggerWarning",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.extraConfiguration[0].content="camunda.cluster.replicationFactor: 4\n"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().NotContains(output, "a dual-region cluster needs a replication factor of 4")
			},
		},
		{
			Name: "UnrelatedExtraConfigurationReplicationFactorStillWarns",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.extraConfiguration[0].content="foo:\n  replicationFactor: 4\n"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "orchestration.replicationFactor is 3 but")
			},
		},
		{
			Name: "ScalarCamundaExtraConfigurationStillWarns",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.extraConfiguration[0].content="camunda: x\n"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "orchestration.replicationFactor is 3 but")
			},
		},
		{
			Name: "ScalarCamundaClusterExtraConfigurationStillWarns",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.multiregion.regions":               "2",
				"orchestration.extraConfiguration[0].file": "cluster.yaml",
			},
			RenderTemplateExtraArgs: []string{"--set-json", `orchestration.extraConfiguration[0].content="camunda:\n  cluster: x\ncamunda.cluster: y\n"`},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "orchestration.replicationFactor is 3 but")
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

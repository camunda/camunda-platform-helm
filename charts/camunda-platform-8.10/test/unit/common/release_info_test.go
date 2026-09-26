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

// ReleaseInfoTest verifies the release-info ConfigMap rendered by
// camundaPlatform.releaseInfo.
type ReleaseInfoTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

type releaseInfoComponent struct {
	ID   string            `json:"id"`
	URL  string            `json:"url"`
	URLs map[string]string `json:"urls"`
}

func releaseInfoComponents(t *testing.T, output string) map[string]releaseInfoComponent {
	t.Helper()
	var configMap corev1.ConfigMap
	helm.UnmarshalK8SYaml(t, output, &configMap)
	require.Contains(t, configMap.Data, "info")
	var releases []struct {
		Components []releaseInfoComponent `json:"components"`
	}
	helm.UnmarshalK8SYaml(t, configMap.Data["info"], &releases)
	require.Len(t, releases, 1)
	components := make(map[string]releaseInfoComponent)
	for _, component := range releases[0].Components {
		components[component.ID] = component
	}
	return components
}

func TestReleaseInfo(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ReleaseInfoTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

// TestKeycloakComponentEntry verifies the Keycloak entry is emitted only when a
// Keycloak URL is resolvable. After the Bitnami subchart removal, identity is
// always enabled independently of a bundled Keycloak, so a non-Keycloak (external
// OIDC) deployment must not emit a Keycloak component with an empty url.
func (s *ReleaseInfoTest) TestKeycloakComponentEntry() {
	testCases := []testhelpers.TestCase{
		{
			Name: "KeycloakEntryPresentWhenKeycloakUrlSet",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.example.com",
				"global.identity.keycloak.url.port":     "443",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "id: keycloak",
					"Keycloak component entry should be present when a Keycloak URL is configured")
			},
		},
		{
			Name: "KeycloakEntryAbsentForExternalOidc",
			Values: map[string]string{
				"identity.enabled":          "true",
				"global.identity.auth.type": "MICROSOFT",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "id: keycloak",
					"Keycloak component entry should be absent when no Keycloak URL is configured (external OIDC)")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ReleaseInfoTest) TestIngressExternalURLs() {
	var testCases []testhelpers.TestCase
	for _, input := range []struct {
		name      string
		tls       string
		httpPort  string
		httpsPort string
		wantURL   string
	}{
		{"DefaultHTTP", "false", "80", "443", "http://camunda.example.com/identity"},
		{"CustomHTTP", "false", "8080", "8443", "http://camunda.example.com:8080/identity"},
		{"DefaultHTTPS", "true", "80", "443", "https://camunda.example.com/identity"},
		{"CustomHTTPS", "true", "8080", "8443", "https://camunda.example.com:8443/identity"},
		{"HTTPOn443", "false", "443", "443", "http://camunda.example.com:443/identity"},
		{"HTTPSOn80", "true", "80", "80", "https://camunda.example.com:80/identity"},
		{"MinimumPort", "false", "1", "443", "http://camunda.example.com:1/identity"},
		{"MaximumPort", "true", "80", "65535", "https://camunda.example.com:65535/identity"},
	} {
		testCases = append(testCases, testhelpers.TestCase{
			Name: input.name,
			Values: map[string]string{
				"global.host":                              "camunda.example.com",
				"global.ingress.enabled":                   "true",
				"global.ingress.tls.enabled":               input.tls,
				"global.ingress.publicPorts.http":          input.httpPort,
				"global.ingress.publicPorts.https":         input.httpsPort,
				"identity.enabled":                         "true",
				"identity.contextPath":                     "/identity",
				"global.identity.keycloak.internal":        "true",
				"optimize.enabled":                         "true",
				"optimize.contextPath":                     "/optimize",
				"orchestration.contextPath":                "/orchestration",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.ingress.grpc.enabled":       "true",
				"orchestration.ingress.grpc.host":          "grpc.{{ .Values.global.host }}",
				"orchestration.ingress.grpc.tls.enabled":   input.tls,
				"camundaHub.enabled":                       "true",
				"camundaHub.contextPath":                   "/modeler",
				"camundaHub.restapi.mail.fromAddress":      "test@example.com",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				components := releaseInfoComponents(t, output)
				baseURL := strings.TrimSuffix(input.wantURL, "/identity")
				for component, contextPath := range map[string]string{
					"identity": "/identity",
					"keycloak": "/auth/",
					"optimize": "/optimize",
					"hub":      "/modeler",
					"operate":  "/orchestration/operate",
					"tasklist": "/orchestration/tasklist",
					"admin":    "/orchestration/admin",
				} {
					require.Equal(t, baseURL+contextPath, components[component].URL, component)
				}
				require.Equal(t, baseURL+"/orchestration", components["orchestration"].URLs["http"])
				require.Equal(t, strings.Replace(baseURL, "://", "://grpc.", 1), components["orchestration"].URLs["grpc"])
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ReleaseInfoTest) TestIngressPortIsolation() {
	var testCases []testhelpers.TestCase
	for _, input := range []struct {
		name        string
		values      map[string]string
		identityURL string
		grpcURL     string
	}{
		{
			name:        "DisabledIngressUsesLocalURLs",
			identityURL: "http://localhost:8080",
			grpcURL:     "http://localhost:26500",
		},
		{
			name: "GRPCWithoutHTTPIngressUsesItsTLSSetting",
			values: map[string]string{
				"orchestration.ingress.grpc.enabled":     "true",
				"orchestration.ingress.grpc.host":        "grpc.example.com",
				"orchestration.ingress.grpc.tls.enabled": "true",
			},
			identityURL: "http://localhost:8080",
			grpcURL:     "https://grpc.example.com:8443",
		},
		{
			name: "PlaintextGRPCIgnoresHTTPIngressTLS",
			values: map[string]string{
				"global.ingress.enabled":             "true",
				"global.ingress.tls.enabled":         "true",
				"global.host":                        "camunda.example.com",
				"orchestration.ingress.grpc.enabled": "true",
				"orchestration.ingress.grpc.host":    "grpc.example.com",
			},
			identityURL: "https://camunda.example.com:8443/identity",
			grpcURL:     "http://grpc.example.com:8080",
		},
		{
			name: "ExternalIngressWithTemplatedHost",
			values: map[string]string{
				"global.ingress.enabled":  "true",
				"global.ingress.external": "true",
				"global.host":             "edge-{{ .Release.Name }}.example.com",
			},
			identityURL: "http://edge-camunda-platform-test.example.com:8080/identity",
			grpcURL:     "http://localhost:26500",
		},
		{
			name: "ExplicitIdentityURLTakesPrecedence",
			values: map[string]string{
				"global.ingress.enabled": "true",
				"global.host":            "camunda.example.com",
				"identity.fullURL":       "https://id-{{ .Release.Name }}.example.com:9443/custom",
			},
			identityURL: "https://id-camunda-platform-test.example.com:9443/custom",
			grpcURL:     "http://localhost:26500",
		},
		{
			name: "GRPCWithoutHostRetainsLocalURL",
			values: map[string]string{
				"orchestration.ingress.grpc.enabled": "true",
			},
			identityURL: "http://localhost:8080",
			grpcURL:     "http://localhost:26500",
		},
	} {
		values := map[string]string{
			"identity.enabled":                       "true",
			"identity.contextPath":                   "/identity",
			"global.ingress.enabled":                 "false",
			"global.gateway.enabled":                 "false",
			"global.ingress.publicPorts.http":        "8080",
			"global.ingress.publicPorts.https":       "8443",
			"global.ingress.tls.enabled":             "false",
			"orchestration.ingress.grpc.enabled":     "false",
			"orchestration.ingress.grpc.host":        "",
			"orchestration.ingress.grpc.tls.enabled": "false",
		}
		for key, value := range input.values {
			values[key] = value
		}
		testCases = append(testCases, testhelpers.TestCase{
			Name:     input.name,
			Values:   values,
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				components := releaseInfoComponents(t, output)
				require.Equal(t, input.identityURL, components["identity"].URL)
				require.Equal(t, input.grpcURL, components["orchestration"].URLs["grpc"])
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ReleaseInfoTest) TestClearedIngressPortsUseDefaults() {
	var testCases []testhelpers.TestCase
	for _, field := range []string{"publicPorts", "publicPorts.http", "publicPorts.https"} {
		for _, protocol := range []string{"http", "https"} {
			tlsEnabled := "false"
			if protocol == "https" {
				tlsEnabled = "true"
			}
			testCases = append(testCases, testhelpers.TestCase{
				Name: field + "/" + protocol,
				Values: map[string]string{
					"global.ingress.enabled":     "true",
					"global.ingress.tls.enabled": tlsEnabled,
					"global.host":                "camunda.example.com",
					"identity.enabled":           "true",
					"identity.contextPath":       "/identity",
				},
				Template:                "templates/common/configmap-release.yaml",
				RenderTemplateExtraArgs: []string{"--set-json", "global.ingress." + field + "=null"},
				Verifier: func(t *testing.T, output string, err error) {
					require.NoError(t, err)
					components := releaseInfoComponents(t, output)
					require.Equal(t, protocol+"://camunda.example.com/identity", components["identity"].URL)
				},
			})
		}
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

func (s *ReleaseInfoTest) TestInvalidIngressPorts() {
	var testCases []testhelpers.TestCase
	for _, protocol := range []string{"http", "https"} {
		for _, value := range []string{"0", "-1", "65536", "1.5", "true", `"8080"`} {
			testCases = append(testCases, testhelpers.TestCase{
				Name:                    protocol + "=" + value,
				Template:                "templates/common/configmap-release.yaml",
				RenderTemplateExtraArgs: []string{"--set-json", "global.ingress.publicPorts." + protocol + "=" + value},
				Verifier: func(t *testing.T, output string, err error) {
					require.Error(t, err)
					require.Contains(t, err.Error(), "schema")
					require.Contains(t, err.Error(), "publicPorts/"+protocol)
				},
			})
		}
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

// TestConnectorsURLScheme verifies the in-cluster Connectors URLs follow the
// EFFECTIVE TLS state, including a connectors.env SERVER_SSL_ENABLED override
// that wins over global.tls.connectors.enabled.
func (s *ReleaseInfoTest) TestConnectorsURLScheme() {
	testCases := []testhelpers.TestCase{
		{
			Name: "ConnectorsUrlsHttpsWhenTlsEnabled",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "https://camunda-platform-test-connectors.")
				require.NotContains(t, output, "http://camunda-platform-test-connectors.")
			},
		},
		{
			Name: "ConnectorsUrlsHttpWhenEnvOverrideDisablesTls",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
				"connectors.env[0].name":                           "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "connectors.env[0].value=false",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "http://camunda-platform-test-connectors.")
				require.NotContains(t, output, "https://camunda-platform-test-connectors.")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

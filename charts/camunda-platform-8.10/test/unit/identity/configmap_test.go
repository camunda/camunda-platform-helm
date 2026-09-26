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

package identity

import (
	"camunda-platform/test/unit/testhelpers"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type configMapSpringTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestSpringConfigMapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &configMapSpringTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/identity/configmap.yaml"},
	})
}

func (s *configMapSpringTemplateTest) TestIngressPublicURL() {
	var testCases []testhelpers.TestCase
	for _, input := range []struct {
		name    string
		ingress string
		tls     string
		fullURL string
		wantURL string
	}{
		{"HTTP", "true", "false", "", "http://camunda.example.com:8080/identity"},
		{"HTTPS", "true", "true", "", "https://camunda.example.com:8443/identity"},
		{"ExplicitURL", "true", "true", "https://id-{{ .Release.Name }}.example.com:9443/custom", "https://id-camunda-platform-test.example.com:9443/custom"},
		{"DisabledIngress", "false", "false", "", "http://localhost:8084"},
	} {
		testCases = append(testCases, testhelpers.TestCase{
			Name: input.name,
			Values: map[string]string{
				"identity.enabled":                 "true",
				"identity.contextPath":             "/identity",
				"identity.fullURL":                 input.fullURL,
				"global.host":                      "camunda.example.com",
				"global.ingress.enabled":           input.ingress,
				"global.ingress.tls.enabled":       input.tls,
				"global.ingress.publicPorts.http":  "8080",
				"global.ingress.publicPorts.https": "8443",
				"global.gateway.enabled":           "false",
				"global.gateway.tls.enabled":       "true",
				"global.identity.auth.enabled":     "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)
				var application IdentityConfigYAML
				require.NoError(t, yaml.Unmarshal([]byte(configMap.Data["application.yaml"]), &application))
				require.Equal(t, input.wantURL, application.Identity.Url)
				var callbackConfig struct {
					Keycloak struct {
						Environment struct {
							Clients []struct {
								RootURL      string   `yaml:"root-url"`
								RedirectURIs []string `yaml:"redirect-uris"`
							} `yaml:"clients"`
						} `yaml:"environment"`
					} `yaml:"keycloak"`
				}
				require.NoError(t, yaml.Unmarshal([]byte(configMap.Data["application.yaml"]), &callbackConfig))
				require.Len(t, callbackConfig.Keycloak.Environment.Clients, 1)
				require.Equal(t, input.wantURL, callbackConfig.Keycloak.Environment.Clients[0].RootURL)
				require.Equal(t, []string{"/auth/login-callback"}, callbackConfig.Keycloak.Environment.Clients[0].RedirectURIs)
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *configMapSpringTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:                 "TestContainerShouldAddContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":     "true",
				"identity.fullURL":     "https://mydomain.com/identity",
				"identity.contextPath": "/identity",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(t, output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.Require().Equal("https://mydomain.com/identity", configmapApplication.Identity.Url)
				s.Require().Equal("/identity", configmapApplication.Server.Servlet.ContextPath)
			},
		}, {
			Name: "TestConfigMapGlobalMultitenancySetsIdentityFlag",
			Values: map[string]string{
				"global.multitenancy.enabled":        "true",
				"identity.externalDatabase.enabled":  "true",
				"identity.externalDatabase.host":     "my-database-host",
				"identity.externalDatabase.username": "my-database-username",
				"identity.enabled":                   "true",
				"global.identity.auth.enabled":       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				s.Require().Equal("true", configmapApplication.Identity.Flags.MultiTenancy)
			},
		}, {
			Name: "TestConfigMapExternalDatabaseEnabled",
			Values: map[string]string{
				"identity.enabled":                   "true",
				"global.identity.auth.enabled":       "true",
				"identity.multitenancy.enabled":      "true",
				"identity.externalDatabase.enabled":  "true",
				"identity.externalDatabase.host":     "my-database-host",
				"identity.externalDatabase.port":     "2345",
				"identity.externalDatabase.database": "my-database-name",
				"identity.externalDatabase.username": "my-database-username",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(t, output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				s.Require().Equal("true", configmapApplication.Identity.Flags.MultiTenancy)
				s.Require().Equal("jdbc:postgresql://my-database-host:2345/my-database-name", configmapApplication.Spring.DataSource.Url)
				s.Require().Equal("my-database-username", configmapApplication.Spring.DataSource.Username)
			},
		}, {
			Name: "TestConfigMapAuthIssuerBackendUrlWhenExplicitlyDefined",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "false",
				"global.identity.auth.issuerBackendUrl": "https://example.com/",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(t, output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				s.Require().Equal("https://example.com/", configmapApplication.Identity.AuthProvider.BackendUrl)
			},
		}, {
			Name: "TestConfigMapAuthIssuerBackendUrlIsTemplated",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "false",
				"global.identity.auth.type":             "generic",
				"global.identity.auth.issuerBackendUrl": "https://{{ .Release.Name }}.example.com/",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(t, output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				s.Require().Equal("https://camunda-platform-test.example.com/", configmapApplication.Identity.AuthProvider.BackendUrl)
			},
		}, {
			Name: "TestConfigMapAuthIssuerBackendUrlWhenKeycloakUrlDefined",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.com",
				"global.identity.keycloak.url.port":     "443",
				"global.identity.keycloak.contextPath":  "/auth/",
				"global.identity.keycloak.realm":        "camunda-platform",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(t, output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				s.Require().Equal("https://keycloak.com:443/auth/camunda-platform", configmapApplication.Identity.AuthProvider.BackendUrl)
			},
		}, {
			Name: "TestConfigMapAuthIssuerBackendUrlNoDoubleSlashWhenContextPathIsRoot",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.example.com",
				"global.identity.keycloak.url.port":     "443",
				"global.identity.keycloak.contextPath":  "/",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(t, output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				s.Require().Equal("https://keycloak.example.com:443/realms/camunda-platform", configmapApplication.Identity.AuthProvider.BackendUrl)
			},
		}, {
			Name: "TestConfigMapAuthIssuerBackendUrlWithTemplatedKeycloakHost",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.{{ .Release.Namespace }}.svc.cluster.local",
				"global.identity.keycloak.url.port":     "443",
				"global.identity.keycloak.contextPath":  "/auth/",
				"global.identity.keycloak.realm":        "camunda-platform",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				helm.UnmarshalK8SYaml(t, output, &configmap)

				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				// Verify the full BackendUrl including the rendered namespace
				expectedBackendURL := "https://keycloak." + s.namespace + ".svc.cluster.local:443/auth/camunda-platform"
				s.Require().Equal(expectedBackendURL, configmapApplication.Identity.AuthProvider.BackendUrl)
			},
		}, {
			Name:                 "TestKeycloakAdminUserCustom",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                                       "true",
				"global.identity.auth.enabled":                           "true",
				"global.identity.keycloak.url.protocol":                  "https",
				"global.identity.keycloak.url.host":                      "keycloak.example.com",
				"global.identity.keycloak.url.port":                      "8443",
				"global.identity.keycloak.auth.adminUser":                "customAdmin",
				"global.identity.keycloak.auth.secret.existingSecret":    "some-secret",
				"global.identity.keycloak.auth.secret.existingSecretKey": "admin-password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				s.Require().Contains(applicationYaml, "user: \"customAdmin\"")
			},
		},
		// Hybrid Auth Tests - verify OIDC client config is only included for components using OIDC auth
		{
			Name:                 "TestBasicAuthExcludesOidcConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "basic",
				"connectors.enabled":                    "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				// then - verify neither connectors nor orchestration OIDC config is present when using global basic auth
				s.Require().NotContains(applicationYaml, "VALUES_KEYCLOAK_INIT_CONNECTORS_SECRET",
					"Connectors OIDC secret should not be present when using basic auth")
				s.Require().NotContains(applicationYaml, "VALUES_KEYCLOAK_INIT_ORCHESTRATION_SECRET",
					"Orchestration OIDC secret should not be present when using basic auth")
			},
		}, {
			Name:                 "TestGlobalOidcAuthIncludesBothOidcConfigs",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"connectors.enabled":                    "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				// then - verify both connectors and orchestration OIDC config IS present when using global OIDC auth
				s.Require().Contains(applicationYaml, "VALUES_KEYCLOAK_INIT_CONNECTORS_SECRET",
					"Connectors OIDC secret should be present when using OIDC auth")
				s.Require().Contains(applicationYaml, "VALUES_KEYCLOAK_INIT_ORCHESTRATION_SECRET",
					"Orchestration OIDC secret should be present when using OIDC auth")
			},
		}, {
			Name:                 "TestHybridAuthConnectorsBasicOrchestrationOidc",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                          "true",
				"global.identity.auth.enabled":              "true",
				"global.security.authentication.method":     "oidc",
				"connectors.security.authentication.method": "basic",
				"connectors.enabled":                        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				// then - verify only orchestration OIDC config is present, not connectors
				s.Require().NotContains(applicationYaml, "VALUES_KEYCLOAK_INIT_CONNECTORS_SECRET",
					"Connectors OIDC secret should not be present when connectors use basic auth")
				s.Require().Contains(applicationYaml, "VALUES_KEYCLOAK_INIT_ORCHESTRATION_SECRET",
					"Orchestration OIDC secret should be present when orchestration uses OIDC auth")
			},
		}, {
			// Test that firstUser gets Orchestration role only when orchestration uses OIDC
			Name:                 "TestFirstUserRolesExcludeOrchestrationWhenBasicAuth",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                             "true",
				"identity.firstUser.enabled":                   "true",
				"global.identity.auth.enabled":                 "true",
				"orchestration.security.authentication.method": "basic",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				// then - firstUser should have ManagementIdentity but NOT Orchestration role
				s.Require().Contains(applicationYaml, "- ManagementIdentity",
					"FirstUser should have ManagementIdentity role")
				// The Orchestration role should NOT appear in the keycloak.init.users section
				// We check that no user has Orchestration in their roles list
				s.Require().NotContains(applicationYaml, "- Orchestration",
					"FirstUser should NOT have Orchestration role when orchestration uses basic auth")
			},
		}, {
			// Test that firstUser gets Orchestration role when orchestration uses OIDC
			Name:                 "TestFirstUserRolesIncludeOrchestrationWhenOidcAuth",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                             "true",
				"identity.firstUser.enabled":                   "true",
				"global.identity.auth.enabled":                 "true",
				"orchestration.security.authentication.method": "oidc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				// then - firstUser should have both ManagementIdentity and Orchestration roles
				s.Require().Contains(applicationYaml, "- ManagementIdentity",
					"FirstUser should have ManagementIdentity role")
				s.Require().Contains(applicationYaml, "- Orchestration",
					"FirstUser should have Orchestration role when orchestration uses OIDC auth")
			},
		}, {
			Name:                 "TestConnectorsDisabledExcludesOidcConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"connectors.enabled":                    "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().NotContains(applicationYaml, "connectors:",
					"Connectors OIDC config should not be present when connectors is disabled")
			},
		}, {
			Name:                 "TestOrchestrationDisabledExcludesOidcConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"orchestration.enabled":                 "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().NotContains(applicationYaml, "orchestration:",
					"Orchestration OIDC config should not be present when orchestration is disabled")
			},
		}, {
			Name:                 "TestBothDisabledExcludesOidcConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"connectors.enabled":                    "false",
				"orchestration.enabled":                 "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().NotContains(applicationYaml, "connectors:",
					"Connectors OIDC config should not be present when connectors is disabled")
				s.Require().NotContains(applicationYaml, "orchestration:",
					"Orchestration OIDC config should not be present when orchestration is disabled")
			},
		}, {
			// Test: Optimize redirect-uris include both the callback path and the root path. See camunda/camunda#59963.
			Name:                 "TestOptimizeRedirectUrisIncludesRoot",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"optimize.enabled":                      "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().Regexp(`redirect-uris:\s*\n\s*-\s*"/api/authentication/callback"\s*\n\s*-\s*"/"\s*\n`, applicationYaml,
					"Optimize redirect-uris should include both the callback path and the root path")
			},
		}, {
			// Test: Optimize disabled should NOT include optimize config in identity configmap
			Name:                 "TestOptimizeDisabledExcludesOptimizeConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"optimize.enabled":                      "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				// Optimize config should NOT be present when optimize is disabled
				s.Require().NotContains(applicationYaml, "VALUES_KEYCLOAK_INIT_OPTIMIZE_SECRET",
					"Optimize config should not be present when optimize.enabled=false")
				s.Require().NotContains(applicationYaml, "CAMUNDA_OPTIMIZE_SECRET",
					"Optimize secret should not be present when optimize.enabled=false")
				s.Require().NotContains(applicationYaml, "optimize-api",
					"Optimize API should not be present when optimize.enabled=false")
			},
		}, {
			// Test: alwaysRegister=true forces the Optimize preset (applications + apis +
			// roles) to render even though optimize is disabled — multi-namespace deployments
			// where a central Identity registers audiences for components running elsewhere.
			Name:                 "TestOptimizeDisabledWithRegisterInIdentityIncludesOptimizeConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                             "true",
				"global.identity.auth.enabled":                 "true",
				"global.security.authentication.method":        "oidc",
				"optimize.enabled":                             "false",
				"global.identity.auth.optimize.alwaysRegister": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().Contains(applicationYaml, "optimize-api",
					"Optimize API should be present when alwaysRegister=true, even though optimize.enabled=false")
				s.Require().Contains(applicationYaml, "name: Optimize API",
					"Optimize apis preset should render when alwaysRegister=true")
				// keycloak.init is the selector that makes Identity actually CREATE the
				// component's Keycloak client + resource server — without this entry,
				// findResourceServerByAudience("optimize-api") 404s even though the
				// component-presets apis block above rendered.
				s.Require().Contains(applicationYaml, "VALUES_KEYCLOAK_INIT_OPTIMIZE_SECRET",
					"keycloak.init.optimize should render when alwaysRegister=true, even though optimize.enabled=false")
				s.Require().Contains(applicationYaml, "- Optimize",
					"first-user Optimize role should render when alwaysRegister=true, even though optimize.enabled=false")
			},
		}, {
			// Backward compat: alwaysRegister=false (the default) + optimize disabled must
			// behave exactly as before this feature — no Optimize preset rendered.
			Name:                 "TestOptimizeDisabledWithRegisterInIdentityDefaultExcludesOptimizeConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"optimize.enabled":                      "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().NotContains(applicationYaml, "optimize-api",
					"Optimize API should not be present when alwaysRegister defaults to false and optimize.enabled=false")
				s.Require().NotContains(applicationYaml, "VALUES_KEYCLOAK_INIT_OPTIMIZE_SECRET",
					"keycloak.init.optimize should not render when alwaysRegister defaults to false and optimize.enabled=false")
				s.Require().NotContains(applicationYaml, "- Optimize",
					"first-user Optimize role should not render when alwaysRegister defaults to false and optimize.enabled=false")
			},
		}, {
			// Test: alwaysRegister=true forces the Connectors preset (application) to
			// render even though connectors is disabled — multi-namespace deployments where
			// a central Identity registers audiences for components running elsewhere.
			Name:                 "TestConnectorsDisabledWithRegisterInIdentityIncludesConnectorsConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                               "true",
				"global.identity.auth.enabled":                   "true",
				"global.security.authentication.method":          "oidc",
				"connectors.enabled":                             "false",
				"global.identity.auth.connectors.alwaysRegister": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().Contains(applicationYaml, "name: Connectors",
					"Connectors component-preset should render when alwaysRegister=true, even though connectors.enabled=false")
				// keycloak.init is the selector that makes Identity actually CREATE the
				// component's Keycloak client + resource server — without this entry,
				// the Connectors client is never provisioned even though the
				// component-presets application block above rendered.
				s.Require().Contains(applicationYaml, "VALUES_KEYCLOAK_INIT_CONNECTORS_SECRET",
					"keycloak.init.connectors should render when alwaysRegister=true, even though connectors.enabled=false")
			},
		}, {
			// Backward compat: alwaysRegister=false (the default) + connectors disabled must
			// behave exactly as before this feature — no Connectors preset rendered.
			Name:                 "TestConnectorsDisabledWithRegisterInIdentityDefaultExcludesConnectorsConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"connectors.enabled":                    "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().NotContains(applicationYaml, "name: Connectors",
					"Connectors component-preset should not render when alwaysRegister defaults to false and connectors.enabled=false")
				s.Require().NotContains(applicationYaml, "VALUES_KEYCLOAK_INIT_CONNECTORS_SECRET",
					"keycloak.init.connectors should not render when alwaysRegister defaults to false and connectors.enabled=false")
			},
		}, {
			// Test: alwaysRegister=true forces the Orchestration preset (applications + apis
			// + roles) to render even though orchestration is disabled.
			Name:                 "TestOrchestrationDisabledWithRegisterInIdentityIncludesOrchestrationConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                                  "true",
				"global.identity.auth.enabled":                      "true",
				"global.security.authentication.method":             "oidc",
				"orchestration.enabled":                             "false",
				"global.identity.auth.orchestration.alwaysRegister": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().Contains(applicationYaml, "orchestration-api",
					"Orchestration API should be present when alwaysRegister=true, even though orchestration.enabled=false")
				s.Require().Contains(applicationYaml, "name: \"Orchestration API\"",
					"Orchestration apis preset should render when alwaysRegister=true")
				// keycloak.init is the selector that makes Identity actually CREATE the
				// component's Keycloak client + resource server — without this entry,
				// findResourceServerByAudience("orchestration-api") 404s even though the
				// component-presets apis block above rendered.
				s.Require().Contains(applicationYaml, "VALUES_KEYCLOAK_INIT_ORCHESTRATION_SECRET",
					"keycloak.init.orchestration should render when alwaysRegister=true, even though orchestration.enabled=false")
				s.Require().Contains(applicationYaml, "- Orchestration",
					"first-user Orchestration role should render when alwaysRegister=true, even though orchestration.enabled=false")
			},
		}, {
			// Backward compat: alwaysRegister=false (the default) + orchestration disabled
			// must behave exactly as before this feature — no Orchestration preset rendered.
			Name:                 "TestOrchestrationDisabledWithRegisterInIdentityDefaultExcludesOrchestrationConfig",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"orchestration.enabled":                 "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().NotContains(applicationYaml, "orchestration:",
					"Orchestration config should not be present when alwaysRegister defaults to false and orchestration.enabled=false")
				s.Require().NotContains(applicationYaml, "VALUES_KEYCLOAK_INIT_ORCHESTRATION_SECRET",
					"keycloak.init.orchestration should not render when alwaysRegister defaults to false and orchestration.enabled=false")
				s.Require().NotContains(applicationYaml, "- Orchestration",
					"first-user Orchestration role should not render when alwaysRegister defaults to false and orchestration.enabled=false")
			},
		}, {
			// Test: the admin-permission block's resourceServerId entries must track custom
			// orchestration/optimize audiences (rather than the hardcoded orchestration-api /
			// optimize-api defaults) so admin permissions still resolve against the actual
			// resource server when a deployment overrides the audience.
			Name:                 "TestAdminPermissionsTrackCustomAudiences",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                                    "true",
				"global.identity.auth.enabled":                        "true",
				"global.identity.auth.admin.enabled":                  "true",
				"global.identity.auth.admin.clientId":                 "custom-admin",
				"global.security.authentication.method":               "oidc",
				"orchestration.security.authentication.oidc.audience": "custom-orchestration-api",
				"global.identity.auth.optimize.audience":              "custom-optimize-api",
				"global.identity.auth.optimize.alwaysRegister":        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]

				s.Require().Contains(applicationYaml, "resourceServerId: \"custom-orchestration-api\"",
					"admin permissions should reference the custom orchestration audience")
				s.Require().Contains(applicationYaml, "resourceServerId: \"custom-optimize-api\"",
					"admin permissions should reference the custom optimize audience")
				s.Require().NotContains(applicationYaml, "resourceServerId: orchestration-api",
					"admin permissions should not reference the literal default orchestration-api once a custom audience is set")
				s.Require().NotContains(applicationYaml, "resourceServerId: optimize-api",
					"admin permissions should not reference the literal default optimize-api once a custom audience is set")
			},
		}, {
			Name: "TestClusterPingRoleRendersForEnabledHubRegardlessOfHubPing",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.auth.enabled":          "true",
				"global.security.authentication.method": "oidc",
				"camundaHub.enabled":                    "true",
				"webModeler.restapi.mail.fromAddress":   "test@example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				s.Require().Contains(applicationYaml, "Hub API - Cluster Ping",
					"enabled Hub must include the cluster-ping role")
				s.Require().NotContains(applicationYaml, "Hub API - Cluster Ping Access",
					"the cluster-ping mapping rule itself should stay opt-in via "+
						"orchestration.hub.ping / hubPingAuthorizationEnabled")
			},
		}, {
			Name: "TestHubPingAddsOrchestrationPermissionsAndMappingRule",
			Values: map[string]string{
				"identity.enabled":                                               "true",
				"global.identity.auth.enabled":                                   "true",
				"global.security.authentication.method":                          "oidc",
				"orchestration.hub.ping.endpoint":                                "https://hub.example/api/v1/clusters",
				"orchestration.security.authentication.method":                   "oidc",
				"orchestration.security.authentication.oidc.secret.inlineSecret": "secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				s.Require().Contains(applicationYaml, "Hub API - Cluster Ping")
				s.Require().Contains(applicationYaml, "audience: \"web-modeler-public-api\"")
				s.Require().Contains(applicationYaml, "claim-value: \"service-account-${CAMUNDA_ORCHESTRATION_CLIENT_ID:${VALUES_KEYCLOAK_INIT_ORCHESTRATION_CLIENT_ID:orchestration}}\"")
			},
		}, {
			Name: "TestHubPingMapsCustomClient",
			Values: map[string]string{
				"identity.enabled":                                                    "true",
				"global.identity.auth.enabled":                                        "true",
				"global.security.authentication.method":                               "oidc",
				"orchestration.hub.ping.endpoint":                                     "https://hub.example/api/v1/clusters",
				"orchestration.hub.ping.credentials.clientId":                         "ping-client",
				"orchestration.hub.ping.credentials.clientSecret.secret.inlineSecret": "ping-secret",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.secret.inlineSecret":      "secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				s.Require().Contains(applicationYaml, "claim-value: \"service-account-ping-client\"")
				s.Require().NotContains(applicationYaml, "audience: \"web-modeler-public-api\"\n                  definition: create:*")
			},
		}, {
			Name: "TestHubPingAuthorizationRendersForCentralIdentity",
			Values: map[string]string{
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
				"global.identity.auth.orchestration.hubPingAuthorizationEnabled": "true",
				"orchestration.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				s.Require().Contains(applicationYaml, "Hub API - Cluster Ping")
				s.Require().Contains(applicationYaml, "claim-value: \"service-account-${CAMUNDA_ORCHESTRATION_CLIENT_ID:${VALUES_KEYCLOAK_INIT_ORCHESTRATION_CLIENT_ID:orchestration}}\"")
			},
		}, {
			Name: "TestHubPingRendersConfiguredMappingForExternalOidc",
			Values: map[string]string{
				"identity.enabled":                                               "true",
				"global.identity.auth.enabled":                                   "true",
				"orchestration.hub.ping.endpoint":                                "https://hub.example/api/v1/clusters",
				"orchestration.security.authentication.method":                   "oidc",
				"orchestration.security.authentication.oidc.type":                "MICROSOFT",
				"orchestration.security.authentication.oidc.secret.inlineSecret": "secret",
				"orchestration.security.authentication.oidc.tokenUrl":            "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
				"global.identity.auth.orchestration.hubPingClaimName":            "azp",
				"global.identity.auth.orchestration.hubPingClaimValue":           "orchestration-client",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				s.Require().Contains(applicationYaml, "Hub API - Cluster Ping")
				s.Require().Contains(applicationYaml, "Hub API - Cluster Ping Access")
				s.Require().Contains(applicationYaml, "claim-name: \"azp\"")
				s.Require().Contains(applicationYaml, "claim-value: \"orchestration-client\"")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *configMapSpringTemplateTest) TestComponentEnablementSurfaces() {
	type registrationCase struct {
		name               string
		values             map[string]string
		hubRegistered      bool
		optimizeRegistered bool
		topology           bool
	}
	scenarios := []registrationCase{
		{
			name:               "OptimizeAlwaysRegister",
			values:             map[string]string{"global.identity.auth.optimize.alwaysRegister": "true"},
			optimizeRegistered: true,
		},
		{
			name:          "RemoteHubPingEndpoint",
			values:        map[string]string{"orchestration.hub.ping.endpoint": "https://hub.example.com/api/v1/clusters"},
			hubRegistered: true,
		},
		{
			name: "RemoteHubPingAuthorization",
			values: map[string]string{
				"global.identity.auth.orchestration.hubPingAuthorizationEnabled": "true",
				"global.identity.auth.orchestration.hubPingClaimName":            "azp",
				"global.identity.auth.orchestration.hubPingClaimValue":           "remote-orchestration",
			},
			hubRegistered: true,
		},
		{
			name:     "HubTopologySuppressesLocalOptimize",
			values:   map[string]string{"optimize.enabled": "true"},
			topology: true,
		},
		{
			name:               "HubTopologyOptimizeAlwaysRegister",
			values:             map[string]string{"global.identity.auth.optimize.alwaysRegister": "true"},
			optimizeRegistered: true,
			topology:           true,
		},
	}
	for mask := 0; mask < 8; mask++ {
		hubEnabled := mask&1 != 0
		webModelerEnabled := mask&2 != 0
		optimizeEnabled := mask&4 != 0
		scenarios = append(scenarios, registrationCase{
			name: fmt.Sprintf("Hub=%t_WebModeler=%t_Optimize=%t", hubEnabled, webModelerEnabled, optimizeEnabled),
			values: map[string]string{
				"camundaHub.enabled": strconv.FormatBool(hubEnabled),
				"webModeler.enabled": strconv.FormatBool(webModelerEnabled),
				"optimize.enabled":   strconv.FormatBool(optimizeEnabled),
			},
			hubRegistered:      hubEnabled || webModelerEnabled,
			optimizeRegistered: optimizeEnabled,
		})
	}

	testCases := []testhelpers.TestCase{}
	for _, authType := range []string{"keycloak", "generic"} {
		for _, scenario := range scenarios {
			values := map[string]string{
				"identity.enabled":                         "true",
				"global.identity.auth.enabled":             "true",
				"global.identity.auth.admin.enabled":       "true",
				"global.identity.auth.admin.clientId":      "registration-admin",
				"global.identity.auth.type":                authType,
				"global.identity.auth.issuerBackendUrl":    "https://issuer.example.com",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.enabled":                    "false",
				"connectors.enabled":                       "false",
				"optimize.enabled":                         "false",
				"camundaHub.enabled":                       "false",
				"webModeler.enabled":                       "false",
				"webModeler.restapi.mail.fromAddress":      "test@example.com",
			}
			for key, value := range scenario.values {
				values[key] = value
			}
			var valuesFiles []string
			if scenario.topology {
				valuesFiles = []string{filepath.Join(s.chartPath, "test/unit/topology/testdata/hub-physical-tenants.yaml")}
			}
			testCases = append(testCases, testhelpers.TestCase{
				Name:        scenario.name + "_" + authType,
				Values:      values,
				ValuesFiles: valuesFiles,
				Verifier: func(t *testing.T, output string, err error) {
					s.Require().NoError(err)
					var configmap corev1.ConfigMap
					helm.UnmarshalK8SYaml(t, output, &configmap)
					var config IdentityConfigYAML
					s.Require().NoError(yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &config))
					s.Require().NotEmpty(config.Identity.ComponentPresets["identity"].Apis)

					granted := map[string]bool{}
					if authType == "keycloak" {
						s.Require().NotNil(config.Keycloak)
						s.Require().Len(config.Keycloak.Clients, 1)
						s.Require().Equal("registration-admin", config.Keycloak.Clients[0].Id)
						for _, permission := range config.Keycloak.Clients[0].Permissions {
							granted[permission.ResourceServerId] = true
						}
						s.Require().True(granted["camunda-identity-resource-server"])
					} else {
						s.Require().Nil(config.Keycloak)
					}

					for _, component := range []struct {
						presetKey  string
						api        string
						registered bool
					}{
						{"webmodeler", "web-modeler-api", scenario.hubRegistered},
						{"optimize", "optimize-api", scenario.optimizeRegistered},
					} {
						preset, present := config.Identity.ComponentPresets[component.presetKey]
						s.Require().True(present, "%s preset must override Identity's defaults", component.presetKey)
						for field, entries := range map[string][]map[string]any{
							"applications": preset.Applications,
							"apis":         preset.Apis,
							"roles":        preset.Roles,
						} {
							s.Require().NotNil(entries, "%s.%s must be an explicit list", component.presetKey, field)
							if component.registered {
								s.Require().NotEmpty(entries, "%s.%s", component.presetKey, field)
							} else {
								s.Require().Empty(entries, "%s.%s", component.presetKey, field)
							}
						}
						if config.Keycloak != nil {
							_, initialized := config.Keycloak.Init[component.presetKey]
							s.Require().Equal(component.registered, initialized, "keycloak.init.%s", component.presetKey)
							s.Require().Equal(component.registered, granted[component.api], "%s admin grant", component.api)
						}
					}
					if scenario.topology {
						var audiences []any
						for _, api := range config.Identity.ComponentPresets["topology-east"].Apis {
							audiences = append(audiences, api["audience"])
						}
						s.Require().Contains(audiences, "optimize-east-api")
						s.Require().Contains(audiences, "optimize-east-ta-api")
						s.Require().Contains(audiences, "optimize-east-tb-api")
						if config.Keycloak != nil {
							s.Require().Contains(config.Keycloak.Init, "topology-east")
						}
					}
				},
			})
		}
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *configMapSpringTemplateTest) TestExtraConfigurationSpringImport() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestExtraConfigWithSpringImportDefault",
			Values: map[string]string{
				"identity.enabled":                       "true",
				"identity.extraConfiguration[0].file":    "custom-spring.yaml",
				"identity.extraConfiguration[0].content": "some: config",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// spring.config.import should include the file
				s.Require().Contains(applicationYaml, "optional:file:/app/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				// File content should be in ConfigMap
				s.Require().Contains(configmap.Data["custom-spring.yaml"], "some: config",
					"File content should be present in ConfigMap")
			},
		},
		{
			Name: "TestExtraConfigWithSpringImportFalse",
			Values: map[string]string{
				"identity.enabled":                            "true",
				"identity.extraConfiguration[0].file":         "log4j2-spring.xml",
				"identity.extraConfiguration[0].springImport": "false",
				"identity.extraConfiguration[0].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// spring.config.import should NOT include the file
				s.Require().NotContains(applicationYaml, "log4j2-spring.xml",
					"File with springImport: false should not be in spring.config.import")
				// spring.config.import block should not be rendered
				s.Require().NotContains(applicationYaml, "optional:file:",
					"spring.config.import block should not be rendered when all entries have springImport: false")
				// File content should still be in ConfigMap
				s.Require().Contains(configmap.Data["log4j2-spring.xml"], "<Configuration/>",
					"File content should be present in ConfigMap even with springImport: false")
			},
		},
		{
			Name: "TestExtraConfigMixedSpringImport",
			Values: map[string]string{
				"identity.enabled":                            "true",
				"identity.extraConfiguration[0].file":         "custom-spring.yaml",
				"identity.extraConfiguration[0].content":      "some: config",
				"identity.extraConfiguration[1].file":         "log4j2-spring.xml",
				"identity.extraConfiguration[1].springImport": "false",
				"identity.extraConfiguration[1].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// Only custom-spring.yaml should be in spring.config.import
				s.Require().Contains(applicationYaml, "optional:file:/app/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				s.Require().NotContains(applicationYaml, "log4j2-spring.xml",
					"File with springImport: false should not be in spring.config.import")
				// Both files should be in ConfigMap
				s.Require().Contains(configmap.Data["custom-spring.yaml"], "some: config",
					"First file content should be present in ConfigMap")
				s.Require().Contains(configmap.Data["log4j2-spring.xml"], "<Configuration/>",
					"Second file content should be present in ConfigMap even with springImport: false")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

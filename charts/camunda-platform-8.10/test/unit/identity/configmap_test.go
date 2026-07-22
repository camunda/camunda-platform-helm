package identity

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
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
		},
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

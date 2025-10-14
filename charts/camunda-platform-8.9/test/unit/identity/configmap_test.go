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
			Skip:                 true,
			Name:                 "TestContainerShouldAddContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
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
			Skip: true,
			Name: "TestConfigMapBuiltinDatabaseEnabled",
			Values: map[string]string{
				"identity.multitenancy.enabled": "true",
				"identityPostgresql.enabled":  "true",
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
				s.Require().Equal("jdbc:postgresql://camunda-platform-test-identity-postgresql:5432/identity", configmapApplication.Spring.DataSource.Url)
				s.Require().Equal("identity", configmapApplication.Spring.DataSource.Username)
			},
		}, {
			Name: "TestConfigMapGlobalMultitenancySetsIdentityFlag",
			Values: map[string]string{
				"global.multitenancy.enabled":   "true",
				"identityPostgresql.enabled":    "true",
				"identity.enabled":              "true",
				"global.identity.auth.enabled":  "true",
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
				"identity.enabled":                     "true",
				"global.identity.auth.enabled":        "true",
				"identity.multitenancy.enabled":       "true",
				"identityPostgresql.enabled":          "false",
				"identity.externalDatabase.enabled":   "true",
				"identity.externalDatabase.host":      "my-database-host",
				"identity.externalDatabase.port":      "2345",
				"identity.externalDatabase.database":  "my-database-name",
				"identity.externalDatabase.username":  "my-database-username",
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
			Skip: true,
			Name: "TestConfigMapAuthIssuerBackendUrlWhenExplicitlyDefined",
			Values: map[string]string{
				"identityKeycloak.enabled":              "false",
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
			Skip: true,
			Name: "TestConfigMapAuthIssuerBackendUrlWhenKeycloakUrlDefined",
			Values: map[string]string{
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
			Skip:   true,
			Name:   "TestConfigMapAuthIssuerBackendUrlWhenKeycloakNotDefined",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication IdentityConfigYAML
				e := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
				if e != nil {
					s.Fail("Failed to unmarshal yaml. error=", e)
				}

				// then
				s.NotEmpty(configmap.Data)

				s.Require().Equal("http://camunda-platform-test-keycloak:80/auth/realms/camunda-platform", configmapApplication.Identity.AuthProvider.BackendUrl)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

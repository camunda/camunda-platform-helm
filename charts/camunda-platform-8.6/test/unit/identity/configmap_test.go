package identity

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
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

func (s *configMapSpringTemplateTest) TestContainerShouldAddContextPath() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"identity.fullURL":     "https://mydomain.com/identity",
			"identity.contextPath": "/identity",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication IdentityConfigYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("https://mydomain.com/identity", configmapApplication.Identity.Url)
	s.Require().Equal("/identity", configmapApplication.Server.Servlet.ContextPath)
}

func (s *configMapSpringTemplateTest) TestConfigMapBuiltinDatabaseEnabled() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.multitenancy.enabled": "true",
			"identityPostgresql.enabled":  "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication IdentityConfigYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.NotEmpty(configmap.Data)

	s.Require().Equal("true", configmapApplication.Identity.Flags.MultiTenancy)
	s.Require().Equal("jdbc:postgresql://camunda-platform-test-identity-postgresql:5432/identity", configmapApplication.Spring.DataSource.Url)
	s.Require().Equal("identity", configmapApplication.Spring.DataSource.Username)
}

func (s *configMapSpringTemplateTest) TestConfigMapExternalDatabaseEnabled() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.multitenancy.enabled":        "true",
			"identityPostgresql.enabled":         "false",
			"identity.externalDatabase.enabled":  "true",
			"identity.externalDatabase.host":     "my-database-host",
			"identity.externalDatabase.port":     "2345",
			"identity.externalDatabase.database": "my-database-name",
			"identity.externalDatabase.username": "my-database-username",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication IdentityConfigYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.NotEmpty(configmap.Data)

	s.Require().Equal("true", configmapApplication.Identity.Flags.MultiTenancy)
	s.Require().Equal("jdbc:postgresql://my-database-host:2345/my-database-name", configmapApplication.Spring.DataSource.Url)
	s.Require().Equal("my-database-username", configmapApplication.Spring.DataSource.Username)
}
func (s *configMapSpringTemplateTest) TestConfigMapAuthIssuerBackendUrlWhenExplicitlyDefined() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"identityKeycloak.enabled":              "false",
			"global.identity.auth.enabled":          "false",
			"global.identity.auth.issuerBackendUrl": "https://example.com/",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication IdentityConfigYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.NotEmpty(configmap.Data)

	s.Require().Equal("https://example.com/", configmapApplication.Identity.AuthProvider.BackendUrl)
}
func (s *configMapSpringTemplateTest) TestConfigMapAuthIssuerBackendUrlWhenKeycloakUrlDefined() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.keycloak.url.protocol": "https",
			"global.identity.keycloak.url.host":     "keycloak.com",
			"global.identity.keycloak.url.port":     "443",
			"global.identity.keycloak.contextPath":  "/auth/",
			"global.identity.keycloak.realm":        "camunda-platform",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication IdentityConfigYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.NotEmpty(configmap.Data)

	s.Require().Equal("https://keycloak.com:443/auth/camunda-platform", configmapApplication.Identity.AuthProvider.BackendUrl)
}
func (s *configMapSpringTemplateTest) TestConfigMapAuthIssuerBackendUrlWhenKeycloakNotDefined() {
	// given
	options := &helm.Options{
		SetValues:      map[string]string{},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication IdentityConfigYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.NotEmpty(configmap.Data)

	s.Require().Equal("http://camunda-platform-test-keycloak:80/auth/realms/camunda-platform", configmapApplication.Identity.AuthProvider.BackendUrl)
}

func (s *configMapSpringTemplateTest) TestConfigMapClientIdWhenClientSecretSet() {
	// given
	options := &helm.Options{
		SetValues:      map[string]string{
			"global.identity.auth.type": "GENERIC",
			"global.identity.auth.issuerBackendUrl": "https://my.idp.org",
			"global.identity.auth.issuer": "https://my.idp.org",
			"global.identity.auth.identity.audience": "identity-client",
			"global.identity.auth.identity.clientId": "identity-client",
			"global.identity.auth.identity.existingSecret": "superSecret",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication IdentityConfigYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.NotEmpty(configmap.Data)
	s.Require().Equal("identity-client", configmapApplication.Camunda.Identity.ClientId)
}

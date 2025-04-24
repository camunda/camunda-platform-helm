package web_modeler

import (
	"maps"
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

type configmapRestAPITemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

var requiredValues = map[string]string{
	"webModeler.enabled":                                  "true",
	"webModeler.restapi.mail.fromAddress":                 "example@example.com",
	"global.identity.auth.connectors.existingSecret.name": "foo",
	"global.identity.auth.core.existingSecret.name":       "foo",
}

func TestRestAPIConfigmapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &configmapRestAPITemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/web-modeler/configmap-restapi.yaml"},
	})
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetCorrectAuthClientApiAudience() {
	// given
	values := map[string]string{
		"global.identity.auth.webModeler.clientApiAudience": "custom-audience",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("custom-audience", configmapApplication.Camunda.Modeler.Security.JWT.Audience.InternalAPI)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetCorrectAuthPublicApiAudience() {
	// given
	values := map[string]string{
		"global.identity.auth.webModeler.publicApiAudience": "custom-audience",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("custom-audience", configmapApplication.Camunda.Modeler.Security.JWT.Audience.PublicAPI)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetCorrectIdentityServiceUrlWithFullnameOverride() {
	// given
	values := map[string]string{
		"identity.fullnameOverride": "custom-identity-fullname",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://custom-identity-fullname:80", configmapApplication.Camunda.Identity.BaseURL)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetCorrectIdentityServiceUrlWithNameOverride() {
	// given
	values := map[string]string{
		"identity.nameOverride": "custom-identity",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://camunda-platform-test-custom-identity:80", configmapApplication.Camunda.Identity.BaseURL)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetCorrectIdentityType() {
	// given
	values := map[string]string{
		"global.identity.auth.type":                    "MICROSOFT",
		"global.identity.auth.issuerBackendUrl":        "https://example.com",
		"global.identity.auth.identity.existingSecret": "foo",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("MICROSOFT", configmapApplication.Camunda.Identity.Type)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetCorrectKeycloakServiceUrl() {
	// given
	values := map[string]string{
		"global.identity.keycloak.url.protocol": "http",
		"global.identity.keycloak.url.host":     "keycloak",
		"global.identity.keycloak.url.port":     "80",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://keycloak:80/auth/realms/camunda-platform", configmapApplication.Camunda.Modeler.Security.JWT.Issuer.BackendUrl)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetCorrectKeycloakServiceUrlWithCustomPort() {
	// given
	values := map[string]string{
		"global.identity.keycloak.url.protocol": "http",
		"global.identity.keycloak.url.host":     "keycloak",
		"global.identity.keycloak.url.port":     "8888",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://keycloak:8888/auth/realms/camunda-platform", configmapApplication.Camunda.Modeler.Security.JWT.Issuer.BackendUrl)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetSmtpCredentials() {
	// given
	values := map[string]string{
		"webModeler.restapi.mail.smtpUser":     "modeler-user",
		"webModeler.restapi.mail.smtpPassword": "modeler-password",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("modeler-user", configmapApplication.Spring.Mail.Username)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetExternalDatabaseConfiguration() {
	// given
	values := map[string]string{
		"webModelerPostgresql.enabled":                 "false",
		"webModeler.restapi.externalDatabase.url":      "jdbc:postgresql://postgres.example.com:65432/modeler-database",
		"webModeler.restapi.externalDatabase.user":     "modeler-user",
		"webModeler.restapi.externalDatabase.password": "modeler-password",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("jdbc:postgresql://postgres.example.com:65432/modeler-database", configmapApplication.Spring.Datasource.Url)
	s.Require().Equal("modeler-user", configmapApplication.Spring.Datasource.Username)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldConfigureClusterFromSameHelmInstallationWithCustomValues() {
	// given
	testCases := []struct {
		name                   string
		authEnabled            string
		authMethod             string
		expectedAuthentication string
	}{
		{"OIDC Authentication", "true", "oidc", "BEARER_TOKEN"},
		{"Basic Authentication", "true", "basic", "BASIC"},
		{"No Authentication", "false", "basic", "NONE"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			values := map[string]string{
				"webModelerPostgresql.enabled":          "false",
				"global.zeebeClusterName":               "test-zeebe",
				"global.identity.auth.enabled":          tc.authEnabled,
				"global.security.authentication.method": tc.authMethod,
				"core.image.tag":                        "8.x.x-alpha1",
				"core.contextPath":                      "/core",
				"core.service.grpcPort":                 "26600",
				"core.service.httpPort":                 "8090",
			}
			maps.Insert(values, maps.All(requiredValues))
			options := &helm.Options{
				SetValues:      values,
				KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
			}

			// when
			output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
			var configmap corev1.ConfigMap
			var configmapApplication WebModelerRestAPIApplicationYAML
			helm.UnmarshalK8SYaml(s.T(), output, &configmap)

			err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
			if err != nil {
				s.Fail("Failed to unmarshal yaml. error=", err)
			}

			// then
			s.Require().Equal(1, len(configmapApplication.Camunda.Modeler.Clusters))
			s.Require().Equal("default-cluster", configmapApplication.Camunda.Modeler.Clusters[0].Id)
			s.Require().Equal("test-zeebe", configmapApplication.Camunda.Modeler.Clusters[0].Name)
			s.Require().Equal("8.x.x-alpha1", configmapApplication.Camunda.Modeler.Clusters[0].Version)
			s.Require().Equal(tc.expectedAuthentication, configmapApplication.Camunda.Modeler.Clusters[0].Authentication)
			s.Require().Equal("grpc://camunda-platform-test-core:26600", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Grpc)
			s.Require().Equal("http://camunda-platform-test-core:8090/core/v1", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Rest)
		})
	}
}

func (s *configmapRestAPITemplateTest) TestContainerShouldUseClustersFromCustomConfiguration() {
	// given
	values := map[string]string{
		"webModeler.restapi.clusters[0].id":             "test-cluster-1",
		"webModeler.restapi.clusters[0].name":           "test cluster 1",
		"webModeler.restapi.clusters[0].version":        "8.6.0",
		"webModeler.restapi.clusters[0].authentication": "NONE",
		"webModeler.restapi.clusters[0].url.zeebe.grpc": "grpc://core.test-1:26500",
		"webModeler.restapi.clusters[0].url.zeebe.rest": "http://core.test-1:8080",
		"webModeler.restapi.clusters[0].url.operate":    "http://operate.test-1:8080",
		"webModeler.restapi.clusters[0].url.tasklist":   "http://tasklist.test-1:8080",
		"webModeler.restapi.clusters[1].id":             "test-cluster-2",
		"webModeler.restapi.clusters[1].name":           "test cluster 2",
		"webModeler.restapi.clusters[1].version":        "8.x.x-alpha1",
		"webModeler.restapi.clusters[1].authentication": "BEARER_TOKEN",
		"webModeler.restapi.clusters[1].url.zeebe.grpc": "grpc://core.test-2:26500",
		"webModeler.restapi.clusters[1].url.zeebe.rest": "http://core.test-2:8080",
		"webModeler.restapi.clusters[1].url.operate":    "http://operate.test-2:8080",
		"webModeler.restapi.clusters[1].url.tasklist":   "http://tasklist.test-2:8080",
		"webModeler.restapi.clusters[2].id":             "test-cluster-3",
		"webModeler.restapi.clusters[2].name":           "test cluster 3",
		"webModeler.restapi.clusters[2].version":        "8.x.x-alpha1",
		"webModeler.restapi.clusters[2].authentication": "BASIC",
		"webModeler.restapi.clusters[2].url.zeebe.grpc": "grpc://core.test-3:26500",
		"webModeler.restapi.clusters[2].url.zeebe.rest": "http://core.test-3:8080",
		"webModeler.restapi.clusters[2].url.operate":    "http://operate.test-3:8080",
		"webModeler.restapi.clusters[2].url.tasklist":   "http://tasklist.test-3:8080",
		"webModelerPostgresql.enabled":                  "false",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal(3, len(configmapApplication.Camunda.Modeler.Clusters))
	s.Require().Equal("test-cluster-1", configmapApplication.Camunda.Modeler.Clusters[0].Id)
	s.Require().Equal("test cluster 1", configmapApplication.Camunda.Modeler.Clusters[0].Name)
	s.Require().Equal("8.6.0", configmapApplication.Camunda.Modeler.Clusters[0].Version)
	s.Require().Equal("NONE", configmapApplication.Camunda.Modeler.Clusters[0].Authentication)
	s.Require().Equal("grpc://core.test-1:26500", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Grpc)
	s.Require().Equal("http://core.test-1:8080", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Rest)
	s.Require().Equal("test-cluster-2", configmapApplication.Camunda.Modeler.Clusters[1].Id)
	s.Require().Equal("test cluster 2", configmapApplication.Camunda.Modeler.Clusters[1].Name)
	s.Require().Equal("8.x.x-alpha1", configmapApplication.Camunda.Modeler.Clusters[1].Version)
	s.Require().Equal("BEARER_TOKEN", configmapApplication.Camunda.Modeler.Clusters[1].Authentication)
	s.Require().Equal("grpc://core.test-2:26500", configmapApplication.Camunda.Modeler.Clusters[1].Url.Zeebe.Grpc)
	s.Require().Equal("http://core.test-2:8080", configmapApplication.Camunda.Modeler.Clusters[1].Url.Zeebe.Rest)
	s.Require().Equal("test-cluster-3", configmapApplication.Camunda.Modeler.Clusters[2].Id)
	s.Require().Equal("test cluster 3", configmapApplication.Camunda.Modeler.Clusters[2].Name)
	s.Require().Equal("8.x.x-alpha1", configmapApplication.Camunda.Modeler.Clusters[2].Version)
	s.Require().Equal("BASIC", configmapApplication.Camunda.Modeler.Clusters[2].Authentication)
	s.Require().Equal("grpc://core.test-3:26500", configmapApplication.Camunda.Modeler.Clusters[2].Url.Zeebe.Grpc)
	s.Require().Equal("http://core.test-3:8080", configmapApplication.Camunda.Modeler.Clusters[2].Url.Zeebe.Rest)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldNotConfigureClustersIfZeebeDisabledAndNoCustomConfiguration() {
	// given
	values := map[string]string{
		"webModelerPostgresql.enabled": "false",
		"core.enabled":                 "false",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Empty(configmapApplication.Camunda.Modeler.Clusters)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetJwkSetUriFromJwksUrlProperty() {
	// given
	values := map[string]string{
		"global.identity.auth.jwksUrl": "https://example.com/auth/realms/test/protocol/openid-connect/certs",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("https://example.com/auth/realms/test/protocol/openid-connect/certs", configmapApplication.Spring.Security.OAuth2.ResourceServer.JWT.JwkSetURI)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetJwkSetUriFromIssuerBackendUrlProperty() {
	// given
	values := map[string]string{
		"global.identity.auth.issuerBackendUrl": "http://test-keycloak/auth/realms/test",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://test-keycloak/auth/realms/test/protocol/openid-connect/certs", configmapApplication.Spring.Security.OAuth2.ResourceServer.JWT.JwkSetURI)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldSetJwkSetUriFromKeycloakUrlProperties() {
	// given
	values := map[string]string{
		"global.identity.keycloak.url.protocol": "https",
		"global.identity.keycloak.url.host":     "example.com",
		"global.identity.keycloak.url.port":     "443",
		"global.identity.keycloak.contextPath":  "/",
		"global.identity.keycloak.realm":        "test",
	}
	maps.Insert(values, maps.All(requiredValues))
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerRestAPIApplicationYAML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("https://example.com:443/test/protocol/openid-connect/certs", configmapApplication.Spring.Security.OAuth2.ResourceServer.JWT.JwkSetURI)
}

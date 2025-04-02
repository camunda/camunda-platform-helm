package web_modeler

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

type configmapRestAPITemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                "true",
			"webModeler.restapi.mail.fromAddress":               "example@example.com",
			"global.identity.auth.webModeler.clientApiAudience": "custom-audience",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                "true",
			"webModeler.restapi.mail.fromAddress":               "example@example.com",
			"global.identity.auth.webModeler.publicApiAudience": "custom-audience",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"identity.fullnameOverride":           "custom-identity-fullname",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"identity.nameOverride":               "custom-identity",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                    "true",
			"webModeler.restapi.mail.fromAddress":   "example@example.com",
			"global.identity.auth.type":             "MICROSOFT",
			"global.identity.auth.issuerBackendUrl": "https://example.com",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                    "true",
			"webModeler.restapi.mail.fromAddress":   "example@example.com",
			"global.identity.keycloak.url.protocol": "http",
			"global.identity.keycloak.url.host":     "keycloak",
			"global.identity.keycloak.url.port":     "80",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                    "true",
			"webModeler.restapi.mail.fromAddress":   "example@example.com",
			"global.identity.keycloak.url.protocol": "http",
			"global.identity.keycloak.url.host":     "keycloak",
			"global.identity.keycloak.url.port":     "8888",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                   "true",
			"webModeler.restapi.mail.fromAddress":  "example@example.com",
			"webModeler.restapi.mail.smtpUser":     "modeler-user",
			"webModeler.restapi.mail.smtpPassword": "modeler-password",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                           "true",
			"webModeler.restapi.mail.fromAddress":          "example@example.com",
			"postgresql.enabled":                           "false",
			"webModeler.restapi.externalDatabase.url":      "jdbc:postgresql://postgres.example.com:65432/modeler-database",
			"webModeler.restapi.externalDatabase.user":     "modeler-user",
			"webModeler.restapi.externalDatabase.password": "modeler-password",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"postgresql.enabled":                  "false",
			"global.zeebeClusterName":             "test-zeebe",
			"zeebe.image.tag":                     "8.7.0-alpha1",
			"zeebeGateway.contextPath":            "/zeebe",
			"zeebeGateway.service.grpcPort":       "26600",
			"zeebeGateway.service.restPort":       "8090",
			"operate.contextPath":                 "/operate",
			"operate.service.port":                "8080",
			"tasklist.contextPath":                "/tasklist",
			"tasklist.service.port":               "8080",
		},
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
	s.Require().Equal("8.7.0-alpha1", configmapApplication.Camunda.Modeler.Clusters[0].Version)
	s.Require().Equal("BEARER_TOKEN", configmapApplication.Camunda.Modeler.Clusters[0].Authentication)
	s.Require().Equal("grpc://test-zeebe-gateway:26600", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Grpc)
	s.Require().Equal("http://test-zeebe-gateway:8090/zeebe", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Rest)
	s.Require().Equal("http://camunda-platform-test-operate:8080/operate", configmapApplication.Camunda.Modeler.Clusters[0].Url.Operate)
	s.Require().Equal("http://camunda-platform-test-tasklist:8080/tasklist", configmapApplication.Camunda.Modeler.Clusters[0].Url.Tasklist)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldConfigureClusterFromSameHelmInstallationForAuthTypeMicrosoft() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                     "true",
			"webModeler.restapi.mail.fromAddress":    "example@example.com",
			"postgresql.enabled":                     "false",
			"identityKeycloak.enabled":               "false",
			"global.identity.auth.type":              "MICROSOFT",
			"global.identity.auth.issuerBackendUrl":  "https://example.com",
			"global.identity.auth.tokenUrl":          "https://example.com/token",
			"global.identity.auth.operate.audience":  "test-audience-operate",
			"global.identity.auth.tasklist.audience": "test-audience-tasklist",
			"global.identity.auth.zeebe.audience":    "test-audience-zeebe",
			"global.identity.auth.zeebe.tokenScope":  "test-scope",
		},
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
	s.Require().Equal("camunda-platform-test-zeebe", configmapApplication.Camunda.Modeler.Clusters[0].Name)
	s.Require().Equal("CLIENT_CREDENTIALS", configmapApplication.Camunda.Modeler.Clusters[0].Authentication)
	s.Require().Equal("grpc://camunda-platform-test-zeebe-gateway:26500", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Grpc)
	s.Require().Equal("http://camunda-platform-test-zeebe-gateway:8080", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Rest)
	s.Require().Equal("http://camunda-platform-test-operate:80", configmapApplication.Camunda.Modeler.Clusters[0].Url.Operate)
	s.Require().Equal("http://camunda-platform-test-tasklist:80", configmapApplication.Camunda.Modeler.Clusters[0].Url.Tasklist)
	s.Require().Equal("https://example.com/token", configmapApplication.Camunda.Modeler.Clusters[0].OAuth.Url)
	s.Require().Equal("test-scope", configmapApplication.Camunda.Modeler.Clusters[0].OAuth.Scope)
	s.Require().Equal("test-audience-zeebe", configmapApplication.Camunda.Modeler.Clusters[0].OAuth.Audience.Zeebe)
	s.Require().Equal("test-audience-operate", configmapApplication.Camunda.Modeler.Clusters[0].OAuth.Audience.Operate)
	s.Require().Equal("test-audience-tasklist", configmapApplication.Camunda.Modeler.Clusters[0].OAuth.Audience.Tasklist)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldUseClustersFromCustomConfiguration() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                            "true",
			"webModeler.restapi.mail.fromAddress":           "example@example.com",
			"webModeler.restapi.clusters[0].id":             "test-cluster-1",
			"webModeler.restapi.clusters[0].name":           "test cluster 1",
			"webModeler.restapi.clusters[0].version":        "8.6.0",
			"webModeler.restapi.clusters[0].authentication": "NONE",
			"webModeler.restapi.clusters[0].url.zeebe.grpc": "grpc://zeebe-gateway.test-1:26500",
			"webModeler.restapi.clusters[0].url.zeebe.rest": "http://zeebe-gateway.test-1:8080",
			"webModeler.restapi.clusters[0].url.operate":    "http://operate.test-1:8080",
			"webModeler.restapi.clusters[0].url.tasklist":   "http://tasklist.test-1:8080",
			"webModeler.restapi.clusters[1].id":             "test-cluster-2",
			"webModeler.restapi.clusters[1].name":           "test cluster 2",
			"webModeler.restapi.clusters[1].version":        "8.7.0-alpha1",
			"webModeler.restapi.clusters[1].authentication": "BEARER_TOKEN",
			"webModeler.restapi.clusters[1].url.zeebe.grpc": "grpc://zeebe-gateway.test-2:26500",
			"webModeler.restapi.clusters[1].url.zeebe.rest": "http://zeebe-gateway.test-2:8080",
			"webModeler.restapi.clusters[1].url.operate":    "http://operate.test-2:8080",
			"webModeler.restapi.clusters[1].url.tasklist":   "http://tasklist.test-2:8080",
			"postgresql.enabled":                            "false",
		},
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
	s.Require().Equal(2, len(configmapApplication.Camunda.Modeler.Clusters))
	s.Require().Equal("test-cluster-1", configmapApplication.Camunda.Modeler.Clusters[0].Id)
	s.Require().Equal("test cluster 1", configmapApplication.Camunda.Modeler.Clusters[0].Name)
	s.Require().Equal("8.6.0", configmapApplication.Camunda.Modeler.Clusters[0].Version)
	s.Require().Equal("NONE", configmapApplication.Camunda.Modeler.Clusters[0].Authentication)
	s.Require().Equal("grpc://zeebe-gateway.test-1:26500", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Grpc)
	s.Require().Equal("http://zeebe-gateway.test-1:8080", configmapApplication.Camunda.Modeler.Clusters[0].Url.Zeebe.Rest)
	s.Require().Equal("http://operate.test-1:8080", configmapApplication.Camunda.Modeler.Clusters[0].Url.Operate)
	s.Require().Equal("http://tasklist.test-1:8080", configmapApplication.Camunda.Modeler.Clusters[0].Url.Tasklist)
	s.Require().Equal("test-cluster-2", configmapApplication.Camunda.Modeler.Clusters[1].Id)
	s.Require().Equal("test cluster 2", configmapApplication.Camunda.Modeler.Clusters[1].Name)
	s.Require().Equal("8.7.0-alpha1", configmapApplication.Camunda.Modeler.Clusters[1].Version)
	s.Require().Equal("BEARER_TOKEN", configmapApplication.Camunda.Modeler.Clusters[1].Authentication)
	s.Require().Equal("grpc://zeebe-gateway.test-2:26500", configmapApplication.Camunda.Modeler.Clusters[1].Url.Zeebe.Grpc)
	s.Require().Equal("http://zeebe-gateway.test-2:8080", configmapApplication.Camunda.Modeler.Clusters[1].Url.Zeebe.Rest)
	s.Require().Equal("http://operate.test-2:8080", configmapApplication.Camunda.Modeler.Clusters[1].Url.Operate)
	s.Require().Equal("http://tasklist.test-2:8080", configmapApplication.Camunda.Modeler.Clusters[1].Url.Tasklist)
}

func (s *configmapRestAPITemplateTest) TestContainerShouldNotConfigureClustersIfZeebeDisabledAndNoCustomConfiguration() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"postgresql.enabled":                  "false",
			"zeebe.enabled":                       "false",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"global.identity.auth.jwksUrl":        "https://example.com/auth/realms/test/protocol/openid-connect/certs",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                    "true",
			"webModeler.restapi.mail.fromAddress":   "example@example.com",
			"global.identity.auth.issuerBackendUrl": "http://test-keycloak/auth/realms/test",
		},
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
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                    "true",
			"webModeler.restapi.mail.fromAddress":   "example@example.com",
			"global.identity.keycloak.url.protocol": "https",
			"global.identity.keycloak.url.host":     "example.com",
			"global.identity.keycloak.url.port":     "443",
			"global.identity.keycloak.contextPath":  "/",
			"global.identity.keycloak.realm":        "test",
		},
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

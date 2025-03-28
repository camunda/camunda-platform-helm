package web_modeler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type configmapWebAppTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestWebAppConfigmapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &configmapWebAppTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/web-modeler/configmap-webapp.yaml"},
	})
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectAuthClientApiAudience() {
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
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}
	// then
	s.Require().Equal("custom-audience", configmapApplication.OAuth2.Token.Audience)
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectAuthClientId() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                       "true",
			"webModeler.restapi.mail.fromAddress":      "example@example.com",
			"global.identity.auth.webModeler.clientId": "custom-clientId",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("custom-clientId", configmapApplication.OAuth2.ClientId)
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectClientPusherConfigurationWithGlobalIngressTlsDisabled() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"webModeler.contextPath":              "/modeler",
			"global.ingress.enabled":              "true",
			"global.ingress.host":                 "c8.example.com",
			"global.ingress.tls.enabled":          "false",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("c8.example.com", configmapApplication.Client.Pusher.Host)
	s.Require().Equal("80", configmapApplication.Client.Pusher.Port)
	s.Require().Equal("/modeler-ws", configmapApplication.Client.Pusher.Path)
	s.Require().Equal("false", configmapApplication.Client.Pusher.ForceTLS)
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectClientPusherConfigurationWithGlobalIngressTlsEnabled() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                  "true",
			"webModeler.restapi.mail.fromAddress": "example@example.com",
			"webModeler.contextPath":              "/modeler",
			"global.ingress.enabled":              "true",
			"global.ingress.host":                 "c8.example.com",
			"global.ingress.tls.enabled":          "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("c8.example.com", configmapApplication.Client.Pusher.Host)
	s.Require().Equal("443", configmapApplication.Client.Pusher.Port)
	s.Require().Equal("/modeler-ws", configmapApplication.Client.Pusher.Path)
	s.Require().Equal("true", configmapApplication.Client.Pusher.ForceTLS)
}

func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectIdentityServiceUrlWithFullnameOverride() {
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
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://custom-identity-fullname:80", configmapApplication.Identity.BaseUrl)
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectIdentityServiceUrlWithNameOverride() {
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
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://camunda-platform-test-custom-identity:80", configmapApplication.Identity.BaseUrl)
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetServerHttpsOnly() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                          "true",
			"webModeler.restapi.mail.fromAddress":         "example@example.com",
			"global.identity.auth.webModeler.redirectUrl": "https://modeler.example.com",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("true", configmapApplication.Server.HttpsOnly)
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectKeycloakServiceUrl() {
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
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("http://keycloak:80/auth/realms/camunda-platform/protocol/openid-connect/certs", configmapApplication.OAuth2.Token.JwksUrl)
}
func (s *configmapWebAppTemplateTest) TestContainerShouldSetCorrectIdentityType() {
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
	var configmapApplication WebModelerWebAppTOML
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	err := toml.Unmarshal([]byte(configmap.Data["application.toml"]), &configmapApplication)
	if err != nil {
		s.Fail("Failed to unmarshal yaml. error=", err)
	}

	// then
	s.Require().Equal("MICROSOFT", configmapApplication.OAuth2.Type)
}

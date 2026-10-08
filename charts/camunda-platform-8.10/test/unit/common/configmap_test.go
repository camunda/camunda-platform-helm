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

type ConfigMapTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigMapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigMapTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/configmap-identity-auth.yaml"},
	})
}

func (s *ConfigMapTemplateTest) TestKeycloakPublicIssuerPorts() {
	cases := []struct {
		name          string
		protocol      string
		port          string
		issuer        string
		publicIssuer  string
		backendIssuer string
		internal      string
		wantPublic    string
	}{
		{name: "HTTPSDefaultPort", protocol: "https", port: "443", wantPublic: "https://keycloak.example.com/auth/realms/camunda-platform"},
		{name: "HTTPDefaultPort", protocol: "http", port: "80", wantPublic: "http://keycloak.example.com/auth/realms/camunda-platform"},
		{name: "HTTPSNondefaultPort", protocol: "https", port: "8443", wantPublic: "https://keycloak.example.com:8443/auth/realms/camunda-platform"},
		{name: "HTTPSOnPort80", protocol: "https", port: "80", wantPublic: "https://keycloak.example.com:80/auth/realms/camunda-platform"},
		{name: "HTTPOnPort443", protocol: "http", port: "443", wantPublic: "http://keycloak.example.com:443/auth/realms/camunda-platform"},
		{name: "ExplicitPublicIssuer", protocol: "https", port: "443", publicIssuer: "https://public.example.com:443/realms/custom", wantPublic: "https://public.example.com:443/realms/custom"},
		{name: "ExplicitIssuerWins", protocol: "https", port: "443", issuer: "https://issuer.example.com:443/realms/custom", publicIssuer: "https://public.example.com/realms/custom", wantPublic: "https://issuer.example.com:443/realms/custom"},
		{name: "BackendOverrideStaysPrivate", protocol: "https", port: "443", backendIssuer: "http://keycloak.svc:8080/realms/private", wantPublic: "https://keycloak.example.com/auth/realms/camunda-platform"},
		{name: "InternalKeycloakSkipsFallback", protocol: "https", port: "443", internal: "true", wantPublic: ""},
	}
	testCases := make([]testhelpers.TestCase, 0, len(cases))
	for _, tc := range cases {
		values := map[string]string{
			"global.identity.auth.enabled":          "true",
			"identity.enabled":                      "true",
			"global.identity.keycloak.url.protocol": tc.protocol,
			"global.identity.keycloak.url.host":     "keycloak.example.com",
			"global.identity.keycloak.url.port":     tc.port,
			"global.identity.auth.issuer":           tc.issuer,
			"global.identity.auth.publicIssuerUrl":  tc.publicIssuer,
			"global.identity.auth.issuerBackendUrl": tc.backendIssuer,
		}
		if tc.internal != "" {
			values["global.identity.keycloak.internal"] = tc.internal
		}
		backendIssuer := tc.backendIssuer
		if backendIssuer == "" {
			backendIssuer = tc.protocol + "://keycloak.example.com:" + tc.port + "/auth/realms/camunda-platform"
		}
		testCases = append(testCases, testhelpers.TestCase{
			Name:   tc.name,
			Values: values,
			Expected: map[string]string{
				"data.CAMUNDA_IDENTITY_ISSUER":             tc.wantPublic,
				"data.CAMUNDA_IDENTITY_ISSUER_BACKEND_URL": backendIssuer,
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestConfigMapIdentityIssuerURL",
			Values: map[string]string{
				"global.identity.auth.enabled":                                        "true",
				"global.identity.auth.issuerBackendUrl":                               "http://keycloak:80/auth/realms/camunda-platform",
				"identity.enabled":                                                    "true",
				"connectors.security.authentication.oidc.secret.existingSecret":       "foo",
				"connectors.security.authentication.oidc.secret.existingSecretKey":    "identity-connectors-client-token",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "bar",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "identity-orchestration-client-token",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				// then
				s.Require().Equal("http://keycloak:80/auth/realms/camunda-platform", configmap.Data["CAMUNDA_IDENTITY_ISSUER_BACKEND_URL"])
			},
		}, {
			Name: "TestConfigMapOmitsIdentityBaseURLWithoutManagementIdentity",
			Values: map[string]string{
				"global.identity.auth.enabled":             "true",
				"global.identity.auth.type":                "GENERIC",
				"global.identity.auth.issuer":              "https://issuer.example.com",
				"global.identity.auth.issuerBackendUrl":    "https://issuer.example.com",
				"global.identity.auth.jwksUrl":             "https://issuer.example.com/certs",
				"global.identity.service.url":              "",
				"identity.enabled":                         "false",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				s.Require().NotContains(configmap.Data, "CAMUNDA_IDENTITY_BASEURL")
				s.Require().Equal("https://issuer.example.com", configmap.Data["CAMUNDA_IDENTITY_ISSUER"])
			},
		}, {
			Name: "TestConfigMapIdentityIssuerURLWithKeycloakURLSyntax",
			Values: map[string]string{
				"global.identity.auth.enabled":                                        "true",
				"global.identity.keycloak.url.protocol":                               "http",
				"global.identity.keycloak.url.host":                                   "keycloak",
				"global.identity.keycloak.url.port":                                   "80",
				"global.identity.keycloak.contextPath":                                "/auth/realms/",
				"global.identity.keycloak.realm":                                      "camunda-platform",
				"global.identity.keycloak.auth.adminUser":                             "admin",
				"global.identity.keycloak.auth.secret.existingSecret":                 "kc-secret",
				"global.identity.keycloak.auth.secret.existingSecretKey":              "password",
				"identity.enabled":                                                    "true",
				"connectors.security.authentication.oidc.secret.existingSecret":       "foo",
				"connectors.security.authentication.oidc.secret.existingSecretKey":    "identity-connectors-client-token",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "bar",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "identity-orchestration-client-token",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				// then
				s.Require().Equal("http://keycloak:80/auth/realms/camunda-platform", configmap.Data["CAMUNDA_IDENTITY_ISSUER_BACKEND_URL"])
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

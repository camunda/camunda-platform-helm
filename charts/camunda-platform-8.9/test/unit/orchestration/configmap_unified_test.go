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

package orchestration

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigmapTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigmapUnifiedTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigmapTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/configmap.yaml"},
	})
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnified() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesBroker",
			Values: map[string]string{
				"orchestration.profiles.broker": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "admin,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesOperate",
			Values: map[string]string{
				"orchestration.profiles.operate": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "admin,broker,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesTasklist",
			Values: map[string]string{
				"orchestration.profiles.tasklist": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "admin,broker,operate,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainContextPath",
			Values: map[string]string{
				"orchestration.contextPath": "/custom",
			},
			Expected: map[string]string{
				"configmapApplication.server.servlet.context-path": "/custom",
				"configmapApplication.management.server.base-path": "/custom",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainSecondaryStorageOpenSearchEnabled",
			Values: map[string]string{
				"global.opensearch.enabled":  "true",
				"global.opensearch.url.host": "opensearch.example.com",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.url": "https://opensearch.example.com:443",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCClientId",
			Values: map[string]string{
				"orchestration.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.client-id": "orchestration",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesWithDeprecatedIdentityProfile",
			Values: map[string]string{
				"orchestration.profiles.identity": "true",
				"orchestration.profiles.admin":    "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldMigrateDeprecatedIdentityProfileWhenAdminNotSet",
			Values: map[string]string{
				"orchestration.profiles.identity": "true",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "admin,broker,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlNoWebAppProfilesWhenNoSecondaryStorageEnabled",
			Values: map[string]string{
				"global.noSecondaryStorage":                    "true",
				"global.elasticsearch.enabled":                 "false",
				"elasticsearch.enabled":                        "false",
				"orchestration.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "admin,broker,consolidated-auth",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedAuthOIDC() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCClientId",
			Values: map[string]string{
				"orchestration.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.client-id":     "orchestration",
				"configmapApplication.camunda.security.authentication.oidc.client-secret": "${VALUES_ORCHESTRATION_CLIENT_SECRET:}",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCWithIssuerAndKeycloakEnabled",
			Values: map[string]string{
				"identity.enabled":                                       "true",
				"identityKeycloak.enabled":                               "true",
				"global.identity.auth.enabled":                           "true",
				"global.identity.auth.publicIssuerUrl":                   "https://public-issuer-url.com/realms/camunda",
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.redirectUrl": "https://redirect.com/orchestration",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.authorization-uri": "https://public-issuer-url.com/realms/camunda/protocol/openid-connect/auth",
				"configmapApplication.camunda.security.authentication.oidc.jwk-set-uri":       "http://camunda-platform-test-keycloak/auth/realms/camunda-platform/protocol/openid-connect/certs",
				"configmapApplication.camunda.security.authentication.oidc.token-uri":         "http://camunda-platform-test-keycloak/auth/realms/camunda-platform/protocol/openid-connect/token",
				"configmapApplication.camunda.security.authentication.oidc.redirect-uri":      "https://redirect.com/orchestration/sso-callback",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCWithIssuerUrlAndKeycloakDisabled",
			Values: map[string]string{
				"identity.enabled":                                       "false",
				"identityKeycloak.enabled":                               "false",
				"global.identity.auth.enabled":                           "false",
				"global.identity.auth.issuer":                            "https://public-issuer-url.com/realms/camunda",
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.redirectUrl": "https://redirect-url.com/orchestration",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.issuer-uri": "https://public-issuer-url.com/realms/camunda",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCWithIssuerUrlUnUsedAndKeycloakDisabled",
			Values: map[string]string{
				"identity.enabled":                                       "false",
				"identityKeycloak.enabled":                               "false",
				"global.identity.auth.enabled":                           "false",
				"global.identity.auth.issuer":                            "",
				"global.identity.auth.authUrl":                           "https://public-issuer-url.com/auth",
				"global.identity.auth.tokenUrl":                          "https://public-issuer-url.com/token",
				"global.identity.auth.jwksUrl":                           "https://public-issuer-url.com/certs",
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.redirectUrl": "https://redirect-url.com/orchestration",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.authorization-uri": "https://public-issuer-url.com/auth",
				"configmapApplication.camunda.security.authentication.oidc.jwk-set-uri":       "https://public-issuer-url.com/certs",
				"configmapApplication.camunda.security.authentication.oidc.token-uri":         "https://public-issuer-url.com/token",
				"configmapApplication.camunda.security.authentication.oidc.redirect-uri":      "https://redirect-url.com/orchestration/sso-callback",
			},
		},
		{
			Name: "TestApplicationYamlShouldRenderTemplatedAuthUrls",
			Values: map[string]string{
				"identity.enabled":                                       "false",
				"identityKeycloak.enabled":                               "false",
				"global.identity.auth.enabled":                           "false",
				"global.identity.auth.issuer":                            "",
				"global.identity.auth.authUrl":                           "https://{{ .Release.Name }}.example.com/auth",
				"global.identity.auth.tokenUrl":                          "https://{{ .Release.Name }}.example.com/token",
				"global.identity.auth.jwksUrl":                           "https://{{ .Release.Name }}.example.com/certs",
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.redirectUrl": "https://redirect-url.com/orchestration",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.authorization-uri": "https://camunda-platform-test.example.com/auth",
				"configmapApplication.camunda.security.authentication.oidc.jwk-set-uri":       "https://camunda-platform-test.example.com/certs",
				"configmapApplication.camunda.security.authentication.oidc.token-uri":         "https://camunda-platform-test.example.com/token",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCWithIssuerUrlUnUsedAndKeycloakExternal",
			Values: map[string]string{
				"identity.enabled":                                       "false",
				"identityKeycloak.enabled":                               "false",
				"global.identity.auth.enabled":                           "false",
				"global.identity.auth.publicIssuerUrl":                   "https://my-keycloak.com:8080/authz/realms/camunda-platform",
				"global.identity.keycloak.contextPath":                   "/authz",
				"global.identity.keycloak.url.protocol":                  "https",
				"global.identity.keycloak.url.host":                      "my-keycloak.com",
				"global.identity.keycloak.url.port":                      "8080",
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.redirectUrl": "https://redirect-url.com/orchestration",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.authorization-uri": "https://my-keycloak.com:8080/authz/realms/camunda-platform/protocol/openid-connect/auth",
				"configmapApplication.camunda.security.authentication.oidc.jwk-set-uri":       "https://my-keycloak.com:8080/authz/realms/camunda-platform/protocol/openid-connect/certs",
				"configmapApplication.camunda.security.authentication.oidc.token-uri":         "https://my-keycloak.com:8080/authz/realms/camunda-platform/protocol/openid-connect/token",
				"configmapApplication.camunda.security.authentication.oidc.redirect-uri":      "https://redirect-url.com/orchestration/sso-callback",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestGroupsClaimConditionalRendering() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldNotContainGroupsClaimWhenDefault",
			Values: map[string]string{
				"orchestration.security.authentication.method": "oidc",
				"orchestration.data.secondaryStorage.type":     "elasticsearch",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "groups-claim")
			},
		},
		{
			Name: "TestApplicationYamlShouldNotContainGroupsClaimWhenExplicitlyEmpty",
			Values: map[string]string{
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.groupsClaim": "",
				"orchestration.data.secondaryStorage.type":               "elasticsearch",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "groups-claim")
			},
		},
		{
			Name: "TestApplicationYamlShouldContainGroupsClaimWhenSet",
			Values: map[string]string{
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.groupsClaim": "custom-groups",
				"orchestration.data.secondaryStorage.type":               "elasticsearch",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.groups-claim": "custom-groups",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestMappingRulesConditionalRendering() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldNotContainMappingRulesWhenDefault",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "mapping-rules")
			},
		},
		{
			Name: "TestApplicationYamlShouldContainMappingRulesWhenSet",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                            "elasticsearch",
				"orchestration.security.initialization.mappingRules[0].mappingRuleID": "demo-user-mapping-rule",
				"orchestration.security.initialization.mappingRules[0].claimName":     "preferred_username",
				"orchestration.security.initialization.mappingRules[0].claimValue":    "demo",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "mapping-rules")
				require.Contains(t, output, "demo-user-mapping-rule")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestUnprotectedApiConditionalRendering() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainAllowUnauthenticatedApiAccessWhenBasicAuthAndUnprotectedApiTrue",
			Values: map[string]string{
				"orchestration.security.authentication.method":         "basic",
				"orchestration.security.authentication.unprotectedApi": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.basic.allow-unauthenticated-api-access": "true",
			},
		},
		{
			Name: "TestApplicationYamlShouldNotContainAllowUnauthenticatedApiAccessWhenBasicAuthAndUnprotectedApiFalse",
			Values: map[string]string{
				"orchestration.security.authentication.method":         "basic",
				"orchestration.security.authentication.unprotectedApi": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "allow-unauthenticated-api-access")
			},
		},
		{
			Name: "TestApplicationYamlShouldNotContainAllowUnauthenticatedApiAccessWhenOidcAuthAndUnprotectedApiTrue",
			Values: map[string]string{
				"orchestration.security.authentication.method":         "oidc",
				"orchestration.security.authentication.unprotectedApi": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "allow-unauthenticated-api-access")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedRDBMS() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainRDBMSBasicConfig",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.rdbms.enabled":             "true",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.url":      "jdbc:postgresql://localhost:5432/camunda",
				"configmapApplication.camunda.data.secondary-storage.rdbms.username": "camunda",
				"configmapApplication.camunda.data.secondary-storage.rdbms.password": "${VALUES_ORCHESTRATION_DATA_SECONDARYSTORAGE_RDBMS_PASSWORD:}",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSPasswordWithExistingSecret",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.rdbms.enabled":                  "true",
				"orchestration.exporters.rdbms.enabled":                              "true",
				"orchestration.data.secondaryStorage.rdbms.url":                      "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                 "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.existingSecret":    "my-secret",
				"orchestration.data.secondaryStorage.rdbms.secret.existingSecretKey": "password",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.password": "${VALUES_ORCHESTRATION_DATA_SECONDARYSTORAGE_RDBMS_PASSWORD:}",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestMultiRegionInitialContactPoints() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainInitialContactPointsForSingleRegion",
			Values: map[string]string{
				"global.multiregion.regions":    "1",
				"orchestration.profiles.broker": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "initial-contact-points")
				require.Contains(t, output, "camunda-platform-test-zeebe-0.${K8S_SERVICE_NAME}:26502")
				require.Contains(t, output, "camunda-platform-test-zeebe-1.${K8S_SERVICE_NAME}:26502")
				require.Contains(t, output, "camunda-platform-test-zeebe-2.${K8S_SERVICE_NAME}:26502")
				require.NotContains(t, output, "Multi-region deployments: initial-contact-points must be provided manually")
			},
		},
		{
			Name: "TestApplicationYamlShouldNotContainInitialContactPointsForMultiRegion",
			Values: map[string]string{
				"global.multiregion.regions":    "2",
				"global.multiregion.regionId":   "0",
				"orchestration.profiles.broker": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "initial-contact-points:")
				require.Contains(t, output, "Multi-region deployments: initial-contact-points must be provided manually")
				require.Contains(t, output, "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS")
				// Ensure no contact points are generated
				require.NotContains(t, output, "camunda-platform-test-zeebe-0.${K8S_SERVICE_NAME}:26502")
			},
		},
		{
			Name: "TestApplicationYamlShouldNotContainInitialContactPointsForThreeRegions",
			Values: map[string]string{
				"global.multiregion.regions":    "3",
				"global.multiregion.regionId":   "1",
				"orchestration.profiles.broker": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "initial-contact-points:")
				require.Contains(t, output, "Multi-region deployments: initial-contact-points must be provided manually")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

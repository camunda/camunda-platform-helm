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

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConstraintTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConstraintTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConstraintTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{},
	})
}

func (s *ConstraintTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestExistingSecretConstraintDisplays",
			Values: map[string]string{
				"identity.enabled":                                            "true",
				"global.identity.auth.enabled":                                "true",
				"global.security.authentication.method":                       "oidc",
				"connectors.security.authentication.oidc.existingSecret.name": "foo",
				"orchestration.security.authentication.oidc.existingSecret":   "",
				"global.identity.auth.issuerBackendUrl":                       "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":        "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().ErrorContains(err, "the Camunda Helm chart will no longer automatically generate passwords for the Identity component")
			},
		}, {
			Name: "TestExistingSecretConstraintDoesNotDisplayErrorForComponentWithExistingSecret",
			Values: map[string]string{
				"identity.enabled":                                               "true",
				"global.identity.auth.enabled":                                   "true",
				"global.security.authentication.method":                          "oidc",
				"orchestration.security.authentication.oidc.existingSecret.name": "bar",
				"global.identity.auth.issuerBackendUrl":                          "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":           "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				requiredComponentsNotSet := strings.Split(err.Error(), "The following values inside your values.yaml need to be set but were not")[1]
				s.Require().NotContains(requiredComponentsNotSet, "orchestration.security.authentication.oidc.existingSecret")
			},
		}, {
			Name: "TestExistingSecretConstraintInWarningModeDoesNotPreventInstall",
			Values: map[string]string{
				"identity.enabled":                                               "true",
				"global.security.authentication.method":                          "oidc",
				"connectors.security.authentication.oidc.existingSecret.name":    "foo",
				"orchestration.security.authentication.oidc.existingSecret.name": "bar",
				"global.identity.auth.issuerBackendUrl":                          "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":           "warning",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestCaBundleAndLegacyJksRenderWithoutCrash() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestGlobalElasticsearchTlsExistingSecretRendersOk",
			Values: map[string]string{
				"global.elasticsearch.tls.existingSecret": "my-legacy-jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestGlobalOpensearchTlsJksInlineSecretRendersOk",
			Values: map[string]string{
				"global.opensearch.tls.jks.secret.inlineSecret": "changeit",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestCaBundleRendersOk",
			Values: map[string]string{
				"global.tls.caBundle.secret.existingSecret": "camunda-ca-bundle",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestCaBundleAndLegacyJksCoexistRenderOk",
			Values: map[string]string{
				"global.elasticsearch.tls.existingSecret":   "my-legacy-jks",
				"global.elasticsearch.url.protocol":         "http",
				"global.tls.caBundle.secret.existingSecret": "camunda-ca-bundle",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestCaBundleConsoleCertKeyFilenameWarningRendersOk() {
	testCases := []testhelpers.TestCase{
		{
			// Exercises the constraints warning that fires when caBundle is set
			// AND console.tls.certKeyFilename is configured (the latter no longer
			// contributes trust). Asserts the warning path renders without crashing.
			Name: "TestCaBundleWithConsoleCertKeyFilenameRendersOk",
			Values: map[string]string{
				"console.enabled":                           "true",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "camunda-ca-bundle",
				"console.tls.certKeyFilename":               "tls.crt",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestManagementIdentityExternalServiceUrl() {
	testCases := []testhelpers.TestCase{
		{
			Name: "WebModelerWithExternalManagementIdentityRenders",
			Values: map[string]string{
				"orchestration.enabled":               "false",
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "test@example.com",
				"identity.enabled":                    "false",
				"global.identity.service.url":         "http://identity.other-ns.svc:8080",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "WebModelerWithoutManagementIdentityFails",
			Values: map[string]string{
				"orchestration.enabled":               "false",
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "test@example.com",
				"identity.enabled":                    "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Web Modeler is enabled but management Identity is not configured")
			},
		},
		{
			Name: "ConsoleWithExternalManagementIdentityRenders",
			Values: map[string]string{
				"orchestration.enabled":       "false",
				"console.enabled":             "true",
				"identity.enabled":            "false",
				"global.identity.service.url": "http://identity.other-ns.svc:8080",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "ConsoleWithoutManagementIdentityFails",
			Values: map[string]string{
				"orchestration.enabled": "false",
				"console.enabled":       "true",
				"identity.enabled":      "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Console is enabled but management Identity is not configured")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestIdentityDatabaseRequiredForNonKeycloakAuth() {
	identityOidcValues := func(extra map[string]string) map[string]string {
		values := map[string]string{
			"identity.enabled":                      "true",
			"global.identity.auth.enabled":          "true",
			"global.identity.auth.type":             "GENERIC",
			"global.identity.auth.issuer":           "https://issuer.example.com",
			"global.identity.auth.issuerBackendUrl": "https://issuer.example.com",
			"global.identity.auth.tokenUrl":         "https://issuer.example.com/token",
			"global.identity.auth.jwksUrl":          "https://issuer.example.com/keys",
			"global.identity.auth.authUrl":          "https://issuer.example.com/auth",
		}
		for key, value := range extra {
			values[key] = value
		}
		return values
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "GenericAuthWithoutDatabaseFails",
			Values: identityOidcValues(nil),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Management Identity requires a database")
			},
		},
		{
			Name: "MicrosoftAuthWithoutDatabaseFails",
			Values: identityOidcValues(map[string]string{
				"global.identity.auth.type": "MICROSOFT",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Management Identity requires a database")
			},
		},
		{
			Name: "GenericAuthWithExternalDatabaseRenders",
			Values: identityOidcValues(map[string]string{
				"identity.externalDatabase.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithBundledPostgresqlRenders",
			Values: identityOidcValues(map[string]string{
				"identityPostgresql.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithDatabaseEnvRenders",
			Values: identityOidcValues(map[string]string{
				"identity.env[0].name":  "IDENTITY_DATABASE_HOST",
				"identity.env[0].value": "postgres.example.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithUnrelatedEnvFails",
			Values: identityOidcValues(map[string]string{
				"identity.env[0].name":  "TZ",
				"identity.env[0].value": "UTC",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Management Identity requires a database")
			},
		},
		{
			Name: "GenericAuthWithEnvFromRenders",
			Values: identityOidcValues(map[string]string{
				"identity.envFrom[0].secretRef.name": "identity-database",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithDatasourceInConfigurationRenders",
			Values: identityOidcValues(map[string]string{
				"identity.configuration": "spring:\n  datasource:\n    url: jdbc:postgresql://postgres.example.com/identity",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "KeycloakAuthWithoutDatabaseRenders",
			Values: map[string]string{
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithIdentityDisabledRenders",
			Values: identityOidcValues(map[string]string{
				"identity.enabled": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "MultiTenancyWithoutIdentityDatabaseFails",
			Values: map[string]string{
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
				"global.multitenancy.enabled":  "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Multi-Tenancy feature")
			},
		},
		{
			Name: "MultiTenancyWithBundledPostgresqlRenders",
			Values: map[string]string{
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
				"global.multitenancy.enabled":  "true",
				"identityPostgresql.enabled":   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithIdentityAuthDisabledRenders",
			Values: identityOidcValues(map[string]string{
				"global.identity.auth.enabled": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

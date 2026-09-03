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
	"testing"
)

func (s *ConstraintTemplateTest) TestIdentityDatabaseRequiredForNonKeycloakAuth() {
	identityOidcValues := func(extra map[string]string) map[string]string {
		values := map[string]string{
			"orchestration.data.secondaryStorage.type": "elasticsearch",
			"identity.enabled":                         "true",
			"global.identity.auth.enabled":             "true",
			"global.identity.auth.type":                "GENERIC",
			"global.identity.auth.issuer":              "https://issuer.example.com",
			"global.identity.auth.issuerBackendUrl":    "https://issuer.example.com",
			"global.identity.auth.tokenUrl":            "https://issuer.example.com/token",
			"global.identity.auth.jwksUrl":             "https://issuer.example.com/keys",
			"global.identity.auth.authUrl":             "https://issuer.example.com/auth",
		}
		for key, value := range extra {
			values[key] = value
		}
		return values
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "GenericAuthWithoutDatabaseFails",
			Values: identityOidcValues(map[string]string{}),
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
			Name:        "GenericAuthWithSpringApplicationJsonStandsDown",
			Values:      identityOidcValues(nil),
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/common/testdata/values-identity-spring-application-json.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithDatasourceEnvStandsDown",
			Values: identityOidcValues(map[string]string{
				"identity.env[0].name":  "IDENTITY_DATABASE_HOST",
				"identity.env[0].value": "postgres.example.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name:        "GenericAuthWithTuningOnlyDatasourceEnvStandsDown",
			Values:      identityOidcValues(nil),
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/common/testdata/values-identity-datasource-tuning-only.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithUnrelatedEnvStandsDown",
			Values: identityOidcValues(map[string]string{
				"identity.env[0].name":  "TZ",
				"identity.env[0].value": "UTC",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithEnvFromStandsDown",
			Values: identityOidcValues(map[string]string{
				"identity.envFrom[0].secretRef.name": "identity-database",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithConfigurationStandsDown",
			Values: identityOidcValues(map[string]string{
				"identity.configuration": "spring:\n  datasource:\n    url: jdbc:postgresql://postgres.example.com/identity",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithMultiDocumentExtraConfigurationStandsDown",
			Values: identityOidcValues(map[string]string{
				"identity.extraConfiguration[0].file":    "datasource.yaml",
				"identity.extraConfiguration[0].content": "server:\n  port: 8084\n---\nspring:\n  datasource:\n    url: jdbc:postgresql://postgres.example.com/identity",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithNonImportedExtraConfigurationStandsDown",
			Values: identityOidcValues(map[string]string{
				"identity.extraConfiguration[0].file":         "log4j2.xml",
				"identity.extraConfiguration[0].content":      "<Configuration/>",
				"identity.extraConfiguration[0].springImport": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "KeycloakAuthWithoutDatabaseRenders",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"global.identity.auth.enabled":             "true",
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
			Name: "GenericAuthWithIdentityAuthDisabledRenders",
			Values: identityOidcValues(map[string]string{
				"global.identity.auth.enabled": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "MultiTenancyWithoutIdentityDatabaseFails",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"global.identity.auth.enabled":             "true",
				"global.multitenancy.enabled":              "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Multi-Tenancy feature")
			},
		},
		{
			Name: "MultiTenancyWithBundledPostgresqlRenders",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"global.identity.auth.enabled":             "true",
				"global.multitenancy.enabled":              "true",
				"identityPostgresql.enabled":               "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "MultiTenancyWithOpaqueConfigurationStillFails",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"global.identity.auth.enabled":             "true",
				"global.multitenancy.enabled":              "true",
				"identity.envFrom[0].configMapRef.name":    "proxy-settings",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Multi-Tenancy feature")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

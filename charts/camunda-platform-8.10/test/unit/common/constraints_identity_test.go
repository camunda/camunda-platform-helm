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
			Name: "GenericAuthWithDatasourceInExtraConfigurationRenders",
			Values: identityOidcValues(map[string]string{
				"identity.extraConfiguration[0].file":    "datasource.yaml",
				"identity.extraConfiguration[0].content": "spring:\n  datasource:\n    url: jdbc:postgresql://postgres.example.com/identity",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithDottedDatasourceInConfigurationRenders",
			Values: identityOidcValues(map[string]string{
				"identity.configuration": "spring.datasource.url: jdbc:postgresql://postgres.example.com/identity",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "GenericAuthWithUnrelatedConfigurationFails",
			Values: identityOidcValues(map[string]string{
				"identity.configuration": "logging:\n  level:\n    root: INFO # datasource audit marker",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Management Identity requires a database")
			},
		},
		{
			Name: "GenericAuthWithNonImportedDatasourceFileFails",
			Values: identityOidcValues(map[string]string{
				"identity.extraConfiguration[0].file":         "notes.txt",
				"identity.extraConfiguration[0].content":      "datasource audit marker",
				"identity.extraConfiguration[0].springImport": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Management Identity requires a database")
			},
		},
		{
			Name: "GenericAuthWithUnrelatedExtraConfigurationFails",
			Values: identityOidcValues(map[string]string{
				"identity.extraConfiguration[0].file":    "log.yaml",
				"identity.extraConfiguration[0].content": "logging:\n  level:\n    root: INFO",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Management Identity requires a database")
			},
		},
		{
			Name: "GenericAuthWithDatasourcePropertiesFileRenders",
			Values: identityOidcValues(map[string]string{
				"identity.extraConfiguration[0].file":    "datasource.properties",
				"identity.extraConfiguration[0].content": "spring.datasource.url=jdbc:postgresql://postgres.example.com/identity",
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
			Name: "GenericAuthWithExternalManagementIdentityRenders",
			Values: identityOidcValues(map[string]string{
				"identity.enabled":            "false",
				"global.identity.service.url": "http://identity.other-ns.svc:8080",
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
			Name: "MultiTenancyWithIdentityDatabaseRenders",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"global.identity.auth.enabled":             "true",
				"global.multitenancy.enabled":              "true",
				"identity.externalDatabase.enabled":        "true",
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

func (s *ConstraintTemplateTest) TestIdentityDatabaseConstraintDefersToRemovedKey() {
	testCases := []testhelpers.TestCase{
		{
			Name: "RemovedIdentityPostgresqlKeyReportsRemovalNotMissingDatabase",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"global.identity.auth.enabled":             "true",
				"global.identity.auth.type":                "GENERIC",
				"global.identity.auth.issuer":              "https://issuer.example.com",
				"global.identity.auth.issuerBackendUrl":    "https://issuer.example.com",
				"global.identity.auth.tokenUrl":            "https://issuer.example.com/token",
				"global.identity.auth.jwksUrl":             "https://issuer.example.com/keys",
				"global.identity.auth.authUrl":             "https://issuer.example.com/auth",
				"identityPostgresql.enabled":               "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "The Helm values file key \"identityPostgresql\" has been removed")
				s.Require().NotContains(err.Error(), "Management Identity requires a database")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

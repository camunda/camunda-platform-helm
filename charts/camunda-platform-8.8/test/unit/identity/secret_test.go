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
	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
)

type secretTest struct {
	suite.Suite
	chartPath  string
	release    string
	namespace  string
	templates  []string
	secretName []string
}

func TestSecretTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretTest{
		chartPath:  chartPath,
		release:    "camunda-platform-test",
		namespace:  "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates:  []string{},
		secretName: []string{},
	})
}

func (s *secretTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Skip: true,
			Name: "TestSecretExternalDatabaseEnabledWithDefinedPassword",
			Values: map[string]string{
				"identity.enabled":                   "true",
				"identityPostgresql.enabled":         "false",
				"identity.externalDatabase.enabled":  "true",
				"identity.externalDatabase.password": "super-secure-ext",
			},
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: []string{"templates/identity/postgresql-secret.yaml"},
			},
			Verifier: func(t *testing.T, output string, err error) {
				var secret coreV1.Secret
				helm.UnmarshalK8SYaml(t, output, &secret)

				// then
				s.NotEmpty(secret.Data)
				s.Require().Equal("super-secure-ext", string(secret.Data["password"]))
			},
		},
		{
			Skip: true,
			Name: "TestFirstUserPassword",
			Values: map[string]string{
				"identity.enabled":                              "true",
				"global.identity.auth.enabled":                  "true",
				"global.identity.auth.issuer":                   "https://login.microsoftonline.com/<directoryId>/v2.0",
				"global.identity.auth.issuerBackendUrl":         "https://login.microsoftonline.com/<directoryId>/v2.0",
				"global.identity.auth.tokenUrl":                 "https://login.microsoftonline.com/<directoryId>/oauth2/v2.0/token",
				"global.identity.auth.jwksUrl":                  "https://login.microsoftonline.com/<directoryId>/discovery/v2.0/keys",
				"global.identity.auth.publicIssuerUrl":          "https://login.microsoftonline.com/<directoryId>/v2.0",
				"global.identity.auth.type":                     "\"MICROSOFT\"",
				"global.identity.auth.core.clientId":            "<clientId>",
				"global.identity.auth.core.audience":            "<clientId>",
				"global.identity.auth.core.existingSecret.name": "integration-test-credentials",
				"global.identity.auth.core.existingSecretKey":   "entra-child-app-client-secret",
				"global.identity.auth.redirectUrl: \"https://{{ .Values.global.ingress.host }}/core\"" +
					//"global.identity.auth.identity:clientId: <clientId>\n        audience: <clientId>\n        # this existngSecret must be a string literal\n        existingSecret: <clientSecret>\n        initialClaimValue: d70412f6-5a6e-4271-8e45-fa497056ac1e # Hamza's object ID\n        redirectUrl: \"https://{{ .Values.global.ingress.host }}/identity\"\n      optimize:\n        clientId: <clientId>\n        audience: <clientId>\n        existingSecret:\n          name: integration-test-credentials\n        existingSecretKey: entra-child-app-client-secret\n        redirectUrl: \"https://{{ .Values.global.ingress.host }}/optimize\"\n      connectors:\n        clientId: <clientId>\n        audience: <clientId>\n        clientApiAudience: <clientId>\n        existingSecret:\n          name: integration-test-credentials\n        existingSecretKey: entra-child-app-client-secret\n        tokenScope: <clientId>/.default\n      webModeler:\n        clientId: <clientId>\n        audience: <clientId>\n        clientApiAudience: <clientId>\n        publicApiAudience: <clientId>\n        redirectUrl: \"https://{{ .Values.global.ingress.host }}/modeler\"\n      console:\n        wellKnown: https://login.microsoftonline.com/common/v2.0/.well-known/openid-configuration\n        clientId: <clientId>\n        audience: <clientId>\n        existingSecret:\n          name: integration-test-credentials\n        existingSecretKey: entra-child-app-client-secret\n        tokenScope: <clientId>/.default\n        redirectUrl: \"https://{{ .Values.global.ingress.host }}/modeler\"",
					"global.identity.auth.type": "KEYCLOAK",
				"identity.firstUser.enabled":  "true",
				"identity.firstUser.password": "foo",
			},
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: []string{"templates/identity/deployment.yaml"},
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				envVars := deployment.Spec.Template.Spec.Containers[0].Env
				var identityFirstUserPassword coreV1.EnvVar
				for _, envVar := range envVars {
					if envVar.Name == "VALUES_IDENTITY_FIRSTUSER_PASSWORD" {
						identityFirstUserPassword = envVar
					}
				}
				s.Require().Equal("foo", identityFirstUserPassword.Value)
			},
		},
		{
			Skip: true,
			Name: "TestExternalIdentityPostgresqlSecretRenderedOnCompatibilityPostgresqlEnabledSecrets",
			Values: map[string]string{
				// note how it's not identityPostgresql.enabled so we can reproduce SUPPORT-21601
				"identity.enabled":                  "true",
				"identity.postgresql.enabled":       "true",
				"identity.externalDatabase.enabled": "false",
			},
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: []string{
					"charts/identityPostgresql/templates/secrets.yaml",
				},
			},
			Verifier: func(t *testing.T, output string, err error) {
				var secret coreV1.Secret

				helm.UnmarshalK8SYaml(t, output, &secret)
				s.Require().Equal("camunda-platform-test-identity-postgresql", secret.Name)
				s.Require().NotEmpty(string(secret.Data["password"]))
			},
		},
		{
			Name: "TestExternalIdentityPostgresqlSecretRenderedOnCompatibilityPostgresqlEnabledDeployment",
			Values: map[string]string{
				// note how it's not identityPostgresql.enabled so we can reproduce SUPPORT-21601
				"identity.enabled":                  "true",
				"identity.postgresql.enabled":       "true",
				"identity.externalDatabase.enabled": "false",
			},
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: []string{
					"templates/identity/deployment.yaml",
				},
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				envVars := deployment.Spec.Template.Spec.Containers[0].Env
				var identityDatabasePassword coreV1.EnvVar
				for _, envVar := range envVars {
					if envVar.Name == "IDENTITY_DATABASE_PASSWORD" {
						identityDatabasePassword = envVar
					}
				}

				s.Require().Equal("IDENTITY_DATABASE_PASSWORD", identityDatabasePassword.Name)
				// I expect Deployment environment variable to reference the secret that is rendered
				s.Require().Equal("camunda-platform-test-identity-postgresql", identityDatabasePassword.ValueFrom.SecretKeyRef.Name)
			},
		},
		{
			Name: "TestExternalIdentityPostgresqlSecretRenderedOnCompatibilityPostgresqlEnabledError",
			Values: map[string]string{
				// note how it's not identityPostgresql.enabled so we can reproduce SUPPORT-21601
				"identity.enabled":                  "false",
				"identity.postgresql.enabled":       "true",
				"identity.externalDatabase.enabled": "false",
			},
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: []string{
					"templates/identity/postgresql-secret.yaml",
				},
			},
			Verifier: func(t *testing.T, output string, err error) {
				// I expect Secret to NOT be rendered via charts/identityPostgresql/templates/secrets.yaml
				s.Require().ErrorContains(err, "could not find template templates/identity/postgresql-secret.yaml in chart")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

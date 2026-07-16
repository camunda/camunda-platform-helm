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
				"identity.enabled":                                              "true",
				"global.identity.auth.enabled":                                  "true",
				"global.security.authentication.method":                         "oidc",
				"connectors.security.authentication.oidc.secret.existingSecret": "foo",
				"global.identity.auth.issuerBackendUrl":                         "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":          "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().ErrorContains(err, "the Camunda Helm chart will no longer automatically generate passwords for the Identity component")
			},
		}, {
			Name: "TestExistingSecretConstraintDoesNotDisplayErrorForComponentWithExistingSecret",
			Values: map[string]string{
				"identity.enabled":                                                 "true",
				"global.identity.auth.enabled":                                     "true",
				"global.security.authentication.method":                            "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret": "bar",
				"global.identity.auth.issuerBackendUrl":                            "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":             "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				requiredComponentsNotSet := strings.Split(err.Error(), "The following values inside your values.yaml need to be set but were not")[1]
				s.Require().NotContains(requiredComponentsNotSet, "orchestration.security.authentication.oidc.secret.existingSecret")
			},
		}, {
			Name: "TestExistingSecretConstraintInWarningModeDoesNotPreventInstall",
			Values: map[string]string{
				"identity.enabled":                                                 "true",
				"global.security.authentication.method":                            "oidc",
				"connectors.security.authentication.oidc.secret.existingSecret":    "foo",
				"orchestration.security.authentication.oidc.secret.existingSecret": "bar",
				"global.identity.auth.issuerBackendUrl":                            "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":             "warning",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestAuthenticationMethodConstraints() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestConnectorsRejectsUnsupportedAuthenticationMethod",
			Values: map[string]string{
				"connectors.security.authentication.method": "none",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "The Connectors authentication method must be either \"basic\" or \"oidc\"")
			},
		},
		{
			Name: "TestOrchestrationRejectsUnsupportedAuthenticationMethod",
			Values: map[string]string{
				"orchestration.security.authentication.method": "none",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "The Orchestration authentication method must be either \"basic\" or \"oidc\"")
			},
		},
		{
			Name: "TestDisabledComponentsIgnoreGlobalAuthenticationMethod",
			Values: map[string]string{
				"connectors.enabled":                    "false",
				"orchestration.enabled":                 "false",
				"global.security.authentication.method": "none",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestSecondaryStorageConstraint() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestSecondaryStorageConstraintFailsWhenOrchestrationEnabledAndNoStorageConfigured",
			Values: map[string]string{
				"orchestration.enabled":        "true",
				"global.elasticsearch.enabled": "false",
				"elasticsearch.enabled":        "false",
				"global.opensearch.enabled":    "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Please configure an expected secondary storage type under `orchestration.data.secondaryStorage.type`")
			},
		},
		{
			Name: "TestSecondaryStorageConstraintDoesNotFireWhenOrchestrationDisabled",
			Values: map[string]string{
				"orchestration.enabled":        "false",
				"global.elasticsearch.enabled": "false",
				"elasticsearch.enabled":        "false",
				"global.opensearch.enabled":    "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestSecondaryStorageConstraintDoesNotFireWhenElasticsearchEnabled",
			Values: map[string]string{
				"orchestration.enabled":        "true",
				"global.elasticsearch.enabled": "true",
				"elasticsearch.enabled":        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestSecondaryStorageConstraintDoesNotFireWhenOpensearchEnabled",
			Values: map[string]string{
				"orchestration.enabled":        "true",
				"global.elasticsearch.enabled": "false",
				"elasticsearch.enabled":        "false",
				"global.opensearch.enabled":    "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestSecondaryStorageConstraintDoesNotFireWhenStorageTypeExplicitlySet",
			Values: map[string]string{
				"orchestration.enabled":                    "true",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"global.elasticsearch.enabled":             "false",
				"elasticsearch.enabled":                    "false",
				"global.opensearch.enabled":                "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestGatewayConstraints() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestGatewayNamespaceWithCreateGatewayResourceFails",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.namespace":             "shared-infra",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "global.gateway.namespace and global.gateway.createGatewayResource=true cannot be set together")
			},
		},
		{
			Name: "TestGatewayNamespaceWithoutCreateGatewayResourceSucceeds",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.namespace":             "shared-infra",
				"global.gateway.createGatewayResource": "false",
				"global.host":                          "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestBitnamiSubchartDeprecationWarnings() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestBitnamiDeprecationWarningDoesNotPreventInstallWithElasticsearch",
			Values: map[string]string{
				// elasticsearch.enabled and global.elasticsearch.enabled default to true via test helper
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestBitnamiDeprecationWarningDoesNotPreventInstallWithMultipleSubcharts",
			Values: map[string]string{
				// elasticsearch.enabled and global.elasticsearch.enabled default to true via test helper
				"identityPostgresql.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestBitnamiDeprecationWarningDoesNotPreventInstallWithAllSubcharts",
			Values: map[string]string{
				// elasticsearch.enabled and global.elasticsearch.enabled default to true via test helper
				"identityPostgresql.enabled":   "true",
				"identityKeycloak.enabled":     "true",
				"identity.enabled":             "true",
				"webModelerPostgresql.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestRenderSucceedsWithAllBitnamiSubchartsDisabled",
			Values: map[string]string{
				"elasticsearch.enabled":                    "false",
				"global.elasticsearch.enabled":             "false",
				"orchestration.data.secondaryStorage.type": "rdbms",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestLegacyJksTruststoreFieldsRenderWithoutCrash() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestSecondaryStorageElasticsearchTlsSecretRendersOk",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                    "elasticsearch",
				"orchestration.data.secondaryStorage.elasticsearch.tls.secret.existingSecret": "my-legacy-jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestSecondaryStorageOpensearchTlsSecretRendersOk",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                 "opensearch",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecret": "my-legacy-jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestOptimizeElasticsearchTlsSecretRendersOk",
			Values: map[string]string{
				"optimize.database.elasticsearch.tls.secret.existingSecret": "my-legacy-jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestOptimizeOpensearchTlsSecretRendersOk",
			Values: map[string]string{
				"optimize.database.opensearch.tls.secret.existingSecret": "my-legacy-jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			// Minimal config: only existingSecret set, existingSecretKey defaults to "".
			// Pins the round-2 P1 fix: deprecation gate must fire on existingSecret-only,
			// not require both fields (which the old hasSecretConfig-based gate did).
			Name: "TestGlobalElasticsearchTlsJksSecretRendersOk_ExistingSecretOnly",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":           "elasticsearch",
				"global.elasticsearch.tls.jks.secret.existingSecret": "my-jks-pw-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestGlobalOpensearchTlsJksSecretRendersOk_ExistingSecretOnly",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":        "opensearch",
				"global.opensearch.tls.jks.secret.existingSecret": "my-jks-pw-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			// Exercises the inlineSecret branch of the gate
			// (or .secret.existingSecret .secret.inlineSecret).
			// Both branches must fire the warning independently.
			Name: "TestGlobalElasticsearchTlsJksSecretRendersOk_InlineSecret",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":         "elasticsearch",
				"global.elasticsearch.tls.jks.secret.inlineSecret": "changeit",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestGlobalOpensearchTlsJksSecretRendersOk_InlineSecret",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":      "opensearch",
				"global.opensearch.tls.jks.secret.inlineSecret": "changeit",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestGlobalElasticsearchTlsSecretRendersOk",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":       "elasticsearch",
				"global.elasticsearch.tls.secret.existingSecret": "my-legacy-jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestGlobalOpensearchTlsSecretRendersOk",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":    "opensearch",
				"global.opensearch.tls.secret.existingSecret": "my-legacy-jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestCaBundleAndLegacyJksCoexistRenderOk_Elasticsearch",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                    "elasticsearch",
				"orchestration.data.secondaryStorage.elasticsearch.tls.secret.existingSecret": "my-legacy-jks",
				"global.tls.caBundle.secret.existingSecret":                                   "camunda-ca-bundle",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestCaBundleAndLegacyJksCoexistRenderOk_Opensearch",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                 "opensearch",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecret": "my-legacy-jks",
				"global.tls.caBundle.secret.existingSecret":                                "camunda-ca-bundle",
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
				"orchestration.data.secondaryStorage.type":  "elasticsearch",
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

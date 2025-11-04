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
				"connectors.security.authentication.oidc.existingSecret.name":           "foo",
				"orchestration.security.authentication.oidc.existingSecret":     "",
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
				"identity.enabled":                                                   "true",
				"global.identity.auth.enabled":                                       "true",
				"orchestration.security.authentication.oidc.existingSecret.name":    "bar",
				"global.identity.auth.issuerBackendUrl":                              "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":               "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				requiredComponentsNotSet := strings.Split(err.Error(), "The following values inside your values.yaml need to be set but were not")[1]
				s.Require().NotContains(requiredComponentsNotSet, "orchestration.security.authentication.oidc.existingSecret")
			},
		}, {
			Name: "TestExistingSecretConstraintInWarningModeDoesNotPreventInstall",
			Values: map[string]string{
				"identity.enabled":                                                "true",
				"connectors.security.authentication.oidc.existingSecret.name":             "foo",
				"orchestration.security.authentication.oidc.existingSecret.name": "bar",
				"global.identity.auth.issuerBackendUrl":                           "http://keycloak:80/auth/realms/camunda-platform",
				"global.testDeprecationFlags.existingSecretsMustBeSet":            "warning",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// then
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestOIDCSecretConstraints() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestConnectorsOIDCWithoutSecretFails",
			Values: map[string]string{
				"identity.enabled":                              "true",
				"global.identity.auth.enabled":                  "true",
				"connectors.enabled":                            "true",
				"connectors.security.authentication.method":     "oidc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Connectors is configured to use OIDC authentication but the client secret is not configured")
			},
		},
		{
			Name: "TestOrchestrationOIDCWithoutSecretFails",
			Values: map[string]string{
				"identity.enabled":                             "true",
				"global.identity.auth.enabled":                 "true",
				"orchestration.enabled":                        "true",
				"orchestration.security.authentication.method": "oidc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Orchestration Cluster is configured to use OIDC authentication but the client secret is not configured")
			},
		},
		{
			Name: "TestOptimizeOIDCWithoutSecretFails",
			Values: map[string]string{
				"identity.enabled":             "true",
				"global.identity.auth.enabled": "true",
				"optimize.enabled":             "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "Optimize is enabled with Identity authentication but the client secret is not configured")
			},
		},
		{
			Name: "TestOIDCWithInternalKeycloakPasses",
			Values: map[string]string{
				"identity.enabled":                          "true",
				"identityKeycloak.enabled":                  "true",
				"global.identity.auth.enabled":              "true",
				"connectors.enabled":                        "true",
				"connectors.security.authentication.method": "oidc",
				"orchestration.enabled":                     "true",
				"orchestration.security.authentication.method": "oidc",
				"optimize.enabled":                          "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name: "TestOIDCWithSecretsProvidedPasses",
			Values: map[string]string{
				"identity.enabled":                                                      "true",
				"global.identity.auth.enabled":                                          "true",
				"connectors.enabled":                                                    "true",
				"connectors.security.authentication.method":                             "oidc",
				"connectors.security.authentication.oidc.secret.existingSecret":         "test-secret",
				"connectors.security.authentication.oidc.secret.existingSecretKey":      "client-secret",
				"orchestration.enabled":                                                 "true",
				"orchestration.security.authentication.method":                          "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":      "test-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey":   "client-secret",
				"optimize.enabled":                                                      "true",
				"global.identity.auth.optimize.secret.existingSecret":                   "test-secret",
				"global.identity.auth.optimize.secret.existingSecretKey":                "client-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

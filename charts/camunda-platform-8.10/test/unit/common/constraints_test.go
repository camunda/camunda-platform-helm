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
	"os/exec"
	"path/filepath"
	"strconv"
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

func (s *ConstraintTemplateTest) TestSecondaryStorageConstraint() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestSecondaryStorageConstraintFailsWhenOrchestrationEnabledAndNoStorageConfigured",
			Values: map[string]string{
				"orchestration.enabled":        "true",
				"global.elasticsearch.enabled": "false",
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
				"global.opensearch.enabled":                "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConstraintTemplateTest) TestPusherSecretConstraint() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestPusherSecretConstraintErrorWhenNotSet",
			Values: map[string]string{
				"identity.enabled":                                     "true",
				"webModeler.enabled":                                   "true",
				"webModeler.restapi.mail.fromAddress":                  "example@example.com",
				"global.testDeprecationFlags.existingSecretsMustBeSet": "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "webModeler.restapi.pusher.secret.existingSecret")
				s.Require().Contains(err.Error(), "webModeler.restapi.pusher.client.secret.existingSecret")
			},
		},
		{
			Name: "TestPusherSecretConstraintDoesNotListSetSecrets",
			Values: map[string]string{
				"identity.enabled":                                          "true",
				"webModeler.enabled":                                        "true",
				"webModeler.restapi.mail.fromAddress":                       "example@example.com",
				"webModeler.restapi.pusher.secret.existingSecret":           "my-pusher-secret",
				"webModeler.restapi.pusher.secret.existingSecretKey":        "secret-key",
				"webModeler.restapi.pusher.client.secret.existingSecret":    "my-pusher-client-secret",
				"webModeler.restapi.pusher.client.secret.existingSecretKey": "client-key",
				"global.testDeprecationFlags.existingSecretsMustBeSet":      "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				if err != nil {
					s.Require().NotContains(err.Error(), "webModeler.restapi.pusher.secret.existingSecret")
					s.Require().NotContains(err.Error(), "webModeler.restapi.pusher.client.secret.existingSecret")
				}
			},
		},
		{
			Name: "TestPusherSecretConstraintNotCheckedWhenWebModelerDisabled",
			Values: map[string]string{
				"webModeler.enabled": "false",
				"global.testDeprecationFlags.existingSecretsMustBeSet": "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				if err != nil {
					s.Require().NotContains(err.Error(), "webModeler.restapi.pusher")
				}
			},
		},
		{
			Name: "TestPusherSecretConstraintErrorWhenNotSetViaCamundaHubEnabled",
			Values: map[string]string{
				"identity.enabled":                                     "true",
				"camundaHub.enabled":                                   "true",
				"webModeler.restapi.mail.fromAddress":                  "example@example.com",
				"global.testDeprecationFlags.existingSecretsMustBeSet": "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "webModeler.restapi.pusher.secret.existingSecret")
				s.Require().Contains(err.Error(), "webModeler.restapi.pusher.client.secret.existingSecret")
			},
		},
		{
			Name: "TestPusherSecretConstraintDoesNotListSecretsSetUnderCamundaHubKeys",
			Values: map[string]string{
				"identity.enabled":                                          "true",
				"camundaHub.enabled":                                        "true",
				"webModeler.restapi.mail.fromAddress":                       "example@example.com",
				"camundaHub.restapi.pusher.secret.existingSecret":           "my-pusher-secret",
				"camundaHub.restapi.pusher.secret.existingSecretKey":        "secret-key",
				"camundaHub.restapi.pusher.client.secret.existingSecret":    "my-pusher-client-secret",
				"camundaHub.restapi.pusher.client.secret.existingSecretKey": "client-key",
				"global.testDeprecationFlags.existingSecretsMustBeSet":      "error",
			},
			Verifier: func(t *testing.T, output string, err error) {
				if err != nil {
					s.Require().NotContains(err.Error(), "webModeler.restapi.pusher.secret.existingSecret")
					s.Require().NotContains(err.Error(), "webModeler.restapi.pusher.client.secret.existingSecret")
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// helmMajorVersion returns the major version of the helm binary on PATH.
func helmMajorVersion() int {
	out, err := exec.Command("helm", "version", "--template={{.Version}}").Output()
	if err != nil {
		return 0
	}
	vStr := strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
	parts := strings.SplitN(vStr, ".", 2)
	if len(parts) == 0 {
		return 0
	}
	major, _ := strconv.Atoi(parts[0])
	return major
}

func (s *ConstraintTemplateTest) TestHelmVersionConstraint() {
	testCases := []testhelpers.TestCase{
		{
			// .Capabilities.HelmVersion.Version is set by the running Helm binary, so this test
			// branches on the detected major version to cover both paths without a fake binary.
			Name:   "TestHelmVersionGuard",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				if helmMajorVersion() >= 4 {
					s.Require().Nil(err)
				} else {
					s.Require().ErrorContains(err, "requires Helm CLI v4")
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// TestLegacyJksTruststoreFieldsRenderWithoutCrash asserts that setting any
// of the legacy JKS truststore fields targeted by the chart-15.x
// deprecation does not break template rendering. The deprecation warning
// itself is emitted via NOTES.txt at install time and is not surfaced by
// `helm template` (the framework these tests use), so warning-content
// assertions live only in manual `helm install --dry-run` / production
// install verification — same constraint that applies to the existing
// global.elasticsearch.tls.secret and Bitnami subchart deprecation
// warnings.
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
			Name: "TestRenderSucceedsWithAllBitnamiSubchartsDisabled",
			Values: map[string]string{
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

// TestCamundaHubConsolidationDeprecationWarningsRenderOk exercises the
// consolidation deprecation-warning branches that fire when the legacy
// console.enabled / webModeler.enabled keys are set. Both branches must render
// without failing; the DEPRECATION text is emitted via NOTES.txt and is not
// surfaced by `helm template`, so content checks live in manual dry-run
// verification (same constraint as the other deprecation warnings above).
func (s *ConstraintTemplateTest) TestCamundaHubConsolidationDeprecationWarningsRenderOk() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestLegacyConsoleEnabledRendersOk",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"console.enabled":                          "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
		{
			Name: "TestLegacyWebModelerEnabledRendersOk",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"webModeler.enabled":                       "true",
				"webModeler.restapi.mail.fromAddress":      "noreply@example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Nil(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// TestWebModelerExternalDatabaseUserRemovedGate verifies the
// webModeler.restapi.externalDatabase.user removal check (camundaPlatform.keyRemoved,
// which calls fail and IS surfaced by helm template) fires on both enablement
// paths: the legacy webModeler.enabled key and the new camundaHub.enabled key.
func (s *ConstraintTemplateTest) TestWebModelerExternalDatabaseUserRemovedGate() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestRemovedKeyFailsViaCamundaHubEnabled",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"camundaHub.enabled":                       "true",
				"webModeler.restapi.mail.fromAddress":      "noreply@example.com",
				"webModeler.restapi.externalDatabase.user": "modeler-user",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "webModeler.restapi.externalDatabase.user")
				s.Require().ErrorContains(err, "has been removed")
			},
		},
		{
			Name: "TestRemovedKeyFailsViaLegacyWebModelerEnabled",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"identity.enabled":                         "true",
				"webModeler.enabled":                       "true",
				"webModeler.restapi.mail.fromAddress":      "noreply@example.com",
				"webModeler.restapi.externalDatabase.user": "modeler-user",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().ErrorContains(err, "webModeler.restapi.externalDatabase.user")
				s.Require().ErrorContains(err, "has been removed")
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

// TestDeprecatedKeyHelperRendersWithoutCrash exercises the
// camundaPlatform.keyDeprecated helper once: a deprecated app-config-proxy key
// (epic #6051) set to a non-default value must render without failing. The
// DEPRECATION warning itself is emitted via NOTES.txt and is not surfaced by
// `helm template` (the framework these tests use), so warning-content checks
// live in manual `helm install --dry-run` verification — same constraint as
// the existing JKS truststore and Bitnami subchart deprecation warnings.
func (s *ConstraintTemplateTest) TestDeprecatedKeyHelperRendersWithoutCrash() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestDeprecatedKeySetRenderOk",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.logLevel":                   "debug",
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
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

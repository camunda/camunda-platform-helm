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
				"configmapApplication.spring.profiles.active": "identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesOperate",
			Values: map[string]string{
				"orchestration.profiles.operate": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesTasklist",
			Values: map[string]string{
				"orchestration.profiles.tasklist": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,consolidated-auth",
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
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedCompatibility() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesBroker",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"orchestration.profiles.broker":              "false",
				"zeebe.enabled":                              "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesOperate",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"orchestration.profiles.operate":             "false",
				"operate.enabled":                            "true",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesTasklist",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"orchestration.profiles.tasklist":            "false",
				"tasklist.enabled":                           "true",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainContextPath",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"zeebeGateway.enabled":                       "true",
				"zeebeGateway.contextPath":                   "/custom",
			},
			Expected: map[string]string{
				"configmapApplication.server.servlet.context-path": "/custom",
				"configmapApplication.management.server.base-path": "/custom",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortServer",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"zeebeGateway.enabled":                       "true",
				"zeebeGateway.service.restPort":              "1111",
			},
			Expected: map[string]string{
				"configmapApplication.server.port": "1111",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortGRPC",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"zeebeGateway.enabled":                       "true",
				"zeebeGateway.service.grpcPort":              "1111",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.api.grpc.port": "1111",
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

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedRDBMS() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainRDBMSBasicConfig",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":              "true",
				"orchestration.data.secondaryStorage.rdbms.url":      "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username": "camunda",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.url":      "jdbc:postgresql://localhost:5432/camunda",
				"configmapApplication.camunda.data.secondary-storage.rdbms.username": "camunda",
				"configmapApplication.camunda.data.secondary-storage.rdbms.password": "${VALUES_ORCHESTRATION_DATA_SECONDARYSTORAGE_RDBMS_PASSWORD:}",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSFlushInterval",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                   "true",
				"orchestration.data.secondaryStorage.rdbms.url":           "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":      "camunda",
				"orchestration.data.secondaryStorage.rdbms.flushInterval": "10s",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.flushInterval": "10s",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSQueueSize",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":               "true",
				"orchestration.data.secondaryStorage.rdbms.url":       "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":  "camunda",
				"orchestration.data.secondaryStorage.rdbms.queueSize": "1000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.queueSize": "1000",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSAutoDDL",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":              "true",
				"orchestration.data.secondaryStorage.rdbms.url":      "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username": "camunda",
				"orchestration.data.secondaryStorage.rdbms.autoDDL":  "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.auto-ddl": "true",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSPrefix",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":              "true",
				"orchestration.data.secondaryStorage.rdbms.url":      "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username": "camunda",
				"orchestration.data.secondaryStorage.rdbms.prefix":   "cam_",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.prefix": "cam_",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSMaxQueueSize",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                  "true",
				"orchestration.data.secondaryStorage.rdbms.url":          "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":     "camunda",
				"orchestration.data.secondaryStorage.rdbms.maxQueueSize": "\"5000\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.maxQueueSize": "\"5000\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSMaxQueueSizeMemoryLimit",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                      "true",
				"orchestration.data.secondaryStorage.rdbms.url":              "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":         "camunda",
				"orchestration.data.secondaryStorage.rdbms.queueMemoryLimit": "5000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.queueMemoryLimit": "5000",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryDefaultTTL",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                               "true",
				"orchestration.data.secondaryStorage.rdbms.url":                       "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                  "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.defaultHistoryTTL": "P30D",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.defaultHistoryTTL": "P30D",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryDefaultBatchOperationTTL",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                             "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                     "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                                "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.defaultBatchOperationHistoryTTL": "P7D",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.defaultBatchOperationHistoryTTL": "P7D",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryCancelProcessInstanceTTL",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                                           "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                                   "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                                              "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.batchOperationCancelProcessInstanceHistoryTTL": "P14D",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.batchOperationCancelProcessInstanceHistoryTTL": "P14D",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryMigrateProcessInstanceTTL",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                                            "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                                    "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                                               "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.batchOperationMigrateProcessInstanceHistoryTTL": "P21D",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.batchOperationMigrateProcessInstanceHistoryTTL": "P21D",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryModifyProcessInstanceTTL",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                                           "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                                   "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                                              "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.batchOperationModifyProcessInstanceHistoryTTL": "P10D",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.batchOperationModifyProcessInstanceHistoryTTL": "P10D",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryResolveIncidentTTL",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                                     "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                             "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                                        "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.batchOperationResolveIncidentHistoryTTL": "P5D",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.batchOperationResolveIncidentHistoryTTL": "P5D",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryMinCleanupInterval",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                       "true",
				"orchestration.data.secondaryStorage.rdbms.url":                               "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                          "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.minHistoryCleanupInterval": "PT1H",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.minHistoryCleanupInterval": "PT1H",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryMaxCleanupInterval",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                       "true",
				"orchestration.data.secondaryStorage.rdbms.url":                               "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                          "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.maxHistoryCleanupInterval": "PT24H",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.maxHistoryCleanupInterval": "PT24H",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryCleanupBatchSize",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                     "true",
				"orchestration.data.secondaryStorage.rdbms.url":                             "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                        "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.historyCleanupBatchSize": "500",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.historyCleanupBatchSize": "500",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryUsageMetricsCleanup",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                 "true",
				"orchestration.data.secondaryStorage.rdbms.url":                         "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                    "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.usageMetricsCleanup": "\"true\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.usageMetricsCleanup": "\"true\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryProcessCacheMaxSize",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                  "true",
				"orchestration.data.secondaryStorage.rdbms.url":                          "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                     "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.processCache.maxSize": "\"1000\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.processCache.maxSize": "\"1000\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryBatchOperationCacheMaxSize",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                            "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.batchOperationCache.maxSize": "500",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.batchOperationCache.maxSize": "500",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryConnectionPoolMaximumSize",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                            "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                    "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                               "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.connectionPool.maximumPoolSize": "\"20\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.connection-pool.maximumPoolSize": "\"20\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryConnectionPoolMinimumIdle",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                        "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                           "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.connectionPool.minimumIdle": "\"5\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.connection-pool.minimumIdle": "\"5\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryConnectionPoolIdleTimeout",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                        "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                           "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.connectionPool.idleTimeout": "\"600000\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.connection-pool.idleTimeout": "\"600000\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryConnectionPoolMaxLifetime",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                        "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                           "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.connectionPool.maxLifetime": "\"1800000\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.connection-pool.maxLifetime": "\"1800000\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSHistoryConnectionPoolConnectionTimeout",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                              "true",
				"orchestration.data.secondaryStorage.rdbms.url":                                      "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                                 "camunda",
				"orchestration.data.secondaryStorage.rdbms.history.connectionPool.connectionTimeout": "\"30000\"",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.history.connection-pool.connectionTimeout": "\"30000\"",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSType",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                                              "true",
				"orchestration.data.secondaryStorage.type":                                           "custom-type",
				"orchestration.data.secondaryStorage.rdbms.url":                                      "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":                                 "camunda",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.type": "custom-type",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedElasticsearch() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestElasticsearchUrlOverridesGlobal",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.url": "http://custom-elasticsearch:9200",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.url": "http://custom-elasticsearch:9200",
			},
		},
		{
			Name: "TestElasticsearchUrlFallsBackToGlobal",
			Values: map[string]string{
				"global.elasticsearch.url.host": "global-elasticsearch",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.url": "http://global-elasticsearch:9200",
			},
		},
		{
			Name: "TestElasticsearchUsernameOverridesGlobal",
			Values: map[string]string{
				"global.elasticsearch.auth.username":                         "global-user",
				"orchestration.data.secondaryStorage.elasticsearch.username": "custom-user",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.username": "custom-user",
			},
		},
		{
			Name: "TestElasticsearchUsernameFallsBackToGlobal",
			Values: map[string]string{
				"global.elasticsearch.auth.username": "global-user",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.username": "global-user",
			},
		},
		{
			Name: "TestElasticsearchClusterNameOverridesGlobal",
			Values: map[string]string{
				"global.elasticsearch.clusterName":                              "global-cluster",
				"orchestration.data.secondaryStorage.elasticsearch.clusterName": "custom-cluster",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.clusterName": "custom-cluster",
			},
		},
		{
			Name: "TestElasticsearchClusterNameFallsBackToGlobal",
			Values: map[string]string{
				"global.elasticsearch.clusterName": "global-cluster",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.clusterName": "global-cluster",
			},
		},
		{
			Name: "TestElasticsearchIndexPrefixOverridesOrchestration",
			Values: map[string]string{
				"orchestration.index.prefix":                                    "orchestration-prefix",
				"orchestration.data.secondaryStorage.elasticsearch.indexPrefix": "custom-prefix",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.indexPrefix": "custom-prefix",
			},
		},
		{
			Name: "TestElasticsearchIndexPrefixFallsBackToOrchestration",
			Values: map[string]string{
				"orchestration.index.prefix": "orchestration-prefix",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.indexPrefix": "orchestration-prefix",
			},
		},
		{
			Name: "TestElasticsearchNumberOfReplicasOverridesOrchestration",
			Values: map[string]string{
				"orchestration.index.replicas":                                       "2",
				"orchestration.data.secondaryStorage.elasticsearch.numberOfReplicas": "5",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.numberOfReplicas": "5",
			},
		},
		{
			Name: "TestElasticsearchNumberOfReplicasFallsBackToOrchestration",
			Values: map[string]string{
				"orchestration.index.replicas": "3",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.numberOfReplicas": "3",
			},
		},
		{
			Name: "TestElasticsearchDateFormat",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.dateFormat": "yyyy-MM-dd",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.dateFormat": "yyyy-MM-dd",
			},
		},
		{
			Name: "TestElasticsearchNumberOfShards",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.numberOfShards": "3",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.numberOfShards": "3",
			},
		},
		{
			Name: "TestElasticsearchVariableSizeThreshold",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.variableSizeThreshold": "16000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.variableSizeThreshold": "16000",
			},
		},
		{
			Name: "TestElasticsearchSecurityEnabled",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.security.enabled": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.security.enabled": "true",
			},
		},
		{
			Name: "TestElasticsearchSecurityVerifyHostname",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.security.verifyHostname": "false",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.security.verifyHostname": "false",
			},
		},
		{
			Name: "TestElasticsearchSecuritySelfSigned",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.security.selfSigned": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.security.selfSigned": "true",
			},
		},
		{
			Name: "TestElasticsearchHistoryPolicyNameOverridesRetention",
			Values: map[string]string{
				"orchestration.history.retention.enabled":                              "true",
				"orchestration.history.retention.policyName":                           "global-policy",
				"orchestration.data.secondaryStorage.elasticsearch.history.policyName": "custom-policy",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.policyName": "custom-policy",
			},
		},
		{
			Name: "TestElasticsearchHistoryPolicyNameFallsBackToRetention",
			Values: map[string]string{
				"orchestration.history.retention.enabled":    "true",
				"orchestration.history.retention.policyName": "global-policy",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.policyName": "global-policy",
			},
		},
		{
			Name: "TestElasticsearchHistoryRolloverSettings",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.history.elsRolloverDateFormat":     "date",
				"orchestration.data.secondaryStorage.elasticsearch.history.rolloverInterval":          "7d",
				"orchestration.data.secondaryStorage.elasticsearch.history.rolloverBatchSize":         "200",
				"orchestration.data.secondaryStorage.elasticsearch.history.waitPeriodBeforeArchiving": "2h",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.elsRolloverDateFormat":     "date",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.rolloverInterval":          "7d",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.rolloverBatchSize":         "200",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.waitPeriodBeforeArchiving": "2h",
			},
		},
		{
			Name: "TestElasticsearchHistoryDelaySettings",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.history.delayBetweenRuns":    "PT5S",
				"orchestration.data.secondaryStorage.elasticsearch.history.maxDelayBetweenRuns": "PT2M",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.delayBetweenRuns":    "PT5S",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.history.maxDelayBetweenRuns": "PT2M",
			},
		},
		{
			Name: "TestElasticsearchCreateSchema",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.createSchema": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.createSchema": "true",
			},
		},
		{
			Name: "TestElasticsearchIncidentNotifier",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.incidentNotifier.auth0Protocol": "https",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.incidentNotifier.auth0Protocol": "https",
			},
		},
		{
			Name: "TestElasticsearchBatchOperationCacheMaxSize",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.batchOperationCache.maxSize": "20000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.batchOperationCache.maxSize": "20000",
			},
		},
		{
			Name: "TestElasticsearchProcessCacheMaxSize",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.processCache.maxSize": "15000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.processCache.maxSize": "15000",
			},
		},
		{
			Name: "TestElasticsearchFormCacheMaxSize",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.formCache.maxSize": "5000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.formCache.maxSize": "5000",
			},
		},
		{
			Name: "TestElasticsearchPostExportSettings",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.postExport.batchSize":           "200",
				"orchestration.data.secondaryStorage.elasticsearch.postExport.delayBetweenRuns":    "PT3S",
				"orchestration.data.secondaryStorage.elasticsearch.postExport.maxDelayBetweenRuns": "PT2M",
				"orchestration.data.secondaryStorage.elasticsearch.postExport.ignoreMissingData":   "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.postExport.batchSize":           "200",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.postExport.delayBetweenRuns":    "PT3S",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.postExport.maxDelayBetweenRuns": "PT2M",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.postExport.ignoreMissingData":   "true",
			},
		},
		{
			Name: "TestElasticsearchBatchOperationsExportItemsOnCreation",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.batchOperations.exportItemsOnCreation": "false",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.batchOperations.exportItemsOnCreation": "false",
			},
		},
		{
			Name: "TestElasticsearchBulkSettings",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.elasticsearch.bulk.delay":       "PT2S",
				"orchestration.data.secondaryStorage.elasticsearch.bulk.size":        "2000",
				"orchestration.data.secondaryStorage.elasticsearch.bulk.memoryLimit": "41943040B",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.bulk.delay":       "PT2S",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.bulk.size":        "2000",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.bulk.memoryLimit": "41943040B",
			},
		},
		{
			Name:   "TestElasticsearchPasswordEnvVar",
			Values: map[string]string{},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.password": "${VALUES_ELASTICSEARCH_PASSWORD:}",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedOpenSearch() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestOpenSearchUrlOverridesGlobal",
			Values: map[string]string{
				"global.opensearch.enabled":                          "true",
				"global.opensearch.url.host":                         "global-opensearch",
				"orchestration.data.secondaryStorage.opensearch.url": "http://custom-opensearch:9200",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.url": "http://custom-opensearch:9200",
			},
		},
		{
			Name: "TestOpenSearchUrlFallsBackToGlobal",
			Values: map[string]string{
				"global.opensearch.enabled":  "true",
				"global.opensearch.url.host": "global-opensearch",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.url": "https://global-opensearch:443",
			},
		},
		{
			Name: "TestOpenSearchUsernameOverridesGlobal",
			Values: map[string]string{
				"global.opensearch.enabled":                               "true",
				"global.opensearch.auth.username":                         "global-user",
				"orchestration.data.secondaryStorage.opensearch.username": "custom-user",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.username": "custom-user",
			},
		},
		{
			Name: "TestOpenSearchUsernameFallsBackToGlobal",
			Values: map[string]string{
				"global.opensearch.enabled":       "true",
				"global.opensearch.auth.username": "global-user",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.username": "global-user",
			},
		},
		{
			Name: "TestOpenSearchClusterNameOverridesGlobal",
			Values: map[string]string{
				"global.opensearch.enabled":                                  "true",
				"global.opensearch.clusterName":                              "global-cluster",
				"orchestration.data.secondaryStorage.opensearch.clusterName": "custom-cluster",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.clusterName": "custom-cluster",
			},
		},
		{
			Name: "TestOpenSearchClusterNameFallsBackToGlobal",
			Values: map[string]string{
				"global.opensearch.enabled":     "true",
				"global.opensearch.clusterName": "global-cluster",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.clusterName": "global-cluster",
			},
		},
		{
			Name: "TestOpenSearchIndexPrefixOverridesOrchestration",
			Values: map[string]string{
				"global.opensearch.enabled":                                  "true",
				"orchestration.index.prefix":                                 "orchestration-prefix",
				"orchestration.data.secondaryStorage.opensearch.indexPrefix": "custom-prefix",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.indexPrefix": "custom-prefix",
			},
		},
		{
			Name: "TestOpenSearchIndexPrefixFallsBackToOrchestration",
			Values: map[string]string{
				"global.opensearch.enabled":  "true",
				"orchestration.index.prefix": "orchestration-prefix",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.indexPrefix": "orchestration-prefix",
			},
		},
		{
			Name: "TestOpenSearchNumberOfReplicasOverridesOrchestration",
			Values: map[string]string{
				"global.opensearch.enabled":                                       "true",
				"orchestration.index.replicas":                                    "2",
				"orchestration.data.secondaryStorage.opensearch.numberOfReplicas": "5",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.numberOfReplicas": "5",
			},
		},
		{
			Name: "TestOpenSearchNumberOfReplicasFallsBackToOrchestration",
			Values: map[string]string{
				"global.opensearch.enabled":    "true",
				"orchestration.index.replicas": "3",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.numberOfReplicas": "3",
			},
		},
		{
			Name: "TestOpenSearchDateFormat",
			Values: map[string]string{
				"global.opensearch.enabled":                                 "true",
				"orchestration.data.secondaryStorage.opensearch.dateFormat": "yyyy-MM-dd",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.dateFormat": "yyyy-MM-dd",
			},
		},
		{
			Name: "TestOpenSearchNumberOfShards",
			Values: map[string]string{
				"global.opensearch.enabled":                                     "true",
				"orchestration.data.secondaryStorage.opensearch.numberOfShards": "3",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.numberOfShards": "3",
			},
		},
		{
			Name: "TestOpenSearchVariableSizeThreshold",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.variableSizeThreshold": "16000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.variableSizeThreshold": "16000",
			},
		},
		{
			Name: "TestOpenSearchSecurityEnabled",
			Values: map[string]string{
				"global.opensearch.enabled":                                       "true",
				"orchestration.data.secondaryStorage.opensearch.security.enabled": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.security.enabled": "true",
			},
		},
		{
			Name: "TestOpenSearchSecurityVerifyHostname",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.security.verifyHostname": "false",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.security.verifyHostname": "false",
			},
		},
		{
			Name: "TestOpenSearchSecuritySelfSigned",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.security.selfSigned": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.security.selfSigned": "true",
			},
		},
		{
			Name: "TestOpenSearchHistoryPolicyNameOverridesRetention",
			Values: map[string]string{
				"global.opensearch.enabled":                                         "true",
				"orchestration.history.retention.enabled":                           "true",
				"orchestration.history.retention.policyName":                        "global-policy",
				"orchestration.data.secondaryStorage.opensearch.history.policyName": "custom-policy",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.policyName": "custom-policy",
			},
		},
		{
			Name: "TestOpenSearchHistoryPolicyNameFallsBackToRetention",
			Values: map[string]string{
				"global.opensearch.enabled":                  "true",
				"orchestration.history.retention.enabled":    "true",
				"orchestration.history.retention.policyName": "global-policy",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.policyName": "global-policy",
			},
		},
		{
			Name: "TestOpenSearchHistoryRolloverSettings",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.history.elsRolloverDateFormat":     "date",
				"orchestration.data.secondaryStorage.opensearch.history.rolloverInterval":          "7d",
				"orchestration.data.secondaryStorage.opensearch.history.rolloverBatchSize":         "200",
				"orchestration.data.secondaryStorage.opensearch.history.waitPeriodBeforeArchiving": "2h",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.elsRolloverDateFormat":     "date",
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.rolloverInterval":          "7d",
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.rolloverBatchSize":         "200",
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.waitPeriodBeforeArchiving": "2h",
			},
		},
		{
			Name: "TestOpenSearchHistoryDelaySettings",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.history.delayBetweenRuns":    "PT5S",
				"orchestration.data.secondaryStorage.opensearch.history.maxDelayBetweenRuns": "PT2M",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.delayBetweenRuns":    "PT5S",
				"configmapApplication.camunda.data.secondary-storage.opensearch.history.maxDelayBetweenRuns": "PT2M",
			},
		},
		{
			Name: "TestOpenSearchCreateSchema",
			Values: map[string]string{
				"global.opensearch.enabled":                                   "true",
				"orchestration.data.secondaryStorage.opensearch.createSchema": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.createSchema": "true",
			},
		},
		{
			Name: "TestOpenSearchIncidentNotifier",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.incidentNotifier.auth0Protocol": "https",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.incidentNotifier.auth0Protocol": "https",
			},
		},
		{
			Name: "TestOpenSearchBatchOperationCacheMaxSize",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.batchOperationCache.maxSize": "20000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.batchOperationCache.maxSize": "20000",
			},
		},
		{
			Name: "TestOpenSearchProcessCacheMaxSize",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.processCache.maxSize": "15000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.processCache.maxSize": "15000",
			},
		},
		{
			Name: "TestOpenSearchFormCacheMaxSize",
			Values: map[string]string{
				"global.opensearch.enabled":                                        "true",
				"orchestration.data.secondaryStorage.opensearch.formCache.maxSize": "5000",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.formCache.maxSize": "5000",
			},
		},
		{
			Name: "TestOpenSearchPostExportSettings",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.postExport.batchSize":           "200",
				"orchestration.data.secondaryStorage.opensearch.postExport.delayBetweenRuns":    "PT3S",
				"orchestration.data.secondaryStorage.opensearch.postExport.maxDelayBetweenRuns": "PT2M",
				"orchestration.data.secondaryStorage.opensearch.postExport.ignoreMissingData":   "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.postExport.batchSize":           "200",
				"configmapApplication.camunda.data.secondary-storage.opensearch.postExport.delayBetweenRuns":    "PT3S",
				"configmapApplication.camunda.data.secondary-storage.opensearch.postExport.maxDelayBetweenRuns": "PT2M",
				"configmapApplication.camunda.data.secondary-storage.opensearch.postExport.ignoreMissingData":   "true",
			},
		},
		{
			Name: "TestOpenSearchBatchOperationsExportItemsOnCreation",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
				"orchestration.data.secondaryStorage.opensearch.batchOperations.exportItemsOnCreation": "false",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.batchOperations.exportItemsOnCreation": "false",
			},
		},
		{
			Name: "TestOpenSearchBulkSettings",
			Values: map[string]string{
				"global.opensearch.enabled":                                       "true",
				"orchestration.data.secondaryStorage.opensearch.bulk.delay":       "PT2S",
				"orchestration.data.secondaryStorage.opensearch.bulk.size":        "2000",
				"orchestration.data.secondaryStorage.opensearch.bulk.memoryLimit": "41943040B",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.bulk.delay":       "PT2S",
				"configmapApplication.camunda.data.secondary-storage.opensearch.bulk.size":        "2000",
				"configmapApplication.camunda.data.secondary-storage.opensearch.bulk.memoryLimit": "41943040B",
			},
		},
		{
			Name: "TestOpenSearchPasswordEnvVar",
			Values: map[string]string{
				"global.opensearch.enabled": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.password": "${VALUES_OPENSEARCH_PASSWORD:}",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

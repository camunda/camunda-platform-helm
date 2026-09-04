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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type ConfigmapTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

type orchestrationApplication struct {
	Camunda struct {
		Security struct {
			Authentication struct {
				OIDC struct {
					GroupsClaim string `yaml:"groups-claim"`
				} `yaml:"oidc"`
				Basic struct {
					AllowUnauthenticatedAPIAccess bool `yaml:"allow-unauthenticated-api-access"`
				} `yaml:"basic"`
			} `yaml:"authentication"`
		} `yaml:"security"`
	} `yaml:"camunda"`
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
				"orchestration.data.secondaryStorage.type":           "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url": "https://opensearch.example.com:443",
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
			Name: "TestApplicationYamlShouldInheritAuthMethodFromGlobal",
			Values: map[string]string{
				"global.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.method":         "oidc",
				"configmapApplication.camunda.security.authentication.oidc.client-id": "orchestration",
			},
		},
		{
			Name: "TestApplicationYamlComponentMethodShouldOverrideGlobal",
			Values: map[string]string{
				"global.security.authentication.method":        "basic",
				"orchestration.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.method": "oidc",
			},
		},
		{
			Name: "TestApplicationYamlNoWebAppProfilesWhenNoSecondaryStorageEnabled",
			Values: map[string]string{
				"global.noSecondaryStorage":                    "true",
				"orchestration.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "admin,broker,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainCamundaHubPingDefaults",
			Values: map[string]string{
				"identity.enabled":                                                    "true",
				"global.identity.auth.enabled":                                        "true",
				"global.identity.keycloak.url.protocol":                               "http",
				"global.identity.keycloak.url.host":                                   "keycloak.prod.svc.cluster.local",
				"global.identity.keycloak.url.port":                                   "8080",
				"global.identity.keycloak.auth.adminUser":                             "admin",
				"global.identity.keycloak.auth.secret.existingSecret":                 "kc-secret",
				"global.identity.keycloak.auth.secret.existingSecretKey":              "password",
				"camundaHub.enabled":                                                  "true",
				"webModeler.restapi.mail.fromAddress":                                 "noreply@example.com",
				"orchestration.hub.ping.endpoint":                                     "https://hub/api/v1/clusters",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.hub.ping.endpoint":                   "https://hub/api/v1/clusters",
				"configmapApplication.camunda.hub.ping.credentials.client-id":      "orchestration",
				"configmapApplication.camunda.hub.ping.credentials.token-endpoint": "http://keycloak.prod.svc.cluster.local:8080/auth/realms/camunda-platform/protocol/openid-connect/token",
				"configmapApplication.camunda.hub.ping.credentials.client-secret":  "${VALUES_CAMUNDAHUB_PING_CLIENT_SECRET:}",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainCamundaHubPingCredentialOverrides",
			Values: map[string]string{
				"identity.enabled":                                                    "true",
				"camundaHub.enabled":                                                  "true",
				"webModeler.restapi.mail.fromAddress":                                 "noreply@example.com",
				"orchestration.hub.ping.endpoint":                                     "https://hub/api/v1/clusters",
				"orchestration.hub.ping.credentials.clientId":                         "ping-client",
				"orchestration.hub.ping.credentials.tokenEndpoint":                    "https://kc/token",
				"orchestration.hub.ping.credentials.tokenRequestParameters.audience":  "https://hub.example/api",
				"orchestration.hub.ping.credentials.tokenRequestParameters.scope":     "api://hub/.default",
				"orchestration.hub.ping.credentials.clientSecret.secret.inlineSecret": "secret",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.hub.ping.credentials.client-id":                         "ping-client",
				"configmapApplication.camunda.hub.ping.credentials.token-endpoint":                    "https://kc/token",
				"configmapApplication.camunda.hub.ping.credentials.token-request-parameters.audience": "https://hub.example/api",
				"configmapApplication.camunda.hub.ping.credentials.token-request-parameters.scope":    "api://hub/.default",
			},
		},
		{
			Name: "TestApplicationYamlShouldUseExplicitPublicKeycloakTokenForHubPing",
			Values: map[string]string{
				"global.identity.auth.publicIssuerUrl":                                "https://idp.example/realms/camunda-platform",
				"orchestration.hub.ping.endpoint":                                     "https://hub/api/v1/clusters",
				"orchestration.hub.ping.credentials.tokenEndpoint":                    "https://idp.example/realms/camunda-platform/protocol/openid-connect/token",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.hub.ping.credentials.token-endpoint": "https://idp.example/realms/camunda-platform/protocol/openid-connect/token",
			},
		},
		{
			Name: "TestApplicationYamlShouldUseExplicitOrchestrationIssuerTokenForHubPing",
			Values: map[string]string{
				"global.identity.auth.publicIssuerUrl":                                "https://global-idp.example/realms/camunda-platform",
				"orchestration.hub.ping.endpoint":                                     "https://hub/api/v1/clusters",
				"orchestration.hub.ping.credentials.tokenEndpoint":                    "https://orch-idp.example/realms/orchestration/protocol/openid-connect/token",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.issuer":                   "https://orch-idp.example/realms/orchestration",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.hub.ping.credentials.token-endpoint": "https://orch-idp.example/realms/orchestration/protocol/openid-connect/token",
			},
		},
		{
			Name: "TestApplicationYamlShouldPreferOrchestrationTokenUrlForHubPing",
			Values: map[string]string{
				"global.identity.auth.tokenUrl":                                       "https://global-idp.example/token",
				"orchestration.hub.ping.endpoint":                                     "https://hub/api/v1/clusters",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
				"orchestration.security.authentication.oidc.tokenUrl":                 "https://orch-idp.example/token",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.hub.ping.credentials.token-endpoint": "https://orch-idp.example/token",
			},
		},
		{
			Name: "TestApplicationYamlShouldUseGlobalTokenUrlForHubPing",
			Values: map[string]string{
				"global.identity.auth.tokenUrl":                                       "https://global-idp.example/token",
				"orchestration.hub.ping.endpoint":                                     "https://hub/api/v1/clusters",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.hub.ping.credentials.token-endpoint": "https://global-idp.example/token",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedOpenSearchAWS() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainOpenSearchAwsEnabledFalseByDefault",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":           "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url": "https://opensearch.example.com:443",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.aws-enabled": "false",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainOpenSearchAwsEnabledViaSecondaryStorage",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                   "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":         "https://opensearch.example.com:443",
				"orchestration.data.secondaryStorage.opensearch.aws.enabled": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.aws-enabled": "true",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestLegacyExporterDatastoreSourceAlignment() {
	distinctOpenSearchSources := map[string]string{
		"optimize.enabled":                                                        "true",
		"optimize.database.elasticsearch.enabled":                                 "false",
		"optimize.database.opensearch.enabled":                                    "true",
		"optimize.database.opensearch.url.protocol":                               "https",
		"optimize.database.opensearch.url.host":                                   "optimize-host",
		"optimize.database.opensearch.url.port":                                   "9555",
		"optimize.database.opensearch.auth.username":                              "optimize-user",
		"optimize.database.opensearch.auth.secret.inlineSecret":                   "optimize-password",
		"orchestration.data.secondaryStorage.type":                                "opensearch",
		"orchestration.data.secondaryStorage.opensearch.url":                      "https://secondary-host:9443",
		"orchestration.data.secondaryStorage.opensearch.auth.username":            "secondary-user",
		"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "secondary-password",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestLegacyOpenSearchExporterUsesOptimizeSource",
			Values: distinctOpenSearchSources,
			Expected: map[string]string{
				"configmapApplication.zeebe.broker.exporters.opensearch.args.url":                     "https://optimize-host:9555",
				"configmapApplication.zeebe.broker.exporters.opensearch.args.authentication.username": "optimize-user",
				"configmapApplication.zeebe.broker.exporters.opensearch.args.authentication.password": "${VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD:}",
				"configmapApplication.camunda.data.secondary-storage.opensearch.url":                  "https://secondary-host:9443",
				"configmapApplication.camunda.data.secondary-storage.opensearch.username":             "secondary-user",
				"configmapApplication.camunda.data.secondary-storage.opensearch.password":             "${VALUES_OPENSEARCH_PASSWORD:}",
			},
		},
		{
			Name: "TestLegacyOpenSearchExporterUsesOptimizePasswordOnSharedEndpoint",
			Values: map[string]string{
				"optimize.enabled":                                      "true",
				"optimize.database.elasticsearch.enabled":               "false",
				"optimize.database.opensearch.enabled":                  "true",
				"optimize.database.opensearch.url.protocol":             "https",
				"optimize.database.opensearch.url.host":                 "opensearch.example.com",
				"optimize.database.opensearch.url.port":                 "443",
				"optimize.database.opensearch.auth.username":            "optimize-user",
				"optimize.database.opensearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":              "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":    "https://opensearch.example.com:443",
			},
			Expected: map[string]string{
				"configmapApplication.zeebe.broker.exporters.opensearch.args.url":                     "https://opensearch.example.com:443",
				"configmapApplication.zeebe.broker.exporters.opensearch.args.authentication.password": "${VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD:}",
			},
		},
		{
			Name: "TestLegacyOpenSearchExporterFallsBackToSecondaryStorageSource",
			Values: map[string]string{
				"optimize.enabled":                                                        "true",
				"optimize.database.elasticsearch.enabled":                                 "false",
				"optimize.database.opensearch.enabled":                                    "true",
				"orchestration.data.secondaryStorage.type":                                "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":                      "https://secondary-host:9443",
				"orchestration.data.secondaryStorage.opensearch.auth.username":            "secondary-user",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "secondary-password",
			},
			Expected: map[string]string{
				"configmapApplication.zeebe.broker.exporters.opensearch.args.url":                     "https://secondary-host:9443",
				"configmapApplication.zeebe.broker.exporters.opensearch.args.authentication.username": "secondary-user",
				"configmapApplication.zeebe.broker.exporters.opensearch.args.authentication.password": "${VALUES_OPENSEARCH_PASSWORD:}",
			},
		},
		{
			Name: "TestLegacyOpenSearchExporterWithoutSecretKeepsOptimizePasswordRef",
			Values: map[string]string{
				"optimize.enabled":                                                        "true",
				"optimize.database.elasticsearch.enabled":                                 "false",
				"optimize.database.opensearch.enabled":                                    "true",
				"optimize.database.opensearch.url.protocol":                               "https",
				"optimize.database.opensearch.url.host":                                   "optimize-host",
				"optimize.database.opensearch.url.port":                                   "9555",
				"optimize.database.opensearch.auth.username":                              "optimize-user",
				"orchestration.data.secondaryStorage.type":                                "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":                      "https://secondary-host:9443",
				"orchestration.data.secondaryStorage.opensearch.auth.username":            "secondary-user",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "secondary-password",
			},
			Expected: map[string]string{
				"configmapApplication.zeebe.broker.exporters.opensearch.args.url":                     "https://optimize-host:9555",
				"configmapApplication.zeebe.broker.exporters.opensearch.args.authentication.username": "optimize-user",
				"configmapApplication.zeebe.broker.exporters.opensearch.args.authentication.password": "${VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD:}",
			},
		},
		{
			Name: "TestLegacyElasticsearchExporterUsesOptimizeSource",
			Values: map[string]string{
				"optimize.enabled":                                         "true",
				"optimize.database.opensearch.enabled":                     "false",
				"optimize.database.elasticsearch.enabled":                  "true",
				"optimize.database.elasticsearch.external":                 "true",
				"optimize.database.elasticsearch.url.protocol":             "https",
				"optimize.database.elasticsearch.url.host":                 "optimize-host",
				"optimize.database.elasticsearch.url.port":                 "9555",
				"optimize.database.elasticsearch.auth.username":            "optimize-user",
				"optimize.database.elasticsearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":                 "elasticsearch",
				"orchestration.data.secondaryStorage.elasticsearch.url":    "https://secondary-host:9443",
			},
			Expected: map[string]string{
				"configmapApplication.zeebe.broker.exporters.elasticsearch.args.url":                     "https://optimize-host:9555",
				"configmapApplication.zeebe.broker.exporters.elasticsearch.args.authentication.username": "optimize-user",
				"configmapApplication.zeebe.broker.exporters.elasticsearch.args.authentication.password": "${VALUES_OPTIMIZE_DATABASE_ELASTICSEARCH_PASSWORD:}",
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.url":                  "https://secondary-host:9443",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

type legacyOpenSearchExporterArgs struct {
	Authentication *struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"authentication"`
	URL string `yaml:"url"`
	AWS *struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"aws"`
}

type legacyExporterApplicationConfig struct {
	Zeebe struct {
		Broker struct {
			Exporters struct {
				OpenSearch struct {
					Args legacyOpenSearchExporterArgs `yaml:"args"`
				} `yaml:"opensearch"`
			} `yaml:"exporters"`
		} `yaml:"broker"`
	} `yaml:"zeebe"`
	Camunda struct {
		Data struct {
			SecondaryStorage struct {
				OpenSearch struct {
					AwsEnabled bool `yaml:"aws-enabled"`
				} `yaml:"opensearch"`
			} `yaml:"secondary-storage"`
		} `yaml:"data"`
	} `yaml:"camunda"`
}

func verifyLegacyExporterApplication(verify func(t *testing.T, application legacyExporterApplicationConfig)) func(t *testing.T, output string, err error) {
	return func(t *testing.T, output string, err error) {
		require.NoError(t, err)

		var configMap corev1.ConfigMap
		helm.UnmarshalK8SYaml(t, output, &configMap)

		var application legacyExporterApplicationConfig
		require.NoError(t, yaml.Unmarshal([]byte(configMap.Data["application.yaml"]), &application))
		verify(t, application)
	}
}

func (s *ConfigmapTemplateTest) TestLegacyOpenSearchExporterAwsSourceAlignment() {
	distinctOpenSearchSources := map[string]string{
		"optimize.enabled":                                                        "true",
		"optimize.database.elasticsearch.enabled":                                 "false",
		"optimize.database.opensearch.enabled":                                    "true",
		"optimize.database.opensearch.url.protocol":                               "https",
		"optimize.database.opensearch.url.host":                                   "optimize-host",
		"optimize.database.opensearch.url.port":                                   "9555",
		"optimize.database.opensearch.auth.username":                              "optimize-user",
		"optimize.database.opensearch.auth.secret.inlineSecret":                   "optimize-password",
		"orchestration.data.secondaryStorage.type":                                "opensearch",
		"orchestration.data.secondaryStorage.opensearch.url":                      "https://secondary-host:9443",
		"orchestration.data.secondaryStorage.opensearch.auth.username":            "secondary-user",
		"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "secondary-password",
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "Optimize AWS mode drops exporter authentication",
			Values: mergeValues(distinctOpenSearchSources, map[string]string{
				"optimize.database.opensearch.aws.enabled":                   "true",
				"orchestration.data.secondaryStorage.opensearch.aws.enabled": "false",
			}),
			Verifier: verifyLegacyExporterApplication(func(t *testing.T, application legacyExporterApplicationConfig) {
				args := application.Zeebe.Broker.Exporters.OpenSearch.Args
				require.Equal(t, "https://optimize-host:9555", args.URL)
				require.Nil(t, args.Authentication)
				require.NotNil(t, args.AWS)
				require.True(t, args.AWS.Enabled)
				require.False(t, application.Camunda.Data.SecondaryStorage.OpenSearch.AwsEnabled)
			}),
		},
		{
			Name: "Secondary AWS mode does not cross to the Optimize source",
			Values: mergeValues(distinctOpenSearchSources, map[string]string{
				"optimize.database.opensearch.aws.enabled":                   "false",
				"orchestration.data.secondaryStorage.opensearch.aws.enabled": "true",
			}),
			Verifier: verifyLegacyExporterApplication(func(t *testing.T, application legacyExporterApplicationConfig) {
				args := application.Zeebe.Broker.Exporters.OpenSearch.Args
				require.Equal(t, "https://optimize-host:9555", args.URL)
				require.Nil(t, args.AWS)
				require.NotNil(t, args.Authentication)
				require.Equal(t, "optimize-user", args.Authentication.Username)
				require.Equal(t, "${VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD:}", args.Authentication.Password)
				require.True(t, application.Camunda.Data.SecondaryStorage.OpenSearch.AwsEnabled)
			}),
		},
		{
			Name: "Optimize AWS does not cross explicit secondary source",
			Values: map[string]string{
				"optimize.enabled":                                                        "true",
				"optimize.database.elasticsearch.enabled":                                 "false",
				"optimize.database.opensearch.enabled":                                    "true",
				"optimize.database.opensearch.aws.enabled":                                "true",
				"orchestration.data.secondaryStorage.type":                                "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":                      "https://secondary-host:9443",
				"orchestration.data.secondaryStorage.opensearch.aws.enabled":              "false",
				"orchestration.data.secondaryStorage.opensearch.auth.username":            "secondary-user",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "secondary-password",
			},
			Verifier: verifyLegacyExporterApplication(func(t *testing.T, application legacyExporterApplicationConfig) {
				args := application.Zeebe.Broker.Exporters.OpenSearch.Args
				require.Equal(t, "https://secondary-host:9443", args.URL)
				require.NotNil(t, args.Authentication)
				require.Equal(t, "secondary-user", args.Authentication.Username)
				require.Equal(t, "${VALUES_OPENSEARCH_PASSWORD:}", args.Authentication.Password)
				require.Nil(t, args.AWS)
			}),
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func mergeValues(base map[string]string, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedElasticsearchAWS() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainElasticsearchAwsEnabledFalseByDefault",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.aws-enabled": "false",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainElasticsearchAwsEnabledViaSecondaryStorage",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                      "elasticsearch",
				"orchestration.data.secondaryStorage.elasticsearch.aws.enabled": "true",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.elasticsearch.aws-enabled": "true",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedRDBMSAWS() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainRDBMSAwsEnabledFalseByDefault",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.aws-enabled": "false",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainRDBMSAwsEnabledViaSecondaryStorage",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.aws.enabled":         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.rdbms.aws-enabled": "true",
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
			Name: "TestApplicationYamlShouldContainAuthOIDCWithIssuerAndExternalKeycloakUrl",
			Values: map[string]string{
				"identity.enabled":                                       "true",
				"global.identity.auth.enabled":                           "true",
				"global.identity.auth.publicIssuerUrl":                   "https://public-issuer-url.com/realms/camunda",
				"global.identity.keycloak.url.protocol":                  "https",
				"global.identity.keycloak.url.host":                      "keycloak.prod.svc.cluster.local",
				"global.identity.keycloak.url.port":                      "8443",
				"global.identity.keycloak.auth.adminUser":                "admin",
				"global.identity.keycloak.auth.secret.existingSecret":    "kc-secret",
				"global.identity.keycloak.auth.secret.existingSecretKey": "password",
				"orchestration.security.authentication.method":           "oidc",
				"orchestration.security.authentication.oidc.redirectUrl": "https://redirect.com/orchestration",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.authorization-uri": "https://public-issuer-url.com/realms/camunda/protocol/openid-connect/auth",
				"configmapApplication.camunda.security.authentication.oidc.jwk-set-uri":       "https://keycloak.prod.svc.cluster.local:8443/auth/realms/camunda-platform/protocol/openid-connect/certs",
				"configmapApplication.camunda.security.authentication.oidc.token-uri":         "https://keycloak.prod.svc.cluster.local:8443/auth/realms/camunda-platform/protocol/openid-connect/token",
				"configmapApplication.camunda.security.authentication.oidc.redirect-uri":      "https://redirect.com/orchestration/sso-callback",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCWithIssuerUrlAndKeycloakDisabled",
			Values: map[string]string{
				"identity.enabled":                                       "false",
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
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)
				var application orchestrationApplication
				require.NoError(t, yaml.Unmarshal([]byte(configMap.Data["application.yaml"]), &application))
				require.Equal(t, "custom-groups", application.Camunda.Security.Authentication.OIDC.GroupsClaim)
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
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)
				var application orchestrationApplication
				require.NoError(t, yaml.Unmarshal([]byte(configMap.Data["application.yaml"]), &application))
				require.True(t, application.Camunda.Security.Authentication.Basic.AllowUnauthenticatedAPIAccess)
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

// requireNestedKeyAbsent walks a decoded YAML document by key path and fails the
// test if the full path resolves to a present key, regardless of indentation/formatting.
func requireNestedKeyAbsent(t *testing.T, root map[string]any, path ...string) {
	current := root
	for i, key := range path {
		value, ok := current[key]
		if !ok {
			return
		}
		if i == len(path)-1 {
			require.Failf(t, "unexpected property present", "path %q should not be set, got: %#v", strings.Join(path, "."), value)
			return
		}
		nested, ok := value.(map[string]any)
		if !ok {
			return
		}
		current = nested
	}
}

func (s *ConfigmapTemplateTest) TestRDBMSDoesNotUseExporterProperties() {
	rdbmsValues := map[string]string{
		"orchestration.exporters.rdbms.enabled":                              "true",
		"orchestration.data.secondaryStorage.rdbms.url":                      "jdbc:postgresql://localhost:5432/camunda",
		"orchestration.data.secondaryStorage.rdbms.username":                 "camunda",
		"orchestration.data.secondaryStorage.rdbms.secret.existingSecret":    "camunda-rdbms-credentials",
		"orchestration.data.secondaryStorage.rdbms.secret.existingSecretKey": "password",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestConfigmapShouldNotUseRDBMSExporterProperties",
			Values: rdbmsValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)

				var applicationYaml map[string]any
				require.NoError(t, yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &applicationYaml))

				requireNestedKeyAbsent(t, applicationYaml, "zeebe", "broker", "exporters", "rdbms")
				requireNestedKeyAbsent(t, applicationYaml, "camunda", "data", "exporters", "rdbms")
			},
		},
		{
			Name:     "TestStatefulSetShouldNotUseRDBMSExporterEnvVarNames",
			Template: "templates/orchestration/statefulset.yaml",
			Values:   rdbmsValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "ZEEBE_BROKER_EXPORTERS_RDBMS_",
					"the RDBMS password must be injected as VALUES_ORCHESTRATION_DATA_SECONDARYSTORAGE_RDBMS_PASSWORD, not a ZEEBE_BROKER_EXPORTERS_RDBMS_* env var")
				require.NotContains(t, output, "CAMUNDA_DATA_EXPORTERS_RDBMS_",
					"the RDBMS password must be injected as VALUES_ORCHESTRATION_DATA_SECONDARYSTORAGE_RDBMS_PASSWORD, not a CAMUNDA_DATA_EXPORTERS_RDBMS_* env var")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestHasLegacyElasticsearchExporter() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestLegacyESExporterAbsentWhenRdbmsAndOptimizeButNoElasticsearch",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                      "rdbms",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                      "true",
				"optimize.database.opensearch.enabled":  "true",
				"optimize.database.opensearch.url.host": "opensearch.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter",
					"rdbms+optimize without elasticsearch must not render legacy ES exporter")
			},
		},
		{
			Name: "TestLegacyESExporterPresentWhenRdbmsAndOptimizeDatabaseElasticsearchOnly",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                      "rdbms",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                         "true",
				"optimize.database.elasticsearch.enabled":  "true",
				"optimize.database.elasticsearch.external": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter",
					"rdbms+optimize with optimize.database.elasticsearch.enabled must render legacy ES exporter")
			},
		},
		{
			Name: "TestLegacyESExporterAbsentForSupport32901CustomerConfig",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                      "rdbms",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.exporters.zeebe.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                      "true",
				"optimize.database.opensearch.enabled":  "true",
				"optimize.database.opensearch.url.host": "opensearch.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter",
					"SUPPORT-32901: rdbms+optimize+zeebe with OpenSearch must not render legacy ES exporter")
				require.Contains(t, output, "io.camunda.zeebe.exporter.opensearch.OpensearchExporter",
					"SUPPORT-32901: OS exporter must still render to feed Optimize")
			},
		},
		{
			Name: "TestLegacyESExporterAbsentWhenOnlyRdbmsNoOptimize",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                      "rdbms",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter",
					"rdbms without optimize must not render legacy ES exporter")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestLegacyExporterMultiRegionGate() {
	verifyExporter := func(exporterKey, exporterClass string, expected bool) func(t *testing.T, output string, err error) {
		return func(t *testing.T, output string, err error) {
			require.NoError(t, err)

			var configmap corev1.ConfigMap
			helm.UnmarshalK8SYaml(t, output, &configmap)

			var applicationYaml map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &applicationYaml))
			require.Contains(t, configmap.Data["application.yaml"], "autoconfigure-camunda-exporter: true")
			require.NotContains(t, configmap.Data["application.yaml"], "io.camunda.exporter.CamundaExporter")
			if expected {
				require.Contains(t, configmap.Data["application.yaml"], exporterClass)
			} else {
				requireNestedKeyAbsent(t, applicationYaml, "zeebe", "broker", "exporters", exporterKey)
			}
		}
	}

	elasticsearchValues := map[string]string{
		"global.multiregion.regions":               "2",
		"global.multiregion.regionId":              "0",
		"orchestration.profiles.broker":            "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"optimize.enabled":                         "true",
		"optimize.database.elasticsearch.enabled":  "true",
	}
	openSearchValues := map[string]string{
		"global.multiregion.regions":               "2",
		"global.multiregion.regionId":              "0",
		"orchestration.profiles.broker":            "true",
		"orchestration.data.secondaryStorage.type": "opensearch",
		"optimize.enabled":                         "true",
		"optimize.database.opensearch.enabled":     "true",
		"optimize.database.opensearch.url.host":    "opensearch.example.com",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:     "Multi-region omits the implicit Elasticsearch exporter",
			Values:   elasticsearchValues,
			Verifier: verifyExporter("elasticsearch", "io.camunda.zeebe.exporter.ElasticsearchExporter", false),
		},
		{
			Name: "Multi-region keeps the explicitly enabled Elasticsearch exporter",
			Values: mergeValues(elasticsearchValues, map[string]string{
				"orchestration.exporters.zeebe.enabled": "true",
			}),
			Verifier: verifyExporter("elasticsearch", "io.camunda.zeebe.exporter.ElasticsearchExporter", true),
		},
		{
			Name:     "Multi-region omits the implicit OpenSearch exporter",
			Values:   openSearchValues,
			Verifier: verifyExporter("opensearch", "io.camunda.zeebe.exporter.opensearch.OpensearchExporter", false),
		},
		{
			Name: "Multi-region keeps the explicitly enabled OpenSearch exporter",
			Values: mergeValues(openSearchValues, map[string]string{
				"orchestration.exporters.zeebe.enabled": "true",
			}),
			Verifier: verifyExporter("opensearch", "io.camunda.zeebe.exporter.opensearch.OpensearchExporter", true),
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestLegacyZeebeExporterReplicas() {
	testCases := []testhelpers.TestCase{
		{
			Name: "ESExporterReplicasInheritIndexReplicasByDefault",
			Values: map[string]string{
				"orchestration.exporters.zeebe.enabled":   "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter")
				require.Contains(t, output, "numberOfReplicas: \"1\"",
					"legacy ES exporter replicas must default to orchestration.index.replicas (1)")
			},
		},
		{
			Name: "ESExporterReplicasInheritCustomIndexReplicas",
			Values: map[string]string{
				"orchestration.exporters.zeebe.enabled":   "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
				"orchestration.index.replicas":            "3",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "numberOfReplicas: \"3\"",
					"legacy ES exporter replicas must inherit a custom orchestration.index.replicas")
			},
		},
		{
			Name: "ESExporterReplicasIndependentOverride",
			Values: map[string]string{
				"orchestration.exporters.zeebe.enabled":   "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
				"orchestration.index.replicas":            "3",
				"orchestration.exporters.zeebe.replicas":  "2",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "numberOfReplicas: \"2\"",
					"orchestration.exporters.zeebe.replicas must override the inherited value")
			},
		},
		{
			Name: "ESExporterReplicasExplicitZero",
			Values: map[string]string{
				"orchestration.exporters.zeebe.enabled":   "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
				"orchestration.exporters.zeebe.replicas":  "0",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "numberOfReplicas: \"0\"",
					"an explicit orchestration.exporters.zeebe.replicas of 0 must be honored")
			},
		},
		{
			Name: "OSExporterReplicas",
			Values: map[string]string{
				"orchestration.exporters.zeebe.enabled":  "true",
				"orchestration.exporters.zeebe.replicas": "5",
				"optimize.enabled":                       "true",
				"optimize.database.opensearch.enabled":   "true",
				"optimize.database.opensearch.url.host":  "opensearch.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "io.camunda.zeebe.exporter.opensearch.OpensearchExporter")
				require.Contains(t, output, "numberOfReplicas: \"5\"",
					"legacy OS exporter must render orchestration.exporters.zeebe.replicas")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
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

func (s *ConfigmapTemplateTest) TestBundledOperateTasklistZeebeClientTLS() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestPlaintextByDefault",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, 2, strings.Count(output, "secure: false"),
					"both bundled Operate and Tasklist zeebe blocks must declare plaintext explicitly")
				require.NotContains(t, output, "secure: true")
				require.NotContains(t, output, "certificatePath:")
			},
		},
		{
			Name: "TestSecureWhenGrpcTLSEnabled",
			Values: map[string]string{
				"global.tls.orchestration.grpc.enabled":                    "true",
				"global.tls.orchestration.grpc.cert.secret.existingSecret": "grpc-pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, 2, strings.Count(output, "secure: true"))
				require.NotContains(t, output, "secure: false")
			},
		},
		{
			Name: "TestCertificatePathFromCaBundleWhenSecure",
			Values: map[string]string{
				"global.tls.orchestration.grpc.enabled":                    "true",
				"global.tls.orchestration.grpc.cert.secret.existingSecret": "grpc-pem",
				"global.tls.caBundle.secret.existingSecret":                "my-ca",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, 2, strings.Count(output, "certificatePath: /etc/camunda/tls/ca.crt"))
			},
		},
		{
			Name: "TestCertificatePathOmittedWithoutCaBundle",
			Values: map[string]string{
				"global.tls.orchestration.grpc.enabled":                    "true",
				"global.tls.orchestration.grpc.cert.secret.existingSecret": "grpc-pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "certificatePath:",
					"the client rejects an EMPTY caCertificatePath; absent means JVM default truststore")
			},
		},
		{
			Name: "TestCertificatePathOmittedWhenPlaintext",
			Values: map[string]string{
				"global.tls.caBundle.secret.existingSecret": "my-ca",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "certificatePath:")
			},
		},
		{
			Name:        "TestSecureWhenGrpcTLSEnabledViaOrchestrationEnv",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-orchestration-grpc-tls-env-only.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, 2, strings.Count(output, "secure: true"),
					"an orchestration.env toggle must drive the bundled clients too")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestNumberedModeConfigurationCompatibility() {
	testCases := []testhelpers.TestCase{
		{
			Name: "DefaultModeUsesPlainNodeIDAndSingleRegionAdvertisedHost",
			Values: map[string]string{
				"orchestration.profiles.broker": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "${K8S_NAME##*-} * 1 + 0")
				require.Contains(t, output, "node-id: \"${VALUES_ORCHESTRATION_NODE_ID:}\"")
				require.Contains(t, output, "advertisedHost: \"${K8S_NAME}.${K8S_SERVICE_NAME}\"")
				require.NotContains(t, output, "CAMUNDA_CLUSTER_ZONE")
				require.NotContains(t, output, "scheme: ZONE_AWARE")
			},
		},
		{
			Name: "ExplicitNumberedModeUsesPlainNodeIDAndMultiRegionAdvertisedHost",
			Values: map[string]string{
				"orchestration.multiregion.mode": "numbered",
				"global.multiregion.regions":     "2",
				"global.multiregion.regionId":    "1",
				"orchestration.profiles.broker":  "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "${K8S_NAME##*-} * 2 + 1")
				require.Contains(t, output, "node-id: \"${VALUES_ORCHESTRATION_NODE_ID:}\"")
				require.Contains(t, output, "advertisedHost: \"${K8S_NAME}.${K8S_SERVICE_NAME}.${K8S_NAMESPACE}.svc\"")
				require.NotContains(t, output, "CAMUNDA_CLUSTER_ZONE")
				require.NotContains(t, output, "scheme: ZONE_AWARE")
			},
		},
		{
			Name: "NumberedCustomConfigurationRemainsAuthoritative",
			Values: map[string]string{
				"orchestration.multiregion.mode": "numbered",
				"orchestration.configuration":    "camunda:\n  cluster:\n    partition-count: 7\n",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "partition-count: 7")
				require.NotContains(t, output, "partition-count: \"3\"")
				require.Contains(t, output, "${K8S_NAME##*-} * 1 + 0")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestZonedConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainZoneAwareConfiguration",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "3",
				"orchestration.multiregion.zones[1].numberOfReplicas": "3",
				"orchestration.multiregion.zones[1].priority":         "50",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "size: \"5\"")
				require.Contains(t, output, "replication-factor: \"5\"")
				require.Contains(t, output, "scheme: ZONE_AWARE")
				require.Contains(t, output, "name: \"region-a\"")
				require.Contains(t, output, "name: \"region-b\"")
				require.Contains(t, output, "VALUES_ORCHESTRATION_NODE_ID:-${K8S_NAME##*-}")
				require.Contains(t, output, "node-id: \"${VALUES_ORCHESTRATION_NODE_ID:}\"")
				// The generated topology includes every zone. Deployments across Kubernetes
				// clusters can override these addresses with externally resolvable endpoints.
				require.Contains(t, output, "initial-contact-points:")
				require.Contains(t, output, "camunda-platform-test-zeebe-region-a-0.camunda-platform-test-zeebe-region-a:26502")
				require.Contains(t, output, "camunda-platform-test-zeebe-region-b-0.camunda-platform-test-zeebe-region-b:26502")
			},
		},
		{
			Name: "TestZonedNodeIdIsTheIndexInsideTheZone",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-b",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "3",
				"orchestration.multiregion.zones[1].numberOfReplicas": "3",
				"orchestration.multiregion.zones[1].priority":         "50",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// Brokers are addressed as "<zone>_<node-id>", so region-b's three
				// Pods are region-b_0, region-b_1 and region-b_2 whatever region-a
				// declares before it. No cluster-wide offset applies.
				require.Contains(t, output, "VALUES_ORCHESTRATION_NODE_ID:-${K8S_NAME##*-}")
				require.NotContains(t, output, "${K8S_NAME##*-} +")
				require.NotContains(t, output, "${K8S_NAME##*-} *")
				require.Contains(t, output, "node-id: \"${VALUES_ORCHESTRATION_NODE_ID:}\"")
				require.Contains(t, output, "size: \"5\"")
			},
		},
		{
			Name: "TestSingleZoneStillRendersItsInitialContactPoints",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// A single-zone topology can use the generated in-cluster addresses.
				require.Contains(t, output, "initial-contact-points:")
				require.Contains(t, output, "camunda-platform-test-zeebe-region-a-0.camunda-platform-test-zeebe-region-a:26502")
				require.Contains(t, output, "camunda-platform-test-zeebe-region-a-1.camunda-platform-test-zeebe-region-a:26502")
				// Two brokers, so two contact points. The count comes from the zone
				// list, not from `orchestration.clusterSize`, which is still on its
				// default of three and would have produced a third.
				require.NotContains(t, output, "camunda-platform-test-zeebe-region-a-2.camunda-platform-test-zeebe-region-a:26502")
				require.Contains(t, output, "size: \"2\"")
			},
		},
		{
			Name: "TestZonedModeDoesNotEnableLegacyElasticsearchExporter",
			Values: map[string]string{
				"orchestration.multiregion.mode":                                "zoned",
				"orchestration.multiregion.zone":                                "region-a",
				"orchestration.multiregion.zones[0].name":                       "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[0].numberOfReplicas":           "2",
				"orchestration.multiregion.zones[0].priority":                   "100",
				"orchestration.multiregion.zones[1].name":                       "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[1].numberOfReplicas":           "1",
				"orchestration.multiregion.zones[1].priority":                   "50",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter")
			},
		},
		{
			Name: "TestZonedModeDoesNotEnableLegacyOpenSearchExporter",
			Values: map[string]string{
				"orchestration.multiregion.mode":                                "zoned",
				"orchestration.multiregion.zone":                                "region-a",
				"orchestration.multiregion.zones[0].name":                       "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[0].numberOfReplicas":           "2",
				"orchestration.multiregion.zones[0].priority":                   "100",
				"orchestration.multiregion.zones[1].name":                       "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[1].numberOfReplicas":           "1",
				"orchestration.multiregion.zones[1].priority":                   "50",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                     "true",
				"optimize.database.opensearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "io.camunda.zeebe.exporter.opensearch.OpensearchExporter")
			},
		},
		{
			// A single zone is one cluster, like a single region: it skews leaders
			// inside a region rather than spreading across them, so it keeps the
			// exporter that a genuinely spread cluster has to give up.
			Name: "TestSingleZoneZonedModeKeepsTheElasticsearchExporter",
			Values: map[string]string{
				"orchestration.multiregion.mode":                                "zoned",
				"orchestration.multiregion.zone":                                "region-a",
				"orchestration.multiregion.zones[0].name":                       "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[0].numberOfReplicas":           "2",
				"orchestration.multiregion.zones[0].priority":                   "100",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter")
			},
		},
		{
			// Control for the two cases above: same inputs, numbered mode. Without it
			// they pass whether or not the zoned guard exists, since the exporter is
			// also absent when Optimize does not ask for that database.
			Name: "TestNumberedSingleRegionStillEnablesTheElasticsearchExporter",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestZonedModeRejectsNumberedRegionSettings() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestZonedModeRejectsNumberedRegions",
			Values: map[string]string{
				"orchestration.multiregion.mode":    "zoned",
				"orchestration.multiregion.regions": "2",
				"orchestration.profiles.broker":     "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.multiregion.regions and orchestration.multiregion.regionId cannot be used with zoned mode",
			},
		},
		{
			Name:                    "TestZonedModeRejectsAClusterSizeItDerives",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=6"},
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[1].numberOfReplicas": "1",
				"orchestration.multiregion.zones[1].priority":         "50",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.clusterSize is 6 but orchestration.multiregion.zones sums to 4 brokers",
			},
		},
		{
			Name:                    "TestZonedModeRejectsAReplicationFactorItDerives",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.replicationFactor=4"},
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.replicationFactor is 4 but orchestration.multiregion.zones sums to 2 replicas",
			},
		},
		{
			Name: "TestZonesWithoutZonedModeAreRejected",
			Values: map[string]string{
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "require orchestration.multiregion.mode=zoned",
			},
		},
		{
			Name: "TestZonedModeRejectsDuplicateZoneNames",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-a",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[1].numberOfReplicas": "1",
				"orchestration.multiregion.zones[1].priority":         "50",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "declares \"region-a\" twice",
			},
		},
		{
			Name: "TestZonedModeRejectsMoreReplicasThanBrokers",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "3",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "asks for 3 replicas on 1 brokers",
			},
		},
		{
			Name: "TestZonedModeRejectsAnUndeclaredZone",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-c",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.multiregion.zone \"region-c\" is not declared in orchestration.multiregion.zones",
			},
		},
		{
			// A release renders exactly one zone, so the zone it belongs to is not
			// optional: without it the broker count and the node IDs are undefined.
			Name: "TestZonedModeRequiresTheZone",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.multiregion.zone must name the zone this release is deployed to",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestCamundaExporterHonorsAutoconfigureFromExtraConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestDisabledViaExtraConfigurationSuppressesBlock",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-camunda-exporter-disable-via-extraconfig.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "autoconfigure-camunda-exporter: false")
				require.NotContains(t, output, "camundaexporter:")
			},
		},
		{
			Name:        "TestNonImportedOverrideIgnoredSoDefaultKeepsBlock",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-camunda-exporter-disable-via-extraconfig-noimport.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "autoconfigure-camunda-exporter: true")
				require.NotContains(t, output, "camundaexporter:")
			},
		},
		{
			Name:        "TestScalarIntermediateDoesNotAbortRender",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-camunda-exporter-scalar-intermediate.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "autoconfigure-camunda-exporter: true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

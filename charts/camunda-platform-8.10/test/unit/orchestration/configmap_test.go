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
	camunda "camunda-platform/test/unit/common"
	"camunda-platform/test/unit/testhelpers"
	"camunda-platform/test/unit/utils"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigmapLegacyTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigmapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigmapLegacyTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/configmap.yaml"},
	})
}

func TestGoldenConfigmapWithLog4j2(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "configmap-log4j2",
		Templates:      []string{"templates/orchestration/configmap.yaml"},
		SetValues:      map[string]string{"orchestration.log4j2": "<xml>\n</xml>"},
	})
}

func TestGoldenConfigmapWithAuthorizationsEnabled(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "configmap-authorizations",
		Templates:      []string{"templates/orchestration/configmap.yaml"},
		SetValues:      map[string]string{"global.authorizations.enabled": "true"},
	})
}

func TestGoldenConfigmapWithHistoryRetentionEnabled(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "configmap-retention",
		Templates:      []string{"templates/orchestration/configmap.yaml"},
		SetValues:      map[string]string{"orchestration.history.retention.enabled": "true"},
	})
}

func TestGoldenConfigmapWithRDBMSEnabled(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "configmap-rdbms",
		Templates:      []string{"templates/orchestration/configmap.yaml"},
		SetValues: map[string]string{
			"orchestration.exporters.rdbms.enabled":                              "true",
			"orchestration.data.secondaryStorage.rdbms.url":                      "jdbc:postgresql://rdbms:5432/camunda",
			"orchestration.data.secondaryStorage.rdbms.username":                 "camunda",
			"orchestration.data.secondaryStorage.rdbms.aws.enabled":              "true",
			"orchestration.data.secondaryStorage.rdbms.secret.existingSecret":    "camunda-rdbms-credentials",
			"orchestration.data.secondaryStorage.rdbms.secret.existingSecretKey": "password",
		},
	})
}

func (s *ConfigmapLegacyTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestExportersShouldBeEmptyByDefault",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				var configmapApplication camunda.OrchestrationApplicationYAML
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				helm.UnmarshalK8SYaml(s.T(), configmap.Data["application.yaml"], &configmapApplication)

				// CamundaExporter is auto-registered via autoconfigure-camunda-exporter: true;
				// the legacy zeebe.broker.exporters.camundaexporter entry must not be present.
				s.Require().Empty(configmapApplication.Zeebe.Broker.Exporters.CamundaExporter.ClassName)

				s.Require().NotContains(configmap.Data["application.yaml"], "exporters: {}")
			},
		},
		{
			Name:   "TestCustomHistorySettingsUseUnifiedElasticsearchConfig",
			Values: customHistoryValues("elasticsearch"),
			Verifier: func(t *testing.T, output string, err error) {
				assertCustomHistorySettings(t, output, false)
			},
		},
		{
			Name:   "TestCustomHistorySettingsUseUnifiedOpenSearchConfig",
			Values: customHistoryValues("opensearch"),
			Verifier: func(t *testing.T, output string, err error) {
				assertCustomHistorySettings(t, output, true)
			},
		},
		{
			Name:   "TestCamundaExporterAutoconfigurationEnabledByDefault",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				assertCamundaExporterAutoconfiguration(t, output, true)
			},
		},
		{
			Name: "TestCamundaExporterAutoconfigurationCanBeDisabled",
			Values: map[string]string{
				"orchestration.exporters.camunda.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				assertCamundaExporterAutoconfiguration(t, output, false)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func customHistoryValues(secondaryStorageType string) map[string]string {
	return map[string]string{
		"orchestration.data.secondaryStorage.type":        secondaryStorageType,
		"orchestration.history.elsRolloverDateFormat":     "yyyy-MM",
		"orchestration.history.rolloverInterval":          "2d",
		"orchestration.history.rolloverBatchSize":         "321",
		"orchestration.history.waitPeriodBeforeArchiving": "3h",
		"orchestration.history.delayBetweenRuns":          "4000",
		"orchestration.history.maxDelayBetweenRuns":       "12000",
	}
}

func assertCamundaExporterAutoconfiguration(t *testing.T, output string, expected bool) {
	var configmap corev1.ConfigMap
	var application camunda.OrchestrationApplicationYAML
	helm.UnmarshalK8SYaml(t, output, &configmap)
	applicationYaml := configmap.Data["application.yaml"]
	require.NoError(t, yaml.Unmarshal([]byte(applicationYaml), &application))

	require.Contains(t, applicationYaml, fmt.Sprintf("autoconfigure-camunda-exporter: %t", expected))
	require.Equal(t, expected, application.Camunda.Data.SecondaryStorage.AutoconfigureCamundaExporter)
}

func assertCustomHistorySettings(t *testing.T, output string, openSearch bool) {
	var configmap corev1.ConfigMap
	var application camunda.OrchestrationApplicationYAML
	helm.UnmarshalK8SYaml(t, output, &configmap)
	require.NoError(t, yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &application))
	require.NotContains(t, configmap.Data["application.yaml"], "camundaexporter:")

	history := application.Camunda.Data.SecondaryStorage.Elasticsearch.History
	if openSearch {
		history = application.Camunda.Data.SecondaryStorage.OpenSearch.History
	}
	require.Equal(t, "yyyy-MM", history.ElsRolloverDateFormat)
	require.Equal(t, "2d", history.RolloverInterval)
	require.Equal(t, 321, history.RolloverBatchSize)
	require.Equal(t, "3h", history.WaitPeriodBeforeArchiving)
	require.Equal(t, 4000, history.DelayBetweenRuns)
	require.Equal(t, 12000, history.MaximumDelayBetweenRuns)
}

func (s *ConfigmapLegacyTemplateTest) TestRequestBodySizeConfiguresUploadLimits() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestRequestBodySizeConfiguresUploadLimits",
			Values: map[string]string{
				"global.config.requestBodySize": "50MB",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				applicationYaml := configmap.Data["application.yaml"]

				require.Contains(t, applicationYaml, "max-http-form-post-size: \"50MB\"")
				require.Contains(t, applicationYaml, "max-file-size: \"50MB\"")
				require.Contains(t, applicationYaml, "max-request-size: \"50MB\"")
				require.NotContains(t, applicationYaml, "max-message-size: \"50MB\"")
				require.NotContains(t, applicationYaml, "maxMessageSize: \"50MB\"")
				require.Equal(t, 0, strings.Count(applicationYaml, "max-message-size:"))
				require.Equal(t, 0, strings.Count(applicationYaml, "maxMessageSize:"))
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapLegacyTemplateTest) TestAzureDocumentStoreConnectionStringBinding() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestAzureDocumentStoreConnectionStringBindingWhenActive",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-azure-documentstore.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				var application struct {
					Camunda struct {
						Document struct {
							DefaultStoreID string `json:"default-store-id"`
							Azure          map[string]struct {
								ConnectionString string `json:"connection-string"`
							} `json:"azure"`
						} `json:"document"`
					} `json:"camunda"`
				}
				helm.UnmarshalK8SYaml(t, configmap.Data["application.yaml"], &application)

				require.Equal(t, "az1", application.Camunda.Document.DefaultStoreID)
				require.Equal(t, "${VALUES_DOCUMENT_STORE_AZURE_CONNECTION_STRING:}", application.Camunda.Document.Azure["az1"].ConnectionString)
			},
		},
		{
			Name: "TestAzureDocumentStoreConnectionStringOmittedWhenInactive",
			ValuesFiles: []string{
				filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-azure-documentstore.yaml"),
			},
			Values: map[string]string{
				"global.documentStore.activeStoreId": "aws",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				var application struct {
					Camunda struct {
						Document struct {
							DefaultStoreID string `json:"default-store-id"`
							Azure          map[string]struct {
								ConnectionString string `json:"connection-string"`
							} `json:"azure"`
						} `json:"document"`
					} `json:"camunda"`
				}
				helm.UnmarshalK8SYaml(t, configmap.Data["application.yaml"], &application)

				require.Equal(t, "aws", application.Camunda.Document.DefaultStoreID)
				require.Empty(t, application.Camunda.Document.Azure)
			},
		},
		{
			Name: "TestAzureDocumentStoreConnectionStringOmittedWithoutExtraConfiguration",
			Values: map[string]string{
				"global.documentStore.activeStoreId":                                        "azure",
				"global.documentStore.type.azure.connectionString.secret.existingSecret":    "azure-document-store",
				"global.documentStore.type.azure.connectionString.secret.existingSecretKey": "connection-string",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				var application struct {
					Camunda struct {
						Document struct {
							DefaultStoreID string         `json:"default-store-id"`
							Azure          map[string]any `json:"azure"`
						} `json:"document"`
					} `json:"camunda"`
				}
				helm.UnmarshalK8SYaml(t, configmap.Data["application.yaml"], &application)

				require.Empty(t, application.Camunda.Document.DefaultStoreID)
				require.Empty(t, application.Camunda.Document.Azure)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapLegacyTemplateTest) TestZeebeMaxMessageSizeUsesEngineDefault() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestZeebeMaxMessageSizeUsesEngineDefaultWithDefaultValues",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				applicationYaml := configmap.Data["application.yaml"]

				// the chart must never render a Zeebe message-size key, keeping the engine's 4MB default
				require.Equal(t, 0, strings.Count(applicationYaml, "max-message-size:"))
				require.Equal(t, 0, strings.Count(applicationYaml, "maxMessageSize:"))
			},
		},
		{
			Name: "TestZeebeMaxMessageSizeUsesEngineDefaultWithRequestBodySizeOverride",
			Values: map[string]string{
				"global.config.requestBodySize": "50MB",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				applicationYaml := configmap.Data["application.yaml"]

				// requestBodySize must not leak into a Zeebe message-size key
				require.Equal(t, 0, strings.Count(applicationYaml, "max-message-size:"))
				require.Equal(t, 0, strings.Count(applicationYaml, "maxMessageSize:"))
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapLegacyTemplateTest) TestExtraConfigurationSpringImport() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestExtraConfigWithSpringImportDefault",
			Values: map[string]string{
				"orchestration.extraConfiguration[0].file":    "custom-spring.yaml",
				"orchestration.extraConfiguration[0].content": "some: config",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// spring.config.import should include the file
				s.Require().Contains(applicationYaml, "optional:file:/usr/local/camunda/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				// File content should be in ConfigMap
				s.Require().Contains(configmap.Data["custom-spring.yaml"], "some: config",
					"File content should be present in ConfigMap")
			},
		},
		{
			Name: "TestExtraConfigWithSpringImportFalse",
			Values: map[string]string{
				"orchestration.extraConfiguration[0].file":         "log4j2-spring.xml",
				"orchestration.extraConfiguration[0].springImport": "false",
				"orchestration.extraConfiguration[0].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// spring.config.import should NOT include the file
				s.Require().NotContains(applicationYaml, "log4j2-spring.xml",
					"File with springImport: false should not be in spring.config.import")
				// spring.config.import block should not be rendered
				s.Require().NotContains(applicationYaml, "optional:file:",
					"spring.config.import block should not be rendered when all entries have springImport: false")
				// File content should still be in ConfigMap
				s.Require().Contains(configmap.Data["log4j2-spring.xml"], "<Configuration/>",
					"File content should be present in ConfigMap even with springImport: false")
			},
		},
		{
			Name: "TestExtraConfigMixedSpringImport",
			Values: map[string]string{
				"orchestration.extraConfiguration[0].file":         "custom-spring.yaml",
				"orchestration.extraConfiguration[0].content":      "some: config",
				"orchestration.extraConfiguration[1].file":         "log4j2-spring.xml",
				"orchestration.extraConfiguration[1].springImport": "false",
				"orchestration.extraConfiguration[1].content":      "<Configuration/>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				// Only custom-spring.yaml should be in spring.config.import
				s.Require().Contains(applicationYaml, "optional:file:/usr/local/camunda/config/custom-spring.yaml",
					"File without springImport should be included in spring.config.import")
				s.Require().NotContains(applicationYaml, "log4j2-spring.xml",
					"File with springImport: false should not be in spring.config.import")
				// Both files should be in ConfigMap
				s.Require().Contains(configmap.Data["custom-spring.yaml"], "some: config",
					"First file content should be present in ConfigMap")
				s.Require().Contains(configmap.Data["log4j2-spring.xml"], "<Configuration/>",
					"Second file content should be present in ConfigMap even with springImport: false")
			},
		},
		{
			Name:   "TestLog4j2KeyNotEmittedWhenUnset",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Equal(0, strings.Count(output, "log4j2.xml: |"),
					"log4j2.xml should not be emitted at all when orchestration.log4j2 is unset")
			},
		},
		{
			Name: "TestLog4j2KeyEmittedOnceFromDeprecatedKey",
			Values: map[string]string{
				"orchestration.log4j2": "<Configuration>deprecated</Configuration>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Equal(1, strings.Count(output, "log4j2.xml: |"),
					"log4j2.xml should be emitted exactly once")

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["log4j2.xml"], "<Configuration>deprecated</Configuration>",
					"orchestration.log4j2 content should be used when extraConfiguration does not supply the file")
			},
		},
		{
			Name: "TestLog4j2KeyEmittedOnceFromExtraConfiguration",
			Values: map[string]string{
				"orchestration.extraConfiguration[0].file":         "log4j2.xml",
				"orchestration.extraConfiguration[0].springImport": "false",
				"orchestration.extraConfiguration[0].content":      "<Configuration>operator</Configuration>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Equal(1, strings.Count(output, "log4j2.xml: |"),
					"log4j2.xml must not be defined twice in ConfigMap data")

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["log4j2.xml"], "<Configuration>operator</Configuration>",
					"extraConfiguration content should be used")
			},
		},
		{
			Name: "TestLog4j2ExtraConfigurationWinsOverDeprecatedKey",
			Values: map[string]string{
				"orchestration.log4j2":                             "<Configuration>deprecated</Configuration>",
				"orchestration.extraConfiguration[0].file":         "log4j2.xml",
				"orchestration.extraConfiguration[0].springImport": "false",
				"orchestration.extraConfiguration[0].content":      "<Configuration>operator</Configuration>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Equal(1, strings.Count(output, "log4j2.xml: |"),
					"log4j2.xml must not be defined twice when both sources set it")

				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["log4j2.xml"], "<Configuration>operator</Configuration>",
					"extraConfiguration should win over the deprecated orchestration.log4j2")
				s.Require().NotContains(configmap.Data["log4j2.xml"], "deprecated",
					"deprecated orchestration.log4j2 content should be suppressed")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapLegacyTemplateTest) TestExtraConfigurationDoesNotSuppressChartRenderedProperties() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestChartStillRendersItsOwnValueAlongsideExtraConfiguration",
			Values: map[string]string{
				"orchestration.extraConfiguration[0].file":    "operator-override.yaml",
				"orchestration.extraConfiguration[0].content": "camunda:\n  data:\n    snapshot-period: 7m\n",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)

				applicationYaml := configmap.Data["application.yaml"]
				s.Require().Contains(applicationYaml, "optional:file:/usr/local/camunda/config/operator-override.yaml",
					"the operator file must be imported by the chart's application.yaml")
				s.Require().Contains(applicationYaml, "snapshot-period: \"5m\"",
					"the chart must keep rendering its own default; the import outranks it at runtime")
				s.Require().Contains(configmap.Data["operator-override.yaml"], "snapshot-period: 7m",
					"the operator value must be delivered as its own imported document")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

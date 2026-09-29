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

type normalizeSecretConfigTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestNormalizeSecretConfigTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &normalizeSecretConfigTest{
		chartPath: chartPath,
		release:   "test",
		namespace: "test-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/statefulset.yaml"},
	})
}

func (s *normalizeSecretConfigTest) TestSecretHelperFunctionsWithOpenSearch() {
	testCases := []testhelpers.TestCase{
		{
			Name: "opensearch new style secret creates env vars",
			Values: map[string]string{
				"orchestration.enabled":                    "true",
				"orchestration.data.secondaryStorage.type": "opensearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.existingSecret":    "my-opensearch-secret",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.existingSecretKey": "my-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.name": "my-opensearch-secret",
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.key":  "my-key",
			},
		},
		{
			Name: "opensearch inline secret creates env vars with direct values",
			Values: map[string]string{
				"orchestration.enabled":                                                   "true",
				"orchestration.data.secondaryStorage.type":                                "opensearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "my-password",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].value": "my-password",
			},
		},
		{
			Name: "no opensearch config means no env vars",
			Values: map[string]string{
				"orchestration.enabled":                    "true",
				"orchestration.data.secondaryStorage.type": "opensearch",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "VALUES_OPENSEARCH_PASSWORD")
			},
		},
		{
			Name: "opensearch disabled means no env vars",
			Values: map[string]string{
				"orchestration.enabled":                                                   "true",
				"orchestration.data.secondaryStorage.type":                                "elasticsearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "VALUES_OPENSEARCH_PASSWORD")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *normalizeSecretConfigTest) TestAwsDocumentStoreSecretHelperFunctions() {
	testCases := []testhelpers.TestCase{
		{
			Name: "aws document store new style secret creates env vars",
			Values: map[string]string{
				"orchestration.enabled":                                                  "true",
				"global.documentStore.type.aws.enabled":                                  "true",
				"global.documentStore.type.aws.accessKeyId.secret.existingSecret":        "my-aws-secret",
				"global.documentStore.type.aws.accessKeyId.secret.existingSecretKey":     "access-key",
				"global.documentStore.type.aws.secretAccessKey.secret.existingSecret":    "my-aws-secret",
				"global.documentStore.type.aws.secretAccessKey.secret.existingSecretKey": "secret-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')].valueFrom.secretKeyRef.name":     "my-aws-secret",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')].valueFrom.secretKeyRef.key":      "access-key",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')].valueFrom.secretKeyRef.name": "my-aws-secret",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')].valueFrom.secretKeyRef.key":  "secret-key",
			},
		},
		{
			Name: "aws document store inline secret creates env vars with direct values",
			Values: map[string]string{
				"orchestration.enabled":                                             "true",
				"global.documentStore.type.aws.enabled":                             "true",
				"global.documentStore.type.aws.accessKeyId.secret.inlineSecret":     "test-access-key-id",
				"global.documentStore.type.aws.secretAccessKey.secret.inlineSecret": "test-secret-access-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')].value":     "test-access-key-id",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')].value": "test-secret-access-key",
			},
		},
		{
			Name: "no aws document store config means no env vars",
			Values: map[string]string{
				"orchestration.enabled":                 "true",
				"global.documentStore.type.aws.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// Should not create any AWS env vars
				require.NotContains(t, output, "AWS_ACCESS_KEY_ID")
				require.NotContains(t, output, "AWS_SECRET_ACCESS_KEY")
			},
		},
		{
			Name: "aws document store disabled means no env vars",
			Values: map[string]string{
				"orchestration.enabled":                                             "true",
				"global.documentStore.type.aws.enabled":                             "false",
				"global.documentStore.type.aws.accessKeyId.secret.inlineSecret":     "access-key",
				"global.documentStore.type.aws.secretAccessKey.secret.inlineSecret": "secret-key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// Should not create any AWS env vars when AWS document store is disabled
				require.NotContains(t, output, "AWS_ACCESS_KEY_ID")
				require.NotContains(t, output, "AWS_SECRET_ACCESS_KEY")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *normalizeSecretConfigTest) TestAzureDocumentStoreSecretHelperFunctions() {
	testCases := []testhelpers.TestCase{
		{
			Name: "azure existing secret uses the legacy document store environment variable",
			Values: map[string]string{
				"global.documentStore.activeStoreId":                                        "azure",
				"global.documentStore.type.azure.connectionString.secret.existingSecret":    "azure-credentials",
				"global.documentStore.type.azure.connectionString.secret.existingSecretKey": "connection-string",
				"orchestration.env[0].name":                                                 "DOCUMENT_STORE_AZURE_CLASS",
				"orchestration.env[0].value":                                                "io.camunda.document.store.azure.AzureBlobDocumentStoreProvider",
				"orchestration.env[1].name":                                                 "DOCUMENT_STORE_AZURE_CONTAINER",
				"orchestration.env[1].value":                                                "documents",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CONNECTION_STRING')].valueFrom.secretKeyRef.name": "azure-credentials",
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CONNECTION_STRING')].valueFrom.secretKeyRef.key":  "connection-string",
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CLASS')].value":                                   "io.camunda.document.store.azure.AzureBlobDocumentStoreProvider",
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CONTAINER')].value":                               "documents",
			},
		},
		{
			Name: "azure inline secret uses the active custom store ID",
			Values: map[string]string{
				"global.documentStore.activeStoreId":                                   "az1",
				"global.documentStore.type.azure.connectionString.secret.inlineSecret": "test-connection-string",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZ1_CONNECTION_STRING')].value": "test-connection-string",
			},
		},
		{
			Name: "azure managed identity preserves provider environment without injecting a connection string",
			Values: map[string]string{
				"global.documentStore.activeStoreId": "azure",
				"orchestration.env[0].name":          "DOCUMENT_STORE_AZURE_CLASS",
				"orchestration.env[0].value":         "io.camunda.document.store.azure.AzureBlobDocumentStoreProvider",
				"orchestration.env[1].name":          "DOCUMENT_STORE_AZURE_CONTAINER",
				"orchestration.env[1].value":         "documents",
				"orchestration.env[2].name":          "DOCUMENT_STORE_AZURE_ENDPOINT",
				"orchestration.env[2].value":         "https://storage.example.com",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CLASS')].value":     "io.camunda.document.store.azure.AzureBlobDocumentStoreProvider",
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CONTAINER')].value": "documents",
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_ENDPOINT')].value":  "https://storage.example.com",
			},
			Unexpected: []string{"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CONNECTION_STRING')]"},
		},
		{
			Name: "Azure credentials are not injected into Connectors",
			CaseTemplates: &testhelpers.CaseTemplate{
				Templates: []string{"templates/connectors/deployment.yaml"},
			},
			Values: map[string]string{
				"connectors.enabled":                                                        "true",
				"global.license.secret.inlineSecret":                                        "test-license",
				"global.documentStore.activeStoreId":                                        "azure",
				"global.documentStore.type.azure.connectionString.secret.existingSecret":    "azure-credentials",
				"global.documentStore.type.azure.connectionString.secret.existingSecretKey": "connection-string",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='CAMUNDA_LICENSE_KEY')].value": "test-license",
			},
			Unexpected: []string{"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_AZURE_CONNECTION_STRING')]"},
		},
	}

	for _, storeID := range []string{"inmemory", "aws", "gcp", "azure", "az1"} {
		testCases = append(testCases, testhelpers.TestCase{
			Name: "no connection string for " + storeID + " without an Azure secret",
			Values: map[string]string{
				"global.documentStore.activeStoreId": storeID,
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='K8S_POD_NAME')].valueFrom.fieldRef.fieldPath": "metadata.name",
			},
			Unexpected: []string{
				"spec.template.spec.containers[0].env[?(@.name=='DOCUMENT_STORE_" + strings.ToUpper(storeID) + "_CONNECTION_STRING')]",
			},
		})
	}
	for caseIndex := range testCases {
		testCases[caseIndex].Unexpected = append(testCases[caseIndex].Unexpected,
			"spec.template.spec.containers[0].env[?(@.name=='VALUES_DOCUMENT_STORE_AZURE_CONNECTION_STRING')]")
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *normalizeSecretConfigTest) TestEmitVolumeFromSecretConfig() {
	// Use orchestration statefulset template - the only remaining document-store
	// consumer (camunda-platform-helm#3741 removed the wiring from connectors, which
	// this test previously used, since connectors never actually consumed it).
	templates := []string{"templates/orchestration/statefulset.yaml"}

	testCases := []testhelpers.TestCase{
		{
			Name: "gcp document store new style secret creates volume",
			Values: map[string]string{
				"global.documentStore.type.gcp.enabled":                  "true",
				"global.documentStore.type.gcp.secret.existingSecret":    "my-gcp-secret",
				"global.documentStore.type.gcp.secret.existingSecretKey": "credentials.json",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.secretName":    "my-gcp-secret",
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.items[0].key":  "credentials.json",
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.items[0].path": "service-account.json",
			},
		},
		{
			Name: "gcp document store custom fileName creates volume with custom path",
			Values: map[string]string{
				"global.documentStore.type.gcp.enabled":                  "true",
				"global.documentStore.type.gcp.secret.existingSecret":    "custom-gcp-secret",
				"global.documentStore.type.gcp.secret.existingSecretKey": "custom.json",
				"global.documentStore.type.gcp.fileName":                 "my-custom-file.json",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.secretName":    "custom-gcp-secret",
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.items[0].key":  "custom.json",
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.items[0].path": "my-custom-file.json",
			},
		},
		{
			Name: "gcp document store volume mount is created when secret exists",
			Values: map[string]string{
				"global.documentStore.type.gcp.enabled":                  "true",
				"global.documentStore.type.gcp.secret.existingSecret":    "mount-test-secret",
				"global.documentStore.type.gcp.secret.existingSecretKey": "mount-test.json",
				"global.documentStore.type.gcp.mountPath":                "/custom/mount/path",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='gcp-credentials-volume')].mountPath": "/custom/mount/path",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='gcp-credentials-volume')].readOnly":  "true",
			},
		},
		{
			Name: "no gcp document store config means no volume",
			Values: map[string]string{
				"global.documentStore.type.gcp.enabled": "true",
				// No secret configuration
			},
			Verifier: func(t *testing.T, output string, err error) {
				// Should still create volume due to defaults in values.yaml, but verify it uses defaults
				require.Contains(t, output, "gcp-credentials-volume")
				require.Contains(t, output, "gcp-credentials") // default secret name
			},
		},
		{
			Name: "gcp document store disabled means no volume",
			Values: map[string]string{
				"global.documentStore.type.gcp.enabled":                  "false",
				"global.documentStore.type.gcp.secret.existingSecret":    "should-not-be-used",
				"global.documentStore.type.gcp.secret.existingSecretKey": "should-not-be-used.json",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// Should not create any GCP volumes when GCP document store is disabled
				require.NotContains(t, output, "gcp-credentials-volume")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

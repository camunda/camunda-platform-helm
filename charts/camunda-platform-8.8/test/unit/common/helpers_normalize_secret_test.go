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

func (s *normalizeSecretConfigTest) TestDeprecatedGlobalOpenSearchSecretCompatibility() {
	testCases := []testhelpers.TestCase{
		{
			Name: "Deprecated global OpenSearch existing-secret compatibility",
			Values: map[string]string{
				"orchestration.enabled":                           "true",
				"global.elasticsearch.enabled":                    "false",
				"elasticsearch.enabled":                           "false",
				"global.opensearch.enabled":                       "true",
				"global.opensearch.auth.secret.existingSecret":    "my-opensearch-secret",
				"global.opensearch.auth.secret.existingSecretKey": "my-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.name": "my-opensearch-secret",
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.key":  "my-key",
			},
		},
		{
			Name: "Deprecated global OpenSearch inline-secret compatibility",
			Values: map[string]string{
				"orchestration.enabled":                      "true",
				"global.elasticsearch.enabled":               "false",
				"elasticsearch.enabled":                      "false",
				"global.opensearch.enabled":                  "true",
				"global.opensearch.auth.secret.inlineSecret": "my-password",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].value": "my-password",
			},
		},
		{
			Name: "Deprecated global OpenSearch legacy existing-secret compatibility",
			Values: map[string]string{
				"orchestration.enabled":                    "true",
				"global.elasticsearch.enabled":             "false",
				"elasticsearch.enabled":                    "false",
				"global.opensearch.enabled":                "true",
				"global.opensearch.auth.existingSecret":    "legacy-secret",
				"global.opensearch.auth.existingSecretKey": "legacy-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.name": "legacy-secret",
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.key":  "legacy-key",
			},
		},
		{
			Name: "Deprecated global OpenSearch plaintext-password compatibility",
			Values: map[string]string{
				"orchestration.enabled":           "true",
				"global.elasticsearch.enabled":    "false",
				"elasticsearch.enabled":           "false",
				"global.opensearch.enabled":       "true",
				"global.opensearch.auth.password": "plain-password",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].value": "plain-password",
			},
		},
		{
			Name: "Deprecated global OpenSearch without secret omits password env",
			Values: map[string]string{
				"orchestration.enabled":        "true",
				"global.elasticsearch.enabled": "false",
				"elasticsearch.enabled":        "false",
				"global.opensearch.enabled":    "true",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].name": "orchestration",
			},
			Unexpected: []string{"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')]"},
		},
		{
			Name: "Deprecated disabled global OpenSearch omits password env",
			Values: map[string]string{
				"orchestration.enabled":                      "true",
				"global.opensearch.enabled":                  "false",
				"global.opensearch.auth.secret.inlineSecret": "password",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].name": "orchestration",
			},
			Unexpected: []string{"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')]"},
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
			Name: "Deprecated AWS document store secret compatibility",
			Values: map[string]string{
				"orchestration.enabled":                            "true",
				"global.documentStore.type.aws.enabled":            "true",
				"global.documentStore.type.aws.existingSecret":     "legacy-aws-secret",
				"global.documentStore.type.aws.accessKeyIdKey":     "legacy-access-key",
				"global.documentStore.type.aws.secretAccessKeyKey": "legacy-secret-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')].valueFrom.secretKeyRef.name":     "legacy-aws-secret",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')].valueFrom.secretKeyRef.key":      "legacy-access-key",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')].valueFrom.secretKeyRef.name": "legacy-aws-secret",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')].valueFrom.secretKeyRef.key":  "legacy-secret-key",
			},
		},
		{
			Name: "Current AWS document store secrets override deprecated compatibility values",
			Values: map[string]string{
				"orchestration.enabled":                                                  "true",
				"global.documentStore.type.aws.enabled":                                  "true",
				"global.documentStore.type.aws.existingSecret":                           "legacy-aws-secret",
				"global.documentStore.type.aws.accessKeyIdKey":                           "legacy-access-key",
				"global.documentStore.type.aws.secretAccessKeyKey":                       "legacy-secret-key",
				"global.documentStore.type.aws.accessKeyId.secret.existingSecret":        "new-aws-secret",
				"global.documentStore.type.aws.accessKeyId.secret.existingSecretKey":     "new-access-key",
				"global.documentStore.type.aws.secretAccessKey.secret.existingSecret":    "new-aws-secret",
				"global.documentStore.type.aws.secretAccessKey.secret.existingSecretKey": "new-secret-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')].valueFrom.secretKeyRef.name":     "new-aws-secret",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')].valueFrom.secretKeyRef.key":      "new-access-key",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')].valueFrom.secretKeyRef.name": "new-aws-secret",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')].valueFrom.secretKeyRef.key":  "new-secret-key",
			},
		},
		{
			Name: "no aws document store config means no env vars",
			Values: map[string]string{
				"orchestration.enabled":                 "true",
				"global.documentStore.type.aws.enabled": "false",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].name": "orchestration",
			},
			Unexpected: []string{
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')]",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')]",
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
			Expected: map[string]string{
				"spec.template.spec.containers[0].name": "orchestration",
			},
			Unexpected: []string{
				"spec.template.spec.containers[0].env[?(@.name=='AWS_ACCESS_KEY_ID')]",
				"spec.template.spec.containers[0].env[?(@.name=='AWS_SECRET_ACCESS_KEY')]",
			},
		},
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
			Name: "gcp document store legacy secret format creates volume",
			Values: map[string]string{
				"global.documentStore.type.gcp.enabled":        "true",
				"global.documentStore.type.gcp.existingSecret": "legacy-gcp-secret",
				"global.documentStore.type.gcp.credentialsKey": "legacy-credentials.json",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.secretName":    "legacy-gcp-secret",
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.items[0].key":  "legacy-credentials.json",
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.items[0].path": "service-account.json",
			},
		},
		{
			Name: "gcp document store mixed configuration - new takes precedence",
			Values: map[string]string{
				"global.documentStore.type.gcp.enabled": "true",
				// Legacy configuration (should be ignored)
				"global.documentStore.type.gcp.existingSecret": "legacy-gcp-secret",
				"global.documentStore.type.gcp.credentialsKey": "legacy-credentials.json",
				// New configuration (should take precedence)
				"global.documentStore.type.gcp.secret.existingSecret":    "new-gcp-secret",
				"global.documentStore.type.gcp.secret.existingSecretKey": "new-credentials.json",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.secretName":    "new-gcp-secret",
				"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')].secret.items[0].key":  "new-credentials.json",
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
			Expected: map[string]string{
				"spec.template.spec.containers[0].name": "connectors",
			},
			Unexpected: []string{"spec.template.spec.volumes[?(@.name=='gcp-credentials-volume')]"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

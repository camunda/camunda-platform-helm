// documentstore_configmap_test.go
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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type documentStoreConfigMapTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestDocumentStoreConfigMapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &documentStoreConfigMapTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/camunda/configmap-documentstore.yaml"},
	})
}

func (s *documentStoreConfigMapTest) renderTemplate(options *helm.Options) corev1.ConfigMap {
    output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
    var configmap corev1.ConfigMap
    helm.UnmarshalK8SYaml(s.T(), output, &configmap)
    return configmap
}

func (s *documentStoreConfigMapTest) verifyConfigMap(testCase string, configmap corev1.ConfigMap, expectedValues map[string]string) {
    for key, expectedValue := range expectedValues {
        s.Require().Equal(expectedValue, strings.TrimSpace(configmap.Data[key]), "%s: Expected %s to be %s", testCase, key, expectedValue)
    }
}


func (s *documentStoreConfigMapTest) TestDifferentValuesInputs() {
	testCases := []struct {
		testCase          string
		values        map[string]string
		expected      map[string]string
	}{
		{
			testCase: "AWS Store",
			values: map[string]string{
				"global.documentStore.activeStoreId":       "aws",
				"global.documentStore.type.aws.enabled":    "true",
				"global.documentStore.type.aws.storeId":    "AWS",
				"global.documentStore.type.aws.class":      "io.camunda.document.store.aws.AwsDocumentStoreProvider",
				"global.documentStore.type.aws.bucket":     "aws-bucket",
				"global.documentStore.type.aws.bucketPath": "/aws/path",
			},
			expected: map[string]string{
				"DOCUMENT_DEFAULT_STORE_ID":    "aws",
				"DOCUMENT_STORE_AWS_CLASS":     "io.camunda.document.store.aws.AwsDocumentStoreProvider",
				"DOCUMENT_STORE_AWS_BUCKET":    "aws-bucket",
				"DOCUMENT_STORE_AWS_BUCKET_PATH": "/aws/path",
			},
		},
		{
			testCase: "GCP Store",
			values: map[string]string{
				"global.documentStore.activeStoreId":    "gcp",
				"global.documentStore.type.gcp.enabled": "true",
				"global.documentStore.type.gcp.storeId": "GCP",
				"global.documentStore.type.gcp.class":   "io.camunda.document.store.gcp.GcpDocumentStoreProvider",
				"global.documentStore.type.gcp.bucket":  "gcp-bucket",
			},
			expected: map[string]string{
				"DOCUMENT_DEFAULT_STORE_ID": "gcp",
				"DOCUMENT_STORE_GCP_CLASS":  "io.camunda.document.store.gcp.GcpDocumentStoreProvider",
				"DOCUMENT_STORE_GCP_BUCKET": "gcp-bucket",
			},
		},
		{
			testCase: "Local Storage",
			values: map[string]string{
				"global.documentStore.activeStoreId":         "inmemory",
				"global.documentStore.type.inmemory.enabled": "true",
				"global.documentStore.type.inmemory.storeId": "INMEMORY",
				"global.documentStore.type.inmemory.class":   "io.camunda.document.store.inmemory.InMemoryDocumentStoreProvider",
			},
			expected: map[string]string{
				"DOCUMENT_DEFAULT_STORE_ID": "inmemory",
				"DOCUMENT_STORE_LOCAL_CLASS": "io.camunda.document.store.inmemory.InMemoryDocumentStoreProvider",
			},
		},
	}
}
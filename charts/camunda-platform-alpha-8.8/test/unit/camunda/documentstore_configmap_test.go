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
	"github.com/gruntwork-io/terratest/modules/k8s"
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

func (s *documentStoreConfigMapTest) TestActiveDocumentStoreAWS() {
	// given: set activeStoreId to aws and enable AWS configuration
	options := &helm.Options{
		SetValues: map[string]string{
			"global.documentStore.activeStoreId":       "aws",
			"global.documentStore.type.aws.enabled":    "true",
			"global.documentStore.type.aws.storeId":    "AWS",
			"global.documentStore.type.aws.class":      "io.camunda.document.store.aws.AwsDocumentStoreProvider",
			"global.documentStore.type.aws.bucket":     "my-aws-bucket",
			"global.documentStore.type.aws.bucketPath": "/my/aws/path",
			"global.documentStore.type.aws.bucketTtl":  "3600",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	// then: verify that active store is AWS and the correct AWS keys are present
	s.Require().Equal("aws", strings.ToLower(configmap.Data["DOCUMENT_DEFAULT_STORE_ID"]))
	s.Require().Equal("io.camunda.document.store.aws.AwsDocumentStoreProvider", strings.TrimSpace(configmap.Data["DOCUMENT_STORE_AWS_CLASS"]))
	s.Require().Equal("my-aws-bucket", strings.TrimSpace(configmap.Data["DOCUMENT_STORE_AWS_BUCKET"]))
	s.Require().Equal("/my/aws/path", strings.TrimSpace(configmap.Data["DOCUMENT_STORE_AWS_BUCKET_PATH"]))
	s.Require().Equal("3600", strings.TrimSpace(configmap.Data["DOCUMENT_STORE_AWS_BUCKET_TTL"]))
}

func (s *documentStoreConfigMapTest) TestActiveDocumentStoreInMemory() {
	// given: set activeStoreId to inmemory and enable InMemory configuration
	options := &helm.Options{
		SetValues: map[string]string{
			"global.documentStore.activeStoreId":         "inmemory",
			"global.documentStore.type.inmemory.enabled": "true",
			"global.documentStore.type.inmemory.storeId": "INMEMORY",
			"global.documentStore.type.inmemory.class":   "io.camunda.document.store.inmemory.InMemoryDocumentStoreProvider",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	// then: verify that active store is InMemory and the correct key is present
	s.Require().Equal("inmemory", strings.ToLower(configmap.Data["DOCUMENT_DEFAULT_STORE_ID"]))
	s.Require().Equal("io.camunda.document.store.inmemory.InMemoryDocumentStoreProvider", strings.TrimSpace(configmap.Data["DOCUMENT_STORE_INMEMORY_CLASS"]))
}

func (s *documentStoreConfigMapTest) TestActiveDocumentStoreGCP() {
	// given: set activeStoreId to gcp and enable GCP configuration
	options := &helm.Options{
		SetValues: map[string]string{
			"global.documentStore.activeStoreId":    "gcp",
			"global.documentStore.type.gcp.enabled": "true",
			"global.documentStore.type.gcp.storeId": "GCP",
			"global.documentStore.type.gcp.class":   "io.camunda.document.store.gcp.GcpDocumentStoreProvider",
			"global.documentStore.type.gcp.bucket":  "my-gcp-bucket",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	// then: verify that active store is GCP and the correct GCP keys are present
	s.Require().Equal("gcp", strings.ToLower(configmap.Data["DOCUMENT_DEFAULT_STORE_ID"]))
	s.Require().Equal("io.camunda.document.store.gcp.GcpDocumentStoreProvider", strings.TrimSpace(configmap.Data["DOCUMENT_STORE_GCP_CLASS"]))
	s.Require().Equal("my-gcp-bucket", strings.TrimSpace(configmap.Data["DOCUMENT_STORE_GCP_BUCKET"]))
}

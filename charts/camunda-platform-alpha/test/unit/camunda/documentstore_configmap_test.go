package camunda

import (
	"camunda-platform/test/unit/testhelpers"
	"testing"

	"github.com/stretchr/testify/suite"
)

type documentStoreConfigMapTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestDocumentStoreConfigMapTemplate(t *testing.T) {
    testutils.TestCreateTestSuite(t, []string{"templates/camunda/configmap-documentstore.yaml"})
}

func (s *documentStoreConfigMapTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "Document Handling: AWS",
			Values: map[string]string{
				"global.documentStore.activeStoreId":       "aws",
				"global.documentStore.type.aws.enabled":    "true",
				"global.documentStore.type.aws.storeId":    "AWS",
				"global.documentStore.type.aws.class":      "io.camunda.document.store.aws.AwsDocumentStoreProvider",
				"global.documentStore.type.aws.bucket":     "aws-bucket",
				"global.documentStore.type.aws.bucketPath": "/aws/path",
			},
			Expected: map[string]string{
				"DOCUMENT_DEFAULT_STORE_ID":    "aws",
				"DOCUMENT_STORE_AWS_CLASS":     "io.camunda.document.store.aws.AwsDocumentStoreProvider",
				"DOCUMENT_STORE_AWS_BUCKET":    "aws-bucket",
				"DOCUMENT_STORE_AWS_BUCKET_PATH": "/aws/path",
			},
		},
		{
			Name: "Document Handling: GCP",
			Values: map[string]string{
				"global.documentStore.activeStoreId":    "gcp",
				"global.documentStore.type.gcp.enabled": "true",
				"global.documentStore.type.gcp.storeId": "GCP",
				"global.documentStore.type.gcp.class":   "io.camunda.document.store.gcp.GcpDocumentStoreProvider",
				"global.documentStore.type.gcp.bucket":  "gcp-bucket",
			},
			Expected: map[string]string{
				"DOCUMENT_DEFAULT_STORE_ID": "gcp",
				"DOCUMENT_STORE_GCP_CLASS":  "io.camunda.document.store.gcp.GcpDocumentStoreProvider",
				"DOCUMENT_STORE_GCP_BUCKET": "gcp-bucket",
			},
		},
		{
			Name: "Document Handling: In Memory",
			Values: map[string]string{
				"global.documentStore.activeStoreId":         "inmemory",
				"global.documentStore.type.inmemory.enabled": "true",
				"global.documentStore.type.inmemory.storeId": "INMEMORY",
				"global.documentStore.type.inmemory.class":   "io.camunda.document.store.inmemory.InMemoryDocumentStoreProvider",
			},
			Expected: map[string]string{
				"DOCUMENT_DEFAULT_STORE_ID": "inmemory",
				"DOCUMENT_STORE_INMEMORY_CLASS": "io.camunda.document.store.inmemory.InMemoryDocumentStoreProvider",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

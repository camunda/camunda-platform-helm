// documentstore_irsa_test.go
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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type documentStoreIRSATest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestDocumentStoreIRSATemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &documentStoreIRSATest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{},
	})
}

// Helper function to check if AWS_ACCESS_KEY_ID env var exists in container
func hasAwsAccessKeyIdEnvVar(containers []corev1.Container) bool {
	for _, container := range containers {
		for _, env := range container.Env {
			if env.Name == "AWS_ACCESS_KEY_ID" {
				return true
			}
		}
	}
	return false
}

// Helper function to check if AWS_SECRET_ACCESS_KEY env var exists in container
func hasAwsSecretAccessKeyEnvVar(containers []corev1.Container) bool {
	for _, container := range containers {
		for _, env := range container.Env {
			if env.Name == "AWS_SECRET_ACCESS_KEY" {
				return true
			}
		}
	}
	return false
}

// baseValues returns common values needed for chart rendering
func baseValues() map[string]string {
	return map[string]string{
		"identity.enabled": "true",
		"connectors.security.authentication.oidc.existingSecret.existingSecret":    "foo",
		"orchestration.security.authentication.oidc.existingSecret.existingSecret": "bar",
	}
}

// awsDocumentStoreValues returns values to enable AWS document store
func awsDocumentStoreValues(useCredentialsInSecret bool) map[string]string {
	values := baseValues()
	values["global.documentStore.activeStoreId"] = "aws"
	values["global.documentStore.type.aws.enabled"] = "true"
	values["global.documentStore.type.aws.bucket"] = "test-bucket"
	values["global.documentStore.type.aws.region"] = "us-east-1"
	if useCredentialsInSecret {
		values["global.documentStore.type.aws.useCredentialsInSecret"] = "true"
		values["global.documentStore.type.aws.existingSecret"] = "aws-credentials"
		values["global.documentStore.type.aws.accessKeyIdKey"] = "awsAccessKeyId"
		values["global.documentStore.type.aws.secretAccessKeyKey"] = "awsSecretAccessKey"
	} else {
		values["global.documentStore.type.aws.useCredentialsInSecret"] = "false"
	}
	return values
}

func (s *documentStoreIRSATest) TestOrchestrationStatefulSetWithIRSA() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/orchestration/statefulset.yaml",
			Values:   awsDocumentStoreValues(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				containers := statefulSet.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "AWS credentials SHOULD be injected when useCredentialsInSecret is true (default)",
			Template: "templates/orchestration/statefulset.yaml",
			Values:   awsDocumentStoreValues(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				containers := statefulSet.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestOrchestrationImporterWithIRSA() {
	valuesIRSA := awsDocumentStoreValues(false)
	valuesIRSA["orchestration.migration.data.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValues(true)
	valuesWithCredentials["orchestration.migration.data.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:     "Importer: AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/orchestration/importer-deployment.yaml",
			Values:   valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "Importer: AWS credentials SHOULD be injected when useCredentialsInSecret is true",
			Template: "templates/orchestration/importer-deployment.yaml",
			Values:   valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestConsoleWithIRSA() {
	valuesIRSA := awsDocumentStoreValues(false)
	valuesIRSA["console.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValues(true)
	valuesWithCredentials["console.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:     "Console: AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/console/deployment.yaml",
			Values:   valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "Console: AWS credentials SHOULD be injected when useCredentialsInSecret is true",
			Template: "templates/console/deployment.yaml",
			Values:   valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestConnectorsWithIRSA() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "Connectors: AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/connectors/deployment.yaml",
			Values:   awsDocumentStoreValues(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "Connectors: AWS credentials SHOULD be injected when useCredentialsInSecret is true",
			Template: "templates/connectors/deployment.yaml",
			Values:   awsDocumentStoreValues(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestIdentityWithIRSA() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "Identity: AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/identity/deployment.yaml",
			Values:   awsDocumentStoreValues(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "Identity: AWS credentials SHOULD be injected when useCredentialsInSecret is true",
			Template: "templates/identity/deployment.yaml",
			Values:   awsDocumentStoreValues(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestOptimizeWithIRSA() {
	valuesIRSA := awsDocumentStoreValues(false)
	valuesIRSA["optimize.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValues(true)
	valuesWithCredentials["optimize.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:     "Optimize: AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/optimize/deployment.yaml",
			Values:   valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "Optimize: AWS credentials SHOULD be injected when useCredentialsInSecret is true",
			Template: "templates/optimize/deployment.yaml",
			Values:   valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerWebappWithIRSA() {
	valuesIRSA := awsDocumentStoreValues(false)
	valuesIRSA["webModeler.enabled"] = "true"
	valuesIRSA["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithCredentials := awsDocumentStoreValues(true)
	valuesWithCredentials["webModeler.enabled"] = "true"
	valuesWithCredentials["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:     "WebModeler Webapp: AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/web-modeler/deployment-webapp.yaml",
			Values:   valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "WebModeler Webapp: AWS credentials SHOULD be injected when useCredentialsInSecret is true",
			Template: "templates/web-modeler/deployment-webapp.yaml",
			Values:   valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerRestapiWithIRSA() {
	valuesIRSA := awsDocumentStoreValues(false)
	valuesIRSA["webModeler.enabled"] = "true"
	valuesIRSA["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithCredentials := awsDocumentStoreValues(true)
	valuesWithCredentials["webModeler.enabled"] = "true"
	valuesWithCredentials["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:     "WebModeler REST API: AWS credentials should NOT be injected when useCredentialsInSecret is false (IRSA mode)",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values:   valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when useCredentialsInSecret is false")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when useCredentialsInSecret is false")
			},
		},
		{
			Name:     "WebModeler REST API: AWS credentials SHOULD be injected when useCredentialsInSecret is true",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values:   valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when useCredentialsInSecret is true")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when useCredentialsInSecret is true")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

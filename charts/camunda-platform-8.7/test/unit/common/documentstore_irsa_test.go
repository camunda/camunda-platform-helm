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
		"global.identity.auth.publicIssuerUrl":  "https://example.com",
		"global.identity.auth.issuerBackendUrl": "https://example.com",
		"identity.firstUser.password":           "testpassword",
		"connectors.inbound.mode":               "disabled",
	}
}

// awsDocumentStoreValuesWithIRSA returns values to enable AWS document store with IRSA enabled
func awsDocumentStoreValuesWithIRSA(irsaEnabled bool) map[string]string {
	values := baseValues()
	values["global.documentStore.activeStoreId"] = "aws"
	values["global.documentStore.type.aws.enabled"] = "true"
	values["global.documentStore.type.aws.bucket"] = "test-bucket"
	values["global.documentStore.type.aws.region"] = "us-east-1"
	if irsaEnabled {
		// IRSA mode: no credentials injected
		values["global.documentStore.type.aws.irsa.enabled"] = "true"
	} else {
		// Credentials mode: credentials are injected from secret
		values["global.documentStore.type.aws.irsa.enabled"] = "false"
		values["global.documentStore.type.aws.existingSecret"] = "aws-credentials"
		values["global.documentStore.type.aws.accessKeyIdKey"] = "awsAccessKeyId"
		values["global.documentStore.type.aws.secretAccessKeyKey"] = "awsSecretAccessKey"
	}
	return values
}

func (s *documentStoreIRSATest) TestZeebeStatefulSetWithIRSA() {
	templates := []string{"templates/zeebe/statefulset.yaml"}
	testCases := []testhelpers.TestCase{
		{
			Name:   "AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: awsDocumentStoreValuesWithIRSA(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				containers := statefulSet.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "AWS credentials SHOULD be injected when irsa.enabled is false (default)",
			Values: awsDocumentStoreValuesWithIRSA(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				containers := statefulSet.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestZeebeGatewayWithIRSA() {
	templates := []string{"templates/zeebe-gateway/deployment.yaml"}
	testCases := []testhelpers.TestCase{
		{
			Name:   "Zeebe Gateway: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: awsDocumentStoreValuesWithIRSA(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "Zeebe Gateway: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: awsDocumentStoreValuesWithIRSA(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestConsoleWithIRSA() {
	templates := []string{"templates/console/deployment.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["console.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["console.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "Console: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "Console: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestConnectorsWithIRSA() {
	templates := []string{"templates/connectors/deployment.yaml"}
	testCases := []testhelpers.TestCase{
		{
			Name:   "Connectors: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: awsDocumentStoreValuesWithIRSA(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "Connectors: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: awsDocumentStoreValuesWithIRSA(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestIdentityWithIRSA() {
	templates := []string{"templates/identity/deployment.yaml"}
	testCases := []testhelpers.TestCase{
		{
			Name:   "Identity: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: awsDocumentStoreValuesWithIRSA(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "Identity: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: awsDocumentStoreValuesWithIRSA(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestOperateWithIRSA() {
	templates := []string{"templates/operate/deployment.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["operate.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["operate.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "Operate: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "Operate: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestTasklistWithIRSA() {
	templates := []string{"templates/tasklist/deployment.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["tasklist.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["tasklist.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "Tasklist: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "Tasklist: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestExecutionIdentityWithIRSA() {
	templates := []string{"templates/execution-identity/deployment.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["executionIdentity.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["executionIdentity.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "ExecutionIdentity: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "ExecutionIdentity: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerWebappWithIRSA() {
	templates := []string{"templates/web-modeler/deployment-webapp.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["webModeler.enabled"] = "true"
	valuesIRSA["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["webModeler.enabled"] = "true"
	valuesWithCredentials["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:   "WebModeler Webapp: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "WebModeler Webapp: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerRestapiWithIRSA() {
	templates := []string{"templates/web-modeler/deployment-restapi.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["webModeler.enabled"] = "true"
	valuesIRSA["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["webModeler.enabled"] = "true"
	valuesWithCredentials["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:   "WebModeler REST API: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Values: valuesIRSA,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
			},
		},
		{
			Name:   "WebModeler REST API: AWS credentials SHOULD be injected when irsa.enabled is false",
			Values: valuesWithCredentials,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

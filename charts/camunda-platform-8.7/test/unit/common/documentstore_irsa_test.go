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
	_ "camunda-platform/test/unit/utils"
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

func requireNoAwsCredentialEnvVars(t *testing.T, containers []corev1.Container) {
	t.Helper()
	require.False(t, hasAwsAccessKeyIdEnvVar(containers),
		"AWS_ACCESS_KEY_ID should not be present when the secret is unset")
	require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
		"AWS_SECRET_ACCESS_KEY should not be present when the secret is unset")
}

// Helper function to check if any container references the shared documentstore-env-vars ConfigMap via envFrom
func hasDocumentStoreEnvFromRef(containers []corev1.Container) bool {
	for _, container := range containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && strings.Contains(envFrom.ConfigMapRef.Name, "documentstore-env-vars") {
				return true
			}
		}
	}
	return false
}

// Helper function to check if any env var references a Secret with an empty name, which Kubernetes
// rejects with an RFC-1123 error (SUPPORT-29235)
func hasEmptySecretKeyRefName(containers []corev1.Container) bool {
	for _, container := range containers {
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == "" {
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

// awsDocumentStoreValuesWithoutSecret enables the AWS document store in credentials mode but
// configures no secret, reproducing SUPPORT-29235
func awsDocumentStoreValuesWithoutSecret() map[string]string {
	values := baseValues()
	values["global.documentStore.activeStoreId"] = "aws"
	values["global.documentStore.type.aws.enabled"] = "true"
	values["global.documentStore.type.aws.bucket"] = "test-bucket"
	values["global.documentStore.type.aws.region"] = "us-east-1"
	values["global.documentStore.type.aws.irsa.enabled"] = "false"
	values["global.documentStore.type.aws.existingSecret"] = ""
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
		{
			Name:   "AWS credentials should NOT be injected when the secret is unset",
			Values: awsDocumentStoreValuesWithoutSecret(),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				containers := statefulSet.Spec.Template.Spec.Containers
				requireNoAwsCredentialEnvVars(t, containers)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
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
		{
			Name:   "Zeebe Gateway: AWS credentials should NOT be injected when the secret is unset",
			Values: awsDocumentStoreValuesWithoutSecret(),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				requireNoAwsCredentialEnvVars(t, containers)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestConsoleNeverGetsDocumentStoreCreds() {
	templates := []string{"templates/console/deployment.yaml"}

	values := awsDocumentStoreValuesWithIRSA(false)
	values["console.enabled"] = "true"

	valuesWithoutSecret := awsDocumentStoreValuesWithoutSecret()
	valuesWithoutSecret["console.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "Console: document-store AWS credentials and envFrom should never be injected",
			Values: values,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should never be present on console")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should never be present on console")
				require.False(t, hasDocumentStoreEnvFromRef(containers),
					"console should never reference the documentstore-env-vars ConfigMap")
			},
		},
		{
			Name:   "Console: no empty secretKeyRef when the AWS document-store secret is unset (SUPPORT-29235)",
			Values: valuesWithoutSecret,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasEmptySecretKeyRefName(containers),
					"Console should not render a secretKeyRef with an empty name")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestIdentityNeverGetsDocumentStoreCreds() {
	templates := []string{"templates/identity/deployment.yaml"}
	testCases := []testhelpers.TestCase{
		{
			Name:   "Identity: document-store AWS credentials and envFrom should never be injected",
			Values: awsDocumentStoreValuesWithIRSA(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should never be present on identity")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should never be present on identity")
				require.False(t, hasDocumentStoreEnvFromRef(containers),
					"identity should never reference the documentstore-env-vars ConfigMap")
			},
		},
		{
			Name:   "Identity: no empty secretKeyRef when the AWS document-store secret is unset (SUPPORT-29235)",
			Values: awsDocumentStoreValuesWithoutSecret(),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasEmptySecretKeyRefName(containers),
					"Identity should not render a secretKeyRef with an empty name")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestOperateWithIRSA() {
	templates := []string{"templates/operate/deployment.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["operate.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["operate.enabled"] = "true"

	valuesWithoutSecret := awsDocumentStoreValuesWithoutSecret()
	valuesWithoutSecret["operate.enabled"] = "true"

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
		{
			Name:   "Operate: AWS credentials should NOT be injected when the secret is unset",
			Values: valuesWithoutSecret,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				requireNoAwsCredentialEnvVars(t, containers)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestTasklistWithIRSA() {
	templates := []string{"templates/tasklist/deployment.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["tasklist.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["tasklist.enabled"] = "true"

	valuesWithoutSecret := awsDocumentStoreValuesWithoutSecret()
	valuesWithoutSecret["tasklist.enabled"] = "true"

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
		{
			Name:   "Tasklist: AWS credentials should NOT be injected when the secret is unset",
			Values: valuesWithoutSecret,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				requireNoAwsCredentialEnvVars(t, containers)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestExecutionIdentityWithIRSA() {
	templates := []string{"templates/execution-identity/deployment.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["executionIdentity.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["executionIdentity.enabled"] = "true"

	valuesWithoutSecret := awsDocumentStoreValuesWithoutSecret()
	valuesWithoutSecret["executionIdentity.enabled"] = "true"

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
		{
			Name:   "ExecutionIdentity: AWS credentials should NOT be injected when the secret is unset",
			Values: valuesWithoutSecret,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				requireNoAwsCredentialEnvVars(t, containers)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerWebappNeverGetsDocumentStoreCreds() {
	templates := []string{"templates/web-modeler/deployment-webapp.yaml"}

	values := awsDocumentStoreValuesWithIRSA(false)
	values["webModeler.enabled"] = "true"
	values["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithoutSecret := awsDocumentStoreValuesWithoutSecret()
	valuesWithoutSecret["webModeler.enabled"] = "true"
	valuesWithoutSecret["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:   "WebModeler Webapp: document-store AWS credentials and envFrom should never be injected",
			Values: values,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should never be present on web-modeler-webapp")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should never be present on web-modeler-webapp")
				require.False(t, hasDocumentStoreEnvFromRef(containers),
					"web-modeler-webapp should never reference the documentstore-env-vars ConfigMap")
			},
		},
		{
			Name:   "Web Modeler webapp: no empty secretKeyRef when the AWS document-store secret is unset (SUPPORT-29235)",
			Values: valuesWithoutSecret,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasEmptySecretKeyRefName(containers),
					"Web Modeler webapp should not render a secretKeyRef with an empty name")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerRestapiWithIRSA() {
	templates := []string{"templates/web-modeler/deployment-restapi.yaml"}

	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["webModeler.enabled"] = "true"
	valuesIRSA["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["webModeler.enabled"] = "true"
	valuesWithCredentials["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithoutSecret := awsDocumentStoreValuesWithoutSecret()
	valuesWithoutSecret["webModeler.enabled"] = "true"
	valuesWithoutSecret["webModeler.restapi.mail.fromAddress"] = "test@example.com"

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
				require.True(t, hasDocumentStoreEnvFromRef(containers),
					"web-modeler-restapi must keep the documentstore-env-vars ConfigMap, its only AWS_REGION source")
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
				require.True(t, hasDocumentStoreEnvFromRef(containers),
					"web-modeler-restapi must keep the documentstore-env-vars ConfigMap, its only AWS_REGION source")
			},
		},
		{
			Name:   "WebModeler REST API: AWS credentials should NOT be injected when the secret is unset",
			Values: valuesWithoutSecret,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				requireNoAwsCredentialEnvVars(t, containers)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, templates, testCases)
}

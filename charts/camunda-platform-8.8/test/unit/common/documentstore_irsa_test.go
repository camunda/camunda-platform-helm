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
		"identity.enabled": "true",
		"connectors.security.authentication.oidc.existingSecret.existingSecret":    "foo",
		"orchestration.security.authentication.oidc.existingSecret.existingSecret": "bar",
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
	return values
}

func (s *documentStoreIRSATest) TestOrchestrationStatefulSetWithIRSA() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Template: "templates/orchestration/statefulset.yaml",
			Values:   awsDocumentStoreValuesWithIRSA(true),
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
			Name:     "AWS credentials SHOULD be injected when irsa.enabled is false (default)",
			Template: "templates/orchestration/statefulset.yaml",
			Values:   awsDocumentStoreValuesWithIRSA(false),
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

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestOrchestrationImporterWithIRSA() {
	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["orchestration.migration.data.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["orchestration.migration.data.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:     "Importer: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Template: "templates/orchestration/importer-deployment.yaml",
			Values:   valuesIRSA,
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
			Name:     "Importer: AWS credentials SHOULD be injected when irsa.enabled is false",
			Template: "templates/orchestration/importer-deployment.yaml",
			Values:   valuesWithCredentials,
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

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// connectors, console, identity, optimize and web-modeler-webapp are not
// document-store consumers (camunda-platform-helm#3741) - these guard against
// the config ever being re-introduced.

func (s *documentStoreIRSATest) TestConsoleNeverGetsDocumentStoreCreds() {
	values := awsDocumentStoreValuesWithIRSA(false)
	values["console.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:     "Console: document-store AWS credentials and envFrom should never be injected",
			Template: "templates/console/deployment.yaml",
			Values:   values,
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
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// Connectors reads the AWS credentials as ambient AWS SDK config for connector tasks, so they stay;
// it never reads the document-store ConfigMap, whose only other payload (AWS_REGION) it cannot
// consume either - AwsUtils.extractRegionOrDefault takes the region from the element template.
func (s *documentStoreIRSATest) TestConnectorsWithIRSA() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "Connectors: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Template: "templates/connectors/deployment.yaml",
			Values:   awsDocumentStoreValuesWithIRSA(true),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should NOT be present when irsa.enabled is true")
				require.False(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should NOT be present when irsa.enabled is true")
				require.False(t, hasDocumentStoreEnvFromRef(containers),
					"connectors should not reference the documentstore-env-vars ConfigMap")
			},
		},
		{
			Name:     "Connectors: AWS credentials SHOULD be injected when irsa.enabled is false",
			Template: "templates/connectors/deployment.yaml",
			Values:   awsDocumentStoreValuesWithIRSA(false),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.True(t, hasAwsAccessKeyIdEnvVar(containers),
					"AWS_ACCESS_KEY_ID should be present when irsa.enabled is false")
				require.True(t, hasAwsSecretAccessKeyEnvVar(containers),
					"AWS_SECRET_ACCESS_KEY should be present when irsa.enabled is false")
				require.False(t, hasDocumentStoreEnvFromRef(containers),
					"connectors should not reference the documentstore-env-vars ConfigMap")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestIdentityNeverGetsDocumentStoreCreds() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "Identity: document-store AWS credentials and envFrom should never be injected",
			Template: "templates/identity/deployment.yaml",
			Values:   awsDocumentStoreValuesWithIRSA(false),
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
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// Optimize reads the credentials and AWS_REGION as ambient AWS SDK config to sign AWS OpenSearch
// requests, so both the credentials and the ConfigMap reference (its only AWS_REGION source) stay.
func (s *documentStoreIRSATest) TestOptimizeWithIRSA() {
	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["optimize.enabled"] = "true"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["optimize.enabled"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:     "Optimize: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Template: "templates/optimize/deployment.yaml",
			Values:   valuesIRSA,
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
					"optimize must keep the documentstore-env-vars ConfigMap, its only AWS_REGION source")
			},
		},
		{
			Name:     "Optimize: AWS credentials SHOULD be injected when irsa.enabled is false",
			Template: "templates/optimize/deployment.yaml",
			Values:   valuesWithCredentials,
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
					"optimize must keep the documentstore-env-vars ConfigMap, its only AWS_REGION source")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerWebappNeverGetsDocumentStoreCreds() {
	values := awsDocumentStoreValuesWithIRSA(false)
	values["webModeler.enabled"] = "true"
	values["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:     "WebModeler Webapp: document-store AWS credentials and envFrom should never be injected",
			Template: "templates/web-modeler/deployment-webapp.yaml",
			Values:   values,
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
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *documentStoreIRSATest) TestWebModelerRestapiWithIRSA() {
	valuesIRSA := awsDocumentStoreValuesWithIRSA(true)
	valuesIRSA["webModeler.enabled"] = "true"
	valuesIRSA["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	valuesWithCredentials := awsDocumentStoreValuesWithIRSA(false)
	valuesWithCredentials["webModeler.enabled"] = "true"
	valuesWithCredentials["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:     "WebModeler REST API: AWS credentials should NOT be injected when irsa.enabled is true (IRSA mode)",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values:   valuesIRSA,
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
			Name:     "WebModeler REST API: AWS credentials SHOULD be injected when irsa.enabled is false",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values:   valuesWithCredentials,
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
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// SUPPORT-29235: enabling the AWS document store without configuring a secret used to render a
// secretKeyRef with an empty name, which Kubernetes rejects, failing the whole helm upgrade. Scoped
// to the components this chart no longer wires document-store config into.
func (s *documentStoreIRSATest) TestNoEmptySecretKeyRefWhenSecretUnset() {
	consoleValues := awsDocumentStoreValuesWithoutSecret()
	consoleValues["console.enabled"] = "true"

	webModelerValues := awsDocumentStoreValuesWithoutSecret()
	webModelerValues["webModeler.enabled"] = "true"
	webModelerValues["webModeler.restapi.mail.fromAddress"] = "test@example.com"

	testCases := []testhelpers.TestCase{
		{
			Name:     "Console: no empty secretKeyRef when the AWS document-store secret is unset",
			Template: "templates/console/deployment.yaml",
			Values:   consoleValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasEmptySecretKeyRefName(containers),
					"Console should not render a secretKeyRef with an empty name")
			},
		},
		{
			Name:     "Identity: no empty secretKeyRef when the AWS document-store secret is unset",
			Template: "templates/identity/deployment.yaml",
			Values:   awsDocumentStoreValuesWithoutSecret(),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containers := deployment.Spec.Template.Spec.Containers
				require.False(t, hasEmptySecretKeyRefName(containers),
					"Identity should not render a secretKeyRef with an empty name")
			},
		},
		{
			Name:     "Web Modeler webapp: no empty secretKeyRef when the AWS document-store secret is unset",
			Template: "templates/web-modeler/deployment-webapp.yaml",
			Values:   webModelerValues,
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

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

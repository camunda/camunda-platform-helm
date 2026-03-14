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
	netv1 "k8s.io/api/networking/v1"
)

// OpenShiftSecurityContextTest verifies that the OpenShift adaptSecurityContext=force
// feature correctly strips runAsUser, runAsGroup, and fsGroup from pod and container
// security contexts across all chart components.
type OpenShiftSecurityContextTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestOpenShiftSecurityContext(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &OpenShiftSecurityContextTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

// TestAdaptSecurityContextForce verifies that when adaptSecurityContext=force,
// runAsUser and fsGroup are stripped from pod and container security contexts
// on the Identity deployment, allowing OpenShift's restricted-v2 SCC to assign IDs.
func (s *OpenShiftSecurityContextTest) TestAdaptSecurityContextForce() {
	testCases := []testhelpers.TestCase{
		{
			Name: "IdentityPodSecurityContextStripsRunAsUserAndFsGroup",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "force",
				"identity.enabled": "true",
			},
			Template: "templates/identity/deployment.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				podSec := deployment.Spec.Template.Spec.SecurityContext
				require.NotNil(t, podSec, "pod security context should be present")
				require.Nil(t, podSec.RunAsUser,
					"pod RunAsUser should be stripped by OpenShift adaptSecurityContext=force")
				require.Nil(t, podSec.FSGroup,
					"pod FSGroup should be stripped by OpenShift adaptSecurityContext=force")
			},
		},
		{
			Name: "IdentityContainerSecurityContextStripsRunAsUser",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "force",
				"identity.enabled": "true",
			},
			Template: "templates/identity/deployment.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				containerSec := deployment.Spec.Template.Spec.Containers[0].SecurityContext
				require.NotNil(t, containerSec, "container security context should be present")
				require.Nil(t, containerSec.RunAsUser,
					"container RunAsUser should be stripped by OpenShift adaptSecurityContext=force")
			},
		},
		{
			Name: "OrchestrationPodSecurityContextStripsRunAsUserAndFsGroup",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "force",
			},
			Template: "templates/orchestration/statefulset.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				podSec := statefulSet.Spec.Template.Spec.SecurityContext
				require.NotNil(t, podSec, "pod security context should be present")
				require.Nil(t, podSec.RunAsUser,
					"pod RunAsUser should be stripped by OpenShift adaptSecurityContext=force")
				require.Nil(t, podSec.FSGroup,
					"pod FSGroup should be stripped by OpenShift adaptSecurityContext=force")
			},
		},
		{
			Name: "OrchestrationContainerSecurityContextStripsRunAsUser",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "force",
			},
			Template: "templates/orchestration/statefulset.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				containerSec := statefulSet.Spec.Template.Spec.Containers[0].SecurityContext
				require.NotNil(t, containerSec, "container security context should be present")
				require.Nil(t, containerSec.RunAsUser,
					"container RunAsUser should be stripped by OpenShift adaptSecurityContext=force")
			},
		},
		{
			Name: "DisabledAdaptSecurityContextPreservesRunAsUser",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
				"identity.enabled": "true",
			},
			Template: "templates/identity/deployment.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				podSec := deployment.Spec.Template.Spec.SecurityContext
				require.NotNil(t, podSec, "pod security context should be present")
				require.NotNil(t, podSec.FSGroup,
					"pod FSGroup should be preserved when adaptSecurityContext=disabled")
				require.EqualValues(t, 1001, *podSec.FSGroup,
					"pod FSGroup should be the default 1001 value when not adapted")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

// TestAdaptSecurityContextConstraints verifies that the chart fails fast with
// a clear error when an invalid adaptSecurityContext value is provided.
func (s *OpenShiftSecurityContextTest) TestAdaptSecurityContextConstraints() {
	testCases := []testhelpers.TestCase{
		{
			Name: "InvalidAdaptSecurityContextValueFails",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "auto",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err,
					"Chart rendering should fail when adaptSecurityContext has an invalid value")
				require.Contains(t, err.Error(), "Invalid value for adaptSecurityContext",
					"Error message should indicate which value is invalid")
			},
		},
		{
			Name: "ForceAdaptSecurityContextIsValid",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "force",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err, "Chart rendering should succeed with adaptSecurityContext=force")
			},
		},
		{
			Name: "DisabledAdaptSecurityContextIsValid",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err, "Chart rendering should succeed with adaptSecurityContext=disabled")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

// OpenShiftIngressTLSTest verifies that the OpenShift Ingress Operator automatic
// TLS certificate management works correctly via the secretName="-" sentinel value.
type OpenShiftIngressTLSTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestOpenShiftIngressTLS(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &OpenShiftIngressTLSTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

// TestHttpIngressOpenShiftTLSAutoManagement verifies that when tls.secretName is
// set to the sentinel value "-", the HTTP ingress renders an empty TLS block
// (tls: [{}]) which triggers the OpenShift Ingress Operator to manage the cert.
func (s *OpenShiftIngressTLSTest) TestHttpIngressOpenShiftTLSAutoManagement() {
	testCases := []testhelpers.TestCase{
		{
			Name: "EmptyTLSBlockRenderedWhenSecretNameIsDash",
			Values: map[string]string{
				"global.ingress.enabled":                              "true",
				"global.ingress.tls.enabled":                          "true",
				"global.ingress.tls.secretName":                       "-",
				"global.ingress.host":                                 "camunda.example.com",
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
			},
			Template: "templates/common/ingress-http.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				require.Len(t, ingress.Spec.TLS, 1,
					"Should have exactly one TLS entry")
				require.Empty(t, ingress.Spec.TLS[0].Hosts,
					"TLS entry should have no explicit hosts for OpenShift auto-management")
				require.Empty(t, ingress.Spec.TLS[0].SecretName,
					"TLS entry should have no secret name for OpenShift auto-management")
			},
		},
		{
			Name: "NormalTLSBlockRenderedWhenSecretNameIsSet",
			Values: map[string]string{
				"global.ingress.enabled":                              "true",
				"global.ingress.tls.enabled":                          "true",
				"global.ingress.tls.secretName":                       "my-tls-secret",
				"global.ingress.host":                                 "camunda.example.com",
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
			},
			Template: "templates/common/ingress-http.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				require.Len(t, ingress.Spec.TLS, 1,
					"Should have exactly one TLS entry")
				require.Equal(t, "my-tls-secret", ingress.Spec.TLS[0].SecretName,
					"TLS entry should use the provided secret name")
				require.Contains(t, ingress.Spec.TLS[0].Hosts, "camunda.example.com",
					"TLS entry should include the configured host")
			},
		},
		{
			Name: "NoTLSBlockWhenTLSDisabled",
			Values: map[string]string{
				"global.ingress.enabled":                              "true",
				"global.ingress.tls.enabled":                          "false",
				"global.ingress.host":                                 "camunda.example.com",
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
			},
			Template: "templates/common/ingress-http.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "tls:",
					"No TLS section should be rendered when tls.enabled=false")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

// TestGrpcIngressOpenShiftTLSAutoManagement verifies that the gRPC ingress also
// supports the OpenShift Ingress Operator automatic TLS via secretName="-".
func (s *OpenShiftIngressTLSTest) TestGrpcIngressOpenShiftTLSAutoManagement() {
	testCases := []testhelpers.TestCase{
		{
			Name: "GrpcEmptyTLSBlockRenderedWhenSecretNameIsDash",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"orchestration.ingress.grpc.enabled":                  "true",
				"orchestration.ingress.grpc.tls.enabled":              "true",
				"orchestration.ingress.grpc.tls.secretName":           "-",
				"orchestration.ingress.grpc.host":                     "grpc.example.com",
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
			},
			Template: "templates/common/ingress-grpc.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				require.Len(t, ingress.Spec.TLS, 1,
					"Should have exactly one TLS entry")
				require.Empty(t, ingress.Spec.TLS[0].Hosts,
					"gRPC TLS entry should have no explicit hosts for OpenShift auto-management")
				require.Empty(t, ingress.Spec.TLS[0].SecretName,
					"gRPC TLS entry should have no secret name for OpenShift auto-management")
			},
		},
		{
			Name: "GrpcNormalTLSBlockRenderedWhenSecretNameIsSet",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"orchestration.ingress.grpc.enabled":                  "true",
				"orchestration.ingress.grpc.tls.enabled":              "true",
				"orchestration.ingress.grpc.tls.secretName":           "my-grpc-secret",
				"orchestration.ingress.grpc.host":                     "grpc.example.com",
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
			},
			Template: "templates/common/ingress-grpc.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				require.Len(t, ingress.Spec.TLS, 1,
					"Should have exactly one TLS entry")
				require.Equal(t, "my-grpc-secret", ingress.Spec.TLS[0].SecretName,
					"gRPC TLS entry should use the provided secret name")
			},
		},
		{
			Name: "GrpcNoTLSBlockWhenTLSDisabled",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"orchestration.ingress.grpc.enabled":                  "true",
				"orchestration.ingress.grpc.tls.enabled":              "false",
				"orchestration.ingress.grpc.host":                     "grpc.example.com",
				"global.compatibility.openshift.adaptSecurityContext": "disabled",
			},
			Template: "templates/common/ingress-grpc.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "tls:",
					"No TLS section should be rendered when tls.enabled=false")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

// OpenShiftCombinedFeaturesTest verifies that all OpenShift features can be
// enabled simultaneously and the chart renders correctly end-to-end.
type OpenShiftCombinedFeaturesTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestOpenShiftCombinedFeatures(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &OpenShiftCombinedFeaturesTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

// TestAllOpenShiftFeaturesEnabled verifies that OpenShift features can be
// enabled simultaneously and the chart renders correctly end-to-end.
func (s *OpenShiftCombinedFeaturesTest) TestAllOpenShiftFeaturesEnabled() {
	testCases := []testhelpers.TestCase{
		{
			Name: "ChartRendersWithOpenShiftValuesOverlay",
			Values: map[string]string{
				"global.compatibility.openshift.adaptSecurityContext": "force",
				"global.ingress.enabled":                              "true",
				"global.ingress.tls.enabled":                          "true",
				"global.ingress.tls.secretName":                       "-",
				"global.ingress.host":                                 "camunda.example.com",
				"elasticsearch.enabled":                               "true",
			},
			Template: "templates/common/ingress-http.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err,
					"Chart should render successfully with all OpenShift features enabled")

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)

				require.Len(t, ingress.Spec.TLS, 1)
				require.Empty(t, ingress.Spec.TLS[0].Hosts,
					"OpenShift auto-managed TLS should produce an empty hosts list")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}

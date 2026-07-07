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

package identity

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

type PersistenceTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestPersistenceTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &PersistenceTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{
			"templates/identity/deployment.yaml",
		},
	})
}

func (s *PersistenceTemplateTest) TestPersistenceConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestPersistenceDisabledUsesEmptyDir",
			Values: map[string]string{
				"identity.enabled": "true",
				// persistence.enabled defaults to false
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				tmpVolume := tmpVolume(t, deployment)
				require.NotNil(t, tmpVolume.EmptyDir, "should use emptyDir when persistence is disabled")
				require.Nil(t, tmpVolume.PersistentVolumeClaim, "should not use PVC when persistence is disabled")
				require.Nil(t, tmpVolume.Ephemeral, "should not use ephemeral volume when persistence is disabled")
			},
		},
		{
			Name: "TestPersistenceEnabledCreatesVolume",
			Values: map[string]string{
				"identity.enabled":                    "true",
				"identity.persistence.enabled":        "true",
				"identity.persistence.size":           "5Gi",
				"identity.persistence.accessModes[0]": "ReadWriteOnce",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				tmpVolume := tmpVolume(t, deployment)
				require.NotNil(t, tmpVolume.Ephemeral, "should use a per-pod ephemeral volume when persistence is enabled")
				require.Nil(t, tmpVolume.EmptyDir, "should not use emptyDir when persistence is enabled")
				require.Nil(t, tmpVolume.PersistentVolumeClaim, "should not reference a shared PVC when persistence is enabled")
				spec := tmpVolume.Ephemeral.VolumeClaimTemplate.Spec
				require.Equal(t, "5Gi", spec.Resources.Requests.Storage().String())
				require.Equal(t, corev1.ReadWriteOnce, spec.AccessModes[0])
			},
		},
		{
			Name: "TestPersistenceWithExistingClaimCreatesVolume",
			Values: map[string]string{
				"identity.enabled":                   "true",
				"identity.persistence.enabled":       "true",
				"identity.persistence.existingClaim": "my-existing-pvc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				tmpVolume := tmpVolume(t, deployment)
				require.NotNil(t, tmpVolume.PersistentVolumeClaim, "should use PVC when existingClaim is set")
				require.Equal(t, "my-existing-pvc", tmpVolume.PersistentVolumeClaim.ClaimName)
				require.Nil(t, tmpVolume.Ephemeral, "should not use ephemeral volume when existingClaim is set")
				require.Nil(t, tmpVolume.EmptyDir, "should not use emptyDir when existingClaim is set")
			},
		},
		{
			Name: "TestPersistenceWithStorageClassCreatesVolume",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"identity.persistence.enabled":          "true",
				"identity.persistence.size":             "10Gi",
				"identity.persistence.storageClassName": "fast-ssd",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				spec := tmpVolume(t, deployment).Ephemeral.VolumeClaimTemplate.Spec
				require.Equal(t, "10Gi", spec.Resources.Requests.Storage().String())
				require.Equal(t, "fast-ssd", *spec.StorageClassName)
			},
		},
		{
			Name: "TestPersistenceWithAnnotationsCreatesVolume",
			Values: map[string]string{
				"identity.enabled":                     "true",
				"identity.persistence.enabled":         "true",
				"identity.persistence.size":            "5Gi",
				"identity.persistence.annotations.foo": "bar",
				"identity.persistence.annotations.baz": "qux",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				annotations := tmpVolume(t, deployment).Ephemeral.VolumeClaimTemplate.ObjectMeta.Annotations
				require.Equal(t, "bar", annotations["foo"])
				require.Equal(t, "qux", annotations["baz"])
			},
		},
		{
			Name: "TestPersistenceWithSelectorCreatesVolume",
			Values: map[string]string{
				"identity.enabled":                                  "true",
				"identity.persistence.enabled":                      "true",
				"identity.persistence.size":                         "5Gi",
				"identity.persistence.selector.matchLabels.storage": "fast",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				spec := tmpVolume(t, deployment).Ephemeral.VolumeClaimTemplate.Spec
				require.Equal(t, "5Gi", spec.Resources.Requests.Storage().String())
				require.Equal(t, "fast", spec.Selector.MatchLabels["storage"])
			},
		},
		{
			Name: "TestPersistenceDisabledWhenComponentDisabled",
			Values: map[string]string{
				"identity.enabled":             "false",
				"identity.persistence.enabled": "true",
				"identity.persistence.size":    "5Gi",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Empty(t, output, "no deployment should be created when component is disabled")
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		s.Run(testCase.Name, func() {
			s.T().Parallel()
			helmChartPath, err := filepath.Abs(s.chartPath)
			s.Require().NoError(err)

			mergedValues := make(map[string]string)
			mergedValues["global.elasticsearch.enabled"] = "true"
			for k, v := range testCase.Values {
				mergedValues[k] = v
			}

			output, err := helm.RenderTemplateE(
				s.T(),
				&helm.Options{
					SetValues: mergedValues,
				},
				helmChartPath,
				s.release,
				s.templates,
			)

			testCase.Verifier(s.T(), output, err)
		})
	}
}

func TestDeploymentStrategyDefaultsToRollingUpdate(t *testing.T) {
	t.Parallel()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	testCase := testhelpers.TestCase{
		Name: "TestDeploymentStrategyDefaultsToRollingUpdate",
		Values: map[string]string{
			"identity.enabled":             "true",
			"global.elasticsearch.enabled": "true",
		},
		Verifier: func(t *testing.T, output string, err error) {
			var deployment appsv1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)
			require.Equal(t, appsv1.RollingUpdateDeploymentStrategyType, deployment.Spec.Strategy.Type)
		},
	}
	testhelpers.RunTestCasesE(t, chartPath, "camunda-platform-test", "camunda-platform-identity", []string{"templates/identity/deployment.yaml"}, []testhelpers.TestCase{testCase})
}

func TestDeploymentStrategyRecreateOptIn(t *testing.T) {
	t.Parallel()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	testCase := testhelpers.TestCase{
		Name: "TestDeploymentStrategyRecreateOptIn",
		Values: map[string]string{
			"identity.enabled":                        "true",
			"global.elasticsearch.enabled":            "true",
			"identity.persistence.enabled":            "true",
			"identity.persistence.deploymentStrategy": "Recreate",
		},
		Verifier: func(t *testing.T, output string, err error) {
			var deployment appsv1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)
			require.Equal(t, appsv1.RecreateDeploymentStrategyType, deployment.Spec.Strategy.Type)
		},
	}
	testhelpers.RunTestCasesE(t, chartPath, "camunda-platform-test", "camunda-platform-identity", []string{"templates/identity/deployment.yaml"}, []testhelpers.TestCase{testCase})
}

func TestDeploymentStrategyInvalidValueFails(t *testing.T) {
	t.Parallel()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	testCase := testhelpers.TestCase{
		Name: "TestDeploymentStrategyInvalidValueFails",
		Values: map[string]string{
			"identity.enabled":                        "true",
			"global.elasticsearch.enabled":            "true",
			"identity.persistence.deploymentStrategy": "Bogus",
		},
		Verifier: func(t *testing.T, output string, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "value must be one of 'RollingUpdate', 'Recreate'")
		},
	}
	testhelpers.RunTestCasesE(t, chartPath, "camunda-platform-test", "camunda-platform-identity", []string{"templates/identity/deployment.yaml"}, []testhelpers.TestCase{testCase})
}

func tmpVolume(t *testing.T, deployment appsv1.Deployment) *corev1.Volume {
	t.Helper()

	for i := range deployment.Spec.Template.Spec.Volumes {
		if deployment.Spec.Template.Spec.Volumes[i].Name == "tmp" {
			return &deployment.Spec.Template.Spec.Volumes[i]
		}
	}

	require.FailNow(t, "tmp volume should exist")
	return nil
}

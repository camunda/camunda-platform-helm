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

package optimize

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

type DeploymentTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestDeploymentTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &DeploymentTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/optimize/deployment.yaml"},
	})
}

func (s *DeploymentTemplateTest) TestAutomountServiceAccountToken() {
	baseValues := func() map[string]string {
		return map[string]string{
			"identity.enabled": "true",
			"optimize.enabled": "true",
		}
	}

	disabledValues := baseValues()
	disabledValues["optimize.automountServiceAccountToken"] = "false"
	enabledValues := baseValues()
	enabledValues["optimize.automountServiceAccountToken"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestPodOmitsAutomountServiceAccountTokenByDefault",
			Values: baseValues(),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				require.Nil(t, deployment.Spec.Template.Spec.AutomountServiceAccountToken)
			},
		}, {
			Name:   "TestPodDisablesAutomountServiceAccountToken",
			Values: disabledValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				require.NotNil(t, deployment.Spec.Template.Spec.AutomountServiceAccountToken)
				require.False(t, *deployment.Spec.Template.Spec.AutomountServiceAccountToken)
			},
		}, {
			Name:   "TestPodEnablesAutomountServiceAccountToken",
			Values: enabledValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				require.NotNil(t, deployment.Spec.Template.Spec.AutomountServiceAccountToken)
				require.True(t, *deployment.Spec.Template.Spec.AutomountServiceAccountToken)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *DeploymentTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerSetPodLabels",
			Values: map[string]string{
				"identity.enabled":       "true",
				"optimize.enabled":       "true",
				"optimize.podLabels.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				s.Require().Equal("bar", deployment.Spec.Template.Labels["foo"])
			},
		}, {
			Name: "TestContainerSetPodAnnotations",
			Values: map[string]string{
				"identity.enabled":            "true",
				"optimize.enabled":            "true",
				"optimize.podAnnotations.foo": "bar",
				"optimize.podAnnotations.foz": "baz",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				s.Require().Equal("bar", deployment.Spec.Template.Annotations["foo"])
				s.Require().Equal("baz", deployment.Spec.Template.Annotations["foz"])
			},
		}, {
			Name:        "TestContainerSetPodLabelsAndAnnotationsWithTemplating",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-templated-labels.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then - verify templating is evaluated
				s.Require().Equal("camunda-platform-test", deployment.Spec.Template.Labels["release"])
				s.Require().Equal("camunda-platform-test", deployment.Spec.Template.Annotations["release"])
			},
		}, {
			Name: "TestContainerSetGlobalAnnotations",
			Values: map[string]string{
				"identity.enabled":       "true",
				"optimize.enabled":       "true",
				"global.annotations.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				s.Require().Equal("bar", deployment.ObjectMeta.Annotations["foo"])
			},
		}, {
			Name: "TestContainerSetImageNameSubChart",
			Values: map[string]string{
				"identity.enabled":          "true",
				"global.image.registry":     "global.custom.registry.io",
				"optimize.enabled":          "true",
				"optimize.image.registry":   "subchart.custom.registry.io",
				"optimize.image.repository": "camunda/optimize-test",
				"optimize.image.tag":        "snapshot",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				container := deployment.Spec.Template.Spec.Containers[0]
				s.Require().Equal(container.Image, "subchart.custom.registry.io/camunda/optimize-test:snapshot")
			},
		}, {
			Name: "TestContainerSetImagePullSecretsGlobal",
			Values: map[string]string{
				"identity.enabled":                 "true",
				"optimize.enabled":                 "true",
				"global.image.pullSecrets[0].name": "SecretName",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				s.Require().Equal("SecretName", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
			},
		}, {
			Name: "TestContainerSetImagePullSecretsSubChart",
			Values: map[string]string{
				"identity.enabled":                   "true",
				"optimize.enabled":                   "true",
				"global.image.pullSecrets[0].name":   "SecretName",
				"optimize.image.pullSecrets[0].name": "SecretNameSubChart",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				s.Require().Equal("SecretNameSubChart", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
			},
		}, {
			Name:                 "TestContainerOverwriteImageTag",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":   "true",
				"optimize.enabled":   "true",
				"optimize.image.tag": "a.b.c",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				expectedContainerImage := "camunda/optimize:a.b.c"
				containers := deployment.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				s.Require().Equal(expectedContainerImage, containers[0].Image)
			},
		}, {
			Name:                 "TestContainerOverwriteImageTagWithChartDirectSetting",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":   "true",
				"optimize.image.tag": "a.b.c",
				"optimize.enabled":   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				expectedContainerImage := "camunda/optimize:a.b.c"
				containers := deployment.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				s.Require().Equal(expectedContainerImage, containers[0].Image)
			},
		}, {
			Name:                 "TestContainerSetContainerCommand",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":    "true",
				"optimize.enabled":    "true",
				"optimize.command[0]": "printenv",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				containers := deployment.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				s.Require().Equal(1, len(containers[0].Command))
				s.Require().Equal("printenv", containers[0].Command[0])
			},
		}, {
			Name:                 "TestContainerSetExtraVolumes",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                               "true",
				"optimize.enabled":                               "true",
				"optimize.extraVolumes[0].name":                  "extraVolume",
				"optimize.extraVolumes[0].configMap.name":        "otherConfigMap",
				"optimize.extraVolumes[0].configMap.defaultMode": "744",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of volumes array before addition of new volume
				var deploymentBefore appsv1.Deployment
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates, "--set", "identity.enabled=true", "--set", "optimize.enabled=true", "--set", "orchestration.data.secondaryStorage.type=elasticsearch")
				helm.UnmarshalK8SYaml(s.T(), before, &deploymentBefore)
				volumeLenBefore := len(deploymentBefore.Spec.Template.Spec.Volumes)
				// given
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				volumes := deployment.Spec.Template.Spec.Volumes
				s.Require().Equal(volumeLenBefore+1, len(volumes))

				extraVolume := volumes[volumeLenBefore]
				s.Require().Equal("extraVolume", extraVolume.Name)
				s.Require().NotNil(*extraVolume.ConfigMap)
				s.Require().Equal("otherConfigMap", extraVolume.ConfigMap.Name)
				s.Require().EqualValues(744, *extraVolume.ConfigMap.DefaultMode)
			},
		}, {
			Name:                 "TestContainerSetExtraVolumeMounts",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                        "true",
				"optimize.enabled":                        "true",
				"optimize.extraVolumeMounts[0].name":      "otherConfigMap",
				"optimize.extraVolumeMounts[0].mountPath": "/usr/local/config",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var deploymentBefore appsv1.Deployment
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates, "--set", "identity.enabled=true", "--set", "optimize.enabled=true", "--set", "orchestration.data.secondaryStorage.type=elasticsearch")
				helm.UnmarshalK8SYaml(s.T(), before, &deploymentBefore)
				containerLenBefore := len(deploymentBefore.Spec.Template.Spec.Containers)
				volumeMountLenBefore := len(deploymentBefore.Spec.Template.Spec.Containers[0].VolumeMounts)
				// given
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				containers := deployment.Spec.Template.Spec.Containers
				s.Require().Equal(containerLenBefore, len(containers))

				volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
				s.Require().Equal(volumeMountLenBefore+1, len(volumeMounts))
				extraVolumeMount := volumeMounts[volumeMountLenBefore]
				s.Require().Equal("otherConfigMap", extraVolumeMount.Name)
				s.Require().Equal("/usr/local/config", extraVolumeMount.MountPath)
			},
		}, {
			Name: "TestContainerSetExtraVolumesAndMounts",
			Values: map[string]string{
				"identity.enabled":                               "true",
				"optimize.enabled":                               "true",
				"optimize.extraVolumeMounts[0].name":             "otherConfigMap",
				"optimize.extraVolumeMounts[0].mountPath":        "/usr/local/config",
				"optimize.extraVolumes[0].name":                  "extraVolume",
				"optimize.extraVolumes[0].configMap.name":        "otherConfigMap",
				"optimize.extraVolumes[0].configMap.defaultMode": "744",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of volumes, volumemounts array before addition of new volume
				var deploymentBefore appsv1.Deployment
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates, "--set", "optimize.enabled=true", "--set", "identity.enabled=true", "--set", "orchestration.data.secondaryStorage.type=elasticsearch")
				helm.UnmarshalK8SYaml(s.T(), before, &deploymentBefore)
				volumeLenBefore := len(deploymentBefore.Spec.Template.Spec.Volumes)
				volumeMountLenBefore := len(deploymentBefore.Spec.Template.Spec.Containers[0].VolumeMounts)
				containerLenBefore := len(deploymentBefore.Spec.Template.Spec.Containers)
				// given
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				volumes := deployment.Spec.Template.Spec.Volumes
				s.Require().Equal(volumeLenBefore+1, len(volumes))

				extraVolume := volumes[volumeLenBefore]
				s.Require().Equal("extraVolume", extraVolume.Name)
				s.Require().NotNil(*extraVolume.ConfigMap)
				s.Require().Equal("otherConfigMap", extraVolume.ConfigMap.Name)
				s.Require().EqualValues(744, *extraVolume.ConfigMap.DefaultMode)

				containers := deployment.Spec.Template.Spec.Containers
				s.Require().Equal(containerLenBefore, len(containers))

				volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
				s.Require().Equal(volumeMountLenBefore+1, len(volumeMounts))
				extraVolumeMount := volumeMounts[volumeMountLenBefore]
				s.Require().Equal("otherConfigMap", extraVolumeMount.Name)
				s.Require().Equal("/usr/local/config", extraVolumeMount.MountPath)
			},
		}, {
			Name:                 "TestContainerSetServiceAccountName",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":             "true",
				"optimize.enabled":             "true",
				"optimize.serviceAccount.name": "accName",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				serviceAccName := deployment.Spec.Template.Spec.ServiceAccountName
				s.Require().Equal("accName", serviceAccName)
			},
		}, {
			Name: "TestPodSetSecurityContext",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"optimize.enabled":                      "true",
				"optimize.podSecurityContext.runAsUser": "1000",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				securityContext := deployment.Spec.Template.Spec.SecurityContext
				s.Require().EqualValues(1000, *securityContext.RunAsUser)
			},
		}, {
			Name: "TestContainerSetSecurityContext",
			Values: map[string]string{
				"identity.enabled": "true",
				"optimize.enabled": "true",
				"optimize.containerSecurityContext.privileged": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				securityContext := deployment.Spec.Template.Spec.Containers[0].SecurityContext
				s.Require().True(*securityContext.Privileged)
			},
		}, {
			// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
			Name: "TestContainerSetNodeSelector",
			Values: map[string]string{
				"identity.enabled":               "true",
				"optimize.enabled":               "true",
				"optimize.nodeSelector.disktype": "ssd",
				"optimize.nodeSelector.cputype":  "arm",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				s.Require().Equal("ssd", deployment.Spec.Template.Spec.NodeSelector["disktype"])
				s.Require().Equal("arm", deployment.Spec.Template.Spec.NodeSelector["cputype"])
			},
		}, {
			// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity
			// affinity:
			//	nodeAffinity:
			//	 requiredDuringSchedulingIgnoredDuringExecution:
			//	   nodeSelectorTerms:
			//	   - matchExpressions:
			//		 - key: kubernetes.io/e2e-az-name
			//		   operator: In
			//		   values:
			//		   - e2e-az1
			//		   - e2e-az2
			//	 preferredDuringSchedulingIgnoredDuringExecution:
			//	 - weight: 1
			//	   preference:
			//		 matchExpressions:
			//		 - key: another-node-label-key
			//		   operator: In
			//		   values:
			//		   - another-node-label-value
			Name: "TestContainerSetAffinity",
			Values: map[string]string{
				"identity.enabled": "true",
				"optimize.enabled": "true",
				"optimize.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].key":       "kubernetes.io/e2e-az-name",
				"optimize.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].operator":  "In",
				"optimize.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[0]": "e2e-a1",
				"optimize.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[1]": "e2e-a2",
				"optimize.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight":                                         "1",
				"optimize.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].key":             "another-node-label-key",
				"optimize.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].operator":        "In",
				"optimize.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].values[0]":       "another-node-label-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				nodeAffinity := deployment.Spec.Template.Spec.Affinity.NodeAffinity
				s.Require().NotNil(nodeAffinity)

				nodeSelectorTerm := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
				s.Require().NotNil(nodeSelectorTerm)
				matchExpression := nodeSelectorTerm.MatchExpressions[0]
				s.Require().NotNil(matchExpression)
				s.Require().Equal("kubernetes.io/e2e-az-name", matchExpression.Key)
				s.Require().EqualValues("In", matchExpression.Operator)
				s.Require().Equal([]string{"e2e-a1", "e2e-a2"}, matchExpression.Values)

				preferredSchedulingTerm := nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0]
				s.Require().NotNil(preferredSchedulingTerm)

				matchExpression = preferredSchedulingTerm.Preference.MatchExpressions[0]
				s.Require().NotNil(matchExpression)
				s.Require().Equal("another-node-label-key", matchExpression.Key)
				s.Require().EqualValues("In", matchExpression.Operator)
				s.Require().Equal([]string{"another-node-label-value"}, matchExpression.Values)
			},
		}, {
			// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration
			//tolerations:
			//- key: "key1"
			//  operator: "Equal"
			//  value: "value1"
			//  effect: "NoSchedule"
			Name:                 "TestContainerSetTolerations",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                 "true",
				"optimize.enabled":                 "true",
				"optimize.tolerations[0].key":      "key1",
				"optimize.tolerations[0].operator": "Equal",
				"optimize.tolerations[0].value":    "Value1",
				"optimize.tolerations[0].effect":   "NoSchedule",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				tolerations := deployment.Spec.Template.Spec.Tolerations
				s.Require().Equal(1, len(tolerations))

				toleration := tolerations[0]
				s.Require().Equal("key1", toleration.Key)
				s.Require().EqualValues("Equal", toleration.Operator)
				s.Require().Equal("Value1", toleration.Value)
				s.Require().EqualValues("NoSchedule", toleration.Effect)
			},
		}, {
			Name:                 "TestContainerShouldSetOptimizeIdentitySecretValue",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                                       "true",
				"optimize.enabled":                                       "true",
				"global.identity.auth.enabled":                           "true",
				"global.identity.auth.optimize.secret.existingSecret":    "camunda-platform-test-optimize-identity-secret",
				"global.identity.auth.optimize.secret.existingSecretKey": "identity-optimize-client-token",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				env := deployment.Spec.Template.Spec.Containers[0].Env
				secretRef := &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "camunda-platform-test-optimize-identity-secret"},
						Key:                  "identity-optimize-client-token",
					},
				}
				s.Require().Contains(env,
					corev1.EnvVar{Name: "CAMUNDA_IDENTITY_CLIENT_SECRET", ValueFrom: secretRef})
				s.Require().Contains(env,
					corev1.EnvVar{Name: "VALUES_OPTIMIZE_CLIENT_SECRET", ValueFrom: secretRef})
			},
		}, {
			Name:                 "TestContainerShouldSetOptimizeIdentitySecretViaReference",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                                       "true",
				"optimize.enabled":                                       "true",
				"global.identity.auth.enabled":                           "true",
				"global.identity.auth.optimize.secret.existingSecret":    "ownExistingSecret",
				"global.identity.auth.optimize.secret.existingSecretKey": "identity-optimize-client-token",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				env := deployment.Spec.Template.Spec.Containers[0].Env
				secretRef := &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "ownExistingSecret"},
						Key:                  "identity-optimize-client-token",
					},
				}
				s.Require().Contains(env,
					corev1.EnvVar{Name: "CAMUNDA_IDENTITY_CLIENT_SECRET", ValueFrom: secretRef})
				s.Require().Contains(env,
					corev1.EnvVar{Name: "VALUES_OPTIMIZE_CLIENT_SECRET", ValueFrom: secretRef})
			},
		}, {
			Name: "TestContainerShouldOverwriteGlobalImagePullPolicy",
			Values: map[string]string{
				"identity.enabled":        "true",
				"optimize.enabled":        "true",
				"global.image.pullPolicy": "Always",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				expectedPullPolicy := corev1.PullAlways
				containers := deployment.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				pullPolicy := containers[0].ImagePullPolicy
				s.Require().Equal(expectedPullPolicy, pullPolicy)
			},
		}, {
			// readinessProbe is enabled by default so it's tested by golden files.
			Name:                 "TestContainerStartupProbe",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                          "true",
				"optimize.enabled":                          "true",
				"optimize.startupProbe.enabled":             "true",
				"optimize.startupProbe.probePath":           "/healthz",
				"optimize.startupProbe.initialDelaySeconds": "5",
				"optimize.startupProbe.periodSeconds":       "10",
				"optimize.startupProbe.successThreshold":    "1",
				"optimize.startupProbe.failureThreshold":    "5",
				"optimize.startupProbe.timeoutSeconds":      "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0].StartupProbe

				s.Require().Equal("/healthz", probe.HTTPGet.Path)
				s.Require().EqualValues(5, probe.InitialDelaySeconds)
				s.Require().EqualValues(10, probe.PeriodSeconds)
				s.Require().EqualValues(1, probe.SuccessThreshold)
				s.Require().EqualValues(5, probe.FailureThreshold)
				s.Require().EqualValues(1, probe.TimeoutSeconds)
			},
		}, {
			Name:                 "TestContainerLivenessProbe",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                           "true",
				"optimize.enabled":                           "true",
				"optimize.livenessProbe.enabled":             "true",
				"optimize.livenessProbe.probePath":           "/healthz",
				"optimize.livenessProbe.initialDelaySeconds": "5",
				"optimize.livenessProbe.periodSeconds":       "10",
				"optimize.livenessProbe.successThreshold":    "1",
				"optimize.livenessProbe.failureThreshold":    "5",
				"optimize.livenessProbe.timeoutSeconds":      "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0].LivenessProbe

				s.Require().EqualValues("/healthz", probe.HTTPGet.Path)
				s.Require().EqualValues(5, probe.InitialDelaySeconds)
				s.Require().EqualValues(10, probe.PeriodSeconds)
				s.Require().EqualValues(1, probe.SuccessThreshold)
				s.Require().EqualValues(5, probe.FailureThreshold)
				s.Require().EqualValues(1, probe.TimeoutSeconds)
			},
		}, {
			Name:                 "TestContainerProbesWithContextPath",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                  "true",
				"optimize.enabled":                  "true",
				"optimize.contextPath":              "/test",
				"optimize.startupProbe.enabled":     "true",
				"optimize.startupProbe.probePath":   "/start",
				"optimize.readinessProbe.enabled":   "true",
				"optimize.readinessProbe.probePath": "/ready",
				"optimize.livenessProbe.enabled":    "true",
				"optimize.livenessProbe.probePath":  "/live",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0]

				s.Require().Equal("/test/start", probe.StartupProbe.HTTPGet.Path)
				s.Require().Equal("/test/ready", probe.ReadinessProbe.HTTPGet.Path)
				s.Require().Equal("/test/live", probe.LivenessProbe.HTTPGet.Path)
			},
		}, {
			Name:                 "TestContainerProbesWithContextPathWithTrailingSlash",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"identity.enabled":                  "true",
				"optimize.enabled":                  "true",
				"optimize.contextPath":              "/test/",
				"optimize.startupProbe.enabled":     "true",
				"optimize.startupProbe.probePath":   "/start",
				"optimize.readinessProbe.enabled":   "true",
				"optimize.readinessProbe.probePath": "/ready",
				"optimize.livenessProbe.enabled":    "true",
				"optimize.livenessProbe.probePath":  "/live",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				probe := deployment.Spec.Template.Spec.Containers[0]

				s.Require().Equal("/test/start", probe.StartupProbe.HTTPGet.Path)
				s.Require().Equal("/test/ready", probe.ReadinessProbe.HTTPGet.Path)
				s.Require().Equal("/test/live", probe.LivenessProbe.HTTPGet.Path)
			},
		}, {
			Name: "TestContainerSetSidecar",
			Values: map[string]string{
				"identity.enabled":                            "true",
				"optimize.enabled":                            "true",
				"optimize.sidecars[0].name":                   "nginx",
				"optimize.sidecars[0].image":                  "nginx:latest",
				"optimize.sidecars[0].ports[0].containerPort": "80",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				podContainers := deployment.Spec.Template.Spec.Containers
				expectedContainer := corev1.Container{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				}

				s.Require().Contains(podContainers, expectedContainer)
			},
		}, {
			Name: "TestInitContainers",
			Values: map[string]string{
				"identity.enabled":                                  "true",
				"optimize.enabled":                                  "true",
				"optimize.initContainers[0].name":                   "nginx",
				"optimize.initContainers[0].image":                  "nginx:latest",
				"optimize.initContainers[0].ports[0].containerPort": "80",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				podContainers := deployment.Spec.Template.Spec.InitContainers
				expectedContainer := corev1.Container{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				}

				s.Require().Contains(podContainers, expectedContainer)
			},
		}, {
			Name: "TestOptimizeMultiTenancyEnabled",
			Values: map[string]string{
				"identity.enabled":                   "true",
				"global.identity.auth.enabled":       "true",
				"optimize.enabled":                   "true",
				"identity.multitenancy.enabled":      "true",
				"identity.externalDatabase.enabled":  "true",
				"identity.externalDatabase.host":     "my-database-host",
				"identity.externalDatabase.username": "my-database-username",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_MULTITENANCY_ENABLED", Value: "true"})
			},
		}, {
			Name: "TestOptimizeWithConfiguration",
			Values: map[string]string{
				"identity.enabled": "true",
				"optimize.enabled": "true",
				"optimize.configuration": `
es:
  settings:
    index:
      number_of_shards: 3
			`,
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
				volumes := deployment.Spec.Template.Spec.Volumes

				// find the volume named environment-config
				var volume corev1.Volume
				for _, candidateVolume := range volumes {
					if candidateVolume.Name == "environment-config" {
						volume = candidateVolume
						break
					}
				}

				// find the volumeMount named environment-config
				var volumeMount corev1.VolumeMount
				for _, candidateVolumeMount := range volumeMounts {
					if candidateVolumeMount.Name == "environment-config" {
						volumeMount = candidateVolumeMount
						break
					}
				}
				s.Require().Equal("environment-config", volumeMount.Name)
				s.Require().Equal("/optimize/config/environment-config.yaml", volumeMount.MountPath)
				s.Require().Equal("environment-config.yaml", volumeMount.SubPath)

				s.Require().Equal("environment-config", volume.Name)
				s.Require().Equal("camunda-platform-test-optimize-configuration", volume.ConfigMap.Name)
			},
		}, {
			Name: "TestOptimizeWithLog4j2Configuration",
			Values: map[string]string{
				"identity.enabled":                       "true",
				"optimize.enabled":                       "true",
				"optimize.extraConfiguration[0].file":    "environment-logbackxml",
				"optimize.extraConfiguration[0].content": "<configuration></configuration>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
				volumes := deployment.Spec.Template.Spec.Volumes

				// find the volume named environment-config
				var volume corev1.Volume
				for _, candidateVolume := range volumes {
					if candidateVolume.Name == "environment-config" {
						volume = candidateVolume
						break
					}
				}

				// find the volumeMount named environment-config
				var volumeMount corev1.VolumeMount
				for _, candidateVolumeMount := range volumeMounts {
					if candidateVolumeMount.Name == "environment-config" && candidateVolumeMount.MountPath != "/optimize/config/environment-config.yaml" && candidateVolumeMount.MountPath != "/optimize/config/application-ccsm.yaml" {
						volumeMount = candidateVolumeMount
						break
					}
				}
				s.Require().Equal("environment-config", volumeMount.Name)
				s.Require().Equal("/optimize/config/environment-logbackxml", volumeMount.MountPath)
				s.Require().Equal("environment-logbackxml", volumeMount.SubPath)

				s.Require().Equal("environment-config", volume.Name)
				s.Require().Equal("camunda-platform-test-optimize-configuration", volume.ConfigMap.Name)
			},
		}, {
			Name: "TestSetDnsPolicyAndDnsConfig",
			Values: map[string]string{
				"identity.enabled":                  "true",
				"optimize.enabled":                  "true",
				"optimize.dnsPolicy":                "ClusterFirst",
				"optimize.dnsConfig.nameservers[0]": "8.8.8.8",
				"optimize.dnsConfig.searches[0]":    "example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// then
				// Check if dnsPolicy is set
				require.NotEmpty(s.T(), deployment.Spec.Template.Spec.DNSPolicy, "dnsPolicy should not be empty")

				// Check if dnsConfig is set
				require.NotNil(s.T(), deployment.Spec.Template.Spec.DNSConfig, "dnsConfig should not be nil")

				expectedDNSConfig := &corev1.PodDNSConfig{
					Nameservers: []string{"8.8.8.8"},
					Searches:    []string{"example.com"},
				}

				require.Equal(s.T(), expectedDNSConfig, deployment.Spec.Template.Spec.DNSConfig, "dnsConfig should match the expected configuration")
			},
		},
		{
			Name: "TestOptimizeCaBundlePreservesJavaOptsAndInjectsTrustConfiguration",
			Values: map[string]string{
				"identity.enabled":                             "true",
				"optimize.enabled":                             "true",
				"optimize.javaOpts":                            "-Xmx512m -Xms256m",
				"global.tls.caBundle.secret.existingSecret":    "ca-bundle-secret",
				"global.tls.caBundle.secret.existingSecretKey": "custom-ca.pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				var truststoreInitContainer *corev1.Container
				for i := range deployment.Spec.Template.Spec.InitContainers {
					if deployment.Spec.Template.Spec.InitContainers[i].Name == "ca-bundle-truststore-init" {
						truststoreInitContainer = &deployment.Spec.Template.Spec.InitContainers[i]
						break
					}
				}
				s.Require().NotNil(truststoreInitContainer)

				var optimizeContainer *corev1.Container
				for i := range deployment.Spec.Template.Spec.Containers {
					if deployment.Spec.Template.Spec.Containers[i].Name == "optimize" {
						optimizeContainer = &deployment.Spec.Template.Spec.Containers[i]
						break
					}
				}
				s.Require().NotNil(optimizeContainer)
				s.Require().Contains(optimizeContainer.Env, corev1.EnvVar{Name: "SSL_CERT_FILE", Value: "/etc/camunda/tls/ca.crt"})

				var javaToolOptions *corev1.EnvVar
				for i := range optimizeContainer.Env {
					if optimizeContainer.Env[i].Name == "JAVA_TOOL_OPTIONS" {
						javaToolOptions = &optimizeContainer.Env[i]
						break
					}
				}
				s.Require().NotNil(javaToolOptions)
				s.Require().Contains(javaToolOptions.Value, "-Xmx512m -Xms256m")
				s.Require().Contains(javaToolOptions.Value, "-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts")

				s.Require().Contains(optimizeContainer.VolumeMounts, corev1.VolumeMount{
					Name:      "ca-bundle",
					MountPath: "/etc/camunda/tls",
					ReadOnly:  true,
				})
				s.Require().Contains(optimizeContainer.VolumeMounts, corev1.VolumeMount{
					Name:      "ca-bundle-truststore",
					MountPath: "/var/camunda/tls-truststore",
					ReadOnly:  true,
				})

				var caBundleVolume *corev1.Volume
				var truststoreVolume *corev1.Volume
				for i := range deployment.Spec.Template.Spec.Volumes {
					switch deployment.Spec.Template.Spec.Volumes[i].Name {
					case "ca-bundle":
						caBundleVolume = &deployment.Spec.Template.Spec.Volumes[i]
					case "ca-bundle-truststore":
						truststoreVolume = &deployment.Spec.Template.Spec.Volumes[i]
					}
				}
				s.Require().NotNil(caBundleVolume)
				s.Require().NotNil(caBundleVolume.Secret)
				s.Require().Equal("ca-bundle-secret", caBundleVolume.Secret.SecretName)
				s.Require().Contains(caBundleVolume.Secret.Items, corev1.KeyToPath{Key: "custom-ca.pem", Path: "ca.crt"})
				s.Require().NotNil(truststoreVolume)
				s.Require().NotNil(truststoreVolume.EmptyDir)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *DeploymentTemplateTest) TestDatabaseOverrides() {
	testCases := []testhelpers.TestCase{
		{
			Name: "Component OpenSearch settings",
			Values: map[string]string{
				"identity.enabled":                                           "true",
				"optimize.enabled":                                           "true",
				"optimize.database.elasticsearch.enabled":                    "false",
				"optimize.database.opensearch.enabled":                       "true",
				"optimize.database.opensearch.url.protocol":                  "https",
				"optimize.database.opensearch.url.host":                      "opensearch-host",
				"optimize.database.opensearch.url.port":                      "9211",
				"optimize.database.opensearch.auth.username":                 "opensearch-user",
				"optimize.database.opensearch.auth.secret.existingSecret":    "opensearch-auth",
				"optimize.database.opensearch.auth.secret.existingSecretKey": "password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_HOST", Value: "opensearch-host"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_HTTP_PORT", Value: "9211"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SECURITY_USERNAME", Value: "opensearch-user"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SSL_ENABLED", Value: "true"})
				s.Require().Contains(env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SECURITY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "opensearch-auth"},
						Key:                  "password",
					}},
				})
			},
		},
		{
			Name: "Component OpenSearch HTTP settings omit SSL",
			Values: map[string]string{
				"identity.enabled":                                           "true",
				"optimize.enabled":                                           "true",
				"optimize.database.elasticsearch.enabled":                    "false",
				"optimize.database.opensearch.enabled":                       "true",
				"optimize.database.opensearch.url.protocol":                  "http",
				"optimize.database.opensearch.url.host":                      "component-os-host",
				"optimize.database.opensearch.url.port":                      "9222",
				"optimize.database.opensearch.auth.username":                 "component-user",
				"optimize.database.opensearch.auth.secret.existingSecret":    "component-os-auth",
				"optimize.database.opensearch.auth.secret.existingSecretKey": "component-password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_HOST", Value: "component-os-host"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_HTTP_PORT", Value: "9222"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SECURITY_USERNAME", Value: "component-user"})
				s.Require().Contains(env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SECURITY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "component-os-auth"},
						Key:                  "component-password",
					}},
				})
				for _, envVar := range env {
					s.Require().NotEqual("CAMUNDA_OPTIMIZE_OPENSEARCH_SSL_ENABLED", envVar.Name)
				}
			},
		},
		{
			Name: "Component Elasticsearch settings",
			Values: map[string]string{
				"identity.enabled":                                              "true",
				"optimize.enabled":                                              "true",
				"optimize.database.elasticsearch.enabled":                       "true",
				"optimize.database.elasticsearch.external":                      "true",
				"optimize.database.opensearch.enabled":                          "false",
				"optimize.database.elasticsearch.url.protocol":                  "https",
				"optimize.database.elasticsearch.url.host":                      "elasticsearch-host",
				"optimize.database.elasticsearch.url.port":                      "9211",
				"optimize.database.elasticsearch.auth.username":                 "elasticsearch-user",
				"optimize.database.elasticsearch.auth.secret.existingSecret":    "elasticsearch-auth",
				"optimize.database.elasticsearch.auth.secret.existingSecretKey": "password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_ELASTICSEARCH_HOST", Value: "elasticsearch-host"})
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_ELASTICSEARCH_HTTP_PORT", Value: "9211"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_USERNAME", Value: "elasticsearch-user"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SSL_ENABLED", Value: "true"})
				s.Require().Contains(env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "elasticsearch-auth"},
						Key:                  "password",
					}},
				})
			},
		},
		{
			Name: "Component Elasticsearch HTTP settings omit SSL",
			Values: map[string]string{
				"identity.enabled":                                              "true",
				"optimize.enabled":                                              "true",
				"optimize.database.elasticsearch.enabled":                       "true",
				"optimize.database.elasticsearch.external":                      "true",
				"optimize.database.opensearch.enabled":                          "false",
				"optimize.database.elasticsearch.url.protocol":                  "http",
				"optimize.database.elasticsearch.url.host":                      "component-es-host",
				"optimize.database.elasticsearch.url.port":                      "9222",
				"optimize.database.elasticsearch.auth.username":                 "component-user",
				"optimize.database.elasticsearch.auth.secret.existingSecret":    "component-es-auth",
				"optimize.database.elasticsearch.auth.secret.existingSecretKey": "component-password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_ELASTICSEARCH_HOST", Value: "component-es-host"})
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_ELASTICSEARCH_HTTP_PORT", Value: "9222"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_USERNAME", Value: "component-user"})
				s.Require().Contains(env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "component-es-auth"},
						Key:                  "component-password",
					}},
				})
				for _, envVar := range env {
					s.Require().NotEqual("CAMUNDA_OPTIMIZE_ELASTICSEARCH_SSL_ENABLED", envVar.Name)
				}
			},
		},
		// ---- Elasticsearch overrides ----
		{
			Name: "TestElasticsearchPortFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                         "true",
				"optimize.enabled":                         "true",
				"optimize.database.elasticsearch.enabled":  "true",
				"optimize.database.elasticsearch.external": "true",
				"optimize.database.elasticsearch.url.host": "elasticsearch-host",
				"optimize.database.elasticsearch.url.port": "9201",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_ELASTICSEARCH_HTTP_PORT", Value: "9201"})
			},
		},
		{
			Name: "TestElasticsearchSecretPasswordFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                                              "true",
				"optimize.enabled":                                              "true",
				"optimize.database.elasticsearch.enabled":                       "true",
				"optimize.database.elasticsearch.external":                      "true",
				"optimize.database.elasticsearch.url.host":                      "elasticsearch-host",
				"optimize.database.elasticsearch.auth.secret.existingSecret":    "my-es-secret",
				"optimize.database.elasticsearch.auth.secret.existingSecretKey": "es-password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-es-secret"},
							Key:                  "es-password",
						},
					},
				})
			},
		},
		{
			Name: "TestElasticsearchPortInMigrationInitContainer",
			Values: map[string]string{
				"identity.enabled":                         "true",
				"optimize.enabled":                         "true",
				"optimize.database.elasticsearch.enabled":  "true",
				"optimize.database.elasticsearch.external": "true",
				"optimize.database.elasticsearch.url.host": "elasticsearch-host",
				"optimize.database.elasticsearch.url.port": "9300",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				// The migration init container should also use the overridden port
				initContainers := deployment.Spec.Template.Spec.InitContainers
				s.Require().GreaterOrEqual(len(initContainers), 1)
				var migrationContainer corev1.Container
				for _, c := range initContainers {
					if c.Name == "migration" {
						migrationContainer = c
						break
					}
				}
				s.Require().Equal("migration", migrationContainer.Name)
				s.Require().Contains(migrationContainer.Env,
					corev1.EnvVar{Name: "OPTIMIZE_ELASTICSEARCH_HTTP_PORT", Value: "9300"})
			},
		},
		// ---- OpenSearch overrides ----
		{
			Name: "TestOpensearchPortFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                        "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "false",
				"optimize.database.opensearch.enabled":    "true",
				"optimize.database.opensearch.url.host":   "opensearch-host",
				"optimize.database.opensearch.url.port":   "9200",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_HTTP_PORT", Value: "9200"})
			},
		},
		{
			Name: "TestOpensearchUsernameFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                           "true",
				"optimize.enabled":                           "true",
				"optimize.database.elasticsearch.enabled":    "false",
				"optimize.database.opensearch.enabled":       "true",
				"optimize.database.opensearch.url.host":      "opensearch-host",
				"optimize.database.opensearch.auth.username": "optimizeuser",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SECURITY_USERNAME", Value: "optimizeuser"})
			},
		},
		{
			Name: "TestOpensearchDatabaseTypeSetWhenHostConfigured",
			Values: map[string]string{
				"identity.enabled":                        "true",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "false",
				"optimize.database.opensearch.enabled":    "true",
				"optimize.database.opensearch.url.host":   "opensearch-host",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_DATABASE", Value: "opensearch"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_HOST", Value: "opensearch-host"})
			},
		},
		{
			Name: "TestOpensearchSslEnabledWhenProtocolIsHttps",
			Values: map[string]string{
				"identity.enabled":                          "true",
				"optimize.enabled":                          "true",
				"optimize.database.elasticsearch.enabled":   "false",
				"optimize.database.opensearch.enabled":      "true",
				"optimize.database.opensearch.url.host":     "opensearch-host",
				"optimize.database.opensearch.url.protocol": "https",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SSL_ENABLED", Value: "true"})
			},
		},
		{
			Name: "TestOpensearchSslNotSetWhenProtocolIsHttp",
			Values: map[string]string{
				"identity.enabled":                          "true",
				"optimize.enabled":                          "true",
				"optimize.database.elasticsearch.enabled":   "false",
				"optimize.database.opensearch.enabled":      "true",
				"optimize.database.opensearch.url.host":     "opensearch-host",
				"optimize.database.opensearch.url.protocol": "http",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				for _, e := range env {
					s.Require().NotEqual("CAMUNDA_OPTIMIZE_OPENSEARCH_SSL_ENABLED", e.Name,
						"SSL should not be enabled when optimize overrides protocol to http")
				}
			},
		},
		{
			Name: "TestOpensearchAwsEnabledFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                         "true",
				"optimize.enabled":                         "true",
				"optimize.database.elasticsearch.enabled":  "false",
				"optimize.database.opensearch.enabled":     "true",
				"optimize.database.opensearch.url.host":    "opensearch-host",
				"optimize.database.opensearch.aws.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_AWS_ENABLED", Value: "true"})
			},
		},
		{
			Name: "TestOpensearchSecretPasswordFromOptimizeDatabase",
			Values: map[string]string{
				"identity.enabled":                                           "true",
				"optimize.enabled":                                           "true",
				"optimize.database.elasticsearch.enabled":                    "false",
				"optimize.database.opensearch.enabled":                       "true",
				"optimize.database.opensearch.url.host":                      "opensearch-host",
				"optimize.database.opensearch.auth.secret.existingSecret":    "optimize-os-auth-secret",
				"optimize.database.opensearch.auth.secret.existingSecretKey": "optimize-password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_OPENSEARCH_SECURITY_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "optimize-os-auth-secret"},
							Key:                  "optimize-password",
						},
					},
				})
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *DeploymentTemplateTest) TestOptimizeEnvHonorsExtraConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestEnvResolvedFromExtraConfiguration",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-extraconfig.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SPRING_PROFILES_ACTIVE", Value: "ccsm,gated"})
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_LOG_LEVEL", Value: "debug"})
				s.Require().Contains(env, corev1.EnvVar{Name: "UPGRADE_LOG_LEVEL", Value: "trace"})
				s.Require().Contains(env, corev1.EnvVar{Name: "ES_LOG_LEVEL", Value: "error"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_CACHES_CLOUD_TENANT_AUTHORIZATIONS_MAX_SIZE", Value: "22222"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_CACHES_CLOUD_TENANT_AUTHORIZATIONS_MIN_FETCH_INTERVAL_SECONDS", Value: "33333"})
			},
		},
		{
			Name:        "TestDeprecatedKeyUsedWithoutExtraConfiguration",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-deprecated.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SPRING_PROFILES_ACTIVE", Value: "ccsm,legacy"})
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_LOG_LEVEL", Value: "warn"})
			},
		},
		{
			Name:        "TestSpringImportFalseEntryDoesNotOverride",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-noimport.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_LOG_LEVEL", Value: "warn"})
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *DeploymentTemplateTest) TestOptimizeEnvGuardsNonScalarExtraConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestNonScalarAndNullNodesFallBackToDeprecatedDefault",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-nonscalar.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SPRING_PROFILES_ACTIVE", Value: "myprofile"},
					"a list node must not render as Go slice syntax; fall back to the deprecated default")
				s.Require().Contains(env, corev1.EnvVar{Name: "OPTIMIZE_LOG_LEVEL", Value: "mylevel"},
					"a null leaf must not render as an empty string; fall back to the deprecated default")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *DeploymentTemplateTest) TestContextPathIsAlsoExposedAsSpringProperty() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestServerServletContextPathRendersAlongsideOptimizeContextPath",
			Values: map[string]string{
				"optimize.enabled":     "true",
				"optimize.contextPath": "/optimize",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_CONTEXT_PATH", Value: "/optimize"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SERVLET_CONTEXT_PATH", Value: "/optimize"},
					"CSL reads server.servlet.context-path to keep the OIDC callback listener path context-relative")
			},
		},
		{
			Name: "TestNeitherIsRenderedWithoutAContextPath",
			Values: map[string]string{
				"optimize.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				for _, e := range deployment.Spec.Template.Spec.Containers[0].Env {
					s.Require().NotEqual("SERVER_SERVLET_CONTEXT_PATH", e.Name)
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *DeploymentTemplateTest) TestOptimizeMountsSpringImportWithoutAuth() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestApplicationCcsmMountedWhenExtraConfigurationSetAndAuthDisabled",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/optimize/testdata/values-optimize-gating-extraconfig.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				mounted := false
				for _, vm := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
					if vm.MountPath == "/optimize/config/application-ccsm.yaml" {
						mounted = true
					}
				}
				s.Require().True(mounted,
					"application-ccsm.yaml carries spring.config.import and must be mounted whenever extraConfiguration is set, even with auth disabled, otherwise the migration file is never imported")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

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

package orchestration

import (
	"camunda-platform/test/unit/testhelpers"
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type StatefulSetTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestStatefulSetTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &StatefulSetTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/statefulset.yaml"},
	})
}

func (s *StatefulSetTest) TestAutomountServiceAccountToken() {
	disabledValues := map[string]string{
		"orchestration.automountServiceAccountToken": "false",
	}
	enabledValues := map[string]string{
		"orchestration.automountServiceAccountToken": "true",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestPodOmitsAutomountServiceAccountTokenByDefault",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.Nil(t, statefulSet.Spec.Template.Spec.AutomountServiceAccountToken)
			},
		}, {
			Name:   "TestPodDisablesAutomountServiceAccountToken",
			Values: disabledValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.NotNil(t, statefulSet.Spec.Template.Spec.AutomountServiceAccountToken)
				require.False(t, *statefulSet.Spec.Template.Spec.AutomountServiceAccountToken)
			},
		}, {
			Name:   "TestPodEnablesAutomountServiceAccountToken",
			Values: enabledValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.NotNil(t, statefulSet.Spec.Template.Spec.AutomountServiceAccountToken)
				require.True(t, *statefulSet.Spec.Template.Spec.AutomountServiceAccountToken)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func TestGoldenStatefulSetWithRDBMSEnabled(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "statefulset-rdbms",
		Templates:      []string{"templates/orchestration/statefulset.yaml"},
		SetValues: map[string]string{
			"global.elasticsearch.enabled":                                       "false",
			"orchestration.exporters.rdbms.enabled":                              "true",
			"orchestration.data.secondaryStorage.rdbms.url":                      "jdbc:postgresql://rdbms:5432/camunda",
			"orchestration.data.secondaryStorage.rdbms.username":                 "camunda",
			"orchestration.data.secondaryStorage.rdbms.aws.enabled":              "true",
			"orchestration.data.secondaryStorage.rdbms.secret.existingSecret":    "camunda-rdbms-credentials",
			"orchestration.data.secondaryStorage.rdbms.secret.existingSecretKey": "password",
		},
		IgnoredLines: []string{
			`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
		},
	})
}

func (s *StatefulSetTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerSetPodLabels",
			Values: map[string]string{
				"orchestration.podLabels.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("bar", statefulSet.Spec.Template.Labels["foo"])
			},
		}, {
			Name: "TestContainerSetPodAnnotations",
			Values: map[string]string{
				"orchestration.podAnnotations.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("bar", statefulSet.Spec.Template.Annotations["foo"])
			},
		}, {
			Name:        "TestContainerSetPodLabelsAndAnnotationsWithTemplating",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-templated-labels.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then - verify templating is evaluated
				s.Require().Equal("camunda-platform-test", statefulSet.Spec.Template.Labels["release"])
				s.Require().Equal("camunda-platform-test", statefulSet.Spec.Template.Annotations["release"])
			},
		}, {
			Name: "TestContainerSetGlobalAnnotations",
			Values: map[string]string{
				"global.annotations.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("bar", statefulSet.ObjectMeta.Annotations["foo"])
			},
		}, {
			Name: "TestContainerSetPriorityClassName",
			Values: map[string]string{
				"orchestration.priorityClassName": "PRIO",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("PRIO", statefulSet.Spec.Template.Spec.PriorityClassName)
			},
		}, {
			Name: "TestContainerSetImageNameSubChart",
			Values: map[string]string{
				"global.image.registry":          "global.custom.registry.io",
				"orchestration.image.registry":   "subchart.custom.registry.io",
				"orchestration.image.repository": "camunda/camunda-test",
				"orchestration.image.tag":        "snapshot",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				container := statefulSet.Spec.Template.Spec.Containers[0]
				s.Require().Equal(container.Image, "subchart.custom.registry.io/camunda/camunda-test:snapshot")
			},
		}, {
			Name: "TestContainerSetImagePullSecretsGlobal",
			Values: map[string]string{
				"global.image.pullSecrets[0].name": "SecretName",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("SecretName", statefulSet.Spec.Template.Spec.ImagePullSecrets[0].Name)
			},
		}, {
			Name: "TestContainerSetImagePullSecretsSubChart",
			Values: map[string]string{
				"global.image.pullSecrets[0].name":        "SecretNameGlobal",
				"orchestration.image.pullSecrets[0].name": "SecretNameSubChart",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("SecretNameSubChart", statefulSet.Spec.Template.Spec.ImagePullSecrets[0].Name)
			},
		}, {
			Name: "TestContainerSetExtraInitContainers",
			Values: map[string]string{
				"orchestration.extraInitContainers[0].name":                      "init-container-{{ .Release.Name }}",
				"orchestration.extraInitContainers[0].image":                     "busybox:1.28",
				"orchestration.extraInitContainers[0].command[0]":                "sh",
				"orchestration.extraInitContainers[0].command[1]":                "-c",
				"orchestration.extraInitContainers[0].command[2]":                "top",
				"orchestration.extraInitContainers[0].volumeMounts[0].name":      "exporters",
				"orchestration.extraInitContainers[0].volumeMounts[0].mountPath": "/exporters/",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				initContainer := statefulSet.Spec.Template.Spec.InitContainers[0]
				s.Require().Equal("init-container-camunda-platform-test", initContainer.Name)
				s.Require().Equal("busybox:1.28", initContainer.Image)
				s.Require().Equal([]string{"sh", "-c", "top"}, initContainer.Command)
				s.Require().Equal("exporters", initContainer.VolumeMounts[0].Name)
				s.Require().Equal("/exporters/", initContainer.VolumeMounts[0].MountPath)
			},
		}, {
			Name: "TestInitContainers",
			Values: map[string]string{
				"orchestration.initContainers[0].name":                   "nginx",
				"orchestration.initContainers[0].image":                  "nginx:latest",
				"orchestration.initContainers[0].ports[0].containerPort": "80",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				initContainer := statefulSet.Spec.Template.Spec.InitContainers[0]
				s.Require().Equal("nginx", initContainer.Name)
				s.Require().Equal("nginx:latest", initContainer.Image)
			},
		}, {
			Name: "TestContainerOverwriteImageTag",
			Values: map[string]string{
				"orchestration.image.tag": "a.b.c",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				expectedContainerImage := "camunda/camunda:a.b.c"
				containers := statefulSet.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				s.Require().Equal(expectedContainerImage, containers[0].Image)
			},
		}, {
			Name: "TestContainerOverwriteImageTagWithChartDirectSetting",
			Values: map[string]string{
				"orchestration.image.tag": "a.b.c",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				expectedContainerImage := "camunda/camunda:a.b.c"
				containers := statefulSet.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				s.Require().Equal(expectedContainerImage, containers[0].Image)
			},
		}, {
			Name: "TestContainerShouldSetTemplateEnvVars",
			Values: map[string]string{
				"orchestration.env[0].name":  "RELEASE_NAME",
				"orchestration.env[0].value": "test-{{ .Release.Name }}",
				"orchestration.env[1].name":  "OTHER_ENV",
				"orchestration.env[1].value": "nothingToSeeHere",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "RELEASE_NAME", Value: "test-camunda-platform-test"})
				s.Require().Contains(env, corev1.EnvVar{Name: "OTHER_ENV", Value: "nothingToSeeHere"})
			},
		}, {
			Name: "TestContainerSetContainerCommand",
			Values: map[string]string{
				"orchestration.command[0]": "printenv",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				containers := statefulSet.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				s.Require().Equal(1, len(containers[0].Command))
				s.Require().Equal("printenv", containers[0].Command[0])
			},
		}, {
			Name: "TestContainerSetLog4j2",
			Values: map[string]string{
				"orchestration.log4j2": "<xml>\n</xml>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{
					SetValues: map[string]string{
						"orchestration.data.secondaryStorage.type": "elasticsearch",
					},
				}, s.chartPath, s.release, s.templates)
				helm.UnmarshalK8SYaml(s.T(), before, &statefulSetBefore)
				volumeMountLenBefore := len(statefulSetBefore.Spec.Template.Spec.Containers[0].VolumeMounts)
				// given
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				s.Require().Equal(volumeMountLenBefore+1, len(volumeMounts))
				s.Require().Equal("config", volumeMounts[4].Name)
				s.Require().Equal("/usr/local/camunda/config/log4j2.xml", volumeMounts[4].MountPath)
				s.Require().Equal("log4j2.xml", volumeMounts[4].SubPath)
			},
		}, {
			Name:                 "TestContainerSetExtraVolumes",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.extraVolumes[0].name":                  "extraVolume",
				"orchestration.extraVolumes[0].configMap.name":        "otherConfigMap",
				"orchestration.extraVolumes[0].configMap.defaultMode": "744",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{
					SetValues: map[string]string{
						"orchestration.data.secondaryStorage.type": "elasticsearch",
					},
				}, s.chartPath, s.release, s.templates)
				helm.UnmarshalK8SYaml(s.T(), before, &statefulSetBefore)
				volumeLenBefore := len(statefulSetBefore.Spec.Template.Spec.Volumes)
				// given
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				volumes := statefulSet.Spec.Template.Spec.Volumes
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
				"orchestration.extraVolumeMounts[0].name":      "otherConfigMap",
				"orchestration.extraVolumeMounts[0].mountPath": "/usr/local/config",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{
					SetValues: map[string]string{
						"orchestration.data.secondaryStorage.type": "elasticsearch",
					},
				}, s.chartPath, s.release, s.templates)
				helm.UnmarshalK8SYaml(s.T(), before, &statefulSetBefore)
				volumeMountLenBefore := len(statefulSetBefore.Spec.Template.Spec.Containers[0].VolumeMounts)
				// given
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				s.Require().Equal(volumeMountLenBefore+1, len(volumeMounts))
				extraVolumeMount := volumeMounts[volumeMountLenBefore]
				s.Require().Equal("otherConfigMap", extraVolumeMount.Name)
				s.Require().Equal("/usr/local/config", extraVolumeMount.MountPath)
			},
		}, {
			Name: "TestContainerSetExtraVolumesAndMounts",
			Values: map[string]string{
				"orchestration.extraVolumeMounts[0].name":             "otherConfigMap",
				"orchestration.extraVolumeMounts[0].mountPath":        "/usr/local/config",
				"orchestration.extraVolumes[0].name":                  "extraVolume",
				"orchestration.extraVolumes[0].configMap.name":        "otherConfigMap",
				"orchestration.extraVolumes[0].configMap.defaultMode": "744",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{
					SetValues: map[string]string{
						"orchestration.data.secondaryStorage.type": "elasticsearch",
					},
				}, s.chartPath, s.release, s.templates)
				helm.UnmarshalK8SYaml(s.T(), before, &statefulSetBefore)
				volumeMountLenBefore := len(statefulSetBefore.Spec.Template.Spec.Containers[0].VolumeMounts)
				volumeLenBefore := len(statefulSetBefore.Spec.Template.Spec.Volumes)

				// given
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				volumes := statefulSet.Spec.Template.Spec.Volumes
				s.Require().Equal(volumeLenBefore+1, len(volumes))

				extraVolume := volumes[volumeLenBefore]
				s.Require().Equal("extraVolume", extraVolume.Name)
				s.Require().NotNil(*extraVolume.ConfigMap)
				s.Require().Equal("otherConfigMap", extraVolume.ConfigMap.Name)
				s.Require().EqualValues(744, *extraVolume.ConfigMap.DefaultMode)

				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				s.Require().Equal(volumeMountLenBefore+1, len(volumeMounts))
				extraVolumeMount := volumeMounts[volumeMountLenBefore]
				s.Require().Equal("otherConfigMap", extraVolumeMount.Name)
				s.Require().Equal("/usr/local/config", extraVolumeMount.MountPath)
			},
		}, {
			Name: "TestPodSetSecurityContext",
			Values: map[string]string{
				"orchestration.podSecurityContext.runAsUser": "1000",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				securityContext := statefulSet.Spec.Template.Spec.SecurityContext
				s.Require().EqualValues(1000, *securityContext.RunAsUser)
			},
		}, {
			Name: "TestContainerSetSecurityContext",
			Values: map[string]string{
				"orchestration.containerSecurityContext.privileged": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				securityContext := statefulSet.Spec.Template.Spec.Containers[0].SecurityContext
				s.Require().True(*securityContext.Privileged)
			},
		}, {
			Name: "TestContainerSetServiceAccountName",
			Values: map[string]string{
				"orchestration.serviceAccount.name": "serviceaccount",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("serviceaccount", statefulSet.Spec.Template.Spec.ServiceAccountName)
			},
		}, {
			// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
			Name: "TestContainerSetNodeSelector",
			Values: map[string]string{
				"orchestration.nodeSelector.disktype": "ssd",
				"orchestration.nodeSelector.cputype":  "arm",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("ssd", statefulSet.Spec.Template.Spec.NodeSelector["disktype"])
				s.Require().Equal("arm", statefulSet.Spec.Template.Spec.NodeSelector["cputype"])
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
				"orchestration.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].key":       "kubernetes.io/e2e-az-name",
				"orchestration.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].operator":  "In",
				"orchestration.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[0]": "e2e-a1",
				"orchestration.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[1]": "e2e-a2",
				"orchestration.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight":                                         "1",
				"orchestration.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].key":             "another-node-label-key",
				"orchestration.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].operator":        "In",
				"orchestration.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].values[0]":       "another-node-label-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				nodeAffinity := statefulSet.Spec.Template.Spec.Affinity.NodeAffinity
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
			Name: "TestContainerSetTolerations",
			Values: map[string]string{
				"orchestration.tolerations[0].key":      "key1",
				"orchestration.tolerations[0].operator": "Equal",
				"orchestration.tolerations[0].value":    "Value1",
				"orchestration.tolerations[0].effect":   "NoSchedule",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				tolerations := statefulSet.Spec.Template.Spec.Tolerations
				s.Require().Equal(1, len(tolerations))

				toleration := tolerations[0]
				s.Require().Equal("key1", toleration.Key)
				s.Require().EqualValues("Equal", toleration.Operator)
				s.Require().Equal("Value1", toleration.Value)
				s.Require().EqualValues("NoSchedule", toleration.Effect)
			},
		}, {
			// https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
			//topologySpreadConstraints:
			//- maxSkew: 1
			//  topologyKey: "topology.kubernetes.io/zone"
			//  whenUnsatisfiable: "ScheduleAnyway"
			//  labelSelector:
			//    matchLabels:
			//      app.kubernetes.io/component: zeebe-broker
			Name: "TestContainerSetTopologySpreadConstraints",
			Values: map[string]string{
				"orchestration.topologySpreadConstraints[0].maxSkew":                                                   "1",
				"orchestration.topologySpreadConstraints[0].topologyKey":                                               "topology.kubernetes.io/zone",
				"orchestration.topologySpreadConstraints[0].whenUnsatisfiable":                                         "ScheduleAnyway",
				"orchestration.topologySpreadConstraints[0].labelSelector.matchLabels.app\\.kubernetes\\.io/component": "zeebe-broker",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				topologySpreadConstraints := statefulSet.Spec.Template.Spec.TopologySpreadConstraints
				s.Require().Equal(1, len(topologySpreadConstraints))

				topologySpreadConstraint := topologySpreadConstraints[0]
				s.Require().EqualValues(1, topologySpreadConstraint.MaxSkew)
				s.Require().Equal("topology.kubernetes.io/zone", topologySpreadConstraint.TopologyKey)
				s.Require().EqualValues("ScheduleAnyway", topologySpreadConstraint.WhenUnsatisfiable)
				s.Require().Equal("zeebe-broker", topologySpreadConstraint.LabelSelector.MatchLabels["app.kubernetes.io/component"])
			},
		}, {
			Name:                 "TestContainerSetPersistenceTypeRam",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.persistenceType": "memory",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{
					SetValues: map[string]string{
						"orchestration.data.secondaryStorage.type": "elasticsearch",
					},
				}, s.chartPath, s.release, s.templates)
				helm.UnmarshalK8SYaml(s.T(), before, &statefulSetBefore)
				volumeMountLenBefore := len(statefulSetBefore.Spec.Template.Spec.Containers[0].VolumeMounts)
				volumeLenBefore := len(statefulSetBefore.Spec.Template.Spec.Volumes)
				// given
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				s.Require().Equal(volumeMountLenBefore, len(volumeMounts))
				dataVolumeMount := volumeMounts[1]
				s.Require().Equal("data", dataVolumeMount.Name)
				s.Require().Equal("/usr/local/camunda/data", dataVolumeMount.MountPath)

				volumes := statefulSet.Spec.Template.Spec.Volumes
				s.Require().Equal(volumeLenBefore+1, len(volumes))
				dataVolume := volumes[0]
				s.Require().Equal("data", dataVolume.Name)
				s.Require().NotEmpty(dataVolume.EmptyDir)
				s.Require().EqualValues("Memory", dataVolume.EmptyDir.Medium)

				s.Require().Equal(0, len(statefulSet.Spec.VolumeClaimTemplates))
			},
		}, {
			Name:                 "TestContainerSetPersistenceTypeLocal",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.persistenceType": "local",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{
					SetValues: map[string]string{
						"orchestration.data.secondaryStorage.type": "elasticsearch",
					},
				}, s.chartPath, s.release, s.templates)
				helm.UnmarshalK8SYaml(s.T(), before, &statefulSetBefore)
				volumeMountLenBefore := len(statefulSetBefore.Spec.Template.Spec.Containers[0].VolumeMounts)
				volumeLenBefore := len(statefulSetBefore.Spec.Template.Spec.Volumes)
				// given
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				s.Require().Equal(volumeMountLenBefore-1, len(volumeMounts))
				for _, volumeMount := range volumeMounts {
					s.Require().NotEqual("data", volumeMount.Name)
				}

				volumes := statefulSet.Spec.Template.Spec.Volumes
				s.Require().Equal(volumeLenBefore, len(volumes))
				for _, volumeMount := range volumeMounts {
					s.Require().NotEqual("data", volumeMount.Name)
				}

				s.Require().Equal(0, len(statefulSet.Spec.VolumeClaimTemplates))
			},
		}, {
			Name: "TestContainerShouldOverwriteGlobalImagePullPolicy",
			Values: map[string]string{
				"global.image.pullPolicy": "Always",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				expectedPullPolicy := corev1.PullAlways
				containers := statefulSet.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))
				pullPolicy := containers[0].ImagePullPolicy
				s.Require().Equal(expectedPullPolicy, pullPolicy)
			},
		}, {
			Name: "TestContainerStartupProbe",
			Values: map[string]string{
				"orchestration.startupProbe.enabled":             "true",
				"orchestration.startupProbe.probePath":           "/healthz",
				"orchestration.startupProbe.initialDelaySeconds": "5",
				"orchestration.startupProbe.periodSeconds":       "10",
				"orchestration.startupProbe.successThreshold":    "1",
				"orchestration.startupProbe.failureThreshold":    "5",
				"orchestration.startupProbe.timeoutSeconds":      "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				probe := statefulSet.Spec.Template.Spec.Containers[0].StartupProbe

				s.Require().Equal("/healthz", probe.HTTPGet.Path)
				s.Require().EqualValues(5, probe.InitialDelaySeconds)
				s.Require().EqualValues(10, probe.PeriodSeconds)
				s.Require().EqualValues(1, probe.SuccessThreshold)
				s.Require().EqualValues(5, probe.FailureThreshold)
				s.Require().EqualValues(1, probe.TimeoutSeconds)
			},
		}, {
			// readinessProbe is enabled by default so it's tested by golden files.
			Name:                 "TestContainerLivenessProbe",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.livenessProbe.enabled":             "true",
				"orchestration.livenessProbe.probePath":           "/healthz",
				"orchestration.livenessProbe.initialDelaySeconds": "5",
				"orchestration.livenessProbe.periodSeconds":       "10",
				"orchestration.livenessProbe.successThreshold":    "1",
				"orchestration.livenessProbe.failureThreshold":    "5",
				"orchestration.livenessProbe.timeoutSeconds":      "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				probe := statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe

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
				"orchestration.contextPath":              "/test",
				"orchestration.startupProbe.enabled":     "true",
				"orchestration.startupProbe.probePath":   "/start",
				"orchestration.readinessProbe.enabled":   "true",
				"orchestration.readinessProbe.probePath": "/ready",
				"orchestration.livenessProbe.enabled":    "true",
				"orchestration.livenessProbe.probePath":  "/live",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				probe := statefulSet.Spec.Template.Spec.Containers[0]

				s.Require().Equal("/test/start", probe.StartupProbe.HTTPGet.Path)
				s.Require().Equal("/test/ready", probe.ReadinessProbe.HTTPGet.Path)
				s.Require().Equal("/test/live", probe.LivenessProbe.HTTPGet.Path)
			},
		}, {
			Name:                 "TestContainerProbesWithContextPathWithTrailingSlash",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"orchestration.contextPath":              "/test/",
				"orchestration.startupProbe.enabled":     "true",
				"orchestration.startupProbe.probePath":   "/start",
				"orchestration.readinessProbe.enabled":   "true",
				"orchestration.readinessProbe.probePath": "/ready",
				"orchestration.livenessProbe.enabled":    "true",
				"orchestration.livenessProbe.probePath":  "/live",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				probe := statefulSet.Spec.Template.Spec.Containers[0]

				s.Require().Equal("/test/start", probe.StartupProbe.HTTPGet.Path)
				s.Require().Equal("/test/ready", probe.ReadinessProbe.HTTPGet.Path)
				s.Require().Equal("/test/live", probe.LivenessProbe.HTTPGet.Path)
			},
		}, {
			Name: "TestContainerSetSidecar",
			Values: map[string]string{
				"orchestration.sidecars[0].name":                   "nginx",
				"orchestration.sidecars[0].image":                  "nginx:latest",
				"orchestration.sidecars[0].ports[0].containerPort": "80",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				podContainers := statefulSet.Spec.Template.Spec.Containers
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
			Name: "TestSetDnsPolicyAndDnsConfig",
			Values: map[string]string{
				"orchestration.dnsPolicy":                "ClusterFirst",
				"orchestration.dnsConfig.nameservers[0]": "8.8.8.8",
				"orchestration.dnsConfig.searches[0]":    "example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				// Check if dnsPolicy is set
				require.NotEmpty(s.T(), statefulSet.Spec.Template.Spec.DNSPolicy, "dnsPolicy should not be empty")

				// Check if dnsConfig is set
				require.NotNil(s.T(), statefulSet.Spec.Template.Spec.DNSConfig, "dnsConfig should not be nil")

				expectedDNSConfig := &corev1.PodDNSConfig{
					Nameservers: []string{"8.8.8.8"},
					Searches:    []string{"example.com"},
				}

				require.Equal(s.T(), expectedDNSConfig, statefulSet.Spec.Template.Spec.DNSConfig, "dnsConfig should match the expected configuration")
			},
		}, {
			Name: "TestHostNetworkEnabledDefaultsDnsPolicy",
			Values: map[string]string{
				"orchestration.hostNetwork": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				require.True(s.T(), statefulSet.Spec.Template.Spec.HostNetwork, "hostNetwork should be true")
				require.Equal(s.T(), corev1.DNSClusterFirstWithHostNet, statefulSet.Spec.Template.Spec.DNSPolicy,
					"dnsPolicy should default to ClusterFirstWithHostNet when hostNetwork is enabled")
			},
		}, {
			Name:   "TestHostNetworkDisabledByDefault",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				require.False(s.T(), statefulSet.Spec.Template.Spec.HostNetwork, "hostNetwork should be false by default")
			},
		}, {
			Name: "TestHostNetworkExplicitDnsPolicyWins",
			Values: map[string]string{
				"orchestration.hostNetwork": "true",
				"orchestration.dnsPolicy":   "ClusterFirst",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				require.True(s.T(), statefulSet.Spec.Template.Spec.HostNetwork, "hostNetwork should be true")
				require.Equal(s.T(), corev1.DNSClusterFirst, statefulSet.Spec.Template.Spec.DNSPolicy,
					"explicit dnsPolicy should override hostNetwork default")
			},
		}, {
			// Test hybrid auth: orchestration uses basic auth, so no OIDC secret needed
			Name: "TestHybridAuthOrchestrationBasicNoOidcSecret",
			Values: map[string]string{
				"identity.enabled":                             "true",
				"global.identity.auth.enabled":                 "true",
				"orchestration.security.authentication.method": "basic",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then - VALUES_ORCHESTRATION_CLIENT_SECRET should NOT be present
				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				for _, envvar := range env {
					s.Require().NotEqual("VALUES_ORCHESTRATION_CLIENT_SECRET", envvar.Name,
						"Orchestration should not have OIDC secret when using basic auth")
				}
			},
		}, {
			// Test that orchestration OIDC secret is only included when orchestration.authMethod=oidc
			Name: "TestOrchestrationOidcSecretOnlyWithOidcAuth",
			Values: map[string]string{
				"identity.enabled":                                                    "true",
				"global.identity.auth.enabled":                                        "true",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then - VALUES_ORCHESTRATION_CLIENT_SECRET should be present
				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(
					env,
					corev1.EnvVar{
						Name: "VALUES_ORCHESTRATION_CLIENT_SECRET",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "orchestration-oidc-secret"},
								Key:                  "client-secret",
							},
						},
					},
					"Orchestration should have OIDC secret when orchestration.authMethod=oidc")
			},
		}, {
			Name: "TestCamundaHubPingClientSecretDefaultsToOrchestrationOidcSecret",
			Values: map[string]string{
				"identity.enabled":                                                    "true",
				"camundaHub.enabled":                                                  "true",
				"webModeler.restapi.mail.fromAddress":                                 "noreply@example.com",
				"orchestration.hub.ping.endpoint":                                     "https://hub/api/v1/clusters",
				"orchestration.security.authentication.method":                        "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":    "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey": "client-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(
					env,
					corev1.EnvVar{
						Name: "VALUES_CAMUNDAHUB_PING_CLIENT_SECRET",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "orchestration-oidc-secret"},
								Key:                  "client-secret",
							},
						},
					},
					"Camunda Hub ping should use the orchestration OIDC secret by default")
			},
		},
		{
			Name: "TestCamundaHubPingClientSecretUsesCompleteCustomSecret",
			Values: map[string]string{
				"identity.enabled":                    "true",
				"camundaHub.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "noreply@example.com",
				"orchestration.hub.ping.endpoint":     "https://hub/api/v1/clusters",
				"orchestration.hub.ping.credentials.clientSecret.secret.existingSecret":    "my-secret",
				"orchestration.hub.ping.credentials.clientSecret.secret.existingSecretKey": "my-key",
				"orchestration.security.authentication.method":                             "oidc",
				"orchestration.security.authentication.oidc.secret.existingSecret":         "orchestration-oidc-secret",
				"orchestration.security.authentication.oidc.secret.existingSecretKey":      "client-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(
					env,
					corev1.EnvVar{
						Name: "VALUES_CAMUNDAHUB_PING_CLIENT_SECRET",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
								Key:                  "my-key",
							},
						},
					},
					"Camunda Hub ping should use the complete custom secret")
			},
		},
		{
			Name: "TestCamundaHubPingClientSecretUsesInlineCustomSecret",
			Values: map[string]string{
				"identity.enabled":                    "true",
				"camundaHub.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "noreply@example.com",
				"orchestration.hub.ping.endpoint":     "https://hub/api/v1/clusters",
				"orchestration.hub.ping.credentials.clientSecret.secret.inlineSecret": "some-secret",
				"orchestration.security.authentication.method":                        "oidc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(
					env,
					corev1.EnvVar{
						Name:  "VALUES_CAMUNDAHUB_PING_CLIENT_SECRET",
						Value: "some-secret",
					},
					"Camunda Hub ping should use the inline custom secret")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *StatefulSetTest) TestGlobalTlsOrchestrationFlagsInjectEnv() {
	testCases := []testhelpers.TestCase{
		{
			Name: "REST TLS only via global.tls.orchestration.rest.enabled",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"global.tls.orchestration.rest.enabled":               "true",
				"global.tls.orchestration.rest.secret.existingSecret": "rest-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_ENABLED", Value: "true"})
				s.Require().NotContains(env, corev1.EnvVar{Name: "CAMUNDA_API_GRPC_SSL_ENABLED", Value: "true"})
			},
		},
		{
			Name: "gRPC TLS only via global.tls.orchestration.grpc.enabled",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"global.tls.orchestration.grpc.enabled":               "true",
				"global.tls.orchestration.grpc.secret.existingSecret": "grpc-pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_API_GRPC_SSL_ENABLED", Value: "true"})
				s.Require().NotContains(env, corev1.EnvVar{Name: "SERVER_SSL_ENABLED", Value: "true"})
			},
		},
		{
			Name: "Both TLS modes via global.tls.orchestration.*",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"global.tls.orchestration.rest.enabled":               "true",
				"global.tls.orchestration.rest.secret.existingSecret": "rest-ks",
				"global.tls.orchestration.grpc.enabled":               "true",
				"global.tls.orchestration.grpc.secret.existingSecret": "grpc-pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_ENABLED", Value: "true"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_API_GRPC_SSL_ENABLED", Value: "true"})
			},
		},
		{
			Name: "Explicit orchestration.env wins via Kubernetes last-wins",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"global.tls.orchestration.rest.enabled":               "true",
				"global.tls.orchestration.rest.secret.existingSecret": "rest-ks",
				"orchestration.env[0].name":                           "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "orchestration.env[0].value=false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				var positions []int
				for i, e := range env {
					if e.Name == "SERVER_SSL_ENABLED" {
						positions = append(positions, i)
					}
				}
				s.Require().Len(positions, 2, "both entries should be rendered so the user-supplied one wins last")
				s.Require().Equal("true", env[positions[0]].Value)
				s.Require().Equal("false", env[positions[1]].Value)
			},
		},
		{
			Name: "REST secret block wires keystore env, mount and volume",
			Values: map[string]string{
				"orchestration.enabled":                                            "true",
				"global.tls.orchestration.rest.enabled":                            "true",
				"global.tls.orchestration.rest.secret.existingSecret":              "rest-keystore",
				"global.tls.orchestration.rest.secret.existingSecretPasswordKey":   "ks-pw",
				"global.tls.orchestration.rest.secret.keyAlias":                    "orchestration-rest",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_KEY_STORE", Value: "file:/usr/local/camunda/certificates/orchestration/rest/keystore.p12"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_KEY_STORE_TYPE", Value: "PKCS12"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_KEY_ALIAS", Value: "orchestration-rest"})
				s.Require().Contains(env, corev1.EnvVar{
					Name: "SERVER_SSL_KEY_STORE_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "rest-keystore"},
							Key:                  "ks-pw",
						},
					},
				})

				mounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				var found bool
				for _, m := range mounts {
					if m.Name == "orchestration-tls-rest" {
						found = true
						s.Require().Equal("/usr/local/camunda/certificates/orchestration/rest", m.MountPath)
						s.Require().True(m.ReadOnly)
					}
				}
				s.Require().True(found, "expected orchestration-tls-rest volumeMount")

				vols := statefulSet.Spec.Template.Spec.Volumes
				found = false
				for _, v := range vols {
					if v.Name == "orchestration-tls-rest" {
						found = true
						s.Require().Equal("rest-keystore", v.Secret.SecretName)
					}
				}
				s.Require().True(found, "expected orchestration-tls-rest volume")
			},
		},
		{
			Name: "gRPC secret block wires PEM env, mount and volume",
			Values: map[string]string{
				"orchestration.enabled":                                            "true",
				"global.tls.orchestration.grpc.enabled":                            "true",
				"global.tls.orchestration.grpc.secret.existingSecret":              "grpc-pem",
				"global.tls.orchestration.grpc.secret.existingSecretKey":           "server.crt",
				"global.tls.orchestration.grpc.secret.existingSecretPrivateKeyKey": "server.key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_API_GRPC_SSL_CERTIFICATE", Value: "/usr/local/camunda/certificates/orchestration/grpc/server.crt"})
				s.Require().Contains(env, corev1.EnvVar{Name: "CAMUNDA_API_GRPC_SSL_CERTIFICATEPRIVATEKEY", Value: "/usr/local/camunda/certificates/orchestration/grpc/server.key"})

				vols := statefulSet.Spec.Template.Spec.Volumes
				var found bool
				for _, v := range vols {
					if v.Name == "orchestration-tls-grpc" {
						found = true
						s.Require().Equal("grpc-pem", v.Secret.SecretName)
					}
				}
				s.Require().True(found, "expected orchestration-tls-grpc volume")
			},
		},
		{
			Name: "Secret block is inert when the matching enabled flag is false",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"global.tls.orchestration.rest.secret.existingSecret": "should-not-mount",
				"global.tls.orchestration.grpc.secret.existingSecret": "should-not-mount",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "orchestration-tls-rest")
				require.NotContains(t, output, "orchestration-tls-grpc")
				require.NotContains(t, output, "SERVER_SSL_KEY_STORE")
				require.NotContains(t, output, "CAMUNDA_API_GRPC_SSL_CERTIFICATE")
			},
		},
		{
			Name: "REST PEM mode emits Spring Boot certificate env vars",
			Values: map[string]string{
				"orchestration.enabled":                                            "true",
				"global.tls.orchestration.rest.enabled":                            "true",
				"global.tls.orchestration.rest.secret.existingSecret":              "rest-pem",
				"global.tls.orchestration.rest.secret.type":                        "pem",
				"global.tls.orchestration.rest.secret.existingSecretKey":           "tls.crt",
				"global.tls.orchestration.rest.secret.existingSecretPrivateKeyKey": "tls.key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				env := statefulSet.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE", Value: "/usr/local/camunda/certificates/orchestration/rest/tls.crt"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE_PRIVATE_KEY", Value: "/usr/local/camunda/certificates/orchestration/rest/tls.key"})
				require.NotContains(t, output, "SERVER_SSL_KEY_STORE")
			},
		},
		{
			Name: "Constraint fails when REST enabled but no cert is configured",
			Values: map[string]string{
				"orchestration.enabled":                 "true",
				"global.tls.orchestration.rest.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.orchestration.rest.enabled is true but no server cert is configured")
			},
		},
		{
			Name: "Constraint allows REST enabled when the operator hand-wires SERVER_SSL_KEY_STORE via orchestration.env",
			Values: map[string]string{
				"orchestration.enabled":                 "true",
				"global.tls.orchestration.rest.enabled": "true",
				"orchestration.env[0].name":             "SERVER_SSL_KEY_STORE",
				"orchestration.env[0].value":            "file:/custom/keystore.p12",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *StatefulSetTest) TestOpenSearchPasswordEnv() {
	const (
		legacyElasticsearchExporterClass = "io.camunda.zeebe.exporter.ElasticsearchExporter"
		legacyOpenSearchExporterClass    = "io.camunda.zeebe.exporter.opensearch.OpensearchExporter"
	)

	verifyOpenSearchPasswordEnv := func(expectedExporterClass, unexpectedExporterClass string, expected ...corev1.EnvVar) func(t *testing.T, output string, err error) {
		return func(t *testing.T, output string, err error) {
			require.NoError(t, err)

			var statefulSet appsv1.StatefulSet
			var configMap corev1.ConfigMap
			for _, manifest := range strings.Split(output, "---") {
				if strings.TrimSpace(manifest) == "" {
					continue
				}

				var renderedObject struct {
					Kind string `yaml:"kind"`
				}
				require.NoError(t, yaml.Unmarshal([]byte(manifest), &renderedObject))
				switch renderedObject.Kind {
				case "ConfigMap":
					helm.UnmarshalK8SYaml(t, manifest, &configMap)
				case "StatefulSet":
					helm.UnmarshalK8SYaml(t, manifest, &statefulSet)
				}
			}
			require.NotEmpty(t, statefulSet.Spec.Template.Spec.Containers)

			var actual []corev1.EnvVar
			for _, envVar := range statefulSet.Spec.Template.Spec.Containers[0].Env {
				if envVar.Name == "VALUES_OPENSEARCH_PASSWORD" {
					actual = append(actual, envVar)
				}
			}
			require.Equal(t, expected, actual)
			if expectedExporterClass != "" {
				require.Contains(t, configMap.Data["application.yaml"], expectedExporterClass)
			}
			if unexpectedExporterClass != "" {
				require.NotContains(t, configMap.Data["application.yaml"], unexpectedExporterClass)
			}
		}
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "Orchestration OpenSearch emits the configured password",
			Values: map[string]string{
				"optimize.enabled":                                                        "false",
				"optimize.database.elasticsearch.enabled":                                 "false",
				"optimize.database.opensearch.enabled":                                    "false",
				"orchestration.data.secondaryStorage.type":                                "opensearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "orchestration-password",
			},
			Verifier: verifyOpenSearchPasswordEnv("", "", corev1.EnvVar{Name: "VALUES_OPENSEARCH_PASSWORD", Value: "orchestration-password"}),
		},
		{
			Name: "Optimize OpenSearch legacy exporter retains component password",
			CaseTemplates: &testhelpers.CaseTemplate{Templates: []string{
				"templates/orchestration/configmap.yaml",
				"templates/orchestration/statefulset.yaml",
			}},
			Values: map[string]string{
				"optimize.enabled":                                                        "true",
				"optimize.database.elasticsearch.enabled":                                 "false",
				"optimize.database.opensearch.enabled":                                    "true",
				"orchestration.data.secondaryStorage.type":                                "elasticsearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "orchestration-password",
			},
			Verifier: verifyOpenSearchPasswordEnv(legacyOpenSearchExporterClass, legacyElasticsearchExporterClass, corev1.EnvVar{Name: "VALUES_OPENSEARCH_PASSWORD", Value: "orchestration-password"}),
		},
		{
			Name: "Optimize OpenSearch legacy exporter retains component secret reference",
			CaseTemplates: &testhelpers.CaseTemplate{Templates: []string{
				"templates/orchestration/configmap.yaml",
				"templates/orchestration/statefulset.yaml",
			}},
			Values: map[string]string{
				"optimize.enabled":                                                             "true",
				"optimize.database.elasticsearch.enabled":                                      "false",
				"optimize.database.opensearch.enabled":                                         "true",
				"orchestration.data.secondaryStorage.type":                                     "elasticsearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.existingSecret":    "orchestration-secret",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.existingSecretKey": "password",
			},
			Verifier: verifyOpenSearchPasswordEnv(legacyOpenSearchExporterClass, legacyElasticsearchExporterClass, corev1.EnvVar{
				Name: "VALUES_OPENSEARCH_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "orchestration-secret"},
					Key:                  "password",
				}},
			}),
		},
		{
			Name: "Elasticsearch exporter precedence omits configured OpenSearch password",
			CaseTemplates: &testhelpers.CaseTemplate{Templates: []string{
				"templates/orchestration/configmap.yaml",
				"templates/orchestration/statefulset.yaml",
			}},
			Values: map[string]string{
				"optimize.enabled":                                                        "true",
				"optimize.database.elasticsearch.enabled":                                 "true",
				"optimize.database.opensearch.enabled":                                    "true",
				"orchestration.data.secondaryStorage.type":                                "elasticsearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "component-password",
			},
			Verifier: verifyOpenSearchPasswordEnv(legacyElasticsearchExporterClass, legacyOpenSearchExporterClass),
		},
		{
			Name: "Orchestration Elasticsearch omits configured OpenSearch password",
			Values: map[string]string{
				"optimize.enabled":                                                        "false",
				"optimize.database.elasticsearch.enabled":                                 "false",
				"optimize.database.opensearch.enabled":                                    "false",
				"orchestration.data.secondaryStorage.type":                                "elasticsearch",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "component-password",
			},
			Verifier: verifyOpenSearchPasswordEnv("", ""),
		},
		{
			Name: "Disabled Optimize OpenSearch omits configured password",
			Values: map[string]string{
				"optimize.enabled":                                      "false",
				"optimize.database.elasticsearch.enabled":               "false",
				"optimize.database.opensearch.enabled":                  "true",
				"optimize.database.opensearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":              "elasticsearch",
			},
			Verifier: verifyOpenSearchPasswordEnv("", ""),
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *StatefulSetTest) TestLegacyExporterComponentPasswordEnv() {
	collectDatastorePasswordEnv := func(expected map[string]string) func(t *testing.T, output string, err error) {
		return func(t *testing.T, output string, err error) {
			require.NoError(t, err)

			var statefulSet appsv1.StatefulSet
			helm.UnmarshalK8SYaml(t, output, &statefulSet)
			require.NotEmpty(t, statefulSet.Spec.Template.Spec.Containers)

			actual := map[string]string{}
			for _, envVar := range statefulSet.Spec.Template.Spec.Containers[0].Env {
				switch envVar.Name {
				case "VALUES_OPENSEARCH_PASSWORD",
					"VALUES_ELASTICSEARCH_PASSWORD",
					"VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD",
					"VALUES_OPTIMIZE_DATABASE_ELASTICSEARCH_PASSWORD":
					actual[envVar.Name] = envVar.Value
				}
			}
			require.Equal(t, expected, actual)
		}
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "Distinct OpenSearch sources emit both passwords",
			Values: map[string]string{
				"optimize.enabled":                                                        "true",
				"optimize.database.elasticsearch.enabled":                                 "false",
				"optimize.database.opensearch.enabled":                                    "true",
				"optimize.database.opensearch.url.host":                                   "optimize-host",
				"optimize.database.opensearch.auth.secret.inlineSecret":                   "optimize-password",
				"orchestration.data.secondaryStorage.type":                                "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":                      "https://secondary-host:9443",
				"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "secondary-password",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{
				"VALUES_OPENSEARCH_PASSWORD":                   "secondary-password",
				"VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD": "optimize-password",
			}),
		},
		{
			Name: "Optimize OpenSearch credentials alone emit the component password",
			Values: map[string]string{
				"optimize.enabled":                                      "true",
				"optimize.database.elasticsearch.enabled":               "false",
				"optimize.database.opensearch.enabled":                  "true",
				"optimize.database.opensearch.url.host":                 "opensearch.example.com",
				"optimize.database.opensearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":              "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":    "https://opensearch.example.com:443",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{
				"VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD": "optimize-password",
			}),
		},
		{
			Name: "Elasticsearch exporter precedence omits the Optimize OpenSearch password",
			Values: map[string]string{
				"optimize.enabled":                                      "true",
				"optimize.database.elasticsearch.enabled":               "true",
				"optimize.database.opensearch.enabled":                  "true",
				"optimize.database.opensearch.url.host":                 "optimize-host",
				"optimize.database.opensearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":              "elasticsearch",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{}),
		},
		{
			Name: "Optimize source without any secret emits no password env",
			Values: map[string]string{
				"optimize.enabled":                                   "true",
				"optimize.database.elasticsearch.enabled":            "false",
				"optimize.database.opensearch.enabled":               "true",
				"optimize.database.opensearch.url.host":              "optimize-host",
				"optimize.database.opensearch.auth.username":         "optimize-user",
				"orchestration.data.secondaryStorage.type":           "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url": "https://secondary-host:9443",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{}),
		},
		{
			Name: "Elasticsearch authentication disabled omits the Optimize password",
			Values: map[string]string{
				"optimize.enabled":                                         "true",
				"optimize.database.opensearch.enabled":                     "false",
				"optimize.database.elasticsearch.enabled":                  "true",
				"optimize.database.elasticsearch.url.host":                 "optimize-host",
				"optimize.database.elasticsearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":                 "elasticsearch",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{}),
		},
		{
			Name: "AWS mode omits the Optimize OpenSearch password",
			Values: map[string]string{
				"optimize.enabled":                                      "true",
				"optimize.database.elasticsearch.enabled":               "false",
				"optimize.database.opensearch.enabled":                  "true",
				"optimize.database.opensearch.url.host":                 "optimize-host",
				"optimize.database.opensearch.aws.enabled":              "true",
				"optimize.database.opensearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":              "opensearch",
				"orchestration.data.secondaryStorage.opensearch.url":    "https://secondary-host:9443",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{}),
		},
		{
			Name: "Disabled Optimize omits the component OpenSearch password",
			Values: map[string]string{
				"optimize.enabled":                                      "false",
				"optimize.database.elasticsearch.enabled":               "false",
				"optimize.database.opensearch.enabled":                  "true",
				"optimize.database.opensearch.url.host":                 "optimize-host",
				"optimize.database.opensearch.auth.secret.inlineSecret": "optimize-password",
				"orchestration.data.secondaryStorage.type":              "elasticsearch",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{}),
		},
		{
			Name: "Distinct Elasticsearch sources emit both passwords",
			Values: map[string]string{
				"optimize.enabled":                                                           "true",
				"optimize.database.opensearch.enabled":                                       "false",
				"optimize.database.elasticsearch.enabled":                                    "true",
				"optimize.database.elasticsearch.external":                                   "true",
				"optimize.database.elasticsearch.url.host":                                   "optimize-host",
				"optimize.database.elasticsearch.auth.secret.inlineSecret":                   "optimize-password",
				"orchestration.data.secondaryStorage.type":                                   "elasticsearch",
				"orchestration.data.secondaryStorage.elasticsearch.url":                      "https://secondary-host:9443",
				"orchestration.data.secondaryStorage.elasticsearch.auth.secret.inlineSecret": "secondary-password",
			},
			Verifier: collectDatastorePasswordEnv(map[string]string{
				"VALUES_ELASTICSEARCH_PASSWORD":                   "secondary-password",
				"VALUES_OPTIMIZE_DATABASE_ELASTICSEARCH_PASSWORD": "optimize-password",
			}),
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *StatefulSetTest) TestLegacyExporterComponentTls() {
	optimizeOpenSearchSource := map[string]string{
		"optimize.enabled":                                   "true",
		"optimize.database.elasticsearch.enabled":            "false",
		"optimize.database.opensearch.enabled":               "true",
		"optimize.database.opensearch.url.host":              "optimize-host",
		"orchestration.data.secondaryStorage.type":           "opensearch",
		"orchestration.data.secondaryStorage.opensearch.url": "https://secondary-host:9443",
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "Optimize OpenSearch TLS reaches the Orchestration truststore",
			Values: mergeValues(optimizeOpenSearchSource, map[string]string{
				"optimize.database.opensearch.tls.secret.existingSecret":    "optimize-tls-secret",
				"optimize.database.opensearch.tls.secret.existingSecretKey": "optimize-ca.jks",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "-Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/optimize-ca.jks")
				require.Contains(t, output, "secretName: \"optimize-tls-secret\"")
			},
		},
		{
			Name: "Optimize OpenSearch source ignores secondary storage TLS config",
			Values: mergeValues(optimizeOpenSearchSource, map[string]string{
				"optimize.database.opensearch.tls.secret.existingSecret":                      "optimize-tls-secret",
				"optimize.database.opensearch.tls.secret.existingSecretKey":                   "optimize-ca.jks",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecret":    "secondary-tls-secret",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecretKey": "secondary-ca.jks",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "-Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/optimize-ca.jks")
				require.NotContains(t, output, "secondary-ca.jks")
				require.NotContains(t, output, "secretName: \"secondary-tls-secret\"")
			},
		},
		{
			Name: "Optimize source without TLS keeps the secondary storage truststore",
			Values: mergeValues(optimizeOpenSearchSource, map[string]string{
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecret":    "secondary-tls-secret",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecretKey": "secondary-ca.jks",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "-Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/secondary-ca.jks")
				require.Contains(t, output, "secretName: \"secondary-tls-secret\"")
			},
		},
		{
			Name: "Optimize Elasticsearch TLS reaches the Orchestration truststore",
			Values: map[string]string{
				"optimize.enabled":                                             "true",
				"optimize.database.opensearch.enabled":                         "false",
				"optimize.database.elasticsearch.enabled":                      "true",
				"optimize.database.elasticsearch.url.host":                     "optimize-host",
				"optimize.database.elasticsearch.tls.secret.existingSecret":    "optimize-tls-secret",
				"optimize.database.elasticsearch.tls.secret.existingSecretKey": "optimize-ca.jks",
				"orchestration.data.secondaryStorage.type":                     "elasticsearch",
				"orchestration.data.secondaryStorage.elasticsearch.url":        "https://secondary-host:9443",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "-Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/optimize-ca.jks")
				require.Contains(t, output, "secretName: \"optimize-tls-secret\"")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *StatefulSetTest) TestDocumentStoreEnvFromGatedByExtraConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestEnvFromSuppressedWhenCamundaDocumentInExtraConfiguration",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-documentstore-via-extraconfig.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				for _, ef := range statefulSet.Spec.Template.Spec.Containers[0].EnvFrom {
					if ef.ConfigMapRef != nil {
						s.Require().NotContains(ef.ConfigMapRef.Name, "documentstore-env-vars",
							"documentstore envFrom must be suppressed when camunda.document is set via extraConfiguration")
					}
				}
				// When the only envFrom entry is suppressed, the whole envFrom key must be
				// omitted rather than rendered as `envFrom:`/`envFrom: null` (invalid under
				// a strict schema). Unmarshalling can't tell null from omitted, so assert on
				// the raw manifest that the key is absent for this orchestration-only render.
				s.Require().NotContains(output, "envFrom:",
					"envFrom key must be omitted when it would be empty, not rendered as null")
			},
		},
		{
			Name: "TestEnvFromPresentWithoutCamundaDocument",
			Values: map[string]string{
				"orchestration.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "-documentstore-env-vars",
					"documentstore envFrom must remain when camunda.document is not set via extraConfiguration")
			},
		},
		{
			Name:        "TestAzureSecretSurvivesDocumentStoreEnvFromSuppression",
			ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata/values-azure-documentstore.yaml")},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				container := statefulSet.Spec.Template.Spec.Containers[0]
				require.Contains(t, container.Env, corev1.EnvVar{
					Name: "VALUES_DOCUMENT_STORE_AZURE_CONNECTION_STRING",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "azure-document-store"},
							Key:                  "connection-string",
						},
					},
				})
				for _, envFrom := range container.EnvFrom {
					if envFrom.ConfigMapRef != nil {
						require.NotContains(t, envFrom.ConfigMapRef.Name, "documentstore-env-vars")
					}
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

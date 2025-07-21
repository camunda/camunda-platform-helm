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

package core

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
		templates: []string{"templates/core/statefulset.yaml"},
	})
}

func (s *StatefulSetTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerSetPodLabels",
			Values: map[string]string{
				"core.podLabels.foo": "bar",
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
				"core.podAnnotations.foo": "bar",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// then
				s.Require().Equal("bar", statefulSet.Spec.Template.Annotations["foo"])
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
				"core.priorityClassName": "PRIO",
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
				"global.image.registry": "global.custom.registry.io",
				"global.image.tag":      "8.x.x",
				"core.image.registry":   "subchart.custom.registry.io",
				"core.image.repository": "camunda/camunda-test",
				"core.image.tag":        "snapshot",
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
				"global.image.pullSecrets[0].name": "SecretNameGlobal",
				"core.image.pullSecrets[0].name":   "SecretNameSubChart",
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
				"core.extraInitContainers[0].name":                      "init-container-{{ .Release.Name }}",
				"core.extraInitContainers[0].image":                     "busybox:1.28",
				"core.extraInitContainers[0].command[0]":                "sh",
				"core.extraInitContainers[0].command[1]":                "-c",
				"core.extraInitContainers[0].command[2]":                "top",
				"core.extraInitContainers[0].volumeMounts[0].name":      "exporters",
				"core.extraInitContainers[0].volumeMounts[0].mountPath": "/exporters/",
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
				"core.initContainers[0].name":                   "nginx",
				"core.initContainers[0].image":                  "nginx:latest",
				"core.initContainers[0].ports[0].containerPort": "80",
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
				"core.image.tag": "a.b.c",
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
			Name: "TestContainerOverwriteGlobalImageTag",
			Values: map[string]string{
				"global.image.tag": "a.b.c",
				"core.image.tag":   "",
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
				"global.image.tag": "x.y.z",
				"core.image.tag":   "a.b.c",
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
				"core.env[0].name":  "RELEASE_NAME",
				"core.env[0].value": "test-{{ .Release.Name }}",
				"core.env[1].name":  "OTHER_ENV",
				"core.env[1].value": "nothingToSeeHere",
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
				"core.command[0]": "printenv",
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
				"core.log4j2": "<xml>\n</xml>",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates)
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
				"core.extraVolumes[0].name":                  "extraVolume",
				"core.extraVolumes[0].configMap.name":        "otherConfigMap",
				"core.extraVolumes[0].configMap.defaultMode": "744",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates)
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
				"core.extraVolumeMounts[0].name":      "otherConfigMap",
				"core.extraVolumeMounts[0].mountPath": "/usr/local/config",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates)
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
				"core.extraVolumeMounts[0].name":             "otherConfigMap",
				"core.extraVolumeMounts[0].mountPath":        "/usr/local/config",
				"core.extraVolumes[0].name":                  "extraVolume",
				"core.extraVolumes[0].configMap.name":        "otherConfigMap",
				"core.extraVolumes[0].configMap.defaultMode": "744",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates)
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
				"core.podSecurityContext.runAsUser": "1000",
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
				"core.containerSecurityContext.privileged": "true",
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
				"core.serviceAccount.name": "serviceaccount",
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
				"core.nodeSelector.disktype": "ssd",
				"core.nodeSelector.cputype":  "arm",
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
				"core.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].key":       "kubernetes.io/e2e-az-name",
				"core.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].operator":  "In",
				"core.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[0]": "e2e-a1",
				"core.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[1]": "e2e-a2",
				"core.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight":                                         "1",
				"core.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].key":             "another-node-label-key",
				"core.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].operator":        "In",
				"core.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].values[0]":       "another-node-label-value",
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
				"core.tolerations[0].key":      "key1",
				"core.tolerations[0].operator": "Equal",
				"core.tolerations[0].value":    "Value1",
				"core.tolerations[0].effect":   "NoSchedule",
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
			Name:                 "TestContainerSetPersistenceTypeRam",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"core.persistenceType": "memory",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates)
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
				"core.persistenceType": "local",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// finding out the length of containers and volumeMounts array before addition of new volumeMount
				var statefulSetBefore appsv1.StatefulSet
				before := helm.RenderTemplate(s.T(), &helm.Options{}, s.chartPath, s.release, s.templates)
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
				"core.startupProbe.enabled":             "true",
				"core.startupProbe.probePath":           "/healthz",
				"core.startupProbe.initialDelaySeconds": "5",
				"core.startupProbe.periodSeconds":       "10",
				"core.startupProbe.successThreshold":    "1",
				"core.startupProbe.failureThreshold":    "5",
				"core.startupProbe.timeoutSeconds":      "1",
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
				"core.livenessProbe.enabled":             "true",
				"core.livenessProbe.probePath":           "/healthz",
				"core.livenessProbe.initialDelaySeconds": "5",
				"core.livenessProbe.periodSeconds":       "10",
				"core.livenessProbe.successThreshold":    "1",
				"core.livenessProbe.failureThreshold":    "5",
				"core.livenessProbe.timeoutSeconds":      "1",
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
				"core.contextPath":              "/test",
				"core.startupProbe.enabled":     "true",
				"core.startupProbe.probePath":   "/start",
				"core.readinessProbe.enabled":   "true",
				"core.readinessProbe.probePath": "/ready",
				"core.livenessProbe.enabled":    "true",
				"core.livenessProbe.probePath":  "/live",
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
				"core.sidecars[0].name":                   "nginx",
				"core.sidecars[0].image":                  "nginx:latest",
				"core.sidecars[0].ports[0].containerPort": "80",
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
				"core.dnsPolicy":                "ClusterFirst",
				"core.dnsConfig.nameservers[0]": "8.8.8.8",
				"core.dnsConfig.searches[0]":    "example.com",
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
		},
	}

	s.T().Skip("Skipping until 8.8 reenables these")
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

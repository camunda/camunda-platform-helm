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

package operate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
)

type deploymentTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestDeploymentTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &deploymentTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/operate/templates/deployment.yaml"},
	})
}

func (s *deploymentTemplateTest) TestContainerSetPodLabels() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.podLabels.foo": "bar",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	s.Require().Equal("bar", deployment.Spec.Template.Labels["foo"])
}

func (s *deploymentTemplateTest) TestContainerSetPodAnnotations() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.podAnnotations.foo": "bar",
			"operate.podAnnotations.foz": "baz",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	s.Require().Equal("bar", deployment.Spec.Template.Annotations["foo"])
	s.Require().Equal("baz", deployment.Spec.Template.Annotations["foz"])
}

func (s *deploymentTemplateTest) TestContainerSetGlobalAnnotations() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.annotations.foo": "bar",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	s.Require().Equal("bar", deployment.ObjectMeta.Annotations["foo"])
}

func (s *deploymentTemplateTest) TestContainerSetImagePullSecretsGlobal() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.pullSecrets[0].name": "SecretName",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	s.Require().Equal("SecretName", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
}

func (s *deploymentTemplateTest) TestContainerSetImagePullSecretsSubChart() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.pullSecrets[0].name":  "SecretName",
			"operate.image.pullSecrets[0].name": "SecretNameSubChart",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	s.Require().Equal("SecretNameSubChart", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
}

func (s *deploymentTemplateTest) TestContainerOverwriteImageTag() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/operate:a.b.c"
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *deploymentTemplateTest) TestContainerOverwriteGlobalImageTag() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/operate:a.b.c"
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *deploymentTemplateTest) TestContainerOverwriteImageTagWithChartDirectSetting() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.tag":  "x.y.z",
			"operate.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/operate:a.b.c"
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *deploymentTemplateTest) TestContainerSetContainerCommand() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.command": "[printenv]",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(1, len(containers[0].Command))
	s.Require().Equal("printenv", containers[0].Command[0])
}

func (s *deploymentTemplateTest) TestContainerSetExtraVolumes() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.extraVolumes[0].name":                  "extraVolume",
			"operate.extraVolumes[0].configMap.name":        "otherConfigMap",
			"operate.extraVolumes[0].configMap.defaultMode": "744",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	volumes := deployment.Spec.Template.Spec.Volumes
	s.Require().Equal(2, len(volumes))

	extraVolume := volumes[1]
	s.Require().Equal("extraVolume", extraVolume.Name)
	s.Require().NotNil(*extraVolume.ConfigMap)
	s.Require().Equal("otherConfigMap", extraVolume.ConfigMap.Name)
	s.Require().EqualValues(744, *extraVolume.ConfigMap.DefaultMode)
}

func (s *deploymentTemplateTest) TestContainerSetExtraVolumeMounts() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.extraVolumeMounts[0].name":      "otherConfigMap",
			"operate.extraVolumeMounts[0].mountPath": "/usr/local/config",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))

	volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
	s.Require().Equal(2, len(volumeMounts))
	extraVolumeMount := volumeMounts[1]
	s.Require().Equal("otherConfigMap", extraVolumeMount.Name)
	s.Require().Equal("/usr/local/config", extraVolumeMount.MountPath)
}

func (s *deploymentTemplateTest) TestContainerSetExtraVolumesAndMounts() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.extraVolumeMounts[0].name":             "otherConfigMap",
			"operate.extraVolumeMounts[0].mountPath":        "/usr/local/config",
			"operate.extraVolumes[0].name":                  "extraVolume",
			"operate.extraVolumes[0].configMap.name":        "otherConfigMap",
			"operate.extraVolumes[0].configMap.defaultMode": "744",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	volumes := deployment.Spec.Template.Spec.Volumes
	s.Require().Equal(2, len(volumes))

	extraVolume := volumes[1]
	s.Require().Equal("extraVolume", extraVolume.Name)
	s.Require().NotNil(*extraVolume.ConfigMap)
	s.Require().Equal("otherConfigMap", extraVolume.ConfigMap.Name)
	s.Require().EqualValues(744, *extraVolume.ConfigMap.DefaultMode)

	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))

	volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
	s.Require().Equal(2, len(volumeMounts))
	extraVolumeMount := volumeMounts[1]
	s.Require().Equal("otherConfigMap", extraVolumeMount.Name)
	s.Require().Equal("/usr/local/config", extraVolumeMount.MountPath)
}

func (s *deploymentTemplateTest) TestContainerSetServiceAccountName() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.serviceAccount.name": "accName",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	serviceAccName := deployment.Spec.Template.Spec.ServiceAccountName
	s.Require().Equal("accName", serviceAccName)
}

func (s *deploymentTemplateTest) TestPodSetSecurityContext() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.podSecurityContext.runAsUser": "1000",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	securityContext := deployment.Spec.Template.Spec.SecurityContext
	s.Require().EqualValues(1000, *securityContext.RunAsUser)
}

func (s *deploymentTemplateTest) TestContainerSetSecurityContext() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.containerSecurityContext.privileged":          "true",
			"operate.containerSecurityContext.capabilities.add[0]": "NET_ADMIN",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	securityContext := deployment.Spec.Template.Spec.Containers[0].SecurityContext
	s.Require().True(*securityContext.Privileged)
	s.Require().EqualValues("NET_ADMIN", securityContext.Capabilities.Add[0])
}

// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
func (s *deploymentTemplateTest) TestContainerSetNodeSelector() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.nodeSelector.disktype": "ssd",
			"operate.nodeSelector.cputype":  "arm",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	s.Require().Equal("ssd", deployment.Spec.Template.Spec.NodeSelector["disktype"])
	s.Require().Equal("arm", deployment.Spec.Template.Spec.NodeSelector["cputype"])
}

// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity
func (s *deploymentTemplateTest) TestContainerSetAffinity() {
	// given

	//affinity:
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

	options := &helm.Options{
		SetValues: map[string]string{
			"operate.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].key":       "kubernetes.io/e2e-az-name",
			"operate.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].operator":  "In",
			"operate.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[0]": "e2e-a1",
			"operate.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchexpressions[0].values[1]": "e2e-a2",
			"operate.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight":                                         "1",
			"operate.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].key":             "another-node-label-key",
			"operate.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].operator":        "In",
			"operate.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].values[0]":       "another-node-label-value",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
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
}

// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration
func (s *deploymentTemplateTest) TestContainerSetTolerations() {
	// given

	//tolerations:
	//- key: "key1"
	//  operator: "Equal"
	//  value: "value1"
	//  effect: "NoSchedule"

	options := &helm.Options{
		SetValues: map[string]string{
			"operate.tolerations[0].key":      "key1",
			"operate.tolerations[0].operator": "Equal",
			"operate.tolerations[0].value":    "Value1",
			"operate.tolerations[0].effect":   "NoSchedule",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
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
}

func (s *deploymentTemplateTest) TestContainerShouldDisableOperateIntegration() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.auth.enabled": "false",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env

	for _, envvar := range env {
		s.Require().NotEqual("CAMUNDA_OPERATE_IDENTITY_ISSUER_URL", envvar.Name)
		s.Require().NotEqual("CAMUNDA_OPERATE_IDENTITY_ISSUER_BACKEND_URL", envvar.Name)
		s.Require().NotEqual("CAMUNDA_OPERATE_IDENTITY_CLIENT_ID", envvar.Name)
		s.Require().NotEqual("CAMUNDA_OPERATE_IDENTITY_CLIENT_SECRET", envvar.Name)
	}

	s.Require().Contains(env, v12.EnvVar{Name: "SPRING_PROFILES_ACTIVE", Value: "auth"})
}

func (s *deploymentTemplateTest) TestContainerShouldSetOperateIdentitySecretValue() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.auth.operate.existingSecret": "secretValue",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env,
		v12.EnvVar{
			Name: "CAMUNDA_OPERATE_IDENTITY_CLIENT_SECRET",
			ValueFrom: &v12.EnvVarSource{
				SecretKeyRef: &v12.SecretKeySelector{
					LocalObjectReference: v12.LocalObjectReference{Name: "camunda-platform-test-operate-identity-secret"},
					Key:                  "operate-secret",
				},
			},
		})
}

func (s *deploymentTemplateTest) TestContainerShouldSetOperateIdentitySecretViaReference() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.auth.operate.existingSecret.name": "ownExistingSecret",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env,
		v12.EnvVar{
			Name: "CAMUNDA_OPERATE_IDENTITY_CLIENT_SECRET",
			ValueFrom: &v12.EnvVarSource{
				SecretKeyRef: &v12.SecretKeySelector{
					LocalObjectReference: v12.LocalObjectReference{Name: "ownExistingSecret"},
					Key:                  "operate-secret",
				},
			},
		})
}

func (s *deploymentTemplateTest) TestContainerShouldSetTheRightKeycloakServiceUrl() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.keycloak.fullname": "keycloak",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env,
		v12.EnvVar{
			Name:  "CAMUNDA_OPERATE_IDENTITY_ISSUER_BACKEND_URL",
			Value: "http://keycloak:80/auth/realms/camunda-platform",
		})
}

func (s *deploymentTemplateTest) TestContainerShouldOverwriteGlobalImagePullPolicy() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.pullPolicy": "Always",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedPullPolicy := v12.PullAlways
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	pullPolicy := containers[0].ImagePullPolicy
	s.Require().Equal(expectedPullPolicy, pullPolicy)
}

func (s *deploymentTemplateTest) TestContainerShouldAddContextPath() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"operate.contextPath": "/operate",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	env := deployment.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env,
		v12.EnvVar{
			Name:  "SERVER_SERVLET_CONTEXT_PATH",
			Value: "/operate",
		},
	)
}

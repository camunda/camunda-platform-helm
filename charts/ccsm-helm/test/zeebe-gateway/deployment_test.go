package gateway

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
)

type deploymentTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestGatewayDeploymentTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &deploymentTemplateTest{
		chartPath: chartPath,
		release:   "ccsm-helm-test",
		namespace: "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/zeebe-gateway/templates/gateway-deployment.yaml"},
	})
}

func (s *deploymentTemplateTest) TestContainerSetPodLabels() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe-gateway.podLabels.foo": "bar",
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

func (s *deploymentTemplateTest) TestContainerSetPriorityClassName() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe-gateway.priorityClassName": "PRIO",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	s.Require().Equal("PRIO", deployment.Spec.Template.Spec.PriorityClassName)
}

func (s *deploymentTemplateTest) TestContainerSetImagePullSecrets() {
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

func (s *deploymentTemplateTest) TestContainerOverwriteImageTag() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe-gateway.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/zeebe:a.b.c"
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
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/zeebe:a.b.c"
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *deploymentTemplateTest) TestContainerOverwriteImageTagWithChartDirectSetting() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.tag":        "x.y.z",
			"zeebe-gateway.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/zeebe:a.b.c"
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *deploymentTemplateTest) TestContainerSetContainerCommand() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe-gateway.command": "[printenv]",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(len(containers), 1)
	s.Require().Equal(1, len(containers[0].Command))
	s.Require().Equal("printenv", containers[0].Command[0])
}

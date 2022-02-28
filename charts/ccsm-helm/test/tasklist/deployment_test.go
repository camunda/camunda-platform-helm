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

package tasklist

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

func TestDeploymentTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &deploymentTemplateTest{
		chartPath: chartPath,
		release:   "ccsm-helm-test",
		namespace: "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/tasklist/templates/deployment.yaml"},
	})
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

func (s *deploymentTemplateTest) TestContainerOverwriteImageTag() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"tasklist.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/tasklist:a.b.c"
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
	expectedContainerImage := "camunda/tasklist:a.b.c"
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *deploymentTemplateTest) TestContainerOverwriteImageTagWithChartDirectSetting() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.tag":   "x.y.z",
			"tasklist.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	// then
	expectedContainerImage := "camunda/tasklist:a.b.c"
	containers := deployment.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *deploymentTemplateTest) TestContainerSetSecurityContext() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"tasklist.podSecurityContext.runAsUser": "1000",
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

func (s *deploymentTemplateTest) TestContainerSetContainerCommand() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"tasklist.command": "[printenv]",
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

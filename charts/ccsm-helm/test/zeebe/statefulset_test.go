package zeebe

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
)

type statefulSetTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestStatefulSetTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &statefulSetTest{
		chartPath: chartPath,
		release:   "ccsm-helm-test",
		namespace: "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/zeebe/templates/statefulset.yaml"},
	})
}

func (s *statefulSetTest) TestContainerSetPodLabels() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe.podLabels.foo": "bar",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	s.Require().Equal("bar", statefulSet.Spec.Template.Labels["foo"])
}

func (s *statefulSetTest) TestContainerSetPriorityClassName() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe.priorityClassName": "PRIO",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	s.Require().Equal("PRIO", statefulSet.Spec.Template.Spec.PriorityClassName)
}

func (s *statefulSetTest) TestContainerSetImagePullSecrets() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.pullSecrets[0].name": "SecretName",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	s.Require().Equal("SecretName", statefulSet.Spec.Template.Spec.ImagePullSecrets[0].Name)
}

func (s *statefulSetTest) TestContainerSetExtraInitContainers() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe.extraInitContainers[0].name":                      "init-container-{{ .Release.Name }}",
			"zeebe.extraInitContainers[0].image":                     "busybox:1.28",
			"zeebe.extraInitContainers[0].command[0]":                "sh",
			"zeebe.extraInitContainers[0].command[1]":                "-c",
			"zeebe.extraInitContainers[0].command[2]":                "top",
			"zeebe.extraInitContainers[0].volumeMounts[0].name":      "exporters",
			"zeebe.extraInitContainers[0].volumeMounts[0].mountPath": "/exporters/",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	initContainer := statefulSet.Spec.Template.Spec.InitContainers[0]
	s.Require().Equal("init-container-ccsm-helm-test", initContainer.Name)
	s.Require().Equal("busybox:1.28", initContainer.Image)
	s.Require().Equal([]string{"sh", "-c", "top"}, initContainer.Command)
	s.Require().Equal("exporters", initContainer.VolumeMounts[0].Name)
	s.Require().Equal("/exporters/", initContainer.VolumeMounts[0].MountPath)
}

func (s *statefulSetTest) TestContainerOverwriteImageTag() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	expectedContainerImage := "camunda/zeebe:a.b.c"
	containers := statefulSet.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *statefulSetTest) TestContainerOverwriteGlobalImageTag() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.tag": "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	expectedContainerImage := "camunda/zeebe:a.b.c"
	containers := statefulSet.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *statefulSetTest) TestContainerOverwriteImageTagWithChartDirectSetting() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.image.tag": "x.y.z",
			"zeebe.image.tag":  "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	expectedContainerImage := "camunda/zeebe:a.b.c"
	containers := statefulSet.Spec.Template.Spec.Containers
	s.Require().Equal(1, len(containers))
	s.Require().Equal(expectedContainerImage, containers[0].Image)
}

func (s *statefulSetTest) TestContainerShouldContainExporterClassPerDefault() {
	// given
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	env := statefulSet.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env, v12.EnvVar{Name: "ZEEBE_BROKER_EXPORTERS_ELASTICSEARCH_CLASSNAME", Value: "io.camunda.zeebe.exporter.ElasticsearchExporter"})
}

func (s *statefulSetTest) TestContainerDisableExporter() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.elasticsearch.disableExporter": "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	env := statefulSet.Spec.Template.Spec.Containers[0].Env
	s.Require().NotContains(env, v12.EnvVar{Name: "ZEEBE_BROKER_EXPORTERS_ELASTICSEARCH_CLASSNAME", Value: "io.camunda.zeebe.exporter.ElasticsearchExporter"})
}



func (s *statefulSetTest) TestContainerShouldSetTemplateEnvVars() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe.env[0].name": "RELEASE_NAME",
			"zeebe.env[0].value": "test-{{ .Release.Name }}",
			"zeebe.env[1].name": "OTHER_ENV",
			"zeebe.env[1].value": "nothingToSeeHere",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	env := statefulSet.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(env, v12.EnvVar{Name: "RELEASE_NAME", Value: "test-ccsm-helm-test"})
	s.Require().Contains(env, v12.EnvVar{Name: "OTHER_ENV", Value: "nothingToSeeHere"})
}

func (s *statefulSetTest) TestContainerSetSecurityContext() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe.containerSecurityContext.privileged":          "true",
			"zeebe.containerSecurityContext.capabilities.add[0]": "NET_ADMIN",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var statefulSet v1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	// then
	securityContext := statefulSet.Spec.Template.Spec.Containers[0].SecurityContext
	s.Require().True(*securityContext.Privileged)
	s.Require().EqualValues("NET_ADMIN", securityContext.Capabilities.Add[0])
}

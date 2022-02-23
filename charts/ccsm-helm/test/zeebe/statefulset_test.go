package zeebe

import (
	"camunda-cloud-helm/charts/ccsm-helm/test/golden"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/apps/v1"
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
			"zeebe.extraInitContainers[0].name": "init-container-{{ .Release.Name }}",
			"zeebe.extraInitContainers[0].image": "busybox:1.28",
			"zeebe.extraInitContainers[0].command[0]": "sh",
			"zeebe.extraInitContainers[0].command[1]": "-c",
			"zeebe.extraInitContainers[0].command[2]": "top",
			"zeebe.extraInitContainers[0].volumeMounts[0].name": "exporters",
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

func TestGoldenContainerSecurityContext(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &golden.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "ccsm-helm-test",
		Namespace:      "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "statefulset-containersecuritycontext",
		Templates:      []string{"charts/zeebe/templates/statefulset.yaml"},
		SetValues: map[string]string{
			"zeebe.containerSecurityContext.privileged":          "true",
			"zeebe.containerSecurityContext.capabilities.add[0]": "NET_ADMIN",
		},
	})
}

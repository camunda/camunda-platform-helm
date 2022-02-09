package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
)

type statefulSetTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestStatefulSetTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../")
	require.NoError(t, err)

	suite.Run(t, &statefulSetTemplateTest{
		chartPath: chartPath,
		release:   "ccsm-helm-test",
		namespace: "zeebe-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/zeebe/templates/statefulset.yaml"},
	})
}

func (s *statefulSetTemplateTest) TestContainerSpecImage() {
	options := &helm.Options{
		SetValues: map[string]string{
			"image.repository": "helm/zeebe",
			"image.tag":        "a.b.c",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs: map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)

	var statefulSet appsv1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	expectedContainerImage := "helm/zeebe:a.b.c"
	containers := statefulSet.Spec.Template.Spec.Containers
	s.Require().Equal(len(containers), 1)
	s.Require().Equal(containers[0].Image, expectedContainerImage)
}


func (s *statefulSetTemplateTest) TestContainerDefaults() {
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs: map[string][]string{"template": {"--debug"}, "install": {"--debug"}},
	}
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)

	var statefulSet appsv1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

	expectedEnv := v1.EnvVar{Name: "ZEEBE_BROKER_CLUSTER_PARTITIONSCOUNT", Value: "3"}
	envs := statefulSet.Spec.Template.Spec.Containers[0].Env
	s.Require().Contains(envs, expectedEnv)
}
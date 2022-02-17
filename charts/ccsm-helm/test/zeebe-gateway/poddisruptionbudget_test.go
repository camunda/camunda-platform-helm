package gateway

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
	"k8s.io/api/policy/v1beta1"
)

func TestGoldenPodDisruptionBudgetDefaults(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &golden.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "ccsm-helm-test",
		Namespace:      "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "poddisruptionbudget",
		Templates:      []string{"charts/zeebe-gateway/templates/gateway-poddisruptionbudget.yaml"},
		SetValues:      map[string]string{"zeebe-gateway.podDisruptionBudget.enabled": "true"},
	})
}

type podDisruptionBudgetTest struct {
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

	suite.Run(t, &podDisruptionBudgetTest{
		chartPath: chartPath,
		release:   "ccsm-helm-test",
		namespace: "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/zeebe-gateway/templates/gateway-poddisruptionbudget.yaml"},
	})
}

func (s *podDisruptionBudgetTest) TestContainerMinAvailableMutualExclusiveWithMaxUnavailable() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebe-gateway.podDisruptionBudget.enabled":      "true",
			"zeebe-gateway.podDisruptionBudget.minAvailable": "1",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var podDisruptionBudget v1beta1.PodDisruptionBudget
	helm.UnmarshalK8SYaml(s.T(), output, &podDisruptionBudget)

	// then
	s.Require().EqualValues(1, podDisruptionBudget.Spec.MinAvailable.IntVal)
	s.Require().Nil(podDisruptionBudget.Spec.MaxUnavailable)
}

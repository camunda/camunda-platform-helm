package zeebe

import (
	"path/filepath"
	"strings"
	"testing"

	"camunda-cloud-helm/charts/ccsm-helm/test/golden"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenDefaultsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)
	templateNames := []string{"service", "serviceaccount", "statefulset", "configmap"}

	for _, name := range templateNames {
		suite.Run(t, &golden.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "ccsm-helm-test",
			Namespace:      "ccsm-helm-" + strings.ToLower(random.UniqueId()),
			GoldenFileName: name,
			Templates:      []string{"charts/zeebe/templates/" + name + ".yaml"},
		})
	}
}

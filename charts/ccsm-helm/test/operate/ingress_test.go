package operate

import (
	"path/filepath"
	"strings"
	"testing"

	"camunda-cloud-helm/charts/ccsm-helm/test/golden"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenIngressDefaultTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &golden.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "ccsm-helm-test",
		Namespace:      "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "ingress",
		Templates:      []string{"charts/operate/templates/ingress.yaml"},
		SetValues:		map[string]string{"operate.ingress.enabled" : "true"},
	})
}

func TestGoldenIngressAllEnabledTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &golden.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "ccsm-helm-test",
		Namespace:      "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "ingress-all-enabled",
		Templates:      []string{"charts/operate/templates/ingress.yaml"},
		SetValues:		map[string]string{
			"operate.ingress.enabled" : "true",
			"operate.ingress.host" : "local",
			"operate.ingress.tls.enabled" : "true",
			"operate.ingress.tls.secretName" : "my-secret",
		},
	})
}

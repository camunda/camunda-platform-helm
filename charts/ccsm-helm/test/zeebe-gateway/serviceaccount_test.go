package gateway

import (
	"camunda-cloud-helm/charts/ccsm-helm/test/golden"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenServiceaccountWithAnnotations(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &golden.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "ccsm-helm-test",
		Namespace:      "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "serviceaccount-annotations",
		Templates:      []string{"charts/zeebe-gateway/templates/gateway-serviceaccount.yaml"},
		SetValues: map[string]string{
			"zeebe-gateway.serviceAccount.annotations.foo":  "bar",
			"zeebe-gateway.serviceAccount.annotations.lulz": "baz",
		},
	})
}

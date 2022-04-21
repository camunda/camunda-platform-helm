package test

import (
	"camunda-platform-helm/charts/camunda-platform/test/golden"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenServiceMonitorDefaults(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../")
	require.NoError(t, err)

	suite.Run(t, &golden.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "service-monitor",
		Templates:      []string{"templates/service-monitor.yaml"},
		SetValues:      map[string]string{"prometheusServiceMonitor.enabled": "true"},
	})
}

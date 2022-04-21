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

func TestGoldenCuratorDefaults(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../")
	require.NoError(t, err)
	templateNames := []string{"curator-configmap", "curator-cronjob"}

	for _, name := range templateNames {
		suite.Run(t, &golden.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
			GoldenFileName: name,
			Templates:      []string{"templates/" + name + ".yaml"},
			SetValues:      map[string]string{"retentionPolicy.enabled": "true"},
		})
	}
}

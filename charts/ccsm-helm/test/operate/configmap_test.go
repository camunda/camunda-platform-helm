package operate

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type configmapTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigmapTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	suite.Run(t, &configmapTemplateTest{
		chartPath: chartPath,
		release:   "ccsm-helm-test",
		namespace: "ccsm-helm-" + strings.ToLower(random.UniqueId()),
		templates: []string{"charts/operate/templates/configmap.yaml"},
	})
}

func (s *configmapTemplateTest) TestContainerGoldenTestConfigMapDefaults() {
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	actual := []byte(output)

	goldenFile := "configmap.golden.yaml"

	if *update {
		err := ioutil.WriteFile(goldenFile, actual, 0644)
		s.Require().NoError(err, "Golden file was not writable")
	}

	expected, err := ioutil.ReadFile(goldenFile)

	// then
	s.Require().NoError(err, "Golden file doesn't exist or was not readable")
	s.Require().Equal(string(expected), output)
}

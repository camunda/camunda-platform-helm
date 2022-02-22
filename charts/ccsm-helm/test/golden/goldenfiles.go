package golden

import (
	"flag"
	"io/ioutil"

	"regexp"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/suite"
)

var update = flag.Bool("update-golden", false, "update golden test output files")

type TemplateGoldenTest struct {
	suite.Suite
	ChartPath      string
	Release        string
	Namespace      string
	GoldenFileName string
	Templates      []string
	SetValues      map[string]string
}

func (s *TemplateGoldenTest) TestContainerGoldenTestDefaults() {
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", s.Namespace),
		SetValues:      s.SetValues,
	}
	output := helm.RenderTemplate(s.T(), options, s.ChartPath, s.Release, s.Templates)
	regex := regexp.MustCompile(".+(helm.sh/chart: ).*\n")
	bytes := regex.ReplaceAll([]byte(output), []byte(""))
	output = string(bytes)

	goldenFile := "golden/" + s.GoldenFileName + ".golden.yaml"

	if *update {
		err := ioutil.WriteFile(goldenFile, bytes, 0644)
		s.Require().NoError(err, "Golden file was not writable")
	}

	expected, err := ioutil.ReadFile(goldenFile)

	// then
	s.Require().NoError(err, "Golden file doesn't exist or was not readable")
	s.Require().Equal(string(expected), output)
}

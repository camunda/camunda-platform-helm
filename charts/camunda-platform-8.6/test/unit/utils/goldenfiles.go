// Copyright 2022 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"camunda-platform/test/unit/testhelpers"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"testing"

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
	IgnoredLines   []string
	SetValues      map[string]string
	ExtraHelmArgs  []string
}

func (s *TemplateGoldenTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:                    "TestContainerGoldenTestDefaults",
			Values:                  s.SetValues,
			RenderTemplateExtraArgs: s.ExtraHelmArgs,
			Verifier: func(t *testing.T, output string, err error) {
				s.IgnoredLines = append(s.IgnoredLines, `\s+helm.sh/chart:\s+.*`)
				bytes := []byte(output)
				for _, ignoredLine := range s.IgnoredLines {
					regex := regexp.MustCompile(ignoredLine)
					bytes = regex.ReplaceAll(bytes, []byte(""))
				}
				output = string(bytes)

				goldenFile := "golden/" + s.GoldenFileName + ".golden.yaml"

				if *update {
					err := os.WriteFile(goldenFile, bytes, 0644)
					s.Require().NoError(err, "Golden file was not writable")
				}

				expected, e := os.ReadFile(goldenFile)

				// then
				s.Require().NoError(e, "Golden file doesn't exist or was not readable")
				s.Require().Equal(string(expected), output)
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.ChartPath, s.Release, s.Namespace, s.Templates, testCases)
}

func (s *TemplateGoldenTest) TestContainerGoldenTestDefaults() {
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", s.Namespace),
		SetValues:      s.SetValues,
	}
	output := helm.RenderTemplate(s.T(), options, s.ChartPath, s.Release, s.Templates, s.ExtraHelmArgs...)

	s.IgnoredLines = append(s.IgnoredLines, `\s+helm.sh/chart:\s+.*`)
	bytes := []byte(output)
	for _, ignoredLine := range s.IgnoredLines {
		regex := regexp.MustCompile(ignoredLine)
		bytes = regex.ReplaceAll(bytes, []byte(""))
	}
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

// cleanupGoldenFiles removes golden files that are no longer needed.
func cleanupGoldenFiles(templateNames []string) {
	goldenDir := "golden"
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		if os.IsNotExist(err) {
			return // Directory doesn't exist, skip cleanup.
		}
		log.Printf("Failed to read golden dir: %v", err)
		return
	}

	// Build a set of expected file names.
	expected := make(map[string]struct{}, len(templateNames))
	for _, name := range templateNames {
		expected[name+".golden.yaml"] = struct{}{}
	}

	for _, entry := range entries {
		if _, keep := expected[entry.Name()]; keep {
			continue
		}

		path := filepath.Join(goldenDir, entry.Name())
		if err := os.Remove(path); err != nil {
			log.Printf("Failed to remove %s: %v", path, err)
		}
	}
}

// TestGoldenTemplates runs golden file tests for a list of template names using provided configuration.
func TestGoldenTemplates(t *testing.T, chartPath string, componentName string, templateNames []string, ignoredLines []string, setValues map[string]string, extraHelmArgs []string) {
	t.Parallel()
	for _, templateName := range templateNames {
		suite.Run(t, &TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform",
			GoldenFileName: templateName,
			Templates:      []string{fmt.Sprintf("templates/%s/%s.yaml", componentName, templateName)},
			IgnoredLines:   ignoredLines,
			SetValues:      setValues,
			ExtraHelmArgs:  extraHelmArgs,
		})
	}

	// Clean up golden files that don't match template names.
	cleanupGoldenFiles(templateNames)
}

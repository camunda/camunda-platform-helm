// Copyright 2026 Camunda Services GmbH
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

package camunda

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MultiregionMigrationConstraintTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestMultiregionMigrationConstraintTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &MultiregionMigrationConstraintTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/configmap-warnings.yaml"},
	})
}

func (s *MultiregionMigrationConstraintTemplateTest) TestRetainedNumberedTopologyInputs() {
	zonedMigrationValues := func() map[string]string {
		return map[string]string{
			"orchestration.multiregion.mode":                      "zoned",
			"orchestration.multiregion.zone":                      "zone-a",
			"orchestration.multiregion.zones[0].name":             "zone-a",
			"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
			"orchestration.multiregion.zones[0].numberOfReplicas": "1",
			"orchestration.multiregion.zones[0].priority":         "100",
			"orchestration.multiregion.keepUnzonedBrokers":        "true",
		}
	}

	invalidCases := []struct {
		name  string
		key   string
		value string
		err   string
	}{
		{name: "regions noninteger", key: "orchestration.multiregion.regions", value: "3.5"},
		{name: "regions negative", key: "orchestration.multiregion.regions", value: "-3", err: "orchestration.multiregion.regions must be a positive integer"},
		{name: "regions zero", key: "orchestration.multiregion.regions", value: "0", err: "orchestration.multiregion.regions must be a positive integer"},
		{name: "regionId noninteger", key: "orchestration.multiregion.regionId", value: "1.5", err: "orchestration.multiregion.regionId must be an integer"},
		{name: "regionId negative", key: "orchestration.multiregion.regionId", value: "-1", err: "orchestration.multiregion.regionId must be an integer"},
		{name: "regionId equal to regions", key: "orchestration.multiregion.regionId", value: "3", err: "orchestration.multiregion.regionId must be less than orchestration.multiregion.regions"},
		{name: "clusterSize noninteger", key: "orchestration.clusterSize", value: "6.5", err: "orchestration.clusterSize must be a positive integer"},
		{name: "clusterSize negative", key: "orchestration.clusterSize", value: "-6", err: "orchestration.clusterSize must be a positive integer"},
		{name: "clusterSize zero", key: "orchestration.clusterSize", value: "0", err: "orchestration.clusterSize must be a positive integer"},
		{name: "clusterSize not divisible by regions", key: "orchestration.clusterSize", value: "5", err: "orchestration.clusterSize must be divisible by orchestration.multiregion.regions"},
	}

	testCases := make([]testhelpers.TestCase, 0, len(invalidCases)+2)
	for _, testCase := range invalidCases {
		values := zonedMigrationValues()
		values["orchestration.multiregion.regions"] = "3"
		values["orchestration.multiregion.regionId"] = "0"
		renderArgs := []string{"--set-string", "orchestration.clusterSize=6"}
		if testCase.key == "orchestration.clusterSize" {
			renderArgs = []string{"--set-string", testCase.key + "=" + testCase.value}
		} else {
			values[testCase.key] = testCase.value
		}
		test := testhelpers.TestCase{
			Name:                    testCase.name,
			Values:                  values,
			RenderTemplateExtraArgs: renderArgs,
		}
		if testCase.err == "" {
			test.Verifier = func(t *testing.T, output string, err error) {
				require.Error(t, err)
			}
		} else {
			test.Expected = map[string]string{"ERROR": testCase.err}
		}
		testCases = append(testCases, test)
	}

	validIntegerValues := zonedMigrationValues()
	validIntegerValues["orchestration.multiregion.regions"] = "3"
	validIntegerValues["orchestration.multiregion.regionId"] = "0"
	testCases = append(testCases, testhelpers.TestCase{
		Name:                    "accepts integer topology inputs with regionId zero",
		Values:                  validIntegerValues,
		RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=6"},
		Verifier: func(t *testing.T, output string, err error) {
			require.NoError(t, err)
		},
	})

	validStringValues := zonedMigrationValues()
	testCases = append(testCases, testhelpers.TestCase{
		Name:   "accepts numeric string inputs with regionId zero",
		Values: validStringValues,
		RenderTemplateExtraArgs: []string{
			"--set-string", "orchestration.multiregion.regions=3",
			"--set-string", "orchestration.multiregion.regionId=0",
			"--set-string", "orchestration.clusterSize=6",
		},
		Verifier: func(t *testing.T, output string, err error) {
			require.NoError(t, err)
		},
	})

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

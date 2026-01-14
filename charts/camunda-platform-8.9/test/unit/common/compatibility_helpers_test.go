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

type CompatibilityHelpersTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestCompatibilityHelpers(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &CompatibilityHelpersTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/configmap-release.yaml"},
	})
}

func (c *CompatibilityHelpersTest) TestElasticsearchDisabledWithOpenShiftAdaptSecurityContext() {
	// This test verifies that the chart can be rendered when elasticsearch is disabled
	// and OpenShift adaptSecurityContext is set to "force".
	// This is a regression test for the bug where hasKey was called on a nil commonLabels.
	testCases := []testhelpers.TestCase{
		{
			Name: "ElasticsearchDisabledWithOpenShiftForce",
			Values: map[string]string{
				"elasticsearch.enabled":                                  "false",
				"global.compatibility.openshift.adaptSecurityContext":    "force",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Nil(t, err, "Chart rendering should succeed when elasticsearch is disabled with OpenShift adaptSecurityContext=force")
			},
		},
		{
			Name: "ElasticsearchDisabledWithOpenShiftForceNoCommonLabels",
			Values: map[string]string{
				"elasticsearch.enabled":                                  "false",
				"global.compatibility.openshift.adaptSecurityContext":    "force",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Nil(t, err, "Chart rendering should succeed when elasticsearch is disabled without commonLabels defined")
			},
		},
		{
			Name: "ElasticsearchEnabledWithOpenShiftForce",
			Values: map[string]string{
				"elasticsearch.enabled":                                  "true",
				"global.compatibility.openshift.adaptSecurityContext":    "force",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Nil(t, err, "Chart rendering should succeed when elasticsearch is enabled with OpenShift adaptSecurityContext=force")
			},
		},
		{
			Name: "ElasticsearchEnabledWithOpenShiftForceAndExistingCommonLabels",
			Values: map[string]string{
				"elasticsearch.enabled":                                  "true",
				"global.compatibility.openshift.adaptSecurityContext":    "force",
				"elasticsearch.commonLabels.custom-label":                "custom-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Nil(t, err, "Chart rendering should succeed when elasticsearch is enabled with existing commonLabels")
			},
		},
	}

	testhelpers.RunTestCasesE(c.T(), c.chartPath, c.release, c.namespace, c.templates, testCases)
}

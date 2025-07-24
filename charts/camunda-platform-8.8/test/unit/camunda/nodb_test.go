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

type NoDBTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestNoDbTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &NoDBTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func (s *NoDBTemplateTest) TestNoDbGlobalValue() {
	testCases := []testhelpers.TestCase{
		{
			Name:                 "TestGlobalNoDbTogglesAllExpectedValues",
			HelmOptionsExtraArgs: map[string][]string{"install": {"--debug"}},
			Values: map[string]string{
				"global.noDb": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// Database type should be none
				require.Contains(t, output, "database:\n        type: none")
				// Agentic AI and inbouond connectors should be disabled
				require.Contains(t, output, "webhook:\n          enabled: false")
				require.Contains(t, output, "polling:\n          enabled: false")
				require.Contains(t, output, "agenticai:\n          enabled: false")
				// Optimize should not be rendered
				require.NotContains(t, output, "charts/optimize")
				// Connectors should not be rendered
				require.NotContains(t, output, "charts/connectors")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

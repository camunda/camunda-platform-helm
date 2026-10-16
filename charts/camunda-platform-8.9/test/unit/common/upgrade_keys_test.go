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
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type UpgradeKeysTest struct {
	suite.Suite
	chartPath string
}

func TestUpgradeKeysTemplate(t *testing.T) {
	t.Parallel()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	suite.Run(t, &UpgradeKeysTest{chartPath: chartPath})
}

func (s *UpgradeKeysTest) TestLegacyKeysWarnWithoutFailing() {
	testCases := []testhelpers.TestCase{}
	for _, input := range []struct {
		key   string
		value string
	}{
		{"global.compatibility.orchestration", `{"enabled":false}`},
		{"orchestration.importer", `{}`},
		{"orchestration.migration", `{"data":{"enabled":true}}`},
		{"webModeler.webapp", `{"replicas":2}`},
	} {
		values := map[string]string{"orchestration.data.secondaryStorage.type": "elasticsearch"}
		args := []string{"--set-json", input.key + "=" + input.value}
		if input.key == "global.compatibility.orchestration" {
			values[input.key+".enabled"] = "false"
			args = nil
		}
		testCases = append(testCases, testhelpers.TestCase{
			Name:                    input.key,
			Values:                  values,
			RenderTemplateExtraArgs: args,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var warnings corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &warnings)
				require.Equal(t, "camunda-platform-test-warnings", warnings.Name)
				require.Contains(t, warnings.Data["warnings"], "[camunda][warning]")
				require.Contains(t, warnings.Data["warnings"], `"`+input.key+`" has no effect since chart 14 (Camunda 8.9). Remove it.`)
				require.NotContains(t, warnings.Data["warnings"], "DEPRECATION:")
				require.Contains(t, warnings.Data["warnings"], "https://docs.camunda.io/docs/8.9/self-managed/upgrade/helm/")
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, "camunda-platform-test", "upgrade-keys", []string{"templates/common/configmap-warnings.yaml"}, testCases)
}

func (s *UpgradeKeysTest) TestSupportedGlobalImageTagDoesNotWarn() {
	testhelpers.RunTestCasesE(s.T(), s.chartPath, "camunda-platform-test", "upgrade-keys", []string{"templates/common/configmap-warnings.yaml"}, []testhelpers.TestCase{{
		Name: "SupportedGlobalImageTag",
		Values: map[string]string{
			"orchestration.data.secondaryStorage.type": "elasticsearch",
			"global.image.tag":                         "legacy-fallback",
			"elasticsearch.enabled":                    "true",
			"global.elasticsearch.enabled":             "true",
		},
		Verifier: func(t *testing.T, output string, err error) {
			require.NoError(t, err)
			var warnings corev1.ConfigMap
			helm.UnmarshalK8SYaml(t, output, &warnings)
			require.NotEmpty(t, warnings.Data["warnings"])
			require.NotContains(t, warnings.Data["warnings"], "global.image.tag")
		},
	}})
}

func (s *UpgradeKeysTest) TestAbsentLegacyKeysDoNotWarn() {
	testhelpers.RunTestCasesE(s.T(), s.chartPath, "camunda-platform-test", "upgrade-keys", []string{"templates/common/configmap-warnings.yaml"}, []testhelpers.TestCase{{
		Name: "AbsentLegacyKeys",
		Values: map[string]string{
			"orchestration.data.secondaryStorage.type": "elasticsearch",
			"elasticsearch.enabled":                    "true",
			"global.elasticsearch.enabled":             "true",
		},
		Verifier: func(t *testing.T, output string, err error) {
			require.NoError(t, err)
			var warnings corev1.ConfigMap
			helm.UnmarshalK8SYaml(t, output, &warnings)
			require.NotEmpty(t, warnings.Data["warnings"])
			for _, key := range []string{"global.compatibility.orchestration", "global.image.tag", "orchestration.importer", "orchestration.migration", "webModeler.webapp"} {
				require.NotContains(t, warnings.Data["warnings"], `"`+key+`"`)
			}
		},
	}})
}

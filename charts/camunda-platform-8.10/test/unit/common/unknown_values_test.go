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

type UnknownValuesTest struct {
	suite.Suite
	chartPath string
}

func TestUnknownValuesTemplate(t *testing.T) {
	t.Parallel()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	suite.Run(t, &UnknownValuesTest{chartPath: chartPath})
}

func (s *UnknownValuesTest) TestWarningsWhenKeysAreUnknown() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TypoWarnsWithoutFailing",
			Values: map[string]string{"orchestration.replicasx": "3"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				s.Contains(configmap.Data["warnings"], "[camunda][warning] UNKNOWN VALUES KEY: orchestration.replicasx. Helm ignores this key. Remove or correct it. A future chart version can reject unknown keys.")
			},
		},
		{
			Name:     "DefaultsProduceNoUnknownWarning",
			Verifier: s.verifyWarningsAbsent,
		},
		{
			Name: "FreeFormAnnotationsAndPodLabelsProduceNoUnknownWarning",
			Values: map[string]string{
				"orchestration.podAnnotations.custom":      "value",
				"orchestration.podLabels.custom":           "value",
				"orchestration.service.annotations.custom": "value",
				"global.annotations.custom":                "value",
				"global.strictValues":                       "true",
				"prometheusServiceMonitor.labels.custom":     "value",
			},
			Verifier: s.verifyWarningsAbsent,
		},
		{
			Name: "ParentChartGlobalProducesNoUnknownWarningEvenWhenStrict",
			Values: map[string]string{
				"global.storageClass": "parent-owned",
				"global.strictValues": "true",
			},
			Verifier: s.verifyWarningsAbsent,
		},
		{
			Name: "DeprecatedConsoleKeysAreNotUnknownEvenWhenStrict",
			Values: map[string]string{
				"console.nodeEnv":     "legacy",
				"global.strictValues": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				s.Contains(configmap.Data["warnings"], "console.* configuration keys have no effect")
				s.NotContains(configmap.Data["warnings"], "UNKNOWN VALUES KEY:")
			},
		},
		{
			Name: "DeprecatedMappingRulesAreNotUnknownEvenWhenStrict",
			Values: map[string]string{
				"orchestration.security.initialization.defaultRoles.admin.mappingRules[0]": "legacy-rule",
				"global.strictValues": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				s.Contains(configmap.Data["warnings"], "defaultRoles.admin.mappingRules\" is deprecated")
				s.NotContains(configmap.Data["warnings"], "UNKNOWN VALUES KEY:")
			},
		},
		{
			Name: "RemovedKeysKeepTheirMigrationErrorWhenStrict",
			Values: map[string]string{
				"identityKeycloak.enabled": "false",
				"global.strictValues":      "true",
			},
			Expected: map[string]string{"ERROR": "The Helm values file key \"identityKeycloak\" has been removed"},
		},
		{
			Name:   "UnknownRootWarnsWithoutExposingValue",
			Values: map[string]string{"unknownRoot": "private-value"},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				s.Contains(configmap.Data["warnings"], "UNKNOWN VALUES KEY: unknownRoot.")
				s.NotContains(configmap.Data["warnings"], "private-value")
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, "camunda-platform-test", "unknown-values-test",
		[]string{"templates/common/configmap-warnings.yaml"}, testCases)
}

func (s *UnknownValuesTest) verifyWarningsAbsent(t *testing.T, output string, err error) {
	if err != nil {
		s.Contains(err.Error(), "could not find template templates/common/configmap-warnings.yaml")
		return
	}
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(t, output, &configmap)
	s.Equal("camunda-platform-test-warnings", configmap.Name)
	s.NotContains(configmap.Data["warnings"], "UNKNOWN VALUES KEY:")
}

func (s *UnknownValuesTest) TestStrictValuesFailsWhenTypoIsPresent() {
	testhelpers.RunTestCasesE(s.T(), s.chartPath, "camunda-platform-test", "unknown-values-test",
		[]string{"templates/common/configmap-warnings.yaml"}, []testhelpers.TestCase{
			{
				Name: "StrictValuesFailsWhenTypoIsPresent",
				Values: map[string]string{
					"orchestration.replicasx": "3",
					"global.strictValues":     "true",
				},
				Expected: map[string]string{"ERROR": "Unknown values keys (global.strictValues=true): orchestration.replicasx"},
			},
		})
}

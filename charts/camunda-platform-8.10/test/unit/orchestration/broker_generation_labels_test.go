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

package orchestration

import (
	"camunda-platform/test/unit/testhelpers"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

func (s *StatefulSetTest) TestMigrationSeparatesBrokerGenerationSelectors() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestLegacyGlobalZoneLabelRemainsOnNumberedSelectorOnly",
			Values: map[string]string{
				"global.labels.camunda\\.io/zone":                     "legacy-zone",
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "zone-a",
				"orchestration.multiregion.zones[0].name":             "zone-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.keepUnzonedBrokers":        "true",
				"orchestration.multiregion.regions":                   "1",
				"orchestration.multiregion.regionId":                  "0",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				statefulSets := map[string]appsv1.StatefulSet{}
				for _, document := range strings.Split(output, "\n---\n") {
					var statefulSet appsv1.StatefulSet
					helm.UnmarshalK8SYaml(t, document, &statefulSet)
					statefulSets[statefulSet.Name] = statefulSet
				}

				require.Contains(t, statefulSets, s.release+"-zeebe")
				require.Contains(t, statefulSets, s.release+"-zeebe-zone-a")
				numbered := statefulSets[s.release+"-zeebe"]
				require.Equal(t, "legacy-zone", numbered.Spec.Selector.MatchLabels["camunda.io/zone"])
				require.NotContains(t, numbered.Spec.Selector.MatchLabels, "camunda.io/broker-generation")
				require.Equal(t, "numbered", numbered.Spec.Template.Labels["camunda.io/broker-generation"])

				zoned := statefulSets[s.release+"-zeebe-zone-a"]
				require.Equal(t, "zone-a", zoned.Spec.Selector.MatchLabels["camunda.io/zone"])
				require.Equal(t, "zoned", zoned.Spec.Selector.MatchLabels["camunda.io/broker-generation"])
				require.Equal(t, zoned.Spec.Selector.MatchLabels["camunda.io/zone"], zoned.Spec.Template.Labels["camunda.io/zone"])
				require.Equal(t, zoned.Spec.Selector.MatchLabels["camunda.io/broker-generation"], zoned.Spec.Template.Labels["camunda.io/broker-generation"])
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

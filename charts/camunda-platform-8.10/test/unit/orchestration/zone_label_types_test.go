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
	"testing"

	"camunda-platform/test/unit/testhelpers"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func (s *StatefulSetTest) TestZoneLabelsRemainStrings() {
	for _, zone := range []string{"123", "true", "null", "zone-a"} {
		testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, []testhelpers.TestCase{{
			Name: zone,
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.multiregion.zone=" + zone + ",orchestration.multiregion.zones[0].name=" + zone},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				jsonOutput, err := yaml.YAMLToJSON([]byte(output))
				require.NoError(t, err)
				require.Contains(t, string(jsonOutput), `"camunda.io/zone":"`+zone+`"`)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.Equal(t, zone, statefulSet.Spec.Selector.MatchLabels["camunda.io/zone"])
			},
		}})
	}
}

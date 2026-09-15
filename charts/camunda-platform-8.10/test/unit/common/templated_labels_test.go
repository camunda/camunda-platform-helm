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
	"encoding/json"
	"testing"

	"camunda-platform/test/unit/testhelpers"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

func (s *ConstraintTemplateTest) TestTemplatedReservedLabels() {
	for _, source := range []string{"global.commonLabels", "orchestration.podLabels"} {
		for _, key := range []string{
			`{{ print "camunda.io/broker-generation" }}`,
			`{{ if .OrchestrationRender }}camunda.io/broker-generation{{ else }}safe-label{{ end }}`,
		} {
			encoded, err := json.Marshal(map[string]string{key: "override"})
			s.Require().NoError(err)
			testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace,
				[]string{"templates/orchestration/statefulset.yaml"}, []testhelpers.TestCase{{
					Name:                    source + key,
					RenderTemplateExtraArgs: []string{"--set-json", source + "=" + string(encoded)},
					Verifier: func(t *testing.T, output string, err error) {
						require.ErrorContains(t, err, "camunda.io/broker-generation is managed by the chart")
					},
				}})
		}
	}
}

func (s *ConstraintTemplateTest) TestStatefulLabelTemplateCannotOverrideGeneration() {
	encoded, err := json.Marshal(map[string]string{
		`{{ if .Values.pwn }}{{ $_ := set .Values "pwn" false }}camunda.io/broker-generation{{ else }}{{ $_ := set .Values "pwn" true }}safe-label{{ end }}`: "attacker",
	})
	s.Require().NoError(err)
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/orchestration/statefulset.yaml"}, []testhelpers.TestCase{{
			Name:                    "StatefulPodLabel",
			RenderTemplateExtraArgs: []string{"--set-json", "orchestration.podLabels=" + string(encoded)},
			Verifier: func(t *testing.T, output string, err error) {
				if err != nil {
					require.ErrorContains(t, err, "camunda.io/broker-generation is managed by the chart")
					return
				}
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.Equal(t, "numbered", statefulSet.Spec.Template.Labels["camunda.io/broker-generation"])
			},
		}})
}

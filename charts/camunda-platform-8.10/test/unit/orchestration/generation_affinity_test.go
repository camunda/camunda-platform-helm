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
	"encoding/json"
	"testing"

	"camunda-platform/test/unit/testhelpers"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *StatefulSetTest) TestGenerationAffinitySelectors() {
	for _, mode := range []string{"numbered", "zoned"} {
		for _, tc := range []struct {
			name     string
			selector *metav1.LabelSelector
			scoped   bool
		}{
			{name: "broker-expression", selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app.kubernetes.io/component", Operator: metav1.LabelSelectorOpIn, Values: []string{"zeebe-broker"}}}}, scoped: true},
			{name: "broker-label", selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/component": "zeebe-broker"}}, scoped: true},
			{name: "negative-expression", selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app.kubernetes.io/component", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"zeebe-broker"}}}}},
			{name: "broad-expression", selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app.kubernetes.io/component", Operator: metav1.LabelSelectorOpIn, Values: []string{"zeebe-broker", "postgres"}}}}},
			{name: "absent-selector"},
		} {
			s.Run(mode+"/"+tc.name, func() {
				term := corev1.PodAffinityTerm{LabelSelector: tc.selector, TopologyKey: "kubernetes.io/hostname"}
				affinity := corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{term}}}
				encoded, err := json.Marshal(affinity)
				s.Require().NoError(err)
				values := map[string]string{"orchestration.multiregion.mode": mode}
				if mode == "zoned" {
					values["orchestration.multiregion.zone"] = "zone-a"
					values["orchestration.multiregion.zones[0].name"] = "zone-a"
					values["orchestration.multiregion.zones[0].numberOfBrokers"] = "1"
					values["orchestration.multiregion.zones[0].numberOfReplicas"] = "1"
					values["orchestration.multiregion.zones[0].priority"] = "100"
				}
				testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, []testhelpers.TestCase{{
					Name: tc.name, Values: values,
					RenderTemplateExtraArgs: []string{"--set-json", "orchestration.affinity=" + string(encoded)},
					Verifier: func(t *testing.T, output string, err error) {
						require.NoError(t, err)
						var statefulSet appsv1.StatefulSet
						helm.UnmarshalK8SYaml(t, output, &statefulSet)
						expected := term.DeepCopy()
						if tc.scoped {
							expected.LabelSelector.MatchExpressions = append(expected.LabelSelector.MatchExpressions, metav1.LabelSelectorRequirement{Key: "camunda.io/broker-generation", Operator: metav1.LabelSelectorOpIn, Values: []string{mode}})
						}
						require.Equal(t, *expected, statefulSet.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0], "only positive broker-only selectors may gain generation scoping")
					},
				}})
			})
		}
	}
}

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
		}{
			{name: "broker-expression", selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app.kubernetes.io/component", Operator: metav1.LabelSelectorOpIn, Values: []string{"zeebe-broker"}}}}},
			{name: "broker-label", selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/component": "zeebe-broker"}}},
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
						require.Equal(t, term, statefulSet.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0], "custom affinity must pass through unchanged")
					},
				}})
			})
		}
	}
}

func (s *StatefulSetTest) TestDefaultAffinityScopesBrokerGeneration() {
	for _, tc := range []struct {
		name       string
		generation string
		values     map[string]string
	}{
		{name: "numbered", generation: "numbered", values: map[string]string{}},
		{name: "zoned", generation: "zoned", values: map[string]string{
			"orchestration.multiregion.mode":                      "zoned",
			"orchestration.multiregion.zone":                      "zone-a",
			"orchestration.multiregion.zones[0].name":             "zone-a",
			"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
			"orchestration.multiregion.zones[0].numberOfReplicas": "1",
			"orchestration.multiregion.zones[0].priority":         "100",
		}},
	} {
		s.Run(tc.name, func() {
			testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, []testhelpers.TestCase{{
				Name: tc.name, Values: tc.values,
				Verifier: func(t *testing.T, output string, err error) {
					require.NoError(t, err)
					var statefulSet appsv1.StatefulSet
					helm.UnmarshalK8SYaml(t, output, &statefulSet)
					require.Equal(t, []metav1.LabelSelectorRequirement{
						{Key: "app.kubernetes.io/component", Operator: metav1.LabelSelectorOpIn, Values: []string{"zeebe-broker"}},
						{Key: "camunda.io/broker-generation", Operator: metav1.LabelSelectorOpIn, Values: []string{tc.generation}},
					}, statefulSet.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions)
				},
			}})
		})
	}
}

func (s *StatefulSetTest) TestNullAffinityRemainsDisabled() {
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, []testhelpers.TestCase{{
		Name:                    "null-affinity",
		RenderTemplateExtraArgs: []string{"--set-json", "orchestration.affinity=null"},
		Verifier: func(t *testing.T, output string, err error) {
			require.NoError(t, err)
			var statefulSet appsv1.StatefulSet
			helm.UnmarshalK8SYaml(t, output, &statefulSet)
			require.Nil(t, statefulSet.Spec.Template.Spec.Affinity)
		},
	}})
}

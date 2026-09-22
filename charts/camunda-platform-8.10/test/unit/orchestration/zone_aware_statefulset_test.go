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
	"fmt"
	"path/filepath"

	"camunda-platform/test/unit/testhelpers"
	"camunda-platform/test/unit/utils"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

func (s *StatefulSetTest) TestZonedNameIsStableAcrossOrdinalWidth() {
	expectedName := strings.Repeat("a", 52) + "-zone-a"
	testCases := make([]testhelpers.TestCase, 0, 3)
	for _, brokers := range []string{"9", "11", "999"} {
		testCases = append(testCases, testhelpers.TestCase{
			Name: "TestZonedNameWith" + brokers + "Brokers",
			Values: map[string]string{
				"orchestration.fullnameOverride":                       strings.Repeat("a", 63),
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "zone-a",
				"orchestration.partitioning.zones[0].name":             "zone-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  brokers,
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.Equal(t, expectedName, statefulSet.Name)
				require.LessOrEqual(t, len(statefulSet.Name+"-998"), 63)
			},
		})
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *StatefulSetTest) renderStatefulSet(values map[string]string, name string) appsv1.StatefulSet {
	output, err := testhelpers.RenderTestCaseE(
		s.T(), s.chartPath, s.release, s.namespace, s.templates,
		testhelpers.TestCase{Values: values},
	)
	require.NoError(s.T(), err)

	for _, doc := range strings.Split(output, "\n---\n") {
		if !strings.Contains(doc, "name: "+name+"\n") {
			continue
		}
		var statefulSet appsv1.StatefulSet
		helm.UnmarshalK8SYaml(s.T(), doc, &statefulSet)
		return statefulSet
	}
	require.Failf(s.T(), "statefulset not found", "no StatefulSet named %q in render", name)
	return appsv1.StatefulSet{}
}

func (s *StatefulSetTest) TestMigrationPreservesNumberedStatefulSet() {
	baseValues := map[string]string{
		"orchestration.partitioning.scheme":        "round-robin",
		"orchestration.partitioning.numberOfZones": "3",
		"orchestration.partitioning.zoneIndex":     "1",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
	}

	before := s.renderStatefulSet(baseValues, s.release+"-zeebe")
	migrationValues := utils.MergeMaps(baseValues, map[string]string{
		"orchestration.partitioning.scheme":                    "zone-aware",
		"orchestration.partitioning.zone":                      "zone-b",
		"orchestration.partitioning.keepUnzonedBrokers":        "true",
		"orchestration.partitioning.zones[0].name":             "zone-a",
		"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
		"orchestration.partitioning.zones[0].numberOfReplicas": "1",
		"orchestration.partitioning.zones[0].priority":         "100",
		"orchestration.partitioning.zones[1].name":             "zone-b",
		"orchestration.partitioning.zones[1].numberOfBrokers":  "1",
		"orchestration.partitioning.zones[1].numberOfReplicas": "1",
		"orchestration.partitioning.zones[1].priority":         "90",
		"orchestration.partitioning.zones[2].name":             "zone-c",
		"orchestration.partitioning.zones[2].numberOfBrokers":  "1",
		"orchestration.partitioning.zones[2].numberOfReplicas": "1",
		"orchestration.partitioning.zones[2].priority":         "80",
	})

	retained := s.renderStatefulSet(migrationValues, s.release+"-zeebe")
	require.Equal(s.T(), before, retained)
	require.Equal(s.T(), int32(1), *retained.Spec.Replicas)
}

func (s *StatefulSetTest) TestKeepUnzonedBrokersDoesNotRestartZonedBrokers() {
	zonedValues := map[string]string{
		"orchestration.partitioning.scheme":                    "zone-aware",
		"orchestration.partitioning.zones[0].name":             "zone-a",
		"orchestration.partitioning.zones[0].numberOfBrokers":  "3",
		"orchestration.partitioning.zones[0].numberOfReplicas": "3",
		"orchestration.partitioning.zones[0].priority":         "100",
		"orchestration.partitioning.zone":                      "zone-a",
		"orchestration.partitioning.numberOfZones":             "1",
		"orchestration.partitioning.zoneIndex":                 "0",
		"orchestration.data.secondaryStorage.type":             "elasticsearch",
	}

	renderFor := func(keepUnzoned string) appsv1.StatefulSet {
		values := utils.MergeMaps(map[string]string{}, zonedValues)
		values["orchestration.partitioning.keepUnzonedBrokers"] = keepUnzoned

		return s.renderStatefulSet(values, s.release+"-zeebe-zone-a")
	}

	withUnzoned := renderFor("true")
	withoutUnzoned := renderFor("false")
	withChecksum := withUnzoned.Spec.Template.Annotations["checksum/config"]
	withoutChecksum := withoutUnzoned.Spec.Template.Annotations["checksum/config"]
	require.NotEmpty(s.T(), withChecksum)
	require.Equal(s.T(), withChecksum, withoutChecksum)
	require.Equal(s.T(), withUnzoned.Spec.Template, withoutUnzoned.Spec.Template)
}

func (s *StatefulSetTest) TestMigrationKeepsNumberedSizingValues() {
	numbered := map[string]string{
		"orchestration.partitioning.numberOfZones": "2",
		"orchestration.partitioning.zoneIndex":     "0",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
	}

	render := func(values map[string]string, name string) appsv1.StatefulSet {
		output, err := testhelpers.RenderTestCaseE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testhelpers.TestCase{
			Values:                  values,
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=4,orchestration.replicationFactor=2"},
		})
		require.NoError(s.T(), err)
		for _, doc := range strings.Split(output, "\n---\n") {
			if !strings.Contains(doc, "name: "+name+"\n") {
				continue
			}
			var sts appsv1.StatefulSet
			helm.UnmarshalK8SYaml(s.T(), doc, &sts)
			return sts
		}
		require.Failf(s.T(), "statefulset not found", "no StatefulSet named %q", name)
		return appsv1.StatefulSet{}
	}

	before := render(numbered, s.release+"-zeebe")

	for _, zoneCount := range []int{1, 2, 3} {
		migrating := utils.MergeMaps(numbered, map[string]string{
			"orchestration.partitioning.scheme":             "zone-aware",
			"orchestration.partitioning.zone":               "zone-a",
			"orchestration.partitioning.keepUnzonedBrokers": "true",
		})
		for i := 0; i < zoneCount; i++ {
			prefix := fmt.Sprintf("orchestration.partitioning.zones[%d].", i)
			migrating[prefix+"name"] = fmt.Sprintf("zone-%c", rune('a'+i))
			migrating[prefix+"numberOfBrokers"] = "2"
			migrating[prefix+"numberOfReplicas"] = "1"
			migrating[prefix+"priority"] = fmt.Sprintf("%d", 100-i*10)
		}

		retained := render(migrating, s.release+"-zeebe")
		require.Equalf(s.T(), *before.Spec.Replicas, *retained.Spec.Replicas,
			"entering migration with %d zone(s) must not resize the retained StatefulSet", zoneCount)
		require.Equalf(s.T(), before.Spec.Template, retained.Spec.Template,
			"entering migration with %d zone(s) must not change the retained pod template", zoneCount)

		zoned := render(migrating, s.release+"-zeebe-zone-a")
		require.Equalf(s.T(), int32(2), *zoned.Spec.Replicas,
			"zoned StatefulSet takes its replicas from its own zone entry, %d zone(s)", zoneCount)
	}
}

func (s *StatefulSetTest) TestScopedRenderKeepsTheSubchartsBuiltin() {
	for _, tc := range []struct {
		name       string
		valuesFile string
	}{
		{name: "TestUnzonedScopeResolvesSubcharts", valuesFile: "values-subcharts-probe.yaml"},
		{name: "TestZonedScopeResolvesSubcharts", valuesFile: "values-subcharts-probe-zoned.yaml"},
	} {
		s.Run(tc.name, func() {
			output, err := testhelpers.RenderTestCaseE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testhelpers.TestCase{
				ValuesFiles: []string{filepath.Join(s.chartPath, "test/unit/orchestration/testdata", tc.valuesFile)},
			})
			require.NoError(s.T(), err)
			var sts appsv1.StatefulSet
			helm.UnmarshalK8SYaml(s.T(), strings.Split(output, "\n---\n")[0], &sts)
			require.Equal(s.T(), "map", sts.Spec.Template.Labels["probe"])
		})
	}
}

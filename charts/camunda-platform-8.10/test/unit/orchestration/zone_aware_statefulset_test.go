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

	"camunda-platform/test/unit/testhelpers"
	"camunda-platform/test/unit/utils"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func (s *StatefulSetTest) TestZonedMode() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestZonedModeUsesLocalZoneBrokerCountAndEnvironmentVariable",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-b",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "3",
				"orchestration.multiregion.zones[1].numberOfReplicas": "3",
				"orchestration.multiregion.zones[1].priority":         "50",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				require.Equal(t, int32(3), *statefulSet.Spec.Replicas)
				var zoneEnv *corev1.EnvVar
				for i := range statefulSet.Spec.Template.Spec.Containers[0].Env {
					if statefulSet.Spec.Template.Spec.Containers[0].Env[i].Name == "CAMUNDA_CLUSTER_ZONE" {
						zoneEnv = &statefulSet.Spec.Template.Spec.Containers[0].Env[i]
					}
				}
				require.NotNil(t, zoneEnv)
				require.Equal(t, "region-b", zoneEnv.Value)
			},
		},
		{
			Name: "TestZonedModePreservesTheZoneSuffixForLongNames",
			Values: map[string]string{
				"orchestration.fullnameOverride":                      "camunda-production-orchestration-cluster-emea-primary-zeebe",
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "zone-b",
				"orchestration.multiregion.zones[0].name":             "zone-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "zone-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[1].numberOfReplicas": "1",
				"orchestration.multiregion.zones[1].priority":         "90",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				require.Equal(t, "camunda-production-orchestration-cluster-emea-primar-zone-b", statefulSet.Name)
				require.LessOrEqual(t, len(statefulSet.Name+"-0"), 63)
			},
		},
		{
			// The resolver falls back to the deprecated block when the orchestration
			// one is untouched. The numbering pair only: zoned mode never shipped under
			// global.multiregion, and setting both blocks is rejected outright.
			Name:                    "TestNumberedFieldsStillResolveFromTheDeprecatedGlobalBlock",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=4"},
			Values: map[string]string{
				"global.multiregion.regions":  "2",
				"global.multiregion.regionId": "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)

				require.Equal(t, int32(2), *statefulSet.Spec.Replicas)
			},
		},
		{
			Name: "TestZonedModeRejectsBothMultiregionBlocks",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"global.multiregion.regions":                          "2",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "orchestration.multiregion and global.multiregion are both configured")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *StatefulSetTest) TestZonedNameIsStableAcrossOrdinalWidth() {
	expectedName := strings.Repeat("a", 52) + "-zone-a"
	testCases := make([]testhelpers.TestCase, 0, 3)
	for _, brokers := range []string{"9", "11", "999"} {
		testCases = append(testCases, testhelpers.TestCase{
			Name: "TestZonedNameWith" + brokers + "Brokers",
			Values: map[string]string{
				"orchestration.fullnameOverride":                      strings.Repeat("a", 63),
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "zone-a",
				"orchestration.multiregion.zones[0].name":             "zone-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  brokers,
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
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

func (s *StatefulSetTest) TestNumberedModeCompatibility() {
	testCases := []testhelpers.TestCase{
		{
			Name:                    "DefaultModeUsesNumberedReplicaDivision",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=3"},
			Values: map[string]string{
				"global.multiregion.regions":  "1",
				"global.multiregion.regionId": "0",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.Equal(t, int32(3), *statefulSet.Spec.Replicas)
				for _, env := range statefulSet.Spec.Template.Spec.Containers[0].Env {
					require.NotEqual(t, "CAMUNDA_CLUSTER_ZONE", env.Name)
				}
			},
		},
		{
			Name:                    "ExplicitNumberedModeUsesNumberedReplicaDivision",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=6"},
			Values: map[string]string{
				"orchestration.multiregion.mode": "numbered",
				"global.multiregion.regions":     "2",
				"global.multiregion.regionId":    "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.Equal(t, int32(3), *statefulSet.Spec.Replicas)
				for _, env := range statefulSet.Spec.Template.Spec.Containers[0].Env {
					require.NotEqual(t, "CAMUNDA_CLUSTER_ZONE", env.Name)
				}
			},
		},
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
		"orchestration.multiregion.mode":           "numbered",
		"orchestration.multiregion.regions":        "3",
		"orchestration.multiregion.regionId":       "1",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
	}

	before := s.renderStatefulSet(baseValues, s.release+"-zeebe")
	migrationValues := utils.MergeMaps(baseValues, map[string]string{
		"orchestration.multiregion.mode":                      "zoned",
		"orchestration.multiregion.zone":                      "zone-b",
		"orchestration.multiregion.keepUnzonedBrokers":        "true",
		"orchestration.multiregion.zones[0].name":             "zone-a",
		"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
		"orchestration.multiregion.zones[0].numberOfReplicas": "1",
		"orchestration.multiregion.zones[0].priority":         "100",
		"orchestration.multiregion.zones[1].name":             "zone-b",
		"orchestration.multiregion.zones[1].numberOfBrokers":  "1",
		"orchestration.multiregion.zones[1].numberOfReplicas": "1",
		"orchestration.multiregion.zones[1].priority":         "90",
		"orchestration.multiregion.zones[2].name":             "zone-c",
		"orchestration.multiregion.zones[2].numberOfBrokers":  "1",
		"orchestration.multiregion.zones[2].numberOfReplicas": "1",
		"orchestration.multiregion.zones[2].priority":         "80",
	})

	retained := s.renderStatefulSet(migrationValues, s.release+"-zeebe")
	require.Equal(s.T(), before, retained)
	require.Equal(s.T(), int32(1), *retained.Spec.Replicas)
}

func (s *StatefulSetTest) TestKeepUnzonedBrokersDoesNotRestartZonedBrokers() {
	zonedValues := map[string]string{
		"orchestration.multiregion.mode":                      "zoned",
		"orchestration.multiregion.zones[0].name":             "zone-a",
		"orchestration.multiregion.zones[0].numberOfBrokers":  "3",
		"orchestration.multiregion.zones[0].numberOfReplicas": "3",
		"orchestration.multiregion.zones[0].priority":         "100",
		"orchestration.multiregion.zone":                      "zone-a",
		"orchestration.multiregion.regions":                   "1",
		"orchestration.multiregion.regionId":                  "0",
		"orchestration.data.secondaryStorage.type":            "elasticsearch",
	}

	renderFor := func(keepUnzoned string) appsv1.StatefulSet {
		values := utils.MergeMaps(map[string]string{}, zonedValues)
		values["orchestration.multiregion.keepUnzonedBrokers"] = keepUnzoned

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

// While keepUnzonedBrokers is set, clusterSize and replicationFactor still describe the
// numbered generation: clusterSize divided by regions is the retained StatefulSet's replica
// count, and replicationFactor is rendered into its ConfigMap. Forcing them to the zone
// totals would resize and restart the brokers the migration exists to preserve, so the
// zoned constraints must stand down until retention is disabled.
func (s *StatefulSetTest) TestMigrationKeepsNumberedSizingValues() {
	// clusterSize and replicationFactor are string-typed in the schema, so they have to go
	// through --set-string rather than --set.
	strValues := map[string]string{
		"orchestration.clusterSize":       "4",
		"orchestration.replicationFactor": "2",
	}
	numbered := map[string]string{
		"orchestration.multiregion.regions":        "2",
		"orchestration.multiregion.regionId":       "0",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
	}

	render := func(values map[string]string, name string) appsv1.StatefulSet {
		output, err := helm.RenderTemplateE(s.T(), &helm.Options{
			SetValues: values, SetStrValues: strValues,
		}, s.chartPath, s.release, s.templates)
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

	// One zone and several zones take different paths: a single zone is one failure domain, so
	// the chart still generates initial-contact-points, while more than one hands that to the
	// operator. The retained generation must be untouched either way.
	for _, zoneCount := range []int{1, 2, 3} {
		migrating := utils.MergeMaps(numbered, map[string]string{
			"orchestration.multiregion.mode":               "zoned",
			"orchestration.multiregion.zone":               "zone-a",
			"orchestration.multiregion.keepUnzonedBrokers": "true",
		})
		for i := 0; i < zoneCount; i++ {
			prefix := fmt.Sprintf("orchestration.multiregion.zones[%d].", i)
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

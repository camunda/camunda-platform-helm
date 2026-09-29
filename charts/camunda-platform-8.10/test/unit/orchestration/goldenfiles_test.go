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

package orchestration

import (
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenDefaultsTemplateOrchestration(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	templateNames := []string{
		"service",
		"service-headless",
		"serviceaccount",
		"statefulset",
		"configmap",
	}

	for _, name := range templateNames {
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
			GoldenFileName: name,
			Templates:      []string{"templates/orchestration/" + name + ".yaml"},
			IgnoredLines: []string{
				`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
			},
		})
	}
}

// The goldens above render the chart defaults, which means the round-robin scheme. Nothing
// byte-pins the zone-aware branch, so a drift in what camundaPlatform.partitioning derives
// from the zone list would only be caught by whichever assertion happened to name the value.
// These pin the two resources the derivation actually reaches: the configuration, which
// carries the summed cluster size and replication factor and the zone list itself, and the
// StatefulSet, whose replica count comes from the local zone.
//
// The zone list is deliberately asymmetric. Equal zones would still match after a change that
// totalled the wrong field or picked the wrong zone.
func TestGoldenZoneAwareTemplateOrchestration(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	zoneAware := map[string]string{
		"orchestration.partitioning.scheme":                    "zone-aware",
		"orchestration.partitioning.zone":                      "london",
		"orchestration.partitioning.zones[0].name":             "paris",
		"orchestration.partitioning.zones[0].numberOfBrokers":  "4",
		"orchestration.partitioning.zones[0].numberOfReplicas": "3",
		"orchestration.partitioning.zones[0].priority":         "3",
		"orchestration.partitioning.zones[1].name":             "london",
		"orchestration.partitioning.zones[1].numberOfBrokers":  "2",
		"orchestration.partitioning.zones[1].numberOfReplicas": "2",
		"orchestration.partitioning.zones[1].priority":         "2",
		"orchestration.partitioning.zones[2].name":             "dublin",
		"orchestration.partitioning.zones[2].numberOfBrokers":  "1",
		"orchestration.partitioning.zones[2].numberOfReplicas": "1",
		"orchestration.partitioning.zones[2].priority":         "1",
	}

	for _, name := range []string{"configmap", "statefulset"} {
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
			GoldenFileName: name + "-zone-aware",
			Templates:      []string{"templates/orchestration/" + name + ".yaml"},
			SetValues:      zoneAware,
			IgnoredLines: []string{
				`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
			},
		})
	}
}

func TestGoldenZoneAwareMigrationTemplateOrchestration(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	templateNames := []string{
		"service",
		"service-headless",
		"serviceaccount",
		"statefulset",
		"configmap",
		"poddisruptionbudget",
	}
	valuesFile := filepath.Join(chartPath, "test/unit/orchestration/testdata/values-zone-aware-migration.yaml")
	states := []struct {
		name      string
		setValues map[string]string
	}{
		{name: "migration"},
		{
			name: "zoned",
			setValues: map[string]string{
				"orchestration.partitioning.keepUnzonedBrokers": "false",
				"orchestration.partitioning.numberOfZones":      "1",
				"orchestration.partitioning.zoneIndex":          "0",
			},
		},
	}

	for _, state := range states {
		for _, name := range templateNames {
			suite.Run(t, &utils.TemplateGoldenTest{
				ChartPath:      chartPath,
				Release:        "camunda-platform-test",
				Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
				GoldenFileName: state.name + "-" + name,
				Templates:      []string{"templates/orchestration/" + name + ".yaml"},
				ValuesFiles:    []string{valuesFile},
				SetValues:      state.setValues,
				IgnoredLines: []string{
					`\s+checksum/.+?:\s+.*`,
				},
			})
		}
	}
}

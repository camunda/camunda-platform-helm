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

	"github.com/stretchr/testify/require"
)

func (s *ConfigmapTemplateTest) TestZonedModeAllowsNumberedRegionSettingsAfterMigration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestMigrationRequiresThePreviousNumberedRegionSettings",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "region-a",
				"orchestration.partitioning.zones[0].name":             "region-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.partitioning.keepUnzonedBrokers":        "true",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "requires both orchestration.partitioning.regions and orchestration.partitioning.regionId",
			},
		},
		{
			Name: "TestMigrationRejectsMissingRegionId",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "region-a",
				"orchestration.partitioning.zones[0].name":             "region-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.partitioning.keepUnzonedBrokers":        "true",
				"orchestration.partitioning.regions":                   "2",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "requires both orchestration.partitioning.regions and orchestration.partitioning.regionId",
			},
		},
		{
			Name: "TestMigrationRejectsEmptyRegionId",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "region-a",
				"orchestration.partitioning.zones[0].name":             "region-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.partitioning.keepUnzonedBrokers":        "true",
				"orchestration.partitioning.regions":                   "2",
				"orchestration.partitioning.regionId":                  "",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "requires both orchestration.partitioning.regions and orchestration.partitioning.regionId",
			},
		},
		{
			Name: "TestZonedModeRejectsRetentionInNumberedMode",
			Values: map[string]string{
				"orchestration.partitioning.keepUnzonedBrokers": "true",
				"orchestration.profiles.broker":                 "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.partitioning.keepUnzonedBrokers requires orchestration.partitioning.scheme=zone-aware",
			},
		},
		{
			Name:                    "TestMigrationAcceptsRegionIdZero",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=4"},
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "region-a",
				"orchestration.partitioning.zones[0].name":             "region-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.partitioning.keepUnzonedBrokers":        "true",
				"orchestration.partitioning.regions":                   "2",
				"orchestration.partitioning.regionId":                  "0",
				"orchestration.profiles.broker":                        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "TestRetainedNumberedRegionsMustBeClearedOnceRetentionIsOff",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "region-a",
				"orchestration.partitioning.zones[0].name":             "region-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.partitioning.keepUnzonedBrokers":        "false",
				"orchestration.partitioning.regions":                   "2",
				"orchestration.partitioning.regionId":                  "1",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.partitioning.regions and orchestration.partitioning.regionId cannot be used with the zone-aware scheme",
			},
		},
		{
			Name:                    "TestZonedModeRejectsInvalidZoneNames",
			RenderTemplateExtraArgs: []string{"--skip-schema-validation"},
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "region_A",
				"orchestration.partitioning.zones[0].name":             "region_A",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "must be an RFC 1123 label.",
			},
		},
		{
			Name: "TestZonedModeAcceptsZoneNamesThatFitResourceNames",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      strings.Repeat("a", 32),
				"orchestration.partitioning.zones[0].name":             strings.Repeat("a", 32),
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.profiles.broker":                        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "TestSchemaRejectsZoneNamesLongerThanResourceNameLimit",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      strings.Repeat("a", 33),
				"orchestration.partitioning.zones[0].name":             strings.Repeat("a", 33),
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "maxLength: got 33, want 32",
			},
		},
		{
			Name:                    "TestZonedModeRejectsZoneNamesThatCannotRetainAResourcePrefix",
			RenderTemplateExtraArgs: []string{"--skip-schema-validation"},
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      strings.Repeat("a", 33),
				"orchestration.partitioning.zones[0].name":             strings.Repeat("a", 33),
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "must be no longer than 32 characters",
			},
		},
		{
			Name: "TestZonedModeRejectsMoreThan999BrokersPerZone",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "zone-a",
				"orchestration.partitioning.zones[0].name":             "zone-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "1000",
				"orchestration.partitioning.zones[0].numberOfReplicas": "1",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.profiles.broker":                        "true",
			},
			Expected: map[string]string{
				"ERROR": "cannot configure more than 999 brokers",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestMigrationContactPoints() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestMigrationUsesOnlyLocalBrokerContactPoints",
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "zone-a",
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
				"orchestration.partitioning.keepUnzonedBrokers":        "true",
				"orchestration.partitioning.regions":                   "3",
				"orchestration.partitioning.regionId":                  "1",
				"orchestration.profiles.broker":                        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "camunda-platform-test-zeebe-0.camunda-platform-test-zeebe:26502")
				require.NotContains(t, output, "camunda-platform-test-zeebe-1.camunda-platform-test-zeebe:26502")
				require.NotContains(t, output, "camunda-platform-test-zeebe-zone-b")
				require.NotContains(t, output, "camunda-platform-test-zeebe-zone-c")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

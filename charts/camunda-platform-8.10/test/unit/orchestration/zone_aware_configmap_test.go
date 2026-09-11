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

func (s *ConfigmapTemplateTest) TestZonedConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainZoneAwareConfiguration",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "3",
				"orchestration.multiregion.zones[1].numberOfReplicas": "3",
				"orchestration.multiregion.zones[1].priority":         "50",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "size: \"5\"")
				require.Contains(t, output, "replication-factor: \"5\"")
				require.Contains(t, output, "scheme: ZONE_AWARE")
				require.Contains(t, output, "name: \"region-a\"")
				require.Contains(t, output, "name: \"region-b\"")
				require.Contains(t, output, "VALUES_ORCHESTRATION_NODE_ID:-${K8S_NAME##*-}")
				require.Contains(t, output, "node-id: \"${VALUES_ORCHESTRATION_NODE_ID:}\"")
				require.NotContains(t, output, "initial-contact-points:")
			},
		},
		{
			Name: "TestZonedNodeIdIsTheIndexInsideTheZone",
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
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// Brokers are addressed as "<zone>_<node-id>", so region-b's three
				// Pods are region-b_0, region-b_1 and region-b_2 whatever region-a
				// declares before it. No cluster-wide offset applies.
				require.Contains(t, output, "VALUES_ORCHESTRATION_NODE_ID:-${K8S_NAME##*-}")
				require.NotContains(t, output, "${K8S_NAME##*-} +")
				require.NotContains(t, output, "${K8S_NAME##*-} *")
				require.Contains(t, output, "node-id: \"${VALUES_ORCHESTRATION_NODE_ID:}\"")
				require.Contains(t, output, "size: \"5\"")
			},
		},
		{
			Name: "TestSingleZoneStillRendersItsInitialContactPoints",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// A single-zone topology can use the generated in-cluster addresses.
				require.Contains(t, output, "initial-contact-points:")
				require.Contains(t, output, "camunda-platform-test-zeebe-region-a-0.camunda-platform-test-zeebe-region-a:26502")
				require.Contains(t, output, "camunda-platform-test-zeebe-region-a-1.camunda-platform-test-zeebe-region-a:26502")
				// Two brokers, so two contact points. The count comes from the zone
				// list, not from `orchestration.clusterSize`, which is still on its
				// default of three and would have produced a third.
				require.NotContains(t, output, "camunda-platform-test-zeebe-region-a-2.camunda-platform-test-zeebe-region-a:26502")
				require.Contains(t, output, "size: \"2\"")
			},
		},
		{
			Name: "TestZonedModeDoesNotEnableLegacyElasticsearchExporter",
			Values: map[string]string{
				"orchestration.multiregion.mode":                                "zoned",
				"orchestration.multiregion.zone":                                "region-a",
				"orchestration.multiregion.zones[0].name":                       "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[0].numberOfReplicas":           "2",
				"orchestration.multiregion.zones[0].priority":                   "100",
				"orchestration.multiregion.zones[1].name":                       "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[1].numberOfReplicas":           "1",
				"orchestration.multiregion.zones[1].priority":                   "50",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter")
			},
		},
		{
			Name: "TestZonedModeDoesNotEnableLegacyOpenSearchExporter",
			Values: map[string]string{
				"orchestration.multiregion.mode":                                "zoned",
				"orchestration.multiregion.zone":                                "region-a",
				"orchestration.multiregion.zones[0].name":                       "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[0].numberOfReplicas":           "2",
				"orchestration.multiregion.zones[0].priority":                   "100",
				"orchestration.multiregion.zones[1].name":                       "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[1].numberOfReplicas":           "1",
				"orchestration.multiregion.zones[1].priority":                   "50",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                     "true",
				"optimize.database.opensearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "io.camunda.zeebe.exporter.opensearch.OpensearchExporter")
			},
		},
		{
			// A single zone is one cluster, like a single region: it skews leaders
			// inside a region rather than spreading across them, so it keeps the
			// exporter that a genuinely spread cluster has to give up.
			Name: "TestSingleZoneZonedModeKeepsTheElasticsearchExporter",
			Values: map[string]string{
				"orchestration.multiregion.mode":                                "zoned",
				"orchestration.multiregion.zone":                                "region-a",
				"orchestration.multiregion.zones[0].name":                       "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":            "2",
				"orchestration.multiregion.zones[0].numberOfReplicas":           "2",
				"orchestration.multiregion.zones[0].priority":                   "100",
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter")
			},
		},
		{
			// Control for the two cases above: same inputs, numbered mode. Without it
			// they pass whether or not the zoned guard exists, since the exporter is
			// also absent when Optimize does not ask for that database.
			Name: "TestNumberedSingleRegionStillEnablesTheElasticsearchExporter",
			Values: map[string]string{
				"orchestration.exporters.rdbms.enabled":                         "true",
				"orchestration.data.secondaryStorage.rdbms.url":                 "jdbc:postgresql://localhost:5432/camunda",
				"orchestration.data.secondaryStorage.rdbms.username":            "camunda",
				"orchestration.data.secondaryStorage.rdbms.secret.inlineSecret": "my-password",
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "io.camunda.zeebe.exporter.ElasticsearchExporter")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestZonedModeAllowsNumberedRegionSettingsAfterMigration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestMigrationRequiresThePreviousNumberedRegionSettings",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.keepUnzonedBrokers":        "true",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "requires both orchestration.multiregion.regions and orchestration.multiregion.regionId",
			},
		},
		{
			Name: "TestMigrationRejectsMissingRegionId",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.keepUnzonedBrokers":        "true",
				"orchestration.multiregion.regions":                   "2",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "requires both orchestration.multiregion.regions and orchestration.multiregion.regionId",
			},
		},
		{
			Name: "TestMigrationRejectsEmptyRegionId",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.keepUnzonedBrokers":        "true",
				"orchestration.multiregion.regions":                   "2",
				"orchestration.multiregion.regionId":                  "",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "requires both orchestration.multiregion.regions and orchestration.multiregion.regionId",
			},
		},
		{
			Name: "TestZonedModeRejectsRetentionInNumberedMode",
			Values: map[string]string{
				"orchestration.multiregion.keepUnzonedBrokers": "true",
				"orchestration.profiles.broker":                "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.multiregion.keepUnzonedBrokers requires orchestration.multiregion.mode=zoned",
			},
		},
		{
			Name: "TestMigrationAcceptsRegionIdZero",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.keepUnzonedBrokers":        "true",
				"orchestration.multiregion.regions":                   "2",
				"orchestration.multiregion.regionId":                  "0",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "TestZonedModeAllowsRetainedNumberedRegionsToBeRemovedWithKeepUnzonedBrokers",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.keepUnzonedBrokers":        "false",
				"orchestration.multiregion.regions":                   "2",
				"orchestration.multiregion.regionId":                  "1",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name:                    "TestZonedModeRejectsAClusterSizeItDerives",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=6"},
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-b",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[1].numberOfReplicas": "1",
				"orchestration.multiregion.zones[1].priority":         "50",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.clusterSize is 6 but orchestration.multiregion.zones sums to 4 brokers",
			},
		},
		{
			Name:                    "TestZonedModeRejectsAReplicationFactorItDerives",
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.replicationFactor=4"},
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.replicationFactor is 4 but orchestration.multiregion.zones sums to 2 replicas",
			},
		},
		{
			Name: "TestZonesWithoutZonedModeAreRejected",
			Values: map[string]string{
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "require orchestration.multiregion.mode=zoned",
			},
		},
		{
			Name:                    "TestZonedModeRejectsInvalidZoneNames",
			RenderTemplateExtraArgs: []string{"--skip-schema-validation"},
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region_A",
				"orchestration.multiregion.zones[0].name":             "region_A",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "must be an RFC 1123 label.",
			},
		},
		{
			Name: "TestZonedModeAcceptsZoneNamesThatFitResourceNames",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      strings.Repeat("a", 32),
				"orchestration.multiregion.zones[0].name":             strings.Repeat("a", 32),
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "TestSchemaRejectsZoneNamesLongerThanResourceNameLimit",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      strings.Repeat("a", 33),
				"orchestration.multiregion.zones[0].name":             strings.Repeat("a", 33),
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "maxLength: got 33, want 32",
			},
		},
		{
			Name:                    "TestZonedModeRejectsZoneNamesThatCannotRetainAResourcePrefix",
			RenderTemplateExtraArgs: []string{"--skip-schema-validation"},
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      strings.Repeat("a", 33),
				"orchestration.multiregion.zones[0].name":             strings.Repeat("a", 33),
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "must be no longer than 32 characters",
			},
		},
		{
			Name: "TestZonedModeRejectsMoreThan999BrokersPerZone",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "zone-a",
				"orchestration.multiregion.zones[0].name":             "zone-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1000",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "cannot configure more than 999 brokers",
			},
		},
		{
			Name: "TestZonedModeRejectsDuplicateZoneNames",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.multiregion.zones[1].name":             "region-a",
				"orchestration.multiregion.zones[1].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[1].numberOfReplicas": "1",
				"orchestration.multiregion.zones[1].priority":         "50",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "declares \"region-a\" twice",
			},
		},
		{
			Name: "TestZonedModeRejectsZeroPriority",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "1",
				"orchestration.multiregion.zones[0].priority":         "0",
				"orchestration.profiles.broker":                       "true",
			},
			Verifier: func(t *testing.T, _ string, err error) {
				require.ErrorContains(t, err, "/orchestration/multiregion/zones/0/priority': minimum: got 0, want 1")
			},
		},
		{
			Name: "TestZonedModeRejectsMoreReplicasThanBrokers",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-a",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
				"orchestration.multiregion.zones[0].numberOfReplicas": "3",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "asks for 3 replicas on 1 brokers",
			},
		},
		{
			Name: "TestZonedModeRejectsAnUndeclaredZone",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "region-c",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.multiregion.zone \"region-c\" is not declared in orchestration.multiregion.zones",
			},
		},
		{
			// A release renders exactly one zone, so the zone it belongs to is not
			// optional: without it the broker count and the node IDs are undefined.
			Name: "TestZonedModeRequiresTheZone",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zones[0].name":             "region-a",
				"orchestration.multiregion.zones[0].numberOfBrokers":  "2",
				"orchestration.multiregion.zones[0].numberOfReplicas": "2",
				"orchestration.multiregion.zones[0].priority":         "100",
				"orchestration.profiles.broker":                       "true",
			},
			Expected: map[string]string{
				"ERROR": "orchestration.multiregion.zone must name the zone this release is deployed to",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestMigrationContactPoints() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestMigrationUsesOnlyRetainedLocalBrokers",
			Values: map[string]string{
				"orchestration.multiregion.mode":                      "zoned",
				"orchestration.multiregion.zone":                      "zone-a",
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
				"orchestration.multiregion.keepUnzonedBrokers":        "true",
				"orchestration.multiregion.regions":                   "3",
				"orchestration.multiregion.regionId":                  "1",
				"orchestration.profiles.broker":                       "true",
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

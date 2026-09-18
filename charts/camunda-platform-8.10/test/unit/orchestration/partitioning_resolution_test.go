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
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// camundaPlatform.partitioning resolves the scheme once and derives clusterSize,
// replicationFactor, localReplicas and spansFailureDomains from it. Consumers read those
// fields instead of branching on the scheme themselves, so a mistake in the resolver is a
// mistake everywhere at once: the rendered cluster size, the replication factor, the
// StatefulSet replica count, the generated bootstrap list and the deprecation warnings all
// move together.
//
// These cases pin the derivation at its observable surface, the rendered configuration and
// the StatefulSet, across both schemes and both the current and the deprecated values block.
type PartitioningResolutionTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestPartitioningResolution(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &PartitioningResolutionTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{},
	})
}

// Sizing is summed from the zone list under the zone-aware scheme and read from the values
// keys otherwise. The asymmetric case matters most: equal zones would pass even if the
// resolver totalled the wrong field.
func (s *PartitioningResolutionTest) TestDerivedSizing() {
	testCases := []testhelpers.TestCase{
		{
			Name: "ZoneAwareSumsAsymmetricZones",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":             "elasticsearch",
				"orchestration.profiles.broker":                        "true",
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "paris",
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
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// brokers 4+2+1, replicas 3+2+1, summed from different fields
				require.Contains(t, output, "size: \"7\"")
				require.Contains(t, output, "replication-factor: \"6\"")
			},
		},
		{
			Name: "RoundRobinReadsTheValuesKeys",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.profiles.broker":            "true",
				"orchestration.partitioning.regions":       "2",
				"orchestration.partitioning.regionId":      "1",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "orchestration.clusterSize=6",
				"--set-string", "orchestration.replicationFactor=5",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "size: \"6\"")
				require.Contains(t, output, "replication-factor: \"5\"")
				require.NotContains(t, output, "scheme: ZONE_AWARE")
			},
		},
		{
			// Precedence is whole-block, never per field. Spelling out a value that happens to
			// equal the chart default leaves the orchestration block unconfigured, so the
			// deprecated pair still supplies the numbering. A per-field merge would read
			// regions 1 here and render the node id as * 1 + 0.
			Name: "ExplicitDefaultDoesNotStealPrecedenceFromTheDeprecatedBlock",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.profiles.broker":            "true",
				"orchestration.partitioning.scheme":        "round-robin",
				"global.multiregion.regions":               "3",
				"global.multiregion.regionId":              "2",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "${K8S_NAME##*-} * 3 + 2")
				require.NotContains(t, output, "${K8S_NAME##*-} * 1 + 0")
			},
		},
		{
			Name: "DeprecatedGlobalBlockStillFeedsTheDerivation",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.profiles.broker":            "true",
				"global.multiregion.regions":               "3",
				"global.multiregion.regionId":              "2",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "orchestration.clusterSize=6"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "size: \"6\"")
				// node id still interleaves from the deprecated pair
				require.Contains(t, output, "${K8S_NAME##*-} * 3 + 2")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// localReplicas is the one derived value that depends on which zone this release deploys to,
// so it is the one a naive implementation gets wrong by taking the first entry.
func (s *PartitioningResolutionTest) TestLocalZoneSelection() {
	zones := map[string]string{
		"orchestration.data.secondaryStorage.type":             "elasticsearch",
		"orchestration.partitioning.scheme":                    "zone-aware",
		"orchestration.partitioning.zones[0].name":             "first",
		"orchestration.partitioning.zones[0].numberOfBrokers":  "2",
		"orchestration.partitioning.zones[0].numberOfReplicas": "2",
		"orchestration.partitioning.zones[0].priority":         "2",
		"orchestration.partitioning.zones[1].name":             "second",
		"orchestration.partitioning.zones[1].numberOfBrokers":  "1",
		"orchestration.partitioning.zones[1].numberOfReplicas": "1",
		"orchestration.partitioning.zones[1].priority":         "1",
	}
	withZone := func(local string) map[string]string {
		out := map[string]string{"orchestration.partitioning.zone": local}
		for k, v := range zones {
			out[k] = v
		}
		return out
	}

	testCases := []testhelpers.TestCase{
		{
			// the local zone is the first entry, so replicas is its broker count
			Name:   "FirstZoneIsLocal",
			Values: withZone("first"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "replicas: 2")
			},
		},
		{
			// the local zone is the second entry; taking zones[0] would render 2 here
			Name:   "SecondZoneIsLocal",
			Values: withZone("second"),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "replicas: 1")
				require.NotContains(t, output, "replicas: 2")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release,
		s.namespace, []string{"templates/orchestration/statefulset.yaml"}, testCases)
}

// spansFailureDomains gates the generated bootstrap list: the chart addresses one headless
// service itself, and refuses to guess beyond that. Both schemes have a boundary at one.
func (s *PartitioningResolutionTest) TestSpansFailureDomainsBoundary() {
	const manualContactPoints = "Multi-region deployments: initial-contact-points must be provided manually"

	testCases := []testhelpers.TestCase{
		{
			Name: "SingleZoneGeneratesContactPoints",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":             "elasticsearch",
				"orchestration.profiles.broker":                        "true",
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "only",
				"orchestration.partitioning.zones[0].name":             "only",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "3",
				"orchestration.partitioning.zones[0].numberOfReplicas": "3",
				"orchestration.partitioning.zones[0].priority":         "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "initial-contact-points:")
				require.NotContains(t, output, manualContactPoints)
			},
		},
		{
			Name: "TwoZonesSuppressThem",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":             "elasticsearch",
				"orchestration.profiles.broker":                        "true",
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "a",
				"orchestration.partitioning.zones[0].name":             "a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "2",
				"orchestration.partitioning.zones[0].numberOfReplicas": "2",
				"orchestration.partitioning.zones[0].priority":         "2",
				"orchestration.partitioning.zones[1].name":             "b",
				"orchestration.partitioning.zones[1].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[1].numberOfReplicas": "1",
				"orchestration.partitioning.zones[1].priority":         "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "initial-contact-points:")
				require.Contains(t, output, manualContactPoints)
			},
		},
		{
			Name: "SingleRegionGeneratesContactPoints",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.profiles.broker":            "true",
				"orchestration.partitioning.regions":       "1",
				"orchestration.partitioning.regionId":      "0",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "initial-contact-points:")
				require.NotContains(t, output, manualContactPoints)
			},
		},
		{
			Name: "TwoRegionsSuppressThem",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.profiles.broker":            "true",
				"orchestration.partitioning.regions":       "2",
				"orchestration.partitioning.regionId":      "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "initial-contact-points:")
				require.Contains(t, output, manualContactPoints)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// qualifiedAdvertisedHost is derived alongside spansFailureDomains and deliberately disagrees
// with it on one input: a single-zone zone-aware release advertises the fully qualified name
// while the chart still generates the bootstrap list. Collapsing the two into one predicate
// would render the short host there, so this pins the divergence and the single-region case
// that anchors the other end of the predicate.
func (s *PartitioningResolutionTest) TestQualifiedAdvertisedHostDivergesFromSpansFailureDomains() {
	const qualified = "advertisedHost: \"${K8S_NAME}.${K8S_SERVICE_NAME}.${K8S_NAMESPACE}.svc\""
	const short = "advertisedHost: \"${K8S_NAME}.${K8S_SERVICE_NAME}\""

	testCases := []testhelpers.TestCase{
		{
			Name: "SingleZoneQualifiesTheHostAndStillGeneratesContactPoints",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":             "elasticsearch",
				"orchestration.profiles.broker":                        "true",
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "only",
				"orchestration.partitioning.zones[0].name":             "only",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "3",
				"orchestration.partitioning.zones[0].numberOfReplicas": "3",
				"orchestration.partitioning.zones[0].priority":         "1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, qualified)
				require.Contains(t, output, "initial-contact-points:")
			},
		},
		{
			// the alternate contract: round-robin at one region is the only case that keeps the
			// short host, so a predicate stuck on true would show up here
			Name: "SingleRegionKeepsTheShortHost",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.profiles.broker":            "true",
				"orchestration.partitioning.regions":       "1",
				"orchestration.partitioning.regionId":      "0",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, short)
				require.NotContains(t, output, qualified)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// The resolved dict is round-tripped through JSON, so every number comes back a float64 and
// Go prints those with %g. configmap.yaml puts regions and regionId straight into shell
// arithmetic, where the exponent form Go switches to at 1e6 is not a number.
func (s *PartitioningResolutionTest) TestRegionCountsRenderAsDecimalIntegers() {
	testCases := []testhelpers.TestCase{
		{
			Name: "LargeRegionCountKeepsDecimalNotation",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type": "elasticsearch",
				"orchestration.profiles.broker":            "true",
				"orchestration.partitioning.regions":       "1000000",
				"orchestration.partitioning.regionId":      "999999",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "${K8S_NAME##*-} * 1000000 + 999999")
				require.NotContains(t, output, "e+06")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

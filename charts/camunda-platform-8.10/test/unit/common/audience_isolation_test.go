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
	"camunda-platform/test/unit/testhelpers"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	goyaml "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Sentinel audiences. Each is unique, so finding one inside a component's rendered
// configuration identifies which component's token that configuration accepts.
const (
	probeIdentity      = "probe-identity-api"
	probeOrchestration = "probe-orchestration-api"
	probeOptimize      = "probe-optimize-api"
	probeModelerClient = "probe-web-modeler-client-api"
	probeModelerPublic = "probe-web-modeler-public-api"
)

var audienceOwner = map[string]string{
	probeIdentity:      "identity",
	probeOrchestration: "orchestration",
	probeOptimize:      "optimize",
	probeModelerClient: "web-modeler",
	probeModelerPublic: "web-modeler",
}

// knownAudienceLeaks records components that accept an audience owned by another
// component. Each entry is a defect, not a design decision: a token minted for the
// owning component is accepted by every component listed here.
//
// Connectors is deliberately absent. It authenticates against the Orchestration
// Cluster as a client and shares that client's audience by design, and it has no
// audience of its own to leak.
//
// Delete an entry when the corresponding chart fix lands. A stale entry fails the
// test, so the allowlist cannot outlive the defect it records.
//
// Tracked in https://github.com/camunda/camunda-platform-helm/issues/7186
var knownAudienceLeaks = map[string][]string{
	probeModelerClient: {"optimize", "orchestration"},
}

type AudienceIsolationTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestAudienceIsolationTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &AudienceIsolationTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{
			"templates/optimize/configmap.yaml",
			"templates/orchestration/configmap.yaml",
			"templates/web-modeler/configmap-restapi.yaml",
		},
	})
}

// TestComponentsDoNotShareAudiences renders every component with a distinct audience
// and asserts none of them accepts an audience owned by another component. The
// audience is the only boundary between components that trust the same issuer, so a
// shared value means a token minted for one component is accepted by the other.
//
// Configuring distinct audiences is not sufficient on its own: a template can append
// another component's audience regardless of what the operator sets, which is why
// this asserts on rendered output rather than on values.
func (s *AudienceIsolationTest) TestComponentsDoNotShareAudiences() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestNoComponentAcceptsAnotherComponentsAudience",
			Values: oidcValuesWithDistinctAudiences(),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				accepted := acceptedAudiences(t, output)
				s.Require().Len(accepted, 3, "expected optimize, orchestration, and web-modeler configuration to render")

				observedLeaks := map[string][]string{}

				for _, component := range sortedKeys(accepted) {
					for _, audience := range accepted[component] {
						owner := audienceOwner[audience]
						if owner == component {
							continue
						}

						observedLeaks[audience] = append(observedLeaks[audience], component)

						s.Require().Containsf(knownAudienceLeaks[audience], component,
							"%s accepts %q, which belongs to %s, so a token minted for %s is accepted by %s. "+
								"Give every component its own audience, or record the leak in knownAudienceLeaks "+
								"with a tracking issue.",
							component, audience, owner, owner, component)
					}
				}

				// Ratchet: a recorded leak that no longer reproduces means the chart was
				// fixed and the allowlist entry has to go, otherwise it silently keeps
				// permitting a collision that would now be caught.
				for audience, components := range knownAudienceLeaks {
					for _, component := range components {
						s.Require().Containsf(observedLeaks[audience], component,
							"knownAudienceLeaks says %s accepts %q, but it no longer does. "+
								"Remove the entry so the leak cannot reappear unnoticed.",
							component, audience)
					}
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// oidcValuesWithDistinctAudiences configures the chart the way the documentation tells
// an operator to: one identity provider, and a different audience for every component.
func oidcValuesWithDistinctAudiences() map[string]string {
	return map[string]string{
		"orchestration.data.secondaryStorage.type":            "elasticsearch",
		"optimize.enabled":                                    "true",
		"webModeler.enabled":                                  "true",
		"identity.enabled":                                    "true",
		"webModeler.restapi.mail.fromAddress":                 "noreply@example.com",
		"global.security.authentication.method":               "oidc",
		"global.identity.auth.enabled":                        "true",
		"global.identity.auth.issuer":                         "https://idp.example.com",
		"global.identity.auth.issuerBackendUrl":               "https://idp.example.com",
		"global.identity.auth.tokenUrl":                       "https://idp.example.com/token",
		"global.identity.auth.jwksUrl":                        "https://idp.example.com/jwks",
		"global.identity.auth.identity.audience":              probeIdentity,
		"orchestration.security.authentication.oidc.audience": probeOrchestration,
		"global.identity.auth.optimize.audience":              probeOptimize,
		"global.identity.auth.webModeler.clientApiAudience":   probeModelerClient,
		"global.identity.auth.webModeler.publicApiAudience":   probeModelerPublic,
	}
}

// acceptedAudiences maps each rendered component to the probe audiences its
// configuration accepts.
func acceptedAudiences(t *testing.T, output string) map[string][]string {
	accepted := map[string][]string{}
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)

	for {
		var object unstructured.Unstructured
		err := decoder.Decode(&object)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		if object.GetKind() != "ConfigMap" {
			continue
		}

		component := componentForConfigMap(object.GetName())
		if component == "" {
			continue
		}

		data, _, err := unstructured.NestedStringMap(object.Object, "data")
		require.NoError(t, err)

		found := []string{}
		for _, document := range data {
			found = append(found, probeAudiencesIn(document)...)
		}
		accepted[component] = dedupe(found)
	}

	return accepted
}

func componentForConfigMap(name string) string {
	switch {
	case strings.HasSuffix(name, "-optimize-configuration"):
		return "optimize"
	case strings.HasSuffix(name, "-zeebe-configuration"):
		return "orchestration"
	case strings.HasSuffix(name, "-web-modeler-restapi-configuration"):
		return "web-modeler"
	default:
		return ""
	}
}

// probeAudiencesIn walks an entire rendered configuration document rather than reading
// fixed paths, so a template that starts emitting an audience under a new key is still
// caught. The probe values are unique, so matching on them cannot produce a false hit.
func probeAudiencesIn(document string) []string {
	var parsed any
	if err := goyaml.Unmarshal([]byte(document), &parsed); err != nil {
		// Not every ConfigMap entry is YAML; non-YAML entries carry no audiences.
		return nil
	}

	found := []string{}

	var walk func(node any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		case string:
			if _, isProbe := audienceOwner[value]; isProbe {
				found = append(found, value)
			}
		}
	}
	walk(parsed)

	return found
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func sortedKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

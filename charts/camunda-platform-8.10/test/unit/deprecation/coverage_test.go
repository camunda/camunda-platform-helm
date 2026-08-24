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

package deprecation

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	_ "camunda-platform/test/unit/utils" // registers the -update-golden flag so this package tolerates the golden-update test runner

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	currentValuesPath  = "../../../values.yaml"
	previousValuesPath = "../../../../camunda-platform-8.9/values.yaml"
	constraintsTplPath = "../../../templates/common/constraints.tpl"
)

var freeFormMapParents = map[string]struct{}{
	"annotations":        {},
	"labels":             {},
	"podAnnotations":     {},
	"podLabels":          {},
	"matchLabels":        {},
	"nodeSelector":       {},
	"tolerations":        {},
	"affinity":           {},
	"env":                {},
	"envFrom":            {},
	"extraConfiguration": {},
	"extraEnvVars":       {},
	"configuration":      {},
	"extraVolumes":       {},
	"extraVolumeMounts":  {},
	"initContainers":     {},
	"sidecars":           {},
	"extraManifests":     {},
}

// bespokeWarnings map removed-key sets onto hand-written warning blocks in
// constraints.tpl. Those blocks carry no "oldName" for the regex to find, so
// each entry names a marker substring that must still be present in the
// executable template body for the keys it covers to count as covered.
var bespokeWarnings = []struct {
	name   string
	marker string
	covers func(key string) bool
}{
	{
		name:   "console.enabled deprecation",
		marker: `\"console.enabled\" is deprecated and will be removed in chart v16 (Camunda 8.11).`,
		covers: func(key string) bool { return key == "console.enabled" },
	},
	{
		name:   "consolidated console.* warning",
		marker: `console.* configuration keys have no effect in 8.10`,
		covers: func(key string) bool {
			return key != "console.enabled" && strings.HasPrefix(key, "console.")
		},
	},
	{
		name:   "global.identity.auth.console.* removal warning",
		marker: `\"global.identity.auth.console.*\" is no longer used in Camunda 8.10.`,
		covers: func(key string) bool {
			return key == "global.identity.auth.console" ||
				strings.HasPrefix(key, "global.identity.auth.console.")
		},
	},
	{
		name:   "webModeler.enabled deprecation",
		marker: `\"webModeler.enabled\" is deprecated and will be removed in a future version.`,
		covers: func(key string) bool { return key == "webModeler.enabled" },
	},
}

var allowlist = map[string]string{
	"global.security.allowInsecureImages": "Bitnami subcharts dropped",
}

var oldNamePattern = regexp.MustCompile(`"oldName"\s+"([^"]+)"`)

// helmCommentPattern matches Helm comment blocks {{/* ... */}} (including the
// {{- -}} whitespace-chomp variants). Non-greedy + dot-matches-newline so
// multi-line comment blocks are removed whole.
var helmCommentPattern = regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)

func TestDeprecationKeyCoverage89To810(t *testing.T) {
	t.Parallel()

	prevKeys, err := flattenValuesFile(previousValuesPath)
	require.NoError(t, err)

	currKeys, err := flattenValuesFile(currentValuesPath)
	require.NoError(t, err)

	removed := setDifference(prevKeys, currKeys)

	constraintsBytes, err := os.ReadFile(constraintsTplPath)
	require.NoError(t, err)

	executable := stripHelmComments(string(constraintsBytes))
	covered := parseCoveredKeys(executable)

	var uncovered []string
	for key := range removed {
		if _, ok := allowlist[key]; ok {
			continue
		}
		// A prev key with a descendant in curr expanded (empty map grew children,
		// e.g. global.identity.keycloak.url) rather than being removed.
		if hasDescendantIn(key, currKeys) {
			continue
		}
		if isCovered(key, covered, executable) {
			continue
		}
		uncovered = append(uncovered, key)
	}

	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		for _, key := range uncovered {
			t.Errorf("removed key %q has no deprecation coverage: add keyDeprecated/keyRemoved in constraints.tpl or allowlist it in coverage_test.go", key)
		}
	}
}

// TestHasDescendantInKeepsTeeth pins the disambiguation logic: an expanded key
// (empty {} that grew children) is NOT flagged, while a truly-removed empty-map
// key (no descendant in curr) still is.
func TestHasDescendantInKeepsTeeth(t *testing.T) {
	t.Parallel()

	curr := map[string]struct{}{
		"global.identity.keycloak.url.protocol": {},
		"global.identity.keycloak.url.host":     {},
	}
	// Expanded: {} in prev, now has children in curr -> not removed.
	require.True(t, hasDescendantIn("global.identity.keycloak.url", curr))
	// Truly removed empty-map key: no descendant in curr -> still flagged.
	require.False(t, hasDescendantIn("global.some.removedEmptyMap", curr))
}

// TestParseCoveredKeysIgnoresComments pins that a live keyDeprecated invocation
// counts as coverage while a commented-out one inside {{/* ... */}} does not.
func TestParseCoveredKeysIgnoresComments(t *testing.T) {
	t.Parallel()

	input := `
{{ include "camundaPlatform.keyDeprecated" (dict
  "condition" true
  "oldName" "foo.live"
  "migration" "bar") }}
{{/*
{{ include "camundaPlatform.keyDeprecated" (dict
  "condition" true
  "oldName" "foo.commented"
  "migration" "bar") }}
*/}}
`
	covered := parseCoveredKeys(stripHelmComments(input))
	_, liveOK := covered["foo.live"]
	require.True(t, liveOK, "live oldName should be covered")
	_, commentedOK := covered["foo.commented"]
	require.False(t, commentedOK, "commented-out oldName must not be covered")
}

// TestBespokeWarningMarkersPresent fails if a hand-written warning block that
// bespokeWarnings relies on is removed or reworded, so its keys cannot silently
// lose coverage.
func TestBespokeWarningMarkersPresent(t *testing.T) {
	t.Parallel()

	constraintsBytes, err := os.ReadFile(constraintsTplPath)
	require.NoError(t, err)

	executable := stripHelmComments(string(constraintsBytes))
	for _, warning := range bespokeWarnings {
		// Asserted via strings.Contains rather than require.Contains to keep the
		// whole constraints.tpl body out of the failure message.
		require.True(t, strings.Contains(executable, warning.marker),
			"constraints.tpl no longer contains the %s marker %q; update the warning or bespokeWarnings",
			warning.name, warning.marker)
	}
}

// TestBespokeWarningCoverageRequiresMarker pins that bespoke coverage is tied to
// the warning text: with the marker absent the key is uncovered, and
// "console.enabled" is never covered by the consolidated console.* warning.
func TestBespokeWarningCoverageRequiresMarker(t *testing.T) {
	t.Parallel()

	covered := map[string]struct{}{}
	consolidated := `"DEPRECATION: console.* configuration keys have no effect in 8.10 — Console has been consolidated into Camunda Hub."`

	require.True(t, isCovered("console.nodeEnv", covered, consolidated))
	require.False(t, isCovered("console.nodeEnv", covered, ""))
	require.False(t, isCovered("console.enabled", covered, consolidated))
}

func flattenValuesFile(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	keys := make(map[string]struct{})
	flattenKeys("", root, keys)
	return keys, nil
}

func flattenKeys(prefix string, value any, keys map[string]struct{}) {
	if prefix != "" {
		if _, stop := freeFormMapParents[lastSegment(prefix)]; stop {
			keys[prefix] = struct{}{}
			return
		}
	}

	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			// Record an empty map under its own prefix so it has presence in the key set.
			if prefix != "" {
				keys[prefix] = struct{}{}
			}
			return
		}
		for key, child := range typed {
			childPrefix := key
			if prefix != "" {
				childPrefix = prefix + "." + key
			}
			flattenKeys(childPrefix, child, keys)
		}
	case []any:
		if prefix != "" {
			keys[prefix] = struct{}{}
		}
	default:
		if prefix != "" {
			keys[prefix] = struct{}{}
		}
	}
}

func lastSegment(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// hasDescendantIn reports whether any key in set is a strict descendant of key
// (i.e. begins with key+"."). Used to tell an expanded key from a removed one.
func hasDescendantIn(key string, set map[string]struct{}) bool {
	prefix := key + "."
	for k := range set {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func setDifference(a, b map[string]struct{}) map[string]struct{} {
	diff := make(map[string]struct{})
	for key := range a {
		if _, ok := b[key]; !ok {
			diff[key] = struct{}{}
		}
	}
	return diff
}

// stripHelmComments removes {{/* ... */}} blocks so only the executable
// template body is scanned: an "oldName" "X" or a warning marker sitting inside
// a comment is not executed and must not count as coverage.
func stripHelmComments(constraints string) string {
	return helmCommentPattern.ReplaceAllString(constraints, "")
}

func parseCoveredKeys(executable string) map[string]struct{} {
	covered := make(map[string]struct{})
	for _, match := range oldNamePattern.FindAllStringSubmatch(executable, -1) {
		covered[match[1]] = struct{}{}
	}
	return covered
}

func isCovered(key string, covered map[string]struct{}, executable string) bool {
	for _, warning := range bespokeWarnings {
		if warning.covers(key) && strings.Contains(executable, warning.marker) {
			return true
		}
	}

	if _, ok := covered[key]; ok {
		return true
	}

	parts := strings.Split(key, ".")
	for i := len(parts) - 1; i > 0; i-- {
		ancestor := strings.Join(parts[:i], ".")
		if _, ok := covered[ancestor]; ok {
			return true
		}
	}

	return false
}

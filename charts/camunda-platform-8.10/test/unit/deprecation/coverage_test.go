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

var handWrittenCovered = []string{
	"console.enabled",
	"webModeler.enabled",
}

var allowlist = map[string]string{
	"global.security.allowInsecureImages": "Bitnami subcharts dropped",
}

var oldNamePattern = regexp.MustCompile(`"oldName"\s+"([^"]+)"`)

func TestDeprecationKeyCoverage89To810(t *testing.T) {
	t.Parallel()

	prevKeys, err := flattenValuesFile(previousValuesPath)
	require.NoError(t, err)

	currKeys, err := flattenValuesFile(currentValuesPath)
	require.NoError(t, err)

	removed := setDifference(prevKeys, currKeys)

	constraintsBytes, err := os.ReadFile(constraintsTplPath)
	require.NoError(t, err)

	covered := parseCoveredKeys(string(constraintsBytes))
	for _, key := range handWrittenCovered {
		covered[key] = struct{}{}
	}

	var uncovered []string
	for key := range removed {
		if _, ok := allowlist[key]; ok {
			continue
		}
		// Exact-string setDifference can't distinguish "key removed" from "key's
		// empty map grew children" (e.g. global.identity.keycloak.url was {} in
		// 8.9 and {protocol,host,port} in 8.10). If the prev key has any
		// descendant in curr, it expanded rather than being removed — not a
		// deprecation gap. A truly-removed empty-map key has no curr descendant
		// and is still flagged.
		if hasDescendantIn(key, currKeys) {
			continue
		}
		if isCovered(key, covered) {
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
			// Record the empty map's own prefix so an empty-map key has presence
			// in the key set (otherwise a removed empty-map key escapes coverage).
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

func parseCoveredKeys(constraints string) map[string]struct{} {
	covered := make(map[string]struct{})
	for _, match := range oldNamePattern.FindAllStringSubmatch(constraints, -1) {
		covered[match[1]] = struct{}{}
	}
	return covered
}

func isCovered(key string, covered map[string]struct{}) bool {
	if strings.HasPrefix(key, "console.") {
		return true
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

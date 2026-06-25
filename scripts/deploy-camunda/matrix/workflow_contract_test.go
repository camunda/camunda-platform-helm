// Copyright 2025 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

// privateRegistrySecret is the imagePullSecret that authenticates against
// registry.camunda.cloud, where 3rd-party callers (camunda/camunda,
// camunda/connectors, camunda/identity, the web-modeler repo, …) publish the
// component builds they ask this repo's reusable integration-test workflow to
// deploy.
const privateRegistrySecret = "registry-camunda-cloud"

// TestThirdPartyImageOverrideHasPullSecret guards the deploy-behaviour contract
// the integration-test workflow exposes to 3rd-party callers.
//
// A caller invokes the reusable workflow and overrides one or more component
// images to a registry.camunda.cloud build (the set of overridable components is
// the top-level keys of chart-full-setup/values/base-image-tags.yaml). For that
// override to be pullable, the component must resolve an imagePullSecret that can
// authenticate against the private registry. The Helm helpers resolve a
// component's pull secrets as "component-level image.pullSecrets if set, else
// global.image.pullSecrets" (see templates/common/_helpers.tpl —
// camundaPlatform.imagePullSecrets / subChartImagePullSecrets; component-level
// REPLACES global, it is not merged).
//
// This test replays that resolution against the merged chart-full-setup values
// for every active chart version and fails if any caller-overridable component
// would not get registry-camunda-cloud — i.e. before the failure ever reaches a
// downstream repo as an ImagePullBackOff. It is intentionally driven by
// chart-versions.yaml and base-image-tags.yaml so new versions and newly
// overridable components are covered automatically.
func TestThirdPartyImageOverrideHasPullSecret(t *testing.T) {
	repoRoot := findRepoRoot(t)

	cv, err := LoadChartVersions(repoRoot)
	if err != nil {
		t.Fatalf("LoadChartVersions: %v", err)
	}
	versions := cv.ActiveVersions()
	if len(versions) == 0 {
		t.Fatal("no active chart versions found in charts/chart-versions.yaml")
	}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			scenarios := filepath.Join(repoRoot, "charts", "camunda-platform-"+version,
				"test", "integration", "scenarios")
			valuesDir := filepath.Join(scenarios, "chart-full-setup", "values")

			imageTagsPath := filepath.Join(valuesDir, "base-image-tags.yaml")
			components := topLevelKeys(t, imageTagsPath)
			if len(components) == 0 {
				t.Fatalf("no overridable components found in %s", imageTagsPath)
			}

			// Replay the deploy value merge for the keys that matter here:
			// common pull-secrets (lowest precedence) overlaid by the
			// chart-full-setup base. Maps deep-merge; arrays/scalars from the
			// higher layer win — matching Helm's own merge semantics.
			merged := map[string]any{}
			commonPullSecrets := filepath.Join(scenarios, "common", "values-integration-test-pull-secrets.yaml")
			if doc := loadYAMLIfExists(t, commonPullSecrets); doc != nil {
				merged = deepMerge(merged, doc)
			}
			merged = deepMerge(merged, loadYAML(t, filepath.Join(valuesDir, "base.yaml")))

			globalSecrets := resolveGlobalPullSecrets(merged)

			for _, component := range components {
				resolved := resolveComponentPullSecrets(merged, component, globalSecrets)
				if !contains(resolved, privateRegistrySecret) {
					t.Errorf(
						"component %q is caller-overridable (declared in base-image-tags.yaml) but its resolved "+
							"imagePullSecrets %v do not include %q.\n"+
							"A 3rd-party caller overriding %s.image to a registry.camunda.cloud build would hit "+
							"ImagePullBackOff. Fix by adding %q to global.image.pullSecrets (covers every component) "+
							"or to %s.image.pullSecrets in chart-full-setup/values/base.yaml.",
						component, resolved, privateRegistrySecret, component, privateRegistrySecret, component,
					)
				}
			}
		})
	}
}

// resolveComponentPullSecrets returns the imagePullSecret names a component
// resolves to, in precedence order:
//  1. camundaHub.<component>.image.pullSecrets — the camundaHub-aware components
//     (console, webModeler on 8.10+) resolve via
//     `or .Values.camundaHub.<c>.image.pullSecrets .Values.<c>.image.pullSecrets`
//     so a camundaHub override shadows the legacy path. camundaHub is absent on
//     older versions, in which case this lookup is simply skipped.
//  2. <component>.image.pullSecrets — the legacy/common path used by every other
//     component (optimize, orchestration, identity, connectors, …).
//  3. global.image.pullSecrets — the fallback when the component sets none.
func resolveComponentPullSecrets(values map[string]any, component string, globalSecrets []string) []string {
	if hub, ok := asMap(values["camundaHub"]); ok {
		if comp, ok := asMap(hub[component]); ok {
			if own := pullSecretNames(comp); len(own) > 0 {
				return own
			}
		}
	}
	if comp, ok := asMap(values[component]); ok {
		if own := pullSecretNames(comp); len(own) > 0 {
			return own
		}
	}
	return globalSecrets
}

// resolveGlobalPullSecrets returns global.image.pullSecrets names.
func resolveGlobalPullSecrets(values map[string]any) []string {
	if global, ok := asMap(values["global"]); ok {
		return pullSecretNames(global)
	}
	return nil
}

// pullSecretNames extracts the "name" field of each entry under
// <node>.image.pullSecrets.
func pullSecretNames(node map[string]any) []string {
	image, ok := asMap(node["image"])
	if !ok {
		return nil
	}
	list, ok := image["pullSecrets"].([]any)
	if !ok {
		return nil
	}
	var names []string
	for _, entry := range list {
		if m, ok := asMap(entry); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// topLevelKeys returns the sorted top-level keys of a YAML mapping file.
func topLevelKeys(t *testing.T, path string) []string {
	doc := loadYAML(t, path)
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func loadYAML(t *testing.T, path string) map[string]any {
	doc := loadYAMLIfExists(t, path)
	if doc == nil {
		t.Fatalf("required values file missing: %s", path)
	}
	return doc
}

func loadYAMLIfExists(t *testing.T, path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc
}

// deepMerge overlays src onto dst: nested maps merge recursively, while arrays
// and scalars from src replace dst (Helm's values merge semantics). It does not
// mutate its inputs; the returned map shares nested-map references with dst for
// keys src did not touch, so callers must treat the result as read-only (the
// guard only ever reads it).
func deepMerge(dst, src map[string]any) map[string]any {
	out := make(map[string]any, len(dst))
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		if srcMap, ok := asMap(v); ok {
			if dstMap, ok := asMap(out[k]); ok {
				out[k] = deepMerge(dstMap, srcMap)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// asMap normalizes a YAML-decoded mapping to map[string]any. yaml.v3 always
// decodes mappings as map[string]any; the map[any]any branch is a defensive
// safety net in case the decoder is ever swapped.
func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprintf("%v", k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestResolveComponentPullSecrets_DetectsMissingSecret pins the resolution logic
// the guard relies on, independent of the on-disk values files: a component with
// no own pull secrets inherits global; a component with its own pull secrets
// replaces (not merges) global; and a missing private-registry secret is
// detectable in both shapes.
func TestResolveComponentPullSecrets_DetectsMissingSecret(t *testing.T) {
	globalOK := []string{"index-docker-io", privateRegistrySecret}
	globalBad := []string{"index-docker-io"}

	values := map[string]any{
		"global": map[string]any{
			"image": map[string]any{
				"pullSecrets": []any{
					map[string]any{"name": "index-docker-io"},
					map[string]any{"name": privateRegistrySecret},
				},
			},
		},
		// Inherits global -> covered.
		"orchestration": map[string]any{},
		// Own secrets that include the private registry -> covered.
		"webModeler": map[string]any{
			"image": map[string]any{
				"pullSecrets": []any{
					map[string]any{"name": privateRegistrySecret},
				},
			},
		},
		// Own secrets that DROP the private registry -> must be flagged even
		// though global would have covered it (component replaces global).
		"connectors": map[string]any{
			"image": map[string]any{
				"pullSecrets": []any{
					map[string]any{"name": "index-docker-io"},
				},
			},
		},
		// camundaHub override (console/webModeler, 8.10+) that DROPS the private
		// registry -> must be flagged: camundaHub.<c>.image.pullSecrets shadows
		// both the legacy path and global.
		"console": map[string]any{},
		"camundaHub": map[string]any{
			"console": map[string]any{
				"image": map[string]any{
					"pullSecrets": []any{
						map[string]any{"name": "index-docker-io"},
					},
				},
			},
		},
	}

	tests := []struct {
		component string
		want      bool
	}{
		{"orchestration", true},
		{"webModeler", true},
		{"connectors", false},
		{"console", false},
	}
	for _, tc := range tests {
		got := contains(resolveComponentPullSecrets(values, tc.component, globalOK), privateRegistrySecret)
		if got != tc.want {
			t.Errorf("component %q with good global: covered=%v, want %v", tc.component, got, tc.want)
		}
	}

	// With a global that lacks the private secret, the inheriting component is
	// no longer covered — the exact failure mode of the original incident.
	if contains(resolveComponentPullSecrets(values, "orchestration", globalBad), privateRegistrySecret) {
		t.Error("orchestration inheriting a private-secret-less global should NOT be covered")
	}
}

// TestDeepMerge_HigherLayerWinsArrays confirms the scenario base overrides the
// common layer for pull-secret arrays (Helm replaces arrays wholesale).
func TestDeepMerge_HigherLayerWinsArrays(t *testing.T) {
	low := map[string]any{
		"global": map[string]any{
			"image": map[string]any{
				"pullSecrets": []any{map[string]any{"name": "index-docker-io"}},
			},
		},
	}
	high := map[string]any{
		"global": map[string]any{
			"image": map[string]any{
				"pullSecrets": []any{
					map[string]any{"name": "index-docker-io"},
					map[string]any{"name": privateRegistrySecret},
				},
			},
		},
	}
	merged := deepMerge(low, high)
	got := resolveGlobalPullSecrets(merged)
	if !contains(got, privateRegistrySecret) {
		t.Errorf("merged global pull secrets %v should include %q (higher layer wins)", got, privateRegistrySecret)
	}
}

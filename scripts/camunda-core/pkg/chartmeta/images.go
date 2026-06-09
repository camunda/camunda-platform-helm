// Package chartmeta derives the container image set of a packaged Camunda chart
// ("the artifact") directly from its declared values + bundled subcharts,
// without rendering templates.
//
// Artifact-based: only images that the packaged chart and its bundled subcharts
// reference are listed. Images injected only by external/test-scenario values
// (e.g. a `busybox` init container declared in test/integration scenario layers)
// are NOT part of the artifact and are excluded — chartmeta reads the chart's
// own values.yaml and vendored subcharts only, never the scenario layers.
//
// Component images are read from explicit declared paths (the same paths the
// build's component-image-versions annotation uses), resolved via the chart's
// imageByParams helper (overlay=component, base=global). Bundled subchart images
// (ES/PG/Keycloak for 8.6–8.9) are read by deep-merging the parent's per-alias
// overrides over the vendored subchart's own values (parent wins per field;
// e.g. parent sets repository=bitnamilegacy/os-shell, subchart provides the tag)
// and walking the merged subtree for every enabled image spec (skipping any
// feature whose `enabled: false`, e.g. metrics/volumePermissions).
package chartmeta

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"scripts/camunda-core/pkg/scenarios"
)

// componentSpec locates a declared component image: the overlay path (component
// holding `.image`) and the base path whose `.image` provides fallbacks
// (global for components; the parent component for split sub-images).
type componentSpec struct {
	overlay string
	base    string
}

const globalBase = "global"

var componentSpecs = []componentSpec{
	{"orchestration", globalBase},
	{"zeebe", globalBase},
	{"zeebeGateway", globalBase},
	{"operate", globalBase},
	{"tasklist", globalBase},
	{"identity", globalBase},
	{"optimize", globalBase},
	{"connectors", globalBase},
	{"console", globalBase},
	{"webModeler", globalBase},
	{"webModeler.restapi", "webModeler"},
	{"webModeler.webapp", "webModeler"},
	{"webModeler.websockets", "webModeler"},
}

// ImageSet returns the sorted, de-duplicated set of fully-qualified image
// references the chart declares: camunda components from values.yaml plus the
// images of every bundled subchart. The chart-full-setup scenario layers (base +
// keycloak identity + elasticsearch persistence) are always overlaid to enable
// all components for discovery; extra overlay values files (e.g.
// values-enterprise.yaml) can be passed and are merged after them. A missing
// overlay file is skipped. Subcharts must be vendored (helm dependency update)
// for their images to be included.
func ImageSet(chartDir string, overlays ...string) ([]string, error) {
	values, err := readValues(filepath.Join(chartDir, "values.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read values.yaml: %w", err)
	}
	// Scenario layers come from the same resolver deploy-camunda uses
	// (scenarios.ResolvePaths, read-only) so the layer set can't drift from CI;
	// skipped when the chart has no scenarios dir (e.g. test fixtures). Only
	// `.image` specs are read, so test init containers (e.g. a busybox
	// identity.initContainers entry) are never picked up.
	scenariosDir := filepath.Join(chartDir, "test", "integration", "scenarios", "chart-full-setup")
	cfg := &scenarios.DeploymentConfig{Identity: "keycloak", Persistence: "elasticsearch"}
	layers, _ := cfg.ResolvePaths(scenariosDir)
	for _, path := range append(layers, overlays...) {
		if _, statErr := os.Stat(path); statErr != nil {
			continue // missing overlay (e.g. a chart without values-enterprise.yaml)
		}
		ov, readErr := readValues(path)
		if readErr != nil {
			return nil, fmt.Errorf("read overlay %s: %w", path, readErr)
		}
		values = deepMerge(values, ov)
	}

	seen := map[string]struct{}{}
	var refs []string
	emit := func(ref string) {
		if ref == "" {
			return
		}
		if _, dup := seen[ref]; dup {
			return
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}

	// Camunda component images (explicit declared paths, no over-inclusion).
	for _, spec := range componentSpecs {
		if ref, ok := resolveComponent(values, spec); ok {
			emit(ref)
		}
	}

	// Bundled subchart images (ES/PG/Keycloak): parent per-alias overrides
	// deep-merged over the vendored subchart's own values, then walked.
	subs, err := bundledSubcharts(chartDir)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		// Bundled subcharts are part of the artifact regardless of their default
		// <alias>.enabled (BYO defaults); the full-setup matrix lists them. Only
		// the subchart's INTERNAL feature gates (metrics/volumePermissions
		// enabled:false) prune aux images, handled by walkImages.
		parentSection, _ := values[sub.alias].(map[string]any)
		subVals, err := vendoredSubchartValues(chartDir, sub.name)
		if err != nil {
			return nil, fmt.Errorf("read vendored subchart %s: %w", sub.name, err)
		}
		merged := deepMerge(subVals, parentSection) // parent (overlay) wins per field
		// Force the subchart root "enabled" so its images are listed even when the
		// chart defaults it off (BYO); internal feature gates (metrics,
		// volumePermissions enabled:false) still prune aux images inside walkImages.
		merged["enabled"] = true
		walkImages(merged, emit)
	}

	sort.Strings(refs)
	return refs, nil
}

// resolveComponent applies the chart's imageByParams helper for one component.
func resolveComponent(values map[string]any, spec componentSpec) (string, bool) {
	overlay := imageMapAt(values, spec.overlay)
	base := imageMapAt(values, spec.base)

	repository := firstNonEmpty(overlay["repository"], base["repository"])
	if repository == "" {
		return "", false
	}
	registry := firstNonEmpty(overlay["registry"], base["registry"])
	digest := firstNonEmpty(overlay["digest"], base["digest"])
	tag := firstNonEmpty(overlay["tag"], base["tag"])
	return composeRef(registry, repository, tag, digest)
}

// walkImages recurses an (already parent-merged) subchart values tree and emits
// a reference for every image spec it finds, skipping any subtree whose feature
// is disabled (`enabled: false`). An image spec is a map carrying `repository`.
// A bitnami `global.imageRegistry`, if set, overrides per-image registries.
func walkImages(node any, emit func(string)) {
	globalRegistry := ""
	if m, ok := node.(map[string]any); ok {
		if g, ok := m["global"].(map[string]any); ok {
			globalRegistry, _ = scalarString(g["imageRegistry"])
		}
	}
	walkImagesCtx(node, globalRegistry, emit)
}

func walkImagesCtx(node any, globalRegistry string, emit func(string)) {
	switch n := node.(type) {
	case map[string]any:
		if isDisabled(n) {
			return
		}
		if repo, ok := scalarString(n["repository"]); ok && repo != "" {
			registry, _ := scalarString(n["registry"])
			if globalRegistry != "" {
				registry = globalRegistry
			}
			tag, _ := scalarString(n["tag"])
			digest, _ := scalarString(n["digest"])
			if ref, ok := composeRef(registry, repo, tag, digest); ok {
				emit(ref)
			}
		}
		for _, v := range n {
			walkImagesCtx(v, globalRegistry, emit)
		}
	case []any:
		for _, v := range n {
			walkImagesCtx(v, globalRegistry, emit)
		}
	}
}

// isDisabled reports whether a map node carries `enabled: false`.
func isDisabled(m map[string]any) bool {
	if m == nil {
		return false
	}
	if v, ok := m["enabled"]; ok {
		if b, ok := v.(bool); ok {
			return !b
		}
	}
	return false
}

// composeRef builds the normalized image reference, preferring a digest over a
// tag. Returns ("", false) when there is neither a tag nor a digest.
func composeRef(registry, repository, tag, digest string) (string, bool) {
	prefix := repository
	if registry != "" {
		prefix = registry + "/" + repository
	}
	switch {
	case digest != "":
		return normalizeRef(prefix + "@" + digest), true
	case tag != "":
		return normalizeRef(prefix + ":" + tag), true
	default:
		return "", false
	}
}

// normalizeRef prefixes docker.io/ to a bare reference whose repository begins
// with camunda or bitnami. References already carrying a registry host
// (docker.io/..., registry.camunda.cloud/...) are unchanged.
func normalizeRef(ref string) string {
	if strings.HasPrefix(ref, "camunda") || strings.HasPrefix(ref, "bitnami") {
		return "docker.io/" + ref
	}
	return ref
}

// --- subchart discovery + vendored values ---

type subchartDep struct {
	name  string // dependency chart name (matches vendored <name>-<ver>.tgz)
	alias string // values key under which it is configured (alias or name)
}

// bundledSubcharts parses Chart.yaml dependencies and returns the bundled ones
// (local `file://` repositories), excluding the `common` helper library.
func bundledSubcharts(chartDir string) ([]subchartDep, error) {
	data, err := os.ReadFile(filepath.Join(chartDir, "Chart.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no Chart.yaml (e.g. fixture) → no bundled subcharts
		}
		return nil, fmt.Errorf("read Chart.yaml: %w", err)
	}
	var chart struct {
		Dependencies []struct {
			Name       string `yaml:"name"`
			Alias      string `yaml:"alias"`
			Repository string `yaml:"repository"`
		} `yaml:"dependencies"`
	}
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, fmt.Errorf("parse Chart.yaml: %w", err)
	}
	var out []subchartDep
	for _, d := range chart.Dependencies {
		if d.Name == "common" || !strings.HasPrefix(d.Repository, "file://") {
			continue
		}
		alias := d.Alias
		if alias == "" {
			alias = d.Name
		}
		out = append(out, subchartDep{name: d.Name, alias: alias})
	}
	return out, nil
}

// vendoredSubchartValues extracts <name>/values.yaml from the vendored
// charts/<name>-<version>.tgz produced by `helm dependency update`.
func vendoredSubchartValues(chartDir, name string) (map[string]any, error) {
	matches, err := filepath.Glob(filepath.Join(chartDir, "charts", name+"-*.tgz"))
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("vendored subchart %s-*.tgz not found in %s/charts (run helm dependency update)", name, chartDir)
	}
	f, err := os.Open(matches[0])
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	want := name + "/values.yaml"
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Clean(hdr.Name) == want {
			raw, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			var out map[string]any
			if err := yaml.Unmarshal(raw, &out); err != nil {
				return nil, fmt.Errorf("parse %s: %w", want, err)
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("%s not found in %s", want, matches[0])
}

// --- helpers ---

func imageMapAt(values map[string]any, componentPath string) map[string]string {
	node := valueAt(values, componentPath+".image")
	m, ok := node.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, 4)
	for _, k := range []string{"registry", "repository", "tag", "digest"} {
		if s, ok := scalarString(m[k]); ok {
			out[k] = s
		}
	}
	return out
}

func valueAt(values map[string]any, path string) any {
	var cur any = values
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[part]
		if !ok {
			return nil
		}
	}
	return cur
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", t), true
	default:
		return "", false
	}
}

// deepMerge returns base with overlay merged on top (overlay wins; maps merge
// recursively). Inputs are not mutated beyond the returned tree.
func deepMerge(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, ov := range overlay {
		if bv, ok := out[k]; ok {
			bm, bIsMap := bv.(map[string]any)
			om, oIsMap := ov.(map[string]any)
			if bIsMap && oIsMap {
				out[k] = deepMerge(bm, om)
				continue
			}
		}
		out[k] = ov
	}
	return out
}

func readValues(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return out, nil
}

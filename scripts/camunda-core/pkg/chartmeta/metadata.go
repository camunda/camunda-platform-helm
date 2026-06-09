package chartmeta

import (
	"fmt"
	"path/filepath"
	"strings"
)

// componentField is one line of the component-image-versions annotation: the
// label emitted in the annotation and the values.yaml path holding its tag.
type componentField struct {
	label string
	path  string
}

// 8.8+ "orchestration" architecture component set.
var orchestrationComponents = []componentField{
	{"camunda", "orchestration.image.tag"},
	{"managementIdentity", "identity.image.tag"},
	{"optimize", "optimize.image.tag"},
	{"webModeler", "webModeler.image.tag"},
	{"connectors", "connectors.image.tag"},
	{"console", "console.image.tag"},
}

// 8.6–8.7 classic architecture component set.
var classicComponents = []componentField{
	{"zeebe", "zeebe.image.tag"},
	{"operate", "operate.image.tag"},
	{"tasklist", "tasklist.image.tag"},
	{"identity", "identity.image.tag"},
	{"optimize", "optimize.image.tag"},
	{"webModeler", "webModeler.image.tag"},
	{"connectors", "connectors.image.tag"},
	{"console", "console.image.tag"},
}

// ComponentImageVersions builds the `camunda.io/component-image-versions`
// annotation block from the chart's values.yaml: the version-gated `label: tag`
// map of each component's image tag.
//
// camundaVersion is the chart's Camunda minor line ("8.8", "8.10", ...). 8.8+
// uses the orchestration component set; 8.6–8.7 the classic set. A missing tag
// renders as "N/A".
func ComponentImageVersions(chartDir, camundaVersion string) (string, error) {
	values, err := readValues(filepath.Join(chartDir, "values.yaml"))
	if err != nil {
		return "", fmt.Errorf("read values.yaml: %w", err)
	}
	fields := classicComponents
	if camundaMinorAtLeast(camundaVersion, 8) {
		fields = orchestrationComponents
	}
	var b strings.Builder
	for _, f := range fields {
		tag := "N/A"
		if v := valueAt(values, f.path); v != nil {
			if s, ok := scalarString(v); ok {
				tag = s
			}
		}
		fmt.Fprintf(&b, "%s: %s\n", f.label, tag)
	}
	return b.String(), nil
}

// camundaMinorAtLeast reports whether the minor of an "8.<minor>" version is >= min.
func camundaMinorAtLeast(version string, min int) bool {
	s := strings.TrimPrefix(version, "8.")
	n, seen := 0, false
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		seen = true
	}
	return seen && n >= min
}

// ImageOverride is one workflow_dispatch image-tag override input.
type ImageOverride struct {
	Key   string
	Value string
}

// ImageOverrides renders the non-empty overrides as a `key: value` YAML block
// (the `camunda.io/imageOverrides` annotation source) and reports whether any
// were provided. Entries keep the caller's slice order.
func ImageOverrides(overrides []ImageOverride) (block string, has bool) {
	var b strings.Builder
	for _, o := range overrides {
		if o.Value != "" {
			fmt.Fprintf(&b, "%s: %s\n", o.Key, o.Value)
			has = true
		}
	}
	return b.String(), has
}

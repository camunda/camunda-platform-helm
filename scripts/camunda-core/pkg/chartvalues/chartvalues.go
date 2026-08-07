// Copyright 2026 Camunda Services GmbH
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

// Package chartvalues consolidates Helm values documents and applies upgrade
// deltas to them.
//
// Chart constraints test key presence with hasKey, so a key cannot be cleared
// by layering an overlay that sets it to null — it must be removed from the
// merged document. A delta therefore supports a "$remove" directive listing
// dotted key paths to delete, alongside the keys it sets.
package chartvalues

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Directive keys a delta may carry alongside the values it sets.
const (
	// RemoveDirective holds dotted paths to delete.
	RemoveDirective = "$remove"
	// RenameDirective maps an old dotted path to a new one, moving the value.
	RenameDirective = "$rename"
	// ScaffoldingDirective holds values applied like any other, but excluded
	// from the customer-facing change list. Baselines composed from CI scenario
	// values carry test fixtures alongside product configuration; both must be
	// applied for a run to succeed, only one belongs in an upgrade guide.
	ScaffoldingDirective = "$scaffolding"
)

// Values is a parsed Helm values document.
type Values map[string]any

// LoadFile reads a YAML values file. A file containing only comments or
// whitespace yields an empty, non-nil Values.
func LoadFile(path string) (Values, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read values %s: %w", path, err)
	}
	v := Values{}
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("parse values %s: %w", path, err)
	}
	if v == nil {
		v = Values{}
	}
	return v, nil
}

// WriteFile serialises a values document.
func WriteFile(path string, v Values) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write values %s: %w", path, err)
	}
	return nil
}

// Merge deep-merges src into a copy of dst, matching how Helm layers successive
// -f arguments: maps merge recursively, scalars and arrays are replaced.
func Merge(dst, src Values) Values {
	out := make(Values, len(dst)+len(src))
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		if existing, ok := out[k]; ok {
			em, eok := toMap(existing)
			vm, vok := toMap(v)
			if eok && vok {
				out[k] = Merge(em, vm)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// DeepCopy returns a copy whose nested maps and slices are independent of the
// original. Merge shares references to nested maps it does not have to merge,
// so callers that mutate a merged document must copy first.
func DeepCopy(v Values) Values {
	out := make(Values, len(v))
	for k, val := range v {
		out[k] = deepCopyValue(val)
	}
	return out
}

func deepCopyValue(v any) any {
	if m, ok := toMap(v); ok {
		return DeepCopy(m)
	}
	if s, ok := v.([]any); ok {
		out := make([]any, len(s))
		for i, item := range s {
			out[i] = deepCopyValue(item)
		}
		return out
	}
	return v
}

// MergeFiles loads and merges files left to right.
func MergeFiles(paths []string) (Values, error) {
	out := Values{}
	for _, p := range paths {
		v, err := LoadFile(p)
		if err != nil {
			return nil, err
		}
		out = Merge(out, v)
	}
	return out, nil
}

// DeleteKey removes a dotted key path, reporting whether anything was removed.
// Only the leaf is removed; emptied parent maps are left in place.
func DeleteKey(v Values, dotted string) bool {
	parts := strings.Split(dotted, ".")
	cur := v
	for i, p := range parts {
		if i == len(parts)-1 {
			if _, ok := cur[p]; !ok {
				return false
			}
			delete(cur, p)
			return true
		}
		next, ok := toMap(cur[p])
		if !ok {
			return false
		}
		cur = next
	}
	return false
}

// HasKey reports whether a dotted key path is present.
func HasKey(v Values, dotted string) bool {
	parts := strings.Split(dotted, ".")
	cur := v
	for i, p := range parts {
		if i == len(parts)-1 {
			_, ok := cur[p]
			return ok
		}
		next, ok := toMap(cur[p])
		if !ok {
			return false
		}
		cur = next
	}
	return false
}

// GetKey returns the value at a dotted key path.
func GetKey(v Values, dotted string) (any, bool) {
	parts := strings.Split(dotted, ".")
	cur := v
	for i, p := range parts {
		if i == len(parts)-1 {
			got, ok := cur[p]
			return got, ok
		}
		next, ok := toMap(cur[p])
		if !ok {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

// SetKey writes a value at a dotted key path, creating intermediate maps.
// Returns false when an intermediate segment holds a non-map.
func SetKey(v Values, dotted string, val any) bool {
	parts := strings.Split(dotted, ".")
	cur := v
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = val
			return true
		}
		next, ok := toMap(cur[p])
		if !ok {
			if _, exists := cur[p]; exists {
				return false
			}
			next = Values{}
			cur[p] = next
		} else {
			cur[p] = next
		}
		cur = next
	}
	return false
}

// Delta is an upgrade delta: keys to set, plus paths to remove.
type Delta struct {
	// Remove holds dotted key paths, sorted for deterministic application.
	Remove []string
	// Rename maps old dotted paths to new ones, moving the existing value.
	Rename map[string]string
	// Set holds the delta's remaining keys.
	Set Values
	// Scaffolding holds harness-only values, applied but not customer-facing.
	Scaffolding Values
}

// LoadDelta reads a delta file and splits its directives from the keys it sets.
// A missing or empty file yields an empty Delta.
func LoadDelta(path string) (Delta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Delta{Set: Values{}}, fmt.Errorf("read delta %s: %w", path, err)
	}
	return ParseDelta(b, path)
}

// ParseDelta splits directives from set keys in already-read delta content.
// name is used only in error messages.
func ParseDelta(data []byte, name string) (Delta, error) {
	var d Delta
	d.Set = Values{}

	raw := Values{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return d, fmt.Errorf("parse delta %s: %w", name, err)
	}

	for k, v := range raw {
		switch k {
		case RemoveDirective:
			items, ok := v.([]any)
			if !ok {
				return d, fmt.Errorf("%s: %q must be a list of dotted key paths", name, RemoveDirective)
			}
			for _, item := range items {
				s, ok := item.(string)
				if !ok {
					return d, fmt.Errorf("%s: %q entries must be strings, got %T", name, RemoveDirective, item)
				}
				if s = strings.TrimSpace(s); s != "" {
					d.Remove = append(d.Remove, s)
				}
			}
		case ScaffoldingDirective:
			m, ok := toMap(v)
			if !ok {
				return d, fmt.Errorf("%s: %q must be a map of values", name, ScaffoldingDirective)
			}
			d.Scaffolding = m
		case RenameDirective:
			m, ok := toMap(v)
			if !ok {
				return d, fmt.Errorf("%s: %q must be a map of old to new dotted key paths", name, RenameDirective)
			}
			d.Rename = map[string]string{}
			for old, newVal := range m {
				s, ok := newVal.(string)
				if !ok {
					return d, fmt.Errorf("%s: %q target for %q must be a string, got %T",
						name, RenameDirective, old, newVal)
				}
				d.Rename[old] = strings.TrimSpace(s)
			}
		default:
			d.Set[k] = v
		}
	}
	sort.Strings(d.Remove)
	return d, nil
}

// Apply renames, then removes, then merges the delta's keys into base.
//
// Renames run first so a moved value survives a subsequent removal of its old
// parent. A rename whose source is absent is skipped.
func (d Delta) Apply(base Values) Values {
	out := DeepCopy(base)

	for _, old := range sortedKeys(d.Rename) {
		val, ok := GetKey(out, old)
		if !ok {
			continue
		}
		if SetKey(out, d.Rename[old], val) {
			DeleteKey(out, old)
		}
	}

	for _, path := range d.Remove {
		DeleteKey(out, path)
	}
	out = Merge(out, d.Set)
	return Merge(out, d.Scaffolding)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// IsEmpty reports whether the delta would change nothing.
func (d Delta) IsEmpty() bool {
	return len(d.Remove) == 0 && len(d.Rename) == 0 &&
		len(d.Set) == 0 && len(d.Scaffolding) == 0
}

// LeafPaths returns the dotted paths of every non-map value, sorted. A path
// stops descending at the first value that is not a map, so a list or scalar is
// reported at the key that holds it.
func LeafPaths(v Values) []string {
	var out []string
	var walk func(Values, string)
	walk = func(n Values, pre string) {
		for k, val := range n {
			p := k
			if pre != "" {
				p = pre + "." + k
			}
			if m, ok := toMap(val); ok && len(m) > 0 {
				walk(m, p)
				continue
			}
			out = append(out, p)
		}
	}
	walk(v, "")
	sort.Strings(out)
	return out
}

// HasScaffolding reports whether the delta carries harness-only values.
func (d Delta) HasScaffolding() bool {
	return len(d.Scaffolding) > 0
}

// Consolidate merges layers, applies an optional delta, and writes the result
// to outPath as a single values file.
func Consolidate(layers []string, deltaPath, outPath string) (Values, error) {
	merged, err := MergeFiles(layers)
	if err != nil {
		return nil, err
	}
	if deltaPath != "" {
		d, err := LoadDelta(deltaPath)
		if err != nil {
			return nil, err
		}
		merged = d.Apply(merged)
	}
	if err := WriteFile(outPath, merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// toMap normalises the two map shapes yaml.v3 can produce.
func toMap(v any) (Values, bool) {
	switch m := v.(type) {
	case Values:
		return m, true
	case map[string]any:
		return Values(m), true
	default:
		return nil, false
	}
}

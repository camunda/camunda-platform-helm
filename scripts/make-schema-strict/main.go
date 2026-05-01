// Command make-schema-strict post-processes a Helm chart values.schema.json
// to inject "additionalProperties": false into every object schema that does
// not already declare an explicit value for it. See:
// https://github.com/camunda/camunda-platform-helm/issues/4564
//
// The transform is non-destructive: existing additionalProperties (whether
// false, true, or a sub-schema like {type: string} for free-form maps such
// as labels/annotations) are preserved as-is. Key ordering is preserved so
// review diffs only show the injected lines.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/iancoleman/orderedmap"
)

func main() {
	in := flag.String("i", "", "input file path, or - for stdin")
	out := flag.String("o", "", "output file path, or - for stdout")
	flag.Parse()

	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: make-schema-strict -i FILE -o FILE")
		os.Exit(2)
	}

	raw, err := readInput(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *in, err)
		os.Exit(1)
	}

	root := orderedmap.New()
	if err := json.Unmarshal(raw, &root); err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", *in, err)
		os.Exit(1)
	}

	strictifyMap(root)

	pretty, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	pretty = append(pretty, '\n')

	if err := writeOutput(*out, pretty); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, err)
		os.Exit(1)
	}
}

// makeStrict walks node, injecting additionalProperties: false into every
// object schema that does not already declare it. Returns the (possibly
// mutated) value so callers can replace it inside the parent container —
// orderedmap stores values by value, so in-place mutation through the
// `interface{}` returned by Get does not propagate.
func makeStrict(node interface{}) interface{} {
	switch v := node.(type) {
	case *orderedmap.OrderedMap:
		strictifyMap(v)
		return v
	case orderedmap.OrderedMap:
		strictifyMap(&v)
		return v
	case []interface{}:
		for i, elem := range v {
			v[i] = makeStrict(elem)
		}
		return v
	}
	return node
}

func strictifyMap(m *orderedmap.OrderedMap) {
	if isObjectSchema(m) {
		if _, exists := m.Get("additionalProperties"); !exists {
			m.Set("additionalProperties", false)
		}
	}

	handled := map[string]bool{}

	if v, ok := m.Get("properties"); ok {
		handled["properties"] = true
		m.Set("properties", recurseInOrderedMap(v))
	}

	if v, ok := m.Get("patternProperties"); ok {
		handled["patternProperties"] = true
		m.Set("patternProperties", recurseInOrderedMap(v))
	}

	if v, ok := m.Get("items"); ok {
		handled["items"] = true
		m.Set("items", makeStrict(v))
	}

	for _, comb := range []string{"allOf", "anyOf", "oneOf"} {
		if v, ok := m.Get(comb); ok {
			handled[comb] = true
			m.Set(comb, makeStrict(v))
		}
	}

	if v, ok := m.Get("not"); ok {
		handled["not"] = true
		m.Set("not", makeStrict(v))
	}

	// Generic fallback for definitions, $defs, and any other nested object
	// container. Only recurse into structural types, not primitives.
	for _, k := range m.Keys() {
		if handled[k] {
			continue
		}
		v, _ := m.Get(k)
		switch v.(type) {
		case orderedmap.OrderedMap, *orderedmap.OrderedMap, []interface{}:
			m.Set(k, makeStrict(v))
		}
	}
}

// isObjectSchema returns true if m looks like a JSON Schema describing an
// object — has properties, patternProperties, or explicit type: object.
func isObjectSchema(m *orderedmap.OrderedMap) bool {
	if _, ok := m.Get("properties"); ok {
		return true
	}
	if _, ok := m.Get("patternProperties"); ok {
		return true
	}
	if t, ok := m.Get("type"); ok {
		if s, isStr := t.(string); isStr && s == "object" {
			return true
		}
	}
	return false
}

// recurseInOrderedMap walks each value of an inner ordered map (used for
// properties and patternProperties, where each key maps to a sub-schema)
// and returns the mutated map by value so the caller can store it back.
func recurseInOrderedMap(v interface{}) interface{} {
	inner, ok := asOrderedMap(v)
	if !ok {
		return v
	}
	for _, k := range inner.Keys() {
		sub, _ := inner.Get(k)
		inner.Set(k, makeStrict(sub))
	}
	return *inner
}

// asOrderedMap normalizes value vs pointer return from orderedmap.Get.
func asOrderedMap(v interface{}) (*orderedmap.OrderedMap, bool) {
	switch vv := v.(type) {
	case *orderedmap.OrderedMap:
		return vv, true
	case orderedmap.OrderedMap:
		return &vv, true
	}
	return nil, false
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func writeOutput(path string, data []byte) error {
	if path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

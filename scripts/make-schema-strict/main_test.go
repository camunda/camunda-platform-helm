package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iancoleman/orderedmap"
)

func runStrict(t *testing.T, in string) string {
	t.Helper()
	root := orderedmap.New()
	if err := json.Unmarshal([]byte(in), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	makeStrict(root)
	out, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out)
}

func TestStrictify(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		mustHave   []string
		mustNotHave []string
	}{
		{
			name: "object with properties gets additionalProperties false",
			in: `{
                "type": "object",
                "properties": {
                    "foo": { "type": "string" }
                }
            }`,
			mustHave: []string{`"additionalProperties": false`},
		},
		{
			name: "explicit additionalProperties is preserved",
			in: `{
                "type": "object",
                "properties": {
                    "labels": {
                        "type": "object",
                        "additionalProperties": { "type": "string" }
                    }
                }
            }`,
			mustHave: []string{
				`"additionalProperties": {`,
				`"type": "string"`,
			},
			mustNotHave: []string{`"additionalProperties": false,\n        "type": "object"`},
		},
		{
			name: "explicit true is preserved",
			in: `{
                "type": "object",
                "properties": {
                    "x": {
                        "type": "object",
                        "additionalProperties": true
                    }
                }
            }`,
			mustHave: []string{`"additionalProperties": true`},
		},
		{
			name: "patternProperties triggers strictification",
			in: `{
                "patternProperties": {
                    "^[a-z]+$": { "type": "string" }
                }
            }`,
			mustHave: []string{`"additionalProperties": false`},
		},
		{
			name: "nested objects are recursed",
			in: `{
                "type": "object",
                "properties": {
                    "outer": {
                        "type": "object",
                        "properties": {
                            "inner": { "type": "string" }
                        }
                    }
                }
            }`,
			mustHave: []string{
				`"additionalProperties": false`,
			},
		},
		{
			name: "items array is recursed",
			in: `{
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "name": { "type": "string" }
                    }
                }
            }`,
			mustHave: []string{`"additionalProperties": false`},
		},
		{
			name: "allOf composition is recursed",
			in: `{
                "allOf": [
                    {
                        "type": "object",
                        "properties": {
                            "a": { "type": "string" }
                        }
                    }
                ]
            }`,
			mustHave: []string{`"additionalProperties": false`},
		},
		{
			name: "definitions block is recursed",
			in: `{
                "definitions": {
                    "Pod": {
                        "type": "object",
                        "properties": {
                            "name": { "type": "string" }
                        }
                    }
                }
            }`,
			mustHave: []string{`"additionalProperties": false`},
		},
		{
			name: "leaf type does not get additionalProperties",
			in: `{
                "type": "string"
            }`,
			mustNotHave: []string{`"additionalProperties"`},
		},
		{
			name: "idempotent: running on already-strict schema is a no-op",
			in: `{
                "type": "object",
                "additionalProperties": false,
                "properties": {
                    "foo": { "type": "string" }
                }
            }`,
			mustHave:    []string{`"additionalProperties": false`},
			mustNotHave: []string{`false,\n    "additionalProperties": false`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runStrict(t, tc.in)
			for _, s := range tc.mustHave {
				if !strings.Contains(out, s) {
					t.Errorf("expected output to contain %q\n--- output ---\n%s", s, out)
				}
			}
			for _, s := range tc.mustNotHave {
				if strings.Contains(out, s) {
					t.Errorf("expected output to NOT contain %q\n--- output ---\n%s", s, out)
				}
			}
		})
	}
}

func TestKeyOrderPreserved(t *testing.T) {
	in := `{
        "type": "object",
        "description": "z-first",
        "properties": {
            "z_first": { "type": "string" },
            "a_second": { "type": "string" }
        }
    }`
	out := runStrict(t, in)
	zIdx := strings.Index(out, `"z_first"`)
	aIdx := strings.Index(out, `"a_second"`)
	if zIdx < 0 || aIdx < 0 {
		t.Fatalf("missing keys in output: %s", out)
	}
	if zIdx > aIdx {
		t.Errorf("expected z_first before a_second, got order:\n%s", out)
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	var out any
	require.NoError(t, json.Unmarshal([]byte(s), &out))
	return out
}

func mustYAMLMap(t *testing.T, s string) map[string]any {
	t.Helper()
	v := mustJSON(t, s)
	m, ok := v.(map[string]any)
	require.True(t, ok)
	return m
}

func TestStrictify(t *testing.T) {
	tests := []struct {
		name           string
		schema         string
		wantAdditional any // value of root additionalProperties after strictify
	}{
		{
			name:           "bare object gets additionalProperties false",
			schema:         `{"type":"object","properties":{"a":{"type":"string"}}}`,
			wantAdditional: false,
		},
		{
			name:           "explicit additionalProperties true preserved",
			schema:         `{"type":"object","additionalProperties":true,"properties":{"a":{"type":"string"}}}`,
			wantAdditional: true,
		},
		{
			name:           "additionalProperties string-map carve-out preserved",
			schema:         `{"type":"object","additionalProperties":{"type":"string"}}`,
			wantAdditional: map[string]any{"type": "string"},
		},
		{
			name:           "non-object (type array) untouched",
			schema:         `{"type":["string","object"]}`,
			wantAdditional: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strictify(mustJSON(t, tc.schema)).(map[string]any)
			if tc.wantAdditional == nil {
				_, ok := got["additionalProperties"]
				assert.False(t, ok, "expected no additionalProperties")
				return
			}
			assert.Equal(t, tc.wantAdditional, got["additionalProperties"])
		})
	}
}

func TestStrictifyRecurses(t *testing.T) {
	got := strictify(mustJSON(t, `{
		"type":"object",
		"properties":{
			"nested":{"type":"object","properties":{"x":{"type":"string"}}},
			"keep":{"type":"object","additionalProperties":true}
		}
	}`)).(map[string]any)
	props := got["properties"].(map[string]any)
	assert.Equal(t, false, props["nested"].(map[string]any)["additionalProperties"])
	assert.Equal(t, true, props["keep"].(map[string]any)["additionalProperties"])
}

func TestFindUnknownKeys(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		values string
		want   []string
	}{
		{
			name:   "known key passes",
			schema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
			values: `{"a":"v"}`,
			want:   nil,
		},
		{
			name:   "unknown key flagged",
			schema: `{"type":"object","properties":{"a":{"type":"string"}}}`,
			values: `{"a":"v","typo":"x"}`,
			want:   []string{"typo"},
		},
		{
			name:   "nested unknown key reports dotted path",
			schema: `{"type":"object","properties":{"a":{"type":"object","properties":{"b":{"type":"string"}}}}}`,
			values: `{"a":{"b":"v","bad":"x"}}`,
			want:   []string{"a.bad"},
		},
		{
			name:   "additionalProperties true allows extra",
			schema: `{"type":"object","properties":{"a":{"type":"object","additionalProperties":true}}}`,
			values: `{"a":{"anything":1,"more":2}}`,
			want:   nil,
		},
		{
			name:   "string-map carve-out allows arbitrary keys",
			schema: `{"type":"object","properties":{"labels":{"type":"object","additionalProperties":{"type":"string"}}}}`,
			values: `{"labels":{"foo/bar":"x","k":"y"}}`,
			want:   nil,
		},
		{
			name:   "patternProperties match allowed, non-match flagged",
			schema: `{"type":"object","properties":{"level":{"type":"object","patternProperties":{"^io\\.":{"type":"string"}}}}}`,
			values: `{"level":{"io.camunda":"INFO","ROOT":"DEBUG"}}`,
			want:   []string{"level.ROOT"},
		},
		{
			name:   "string-or-object secret accepts object form",
			schema: `{"type":"object","properties":{"existingSecret":{"type":["string","object"]}}}`,
			values: `{"existingSecret":{"name":"my-secret"}}`,
			want:   nil,
		},
		{
			name:   "array of objects flags unknown item key",
			schema: `{"type":"object","properties":{"env":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`,
			values: `{"env":[{"name":"OK"},{"name":"OK","oops":"x"}]}`,
			want:   []string{"env[1].oops"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			strict := strictify(mustJSON(t, tc.schema))
			got := findUnknownKeys(strict, mustYAMLMap(t, tc.values), "")
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestFindUnknownKeysIsIdempotentOnStrictify(t *testing.T) {
	schema := mustJSON(t, `{"type":"object","properties":{"a":{"type":"string"}}}`)
	s1 := strictify(schema)
	s2 := strictify(s1)
	values := mustYAMLMap(t, `{"a":"v","typo":1}`)
	assert.Equal(t, findUnknownKeys(s1, values, ""), findUnknownKeys(s2, values, ""))
}

func TestChartDependencyRoots(t *testing.T) {
	dir := t.TempDir()
	chartYAML := filepath.Join(dir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartYAML, []byte(`
apiVersion: v2
name: camunda-platform
dependencies:
  - name: keycloak
    alias: identityKeycloak
  - name: postgresql
    alias: identityPostgresql
  - name: elasticsearch
  - name: common
    alias: ""
`), 0o600))

	roots, err := chartDependencyRoots(chartYAML)
	require.NoError(t, err)
	// alias wins when set; name is the fallback (including empty alias).
	assert.Equal(t, []string{"identityKeycloak", "identityPostgresql", "elasticsearch", "common"}, roots)
}

func TestChartDependencyRootsNoDependencies(t *testing.T) {
	dir := t.TempDir()
	chartYAML := filepath.Join(dir, "Chart.yaml")
	require.NoError(t, os.WriteFile(chartYAML, []byte("apiVersion: v2\nname: x\n"), 0o600))
	roots, err := chartDependencyRoots(chartYAML)
	require.NoError(t, err)
	assert.Empty(t, roots)
}

func TestChartDependencyRootsMissingFile(t *testing.T) {
	_, err := chartDependencyRoots(filepath.Join(t.TempDir(), "nope.yaml"))
	assert.Error(t, err)
}

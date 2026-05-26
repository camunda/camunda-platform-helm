package deploy

import (
	"testing"
)

func TestGetNestedString(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		keys []string
		want string
	}{
		{
			name: "simple one-level key",
			m:    map[string]interface{}{"foo": "bar"},
			keys: []string{"foo"},
			want: "bar",
		},
		{
			name: "two-level nested key",
			m: map[string]interface{}{
				"orchestration": map[string]interface{}{
					"index": map[string]interface{}{
						"prefix": "orch-eske-abcd1234",
					},
				},
			},
			keys: []string{"orchestration", "index", "prefix"},
			want: "orch-eske-abcd1234",
		},
		{
			name: "missing intermediate key",
			m:    map[string]interface{}{"foo": "bar"},
			keys: []string{"missing", "path"},
			want: "",
		},
		{
			name: "leaf is not a string",
			m:    map[string]interface{}{"foo": 42},
			keys: []string{"foo"},
			want: "",
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			keys: []string{"foo"},
			want: "",
		},
		{
			name: "intermediate is not a map",
			m:    map[string]interface{}{"foo": "not-a-map"},
			keys: []string{"foo", "bar"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNestedString(tt.m, tt.keys...)
			if got != tt.want {
				t.Errorf("getNestedString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindEnvValue(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]interface{}
		path    []string
		envName string
		want    string
	}{
		{
			name: "finds env var in array",
			m: map[string]interface{}{
				"orchestration": map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{"name": "FOO", "value": "bar"},
						map[string]interface{}{"name": "CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_INDEXPREFIX", "value": "op-eske-abcd1234"},
					},
				},
			},
			path:    []string{"orchestration", "env"},
			envName: "CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_INDEXPREFIX",
			want:    "op-eske-abcd1234",
		},
		{
			name: "env var not found",
			m: map[string]interface{}{
				"orchestration": map[string]interface{}{
					"env": []interface{}{
						map[string]interface{}{"name": "FOO", "value": "bar"},
					},
				},
			},
			path:    []string{"orchestration", "env"},
			envName: "MISSING",
			want:    "",
		},
		{
			name:    "path does not exist",
			m:       map[string]interface{}{},
			path:    []string{"orchestration", "env"},
			envName: "FOO",
			want:    "",
		},
		{
			name: "array is empty",
			m: map[string]interface{}{
				"orchestration": map[string]interface{}{
					"env": []interface{}{},
				},
			},
			path:    []string{"orchestration", "env"},
			envName: "FOO",
			want:    "",
		},
		{
			name: "array contains non-map entries",
			m: map[string]interface{}{
				"orchestration": map[string]interface{}{
					"env": []interface{}{
						"not-a-map",
						map[string]interface{}{"name": "TARGET", "value": "found"},
					},
				},
			},
			path:    []string{"orchestration", "env"},
			envName: "TARGET",
			want:    "found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findEnvValue(tt.m, tt.path, tt.envName)
			if got != tt.want {
				t.Errorf("findEnvValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadInstalledPrefixes_ParsesRealisticValues(t *testing.T) {
	// This tests the extraction logic with a realistic Helm values structure.
	// We can't call GetInstalledValues (it shells out to helm), but we can
	// test the parsing directly using the same map structure.
	vals := map[string]interface{}{
		"orchestration": map[string]interface{}{
			"index": map[string]interface{}{
				"prefix": "orch-qa-opensearch-upg-abc12345",
			},
			"env": []interface{}{
				map[string]interface{}{
					"name":  "CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_INDEXPREFIX",
					"value": "op-qa-opensearch-upg-abc12345",
				},
				map[string]interface{}{
					"name":  "CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX",
					"value": "task-qa-opensearch-upg-abc12345",
				},
			},
		},
		"optimize": map[string]interface{}{
			"database": map[string]interface{}{
				"opensearch": map[string]interface{}{
					"prefix": "opt-qa-opensearch-upg-abc12345",
				},
			},
		},
	}

	// Directly test the parsing logic (same code as ReadInstalledPrefixes minus the helm call).
	var result InstalledPrefixes
	result.OrchestrationIndexPrefix = getNestedString(vals, "orchestration", "index", "prefix")
	result.OptimizeIndexPrefix = getNestedString(vals, "optimize", "database", "opensearch", "prefix")
	result.OperateIndexPrefix = findEnvValue(vals, []string{"orchestration", "env"}, "CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_INDEXPREFIX")
	if tp := findEnvValue(vals, []string{"orchestration", "env"}, "CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX"); tp != "" {
		result.TasklistIndexPrefix = tp
	}

	if result.OrchestrationIndexPrefix != "orch-qa-opensearch-upg-abc12345" {
		t.Errorf("OrchestrationIndexPrefix = %q, want %q", result.OrchestrationIndexPrefix, "orch-qa-opensearch-upg-abc12345")
	}
	if result.OptimizeIndexPrefix != "opt-qa-opensearch-upg-abc12345" {
		t.Errorf("OptimizeIndexPrefix = %q, want %q", result.OptimizeIndexPrefix, "opt-qa-opensearch-upg-abc12345")
	}
	if result.OperateIndexPrefix != "op-qa-opensearch-upg-abc12345" {
		t.Errorf("OperateIndexPrefix = %q, want %q", result.OperateIndexPrefix, "op-qa-opensearch-upg-abc12345")
	}
	if result.TasklistIndexPrefix != "task-qa-opensearch-upg-abc12345" {
		t.Errorf("TasklistIndexPrefix = %q, want %q", result.TasklistIndexPrefix, "task-qa-opensearch-upg-abc12345")
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is to..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := truncateStr(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

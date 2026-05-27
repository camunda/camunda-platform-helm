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

func TestReadPrefixesFromMap_FullValues(t *testing.T) {
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

	result := readPrefixesFromMap(vals)

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

func TestReadPrefixesFromMap_EmptyMap(t *testing.T) {
	result := readPrefixesFromMap(map[string]interface{}{})

	if result.OrchestrationIndexPrefix != "" {
		t.Errorf("OrchestrationIndexPrefix should be empty, got %q", result.OrchestrationIndexPrefix)
	}
	if result.OptimizeIndexPrefix != "" {
		t.Errorf("OptimizeIndexPrefix should be empty, got %q", result.OptimizeIndexPrefix)
	}
	if result.OperateIndexPrefix != "" {
		t.Errorf("OperateIndexPrefix should be empty, got %q", result.OperateIndexPrefix)
	}
	if result.TasklistIndexPrefix != "" {
		t.Errorf("TasklistIndexPrefix should be empty, got %q", result.TasklistIndexPrefix)
	}
}

func TestReadPrefixesFromMap_PartialValues(t *testing.T) {
	// ES scenario: only orchestration.index.prefix is set, no env vars for OS prefixes.
	vals := map[string]interface{}{
		"orchestration": map[string]interface{}{
			"index": map[string]interface{}{
				"prefix": "orch-eske-12345678",
			},
		},
	}

	result := readPrefixesFromMap(vals)

	if result.OrchestrationIndexPrefix != "orch-eske-12345678" {
		t.Errorf("OrchestrationIndexPrefix = %q, want %q", result.OrchestrationIndexPrefix, "orch-eske-12345678")
	}
	if result.OptimizeIndexPrefix != "" {
		t.Errorf("OptimizeIndexPrefix should be empty for ES scenario, got %q", result.OptimizeIndexPrefix)
	}
	if result.OperateIndexPrefix != "" {
		t.Errorf("OperateIndexPrefix should be empty for ES scenario, got %q", result.OperateIndexPrefix)
	}
	if result.TasklistIndexPrefix != "" {
		t.Errorf("TasklistIndexPrefix should be empty for ES scenario, got %q", result.TasklistIndexPrefix)
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

package deploy

import (
	"reflect"
	"testing"
)

func TestBuildValuesChain(t *testing.T) {
	tests := []struct {
		name      string
		common    []string
		overlays  []string
		extra     []string
		scenario  []string
		debugFile string
		want      []string
	}{
		{
			name:      "full chain with all segments",
			common:    []string{"common-linux.yaml", "common-shared.yaml"},
			overlays:  []string{"values-latest.yaml", "values-enterprise.yaml"},
			extra:     []string{"my-overrides.yaml"},
			scenario:  []string{"base.yaml", "keycloak.yaml", "elasticsearch-external.yaml", "gke.yaml"},
			debugFile: "debug.yaml",
			want: []string{
				"common-linux.yaml", "common-shared.yaml",
				"values-latest.yaml", "values-enterprise.yaml",
				"my-overrides.yaml",
				"base.yaml", "keycloak.yaml", "elasticsearch-external.yaml", "gke.yaml",
				"debug.yaml",
			},
		},
		{
			name:     "no debug file omits trailing entry",
			common:   []string{"common.yaml"},
			overlays: []string{"values-latest.yaml"},
			extra:    []string{"extra.yaml"},
			scenario: []string{"scenario.yaml"},
			want:     []string{"common.yaml", "values-latest.yaml", "extra.yaml", "scenario.yaml"},
		},
		{
			name:     "empty overlays and extra",
			common:   []string{"common.yaml"},
			scenario: []string{"scenario.yaml"},
			want:     []string{"common.yaml", "scenario.yaml"},
		},
		{
			name: "all empty returns empty slice",
			want: []string{},
		},
		{
			name:      "only debug file",
			debugFile: "debug.yaml",
			want:      []string{"debug.yaml"},
		},
		{
			name:      "scenario always after overlays and extra",
			common:    []string{"c.yaml"},
			overlays:  []string{"o.yaml"},
			extra:     []string{"e.yaml"},
			scenario:  []string{"s.yaml"},
			debugFile: "d.yaml",
			want:      []string{"c.yaml", "o.yaml", "e.yaml", "s.yaml", "d.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildValuesChain(tt.common, tt.overlays, tt.extra, tt.scenario, tt.debugFile)
			// Treat nil and empty slice as equivalent
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildValuesChain() = %v, want %v", got, tt.want)
			}
		})
	}
}

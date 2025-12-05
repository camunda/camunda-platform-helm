package deploy

import (
	"scripts/camunda-deployer/pkg/types"
	"strings"
	"testing"
	"time"
)

func TestBuildHelmCommandPreview(t *testing.T) {
	tests := []struct {
		name        string
		opts        types.Options
		valuesFiles []string
		wantParts   []string
	}{
		{
			name: "basic local chart",
			opts: types.Options{
				ReleaseName: "camunda",
				Namespace:   "test-ns",
				ChartPath:   "/path/to/chart",
				Wait:        true,
				Atomic:      true,
				Timeout:     5 * time.Minute,
			},
			valuesFiles: []string{"/tmp/values.yaml"},
			wantParts:   []string{"helm upgrade --install", "camunda", "/path/to/chart", "--namespace", "test-ns", "--wait", "--atomic", "-f", "/tmp/values.yaml"},
		},
		{
			name: "remote chart with version",
			opts: types.Options{
				ReleaseName: "camunda",
				Namespace:   "prod",
				Chart:       "camunda-platform",
				Version:     "8.8.0",
				Wait:        true,
			},
			valuesFiles: nil,
			wantParts:   []string{"helm upgrade --install", "camunda", "camunda-platform", "--version", "8.8.0", "--namespace", "prod", "--wait"},
		},
		{
			name: "multiple values files",
			opts: types.Options{
				ReleaseName: "test",
				Namespace:   "default",
				ChartPath:   "./chart",
			},
			valuesFiles: []string{"values1.yaml", "values2.yaml", "values3.yaml"},
			wantParts:   []string{"-f", "values1.yaml", "-f", "values2.yaml", "-f", "values3.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildHelmCommandPreview(tt.opts, tt.valuesFiles)

			for _, part := range tt.wantParts {
				if !strings.Contains(result, part) {
					t.Errorf("buildHelmCommandPreview() = %q, want to contain %q", result, part)
				}
			}
		})
	}
}

func TestBuildHelmCommandPreview_CreateNamespace(t *testing.T) {
	opts := types.Options{
		ReleaseName: "test",
		Namespace:   "new-ns",
		ChartPath:   "./chart",
	}

	result := buildHelmCommandPreview(opts, nil)

	if !strings.Contains(result, "--create-namespace") {
		t.Errorf("buildHelmCommandPreview() should include --create-namespace")
	}
}

func TestBuildHelmCommandPreview_Timeout(t *testing.T) {
	opts := types.Options{
		ReleaseName: "test",
		Namespace:   "ns",
		ChartPath:   "./chart",
		Timeout:     10 * time.Minute,
	}

	result := buildHelmCommandPreview(opts, nil)

	if !strings.Contains(result, "--timeout") {
		t.Errorf("buildHelmCommandPreview() should include --timeout")
	}
	if !strings.Contains(result, "10m0s") {
		t.Errorf("buildHelmCommandPreview() timeout should be formatted correctly")
	}
}


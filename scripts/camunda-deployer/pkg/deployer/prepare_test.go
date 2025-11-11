package deployer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildValuesList(t *testing.T) {
	// Create a temporary chart directory structure for testing
	tmpDir := t.TempDir()
	chartPath := filepath.Join(tmpDir, "test-chart")
	scenarioBase := filepath.Join(chartPath, "test", "integration", "scenarios", "chart-full-setup")
	
	if err := os.MkdirAll(scenarioBase, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create test scenario files
	scenarios := []string{"basic", "keycloak", "opensearch"}
	for _, s := range scenarios {
		filename := filepath.Join(scenarioBase, "values-integration-test-ingress-"+s+".yaml")
		if err := os.WriteFile(filename, []byte("# test values"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Create optional overlay files
	enterpriseFile := filepath.Join(scenarioBase, "values-enterprise.yaml")
	if err := os.WriteFile(enterpriseFile, []byte("# enterprise"), 0644); err != nil {
		t.Fatalf("failed to create enterprise file: %v", err)
	}

	digestFile := filepath.Join(scenarioBase, "values-digest.yaml")
	if err := os.WriteFile(digestFile, []byte("# digest"), 0644); err != nil {
		t.Fatalf("failed to create digest file: %v", err)
	}

	tests := []struct {
		name              string
		scenarioDir       string
		scenarios         []string
		auth              string
		includeEnterprise bool
		includeDigest     bool
		userValues        []string
		want              []string
		wantErr           bool
	}{
		{
			name:        "single scenario no auth",
			scenarioDir: scenarioBase,
			scenarios:   []string{"basic"},
			auth:        "",
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
			},
			wantErr: false,
		},
		{
			name:        "scenario with auth",
			scenarioDir: scenarioBase,
			scenarios:   []string{"basic"},
			auth:        "keycloak",
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-keycloak.yaml"),
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
			},
			wantErr: false,
		},
		{
			name:        "multiple scenarios",
			scenarioDir: scenarioBase,
			scenarios:   []string{"basic", "opensearch"},
			auth:        "",
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
				filepath.Join(scenarioBase, "values-integration-test-ingress-opensearch.yaml"),
			},
			wantErr: false,
		},
		{
			name:              "with enterprise overlay",
			scenarioDir:       scenarioBase,
			scenarios:         []string{"basic"},
			auth:              "",
			includeEnterprise: true,
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
				enterpriseFile,
			},
			wantErr: false,
		},
		{
			name:          "with digest overlay",
			scenarioDir:   scenarioBase,
			scenarios:     []string{"basic"},
			auth:          "",
			includeDigest: true,
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
				digestFile,
			},
			wantErr: false,
		},
		{
			name:        "with user values",
			scenarioDir: scenarioBase,
			scenarios:   []string{"basic"},
			auth:        "",
			userValues: []string{
				"/custom/values1.yaml",
				"/custom/values2.yaml",
			},
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
				"/custom/values1.yaml",
				"/custom/values2.yaml",
			},
			wantErr: false,
		},
		{
			name:              "full layering: auth + scenarios + overlays + user",
			scenarioDir:       scenarioBase,
			scenarios:         []string{"basic", "opensearch"},
			auth:              "keycloak",
			includeEnterprise: true,
			includeDigest:     true,
			userValues: []string{
				"/custom/override.yaml",
			},
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-keycloak.yaml"),
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
				filepath.Join(scenarioBase, "values-integration-test-ingress-opensearch.yaml"),
				enterpriseFile,
				digestFile,
				"/custom/override.yaml",
			},
			wantErr: false,
		},
		{
			name:        "empty scenarios list",
			scenarioDir: scenarioBase,
			scenarios:   []string{},
			auth:        "",
			want:        nil,
			wantErr:     false,
		},
		{
			name:        "scenario with whitespace",
			scenarioDir: scenarioBase,
			scenarios:   []string{"  basic  ", ""},
			auth:        "",
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
			},
			wantErr: false,
		},
		{
			name:        "auth with whitespace",
			scenarioDir: scenarioBase,
			scenarios:   []string{"basic"},
			auth:        "  keycloak  ",
			want: []string{
				filepath.Join(scenarioBase, "values-integration-test-ingress-keycloak.yaml"),
				filepath.Join(scenarioBase, "values-integration-test-ingress-basic.yaml"),
			},
			wantErr: false,
		},
		{
			name:        "missing scenario file",
			scenarioDir: scenarioBase,
			scenarios:   []string{"nonexistent"},
			auth:        "",
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "missing auth scenario file",
			scenarioDir: scenarioBase,
			scenarios:   []string{"basic"},
			auth:        "nonexistent-auth",
			want:        nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildValuesList(
				tt.scenarioDir,
				tt.scenarios,
				tt.auth,
				tt.includeEnterprise,
				tt.includeDigest,
				tt.userValues,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildValuesList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Treat nil and empty slice as equivalent
			if !slicesEqual(got, tt.want) {
				t.Errorf("BuildValuesList() got:\n%v\n\nwant:\n%v", got, tt.want)
			}
		})
	}
}

// slicesEqual compares two slices, treating nil and empty slices as equivalent
func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func TestOverlayIfExists(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "scenarios")
	
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create a test overlay file
	existingFile := filepath.Join(scenarioDir, "values-enterprise.yaml")
	if err := os.WriteFile(existingFile, []byte("# test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		scenarioDir string
		fileName    string
		want        string
	}{
		{
			name:        "existing file",
			scenarioDir: scenarioDir,
			fileName:    "values-enterprise.yaml",
			want:        existingFile,
		},
		{
			name:        "non-existing file",
			scenarioDir: scenarioDir,
			fileName:    "values-nonexistent.yaml",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overlayIfExists(tt.scenarioDir, tt.fileName)
			if got != tt.want {
				t.Errorf("overlayIfExists() = %v, want %v", got, tt.want)
			}
		})
	}
}


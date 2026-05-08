package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubstituteManifestVars(t *testing.T) {
	tests := []struct {
		name    string
		content string
		vars    map[string]string
		want    string
	}{
		{
			name:    "substitutes both variables",
			content: "namespace: $NAMESPACE\nname: $RELEASE_NAME-camunda-platform",
			vars:    map[string]string{"NAMESPACE": "my-namespace", "RELEASE_NAME": "integration"},
			want:    "namespace: my-namespace\nname: integration-camunda-platform",
		},
		{
			name:    "substitutes multiple occurrences",
			content: "namespace: $NAMESPACE\ntargetRef: $RELEASE_NAME-camunda-platform\nname: $RELEASE_NAME-camunda-platform",
			vars:    map[string]string{"NAMESPACE": "test-ns", "RELEASE_NAME": "release1"},
			want:    "namespace: test-ns\ntargetRef: release1-camunda-platform\nname: release1-camunda-platform",
		},
		{
			name:    "no placeholders",
			content: "namespace: hardcoded\nname: fixed",
			vars:    map[string]string{"NAMESPACE": "my-namespace", "RELEASE_NAME": "integration"},
			want:    "namespace: hardcoded\nname: fixed",
		},
		{
			name:    "empty values",
			content: "namespace: $NAMESPACE\nname: $RELEASE_NAME-gw",
			vars:    map[string]string{"NAMESPACE": "", "RELEASE_NAME": ""},
			want:    "namespace: \nname: -gw",
		},
		{
			name:    "braced form",
			content: "user: ${RDBMS_USER}\nnamespace: ${NAMESPACE}",
			vars:    map[string]string{"RDBMS_USER": "camunda", "NAMESPACE": "ns1"},
			want:    "user: camunda\nnamespace: ns1",
		},
		{
			name:    "mixed braced and bare",
			content: "user: ${RDBMS_USER}\nuser2: $RDBMS_USER",
			vars:    map[string]string{"RDBMS_USER": "camunda"},
			want:    "user: camunda\nuser2: camunda",
		},
		{
			name:    "longer keys win — no partial corruption",
			content: "x: $NAMESPACE_TAG y: $NAMESPACE",
			vars:    map[string]string{"NAMESPACE": "ns", "NAMESPACE_TAG": "v1"},
			want:    "x: v1 y: ns",
		},
		{
			name:    "missing var leaves placeholder when not in map",
			content: "x: $UNKNOWN y: $NAMESPACE",
			vars:    map[string]string{"NAMESPACE": "ns"},
			want:    "x: $UNKNOWN y: ns",
		},
		{
			name:    "empty map is no-op",
			content: "x: $A",
			vars:    map[string]string{},
			want:    "x: $A",
		},
		{
			name:    "nil map is no-op",
			content: "x: $A",
			vars:    nil,
			want:    "x: $A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteManifestVars(tt.content, tt.vars)
			if got != tt.want {
				t.Errorf("substituteManifestVars() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveResourcesDir(t *testing.T) {
	t.Run("returns empty for empty chart path", func(t *testing.T) {
		got := resolveResourcesDir("")
		if got != "" {
			t.Errorf("resolveResourcesDir(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("returns empty when directory does not exist", func(t *testing.T) {
		got := resolveResourcesDir("/nonexistent/path")
		if got != "" {
			t.Errorf("resolveResourcesDir() = %q, want \"\"", got)
		}
	})

	t.Run("returns path when directory exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		resourcesPath := filepath.Join(tmpDir, "test", "integration", "scenarios", "common", "resources")
		if err := os.MkdirAll(resourcesPath, 0755); err != nil {
			t.Fatal(err)
		}

		got := resolveResourcesDir(tmpDir)
		if got != resourcesPath {
			t.Errorf("resolveResourcesDir() = %q, want %q", got, resourcesPath)
		}
	})
}

func TestLoadSelectedManifests(t *testing.T) {
	t.Run("loads named files in order with substitution", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(tmpDir, "a.yaml"), []byte("kind: A\nns: $NAMESPACE\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "b.yaml"), []byte("kind: B\nuser: ${RDBMS_USER}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "c.yaml"), []byte("kind: C"), 0644); err != nil {
			t.Fatal(err)
		}

		vars := map[string]string{"NAMESPACE": "ns1", "RDBMS_USER": "camunda"}
		manifests, err := loadSelectedManifests(tmpDir, []string{"b.yaml", "a.yaml"}, vars)
		if err != nil {
			t.Fatalf("loadSelectedManifests: %v", err)
		}
		if len(manifests) != 2 {
			t.Fatalf("manifests: want 2, got %d", len(manifests))
		}
		if manifests[0].filename != "b.yaml" || manifests[1].filename != "a.yaml" {
			t.Errorf("order: got %q,%q", manifests[0].filename, manifests[1].filename)
		}
		if !strings.Contains(string(manifests[0].data), "user: camunda") {
			t.Errorf("b.yaml: missing substitution: %s", string(manifests[0].data))
		}
		if !strings.Contains(string(manifests[1].data), "ns: ns1") {
			t.Errorf("a.yaml: missing substitution: %s", string(manifests[1].data))
		}
	})

	t.Run("errors on missing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := loadSelectedManifests(tmpDir, []string{"missing.yaml"}, nil)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("nil filenames returns empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifests, err := loadSelectedManifests(tmpDir, nil, nil)
		if err != nil {
			t.Fatalf("loadSelectedManifests: %v", err)
		}
		if len(manifests) != 0 {
			t.Errorf("manifests: want empty, got %d", len(manifests))
		}
	})
}

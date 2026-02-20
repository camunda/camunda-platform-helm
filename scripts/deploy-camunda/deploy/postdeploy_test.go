package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubstituteManifestVars(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		namespace   string
		releaseName string
		want        string
	}{
		{
			name:        "substitutes both variables",
			content:     "namespace: $NAMESPACE\nname: $RELEASE_NAME-camunda-platform",
			namespace:   "my-namespace",
			releaseName: "integration",
			want:        "namespace: my-namespace\nname: integration-camunda-platform",
		},
		{
			name:        "substitutes multiple occurrences",
			content:     "namespace: $NAMESPACE\ntargetRef: $RELEASE_NAME-camunda-platform\nname: $RELEASE_NAME-camunda-platform",
			namespace:   "test-ns",
			releaseName: "release1",
			want:        "namespace: test-ns\ntargetRef: release1-camunda-platform\nname: release1-camunda-platform",
		},
		{
			name:        "no placeholders",
			content:     "namespace: hardcoded\nname: fixed",
			namespace:   "my-namespace",
			releaseName: "integration",
			want:        "namespace: hardcoded\nname: fixed",
		},
		{
			name:        "empty values",
			content:     "namespace: $NAMESPACE\nname: $RELEASE_NAME-gw",
			namespace:   "",
			releaseName: "",
			want:        "namespace: \nname: -gw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteManifestVars(tt.content, tt.namespace, tt.releaseName)
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

func TestLoadAndSubstituteManifests(t *testing.T) {
	t.Run("loads and substitutes YAML files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a test manifest
		content := `apiVersion: gateway.nginx.org/v1alpha1
kind: ProxySettingsPolicy
metadata:
  name: $RELEASE_NAME-camunda-platform
  namespace: $NAMESPACE
spec:
  targetRefs:
  - name: $RELEASE_NAME-camunda-platform
`
		if err := os.WriteFile(filepath.Join(tmpDir, "test-resource.yaml"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		manifests, err := loadAndSubstituteManifests(tmpDir, "my-ns", "my-release")
		if err != nil {
			t.Fatalf("loadAndSubstituteManifests() error = %v", err)
		}

		if len(manifests) != 1 {
			t.Fatalf("expected 1 manifest, got %d", len(manifests))
		}

		got := string(manifests[0].data)
		if manifests[0].filename != "test-resource.yaml" {
			t.Errorf("filename = %q, want %q", manifests[0].filename, "test-resource.yaml")
		}

		wantContains := []string{
			"name: my-release-camunda-platform",
			"namespace: my-ns",
		}
		for _, want := range wantContains {
			if !strings.Contains(got, want) {
				t.Errorf("manifest does not contain %q.\nGot:\n%s", want, got)
			}
		}

		// Should NOT contain unsubstituted placeholders
		unwanted := []string{"$NAMESPACE", "$RELEASE_NAME"}
		for _, u := range unwanted {
			if strings.Contains(got, u) {
				t.Errorf("manifest should not contain placeholder %q.\nGot:\n%s", u, got)
			}
		}
	})

	t.Run("skips non-YAML files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a non-YAML file
		if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
			t.Fatal(err)
		}
		// Create a YAML file
		if err := os.WriteFile(filepath.Join(tmpDir, "resource.yaml"), []byte("kind: Test"), 0644); err != nil {
			t.Fatal(err)
		}

		manifests, err := loadAndSubstituteManifests(tmpDir, "ns", "rel")
		if err != nil {
			t.Fatalf("loadAndSubstituteManifests() error = %v", err)
		}

		if len(manifests) != 1 {
			t.Fatalf("expected 1 manifest (skipping .txt), got %d", len(manifests))
		}
		if manifests[0].filename != "resource.yaml" {
			t.Errorf("expected resource.yaml, got %s", manifests[0].filename)
		}
	})

	t.Run("handles .yml extension", func(t *testing.T) {
		tmpDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(tmpDir, "resource.yml"), []byte("kind: Test"), 0644); err != nil {
			t.Fatal(err)
		}

		manifests, err := loadAndSubstituteManifests(tmpDir, "ns", "rel")
		if err != nil {
			t.Fatalf("loadAndSubstituteManifests() error = %v", err)
		}

		if len(manifests) != 1 {
			t.Fatalf("expected 1 manifest, got %d", len(manifests))
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a subdirectory
		if err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
			t.Fatal(err)
		}

		manifests, err := loadAndSubstituteManifests(tmpDir, "ns", "rel")
		if err != nil {
			t.Fatalf("loadAndSubstituteManifests() error = %v", err)
		}

		if len(manifests) != 0 {
			t.Fatalf("expected 0 manifests, got %d", len(manifests))
		}
	})

	t.Run("returns empty for empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		manifests, err := loadAndSubstituteManifests(tmpDir, "ns", "rel")
		if err != nil {
			t.Fatalf("loadAndSubstituteManifests() error = %v", err)
		}

		if len(manifests) != 0 {
			t.Fatalf("expected 0 manifests, got %d", len(manifests))
		}
	})

	t.Run("returns error for nonexistent directory", func(t *testing.T) {
		_, err := loadAndSubstituteManifests("/nonexistent/dir", "ns", "rel")
		if err == nil {
			t.Fatal("expected error for nonexistent directory")
		}
	})
}

func TestScenariosRequiringPostDeploy(t *testing.T) {
	t.Run("gateway-keycloak requires post-deploy", func(t *testing.T) {
		if !scenariosRequiringPostDeploy["gateway-keycloak"] {
			t.Error("gateway-keycloak should require post-deploy resources")
		}
	})

	t.Run("other scenarios do not require post-deploy", func(t *testing.T) {
		for _, scenario := range []string{"keycloak", "elasticsearch", "opensearch", "oidc"} {
			if scenariosRequiringPostDeploy[scenario] {
				t.Errorf("scenario %q should NOT require post-deploy resources", scenario)
			}
		}
	})
}

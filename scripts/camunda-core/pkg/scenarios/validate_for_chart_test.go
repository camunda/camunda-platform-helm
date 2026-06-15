package scenarios

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateForChart(t *testing.T) {
	chartDir := t.TempDir()
	mustMkdir := func(p string) {
		t.Helper()
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite := func(p string) {
		t.Helper()
		if err := os.WriteFile(p, []byte("# fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	valuesDir := filepath.Join(chartDir, ValuesDir)
	mustMkdir(filepath.Join(valuesDir, IdentityDir))
	mustMkdir(filepath.Join(valuesDir, PersistenceDir))
	mustMkdir(filepath.Join(valuesDir, PlatformDir))
	mustMkdir(filepath.Join(valuesDir, FeaturesDir))
	mustWrite(filepath.Join(valuesDir, IdentityDir, "keycloak.yaml"))
	mustWrite(filepath.Join(valuesDir, PersistenceDir, "elasticsearch.yaml"))
	mustWrite(filepath.Join(valuesDir, PlatformDir, "gke.yaml"))
	mustWrite(filepath.Join(valuesDir, FeaturesDir, "documentstore.yaml"))

	t.Run("all layers present", func(t *testing.T) {
		cfg := &DeploymentConfig{Identity: "keycloak", Persistence: "elasticsearch", Platform: "gke", Features: []string{"documentstore"}}
		if err := cfg.ValidateForChart(chartDir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing persistence file", func(t *testing.T) {
		cfg := &DeploymentConfig{Identity: "keycloak", Persistence: "no-elasticsearch", Platform: "gke"}
		err := cfg.ValidateForChart(chartDir)
		if err == nil {
			t.Fatal("expected error for missing persistence file")
		}
		if !strings.Contains(err.Error(), "persistence") {
			t.Errorf("expected error to mention persistence, got %q", err.Error())
		}
	})

	t.Run("missing feature file", func(t *testing.T) {
		cfg := &DeploymentConfig{Identity: "keycloak", Persistence: "elasticsearch", Platform: "gke", Features: []string{"rba"}}
		err := cfg.ValidateForChart(chartDir)
		if err == nil {
			t.Fatal("expected error for missing feature file")
		}
		if !strings.Contains(err.Error(), "feature") {
			t.Errorf("expected error to mention feature, got %q", err.Error())
		}
	})

	t.Run("missing values directory", func(t *testing.T) {
		cfg := &DeploymentConfig{Identity: "keycloak", Persistence: "elasticsearch", Platform: "gke"}
		err := cfg.ValidateForChart(t.TempDir())
		if err == nil {
			t.Fatal("expected error for missing values directory")
		}
	})
}

func TestBuildDeploymentConfig_WithChartDir(t *testing.T) {
	chartDir := t.TempDir()
	valuesDir := filepath.Join(chartDir, ValuesDir)
	for _, d := range []string{IdentityDir, PersistenceDir, PlatformDir, FeaturesDir} {
		if err := os.MkdirAll(filepath.Join(valuesDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		filepath.Join(valuesDir, IdentityDir, "keycloak.yaml"),
		filepath.Join(valuesDir, PersistenceDir, "elasticsearch.yaml"),
		filepath.Join(valuesDir, PlatformDir, "gke.yaml"),
	} {
		if err := os.WriteFile(f, []byte("# fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("validates layer files when ChartDir is set", func(t *testing.T) {
		_, err := BuildDeploymentConfig("elasticsearch", BuilderOverrides{
			Identity:    "keycloak",
			Persistence: "no-elasticsearch", // file does not exist in tmp chart
			Platform:    "gke",
			ChartDir:    chartDir,
		})
		if err == nil {
			t.Fatal("expected error when ChartDir validation finds a missing layer")
		}
	})

	t.Run("no validation when ChartDir is empty (legacy behaviour)", func(t *testing.T) {
		_, err := BuildDeploymentConfig("elasticsearch", BuilderOverrides{
			Identity:    "keycloak",
			Persistence: "no-elasticsearch",
			Platform:    "gke",
		})
		if err != nil {
			t.Fatalf("did not expect error without ChartDir, got %v", err)
		}
	})
}

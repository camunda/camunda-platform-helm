package wizard

import (
	"scripts/deploy-camunda/config"
	"testing"
)

func TestBuildSummary(t *testing.T) {
	cs := &chartSourceChoice{Mode: "local", ChartPath: "/path/to/chart"}
	summary := buildSummary("dev", "gke", cs, "my-ns", "my-release", "default", "install",
		"skip", "", "", "", "keycloak", true)

	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	// Should contain key fields
	for _, want := range []string{"dev", "gke", "my-ns", "my-release", "default", "install", "/path/to/chart"} {
		if !contains(summary, want) {
			t.Errorf("summary missing %q:\n%s", want, summary)
		}
	}

	// Auth=keycloak (default) should NOT appear in summary
	if contains(summary, "auth:") {
		t.Errorf("default auth should not appear in summary:\n%s", summary)
	}
}

func TestBuildSummaryRemoteChart(t *testing.T) {
	cs := &chartSourceChoice{Mode: "remote", Chart: "oci://ghcr.io/camunda/helm/camunda-platform", Version: "11.0.0"}
	summary := buildSummary("prod", "eks", cs, "prod-ns", "camunda", "default", "upgrade",
		"hostname", "camunda.example.com", "", "", "oidc", false)

	for _, want := range []string{"prod", "eks", "prod-ns", "camunda", "oci://ghcr.io", "11.0.0", "upgrade", "camunda.example.com", "oidc"} {
		if !contains(summary, want) {
			t.Errorf("summary missing %q:\n%s", want, summary)
		}
	}
}

func TestMockDataSource(t *testing.T) {
	ds := MockDataSource{
		Contexts:  []string{"gke_project_zone_cluster", "eks-dev"},
		RepoRoot:  "/home/user/camunda-platform-helm",
		Scenarios: []string{"default", "keycloak-mt", "opensearch"},
	}

	contexts, err := ds.KubeContexts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(contexts))
	}

	root, err := ds.DetectRepoRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != "/home/user/camunda-platform-helm" {
		t.Fatalf("expected repo root, got %q", root)
	}

	scenarios, err := ds.ListScenarios("/some/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenarios) != 3 {
		t.Fatalf("expected 3 scenarios, got %d", len(scenarios))
	}
}

func TestNewWizardDefaults(t *testing.T) {
	ds := MockDataSource{}
	w := NewWizard(ds, nil, false)

	if w.ds == nil {
		t.Fatal("expected non-nil DataSource")
	}
	if w.existing != nil {
		t.Fatal("expected nil existing config")
	}
	if w.accessible {
		t.Fatal("expected accessible=false")
	}
}

func TestNewWizardEditMode(t *testing.T) {
	ds := MockDataSource{}
	existing := &config.RootConfig{
		InfraConfig: config.InfraConfig{Platform: "eks"},
	}
	w := NewWizard(ds, existing, true)

	if w.existing == nil {
		t.Fatal("expected non-nil existing config")
	}
	if w.existing.Platform != "eks" {
		t.Fatalf("expected platform eks, got %q", w.existing.Platform)
	}
	if !w.accessible {
		t.Fatal("expected accessible=true")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

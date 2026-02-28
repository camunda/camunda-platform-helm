package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-deployer/pkg/types"
	"scripts/deploy-camunda/config"
	"strings"
	"testing"
)

// --- ScenarioPath fallback tests ---
// These tests verify the critical backward compatibility fix: when ScenarioPath is empty
// (as happens in CI without a config file), it must default to
// {chartPath}/test/integration/scenarios/chart-full-setup.

func TestPrepareScenarioValuesDefaultsScenarioPath(t *testing.T) {
	// Create a realistic chart directory structure mirroring what CI uses
	chartPath := t.TempDir()
	scenarioDir := filepath.Join(chartPath, "test", "integration", "scenarios", "chart-full-setup")
	commonDir := filepath.Join(chartPath, "test", "integration", "scenarios", "common")

	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatalf("failed to create scenario dir: %v", err)
	}
	if err := os.MkdirAll(commonDir, 0755); err != nil {
		t.Fatalf("failed to create common dir: %v", err)
	}

	// Create a scenario file with no env var placeholders to avoid MissingEnvError
	scenarioFile := filepath.Join(scenarioDir, "values-integration-test-ingress-keycloak.yaml")
	if err := os.WriteFile(scenarioFile, []byte("global:\n  testMode: true\n"), 0644); err != nil {
		t.Fatalf("failed to create scenario file: %v", err)
	}

	// Create common values (also no placeholders)
	commonFile := filepath.Join(commonDir, "values-integration-test.yaml")
	if err := os.WriteFile(commonFile, []byte("global:\n  common: true\n"), 0644); err != nil {
		t.Fatalf("failed to create common file: %v", err)
	}

	scenarioCtx := &ScenarioContext{
		ScenarioName:             "keycloak",
		Namespace:                "test-ns",
		Release:                  "integration",
		KeycloakRealm:            "test-realm",
		OptimizeIndexPrefix:      "opt-test",
		OrchestrationIndexPrefix: "orch-test",
	}

	flags := &config.RuntimeFlags{
		ChartPath:    chartPath,
		ScenarioPath: "", // deliberately empty — this is the bug we fixed
		Auth:         "",
		Platform:     "",
		Interactive:  false,
	}

	prepared, err := prepareScenarioValues(scenarioCtx, flags)
	if err != nil {
		t.Fatalf("prepareScenarioValues() failed with empty ScenarioPath: %v", err)
	}

	// Verify the ScenarioPath was defaulted
	expectedScenarioPath := filepath.Join(chartPath, "test/integration/scenarios/chart-full-setup")
	if flags.ScenarioPath != expectedScenarioPath {
		t.Errorf("ScenarioPath not defaulted: got %q, want %q", flags.ScenarioPath, expectedScenarioPath)
	}

	// Verify values files were resolved
	if len(prepared.ValuesFiles) == 0 {
		t.Fatal("prepareScenarioValues() returned no values files")
	}

	// At least the scenario file should be present
	var foundScenario bool
	for _, f := range prepared.ValuesFiles {
		if strings.Contains(f, "values-integration-test-ingress-keycloak.yaml") {
			foundScenario = true
			break
		}
	}
	if !foundScenario {
		t.Errorf("expected keycloak scenario file in values list, got: %v", prepared.ValuesFiles)
	}

	// Clean up temp dir created by prepareScenarioValues
	if prepared.TempDir != "" {
		os.RemoveAll(prepared.TempDir)
	}
}

func TestPrepareScenarioValuesExplicitScenarioPath(t *testing.T) {
	// When ScenarioPath is explicitly set, it should NOT be overridden
	chartPath := t.TempDir()
	customScenarioDir := t.TempDir()

	// Create scenario file in the custom location
	scenarioFile := filepath.Join(customScenarioDir, "values-integration-test-ingress-custom.yaml")
	if err := os.WriteFile(scenarioFile, []byte("global:\n  custom: true\n"), 0644); err != nil {
		t.Fatalf("failed to create scenario file: %v", err)
	}

	scenarioCtx := &ScenarioContext{
		ScenarioName:             "custom",
		Namespace:                "test-ns",
		Release:                  "integration",
		KeycloakRealm:            "test-realm",
		OptimizeIndexPrefix:      "opt-test",
		OrchestrationIndexPrefix: "orch-test",
	}

	flags := &config.RuntimeFlags{
		ChartPath:    chartPath,
		ScenarioPath: customScenarioDir, // explicitly set
		Auth:         "",
		Platform:     "",
		Interactive:  false,
	}

	prepared, err := prepareScenarioValues(scenarioCtx, flags)
	if err != nil {
		t.Fatalf("prepareScenarioValues() failed with explicit ScenarioPath: %v", err)
	}

	// ScenarioPath should remain as the custom value
	if flags.ScenarioPath != customScenarioDir {
		t.Errorf("ScenarioPath changed from explicit value: got %q, want %q", flags.ScenarioPath, customScenarioDir)
	}

	if prepared.TempDir != "" {
		os.RemoveAll(prepared.TempDir)
	}
}

func TestPrepareScenarioValuesWithAuthScenario(t *testing.T) {
	// Test the layering: auth scenario + main scenario
	chartPath := t.TempDir()
	scenarioDir := filepath.Join(chartPath, "test", "integration", "scenarios", "chart-full-setup")
	commonDir := filepath.Join(chartPath, "test", "integration", "scenarios", "common")

	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatalf("failed to create scenario dir: %v", err)
	}
	if err := os.MkdirAll(commonDir, 0755); err != nil {
		t.Fatalf("failed to create common dir: %v", err)
	}

	// Auth scenario (keycloak)
	authFile := filepath.Join(scenarioDir, "values-integration-test-ingress-keycloak.yaml")
	if err := os.WriteFile(authFile, []byte("global:\n  auth: keycloak\n"), 0644); err != nil {
		t.Fatalf("failed to create auth scenario file: %v", err)
	}

	// Main scenario (elasticsearch)
	mainFile := filepath.Join(scenarioDir, "values-integration-test-ingress-elasticsearch.yaml")
	if err := os.WriteFile(mainFile, []byte("global:\n  search: elasticsearch\n"), 0644); err != nil {
		t.Fatalf("failed to create main scenario file: %v", err)
	}

	// Common values
	commonFile := filepath.Join(commonDir, "values-integration-test.yaml")
	if err := os.WriteFile(commonFile, []byte("global:\n  common: true\n"), 0644); err != nil {
		t.Fatalf("failed to create common file: %v", err)
	}

	scenarioCtx := &ScenarioContext{
		ScenarioName:             "elasticsearch",
		Namespace:                "test-ns",
		Release:                  "integration",
		KeycloakRealm:            "test-realm",
		OptimizeIndexPrefix:      "opt-test",
		OrchestrationIndexPrefix: "orch-test",
	}

	flags := &config.RuntimeFlags{
		ChartPath:    chartPath,
		ScenarioPath: "", // empty — should be defaulted
		Auth:         "keycloak",
		Platform:     "",
		Interactive:  false,
	}

	prepared, err := prepareScenarioValues(scenarioCtx, flags)
	if err != nil {
		t.Fatalf("prepareScenarioValues() failed: %v", err)
	}

	// Verify auth and main scenario are both in values files
	var foundAuth, foundMain bool
	for _, f := range prepared.ValuesFiles {
		if strings.Contains(f, "keycloak") {
			foundAuth = true
		}
		if strings.Contains(f, "elasticsearch") {
			foundMain = true
		}
	}
	if !foundAuth {
		t.Errorf("auth scenario 'keycloak' not found in values files: %v", prepared.ValuesFiles)
	}
	if !foundMain {
		t.Errorf("main scenario 'elasticsearch' not found in values files: %v", prepared.ValuesFiles)
	}

	if prepared.TempDir != "" {
		os.RemoveAll(prepared.TempDir)
	}
}

// --- enhanceScenarioError tests ---

func TestEnhanceScenarioError(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		got := enhanceScenarioError(nil, "keycloak", "/some/path", "/chart")
		if got != nil {
			t.Errorf("enhanceScenarioError(nil) = %v, want nil", got)
		}
	})

	t.Run("non-notfound error returned as-is", func(t *testing.T) {
		original := fmt.Errorf("connection timeout")
		got := enhanceScenarioError(original, "keycloak", "/some/path", "/chart")
		if got != original {
			t.Errorf("enhanceScenarioError() = %v, want original error %v", got, original)
		}
	})

	t.Run("not-found error enhanced with available scenarios", func(t *testing.T) {
		dir := t.TempDir()
		// Create some scenario files
		for _, s := range []string{"keycloak", "elasticsearch", "opensearch"} {
			filename := fmt.Sprintf("values-integration-test-ingress-%s.yaml", s)
			os.WriteFile(filepath.Join(dir, filename), []byte("# test"), 0644)
		}

		original := fmt.Errorf("scenario values file not found: whatever")
		got := enhanceScenarioError(original, "nonexistent", dir, "/chart")
		errStr := got.Error()

		if !strings.Contains(errStr, "nonexistent") {
			t.Errorf("error should mention the missing scenario")
		}
		if !strings.Contains(errStr, "keycloak") {
			t.Errorf("error should list available scenario 'keycloak'")
		}
		if !strings.Contains(errStr, "elasticsearch") {
			t.Errorf("error should list available scenario 'elasticsearch'")
		}
		if !strings.Contains(errStr, "opensearch") {
			t.Errorf("error should list available scenario 'opensearch'")
		}
	})

	t.Run("defaults scenarioDir from chartPath when scenarioPath empty", func(t *testing.T) {
		chartPath := t.TempDir()
		defaultDir := filepath.Join(chartPath, "test/integration/scenarios/chart-full-setup")
		os.MkdirAll(defaultDir, 0755)
		os.WriteFile(filepath.Join(defaultDir, "values-integration-test-ingress-keycloak.yaml"), []byte("# test"), 0644)

		original := fmt.Errorf("scenario values file not found")
		got := enhanceScenarioError(original, "missing", "", chartPath)
		errStr := got.Error()

		// Should list scenarios from the default dir
		if !strings.Contains(errStr, "keycloak") {
			t.Errorf("error should list 'keycloak' from default dir, got: %s", errStr)
		}
		if !strings.Contains(errStr, defaultDir) {
			t.Errorf("error should mention default search dir %q", defaultDir)
		}
	})

	t.Run("handles no such file error", func(t *testing.T) {
		dir := t.TempDir()
		original := fmt.Errorf("open /foo/bar: no such file or directory")
		got := enhanceScenarioError(original, "missing", dir, "/chart")
		if got == original {
			t.Errorf("error with 'no such file' should be enhanced")
		}
	})
}

// --- processCommonValues tests ---

func TestProcessCommonValues(t *testing.T) {
	t.Run("discovers and processes common files", func(t *testing.T) {
		root := t.TempDir()
		scenarioDir := filepath.Join(root, "scenarios", "chart-full-setup")
		commonDir := filepath.Join(root, "scenarios", "common")
		outputDir := t.TempDir()

		os.MkdirAll(scenarioDir, 0755)
		os.MkdirAll(commonDir, 0755)

		// Create common values files (no placeholders)
		os.WriteFile(filepath.Join(commonDir, "values-integration-test.yaml"), []byte("global:\n  test: true\n"), 0644)
		os.WriteFile(filepath.Join(commonDir, "values-integration-test-pull-secrets.yaml"), []byte("global:\n  pull: true\n"), 0644)

		// scenarioPath is a path inside chart-full-setup; processCommonValues uses filepath.Dir to go up
		files, err := processCommonValues(scenarioDir, outputDir, "", "")
		if err != nil {
			t.Fatalf("processCommonValues() error: %v", err)
		}

		if len(files) != 2 {
			t.Errorf("processCommonValues() returned %d files, want 2", len(files))
		}

		// All returned files should exist in outputDir
		for _, f := range files {
			if _, err := os.Stat(f); err != nil {
				t.Errorf("processed file %q does not exist", f)
			}
		}
	})

	t.Run("includes platform-specific files", func(t *testing.T) {
		root := t.TempDir()
		scenarioDir := filepath.Join(root, "scenarios", "chart-full-setup")
		commonDir := filepath.Join(root, "scenarios", "common")
		eksDir := filepath.Join(commonDir, "eks")
		outputDir := t.TempDir()

		os.MkdirAll(scenarioDir, 0755)
		os.MkdirAll(eksDir, 0755)

		// Common base
		os.WriteFile(filepath.Join(commonDir, "values-integration-test.yaml"), []byte("global:\n  common: true\n"), 0644)

		// EKS-specific
		os.WriteFile(filepath.Join(eksDir, "base-layer.yaml"), []byte("global:\n  eks: true\n"), 0644)
		os.WriteFile(filepath.Join(eksDir, "tls.yaml"), []byte("global:\n  tls: true\n"), 0644)

		files, err := processCommonValues(scenarioDir, outputDir, "", "eks")
		if err != nil {
			t.Fatalf("processCommonValues() error: %v", err)
		}

		// Should have 1 common + 2 platform = 3 files
		if len(files) != 3 {
			t.Errorf("processCommonValues() returned %d files, want 3 (1 common + 2 eks)", len(files))
		}
	})

	t.Run("gracefully handles missing common directory", func(t *testing.T) {
		root := t.TempDir()
		scenarioDir := filepath.Join(root, "scenarios", "chart-full-setup")
		outputDir := t.TempDir()

		os.MkdirAll(scenarioDir, 0755)
		// No common directory created

		files, err := processCommonValues(scenarioDir, outputDir, "", "")
		if err != nil {
			t.Fatalf("processCommonValues() should not error when common dir missing: %v", err)
		}
		if files != nil {
			t.Errorf("processCommonValues() = %v, want nil when common dir missing", files)
		}
	})

	t.Run("gracefully handles missing platform directory", func(t *testing.T) {
		root := t.TempDir()
		scenarioDir := filepath.Join(root, "scenarios", "chart-full-setup")
		commonDir := filepath.Join(root, "scenarios", "common")
		outputDir := t.TempDir()

		os.MkdirAll(scenarioDir, 0755)
		os.MkdirAll(commonDir, 0755)

		os.WriteFile(filepath.Join(commonDir, "values-integration-test.yaml"), []byte("global:\n  test: true\n"), 0644)

		// Request eks platform but don't create the directory
		files, err := processCommonValues(scenarioDir, outputDir, "", "eks")
		if err != nil {
			t.Fatalf("processCommonValues() should not error for missing platform dir: %v", err)
		}
		// Should still return the common file
		if len(files) != 1 {
			t.Errorf("processCommonValues() returned %d files, want 1", len(files))
		}
	})

	t.Run("orders predefined files before additional files", func(t *testing.T) {
		root := t.TempDir()
		scenarioDir := filepath.Join(root, "scenarios", "chart-full-setup")
		commonDir := filepath.Join(root, "scenarios", "common")
		outputDir := t.TempDir()

		os.MkdirAll(scenarioDir, 0755)
		os.MkdirAll(commonDir, 0755)

		// Create predefined files
		os.WriteFile(filepath.Join(commonDir, "values-integration-test.yaml"), []byte("# base"), 0644)
		os.WriteFile(filepath.Join(commonDir, "values-integration-test-pull-secrets.yaml"), []byte("# pull"), 0644)
		// Create additional file (should come after predefined ones)
		os.WriteFile(filepath.Join(commonDir, "values-extra.yaml"), []byte("# extra"), 0644)

		files, err := processCommonValues(scenarioDir, outputDir, "", "")
		if err != nil {
			t.Fatalf("processCommonValues() error: %v", err)
		}

		if len(files) != 3 {
			t.Fatalf("processCommonValues() returned %d files, want 3", len(files))
		}

		// First two should be the predefined files (by their basename)
		if !strings.HasSuffix(files[0], "values-integration-test.yaml") {
			t.Errorf("first file should be values-integration-test.yaml, got %s", files[0])
		}
		if !strings.HasSuffix(files[1], "values-integration-test-pull-secrets.yaml") {
			t.Errorf("second file should be values-integration-test-pull-secrets.yaml, got %s", files[1])
		}
	})

	t.Run("substitutes env vars in common files", func(t *testing.T) {
		root := t.TempDir()
		scenarioDir := filepath.Join(root, "scenarios", "chart-full-setup")
		commonDir := filepath.Join(root, "scenarios", "common")
		outputDir := t.TempDir()

		os.MkdirAll(scenarioDir, 0755)
		os.MkdirAll(commonDir, 0755)

		// Create a common file with a placeholder
		os.WriteFile(filepath.Join(commonDir, "values-integration-test.yaml"),
			[]byte("global:\n  registry: $TEST_DOCKER_REGISTRY\n"), 0644)

		// Set the env var
		os.Setenv("TEST_DOCKER_REGISTRY", "my-registry.example.com")
		defer os.Unsetenv("TEST_DOCKER_REGISTRY")

		files, err := processCommonValues(scenarioDir, outputDir, "", "")
		if err != nil {
			t.Fatalf("processCommonValues() error: %v", err)
		}

		if len(files) != 1 {
			t.Fatalf("processCommonValues() returned %d files, want 1", len(files))
		}

		// Read the processed file and verify substitution
		content, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatalf("failed to read processed file: %v", err)
		}
		if !strings.Contains(string(content), "my-registry.example.com") {
			t.Errorf("processed file should contain substituted value, got: %s", string(content))
		}
		if strings.Contains(string(content), "$TEST_DOCKER_REGISTRY") {
			t.Errorf("processed file should not contain placeholder, got: %s", string(content))
		}
	})
}

// --- generateScenarioContext tests ---

func TestGenerateScenarioContext(t *testing.T) {
	t.Run("single scenario uses provided namespace", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace: "my-ns",
			Scenarios: []string{"keycloak"},
			Release:   "integration",
		}
		ctx := generateScenarioContext("keycloak", flags)
		if ctx.ScenarioName != "keycloak" {
			t.Errorf("ScenarioName = %q, want keycloak", ctx.ScenarioName)
		}
		if ctx.Namespace != "my-ns" {
			t.Errorf("Namespace = %q, want my-ns", ctx.Namespace)
		}
		if ctx.Release != "integration" {
			t.Errorf("Release = %q, want integration", ctx.Release)
		}
	})

	t.Run("multi scenario appends scenario name to namespace", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace: "base-ns",
			Scenarios: []string{"keycloak", "elasticsearch"},
			Release:   "integration",
		}
		ctx := generateScenarioContext("elasticsearch", flags)
		if ctx.Namespace != "base-ns-elasticsearch" {
			t.Errorf("Namespace = %q, want base-ns-elasticsearch", ctx.Namespace)
		}
	})

	t.Run("uses explicit keycloak realm for single scenario", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace:     "my-ns",
			Scenarios:     []string{"keycloak"},
			Release:       "integration",
			KeycloakRealm: "my-explicit-realm",
		}
		ctx := generateScenarioContext("keycloak", flags)
		if ctx.KeycloakRealm != "my-explicit-realm" {
			t.Errorf("KeycloakRealm = %q, want my-explicit-realm", ctx.KeycloakRealm)
		}
	})

	t.Run("generates realm for multi scenario even if explicit provided", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace:     "my-ns",
			Scenarios:     []string{"keycloak", "elasticsearch"},
			Release:       "integration",
			KeycloakRealm: "explicit-realm",
		}
		ctx := generateScenarioContext("keycloak", flags)
		// For multi-scenario, explicit realm is ignored; unique realm is generated
		if ctx.KeycloakRealm == "explicit-realm" {
			t.Error("KeycloakRealm should be auto-generated for multi-scenario, not use explicit value")
		}
		if ctx.KeycloakRealm == "" {
			t.Error("KeycloakRealm should not be empty")
		}
	})

	t.Run("namespace prefix applied", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace:       "my-ns",
			NamespacePrefix: "distribution",
			Scenarios:       []string{"keycloak"},
			Release:         "integration",
		}
		ctx := generateScenarioContext("keycloak", flags)
		if ctx.Namespace != "distribution-my-ns" {
			t.Errorf("Namespace = %q, want distribution-my-ns", ctx.Namespace)
		}
	})

	t.Run("ingress host from subdomain+base for single scenario", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace:         "my-ns",
			Scenarios:         []string{"keycloak"},
			Release:           "integration",
			IngressSubdomain:  "my-app",
			IngressBaseDomain: "ci.distro.ultrawombat.com",
		}
		ctx := generateScenarioContext("keycloak", flags)
		if ctx.IngressHost != "my-app.ci.distro.ultrawombat.com" {
			t.Errorf("IngressHost = %q, want my-app.ci.distro.ultrawombat.com", ctx.IngressHost)
		}
	})

	t.Run("ingress host prefixed with scenario for multi scenario", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace:         "my-ns",
			Scenarios:         []string{"keycloak", "elasticsearch"},
			Release:           "integration",
			IngressSubdomain:  "my-app",
			IngressBaseDomain: "ci.distro.ultrawombat.com",
		}
		ctx := generateScenarioContext("elasticsearch", flags)
		if ctx.IngressHost != "elasticsearch-my-app.ci.distro.ultrawombat.com" {
			t.Errorf("IngressHost = %q, want elasticsearch-my-app.ci.distro.ultrawombat.com", ctx.IngressHost)
		}
	})

	t.Run("ingress host from direct hostname flag for single scenario", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace:       "my-ns",
			Scenarios:       []string{"oidc"},
			Release:         "integration",
			IngressHostname: "8f97af-gke-5148.ci.distro.ultrawombat.com",
		}
		ctx := generateScenarioContext("oidc", flags)
		if ctx.IngressHost != "8f97af-gke-5148.ci.distro.ultrawombat.com" {
			t.Errorf("IngressHost = %q, want 8f97af-gke-5148.ci.distro.ultrawombat.com", ctx.IngressHost)
		}
	})

	t.Run("ingress host from direct hostname flag for multi scenario", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace:       "my-ns",
			Scenarios:       []string{"oidc", "keycloak"},
			Release:         "integration",
			IngressHostname: "8f97af-gke-5148.ci.distro.ultrawombat.com",
		}
		ctx := generateScenarioContext("oidc", flags)
		if ctx.IngressHost != "oidc-8f97af-gke-5148.ci.distro.ultrawombat.com" {
			t.Errorf("IngressHost = %q, want oidc-8f97af-gke-5148.ci.distro.ultrawombat.com", ctx.IngressHost)
		}
	})

	t.Run("release is always integration", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Namespace: "my-ns",
			Scenarios: []string{"keycloak"},
			Release:   "custom-release",
		}
		ctx := generateScenarioContext("keycloak", flags)
		if ctx.Release != "integration" {
			t.Errorf("Release = %q, want integration (always hardcoded)", ctx.Release)
		}
	})
}

// --- generateCompactRealmName tests ---

func TestGenerateCompactRealmName(t *testing.T) {
	t.Run("short name fits within 36 chars", func(t *testing.T) {
		result := generateCompactRealmName("ns", "keycloak", "a1b2c3d4")
		if len(result) > 36 {
			t.Errorf("realm name %q exceeds 36 chars (length=%d)", result, len(result))
		}
		if !strings.Contains(result, "keycloak") {
			t.Errorf("realm name %q should contain scenario name", result)
		}
	})

	t.Run("long scenario name truncated", func(t *testing.T) {
		longScenario := "this-is-a-very-long-scenario-name-that-exceeds-limits"
		result := generateCompactRealmName("long-namespace", longScenario, "a1b2c3d4")
		if len(result) > 36 {
			t.Errorf("realm name %q exceeds 36 chars (length=%d)", result, len(result))
		}
	})

	t.Run("contains suffix for uniqueness", func(t *testing.T) {
		result := generateCompactRealmName("ns", "keycloak", "uniq1234")
		if !strings.Contains(result, "uniq1234") {
			t.Errorf("realm name %q should contain suffix 'uniq1234'", result)
		}
	})
}

// --- generateRandomSuffix tests ---

func TestGenerateRandomSuffix(t *testing.T) {
	t.Run("returns 8 characters", func(t *testing.T) {
		suffix := generateRandomSuffix()
		if len(suffix) != 8 {
			t.Errorf("generateRandomSuffix() length = %d, want 8", len(suffix))
		}
	})

	t.Run("only contains alphanumeric lowercase", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			suffix := generateRandomSuffix()
			for _, c := range suffix {
				if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
					t.Errorf("generateRandomSuffix() contains invalid char %q in %q", string(c), suffix)
				}
			}
		}
	})

	t.Run("generates unique values", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			s := generateRandomSuffix()
			if seen[s] {
				t.Logf("warning: duplicate suffix %q (may happen rarely)", s)
			}
			seen[s] = true
		}
		// With 36^8 possible values, collisions in 100 tries are extremely unlikely
		if len(seen) < 90 {
			t.Errorf("expected at least 90 unique suffixes, got %d", len(seen))
		}
	})
}

// --- captureEnv / restoreEnv tests ---

func TestCaptureAndRestoreEnv(t *testing.T) {
	t.Run("captures and restores existing vars", func(t *testing.T) {
		os.Setenv("TEST_CAPTURE_A", "valueA")
		os.Setenv("TEST_CAPTURE_B", "valueB")
		defer os.Unsetenv("TEST_CAPTURE_A")
		defer os.Unsetenv("TEST_CAPTURE_B")

		captured := captureEnv([]string{"TEST_CAPTURE_A", "TEST_CAPTURE_B"})

		if captured["TEST_CAPTURE_A"] != "valueA" {
			t.Errorf("captured TEST_CAPTURE_A = %q, want valueA", captured["TEST_CAPTURE_A"])
		}

		// Modify the vars
		os.Setenv("TEST_CAPTURE_A", "modified")
		os.Setenv("TEST_CAPTURE_B", "modified")

		// Restore
		restoreEnv(captured)

		if v := os.Getenv("TEST_CAPTURE_A"); v != "valueA" {
			t.Errorf("after restore, TEST_CAPTURE_A = %q, want valueA", v)
		}
		if v := os.Getenv("TEST_CAPTURE_B"); v != "valueB" {
			t.Errorf("after restore, TEST_CAPTURE_B = %q, want valueB", v)
		}
	})

	t.Run("restores unset vars by unsetting", func(t *testing.T) {
		// Ensure var is not set
		os.Unsetenv("TEST_CAPTURE_UNSET")

		captured := captureEnv([]string{"TEST_CAPTURE_UNSET"})
		if captured["TEST_CAPTURE_UNSET"] != "" {
			t.Errorf("captured value should be empty for unset var")
		}

		// Set it
		os.Setenv("TEST_CAPTURE_UNSET", "temporary")

		// Restore should unset it
		restoreEnv(captured)

		if _, exists := os.LookupEnv("TEST_CAPTURE_UNSET"); exists {
			t.Error("TEST_CAPTURE_UNSET should be unset after restore")
		}
	})
}

// --- redactDeployOpts tests ---

func TestRedactDeployOpts(t *testing.T) {
	// Verify sensitive fields are redacted
	opts := types.Options{
		ChartPath:              "/my/chart",
		Namespace:              "test-ns",
		DockerRegistryPassword: "super-secret",
	}

	redacted := redactDeployOpts(opts)

	if redacted["chartPath"] != "/my/chart" {
		t.Errorf("chartPath should not be redacted")
	}
	if redacted["namespace"] != "test-ns" {
		t.Errorf("namespace should not be redacted")
	}
	if redacted["dockerRegistryPassword"] != "[REDACTED]" {
		t.Errorf("dockerRegistryPassword = %q, want [REDACTED]", redacted["dockerRegistryPassword"])
	}
}

func TestRedactDeployOptsEmptyPassword(t *testing.T) {
	opts := types.Options{
		DockerRegistryPassword: "",
	}

	redacted := redactDeployOpts(opts)

	if redacted["dockerRegistryPassword"] != "" {
		t.Errorf("empty password should remain empty, got %q", redacted["dockerRegistryPassword"])
	}
}

// --- Integration-style backward compatibility tests ---
// These tests simulate the full chart directory structure as it exists in the real repo
// to verify backward compatibility between the old Taskfile-based approach and the new CLI.

func TestBackwardCompatibilityOldChartStructure(t *testing.T) {
	// Simulate an older chart version that does NOT have the common/ directory
	// (e.g., charts/camunda-platform-8.3). The CLI should work without common values.

	chartPath := t.TempDir()
	scenarioDir := filepath.Join(chartPath, "test", "integration", "scenarios", "chart-full-setup")
	os.MkdirAll(scenarioDir, 0755)

	// Only scenario files, no common/ dir
	os.WriteFile(filepath.Join(scenarioDir, "values-integration-test-ingress-keycloak.yaml"),
		[]byte("global:\n  testMode: true\n"), 0644)

	scenarioCtx := &ScenarioContext{
		ScenarioName:             "keycloak",
		Namespace:                "test-ns",
		Release:                  "integration",
		KeycloakRealm:            "test-realm",
		OptimizeIndexPrefix:      "opt-test",
		OrchestrationIndexPrefix: "orch-test",
	}

	flags := &config.RuntimeFlags{
		ChartPath:    chartPath,
		ScenarioPath: "",
		Interactive:  false,
	}

	prepared, err := prepareScenarioValues(scenarioCtx, flags)
	if err != nil {
		t.Fatalf("prepareScenarioValues() should work without common/ dir: %v", err)
	}

	// Should have at least the scenario file
	if len(prepared.ValuesFiles) == 0 {
		t.Fatal("expected at least one values file")
	}

	var foundScenario bool
	for _, f := range prepared.ValuesFiles {
		if strings.Contains(f, "keycloak") {
			foundScenario = true
		}
	}
	if !foundScenario {
		t.Errorf("scenario file not in values list: %v", prepared.ValuesFiles)
	}

	if prepared.TempDir != "" {
		os.RemoveAll(prepared.TempDir)
	}
}

func TestBackwardCompatibilityNewChartStructure(t *testing.T) {
	// Simulate a newer chart version (e.g., 8.9) with common/ and platform subdirs.
	// This tests that the new layered approach works end-to-end.

	chartPath := t.TempDir()
	scenarioDir := filepath.Join(chartPath, "test", "integration", "scenarios", "chart-full-setup")
	commonDir := filepath.Join(chartPath, "test", "integration", "scenarios", "common")
	eksDir := filepath.Join(commonDir, "eks")
	gkeDir := filepath.Join(commonDir, "gke")

	os.MkdirAll(scenarioDir, 0755)
	os.MkdirAll(eksDir, 0755)
	os.MkdirAll(gkeDir, 0755)

	// Common values
	os.WriteFile(filepath.Join(commonDir, "values-integration-test.yaml"),
		[]byte("global:\n  identity:\n    auth:\n      enabled: true\n"), 0644)
	os.WriteFile(filepath.Join(commonDir, "values-integration-test-pull-secrets.yaml"),
		[]byte("global:\n  image:\n    pullPolicy: Always\n"), 0644)

	// EKS platform files
	os.WriteFile(filepath.Join(eksDir, "base-layer.yaml"),
		[]byte("prometheusServiceMonitor:\n  enabled: false\n"), 0644)
	os.WriteFile(filepath.Join(eksDir, "tls.yaml"),
		[]byte("global:\n  ingress:\n    tls:\n      enabled: true\n"), 0644)

	// GKE platform files
	os.WriteFile(filepath.Join(gkeDir, "tls.yaml"),
		[]byte("global:\n  ingress:\n    tls:\n      enabled: true\n"), 0644)

	// Scenario files
	os.WriteFile(filepath.Join(scenarioDir, "values-integration-test-ingress-keycloak.yaml"),
		[]byte("identity:\n  keycloak:\n    enabled: true\n"), 0644)
	os.WriteFile(filepath.Join(scenarioDir, "values-integration-test-ingress-elasticsearch.yaml"),
		[]byte("elasticsearch:\n  enabled: true\n"), 0644)

	// Test with GKE platform
	t.Run("gke platform layering", func(t *testing.T) {
		scenarioCtx := &ScenarioContext{
			ScenarioName:             "elasticsearch",
			Namespace:                "test-ns",
			Release:                  "integration",
			KeycloakRealm:            "test-realm",
			OptimizeIndexPrefix:      "opt-test",
			OrchestrationIndexPrefix: "orch-test",
		}

		flags := &config.RuntimeFlags{
			ChartPath:    chartPath,
			ScenarioPath: "",
			Auth:         "keycloak",
			Platform:     "gke",
			Interactive:  false,
		}

		prepared, err := prepareScenarioValues(scenarioCtx, flags)
		if err != nil {
			t.Fatalf("prepareScenarioValues() failed: %v", err)
		}
		defer os.RemoveAll(prepared.TempDir)

		// Should have: common(2) + gke(1) + auth(1) + scenario(1) = 5
		// The exact count depends on which common files exist
		if len(prepared.ValuesFiles) < 3 {
			t.Errorf("expected at least 3 values files (common + auth + scenario), got %d: %v",
				len(prepared.ValuesFiles), prepared.ValuesFiles)
		}

		// Verify layering order: common files should come before scenario files
		var lastCommonIdx, firstScenarioIdx int
		lastCommonIdx = -1
		firstScenarioIdx = len(prepared.ValuesFiles)
		for i, f := range prepared.ValuesFiles {
			base := filepath.Base(f)
			if strings.HasPrefix(base, "values-integration-test.") ||
				strings.HasPrefix(base, "values-integration-test-pull") ||
				base == "tls.yaml" {
				if i > lastCommonIdx {
					lastCommonIdx = i
				}
			}
			if strings.Contains(base, "ingress-keycloak") || strings.Contains(base, "ingress-elasticsearch") {
				if i < firstScenarioIdx {
					firstScenarioIdx = i
				}
			}
		}
		if lastCommonIdx >= firstScenarioIdx {
			t.Errorf("common files should come before scenario files in layering order, but found common at index %d and scenario at %d",
				lastCommonIdx, firstScenarioIdx)
		}
	})

	// Test with EKS platform
	t.Run("eks platform layering", func(t *testing.T) {
		scenarioCtx := &ScenarioContext{
			ScenarioName:             "keycloak",
			Namespace:                "test-ns",
			Release:                  "integration",
			KeycloakRealm:            "test-realm",
			OptimizeIndexPrefix:      "opt-test",
			OrchestrationIndexPrefix: "orch-test",
		}

		flags := &config.RuntimeFlags{
			ChartPath:    chartPath,
			ScenarioPath: "",
			Auth:         "",
			Platform:     "eks",
			Interactive:  false,
		}

		prepared, err := prepareScenarioValues(scenarioCtx, flags)
		if err != nil {
			t.Fatalf("prepareScenarioValues() failed: %v", err)
		}
		defer os.RemoveAll(prepared.TempDir)

		// Should have: common(2) + eks(2) + scenario(1) = 5
		if len(prepared.ValuesFiles) < 3 {
			t.Errorf("expected at least 3 values files, got %d: %v",
				len(prepared.ValuesFiles), prepared.ValuesFiles)
		}
	})

	// Test without platform (backward compat with old charts)
	t.Run("no platform", func(t *testing.T) {
		scenarioCtx := &ScenarioContext{
			ScenarioName:             "keycloak",
			Namespace:                "test-ns",
			Release:                  "integration",
			KeycloakRealm:            "test-realm",
			OptimizeIndexPrefix:      "opt-test",
			OrchestrationIndexPrefix: "orch-test",
		}

		flags := &config.RuntimeFlags{
			ChartPath:    chartPath,
			ScenarioPath: "",
			Auth:         "",
			Platform:     "",
			Interactive:  false,
		}

		prepared, err := prepareScenarioValues(scenarioCtx, flags)
		if err != nil {
			t.Fatalf("prepareScenarioValues() failed: %v", err)
		}
		defer os.RemoveAll(prepared.TempDir)

		// No platform-specific files should be included
		for _, f := range prepared.ValuesFiles {
			if strings.Contains(f, "eks") || strings.Contains(f, "gke") {
				t.Errorf("platform-specific file %q should not be included when platform is empty", f)
			}
		}
	})
}

func TestBackwardCompatibilityScenarioPathFromConfig(t *testing.T) {
	// Test that ScenarioPath from config file works correctly
	// This simulates the path: config file -> ApplyActiveDeployment -> Validate -> prepareScenarioValues

	customDir := t.TempDir()
	os.WriteFile(filepath.Join(customDir, "values-integration-test-ingress-custom.yaml"),
		[]byte("global:\n  custom: true\n"), 0644)

	// Simulate config merging
	rc := &config.RootConfig{
		ScenarioPath: customDir,
		Deployments: map[string]config.DeploymentConfig{
			"dev": {},
		},
	}

	flags := &config.RuntimeFlags{}
	if err := config.ApplyActiveDeployment(rc, "dev", flags); err != nil {
		t.Fatalf("ApplyActiveDeployment() error: %v", err)
	}

	// ScenarioPath should come from root config
	if flags.ScenarioPath != customDir {
		t.Errorf("ScenarioPath = %q, want %q from config", flags.ScenarioPath, customDir)
	}

	// Now simulate deployment with this config
	scenarioCtx := &ScenarioContext{
		ScenarioName:             "custom",
		Namespace:                "test-ns",
		Release:                  "integration",
		KeycloakRealm:            "test-realm",
		OptimizeIndexPrefix:      "opt-test",
		OrchestrationIndexPrefix: "orch-test",
	}

	flags.ChartPath = t.TempDir() // dummy chart path
	flags.Interactive = false

	prepared, err := prepareScenarioValues(scenarioCtx, flags)
	if err != nil {
		t.Fatalf("prepareScenarioValues() with config-sourced ScenarioPath failed: %v", err)
	}

	// ScenarioPath should still be the custom dir (not defaulted)
	if flags.ScenarioPath != customDir {
		t.Errorf("ScenarioPath changed to %q, should remain %q", flags.ScenarioPath, customDir)
	}

	if prepared.TempDir != "" {
		os.RemoveAll(prepared.TempDir)
	}
}

// --- auth == scenario deduplication tests ---

func TestPrepareScenarioValuesAuthEqualToScenarioNoDuplicates(t *testing.T) {
	// When auth == scenario (e.g., both are "oidc"), the values file should appear
	// only once in the final list, not twice.
	chartPath := t.TempDir()
	scenarioDir := filepath.Join(chartPath, "test", "integration", "scenarios", "chart-full-setup")
	commonDir := filepath.Join(chartPath, "test", "integration", "scenarios", "common")

	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatalf("failed to create scenario dir: %v", err)
	}
	if err := os.MkdirAll(commonDir, 0755); err != nil {
		t.Fatalf("failed to create common dir: %v", err)
	}

	// Create scenario file for "oidc"
	oidcFile := filepath.Join(scenarioDir, "values-integration-test-ingress-oidc.yaml")
	if err := os.WriteFile(oidcFile, []byte("global:\n  auth: oidc\n"), 0644); err != nil {
		t.Fatalf("failed to create oidc scenario file: %v", err)
	}

	// Common values
	commonFile := filepath.Join(commonDir, "values-integration-test.yaml")
	if err := os.WriteFile(commonFile, []byte("global:\n  common: true\n"), 0644); err != nil {
		t.Fatalf("failed to create common file: %v", err)
	}

	scenarioCtx := &ScenarioContext{
		ScenarioName:             "oidc",
		Namespace:                "test-ns",
		Release:                  "integration",
		KeycloakRealm:            "test-realm",
		OptimizeIndexPrefix:      "opt-test",
		OrchestrationIndexPrefix: "orch-test",
	}

	flags := &config.RuntimeFlags{
		ChartPath:    chartPath,
		ScenarioPath: "",
		Auth:         "oidc", // same as scenario name
		Platform:     "",
		Interactive:  false,
	}

	prepared, err := prepareScenarioValues(scenarioCtx, flags)
	if err != nil {
		t.Fatalf("prepareScenarioValues() failed: %v", err)
	}
	defer func() {
		if prepared.TempDir != "" {
			os.RemoveAll(prepared.TempDir)
		}
	}()

	// Count how many times the oidc file appears
	oidcCount := 0
	for _, f := range prepared.ValuesFiles {
		if strings.Contains(f, "ingress-oidc") {
			oidcCount++
		}
	}

	if oidcCount != 1 {
		t.Errorf("expected oidc values file to appear exactly once, but found %d times in: %v",
			oidcCount, prepared.ValuesFiles)
	}
}

func TestPrepareScenarioValuesAuthDifferentFromScenario(t *testing.T) {
	// When auth != scenario (e.g., auth=keycloak, scenario=elasticsearch),
	// both should appear in the values list.
	chartPath := t.TempDir()
	scenarioDir := filepath.Join(chartPath, "test", "integration", "scenarios", "chart-full-setup")
	commonDir := filepath.Join(chartPath, "test", "integration", "scenarios", "common")

	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatalf("failed to create scenario dir: %v", err)
	}
	if err := os.MkdirAll(commonDir, 0755); err != nil {
		t.Fatalf("failed to create common dir: %v", err)
	}

	// Auth scenario file
	if err := os.WriteFile(filepath.Join(scenarioDir, "values-integration-test-ingress-keycloak.yaml"),
		[]byte("global:\n  auth: keycloak\n"), 0644); err != nil {
		t.Fatalf("failed to create keycloak file: %v", err)
	}

	// Main scenario file
	if err := os.WriteFile(filepath.Join(scenarioDir, "values-integration-test-ingress-elasticsearch.yaml"),
		[]byte("global:\n  search: elasticsearch\n"), 0644); err != nil {
		t.Fatalf("failed to create elasticsearch file: %v", err)
	}

	// Common values
	if err := os.WriteFile(filepath.Join(commonDir, "values-integration-test.yaml"),
		[]byte("global:\n  common: true\n"), 0644); err != nil {
		t.Fatalf("failed to create common file: %v", err)
	}

	scenarioCtx := &ScenarioContext{
		ScenarioName:             "elasticsearch",
		Namespace:                "test-ns",
		Release:                  "integration",
		KeycloakRealm:            "test-realm",
		OptimizeIndexPrefix:      "opt-test",
		OrchestrationIndexPrefix: "orch-test",
	}

	flags := &config.RuntimeFlags{
		ChartPath:    chartPath,
		ScenarioPath: "",
		Auth:         "keycloak", // different from scenario
		Platform:     "",
		Interactive:  false,
	}

	prepared, err := prepareScenarioValues(scenarioCtx, flags)
	if err != nil {
		t.Fatalf("prepareScenarioValues() failed: %v", err)
	}
	defer func() {
		if prepared.TempDir != "" {
			os.RemoveAll(prepared.TempDir)
		}
	}()

	// Both auth and main scenario should be present
	var foundAuth, foundMain bool
	for _, f := range prepared.ValuesFiles {
		if strings.Contains(f, "ingress-keycloak") {
			foundAuth = true
		}
		if strings.Contains(f, "ingress-elasticsearch") {
			foundMain = true
		}
	}
	if !foundAuth {
		t.Errorf("auth scenario 'keycloak' not found in values files: %v", prepared.ValuesFiles)
	}
	if !foundMain {
		t.Errorf("main scenario 'elasticsearch' not found in values files: %v", prepared.ValuesFiles)
	}

	// No duplicates — each should appear exactly once
	keycloakCount := 0
	elasticsearchCount := 0
	for _, f := range prepared.ValuesFiles {
		if strings.Contains(f, "ingress-keycloak") {
			keycloakCount++
		}
		if strings.Contains(f, "ingress-elasticsearch") {
			elasticsearchCount++
		}
	}
	if keycloakCount != 1 {
		t.Errorf("keycloak values appeared %d times, want 1", keycloakCount)
	}
	if elasticsearchCount != 1 {
		t.Errorf("elasticsearch values appeared %d times, want 1", elasticsearchCount)
	}
}

// --- generateDebugValuesFile tests ---

func TestGenerateDebugValuesFile(t *testing.T) {
	t.Run("no debug components returns empty", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		path, err := generateDebugValuesFile(t.TempDir(), flags)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "" {
			t.Errorf("expected empty path for no debug components, got %q", path)
		}
	})

	t.Run("orchestration debug generates values", func(t *testing.T) {
		outputDir := t.TempDir()
		flags := &config.RuntimeFlags{
			DebugComponents: map[string]config.DebugConfig{
				"orchestration": {Port: 5005},
			},
		}
		path, err := generateDebugValuesFile(outputDir, flags)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path == "" {
			t.Fatal("expected path for orchestration debug")
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read debug values file: %v", err)
		}
		if !strings.Contains(string(content), "5005") {
			t.Errorf("debug values should contain port 5005")
		}
		if !strings.Contains(string(content), "orchestration") {
			t.Errorf("debug values should contain 'orchestration'")
		}
	})

	t.Run("connectors debug generates values", func(t *testing.T) {
		outputDir := t.TempDir()
		flags := &config.RuntimeFlags{
			DebugComponents: map[string]config.DebugConfig{
				"connectors": {Port: 9999},
			},
		}
		path, err := generateDebugValuesFile(outputDir, flags)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path == "" {
			t.Fatal("expected path for connectors debug")
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read debug values file: %v", err)
		}
		if !strings.Contains(string(content), "connectors") {
			t.Errorf("debug values should contain 'connectors'")
		}
	})

	t.Run("unknown component silently ignored", func(t *testing.T) {
		outputDir := t.TempDir()
		flags := &config.RuntimeFlags{
			DebugComponents: map[string]config.DebugConfig{
				"unknown-component": {Port: 5005},
			},
		}
		path, err := generateDebugValuesFile(outputDir, flags)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Unknown component produces no YAML output
		if path != "" {
			t.Errorf("unknown component should produce empty path, got %q", path)
		}
	})
}

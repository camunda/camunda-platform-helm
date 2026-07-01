<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# test

Test suite for the Camunda Platform 8.10 Helm chart. Three parallel testing strategies: unit tests (template rendering + golden snapshots), integration tests (live cluster scenarios), and e2e tests (Playwright against deployed instances).

## Subdirectories

### `unit/`
Go-based unit tests using Helm template rendering and assertion helpers.

**Purpose**: Validate that template rendering produces correct YAML output for every component and configuration combination.

**Files and packages**:
- `testhelpers/testhelpers.go` — Shared test utilities for Helm options setup, template rendering, and assertions
- Per-component packages: `orchestration/`, `connectors/`, `console/`, `identity/`, `optimize/`, `web-modeler/`, `common/`, `utils/`
- Each package contains: `*_test.go` (test cases), golden files in `golden/` subdirectory

**Key patterns**:
- Test names: `TestXxxTemplate_Description()` (e.g., `TestStatefulSetTemplate_WithDefaults`)
- Golden files: `<component-resource>.golden.yaml` (e.g., `statefulset.golden.yaml`)
- Test structure: Use `testify/suite.Suite` for grouped tests, table-driven test helpers for variations
- Golden updates never by hand — only via `make go.update-golden-only chartPath=charts/camunda-platform-8.10`

**Run all unit tests**:
```bash
make go.test chartPath=charts/camunda-platform-8.10
```

**Run one package**:
```bash
cd test/unit && go test ./orchestration/... -v
```

**Run a single test by name**:
```bash
cd test/unit && go test ./orchestration/... -run TestStatefulSetTemplate
```

### `integration/`
Kubernetes integration test scenarios using live clusters (kind, GKE, EKS, OpenShift).

**Structure**:
- `scenarios/` — Predefined test scenarios (e.g., elasticsearch, opensearch, keycloak, oidc)
- `testsuites/` — Test suites that compose scenarios with assertion logic
- `external-secrets/` — External Secrets Operator integration helpers
- `ci-test-config.yaml` — Matrix configuration for GitHub Actions

**Scenarios** define persistence backend, identity provider, platform, features, and upgrade flows:
```yaml
- name: elasticsearch
  identity: keycloak
  persistence: elasticsearch
  platform: gke
  features: [multitenancy]
```

**Pre-install hooks**: `pre-setup-scripts/pre-install-<scenario-name>.sh` runs after namespace creation, before helm install (example: TLS secret setup).

**Pre-upgrade hooks**: `pre-setup-scripts/pre-upgrade-minor.sh` runs between Step 1 and Step 2 of upgrade flows (example: delete incompatible resources).

### `e2e/`
Playwright end-to-end browser tests.

**Structure**:
- `playwright.config.ts` — Playwright configuration
- Tests imported from `@camunda/e2e-test-suite` npm package (Camunda-published suite)
- Fallback: `empty-test-dir/` placeholder when suite is not published (test skipped)
- `package.json` — Dependencies (Playwright, @camunda/e2e-test-suite)
- `test-results/` — Playwright output (ignored in git)
- `.env.template` — Environment variable template for cluster/auth config

**Run**:
```bash
cd test/e2e
npm install
npm test
```

**Projects** (from config):
- `smoke-tests` — Quick validation tests
- `full-suite` — Complete feature coverage

## For AI Agents

### Working In This Directory

1. **Identify scope**: Unit (template only), integration (live cluster), or e2e (browser)?
2. **Unit tests**: Modify in `test/unit/<component>/`, run `make go.test chartPath=...` locally.
3. **Integration**: Add scenarios to `ci-test-config.yaml`, implement pre-install/pre-upgrade scripts if needed.
4. **E2E**: Managed by Camunda via npm package — only configure `.env` and `playwright.config.ts` locally.

### Golden File Rules

Golden files are **versioned snapshots** of rendered Helm templates. They serve as regression detection and documentation.

**NEVER edit by hand.** Always use the make target:
```bash
make go.update-golden-only chartPath=charts/camunda-platform-8.10
```

This command:
- Renders all templates with test values
- Compares against golden files
- Updates only changed golden files
- Runs the full test suite to confirm no regressions

**When to update**:
- After intentional template changes (e.g., adding a new env var, changing mount paths)
- After dependency updates (e.g., Bitnami subchart version bump affecting rendered output)
- Never as part of unrelated refactors

**Workflow**:
1. Make template changes
2. Run `make go.test chartPath=...` — tests fail with diff output
3. Review the diff (golden file updates) in the test output
4. If correct, run `make go.update-golden-only chartPath=...`
5. Commit golden files — they are part of the test contract

### Testing Requirements

**Before committing**:
1. Unit tests pass: `make go.test chartPath=charts/camunda-platform-8.10`
2. Linting passes: `make helm.lint chartPath=charts/camunda-platform-8.10` and `make go.fmt`
3. Golden files updated only intentionally (review `git diff test/unit/*/golden/`)
4. Apache license headers present: `make go.addlicense-check chartPath=charts/camunda-platform-8.10`

**Pre-requisite**:
```bash
make helm.dependency-update chartPath=charts/camunda-platform-8.10
```

This fetches Bitnami subcharts (elasticsearch, postgresql, etc.). Required before template rendering.

### Common Patterns — Writing New Tests

#### Unit Test Structure
```go
type YourResourceTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestYourResourceTemplate(t *testing.T) {
	t.Parallel()
	chartPath, _ := filepath.Abs("../../../")
	suite.Run(t, &YourResourceTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/path/to/resource.yaml"},
	})
}

func (s *YourResourceTest) TestDescriptiveName() {
	// Arrange: set values for this test case
	values := map[string]string{
		"component.enabled": "true",
		"component.field":   "value",
	}

	// Act: render the template
	output := helm.RenderTemplate(s.T(), &helm.Options{
		SetValues: values,
		// ... other options
	}, s.chartPath, s.release, s.templates)

	// Assert: check rendered output
	var resource corev1.Pod
	helm.UnmarshalK8sYaml(s.T(), output, &resource)
	require.NotNil(s.T(), resource)
	require.Equal(s.T(), "expected-name", resource.Name)
}
```

**Helpers available** (from `testhelpers.testhelpers`):
- `RenderTemplate()` — Render templates with values
- `UnmarshalK8sYaml()` — Parse YAML to Go types (corev1.Pod, appsv1.Deployment, etc.)
- `TestCase` struct — For table-driven tests with skip, values, custom verifiers
- `TestRenderTemplateWithValues()` — Render with golden file comparison

#### Table-Driven Unit Tests
```go
func (s *YourResourceTest) TestVariousConfigurations() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "with defaults",
			Values: map[string]string{},
		},
		{
			Name: "with custom settings",
			Values: map[string]string{
				"field": "custom",
			},
		},
		{
			Name: "invalid configuration",
			Values: map[string]string{
				"field": "invalid",
			},
			Expected: map[string]string{
				"ERROR": "validation failed",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.Name, func() {
			output := testhelpers.RenderTemplate(s.T(), /* ... */, tc)
			// Assert ...
		})
	}
}
```

#### Adding Integration Test Scenarios
1. Create `ci-test-config.yaml` entry:
```yaml
- name: my-scenario
  identity: keycloak
  persistence: elasticsearch
  platform: gke
  features: [multitenancy]
```

2. If scenario needs prerequisites, create `pre-setup-scripts/pre-install-my-scenario.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail
# Receives: $NAMESPACE, $RELEASE, $KUBE_CONTEXT
kubectl create secret -n "$NAMESPACE" tls my-tls \
  --cert=cert.pem --key=key.pem
```

3. If scenario needs cleanup before upgrade, create `pre-setup-scripts/pre-upgrade-minor.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail
# Cleanup incompatible 8.9 resources before upgrading to 8.10
kubectl delete deployment -n "$NAMESPACE" old-component
```

#### Common Assertions
```go
// Check existence and properties
require.NotNil(s.T(), resource)
require.Equal(s.T(), "expected-value", resource.Spec.Field)
require.Len(s.T(), resource.Spec.Containers, 1)

// Check env vars
envMap := map[string]string{}
for _, env := range resource.Spec.Containers[0].Env {
	envMap[env.Name] = env.Value
}
require.Equal(s.T(), "value", envMap["VAR_NAME"])

// Check volume mounts
require.NotNil(s.T(), resource.Spec.Containers[0].VolumeMounts)
```

<!-- MANUAL: Add domain-specific patterns and gotchas for this test directory -->

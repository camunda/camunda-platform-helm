<!-- Parent: ../../../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# Camunda Platform 8.9 Helm Chart Tests

## Purpose

This directory contains **unit** and **e2e tests** for the Camunda Platform 8.9 Helm chart. Unit tests verify template rendering across all components (Orchestration, Console, Modeler, Connectors, Identity, Optimize) using golden file snapshots. E2E tests validate multi-component integration behavior.

## Directory Structure

```
test/
├── unit/                          # Go unit tests with testify/suite + terratest
│   ├── common/                    # Shared test utilities and helpers
│   ├── connectors/                # Connectors component tests
│   ├── console/                   # Console component tests
│   ├── identity/                  # Identity component tests
│   ├── optimize/                  # Optimize component tests
│   ├── orchestration/             # Orchestration (Zeebe) component tests
│   ├── web-modeler/               # Web Modeler component tests
│   ├── utils/                     # Test utility packages
│   ├── testhelpers/               # Test suite base helpers
│   └── README.md                  # Unit test documentation
├── e2e/                           # Playwright e2e tests (legacy)
├── integration/                   # Integration test scenarios
├── ci-test-config.yaml            # CI matrix configuration
└── AGENTS.md                      # This file
```

## Key Files

### Unit Tests
- **Test Files**: `<component>/<resource>_test.go` (e.g., `orchestration/service_test.go`)
- **Test Helpers**: `testhelpers/types.go` — base test suite struct with common methods
- **Golden Files**: `<component>/golden_files/<resource>/*.golden` — expected template output snapshots

### Patterns
- **Suite Pattern**: `type <Resource>Test struct { suite.Suite; chartPath, release, namespace string; templates []string }`
- **Test Entry**: `func Test<Resource>Template(t *testing.T)` — each resource has one entry point
- **Test Methods**: `func (suite *<Resource>Test) Test<Resource>Template_With<Condition>()` — table-driven test methods
- **Golden Files**: Auto-compared via `helm template` output; failures mean template logic changed

### Naming
- Test suites: `<Resource>Test` (e.g., `ServiceTest`, `DeploymentTest`)
- Test functions: `Test<Resource>Template_With<Condition>()` (e.g., `TestStatefulSetTemplate_WithPersistence()`)
- Golden files: Component-scoped, indexed by test case (e.g., `Deployment.yaml`, `Service.yaml`)

## For AI Agents

### Working In This Directory

#### Running Tests

```bash
# All unit tests for 8.9
cd /Users/eamonn.moloney/workspaces/camunda-platform-helm/charts/camunda-platform-8.9/test/unit
go test ./...

# Single component package
go test ./orchestration/...

# Single test by name (most useful)
go test ./orchestration/... -run TestStatefulSetTemplate

# Update golden files only (no test run)
make go.update-golden-only chartPath=charts/camunda-platform-8.9

# Faster golden update (skips cleanup)
make go.update-golden-only-lite chartPath=charts/camunda-platform-8.9
```

#### Before Testing
Always update Helm dependencies first:
```bash
make helm.dependency-update chartPath=charts/camunda-platform-8.9
```

#### Understanding Test Structure
1. Each component has a dedicated subdirectory under `unit/`
2. Each resource (Service, Deployment, StatefulSet, ConfigMap, etc.) gets its own test file
3. Test methods use the pattern `Test<Resource>Template_With<Condition>()`
4. Golden files live in `<component>/golden_files/<resource>/` and are auto-compared
5. Changes to template rendering will cause golden file mismatches — this is expected and requires explicit approval via `make go.update-golden-only`

#### Common Tasks

**Write a new test for a template:**
1. Identify the component and resource (e.g., `orchestration/service.yaml`)
2. Create or edit `orchestration/service_test.go` following the existing pattern
3. Define test cases in a table-driven style
4. Run `go test ./orchestration/... -run TestServiceTemplate` to execute
5. If output differs from golden files, run `make go.update-golden-only` to approve

**Debug a failing test:**
1. Run the specific test: `go test ./orchestration/... -run TestServiceTemplate -v`
2. Check golden file mismatch details in the error output
3. Compare expected vs actual in `<component>/golden_files/<resource>/`
4. If the change is intentional, update golden files; otherwise fix the template

**Add a new component test package:**
1. Create `unit/<component>/` subdirectory
2. Create `<component>_test.go` and any helper files
3. Import shared helpers: `"camunda-platform/test/unit/testhelpers"`
4. Define your test suite and run `go test ./<component>/...`

### Common Patterns

#### Test Suite Pattern
All unit tests follow this structure:
```go
type ServiceTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestServiceTemplate(t *testing.T) {
	t.Parallel()
	chartPath, _ := filepath.Abs("../../../")
	suite.Run(t, &ServiceTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/connectors/service.yaml"},
	})
}

func (suite *ServiceTest) TestServiceTemplate_WithServiceName() {
	opts := &helm.Options{
		ValuesFiles: []string{"values.yaml"},
	}
	output := helm.RenderTemplate(suite.T(), opts, suite.chartPath, suite.release, suite.templates)
	// Assert output matches golden file
}
```

#### Golden File Usage
- Golden files are YAML snapshots of expected `helm template` output
- Stored in `<component>/golden_files/<resource>/` (e.g., `orchestration/golden_files/StatefulSet.yaml`)
- Use `testhelpers.CompareOutput()` or similar to validate
- **Never edit golden files by hand** — always use `make go.update-golden-only` to approve changes

#### Resource Testing
Components test these Kubernetes resources:
- **Deployment**: Stateless applications (Console, Modeler, Web Modeler REST APIs)
- **StatefulSet**: Stateful services (Zeebe, Elasticsearch)
- **Service**: Internal/external networking (ClusterIP, LoadBalancer)
- **ConfigMap**: Non-secret configuration
- **Secret**: Sensitive data (YAML structure, not content tested)
- **Ingress**: External routing (Nginx, Istio)
- **PersistentVolumeClaim**: Storage claims

#### Environment Variables and Secrets
- Test with placeholder values (random suffixes prevent collisions)
- Do not hardcode sensitive data
- Use `testhelpers` for constructing realistic test scenarios

### Common Gotchas

**Golden file mismatches after Helm dependency update:**
- Bitnami subchart versions may have changed, affecting template output
- Run `make helm.dependency-update chartPath=charts/camunda-platform-8.9` first
- Review the diff carefully before updating golden files
- If unexpected, check the parent chart's `Chart.yaml` for subchart version pins

**Tests pass locally but fail in CI:**
- Ensure you ran `make helm.dependency-update chartPath=charts/camunda-platform-8.9`
- Verify golden files were updated with the exact same command used in CI
- Check that `.tool-versions` matches the pinned versions (Go, Helm, etc.)

**Selector mismatches in multi-replica tests:**
- Orchestration tests often verify Zeebe broker scaling (replicas)
- Ensure test namespace uniqueness (handled by `random.UniqueId()`)
- Validate label selectors match expected pod counts

<!-- MANUAL: Add specific test examples, debugging workflows, or integration patterns as needed -->

# Co-locate Playwright E2E tests with versioned Helm charts for PR-level integration validation

- Status: accepted
- Date: 2025-07-09
- Decision-makers: Eamonn Moloney

## Context and Problem Statement

The Helm chart repository relied exclusively on Go-based unit tests (template assertions) and linting to validate chart correctness. These mechanisms cannot detect integration-level failures — broken authentication flows, misconfigured ingress routing, or inter-component connectivity issues — which only manifest on a live Kubernetes cluster. Chart authors received no feedback on these classes of defects until nightly E2E runs executed in a separate repository, creating a dangerous feedback gap between chart change and breakage discovery.

## Decision Drivers

- **Shift-left validation**: Chart authors need integration feedback at PR time, not hours later in nightly pipelines
- **Version isolation**: Charts 8.4–8.8 diverge in structure and supported features; tests must evolve independently per version without cross-version conflicts
- **CI determinism**: Declarative scenario definitions (`ci-test-config.yaml`) enable matrix-driven test execution that is reproducible and auditable
- **Ownership locality**: Teams maintaining a chart version should own the integration tests that validate it, co-located in the same directory tree

## Considered Options

- **External E2E repository only** (existing `c8-cross-component-e2e-tests`) — rejected because the feedback loop is too slow; chart authors don't discover breakage until nightly runs, by which time the breaking commit is merged and context is lost
- **Helm test hooks** (`helm test`) — rejected as too limited for browser-based UI validation and multi-component interaction flows requiring Playwright
- **Centralized test directory** (single `test/e2e/` at repo root) — rejected because version-specific test divergence would cause merge conflicts and conditional logic accumulation; per-version co-location scales cleanly

## Decision Outcome

Playwright-based E2E tests are embedded directly within each chart version's directory (`charts/camunda-platform-8.x/test/`), with declarative scenario configurations (`ci-test-config.yaml`) driving a matrix-based CI pipeline that provisions GKE clusters per PR. This establishes the Helm chart repository as the single feedback point for both template correctness and runtime integration validity.

### Positive Consequences

- Chart authors receive integration-level pass/fail signal before merge, eliminating the class of "works in unit tests, fails on cluster" defects
- Per-version test co-location enables independent evolution of test suites as chart versions diverge in features and structure
- Declarative scenario configs (`ci-test-config.yaml`) create a single source of truth for what scenarios exist per version, consumable by both local tooling and CI

### Negative Consequences

- Test infrastructure (package.json, Playwright config, env templates) is partially duplicated across chart versions, increasing maintenance surface when shared dependencies need updating
- PR CI now provisions GKE resources for matrix scenarios, significantly increasing pipeline duration and cloud cost compared to unit-test-only validation
- Two E2E test locations must remain aligned — the helm repo for PR-level smoke tests and the cross-component repo for nightly depth coverage — creating a coordination burden
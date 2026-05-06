# Migrate integration test infrastructure from custom testsuite runner to Venom declarative framework

- Status: accepted
- Date: 2023-02-13
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The integration test infrastructure relied on a custom Kustomize-based job runner with fragmented CI pipelines — separate workflows for Kubernetes and OpenShift — each with bespoke job definitions that were difficult to maintain and extend. Adding new integration scenarios required duplicating boilerplate across multiple locations, and there was no shared library or fixture mechanism to reduce repetition. A unified, declarative test framework was needed to consolidate all scenarios under a single execution model with clear isolation boundaries.

## Decision Drivers

- **Maintainability:** The existing fragmented pipelines (separate K8s/OpenShift workflows with custom job definitions) created a high maintenance burden for adding or modifying scenarios.
- **Extensibility:** New integration scenarios should be addable with minimal boilerplate — a new directory with a Taskfile and values file, not a new Kustomize overlay and workflow job.
- **Ecosystem alignment:** The test framework should use YAML-native declarative syntax that aligns with the Helm/Kubernetes tooling the team already operates in daily.
- **Scenario isolation:** Each test scenario should be independently configurable, runnable, and debuggable without coupling to other scenarios.

## Considered Options

- **Keep existing Kustomize-based test jobs** — Rejected due to high maintenance overhead, poor extensibility, and tight coupling between test definitions and CI orchestration.
- **Use Helm test or BATS** — Rejected because neither provides the declarative YAML-native scenario composition that Venom offers, and both would require more imperative scripting for complex multi-step integration flows.
- **Venom with Taskfile-based scenario orchestration** — Selected for its declarative test definitions, native YAML syntax, support for shared libraries, and clean separation of scenario configuration from execution logic.

## Decision Outcome

All integration scenarios were consolidated under Venom as the single test framework, orchestrated through a unified GitHub Action (`venom-deploy-testsuite`) and per-scenario Taskfiles. Scenarios are organized as discrete, self-contained directories (`chart-upgrade`, `chart-with-custom-values`, `chart-with-keycloak-v19`, `chart-with-web-modeler`, `chart-with-default-values`) with shared libraries and environment variables factored into `lib/` and `vars/` respectively. The old Kubernetes and OpenShift integration workflows were disabled in favour of a single `test-integration.yaml` entry point.

### Positive Consequences

- Single workflow entry point and consistent execution model across all integration scenarios reduces cognitive load and CI maintenance surface.
- Shared libraries (`lib/`) and fixtures eliminate duplication, making new scenario creation a matter of adding a directory with a Taskfile and values file.
- Declarative YAML-based test definitions align with the Helm/K8s ecosystem, making tests readable and reviewable by the same engineers who maintain charts.

### Negative Consequences

- Introduces Venom and Taskfile as new dependencies that the team must learn and maintain, adding to the project's toolchain surface area.
- The old test infrastructure remains in the repository (disabled but not deleted), creating temporary clutter and potential confusion until a cleanup pass removes it entirely.
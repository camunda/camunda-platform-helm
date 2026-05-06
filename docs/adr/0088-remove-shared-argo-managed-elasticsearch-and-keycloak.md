# Remove shared Argo-managed Elasticsearch and Keycloak infrastructure from CI

- Status: accepted
- Date: 2026-03-17
- Decision-makers: Balázs

## Context and Problem Statement

The CI infrastructure relied on long-lived, shared Elasticsearch and Keycloak instances managed through dedicated deployment workflows and Argo-style infrastructure definitions. This created a coupling between test scenarios and shared stateful services that required dedicated cleanup cronjobs, autoscaling logic, and lifecycle management scripts. The shared infrastructure model introduced flakiness from cross-test contamination and operational burden from maintaining bespoke infra tooling outside the Helm chart's own test lifecycle.

## Decision Drivers

- **Test isolation**: Shared stateful services (ES, Keycloak) caused cross-scenario interference and non-deterministic failures in nightly runs
- **Reduced operational burden**: Dedicated deploy/delete workflows, cleanup cronjobs, and RBAC resources added maintenance cost with no product value
- **Alignment with ephemeral test environments**: The `deploy-camunda` CLI already provisions per-namespace dependencies, making shared infra redundant
- **Simpler CI topology**: Fewer long-lived components means fewer failure modes and easier reasoning about CI state

## Considered Options

- **Keep shared infra with improved cleanup** — Rejected because it treats symptoms (stale data, realm leaks) rather than the root cause (shared mutable state)
- **Move shared infra management to Terraform/Crossplane** — Rejected as over-engineering for CI-only resources that can be ephemeral
- **Remove shared infra entirely, rely on per-scenario provisioning** — Selected; aligns with existing `deploy-camunda` capabilities

## Decision Outcome

All shared Elasticsearch and Keycloak infrastructure — including Helm values, deployment/deletion workflows, cleanup cronjobs, RBAC definitions, and associated shell scripts — was removed. Each CI test scenario now provisions its own dependencies as part of the `deploy-camunda` lifecycle, eliminating the need for centrally managed stateful services.

### Positive Consequences

- Complete test isolation: each scenario owns its full dependency stack, eliminating cross-contamination
- Reduced CI codebase surface area by 19 files of bespoke infrastructure management
- Faster root-cause analysis when failures occur, since no shared state can leak between runs

### Negative Consequences

- Slightly longer per-scenario setup time due to provisioning dependencies from scratch rather than reusing existing instances
- Higher aggregate resource consumption on the GKE cluster when multiple scenarios run concurrently, each with their own ES/Keycloak
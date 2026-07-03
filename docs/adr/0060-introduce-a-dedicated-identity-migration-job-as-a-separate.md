# Introduce a dedicated Identity Migration Job as a separate Helm component (Phase 2)

- Status: accepted
- Date: 2025-07-24
- Decision-makers: Hamza Masood

## Context and Problem Statement

The Camunda 8 Identity service requires data migrations when upgrading between versions (e.g., migrating users, roles, and permissions to new schemas). Previously, migration logic was either embedded within the Identity deployment itself or handled ad-hoc. Phase 2 introduces a standalone Kubernetes Job with its own templates, helpers, and ConfigMap, separating the migration concern from the runtime Identity deployment.

## Decision Drivers

- **Deployment reliability**: Migrations must complete successfully before the Identity service starts serving traffic, and mixing migration with runtime creates failure ambiguity
- **Separation of concerns**: Migration logic has a different lifecycle (run-once) than the Identity service (long-running), warranting distinct Kubernetes resource types
- **Upgrade safety**: A dedicated Job with its own ConfigMap allows independent configuration tuning (timeouts, retry policies) without affecting the Identity Deployment
- **Maintainability**: Isolating migration templates into their own directory (`identity-migration/`) makes chart contributions and reviews clearer

## Considered Options

- **Init container on the Identity Deployment** — Rejected because init container failures block the entire Deployment rollout and provide poor observability into migration-specific errors
- **Helm pre-upgrade hook on existing templates** — Rejected because hook lifecycle management (weight ordering, deletion policies) becomes fragile across multi-component upgrades
- **Standalone Job with dedicated templates (chosen)** — Provides clear ownership, independent retry semantics, and explicit completion signaling

## Decision Outcome

A new `identity-migration` component was added to the chart with its own Job, ConfigMap, and helper templates, separate from the core Identity Deployment. The core directory retains a migration ConfigMap for backward compatibility or shared configuration, while the new component owns the execution lifecycle. The Identity Deployment template was updated to reference the migration Job's completion rather than running migration logic inline.

### Positive Consequences

- Migration failures are isolated and independently retriable without restarting the Identity service
- Chart operators can configure migration resources (CPU/memory limits, backoff policies) independently of the Identity runtime
- Clearer audit trail — Job completion status in the namespace explicitly signals whether migration succeeded

### Negative Consequences

- Increased template surface area in the chart (new directory, helpers, and ConfigMap) adds maintenance overhead for chart contributors
- Operators upgrading from Phase 1 must understand the new component's interaction with the Identity Deployment, adding cognitive load during upgrades
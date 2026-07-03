# Introduce Process Migration as a Dedicated Kubernetes Job with Separate ConfigMap

- Status: accepted
- Date: 2025-07-29
- Decision-makers: Hamza Masood

## Context and Problem Statement

Camunda 8.8 requires a process migration capability that transforms or relocates process definitions/instances as part of platform upgrades or schema evolution. The migration logic needs to run as a discrete, one-shot operation rather than being embedded in the core application's startup sequence, requiring a clear separation between the runtime configuration and the migration execution context.

## Decision Drivers

- **Deployment safety**: Migration must complete independently of the core services' lifecycle — a failed migration should not crash or block the main application pods.
- **Separation of concerns**: Migration configuration (timeouts, batch sizes, target schemas) differs from runtime configuration and should be managed independently.
- **Idempotency and observability**: A Kubernetes Job provides built-in retry semantics, completion tracking, and clear success/failure signals in CI pipelines.
- **Phased rollout**: Labeling this as "Phase 1" implies incremental delivery; the architecture must support future phases without restructuring existing resources.

## Considered Options

- **Embed migration in an init container on the core StatefulSet** — Rejected because it couples migration lifecycle to pod restarts and makes partial retries impossible without recycling all core pods.
- **Run migration as a Helm post-upgrade hook** — Likely considered but a dedicated Job with a separate ConfigMap offers more control over ordering, parallelism, and can be triggered independently of `helm upgrade`.
- **Include migration config in the existing core ConfigMap** — Rejected to avoid polluting the runtime configuration surface and to allow independent versioning of migration parameters.

## Decision Outcome

Process migration is implemented as a standalone Kubernetes Job (`process-migration-job.yaml`) backed by its own ConfigMap (`process-migration-configmap.yaml`), with feature gating and configuration exposed through `values.yaml`. This establishes migration as a first-class lifecycle phase in the chart, decoupled from the core application's runtime.

### Positive Consequences

- Migration failures are isolated — core services remain unaffected and can be debugged/retried independently.
- Future migration phases can add additional Jobs or extend the ConfigMap without modifying core templates.
- CI/CD pipelines gain explicit pass/fail signals from the Job's completion status, improving nightly workflow reliability.

### Negative Consequences

- Additional resource manifests increase chart complexity and the surface area for template bugs (e.g., label selectors, RBAC).
- Operators must now reason about Job completion ordering relative to core pod readiness, adding operational cognitive load during upgrades.
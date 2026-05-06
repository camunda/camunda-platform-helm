# Add first-class initContainers extension points to all Helm chart components

- Status: accepted
- Date: 2023-09-21
- Decision-makers: Hamza Masood

## Context and Problem Statement

Users deploying Camunda Platform on Kubernetes frequently need init containers for operational tasks such as waiting on dependency readiness, downloading plugins, adjusting file permissions, or running pre-start migrations. The Helm chart provided no native mechanism for injecting init containers, forcing users to fork or patch templates to achieve these workflows. This created a maintenance burden for downstream consumers and divergence from Helm community conventions established by charts like Bitnami's.

## Decision Drivers

- **Extensibility without forking:** Users should be able to customize pod initialization behavior purely through values, without maintaining template patches across upgrades.
- **Consistency across components:** A uniform API surface reduces cognitive load and prevents partial solutions that only cover some components.
- **Backward compatibility:** Existing deployments must be unaffected — the feature must be purely additive with safe defaults.
- **Alignment with Helm ecosystem conventions:** Following patterns established by widely-adopted charts reduces onboarding friction for platform engineers familiar with the ecosystem.

## Considered Options

- **Mutating admission webhooks with pod annotations** — Rejected due to external infrastructure dependency, operational complexity, and coupling to cluster-level configuration rather than chart-level values.
- **Supporting initContainers on only high-demand components** — Rejected because inconsistent coverage creates a confusing UX and inevitably leads to requests for the remaining components.
- **A single global `initContainers` field applied to all pods** — Rejected because it lacks per-component granularity; different components have different initialization needs.
- **Relying on `extraVolumes` and sidecar patterns** — Rejected because sidecars cannot solve sequencing requirements where initialization must complete before the main container starts.

## Decision Outcome

Each component's deployment or statefulset template was extended with a per-component `initContainers` field in `values.yaml`, rendered directly into the pod spec when populated. This provides a consistent, granular extension point across all ten deployable components (Identity, Operate, Optimize, Tasklist, Zeebe, Zeebe Gateway, Connectors, and Web Modeler's three sub-components). The field defaults to empty, preserving existing behavior.

### Positive Consequences

- Eliminates the need for users to fork or patch templates for common initialization workflows, reducing upgrade friction.
- Establishes a uniform contract across all components, making the chart predictable and self-documenting.
- Fully backward-compatible — no migration required for existing deployments.

### Negative Consequences

- Increases template maintenance surface (32 files touched), requiring discipline to keep the pattern consistent as new components are added.
- Widens the security surface since users can inject arbitrary containers with elevated privileges; chart consumers must implement values review processes or policy enforcement (e.g., OPA/Kyverno) externally.
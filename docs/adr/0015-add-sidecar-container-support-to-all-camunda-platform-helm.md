# Add sidecar container support to all Camunda platform Helm chart components

- Status: accepted
- Date: 2023-06-08
- Decision-makers: Jesse Simpson

## Context and Problem Statement

The Camunda Platform Helm chart lacked a uniform mechanism for injecting arbitrary sidecar containers into component pods. Users needing sidecars (log shippers, service meshes, vault agents, monitoring agents) had to fork templates or use post-render hooks, creating maintenance burden and divergence from upstream charts. A consistent extensibility point was needed across all deployable components.

## Decision Drivers

- **Operational extensibility**: Enterprise deployments frequently require sidecars for security, observability, and compliance tooling that vary by environment.
- **Chart maintainability**: A uniform pattern across all components (Identity, Operate, Optimize, Tasklist, Zeebe, Zeebe Gateway, Connectors, Web Modeler) reduces template drift and simplifies future changes.
- **Avoid forking**: Users should be able to extend pod specs via values without modifying templates directly.
- **Parity across components**: Inconsistent support (some components having sidecars, others not) creates confusion and forces workarounds.

## Considered Options

- **Pod-level annotations for mutation webhooks only** — Rejected because it forces dependency on external admission controllers and doesn't work in all environments.
- **Selective sidecar support on high-demand components only** — Rejected because partial coverage creates inconsistency and inevitably requires revisiting remaining components.
- **Generic `extraContainers` field at chart root level** — Rejected in favor of per-component configuration, which gives operators fine-grained control over which pods receive which sidecars.

## Decision Outcome

A `sidecars` (or equivalent extra containers) value was added to every component's values schema, and each deployment/statefulset template was updated to render optional sidecar containers. This provides a uniform, per-component extensibility point without altering the core container specs.

### Positive Consequences

- Users can inject sidecars (Vault agent, Filebeat, Envoy, etc.) purely through values overrides, eliminating the need to fork or patch templates.
- Consistent pattern across all 11 deployable units reduces cognitive load for platform teams managing the chart.
- Unit tests for every component validate the sidecar injection path, preventing regressions.

### Negative Consequences

- Increases template complexity across all components; contributors must remember to include sidecar support when adding new components in the future.
- No schema validation on sidecar container specs — malformed values will only fail at Helm render or Kubernetes admission time rather than at lint time.
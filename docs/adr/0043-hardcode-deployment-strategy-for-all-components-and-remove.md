# Hardcode deployment strategy for all components and remove user-configurable strategy option

- Status: accepted (amended by [ADR 0092](0092-allow-opt-in-deployment-strategy-for-components-with-chart-managed-rwo-persistence.md))
- Date: 2024-07-30
- Decision-makers: Hamza Masood

## Context and Problem Statement

The Camunda Platform Helm charts exposed deployment strategy (e.g., `RollingUpdate`, `Recreate`) as a user-configurable value for all stateless components across chart versions 8.2–8.4. This configuration surface provided no practical benefit — the correct strategy is architecturally determined by each component's statefulness — while creating risk of user misconfiguration leading to downtime or split-brain scenarios during rollouts.

## Decision Drivers

- **Operational safety:** Incorrect strategy choices (e.g., `Recreate` on a stateless service behind a load balancer) could cause unnecessary downtime that users wouldn't anticipate.
- **Maintainability:** Reducing the values.yaml API surface decreases documentation burden, test matrix size, and support overhead.
- **Consistency:** All stateless components should behave uniformly; per-component strategy overrides fragmented operational expectations.
- **Opinionated-defaults philosophy:** The chart should encode operational best practices rather than defer every decision to the user.

## Considered Options

- **Keep strategy configurable but improve defaults** — Rejected because no legitimate use case existed for overriding; keeping the option implied it was safe to change.
- **Hardcode only for specific components** — Rejected in favor of uniform treatment across all stateless components to avoid inconsistency and confusion about which components are tunable.
- **Hardcode strategy for all components (chosen)** — Removes the configuration entirely, encoding the architecturally correct strategy directly in templates.

## Decision Outcome

Deployment strategy was hardcoded directly into Helm templates for Identity, Optimize, Tasklist, Zeebe Gateway, Connectors, Console, and Web Modeler across all supported chart versions. The corresponding `strategy` fields were removed from `values.yaml`, eliminating the configuration path entirely. This establishes a precedent that architecturally-determined behaviors should not be exposed as user-tunable knobs.

### Positive Consequences

- Eliminates a class of user misconfiguration that could cause production outages during deployments.
- Reduces the chart's public API surface, making it easier to maintain, document, and evolve without breaking changes.
- Establishes a clear design principle: expose configuration only where users have legitimate, safe choices to make.

### Negative Consequences

- Users with custom deployment strategies (edge case) lose that capability without forking the chart, reducing flexibility for advanced operators.
- The mechanical refactor touched 114 files across multiple chart versions, increasing the risk of merge conflicts with concurrent work and requiring careful review of golden file updates.
# Relocate security configuration from global scope to orchestration component scope

- Status: accepted
- Date: 2025-08-23
- Decision-makers: Hamza Masood

## Context and Problem Statement

The Camunda Platform Helm chart previously placed security-related configuration (authentication, authorization, identity provider settings) under `global.security.*` in the values hierarchy. This global placement created implicit coupling — every component consumed security settings from a shared namespace, making it unclear which component owned the security configuration and preventing independent evolution of security settings per component. As the chart moved toward a unified orchestration layer (8.8+), security configuration needed to be co-located with the orchestration component that actually manages cross-component authentication flows.

## Decision Drivers

- **Ownership clarity**: Security configuration should be owned by the component responsible for enforcing it (orchestration), not scattered in a global namespace that implies shared responsibility
- **Independent component evolution**: Components like Connectors and Web Modeler should reference security config through explicit dependency on orchestration rather than implicit global access
- **Schema maintainability**: Reducing the global values surface area simplifies chart upgrades and reduces the risk of unintended cross-component side effects when security settings change
- **Alignment with unified orchestration architecture**: The 8.8+ chart consolidates coordination concerns under `orchestration.*`, and security is fundamentally a coordination concern

## Considered Options

- **Keep `global.security.*` with aliases** — Maintain backward compatibility by aliasing the old path to the new one. Rejected because it preserves the ambiguity of ownership and increases schema complexity.
- **Per-component security configuration** — Give each component its own independent security block. Rejected because security policy must be consistent across components and this would create drift risk.
- **Move to `orchestration.security.*`** — Selected approach; places security under the component that orchestrates authentication flows across the platform.

## Decision Outcome

Security configuration was relocated from `global.security.*` to `orchestration.security.*`, establishing the orchestration component as the single owner of platform-wide security policy. All dependent components (Connectors, Web Modeler) now reference security settings through the orchestration component's namespace. CI pipelines and integration test values were updated to reflect the new path.

### Positive Consequences

- Clear ownership model: the orchestration component is the authoritative source for security configuration, eliminating ambiguity about where to make security-related changes
- Reduced global namespace pollution: fewer keys in `global.*` means fewer unintended coupling paths between unrelated components
- Better alignment with the unified orchestration architecture introduced in 8.8, making the chart's structural intent more legible to operators

### Negative Consequences

- Breaking change for users who have overridden `global.security.*` in their custom values files — requires migration documentation and potentially a deprecation shim in earlier versions
- Increases the coupling between peripheral components (Connectors, Web Modeler) and the orchestration component's values structure, meaning orchestration schema changes now have a wider blast radius
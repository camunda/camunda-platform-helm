# Support Core Identity OIDC as the authentication mechanism for Camunda 8.8 components

- Status: accepted
- Date: 2025-06-11
- Decision-makers: maryarm

## Context and Problem Statement

Camunda 8 Self-Managed components (Connectors, Core, Identity) historically relied on Keycloak as the sole OIDC provider for inter-component authentication. As the platform evolves toward a "Core Identity" model — where the core orchestration engine can act as its own identity provider — the Helm charts needed to support this alternative OIDC configuration path. This required threading new environment variables, configmaps, and secrets through multiple component templates to allow operators to choose between Keycloak-based and Core Identity-based OIDC.

## Decision Drivers

- **Reduced external dependency**: Eliminating the hard requirement on Keycloak simplifies deployment topologies and reduces operational overhead for teams that adopt Core Identity.
- **Deployment flexibility**: Operators need to choose their identity provider without forking or patching charts, preserving a single canonical chart per version.
- **Backward compatibility**: Existing Keycloak-based deployments must continue to work without configuration changes.
- **Component autonomy**: Each component (Connectors, Core, Identity) should receive only the OIDC configuration it needs, avoiding a monolithic auth configuration block.

## Considered Options

- **Keep Keycloak as the only supported OIDC provider** — rejected because it blocks adoption of the lighter-weight Core Identity model and forces all deployments to run a Keycloak StatefulSet.
- **External OIDC abstraction layer (e.g., a shared auth sidecar)** — rejected due to added runtime complexity and latency; direct environment variable injection is simpler and aligns with existing Helm patterns.
- **Separate chart variant for Core Identity deployments** — rejected to avoid chart proliferation and maintenance burden across versions.

## Decision Outcome

The Helm chart for Camunda Platform 8.8 was extended to support Core Identity OIDC by adding conditional configuration in configmaps, deployment templates, and secrets for Connectors, Core, and Identity components. The values schema was updated to accept Core Identity OIDC parameters alongside the existing Keycloak configuration, with template helpers determining which path to render based on the operator's chosen identity provider.

### Positive Consequences

- Operators can deploy Camunda 8.8 without Keycloak, reducing cluster resource consumption and operational surface area.
- The chart remains a single source of truth — no forks or overlays needed for different identity backends.
- Future identity provider options can follow the same conditional-template pattern established here.

### Negative Consequences

- Increased template complexity in helpers and configmaps — contributors must now reason about two OIDC code paths when modifying authentication-related templates.
- Testing surface area grows (golden files for both Keycloak and Core Identity configurations must be maintained), increasing CI time and review burden.
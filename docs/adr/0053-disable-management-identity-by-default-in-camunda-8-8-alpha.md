# Disable management identity by default in Camunda 8.8 alpha chart

- Status: accepted
- Date: 2025-03-05
- Decision-makers: Sebastian Bathke

## Context and Problem Statement

Camunda 8.8 is transitioning from a standalone Keycloak-backed management Identity service toward built-in authorization handling within the platform core. The alpha Helm chart needed to reflect this architectural direction by changing the default posture, signaling to downstream consumers that the standalone identity component is no longer the primary authorization mechanism going forward.

## Decision Drivers

- **Platform convergence:** Authorization capabilities are being absorbed into the core platform, eliminating the need for a separate identity service in the default deployment topology.
- **Deployment simplicity:** Reducing the default pod count and removing the Keycloak dependency lowers operational complexity for new adopters.
- **Alpha channel purpose:** The alpha release track exists specifically to preview breaking changes early, giving consumers time to adapt before stable release.
- **Migration safety:** The change must be reversible — users still migrating from Keycloak-based identity need a supported opt-in path.

## Considered Options

- **Keep management identity enabled by default and let users opt out** — rejected because it contradicts the alpha channel's purpose of previewing the target architecture and delays ecosystem adaptation.
- **Remove the identity component code entirely** — rejected as premature; preserving the toggleable code path maintains backward compatibility for users mid-migration and avoids a hard break before the stable release.

## Decision Outcome

Management identity is disabled by default in the alpha-8.8 chart via `values.yaml` and schema changes, with all conditional template logic (Connectors, Core, Web Modeler configmaps, identity helpers) respecting the new default. The component remains fully functional when explicitly enabled, preserving an opt-in migration path.

### Positive Consequences

- Default deployments have a smaller footprint with no Keycloak dependency, reducing resource requirements and failure surface.
- Establishes a clear architectural signal that embedded authorization is the forward direction, aligning chart defaults with platform strategy.
- Preserves backward compatibility through a toggle rather than removal, giving teams a controlled migration window.

### Negative Consequences

- Existing alpha-8.8 consumers relying on management identity must update their values overrides to explicitly enable it, creating a breaking change within the alpha channel.
- Both enabled and disabled modes must be tested and maintained in CI and E2E suites until the standalone identity path is fully deprecated, increasing test matrix surface.
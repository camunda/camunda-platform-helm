# Support Keycloak v19 via Bitnami Helm Chart v12.2.0 as Identity Provider Dependency

- Status: accepted
- Date: 2023-01-30
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart depends on Keycloak (via Identity) for authentication and authorization. Keycloak v19 introduced breaking changes in its architecture (the move from WildFly to Quarkus as the underlying runtime), and the upstream Bitnami Keycloak Helm chart v12.2.0 adopted these changes. The platform needed to support this new major version to stay current with security patches and community support, while maintaining backward compatibility during the transition period.

## Decision Drivers

- **Security and supportability**: Older Keycloak versions would eventually lose upstream security patches, making it necessary to track the latest major release.
- **Dependency alignment**: The Bitnami chart ecosystem is the upstream source for Keycloak packaging, and falling behind creates compounding integration debt.
- **Platform compatibility**: Both standard Kubernetes and OpenShift environments must be validated, requiring parallel integration test coverage.
- **Incremental adoption**: Existing deployments on older Keycloak versions must not be disrupted by the chart upgrade.

## Considered Options

- **Fork the Keycloak subchart internally** — Rejected because it would create a long-term maintenance burden and diverge from the community packaging.
- **Wait for full Keycloak v19 stabilization before supporting it** — Rejected because downstream users need early access, and delaying increases the eventual migration effort.
- **Drop support for pre-v19 Keycloak entirely** — Rejected because existing deployments require a transition window; a breaking change without overlap would harm adoption.

## Decision Outcome

The chart templates and helpers were extended to accommodate the structural differences in Keycloak v19 (Quarkus-based configuration paths, changed container ports, different health check endpoints) while preserving compatibility with earlier versions through conditional logic in the template helpers. A dedicated values file for Keycloak v19 was added alongside expanded integration tests covering both Kubernetes and OpenShift.

### Positive Consequences

- **Forward compatibility**: Users can adopt Keycloak v19 immediately, gaining access to the Quarkus runtime's performance and security improvements.
- **Dual-version support**: The template conditional approach allows the chart to serve both legacy and new Keycloak deployments during the transition.
- **Validated across platforms**: Integration test coverage on both K8s and OpenShift ensures the new version works across deployment targets.

### Negative Consequences

- **Increased template complexity**: Conditional logic in helpers to handle two Keycloak architectures adds cognitive load for chart maintainers and risk of subtle rendering bugs.
- **Temporary maintenance burden**: Supporting two Keycloak versions in parallel means doubled test matrix and more surface area for regressions until the older version is deprecated.
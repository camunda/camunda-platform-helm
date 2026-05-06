# Add Connectors as a first-class Helm chart component without Keycloak dependency

- Status: accepted
- Date: 2023-03-08
- Decision-makers: Igor Petrov

## Context and Problem Statement

Camunda 8 Self-Managed needed to support outbound Connectors (HTTP, REST, messaging integrations) as a deployable component within the Helm chart. The challenge was that the full Connectors runtime typically requires Keycloak-based authentication, but many users deploy Camunda without Identity/Keycloak enabled. An architectural decision was needed on whether to gate Connectors behind the authentication stack or introduce it as an independently deployable component.

## Decision Drivers

- **Incremental adoption** — Users should be able to deploy Connectors without being forced to adopt the full Identity/Keycloak stack, lowering the barrier to entry.
- **Component independence** — Each Camunda component should be deployable and scalable independently, following the existing chart pattern (Operate, Tasklist, etc.).
- **Iterative delivery** — Shipping a functional but limited first iteration allows gathering feedback before adding authentication complexity.
- **Chart consistency** — The new component should follow established patterns (dedicated helpers, deployment, service, serviceaccount templates) to maintain chart maintainability.

## Considered Options

- **Require Keycloak as a prerequisite for Connectors** — Rejected because it would couple Connectors availability to Identity, preventing simpler deployment topologies and delaying the feature for users who don't need auth.
- **Deploy Connectors as a sidecar to Zeebe** — Rejected because it would violate component isolation, complicate scaling, and diverge from the established one-deployment-per-component pattern.
- **Wait for full authentication support before releasing** — Rejected in favor of iterative delivery to unblock users and gather real-world feedback.

## Decision Outcome

Connectors was introduced as an independent Helm chart component with its own deployment, service, service account, and helper templates — mirroring the structure of existing components. Authentication was explicitly scoped out of this iteration, making the component functional in non-Keycloak deployments immediately.

### Positive Consequences

- Users can deploy outbound Connectors in minimal Camunda installations without Identity/Keycloak overhead.
- The component follows established chart patterns, making future maintenance and review straightforward for chart contributors.
- Creates a clean extension point for adding authentication in a subsequent iteration without restructuring.

### Negative Consequences

- Connectors deployed without authentication have no access control, which is unsuitable for production multi-tenant environments until the auth layer is added.
- Introduces a temporary divergence where Connectors behaves differently from other components regarding security posture, requiring clear documentation and follow-up work.
# Support OIDC as an alternative identity provider to Keycloak in the Camunda platform Helm chart

- Status: accepted
- Date: 2024-03-01
- Decision-makers: Ben Sheppard

## Context and Problem Statement

The Camunda 8 Self-Managed platform historically required Keycloak as its sole identity provider, tightly coupling the platform's authentication layer to a specific technology. Customers deploying in enterprise environments often have existing OIDC-compliant identity providers (Azure AD, Okta, Auth0) and need to integrate Camunda without deploying a redundant Keycloak instance. The Helm chart needed to support a generic OIDC configuration path for the Identity component while maintaining backward compatibility with the existing Keycloak-based flow.

## Decision Drivers

- **Customer flexibility**: Enterprise adopters require integration with existing corporate identity providers rather than managing a separate Keycloak deployment
- **Reduced operational burden**: Eliminating the mandatory Keycloak dependency reduces the infrastructure footprint and operational complexity for teams that already have an IdP
- **Backward compatibility**: Existing deployments using Keycloak must continue to work without migration effort
- **Standards compliance**: OIDC is the industry-standard protocol, making the platform more portable across identity solutions

## Considered Options

- **Keep Keycloak as the only supported provider** — rejected because it forces unnecessary infrastructure on enterprise customers and limits adoption
- **Abstract identity behind a custom authentication gateway** — rejected due to added complexity and maintenance burden of a bespoke component
- **Support generic OIDC configuration at the Helm chart level** — chosen as the minimal, standards-based approach that delegates provider choice to the operator

## Decision Outcome

The Identity component's Helm chart was extended to accept generic OIDC configuration parameters (issuer URL, client credentials, token endpoints) as an alternative to the bundled Keycloak. Validation constraints were added to ensure operators provide a coherent configuration — either Keycloak or external OIDC — preventing ambiguous hybrid states. Downstream components (Web Modeler) were updated to consume the OIDC configuration through the same environment variable injection pattern.

### Positive Consequences

- Operators can now integrate Camunda with any OIDC-compliant provider, removing the hard dependency on Keycloak
- The platform's deployment footprint is reduced for customers who bring their own IdP
- The configuration surface follows Helm conventions (values.yaml driven), keeping the operational model consistent

### Negative Consequences

- Increased template complexity in Identity's helpers and configmap, with conditional logic branching on the chosen provider type
- Testing surface area expands — both OIDC and Keycloak paths must be validated, and constraint templates must guard against misconfiguration
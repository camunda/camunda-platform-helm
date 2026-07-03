# Allow using an external/existing Keycloak instance instead of the bundled one

- Status: accepted
- Date: 2022-10-24
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart bundled Keycloak as a sub-dependency for Identity's authentication needs. However, many enterprise customers already operate a centrally-managed Keycloak instance (or a compatible OIDC provider) and do not want a second, chart-managed Keycloak deployed into their cluster. The chart needed a way to decouple Identity from the lifecycle of the bundled Keycloak while still providing a seamless default experience for new users.

## Decision Drivers

- Enterprise customers require integration with existing identity infrastructure rather than deploying duplicate auth services
- Maintaining a single Keycloak instance reduces operational overhead, secret sprawl, and attack surface
- The chart must remain simple for greenfield deployments where bundled Keycloak is appropriate
- Identity service configuration (URLs, realms, client credentials) must be flexible enough to point at arbitrary Keycloak endpoints

## Considered Options

- **Remove bundled Keycloak entirely and require external** — Rejected because it raises the barrier to entry for new adopters and breaks the "works out of the box" principle.
- **Proxy/federation between bundled and external Keycloak** — Rejected due to excessive complexity and fragile failure modes in a Helm-managed deployment.
- **Conditional bundled Keycloak with external override values** — Selected as it preserves the default experience while giving operators a clear configuration surface to point at their own instance.

## Decision Outcome

The chart introduces configuration values that allow operators to disable the bundled Keycloak and supply connection details for an external instance. Identity's templates conditionally render environment variables and constraints based on whether an external Keycloak URL is provided, decoupling Identity's runtime configuration from the sub-chart's deployment lifecycle.

### Positive Consequences

- Operators with existing Keycloak infrastructure can adopt Camunda Platform without running a redundant auth service
- Clearer separation of concerns: the chart no longer assumes ownership of the full authentication stack
- Reduces resource consumption and operational burden in production environments that already manage Keycloak at scale

### Negative Consequences

- Increased template complexity with conditional logic paths that must be tested for both bundled and external scenarios
- Users must correctly supply external Keycloak configuration (URL, realm, client secrets), shifting responsibility for auth connectivity to the operator
# Switch Connectors component to Identity-based authentication configuration

- Status: accepted
- Date: 2024-02-13
- Decision-makers: Pavel Kotelevsky

## Context and Problem Statement

The Connectors component in the Camunda Platform Helm chart was configured with direct credentials and endpoint references to Zeebe and Operate, bypassing the centralized Identity service. As the platform matured toward a unified authentication model, Connectors needed to authenticate through Identity like other components, ensuring consistent security boundaries and token-based service-to-service communication.

## Decision Drivers

- **Unified authentication model**: All platform components should authenticate via Identity (OAuth2/OIDC) rather than maintaining bespoke credential configurations
- **Reduced secret sprawl**: Centralizing auth through Identity eliminates per-component credential management and simplifies secret rotation
- **Consistency with platform direction**: Other components (Operate, Tasklist, Optimize) already used Identity; Connectors was an outlier
- **Maintainability**: A single authentication pattern reduces cognitive load for operators and chart maintainers

## Considered Options

- **Keep direct credential configuration** — simpler for standalone deployments but diverges from the platform's Identity-first architecture and creates maintenance burden as Identity evolves
- **Adopt Identity configuration for Zeebe and Operate properties** — aligns Connectors with the established authentication pattern used by all other components

## Decision Outcome

Connectors was reconfigured to obtain Zeebe and Operate connection properties through Identity-managed OAuth2 client credentials, replacing direct endpoint and secret references in the Helm templates. The deployment template and helpers were restructured to inject Identity-aware environment variables, and the component was bumped to version 8.4.4 which supports this authentication mode natively.

### Positive Consequences

- **Consistent security boundary**: All inter-component communication now flows through a single identity provider, simplifying audit and access control
- **Simplified operator experience**: Credential configuration follows the same pattern as every other Camunda component
- **Future-proofing**: Enables Connectors to participate in token refresh, scope restrictions, and identity federation without further chart changes

### Negative Consequences

- **Hard dependency on Identity**: Connectors can no longer function without a running Identity service, removing the option for lightweight standalone deployments
- **Migration burden**: Existing deployments upgrading to this chart version must ensure Identity is configured with appropriate client credentials for Connectors
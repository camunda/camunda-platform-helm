# Integrate Console Self-Managed with Identity for authentication and authorization

- Status: accepted
- Date: 2023-10-16
- Decision-makers: Adam Urban

## Context and Problem Statement

Console Self-Managed needed to authenticate users and authorize access through Camunda's centralized Identity service. Without this integration, Console would either lack proper access control or require its own standalone authentication mechanism, creating inconsistency across the Camunda 8 Self-Managed platform. The architectural challenge was wiring Console into Identity's OIDC client registration and secret management within the Helm chart's templating layer.

## Decision Drivers

- **Platform consistency**: All Camunda 8 components should authenticate through Identity, providing a unified security boundary
- **Operational simplicity**: Operators should manage one identity provider rather than per-component auth configurations
- **Secret lifecycle management**: Client credentials for Console must be generated and injected via Kubernetes secrets, following the same pattern as other components (Operate, Tasklist, Optimize)
- **Upgrade compatibility**: The integration must not break existing deployments during chart upgrades

## Considered Options

- **Standalone authentication for Console** — Rejected because it would fragment the security model and increase operator burden
- **Manual secret provisioning outside the chart** — Rejected because it breaks the self-contained deployment model and adds operational steps
- **Integration via Identity's existing client registration pattern** — Selected, as it follows established conventions used by other components in the chart

## Decision Outcome

Console was integrated with Identity by adding it as a registered OIDC client within Identity's deployment configuration, with a dedicated Kubernetes secret template for Console's client credentials. The Identity chart's helpers and deployment template were extended to provision Console's client alongside existing component clients, and Console's own deployment was updated to consume the injected credentials.

### Positive Consequences

- Console follows the same authentication pattern as Operate, Tasklist, and Optimize — reducing cognitive load for operators and chart maintainers
- Client secret lifecycle is fully managed by Helm, requiring no manual intervention during install or upgrade
- Establishes Console as a first-class citizen in the Self-Managed platform's security architecture

### Negative Consequences

- Tighter coupling between the Console and Identity subcharts — Console cannot be deployed without Identity enabled
- Additional complexity in Identity's deployment template, which now must manage yet another client registration and secret
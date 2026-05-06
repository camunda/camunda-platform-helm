# Centralize OIDC and Microsoft Entra authentication configuration into shared helpers and a unified ConfigMap

- Status: accepted
- Date: 2025-10-10
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

OIDC and Microsoft Entra ID authentication configuration was duplicated across multiple component templates (Connectors, Console, Identity, Optimize, Orchestration, Web Modeler) in the Camunda Platform 8.8 Helm chart. Each component independently rendered auth-related environment variables and config blocks, leading to inconsistencies when provider settings changed and making it error-prone to introduce new identity providers or auth scenarios.

## Decision Drivers

- **Maintainability**: Duplicated auth logic across 6+ components created drift risk and increased the cost of every auth-related change.
- **Consistency**: Users expect uniform OIDC/Entra behavior across all platform components; divergent templates violated that contract.
- **Extensibility**: Adding new auth scenarios (e.g., Entra-specific QA integration tests) required touching every component individually.
- **Template-time validation**: Auth misconfiguration should be caught at `helm template` time, not at runtime.

## Considered Options

- **Per-component auth configuration (status quo)** — rejected due to maintenance burden, inconsistency risk, and high cost of adding new providers.
- **Shared library chart for auth** — rejected as too heavyweight for a single cross-cutting concern; Helm library charts add indirection without proportional benefit here.
- **Runtime configuration via init containers** — rejected because it shifts validation to deploy-time, loses template-time error detection, and adds operational complexity.

## Decision Outcome

Authentication configuration was extracted into a shared `configmap-identity-auth.yaml` and centralized helper functions in `_helpers.tpl` and `_utilz.tpl`. All components now source their OIDC/Entra settings from these shared definitions rather than maintaining independent copies. A compatibility shim (`z_compatibility_helpers.tpl`) preserves backward compatibility for existing deployments during the transition.

### Positive Consequences

- Single source of truth for auth configuration eliminates cross-component drift and reduces the surface area for auth-related bugs.
- Adding new identity providers or auth scenarios (e.g., the new `ingress-qa-entra` test scenario) requires changes in one location rather than six.
- Template-time validation remains intact, catching misconfigurations before deployment reaches the cluster.

### Negative Consequences

- The 39-file atomic change increases merge conflict risk and review burden; incremental migration was sacrificed for consistency guarantees.
- The compatibility shim adds legacy code that must be carried until older deployment patterns are fully deprecated, increasing template complexity in the interim.
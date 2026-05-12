# Standardize existingSecret input interface across all Helm chart components via a common normalization layer

- Status: accepted
- Date: 2025-08-28
- Decision-makers: Balázs

## Context and Problem Statement

The Camunda 8.8 Helm chart exposed inconsistent interfaces for referencing pre-existing Kubernetes Secrets across its components (Connectors, Console, Identity, Optimize, Orchestration, Web Modeler). Different components used different field names, nesting structures, and validation rules for specifying secret name/key pairs. This made the `values.yaml` API confusing for platform operators and error-prone when configuring external secrets for databases, messaging systems, or identity providers.

## Decision Drivers

- **API consistency for operators**: A unified interface reduces cognitive load and configuration errors when deploying across multiple components
- **Maintainability**: Per-component bespoke secret resolution logic created duplicated code paths that diverged over time
- **Backward compatibility**: Existing deployments must not break — the solution must handle legacy input shapes gracefully
- **Validation clarity**: Constraints must enforce consistent input shapes early, with actionable error messages

## Considered Options

- **Per-component incremental fixes** — rejected because it addresses symptoms without solving the systemic inconsistency; components would drift again over time
- **Breaking schema change with a new `existingSecret` contract** — rejected because it would break existing deployments and require coordinated migration across all users
- **Backward-compatible normalization layer in common helpers** — selected; maps both legacy and standardized input shapes to a canonical form at template rendering time

## Decision Outcome

A common normalization helper was introduced in `_helpers.tpl` that resolves secret name/key pairs into a canonical structure regardless of the component or legacy input format used. Each component's deployment template was refactored to consume secrets exclusively through this normalized helper, and `constraints.tpl` was updated to validate inputs against the standardized schema. This establishes a single code path for secret resolution chart-wide.

### Positive Consequences

- Operators now have a single, documented pattern for configuring external secrets across all components, reducing misconfiguration risk
- Future components inherit correct secret handling by using the common helper rather than implementing bespoke logic
- Centralized validation catches malformed secret references at `helm template` time rather than at runtime pod failures

### Negative Consequences

- The 63-file change set increases merge conflict risk with concurrent work on the 8.8 chart and requires careful forward-porting to 8.9/8.10
- Added indirection in common helpers makes template debugging slightly harder — contributors must understand the normalization layer before tracing how a secret reference resolves
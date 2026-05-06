# Centralize auth configuration through a shared Identity ConfigMap rather than per-component variables

- Status: accepted
- Date: 2023-12-19
- Decision-makers: Ben Sheppard

## Context and Problem Statement

Each Camunda platform component (Operate, Optimize, Tasklist, Zeebe Gateway, Connectors) independently managed its own OIDC/auth environment variables through separate values.yaml entries and template helpers. This led to duplicated configuration, inconsistent auth behavior across components, and a maintenance burden when OIDC provider settings changed — requiring updates in multiple places. The upstream Identity service introduced new configuration variables that needed a coherent integration point across the chart.

## Decision Drivers

- **Consistency** — OIDC configuration must behave identically across all components to avoid subtle auth failures in production
- **Maintainability** — A single update point for auth settings reduces the risk of partial migrations and configuration drift
- **Alignment with upstream** — The Identity service now exposes a unified auth configuration surface that the chart should mirror structurally
- **Operational clarity** — Operators need one place to reason about auth configuration rather than hunting across per-component values

## Considered Options

- **Per-component auth variables in values.yaml (status quo)** — Rejected because it required N updates for any OIDC change and allowed configuration drift between components
- **Shared global values without a dedicated ConfigMap** — Likely considered but rejected because it conflates auth config with other globals, making RBAC scoping and change auditing harder
- **Dedicated Identity auth ConfigMap as single source of truth** — Selected for clear separation of concerns and Kubernetes-native configuration injection

## Decision Outcome

Auth configuration was extracted into a shared ConfigMap (`configmap-identity-auth.yaml`) that serves as the single source of truth for OIDC settings across all components. Component deployments now reference this ConfigMap via `envFrom` rather than declaring their own auth environment variables. Client credentials were restructured into per-component secrets that align with the new Identity variable naming conventions.

### Positive Consequences

- Single point of change for OIDC provider configuration eliminates drift between components
- Clearer separation of concerns — auth configuration is an explicit, auditable Kubernetes resource rather than scattered template logic
- Easier onboarding for new components — they simply mount the shared ConfigMap rather than reimplementing auth variable assembly

### Negative Consequences

- **Breaking change for upgrades** — existing `values.yaml` files require migration, adding friction for operators on established deployments
- **Increased blast radius** — a misconfiguration in the identity ConfigMap now affects all components simultaneously rather than being isolated to one service
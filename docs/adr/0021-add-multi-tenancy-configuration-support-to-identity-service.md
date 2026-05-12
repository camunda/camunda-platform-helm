# Add multi-tenancy configuration support to Identity service

- Status: accepted
- Date: 2023-10-06
- Decision-makers: Ben Sheppard

## Context and Problem Statement

The Camunda 8 platform needed to support multi-tenancy as a first-class deployment option, allowing a single platform installation to serve multiple isolated tenants. Identity, as the central authentication and authorization service, required configuration to enable and propagate tenant-awareness across dependent components (Operate, Optimize, Tasklist, Zeebe Gateway). The Helm chart needed to expose this capability through values while enforcing constraints to prevent invalid configurations.

## Decision Drivers

- Platform operators need tenant isolation without deploying separate clusters per customer, reducing infrastructure cost and operational overhead
- Multi-tenancy must be opt-in and backward-compatible — existing single-tenant deployments must not be affected
- Tenant configuration must propagate consistently from Identity to all consuming components via environment variables, avoiding configuration drift
- Validation constraints must prevent deploying multi-tenancy without its prerequisite dependencies (e.g., database backing)

## Considered Options

- **Per-component tenancy configuration** — each service independently configured for multi-tenancy. Rejected due to high risk of configuration inconsistency and operator burden.
- **Global platform-level toggle only** — a single `global.multitenancy.enabled` flag with no Identity-specific configuration. Rejected because Identity requires additional database and secret configuration beyond a simple boolean.
- **Centralized configuration in Identity with propagation via environment variables** — chosen approach, balancing operator ergonomics with component autonomy.

## Decision Outcome

Multi-tenancy configuration was centralized in the Identity subchart, with environment variables propagated to downstream components (Operate, Optimize, Tasklist, Zeebe Gateway) via their deployment templates. A constraint template enforces that multi-tenancy cannot be enabled without a PostgreSQL database configured for Identity, preventing runtime failures from invalid state. The feature is gated behind explicit opt-in through `values.yaml`.

### Positive Consequences

- Single point of configuration for multi-tenancy reduces operator error and ensures consistent tenant-awareness across all platform components
- Constraint validation at deploy-time catches invalid configurations before they reach runtime, improving reliability
- Backward-compatible design means existing deployments require zero migration effort

### Negative Consequences

- Introduces coupling between Identity's configuration schema and downstream component deployments — changes to tenancy model require coordinated template updates across multiple subcharts
- Additional PostgreSQL secret management complexity when multi-tenancy is enabled, increasing the surface area operators must understand
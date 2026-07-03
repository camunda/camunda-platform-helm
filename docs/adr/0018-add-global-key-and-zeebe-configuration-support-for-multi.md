# Add global key and Zeebe configuration support for multi-tenancy

- Status: accepted
- Date: 2023-09-21
- Decision-makers: Abbas Adel Ibraim

## Context and Problem Statement

The Camunda Platform Helm chart needed to support multi-tenancy at the Zeebe gateway level, requiring tenant-aware configuration to be propagated through the deployment stack. Without a global configuration key, each component would need independent tenant configuration, creating inconsistency and configuration drift across the platform's microservices.

## Decision Drivers

- **Consistency across components**: Multi-tenancy configuration must be uniform across all platform services to avoid partial tenant isolation
- **Centralized configuration**: A global key prevents operators from needing to configure tenancy independently per subchart, reducing misconfiguration risk
- **Deployment simplicity**: Operators should enable multi-tenancy with a single configuration toggle rather than coordinating multiple values across subcharts

## Considered Options

- **Per-component tenancy configuration**: Each subchart (Zeebe, Operate, Tasklist) manages its own tenancy settings independently. Rejected due to configuration sprawl and higher risk of inconsistent tenant boundaries.
- **Global values key with subchart propagation**: A single global configuration entry that subcharts reference. Chosen for consistency and operational simplicity.

## Decision Outcome

A global configuration key was introduced in the top-level `values.yaml` that the Zeebe gateway deployment template consumes via environment variables. This establishes the pattern that multi-tenancy is a platform-wide concern configured at the global scope and propagated to individual components, rather than a per-service opt-in feature.

### Positive Consequences

- Operators enable multi-tenancy in one place, reducing configuration errors across distributed services
- Establishes a reusable pattern for future platform-wide feature flags that span multiple subcharts
- Zeebe gateway deployment is now tenant-aware without requiring custom overrides

### Negative Consequences

- Introduces coupling between the global values schema and subchart templates — changes to the global tenancy structure require updates across multiple templates
- Components that may not need tenancy awareness still inherit the global configuration surface area, slightly increasing cognitive load for chart maintainers
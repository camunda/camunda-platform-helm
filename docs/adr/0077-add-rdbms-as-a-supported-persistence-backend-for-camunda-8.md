# Add RDBMS as a supported persistence backend for Camunda 8.9 orchestration components

- Status: accepted
- Date: 2025-12-02
- Decision-makers: Christian Thiel

## Context and Problem Statement

Camunda 8 has historically relied exclusively on Elasticsearch/OpenSearch as its persistence layer. As the platform matures toward enterprise adoption, customers require support for traditional relational databases (PostgreSQL, Oracle) as an alternative persistence backend for orchestration components. The Helm charts needed to be extended to allow configuring RDBMS connectivity, including connection strings, credentials, and database-specific application configuration, without breaking the existing document store paths.

## Decision Drivers

- Enterprise customers require RDBMS support due to existing operational expertise, compliance requirements, and infrastructure constraints that preclude running Elasticsearch clusters
- Deployment flexibility — allowing operators to choose persistence backends based on their infrastructure capabilities and licensing
- Maintainability of the Helm chart — integrating RDBMS configuration into the existing unified orchestration template structure rather than creating parallel deployment paths
- Multi-database support (PostgreSQL and Oracle) requiring an abstraction that accommodates vendor-specific connection parameters

## Considered Options

- **Separate Helm chart per persistence backend** — rejected due to maintenance burden and chart divergence over time
- **External configuration only (no chart-level support)** — rejected because it pushes too much complexity to operators and prevents validation via values schema
- **Unified chart with conditional RDBMS configuration blocks** — chosen approach, keeps a single chart with feature-flag-style configuration

## Decision Outcome

RDBMS support was integrated into the existing unified orchestration chart structure via conditional template helpers and application configuration overlays. The values schema was extended to accept RDBMS connection parameters (including Oracle-specific variants), and the statefulset template conditionally mounts database configuration when RDBMS is enabled. Integration test scenarios for both generic RDBMS and Oracle were added to validate the new persistence paths in CI.

### Positive Consequences

- Single chart supports multiple persistence backends, reducing chart proliferation and simplifying version management
- Schema validation ensures operators receive early feedback on misconfiguration before deployment
- Integration test coverage for RDBMS scenarios prevents regressions as the chart evolves

### Negative Consequences

- Increased template complexity in the orchestration helpers and unified application config, with more conditional logic paths to reason about
- Two database vendors (PostgreSQL, Oracle) with different connection semantics increases the testing matrix and potential for vendor-specific edge cases
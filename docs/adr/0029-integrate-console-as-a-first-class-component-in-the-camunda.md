# Integrate Console as a first-class component in the Camunda Platform Helm chart

- Status: accepted
- Date: 2024-02-19
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart needed to incorporate Console as a fully integrated self-managed component with the same operational standards as existing components (Operate, Tasklist, Optimize, etc.). Previously, Console was either absent or not treated with the same level of observability, health checking, and configuration consistency as other platform components. This created an operational gap where Console deployments lacked probes, metrics, and standardized identity integration.

## Decision Drivers

- **Operational consistency**: Every platform component should have the same observability primitives (health probes, metrics endpoints, service monitors) to enable uniform monitoring and alerting
- **Identity integration correctness**: Components must reference Identity using a consistent base URL pattern to avoid authentication failures across the mesh
- **Component traceability**: Each component needs a unique identifier to support multi-instance deployments and debugging in shared clusters
- **Test coverage parity**: New components must have the same golden-file unit test coverage as established components to prevent regression

## Considered Options

- **Minimal integration (deployment only, no probes/metrics)**: Rejected because it would create operational blind spots and inconsistency with other components
- **External Console deployment managed separately**: Rejected because it fragments the deployment lifecycle and complicates version coordination
- **Full integration with shared helper patterns**: Chosen — leverages existing `_helpers.tpl` conventions and service monitor templates for consistency

## Decision Outcome

Console was integrated as a full component following the established chart patterns: dedicated template directory, helper functions, service account, ingress, service monitors for metrics, and readiness/liveness probes. A unique component ID system was introduced across all components, and the `IDENTITY_BASE` variable was corrected to use the base URL only, fixing a cross-cutting configuration issue that affected Console and WebModeler integration.

### Positive Consequences

- Console now has identical operational visibility (metrics, probes, service monitors) as all other platform components, enabling consistent SRE workflows
- The unique component ID pattern improves traceability in multi-tenant and multi-instance cluster deployments
- Golden file tests ensure Console templates won't silently regress during future chart refactoring

### Negative Consequences

- Adding another full component increases the chart's surface area and maintenance burden (templates, tests, values schema)
- The `IDENTITY_BASE` fix and variable renaming may require users upgrading from pre-alpha Console configurations to update their custom values overrides
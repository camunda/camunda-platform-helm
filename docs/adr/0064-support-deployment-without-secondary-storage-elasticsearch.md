# Support deployment without secondary storage (Elasticsearch/OpenSearch) in Camunda 8.8 Helm chart

- Status: accepted
- Date: 2025-08-06
- Decision-makers: Alexandre Bremard

## Context and Problem Statement

The Camunda 8 Self-Managed Helm chart historically required a secondary storage layer (Elasticsearch or OpenSearch) for all deployments. However, not all deployment scenarios require this — some users run Camunda without Optimize or other components that depend on a document/search store. The chart needed to support a "no secondary storage" deployment mode where neither Elasticsearch nor OpenSearch is provisioned, reducing infrastructure cost and operational complexity for simpler use cases.

## Decision Drivers

- **Deployment flexibility**: Users with minimal Camunda topologies (e.g., Zeebe + Tasklist without Optimize) should not be forced to run a search database they don't need.
- **Infrastructure cost reduction**: Elasticsearch/OpenSearch clusters are resource-intensive; eliminating them where unnecessary reduces cloud spend significantly.
- **Maintainability of constraints**: The chart's template logic must clearly express which components are valid without secondary storage, preventing misconfigurations at deploy time rather than runtime.

## Considered Options

- **Always require secondary storage** — rejected because it forces unnecessary infrastructure overhead on users with simple deployments.
- **Separate chart variant without storage** — rejected due to maintenance burden of divergent chart codebases.
- **Conditional templates within the existing chart** — chosen; keeps a single chart with feature flags controlling component inclusion.

## Decision Outcome

The chart was extended with conditional logic and constraint validation to allow deployments where no secondary storage is configured. Components that depend on secondary storage (Optimize, certain Connectors configurations) are gated by template conditions, and a constraints template enforces valid combinations at render time. The values schema was updated to reflect that storage configuration is optional.

### Positive Consequences

- Users can deploy minimal Camunda topologies without provisioning Elasticsearch or OpenSearch, reducing cost and complexity.
- Constraint validation at template rendering time provides clear, early failure messages for invalid configurations rather than silent runtime errors.
- Single chart codebase is preserved, avoiding fork/variant maintenance overhead.

### Negative Consequences

- Increased conditional complexity in Helm templates makes the chart harder to reason about and test — requires dedicated unit tests for the no-storage path.
- Optimize and other storage-dependent components become implicitly excluded, which may surprise users upgrading from configurations that previously included them by default.
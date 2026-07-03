# Introduce dedicated OpenSearch index prefix configuration independent of Elasticsearch settings

- Status: accepted
- Date: 2025-05-06
- Decision-makers: Daniel Rodriguez

## Context and Problem Statement

Camunda's Helm charts supported both Elasticsearch and OpenSearch as persistence backends, but index prefix configuration was only available through Elasticsearch-specific settings. Users deploying against OpenSearch in multi-tenant or shared-cluster environments had no first-class mechanism to set index name prefixes, forcing them into fragile workarounds that conflated backend-specific concerns.

## Decision Drivers

- **Index isolation in shared clusters:** Multi-tenant deployments on OpenSearch require explicit prefix control to prevent index collisions between tenants or environments.
- **Backend configuration independence:** Elasticsearch and OpenSearch are distinct systems with diverging configuration semantics; coupling their prefix settings creates fragility when only one backend is active.
- **Discoverability and explicit API design:** Platform operators expect persistence configuration to be visible in `values.yaml` rather than hidden behind undocumented `extraConfiguration` overrides.
- **Version-isolation strategy compliance:** The chart repository maintains independent chart versions (8.5, 8.6, 8.7) to allow version-scoped changes without cross-version regression risk.

## Considered Options

- **Reuse the existing Elasticsearch prefix field for both backends** — Rejected because it conflates two distinct backend configurations, creating confusion when both are present and risking incorrect prefix application during backend switches.
- **Require users to configure prefixes via `extraConfiguration` overrides** — Rejected due to poor discoverability, lack of validation, and inconsistent UX compared to other first-class chart settings.
- **Introduce a unified `database.prefix` abstraction** — Rejected as premature; the refactoring cost across multiple chart versions and components was disproportionate to the immediate need, and would require coordinated application-level changes.

## Decision Outcome

A dedicated `opensearch.prefix` configuration path was added to the Helm chart values schema, wired through configmap templates for Operate, Optimize, Tasklist, and Zeebe across chart versions 8.5, 8.6, and 8.7. A constraints template validates prefix usage at render time, providing early feedback before deployment. The change maintains strict version isolation by replicating the implementation independently in each chart version.

### Positive Consequences

- **Clear separation of concerns:** OpenSearch and Elasticsearch configurations are now independent, eliminating accidental cross-contamination of settings when switching backends.
- **Improved multi-tenancy support:** Operators can now isolate indices per deployment in shared OpenSearch clusters using a discoverable, validated configuration option.
- **Template-level validation:** The constraints template catches misconfiguration at `helm template` / `helm install` time rather than at application startup, reducing debugging cycles.

### Negative Consequences

- **Maintenance burden from version duplication:** The same logical change exists in three chart versions independently, meaning future prefix-related fixes must be applied three times.
- **Expanded configuration surface area:** Adding a new values path increases cognitive load for operators and documentation scope, though this is offset by improved explicitness.
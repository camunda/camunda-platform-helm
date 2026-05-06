# Introduce a Camunda Exporter abstraction layer for Zeebe data export

- Status: accepted
- Date: 2024-10-23
- Decision-makers: Daniel Rodriguez

## Context and Problem Statement

Zeebe's data export pipeline was tightly coupled to specific persistence backends (Elasticsearch and OpenSearch), requiring Zeebe to maintain direct knowledge of each downstream store. As the platform evolves toward a unified data layer, this coupling limits architectural flexibility and forces changes in Zeebe whenever a new backend is introduced or an existing one is restructured. A backend-agnostic export mechanism was needed to decouple Zeebe from persistence implementation details.

## Decision Drivers

- **Reduced coupling to persistence backends:** Zeebe should not need to understand the specifics of every data store it exports to; an abstraction layer enables backend evolution without Zeebe changes.
- **Forward compatibility with unified data layer:** The platform is moving toward a Camunda-managed intermediary for process data, requiring a new export path that can target multiple backends transparently.
- **Controlled blast radius for experimental changes:** Architectural shifts in data pipeline topology carry risk and should be validated incrementally before reaching stable releases.
- **Operator flexibility:** Self-managed deployments need the ability to choose and configure how execution data flows to downstream components (Operate, Tasklist, Optimize).

## Considered Options

- **Continue with direct Elasticsearch/OpenSearch exporters only:** Rejected because it perpetuates tight coupling between Zeebe and specific persistence stores, making future backend migrations costly and requiring Zeebe-side changes for each new store.
- **Replace ES/OS exporters entirely with the Camunda Exporter:** Rejected as premature — existing deployments depend on the current exporters, and the new mechanism needs validation before becoming the sole path.
- **Implement the abstraction at the application layer only (no Helm support):** Rejected because operators need chart-level configuration to enable and tune the new exporter in deployed environments.

## Decision Outcome

A new Camunda Exporter configuration was added to the Zeebe configmap in the `camunda-platform-alpha` chart, introducing a backend-agnostic export path that can coexist with existing Elasticsearch/OpenSearch exporters. By scoping this to the alpha chart, the change is validated experimentally before promotion to stable versioned charts (8.8/8.9/8.10).

### Positive Consequences

- **Decoupled data pipeline:** Zeebe no longer needs direct awareness of every persistence backend; the Camunda Exporter acts as an intermediary abstraction.
- **Incremental migration path:** Operators can run both export mechanisms in parallel, enabling gradual transition without downtime or data loss.
- **Platform extensibility:** New downstream consumers or storage backends can be supported without modifying Zeebe's exporter configuration.

### Negative Consequences

- **Increased chart surface area:** An additional exporter configuration path adds complexity to Helm chart maintenance, testing (65 golden files regenerated), and operator documentation.
- **Transitional maintenance burden:** Both old (ES/OS) and new (Camunda) exporter paths must be supported simultaneously until the migration is complete, increasing the number of configuration permutations to validate.
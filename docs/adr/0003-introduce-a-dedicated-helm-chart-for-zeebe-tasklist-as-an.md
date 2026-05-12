# Introduce a dedicated Helm chart for Zeebe Tasklist as an independently deployable component

- Status: accepted
- Date: 2021-08-18
- Decision-makers: Christopher Zell

## Context and Problem Statement

As the Camunda Platform evolves toward a microservices architecture, each component (Zeebe, Operate, Tasklist, etc.) needs independent lifecycle management on Kubernetes. Tasklist requires its own Helm chart to enable teams to deploy, configure, and scale it independently of other platform components, rather than bundling it into a monolithic chart or requiring manual Kubernetes manifest management.

## Decision Drivers

- **Deployment independence**: Each Camunda component should be deployable and upgradeable on its own schedule without forcing coordinated releases across the entire platform.
- **Operational flexibility**: Teams need per-component configuration (resource limits, replicas, ingress rules) without navigating a deeply nested monolithic values file.
- **Composability**: A dedicated chart can be consumed as a subchart in an umbrella chart or deployed standalone, supporting diverse customer topologies.
- **Maintainability**: Isolating Tasklist's Kubernetes resources into its own chart keeps ownership boundaries clear and reduces blast radius of template changes.

## Considered Options

- **Single monolithic Helm chart for the entire platform** — rejected because it couples release cycles of unrelated components and makes partial upgrades impossible.
- **Raw Kubernetes manifests managed via kustomize** — rejected because Helm provides templating, dependency management, and a standardized release lifecycle that aligns with the rest of the Camunda platform charts.
- **Embedding Tasklist templates directly in the Zeebe broker chart** — rejected because Tasklist is a distinct service with its own scaling characteristics and configuration surface.

## Decision Outcome

A new standalone Helm chart (`zeebe-tasklist-helm`) was created with the full set of standard Kubernetes resources: Deployment, Service, ConfigMap, Ingress, and connection tests. This establishes Tasklist as a first-class independently deployable unit within the Camunda Helm ecosystem, following the same structural conventions as existing component charts.

### Positive Consequences

- Tasklist can be versioned, released, and rolled back independently of other Camunda components.
- Configuration is self-contained in a dedicated `values.yaml`, reducing cognitive load for operators managing only Tasklist.
- The chart can be composed into an umbrella chart or deployed standalone, supporting both simple and complex Kubernetes topologies.

### Negative Consequences

- Increases the number of charts to maintain, requiring consistent standards and tooling across the growing chart portfolio.
- Cross-component configuration (e.g., Zeebe gateway endpoints, Elasticsearch connection details) must now be coordinated across chart boundaries rather than shared implicitly.
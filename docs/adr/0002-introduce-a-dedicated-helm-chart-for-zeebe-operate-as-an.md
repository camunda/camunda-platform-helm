# Introduce a dedicated Helm chart for Zeebe Operate as an independently deployable component

- Status: accepted
- Date: 2021-08-18
- Decision-makers: Christopher Zell

## Context and Problem Statement

Zeebe Operate (the workflow monitoring and troubleshooting UI for Zeebe) needed a standardized deployment mechanism for Kubernetes environments. Without a dedicated Helm chart, teams deploying Camunda's Zeebe ecosystem had to manually configure Operate's deployment, service, ingress, and configuration resources, leading to inconsistent deployments and tight coupling with other components' release cycles.

## Decision Drivers

- **Deployment independence**: Allow Operate to be versioned, released, and deployed independently from other Zeebe components (broker, gateway)
- **Operational consistency**: Provide a standardized, repeatable deployment pattern using Helm's templating and lifecycle management
- **Composability**: Enable Operate to participate in umbrella charts while remaining self-sufficient as a standalone deployment
- **Reduced operational burden**: Encapsulate best-practice configuration (resource limits, probes, ingress) as chart defaults

## Considered Options

- **Monolithic chart containing all Zeebe components** — rejected because it couples release cycles and forces full redeployment for single-component changes
- **Raw Kubernetes manifests without Helm** — rejected because it lacks parameterization, lifecycle hooks, and ecosystem tooling integration
- **Kustomize overlays** — rejected in favor of Helm for consistency with the existing Zeebe broker chart and broader Camunda ecosystem conventions

## Decision Outcome

A new Helm chart (`zeebe-operate-helm`) was introduced with the full standard chart structure: configmap for application configuration, deployment for the Operate container, service for internal networking, ingress for external access, and a values.yaml providing sensible defaults. This establishes Operate as a first-class independently deployable unit within the Camunda platform Kubernetes story.

### Positive Consequences

- Teams can upgrade Operate independently without redeploying the Zeebe broker or gateway
- Configuration is centralized in `values.yaml` with clear override points, reducing deployment errors
- The chart can be composed into umbrella charts or deployed standalone, supporting both simple and complex topologies

### Negative Consequences

- Introduces chart maintenance overhead: template changes must be synchronized when cross-component contracts change (e.g., Elasticsearch connection details)
- Operators must now manage version compatibility between the Operate chart and the Zeebe broker chart independently, increasing the coordination surface
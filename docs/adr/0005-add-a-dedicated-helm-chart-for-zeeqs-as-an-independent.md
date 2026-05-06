# Add a dedicated Helm chart for ZeeQS as an independent deployable component

- Status: accepted
- Date: 2021-08-18
- Decision-makers: Christopher Zell

## Context and Problem Statement

ZeeQS (Zeebe Query Service) provides a GraphQL query layer over Zeebe workflow engine data, enabling read-access to process instance state without directly coupling to the Zeebe broker. To deploy ZeeQS alongside Zeebe in Kubernetes environments, a standardized packaging mechanism was needed that aligns with the existing Helm-based deployment strategy used across the Camunda platform components.

## Decision Drivers

- **Deployment independence**: ZeeQS should be deployable, scalable, and configurable independently of the core Zeebe broker chart, allowing teams to adopt it optionally.
- **Consistency with ecosystem**: Other Camunda components (Zeebe, Operate, Tasklist) already use dedicated Helm charts; ZeeQS should follow the same pattern for operator familiarity.
- **Separation of concerns**: Keeping the query service chart separate from the broker chart avoids bloating the core chart with optional read-layer configuration.
- **Operational flexibility**: Independent charts allow distinct upgrade cadences, resource tuning, and replica scaling for the query layer versus the command layer.

## Considered Options

- **Embed ZeeQS as a sub-chart of the Zeebe broker chart** — Rejected because it would force deployment of the query service on all Zeebe users and complicate the broker chart's lifecycle.
- **Deploy ZeeQS via raw Kubernetes manifests without Helm** — Rejected because it would break consistency with the rest of the platform's Helm-based deployment tooling and lose templating benefits.
- **Single monolithic chart for all Camunda components** — Rejected at this stage in favor of composable per-component charts that can later be aggregated via an umbrella chart.

## Decision Outcome

A new standalone Helm chart (`zeebe-zeeqs-helm`) was introduced with its own deployment, service, values, and test templates. This establishes ZeeQS as a first-class, independently deployable component within the Camunda Kubernetes ecosystem, following the same structural conventions as sibling charts.

### Positive Consequences

- Operators can adopt ZeeQS independently without modifying existing Zeebe broker deployments.
- The chart can be versioned and released on its own cadence, decoupling query-layer evolution from broker releases.
- Standard Helm conventions (values overrides, template helpers, connection tests) provide a familiar operational interface.

### Negative Consequences

- Introduces another chart to maintain, increasing the surface area for version compatibility testing across Zeebe and ZeeQS.
- Operators deploying the full platform must now coordinate multiple chart installations (until an umbrella chart aggregates them).
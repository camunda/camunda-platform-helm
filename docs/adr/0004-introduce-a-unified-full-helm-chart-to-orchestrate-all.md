# Introduce a unified "full" Helm chart to orchestrate all Zeebe sub-charts as a single deployable unit

- Status: accepted
- Date: 2021-08-18
- Decision-makers: Christopher Zell

## Context and Problem Statement

Deploying Zeebe and its dependent components (gateway, exporters, Elasticsearch, etc.) required users to manually coordinate multiple independent Helm charts with compatible configurations. This created a fragile deployment experience where version mismatches and configuration drift between components were common failure modes. A single umbrella chart was needed to provide a curated, tested composition of the full Zeebe stack.

## Decision Drivers

- **Deployment simplicity**: Operators need a single `helm install` command to stand up a complete, working Zeebe environment rather than orchestrating multiple releases manually.
- **Version coherence**: Sub-chart versions must be pinned together to guarantee compatibility, reducing the surface area for integration failures.
- **Extensibility**: The umbrella chart pattern allows adding or removing components (e.g., Operate, Tasklist) as sub-chart dependencies without restructuring the deployment model.
- **Alignment with Helm ecosystem conventions**: Umbrella charts are the idiomatic Helm approach for multi-component applications, making the project approachable for Kubernetes practitioners.

## Considered Options

- **Documented manual multi-release deployment** — Rejected because it shifts integration burden onto every user and cannot enforce version compatibility.
- **Kustomize overlays or Operator pattern** — Rejected due to higher implementation complexity and smaller community familiarity at the time; Helm was already the established delivery mechanism for Zeebe components.
- **Single monolithic chart with all templates inline** — Rejected because it would prevent independent versioning and release of sub-components and complicate chart maintenance.

## Decision Outcome

A new `zeebe-full-helm` umbrella chart was introduced containing only dependency declarations, default value overrides, and usage documentation — no application templates of its own. This chart composes existing sub-charts (Zeebe broker, gateway, Elasticsearch) into a single installable unit with coordinated default configuration.

### Positive Consequences

- Users gain a single entry point for deploying the entire Zeebe platform with known-good defaults, dramatically reducing time-to-first-deployment.
- Version compatibility between components is enforced at the chart dependency level, eliminating a class of integration bugs.
- Individual sub-charts remain independently releasable and testable, preserving team autonomy for component owners.

### Negative Consequences

- Adds a coordination layer that must be updated whenever any sub-chart publishes a new version, creating a release sequencing dependency.
- Users who need fine-grained control over individual component lifecycles may find the umbrella chart's opinionated defaults constraining, requiring value overrides that can become complex.
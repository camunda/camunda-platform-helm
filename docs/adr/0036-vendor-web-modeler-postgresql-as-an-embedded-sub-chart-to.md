# Vendor Web Modeler PostgreSQL as an embedded sub-chart to resolve Helm dependency name collisions

- Status: accepted
- Date: 2024-04-10
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda platform Helm chart includes multiple components that depend on PostgreSQL (e.g., Web Modeler, Keycloak). Helm has a known bug (helm/helm#12666) where sub-charts sharing the same chart name cause conflicts during template rendering, making `helm install` fail or produce incorrect manifests. This blocked users from deploying the full platform chart reliably.

## Decision Drivers

- **Deployment reliability** — Users must be able to install the Camunda platform chart without encountering cryptic Helm rendering failures caused by dependency name collisions.
- **Self-contained deployment model** — The chart must ship with all required databases; externalizing PostgreSQL would break the single-chart install experience.
- **Pragmatism over purity** — The upstream Helm fix has been stalled with no resolution timeline; production users cannot wait indefinitely.
- **Maintainability trade-off acceptance** — A vendored dependency is less elegant but immediately unblocks users.

## Considered Options

- **Helm alias in Chart.yaml** — Aliases do not fully resolve the underlying naming conflict across all Helm code paths; collisions persist in certain template rendering scenarios.
- **Wait for upstream Helm fix** — The issue (helm/helm#12666) has been open without resolution; blocking on it was not viable for production users with immediate deployment needs.
- **Externalize the database requirement** — Would break the self-contained deployment model and shift operational burden to users who expect a turnkey install.

## Decision Outcome

The Bitnami PostgreSQL chart used by Web Modeler was extracted from the standard Helm dependency mechanism and embedded directly into the chart repository as `charts/web-modeler-postgresql/`, a renamed vendored copy. This eliminates the name collision at Helm's dependency resolution level by giving the chart a unique name, while preserving the self-contained deployment model.

### Positive Consequences

- **Immediate resolution** — Users can `helm install` the full platform chart without PostgreSQL name collisions, unblocking production deployments.
- **Deployment fidelity** — CI now installs from a locally packaged chart, mirroring production behavior more closely and catching packaging issues earlier.
- **Independence from upstream** — The fix is self-contained and does not depend on the Helm project's release timeline.

### Negative Consequences

- **Maintenance burden** — The vendored chart must be manually synchronized with upstream Bitnami PostgreSQL releases for security patches and features, creating ongoing technical debt.
- **Increased repository surface** — 60+ new files increase chart size, review complexity, and the cognitive load of understanding the dependency tree.
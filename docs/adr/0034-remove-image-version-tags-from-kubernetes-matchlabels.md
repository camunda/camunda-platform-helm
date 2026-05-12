# Remove image version tags from Kubernetes matchLabels selectors to enable non-destructive Helm upgrades

- Status: accepted
- Date: 2024-04-03
- Decision-makers: Jesse Simpson

## Context and Problem Statement

Kubernetes Deployment and StatefulSet resources have immutable `spec.selector.matchLabels` fields after initial creation. The Camunda platform Helm chart included the application image version tag in these selectors, meaning any Helm upgrade that changed the application version would be rejected by the Kubernetes API server. This forced operators to delete and recreate workloads to perform routine version upgrades, causing unnecessary downtime and operational risk.

## Decision Drivers

- **Upgrade reliability:** Helm upgrades must succeed without manual intervention or downtime for production clusters
- **Kubernetes API compliance:** Respecting the immutability constraint on selector fields rather than working around it with destructive operations
- **Operational simplicity:** Reducing the burden on platform operators who expect `helm upgrade` to work idiomatically
- **Consistency across components:** All eight platform components exhibited the same defect, requiring a uniform fix

## Considered Options

- **Remove version from matchLabels, retain in metadata labels (chosen):** Least disruptive; follows Kubernetes and Helm community best practices. Version information remains available for observability without constraining selector immutability.
- **Replace app-version with a fixed chart-version label in selectors:** Would solve immutability but introduces a different coupling — chart version bumps would trigger the same failure.
- **Document manual deletion as the upgrade path:** Shifts the problem to operators, increases downtime risk, and contradicts the purpose of a managed Helm chart.

## Decision Outcome

The `matchLabels` helper templates for all eight Camunda platform components were modified to exclude the image version tag from selector definitions, while retaining version information in standard metadata labels for observability. This is a surgical change to the selector construction logic in each component's `_helpers.tpl`, with all downstream golden files regenerated to reflect the new label set.

### Positive Consequences

- Helm upgrades across application version bumps now succeed without Kubernetes API rejection, enabling zero-downtime rolling updates
- Aligns the chart with Kubernetes best practices and the broader Helm ecosystem convention of stable selectors
- Uniform fix across all components eliminates an entire class of upgrade failure for operators

### Negative Consequences

- **One-time migration burden for existing installations:** Clusters deployed with the old selectors still carry the immutable mismatch; operators must perform a one-time `kubectl delete --cascade=orphan` before their next upgrade to adopt the corrected selectors
- **Reduced commit atomicity:** The bundled yaml.v2→v3 dependency migration mixes infrastructure maintenance with the behavioral fix, making the changeset slightly harder to bisect if regressions occur
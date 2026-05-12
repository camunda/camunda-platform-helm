# Remove standalone migrator component from Camunda 8.9 chart in favor of integrated migration lifecycle

- Status: accepted
- Date: 2025-10-17
- Decision-makers: Jesse Simpson

## Context and Problem Statement

The Camunda 8.8 Helm chart included a standalone "migrator" component — dedicated Kubernetes Jobs for identity and data migrations that ran during upgrades. This introduced operational complexity: separate Jobs to monitor, independent failure modes, sequencing concerns between the migrator and the main application, and additional templates to maintain. In 8.9, the orchestration layer and importer deployment have matured sufficiently to absorb migration responsibilities directly, making the standalone component redundant.

## Decision Drivers

- **Operational simplicity:** Fewer moving parts during upgrades reduces the surface area for failures and the cognitive load on operators monitoring deployments.
- **Maintainability:** Eliminating dedicated migration templates, ConfigMaps, schema entries, and golden files reduces long-term chart maintenance burden.
- **Architectural cohesion:** Migration logic is inherently tied to the application's data model and should live alongside the components that own that data, not in a separate orchestration artifact.
- **Version boundary opportunity:** A new chart version (8.9) provides a clean boundary for breaking changes without requiring a multi-version deprecation cycle.

## Considered Options

- **Keep the migrator as an optional, disabled-by-default component:** Rejected because maintaining dead templates increases confusion for users inspecting the chart and creates ongoing test/maintenance costs for code that should never execute.
- **Deprecate in 8.9, remove in 8.10:** Rejected because the 8.9 version boundary already represents a supported upgrade path where breaking changes are expected, and the two-version deprecation cycle would delay simplification without providing meaningful user benefit.
- **Integrate migration into a Helm hook (pre-upgrade Job) rather than the importer deployment:** Likely considered but rejected in favor of embedding migration logic in the application startup path, which provides better lifecycle integration and avoids hook-ordering issues.

## Decision Outcome

The standalone migrator was fully removed from the 8.9 chart. Migration responsibilities (schema evolution, data transformation, identity migration) are now handled within the orchestration layer's unified application configuration and the importer deployment's startup lifecycle. The 8.8 chart's integration test scenarios and cross-version upgrade taskfile were updated to reflect that the 8.8→8.9 upgrade path no longer exercises standalone migration Jobs.

### Positive Consequences

- Reduced operational complexity: operators monitor fewer Kubernetes resources during upgrades, with migration success/failure observable through standard deployment health checks.
- Lower maintenance burden: ~41 files removed or simplified, eliminating dedicated templates, golden files, schema entries, and CI configuration for a component that no longer exists.
- Clearer architectural boundaries: migration logic is co-located with the data-owning components, improving debuggability and reducing sequencing ambiguity.

### Negative Consequences

- Loss of independent observability: migration failures are no longer surfaced as a discrete Job status; operators must inspect importer/orchestration logs to diagnose migration issues.
- Narrowed CI coverage of legacy paths: the upgrade path from 8.8→8.9 no longer exercises the standalone migrator pattern, reducing confidence in rollback scenarios that might require it.
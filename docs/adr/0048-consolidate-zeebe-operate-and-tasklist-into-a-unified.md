# Consolidate Zeebe, Operate, and Tasklist into a unified Orchestration Core statefulset

- Status: accepted
- Date: 2024-11-17
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

Camunda's process engine (Zeebe), task management (Tasklist), and operations UI (Operate) were deployed as independent components with separate Helm templates, scaling policies, and network configurations. As the upstream product converges these runtimes into a single binary, the Helm chart's component-per-deployment model no longer reflects the actual system architecture, creating operational overhead from managing inter-service communication between processes that now naturally co-locate.

## Decision Drivers

- **Alignment with upstream product direction**: Camunda 8.x+ ships a consolidated runtime; the deployment model should mirror the binary architecture
- **Reduced operational complexity**: Fewer pods, fewer network hops, and a single scaling unit simplify day-2 operations
- **Incremental migration safety**: Need to introduce breaking structural changes without disrupting existing users on stable chart versions
- **Maintainability**: Consolidating duplicated template logic across Zeebe, Operate, and Tasklist reduces long-term chart maintenance burden

## Considered Options

- **Keep separate deployments with tighter inter-service coupling** — Rejected because it diverges from the upstream binary consolidation, maintaining artificial separation that no longer exists at the application layer
- **Merge directly into the stable chart without versioned separation** — Rejected because the breaking changes to values.yaml keys and scaling semantics require an opt-in migration path; a single-chart approach would force all users to migrate simultaneously
- **Introduce Orchestration Core in the existing `camunda-platform` chart behind feature flags** — Rejected (inferred) because feature-flagged template logic increases complexity and testing surface disproportionately compared to a dedicated Alpha chart

## Decision Outcome

A new `templates/core/` directory replaces the Zeebe broker and gateway templates with a unified statefulset, while Operate and Tasklist retain separate template directories but depend on Core rather than Zeebe directly. This consolidation lands exclusively in the `camunda-platform-alpha` chart, establishing it as the forward-looking deployment model while stable charts remain unchanged.

### Positive Consequences

- **Clearer architectural boundaries**: The chart structure now reflects the actual runtime topology — one statefulset for the engine core, satellites for ancillary services
- **Reduced inter-component coupling surface**: Eliminating separate Zeebe gateway services removes a network hop and failure domain for Operate/Tasklist communication with the engine
- **Safe migration path**: Alpha chart isolation allows early adopters to validate while stable users are unaffected, enabling confidence-building before promotion

### Negative Consequences

- **Breaking migration for adopters**: Existing `values.yaml` configurations referencing `zeebe`, `operate`, or `tasklist` top-level keys require rework, creating a migration burden when users move to the new chart
- **Loss of independent scaling granularity**: Zeebe brokers and web-app frontends can no longer be scaled independently, which may be suboptimal for workloads with asymmetric compute/memory profiles across engine and UI layers
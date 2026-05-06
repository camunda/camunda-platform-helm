# Promote Identity from subchart to top-level template directory for structural consistency

- Status: accepted
- Date: 2024-03-16
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart housed most components (Operate, Tasklist, Zeebe, Connectors) as top-level templates, but Identity remained packaged as a nested subchart with its own `Chart.yaml` and sub-dependencies (Keycloak, PostgreSQL). This structural outlier created inconsistencies in values resolution, helper function scoping, and dependency management — making cross-component refactors and shared logic (e.g., unified ingress handling) unnecessarily complex.

## Decision Drivers

- **Naming and structural consistency:** All other components already followed the top-level template pattern; Identity was the sole exception, increasing cognitive overhead for contributors.
- **Simplified template resolution:** Subcharts operate in an isolated scope, requiring explicit value passing and aliasing that complicates global value propagation and helper reuse.
- **Maintainability of cross-component features:** Shared concerns like ingress configuration, auth propagation, and naming conventions were harder to implement uniformly with a subchart boundary in place.
- **Reduced indirection for chart consumers:** A flat, predictable structure makes it easier for operators to understand and override values without navigating nested subchart semantics.

## Considered Options

- **Keep Identity as a subchart with aliased values (status quo):** Avoids breaking changes but perpetuates structural inconsistency and template resolution complexity. Rejected because the long-term maintenance cost outweighed the short-term migration burden.
- **Introduce a shim/alias layer to bridge subchart and top-level patterns:** Would preserve backward compatibility while exposing a unified interface. Rejected as it adds indirection without eliminating the underlying architectural inconsistency.
- **Full promotion to top-level templates (chosen):** Accepts a one-time breaking change in exchange for permanent structural alignment across all components.

## Decision Outcome

Identity was promoted from `charts/camunda-platform/charts/identity/` to `charts/camunda-platform/templates/identity/`, and its sub-dependencies were flattened from nested keys (`identity.keycloak`, `identity.postgresql`) to top-level keys (`identityKeycloak`, `identityPostgresql`). This eliminates the subchart boundary entirely, making Identity architecturally identical to every other component in the chart.

### Positive Consequences

- All components now share a single template scope, enabling straightforward use of global values and shared helpers without subchart bridging.
- Cross-component refactors (unified ingress, shared naming conventions, consistent label propagation) can now be applied uniformly without special-casing Identity.
- Contributors face a single, predictable pattern for all components, reducing onboarding friction and review complexity for future changes.

### Negative Consequences

- **Breaking change for existing users:** Anyone with `identity.keycloak` or `identity.postgresql` in their values files must migrate to the new top-level keys during upgrade, with no backward-compatible fallback.
- **Large initial diff surface:** 61 files touched across templates, tests, and golden files increases short-term review burden and risk of subtle regressions in the migration commit.
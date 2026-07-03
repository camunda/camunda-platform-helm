# Consolidate Operate from subchart into parent Helm chart templates

- Status: accepted
- Date: 2023-10-17
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Operate component was packaged as a Helm subchart within the camunda-platform chart, which introduced scoping barriers for value resolution and helper access. Cross-component references required workarounds due to Helm's subchart isolation model, where each subchart has its own template namespace and limited visibility into sibling charts. This created maintenance overhead and inconsistency, as Connectors and Web Modeler had already been moved to the parent chart's template directory.

## Decision Drivers

- **Simplified cross-component dependency management** — subchart scoping forced complex value passing and helper duplication for inter-service references (Identity, Optimize, Tasklist, Zeebe)
- **Consistency across chart structure** — Connectors and Web Modeler already followed the parent-chart template pattern; Operate was an outlier
- **Maintainability** — a unified template namespace reduces the cognitive overhead of understanding which helpers are available where and eliminates duplicated logic
- **Upgrade path simplicity** — a single chart version governs all components, avoiding subchart version drift

## Considered Options

- **Keep Operate as a subchart** — rejected because it required increasingly complex value passthrough and helper duplication as cross-component integrations grew
- **Extract all components into a Helm library chart pattern** — rejected because it adds abstraction layers without solving the immediate coupling problem and increases the learning curve for contributors
- **Move Operate into parent chart templates (chosen)** — directly solves the scoping issues with minimal conceptual overhead

## Decision Outcome

Operate was relocated from `charts/camunda-platform/charts/operate/` to `charts/camunda-platform/templates/operate/`, placing its templates, helpers, and tests under the parent chart's namespace. This gives Operate direct access to shared helpers in `_helpers.tpl` and a unified values context, eliminating subchart scoping workarounds. References in sibling subcharts (Identity, Optimize, Tasklist, Zeebe, Zeebe Gateway) were updated to use the new helper paths.

### Positive Consequences

- Direct access to shared helpers and global values eliminates duplication and reduces drift between component configurations
- Consistent structural pattern across Operate, Connectors, and Web Modeler lowers contributor onboarding friction
- Simplified upgrade logic — no subchart ownership transitions to manage during Helm releases

### Negative Consequences

- The parent chart grows larger with more templates at a single level, making component boundaries less visually distinct
- Operate can no longer be independently versioned or deployed as a standalone subchart, increasing coupling to the umbrella chart's release cadence
- Users with custom value overrides targeting the subchart's scoped namespace may require migration adjustments during upgrade
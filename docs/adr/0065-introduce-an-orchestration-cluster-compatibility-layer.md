# Introduce an orchestration cluster compatibility layer between 8.7 and 8.8 charts

- Status: accepted
- Date: 2025-08-08
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform 8.8 chart introduced a unified orchestration architecture that diverges structurally from the 8.7 chart's component-based layout. Users upgrading from 8.7 to 8.8 face breaking changes in template helpers, value paths, and resource naming conventions. A compatibility layer is needed to bridge these differences, allowing the 8.8 chart to consume 8.7-era values and naming patterns without requiring users to rewrite their configurations in a single migration step.

## Decision Drivers

- **Upgrade safety**: Users must be able to migrate from 8.7 to 8.8 without data loss or unexpected resource recreation due to naming changes
- **Maintainability**: The compatibility shim should be isolated and removable once the 8.7 upgrade path is no longer supported
- **Backward compatibility**: Existing 8.7 value structures and Helm release names must continue to resolve correctly in the 8.8 chart context
- **Test confidence**: Golden file tests must validate that the compatibility layer produces correct rendered output for both fresh installs and upgrades

## Considered Options

- **Require a clean break with migration documentation only** — rejected because it places too much burden on operators and risks stateful resource (PVC, ConfigMap) orphaning during upgrades
- **Maintain a single chart version supporting both architectures via feature flags** — rejected due to combinatorial complexity in templates and long-term maintenance cost
- **Dedicated compatibility helper templates with deep nesting isolation** — chosen approach, isolating shims in a clearly namespaced template path

## Decision Outcome

A compatibility helpers template (`z_compatibility_helpers.tpl`) was introduced in a deeply nested path to ensure it loads last in Helm's alphabetical template rendering order, allowing it to safely override or augment helpers defined earlier. The values schema, ConfigMap templates, and constraint validations were updated to accept both legacy 8.7 value paths and new 8.8 unified orchestration paths, with the compatibility layer translating between them at render time.

### Positive Consequences

- Operators can upgrade from 8.7 to 8.8 incrementally without rewriting values files in a single release
- The compatibility layer is structurally isolated (deep path nesting + naming convention) making future removal straightforward
- Golden file tests provide regression safety for both fresh-install and upgrade-from-8.7 rendering paths

### Negative Consequences

- Added template complexity and an unconventional file path (`z/1/2/3/4/5/6/7/8/`) that requires team documentation to explain the load-order reasoning
- The compatibility layer must be actively maintained until the 8.7 upgrade path is formally deprecated, creating ongoing test and review burden
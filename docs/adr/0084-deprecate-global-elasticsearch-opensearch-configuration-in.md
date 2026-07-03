# Deprecate global Elasticsearch/OpenSearch configuration in favor of component-scoped settings for Optimize

- Status: accepted
- Date: 2026-03-02
- Decision-makers: Jesse Simpson

## Context and Problem Statement

In the Camunda 8.8+ unified orchestration architecture, most components receive their Elasticsearch/OpenSearch configuration through component-specific or orchestration-level mechanisms. However, the `global.elasticsearch` and `global.opensearch` Helm values keys remained, creating a misleading abstraction — these "global" settings were effectively consumed only by Optimize. This coupling obscured the chart's actual configuration contract and made it harder for operators to reason about which components are affected by which settings.

## Decision Drivers

- **Accuracy of abstraction**: A "global" key that serves a single component is a misleading API surface that confuses chart consumers.
- **Maintainability**: Reducing implicit coupling between unrelated components simplifies future chart evolution and version-scoped changes.
- **Backward compatibility**: Any change to widely-used configuration keys must follow a deprecation cycle to avoid breaking existing deployments.
- **Preparation for removal**: Establishing the copy-down pattern now enables clean removal of the global keys in a future major version.

## Considered Options

- **Remove `global.elasticsearch`/`global.opensearch` entirely** — Rejected because it would constitute a breaking change without giving users a deprecation window to migrate their values files.
- **Maintain the status quo** — Rejected because it perpetuates a misleading abstraction where "global" configuration only serves one component, increasing cognitive load and coupling risk as the chart evolves.
- **Copy global values to Optimize's component-level config with deprecation warnings** — Selected as the balanced approach preserving compatibility while signaling the correct architectural direction.

## Decision Outcome

The global ES/OS configuration keys are formally deprecated and their values are automatically copied into `optimize.elasticsearch`/`optimize.opensearch` via helper template logic. Optimize becomes the explicit, sole owner of its search backend configuration, aligning the chart's public API with the actual dependency graph established by the unified orchestration architecture.

### Positive Consequences

- **Clearer ownership boundaries**: Operators can now see that ES/OS configuration is an Optimize concern, not a platform-wide setting, reducing misconfiguration risk.
- **Reduced implicit coupling**: Future chart changes to search backend configuration need only consider Optimize's scope, not a global surface area.
- **Safe migration path**: Existing deployments continue to work unchanged while deprecation warnings guide users toward the component-scoped pattern.

### Negative Consequences

- **Temporary helper complexity**: The copy-down logic in `_helpers.tpl` introduces indirection that maintainers must understand during the deprecation window until the global keys are fully removed.
- **Large mechanical diff surface**: Updating 40 files (primarily integration test values) increases merge conflict potential and review burden for concurrent chart work.
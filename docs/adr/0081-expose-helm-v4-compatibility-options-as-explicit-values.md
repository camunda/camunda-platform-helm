# Expose Helm v4 compatibility options as explicit values.yaml configuration across all supported chart versions

- Status: accepted
- Date: 2026-02-18
- Decision-makers: Jesse Simpson

## Context and Problem Statement

Helm v4 introduces breaking changes in template rendering behavior (resource policies, lifecycle hooks, field handling) that will affect all Camunda platform deployments. Users running Helm v3 need a migration path that does not require simultaneously upgrading both their Helm client and their chart version. The chart needed a forward-compatibility layer that allows users to opt into Helm v4-compatible behavior incrementally while remaining on Helm v3.

## Decision Drivers

- **Deployment independence:** Users should be able to upgrade their Helm client version independently of their Camunda chart version, avoiding a coordinated "big bang" migration.
- **Backward compatibility:** Existing Helm v3 deployments must not break — new behavior must be opt-in, not default.
- **Version consistency:** All supported chart versions (8.6–8.9) should offer the same migration affordances, so users on older versions are not left behind.
- **Testability:** Explicit configuration values are easier to validate in CI (schema validation, golden tests) than runtime version detection.

## Considered Options

- **Only support Helm v4 in newer chart versions (8.9+)** — Rejected because it forces users to upgrade both chart and Helm client simultaneously, creating a risky coordinated migration.
- **Auto-detect Helm version at render time** — Rejected as fragile, difficult to test deterministically, and prone to subtle behavioral differences between environments.
- **Make Helm v4 behavior the default** — Rejected as a breaking change for existing Helm v3 users who have not yet prepared their deployment pipelines.

## Decision Outcome

Helm v4 compatibility options were exposed as explicit, opt-in fields in `values.yaml` across all supported chart versions (8.6–8.9). Deployment templates for Console, Operate, Optimize, Tasklist, Web Modeler, and Zeebe/Orchestration were updated to conditionally render Helm v4-compatible output based on these new values. Schema validation was added in 8.8/8.9 to enforce correct usage of the new options.

### Positive Consequences

- Users gain a controlled, incremental migration path to Helm v4 without being forced into a version coupling between chart and client.
- Explicit configuration is self-documenting and discoverable via `values.yaml` and schema files, reducing support burden.
- Uniform application across 8.6–8.9 means any supported deployment can be migrated independently, preserving team autonomy over upgrade timelines.

### Negative Consequences

- Backporting to 8.6 increases long-term maintenance surface — four chart versions must carry and maintain these options until older versions reach end-of-life.
- The opt-in model adds configuration burden; users must be aware of and explicitly enable the new options, creating a risk that some deployments miss the migration step entirely.
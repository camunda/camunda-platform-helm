# Deprecate Bitnami subcharts in Camunda Platform Helm chart 8.9

- Status: accepted
- Date: 2026-02-26
- Decision-makers: Balázs

## Context and Problem Statement

The Camunda Platform Helm chart 8.9 relied on Bitnami subcharts (PostgreSQL, Elasticsearch, etc.) for bundled infrastructure dependencies. These transitive dependencies introduced version coupling, unpredictable breaking changes from upstream Bitnami releases, and maintenance burden when Bitnami chart APIs changed between minor versions. A formal deprecation was needed to signal the transition toward externally-managed infrastructure or alternative bundled solutions.

## Decision Drivers

- **Supply chain stability**: Bitnami subchart updates frequently introduced breaking changes in values schema, causing silent deployment failures for users who upgraded without reviewing upstream changelogs.
- **Maintainability**: Keeping compatibility shims for multiple Bitnami chart versions across Camunda chart versions created compounding test and template complexity.
- **Deployment flexibility**: Users increasingly provision infrastructure (databases, search engines) outside the Helm release, making bundled subcharts an unnecessary default rather than a requirement.
- **Clear migration path**: Explicit deprecation with constraint validation gives users advance notice before removal in a future major version.

## Considered Options

- **Remove Bitnami subcharts immediately** — Rejected because it would break existing deployments without a migration window, violating semver expectations within the 8.x line.
- **Continue maintaining without deprecation** — Rejected because it defers the pain indefinitely and signals continued support for a pattern the team intends to abandon.
- **Replace with alternative bundled charts** — Deferred to a future version; deprecation is the necessary prerequisite step.

## Decision Outcome

Bitnami subcharts are formally deprecated in 8.9 via template-level constraint warnings and updated default values. Users enabling the bundled subcharts will receive deprecation notices, guiding them toward external provisioning. The constraints template enforces awareness at render time rather than failing silently.

### Positive Consequences

- Users receive clear, actionable deprecation warnings during `helm template` or `helm install`, reducing surprise when removal occurs.
- Reduces future maintenance scope by establishing that Bitnami subchart compatibility is no longer a first-class concern for new features.
- Opens the path for simpler chart architecture in 9.x without legacy bundled infrastructure.

### Negative Consequences

- Users relying on the convenience of bundled subcharts for development/testing environments must plan migration or accept deprecation warnings.
- Increases short-term documentation and support burden as users ask about recommended alternatives for local development setups.
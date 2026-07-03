# Remove backward compatibility support for legacy Keycloak v16 in Helm chart templates

- Status: accepted
- Date: 2023-04-26
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart maintained conditional logic in its template helpers to support both the legacy built-in Keycloak v16 and the newer Keycloak version. Since Keycloak v16 was no longer the default and had been superseded, this dual-path logic added unnecessary complexity to the chart's template rendering, URL construction, and ingress configuration across multiple components (Connectors, Identity, Operate, Optimize, Tasklist, Web Modeler).

## Decision Drivers

- **Reduced maintenance burden**: Carrying compatibility shims for a deprecated component version increases cognitive load and risk of subtle bugs in template logic.
- **Chart clarity**: New contributors and operators should not need to reason about legacy Keycloak v16 code paths when deploying or customizing the platform.
- **Alignment with supported defaults**: The chart should reflect the currently supported and tested component versions, not historical ones.

## Considered Options

- **Deprecation warning only**: Keep the v16 paths but emit warnings. Rejected because this delays cleanup indefinitely and still requires maintaining untested code paths.
- **Feature flag with opt-in**: Gate v16 support behind an explicit values toggle. Rejected because no users are expected to run the old built-in Keycloak v16 going forward, making the toggle dead configuration.
- **Full removal (chosen)**: Remove all v16-specific conditional logic from helpers, values, and golden test files.

## Decision Outcome

All Keycloak v16 compatibility logic was removed from the Helm chart's template helpers (`_helpers.tpl`), default values, and associated unit test golden files. The chart now assumes the current Keycloak version exclusively, simplifying URL construction and environment variable injection across six downstream component deployments.

### Positive Consequences

- Simpler template logic in `_helpers.tpl` reduces the surface area for misconfiguration and makes future Keycloak-related changes less risky.
- Golden test files reflect a single, well-defined expected output, making test failures easier to diagnose.
- Operators reading `values.yaml` encounter fewer legacy options, reducing confusion during chart customization.

### Negative Consequences

- Any environment still running the old built-in Keycloak v16 with this chart version will break without a migration path; this is an intentional breaking change scoped to an unsupported configuration.
- Contributors reviewing historical PRs or backporting fixes to older chart versions must be aware that v16 logic no longer exists on the main branch.
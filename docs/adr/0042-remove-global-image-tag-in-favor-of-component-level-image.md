# Remove global image tag in favor of component-level image versioning

- Status: accepted
- Date: 2024-06-27
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm charts (8.2, 8.3, 8.4) provided a `global.image.tag` value that, when set, would override the image tag for all components simultaneously. However, Camunda 8 components ship at independent patch versions within a minor release — Operate might be at 8.3.4 while Zeebe is at 8.3.5. The global tag created an implicit coupling that could silently pin all components to a single version, producing incorrect deployments and masking version drift during upgrades.

## Decision Drivers

- **Version independence**: Components within a Camunda minor release follow independent patch cadences and must be deployable at different versions simultaneously.
- **Deployment safety**: A single global override created a class of silent misconfiguration errors that were difficult to detect in CI/CD pipelines.
- **Explicitness over convention**: Self-documenting values files where each component's version is visible reduce cognitive load during upgrades and incident response.
- **Multi-version chart maintenance**: The versioned chart architecture (8.2, 8.3, 8.4 coexisting) requires that each chart accurately reflects its components' actual versions without shared implicit defaults.

## Considered Options

- **Keep global tag as lowest-priority fallback (component-level wins if set)**: This was the prior design. Rejected because even as a fallback, the option's existence encouraged misuse — users and CI pipelines would set it for convenience, inadvertently overriding intentional per-component pinning.
- **Deprecate with a warning but retain for one release cycle**: Likely considered but rejected in favor of a clean removal across supported versions, avoiding prolonged ambiguity about which tag source is authoritative.
- **Remove only from newest chart version**: Rejected because the coupling problem existed equally in 8.2–8.4, and inconsistent behavior across chart versions would compound user confusion.

## Decision Outcome

The `global.image.tag` value was removed entirely from `values.yaml` in charts 8.2, 8.3, and 8.4. All helper templates (`_helpers.tpl`) were updated to resolve image tags exclusively from each component's own configuration block, eliminating the fallback chain. Golden test files were regenerated to validate the new rendering behavior.

### Positive Consequences

- Eliminates an entire class of deployment errors where a global tag silently forces version uniformity across independently-versioned components.
- Each component's version is explicitly declared and visible in values files, improving auditability and reducing surprise during upgrades.
- Aligns the chart's configuration model with Camunda's actual release model, where components evolve at different patch cadences.

### Negative Consequences

- Users who relied on `global.image.tag` for uniform deployments (e.g., dev environments pinned to a single version) must now configure tags per-component, increasing configuration verbosity.
- The 86-file change across three chart versions increases backport maintenance cost and review burden, setting a precedent for large cross-version refactors.
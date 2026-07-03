# Adopt reusable workflow templates with declarative per-version configuration for CI/CD pipelines

- Status: accepted
- Date: 2024-06-10
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The CI/CD pipeline duplicated entire workflow files for each supported Camunda platform version (8.2, 8.3, etc.), requiring changes to be replicated across multiple nearly-identical files. As the number of supported chart versions grew, this duplication became unsustainable — increasing maintenance burden, risk of drift between versions, and the likelihood of errors when propagating fixes.

## Decision Drivers

- **Scalability of version support**: Adding a new platform version should not require duplicating hundreds of lines of workflow definitions
- **Maintainability**: Pipeline logic changes should be made in one place and propagated automatically
- **Per-version customizability**: Different chart versions have legitimately different test configurations and behaviors that must remain expressible
- **Failure isolation vs. DRY trade-off**: Balancing shared logic efficiency against the blast radius of template bugs

## Considered Options

- **Keep per-version workflows** — Simple and self-contained, but creates linear growth in maintenance cost per supported version; rejected due to unsustainable redundancy
- **Single monolithic workflow with conditional logic** — Eliminates duplication but concentrates complexity into brittle conditional branches that are difficult to debug and reason about
- **Reusable templates with declarative config files (chosen)** — Separates shared pipeline logic from version-specific parameters, making new version onboarding a config-only change

## Decision Outcome

The CI/CD pipeline was restructured into reusable workflow templates (`chart-validate-template.yaml`, `test-unit-template.yml`, `test-integration-template.yaml`) that accept parameters resolved dynamically from per-version `ci-test-config.yaml` files. Supporting composite actions (`generate-chart-matrix`, `test-type-vars`, `workflow-vars`) bridge the gap between declarative config and GitHub Actions execution. This establishes a clear boundary: templates own the "how" of pipeline execution while config files own the "what" of per-version behavior.

### Positive Consequences

- Adding a new chart version becomes a config-only change rather than full workflow duplication, reducing onboarding effort from hours to minutes
- Pipeline improvements (new test stages, better caching, matrix strategies) propagate to all versions through a single template change
- Declarative config files serve as documentation of each version's CI characteristics, improving discoverability

### Negative Consequences

- Increased indirection when debugging CI failures — engineers must trace through templates, matrix generation actions, and config files rather than reading a single self-contained workflow
- Shared template logic creates correlated failure risk — a bug in a template can break all supported versions simultaneously rather than being isolated to one
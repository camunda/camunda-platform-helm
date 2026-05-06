# Introduce a standalone alpha release channel for the Camunda Platform Helm chart

- Status: accepted
- Date: 2024-06-19
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart repository maintains versioned stable charts (8.8, 8.9, 8.10), but lacked a persistent, installable artifact for bleeding-edge changes. Breaking or experimental template changes had no safe landing zone — they either blocked stable releases or lived in ephemeral feature branches that couldn't be consumed by CI or early adopters. A dedicated channel was needed to decouple unstable iteration from the stable release lines.

## Decision Drivers

- **Release independence**: Chart maintainers need to land breaking changes without gating on a numbered release cycle or risking stable consumers.
- **Continuous validation**: Nightly and snapshot CI pipelines require a persistent, deployable chart to exercise new templates before promotion.
- **Explicit opt-in**: Early adopters and internal testing must consciously choose the unstable channel, preventing accidental production use of unvalidated changes.
- **Reduced release coupling**: Versioned charts should remain frozen for patch-only changes; experimental work should not create merge pressure on stable branches.

## Considered Options

- **Feature branches only** — Rejected because they lack a persistent, installable Helm artifact. CI cannot continuously deploy and test a moving branch target without custom tooling.
- **A `latest` or `dev` tag on the existing chart** — Rejected because it conflates stable and unstable in the same Helm repository entry, risking accidental production consumption and complicating rollback semantics.
- **Separate repository for alpha** — Rejected due to maintenance burden, divergence risk, and the friction of cross-repo template synchronisation.

## Decision Outcome

A full standalone `camunda-platform-alpha` chart directory was introduced, mirroring the complete structure of versioned charts (templates, helpers, Go test modules, OpenShift support). CI pipelines were extended to include snapshot releases and integration testing for this alpha channel. The chart operates as the first stage in a promotion pipeline: changes land in alpha, are validated via nightly CI, then are promoted to the next numbered release.

### Positive Consequences

- **Clear lifecycle boundary**: Experimental and breaking changes have a well-defined home, reducing risk to stable consumers and eliminating ambiguity about chart maturity.
- **Continuous integration coverage**: Nightly pipelines can exercise bleeding-edge templates against real clusters, catching regressions before they reach versioned charts.
- **Deployment independence**: The alpha chart can evolve at its own pace — different dependency versions, new components, structural refactors — without coordinating with stable patch cadences.

### Negative Consequences

- **Template duplication**: The alpha chart is a full copy rather than a shared-template or symlink approach, meaning drift between alpha and versioned charts must be managed manually during promotion. This trades DRY principles for release independence.
- **Increased CI cost**: Adding another chart to the release and test matrix increases pipeline runtime, resource consumption, and the surface area for flaky failures in nightly workflows.
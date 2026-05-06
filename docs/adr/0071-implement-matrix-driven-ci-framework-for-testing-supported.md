# Implement matrix-driven CI framework for testing supported minor version upgrade paths

- Status: accepted
- Date: 2025-10-01
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

Camunda 8 Self-Managed supports in-place minor version upgrades (8.2→8.3→…→8.8), but CI only validated fresh installations. Upgrade regressions — schema migrations, breaking value changes, deprecated Helm fields — were undetected until customers encountered them in production. A systematic, automated approach to validating every supported N→N+1 upgrade path was needed.

## Decision Drivers

- **Upgrade reliability for customers** — self-managed users perform in-place upgrades and expect each supported minor hop to work without manual intervention
- **Regression detection at scale** — with 6+ supported minor versions, manual upgrade testing does not scale and is error-prone
- **Per-version independence** — each chart version has unique migration prerequisites (CRD changes, value deprecations, schema shifts) that require version-specific setup logic
- **CI maintainability** — upgrade test orchestration must integrate with the existing matrix-based test infrastructure rather than creating a parallel system

## Considered Options

- **Manual upgrade testing per release** — rejected because it doesn't scale across the supported version matrix and is prone to human error
- **Single monolithic upgrade script covering all versions** — rejected because version-specific prerequisites make a one-size-fits-all script brittle and hard to maintain
- **Testing only latest→latest upgrade** — rejected because intermediate version transitions (e.g., 8.4→8.5) have distinct migration behaviors that would go untested

## Decision Outcome

The chosen approach introduces per-version lifecycle hook scripts (`pre-upgrade-minor.sh`, `pre-upgrade-patch.sh`) alongside a matrix-driven `ci-test-config.yaml` that declares upgrade scenarios as first-class test entries. Workflow templates were refactored to orchestrate the install-old → upgrade → validate cycle. This allows each chart version to independently define its upgrade prerequisites while the CI matrix handles execution and parallelism.

### Positive Consequences

- Every supported minor upgrade path is validated automatically on every chart change, catching regressions before release
- Version-specific upgrade logic is encapsulated in isolated scripts, allowing teams to modify one version's prerequisites without affecting others
- The matrix-based approach scales linearly — adding a new chart version requires only adding its scenario scripts and a config entry

### Negative Consequences

- Significant maintenance surface (96 files) — each new chart version requires authoring upgrade scripts, increasing onboarding complexity for contributors
- CI runtime increases substantially due to multi-cycle deployments (install previous version → upgrade → validate) for each matrix entry
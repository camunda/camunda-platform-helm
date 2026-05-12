# Adopt directory-based multi-version chart structure for independent version lifecycle management

- Status: accepted
- Date: 2024-06-06
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart repository maintained a single-chart layout where version management relied on Git tags or branches. As the number of concurrently supported minor versions grew (8.2, 8.3, etc.), templates and component configurations diverged significantly between versions, making it untenable to maintain all versions in a single chart definition. The team needed a structure that allowed parallel, independent evolution of each supported version without cross-contamination or coordination overhead.

## Decision Drivers

- **Parallel maintainability**: Multiple minor versions with diverging templates must be maintained simultaneously on a single branch without merge conflicts or conditional complexity
- **CI isolation**: Each chart version requires independent linting, unit testing, and integration testing without risk of one version's changes breaking another's pipeline
- **Contributor clarity**: Clear ownership boundaries per version reduce cognitive load and review scope for contributors working on version-specific changes
- **Deployment independence**: Release cadence for each version should be decoupled, allowing hotfixes to older versions without gating on newer version readiness

## Considered Options

- **Git branches per version** — Rejected because cross-version CI workflow changes would need cherry-picking across branches, workflow files would be duplicated, and holistic repository-level improvements become coordination-heavy
- **Single chart with conditional templates** — Rejected due to complexity explosion as versions diverge; Helm's templating language lacks the abstraction power to cleanly handle version-specific logic at scale
- **Monorepo with chart aliases or symlinks** — Rejected because Helm tooling (helm package, helm lint, dependency resolution) does not reliably handle symlinked chart structures

## Decision Outcome

The repository was restructured into a `charts/camunda-platform-8.x/` directory layout where each supported minor version is a fully self-contained Helm chart with its own templates, values, dependencies, and test fixtures. CI workflows were parameterized with a `chartPath` input to enable version-aware pipeline execution, and image version tracking was migrated from GitHub Release scraping to Docker Hub registry queries for reliability.

### Positive Consequences

- Each version can evolve independently — template changes, dependency bumps, and value schema modifications are scoped and cannot regress other versions
- CI pipelines are version-isolated, enabling targeted test execution and faster feedback loops for version-specific PRs
- The structure naturally documents which versions are actively maintained (directories present on `main`) and simplifies end-of-life removal

### Negative Consequences

- One-time migration cost of 843 files changed, creating a large initial churn commit that complicates git blame and historical bisection
- Repository size grows as all supported versions coexist on `main`, and shared patterns across versions can drift without explicit enforcement mechanisms (linting rules, shared libraries, or code generation)
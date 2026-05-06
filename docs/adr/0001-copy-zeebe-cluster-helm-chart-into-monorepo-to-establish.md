# Copy zeebe-cluster-helm chart into monorepo to establish unified Helm chart CI/CD pipeline

- Status: accepted
- Date: 2021-08-16
- Decision-makers: Christopher Zell

## Context and Problem Statement

The Camunda platform consists of multiple components (Zeebe, Operate, Tasklist, etc.) each with their own Helm charts maintained in separate repositories. To enable a unified deployment experience and consistent CI/CD practices (linting, testing, releasing), the team needed to consolidate these charts into a single monorepo structure. The zeebe-cluster-helm chart was chosen as the first candidate to validate the build, test, and release pipeline in the new repository structure.

## Decision Drivers

- **Deployment cohesion**: Users deploying Camunda Self-Managed need a single source of truth for all related Helm charts rather than tracking multiple repositories
- **CI/CD consistency**: A shared pipeline for linting, testing, and releasing charts reduces maintenance burden and ensures uniform quality standards
- **Release coordination**: Co-locating charts enables synchronized versioning and cross-chart dependency management
- **Incremental migration**: Starting with one chart allows validating infrastructure before committing to a full migration

## Considered Options

- **Keep charts in separate repositories** — rejected because it fragments CI/CD tooling, complicates cross-chart testing, and burdens users with multiple Helm repo sources
- **Build a meta-repository with git submodules** — rejected due to submodule complexity and the inability to share CI workflows cleanly
- **Monorepo with all charts copied at once** — rejected in favor of an incremental approach to reduce risk and validate the pipeline first

## Decision Outcome

The zeebe-cluster-helm chart was copied wholesale into a `charts/` directory within the new monorepo as a proof-of-concept for the unified build, test, lint, and release pipeline. This established the directory convention (`charts/<chart-name>/`) and validated that existing chart templates, dependencies, and values could function correctly within the new repository structure.

### Positive Consequences

- Established the monorepo directory convention that subsequent charts would follow
- Validated CI/CD pipeline mechanics (Helm lint, template tests, release automation) with a real, production-grade chart
- Reduced risk for migrating remaining charts by proving the approach incrementally

### Negative Consequences

- Temporary divergence between the source-of-truth zeebe-cluster-helm repository and this copy until migration completes
- Initial duplication of chart code across repositories during the transition period
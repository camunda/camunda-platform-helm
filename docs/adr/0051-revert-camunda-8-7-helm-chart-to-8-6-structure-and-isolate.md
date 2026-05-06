# Revert Camunda 8.7 Helm chart to 8.6 structure and isolate architectural refactoring in alpha-8.8

- Status: accepted
- Date: 2025-01-21
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart for version 8.7 had prematurely adopted a unified `orchestration/` template structure intended for the next major iteration. This created stability risk for the 8.7 release line by coupling it to an immature architectural pattern. A decision was needed on whether to keep 8.7 on the new structure or revert it to the proven 8.6 layout, isolating structural innovation in the alpha track.

## Decision Drivers

- **Release stability:** The 8.7 chart serves production users and must ship with a battle-tested, well-understood layout
- **Backport simplicity:** Security and bug fixes need to flow easily between 8.6 and 8.7; shared structure makes this trivial
- **Innovation isolation:** Structural refactors carry integration risk that should be absorbed in alpha/pre-release channels, not stable releases
- **Versioning discipline:** Each release line should only adopt structural changes that have been validated in a prior alpha cycle

## Considered Options

- **Keep 8.7 on the new unified structure and backport fixes** — rejected because it increases maintenance burden and exposes production users to an unproven layout
- **Maintain both old and new structures within 8.7** — rejected due to excessive complexity and unclear ownership boundaries
- **Revert 8.7 to 8.6 structure; establish alpha-8.8 with the component-per-directory architecture** — chosen as it cleanly separates stable from experimental

## Decision Outcome

The 8.7 chart was reverted to the 8.6-era layout (flat template structure), and the alpha-8.8 chart was established with the component-per-directory pattern (connectors/, console/, core/, identity/, optimize/, etc.). This creates a clear architectural boundary: stable releases (8.6, 8.7) share one structure, while the next generation matures independently in alpha.

### Positive Consequences

- Reduces risk for 8.7 production deployments by shipping only proven template patterns
- Simplifies cross-version maintenance since 8.6 and 8.7 share identical structural conventions, making backports mechanical
- Establishes a clear precedent that structural refactors are introduced in alpha tracks and validated before GA promotion

### Negative Consequences

- Two coexisting structural patterns across maintained chart versions increases cognitive load for contributors working across versions
- The 436-file diff is difficult to review atomically, and the alpha-8.8 component-per-directory layout may itself be refactored before GA, meaning this intermediate structure could be short-lived
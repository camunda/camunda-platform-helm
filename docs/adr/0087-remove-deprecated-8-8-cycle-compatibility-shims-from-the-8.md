# Remove deprecated 8.8-cycle compatibility shims from the 8.10 Helm chart

- Status: accepted
- Date: 2026-03-13
- Decision-makers: Balázs

## Context and Problem Statement

The Camunda Platform Helm charts carry forward compatibility shims (deprecated value aliases, constraint templates, helper functions) so that users upgrading from version N-1 receive deprecation warnings rather than hard failures. By the time the 8.10 chart became the active development target, the 8.8-cycle shims had exceeded their two-minor-version support window and were adding significant template complexity without serving current users.

## Decision Drivers

- **Template maintainability:** Accumulated shims made the compatibility helpers layer (`z_compatibility_helpers.tpl`) increasingly difficult to reason about and a source of subtle rendering bugs.
- **Adherence to N-1 upgrade policy:** Camunda officially supports single-minor-version upgrades only; retaining N-2 shims contradicts this contract and sets false expectations.
- **Contributor onboarding:** Reducing dead code paths lowers the cognitive load for engineers working on the 8.10 chart.
- **CI signal relevance:** Testing backward-compatibility scenarios that no longer apply wastes resources and obscures meaningful failures.

## Considered Options

- **Keep shims indefinitely for cautious users** — Rejected because it accumulates technical debt exponentially across versions and makes template logic brittle, increasing the risk of correctness bugs in active code paths.
- **Automate shim removal via tooling** — Rejected as over-engineering for a periodic manual task that occurs once per minor release cycle and benefits from human review of each removal.
- **Partial removal (keep only value aliases, drop constraints)** — Rejected because half-measures still leave confusing code paths and don't fully simplify the schema or test surface.

## Decision Outcome

All 8.8-cycle deprecated value paths, constraint templates, and compatibility helper logic were removed from the 8.10 chart. The values schema and defaults were tightened to reflect only 8.10-valid structure, and CI workflows were updated to stop exercising backward-compatibility scenarios that no longer apply. Unit and integration test values were migrated to use canonical paths exclusively.

### Positive Consequences

- Template rendering logic is simpler and easier to audit, reducing the probability of subtle bugs in Helm output.
- The values schema now serves as accurate documentation of the 8.10 contract, eliminating ambiguity about which paths are supported.
- CI pipelines test only relevant scenarios, improving signal-to-noise ratio and reducing execution time.

### Negative Consequences

- Users attempting unsupported skip-version upgrades (8.8 → 8.10 direct) will encounter hard failures with no deprecation guidance; this is an accepted trade-off per the N-1 policy.
- The large deletion-heavy diff (46 files) increases short-term merge-conflict risk with parallel 8.10 development, though conflicts are straightforward to resolve given the nature of the changes.
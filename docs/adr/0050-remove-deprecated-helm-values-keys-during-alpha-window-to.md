
# Remove deprecated Helm values keys during alpha window to eliminate compatibility debt

- Status: accepted
- Date: 2024-12-13
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda Platform Helm chart had accumulated deprecated values keys that required backward-compatibility shims (`z_compatibility_helpers.tpl`) to map old key names to new ones. This compatibility layer added template complexity, confused users reading `values.yaml`, and increased maintenance burden across templates, secrets, and test infrastructure. The alpha phase (8.7 alpha 3) presented a narrow window to make breaking changes before GA locks in backward-compatibility guarantees.

## Decision Drivers

- **Technical debt reduction**: Compatibility mapping logic in templates adds cognitive load and bug surface area with every new feature
- **Alpha-phase timing**: Breaking changes are acceptable pre-GA but become exponentially more costly afterward
- **Naming consistency**: Canonical key names improve discoverability and reduce support burden for users configuring the chart
- **Atomicity of change**: Coordinated removal across templates, secrets, and tests prevents intermediate broken states in CI

## Considered Options

- **Keep deprecated keys with deprecation warnings**: Lower migration risk for users but perpetuates template complexity indefinitely and signals that deprecated keys are safe to rely on
- **Gradual removal across multiple alpha releases**: Reduces per-release blast radius but extends the period of inconsistency and requires tracking which keys have been removed in which release
- **Provide automated migration tooling (e.g., a values transformation script)**: Eases user transition but adds maintenance overhead for a one-time migration that affects only alpha users

## Decision Outcome

All deprecated values keys were removed or renamed in a single atomic commit, eliminating the compatibility shim layer and establishing clean canonical key names across the chart. Secret templates, helper functions, and constraint validators were updated to reference only the new key paths, with all golden files and unit tests regenerated to match.

### Positive Consequences

- **Reduced template complexity**: Removal of compatibility helpers eliminates an entire class of indirection in template rendering, making the chart easier to reason about and extend
- **Clearer API surface**: Users and documentation now reference a single canonical set of values keys without ambiguity about which name to use
- **Lower maintenance burden**: Future template changes no longer need to consider or test both old and new key paths

### Negative Consequences

- **Breaking change for alpha users**: Anyone upgrading from earlier alpha releases must manually update their values files, with no automated migration path provided
- **Large review surface (39 files)**: The all-at-once approach trades reviewability for atomicity, making it harder to verify individual changes in isolation during code review
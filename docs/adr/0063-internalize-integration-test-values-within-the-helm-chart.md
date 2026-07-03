# Internalize integration test values within the Helm chart repository to eliminate cross-repo push dependency

- Status: accepted
- Date: 2025-08-06
- Decision-makers: Eamonn Moloney

## Context and Problem Statement

Integration test configuration values for QA scenarios were previously sourced by being pushed from an external QA repository into the Helm chart CI pipelines. This created an implicit cross-repository coupling where the Helm chart's CI correctness depended on an external repo's push mechanism being functional and in sync. The architecture needed to shift ownership of these test values to the repository that actually consumes them, enabling self-contained CI execution.

## Decision Drivers

- **Repository autonomy**: The Helm chart repo should be able to run its full integration test suite without depending on external repositories pushing configuration at runtime
- **Change traceability**: Test value changes should be versioned alongside the chart versions they apply to, making it clear which configuration belongs to which release
- **CI reliability**: Removing the cross-repo push dependency eliminates a fragile coordination point that could cause silent test failures
- **Multi-version maintainability**: With charts versioned 8.4 through 8.8, each version needs its own test values co-located with its chart definition

## Considered Options

- **Keep values pushed from QA repo** — Rejected because it couples CI reliability to an external system and makes it impossible to test chart changes in isolation
- **Shared values in a single location within this repo** — Rejected because different chart versions may need divergent test configurations as the platform evolves
- **Per-version values co-located with each chart** — Selected as it provides version-specific ownership and aligns with the existing per-version chart structure

## Decision Outcome

Integration test values for all QA scenarios (Elasticsearch, OpenSearch, multi-tenancy, role-based access) were internalized into each versioned chart directory under `test/integration/scenarios/chart-full-setup/`. The CI workflows and runner scripts were updated to source these values locally rather than expecting them from an external push mechanism, making the Helm chart repository fully self-contained for integration testing.

### Positive Consequences

- Each chart version owns its complete test configuration, enabling independent evolution without cross-version interference
- CI pipelines are now hermetic — a single `git clone` provides everything needed to run the full test suite
- Contributors can modify test scenarios alongside chart changes in a single PR, improving review quality

### Negative Consequences

- Configuration duplication across chart versions (8.4–8.8) increases maintenance surface when shared values need updating
- Any future coordination with the QA repo requires explicit synchronization rather than the previously implicit push mechanism
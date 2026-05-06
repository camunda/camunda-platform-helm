# Integrate Azure Blob Storage as a document handling backend in the Helm chart

- Status: accepted
- Date: 2026-03-19
- Decision-makers: Rei

## Context and Problem Statement

Camunda's document handling capability previously supported only a subset of storage backends. Customers deploying on Azure needed native Azure Blob Storage integration for document persistence without resorting to workarounds or S3-compatible shims. The Helm chart needed to expose Azure-specific configuration across both the orchestration (Zeebe) and connectors components in a consistent, schema-validated manner across chart versions 8.9 and 8.10.

## Decision Drivers

- **Cloud-native parity**: Azure customers expect first-class support equivalent to AWS/GCP storage backends, reducing operational friction in enterprise deployments.
- **Consistency across components**: Document handling configuration must be propagated uniformly to both orchestration (statefulset) and connectors (deployment) to avoid runtime mismatches.
- **Schema-driven validation**: All new configuration must be captured in `values.schema.json` to provide early feedback on misconfiguration during `helm install/upgrade`.
- **Multi-version maintainability**: Changes must land in both 8.9 and 8.10 charts to support customers on current and next minor versions.

## Considered Options

- **S3-compatible proxy for Azure Blob Storage** — Rejected because it adds an extra network hop, increases operational complexity, and obscures Azure-native features (managed identity, SAS tokens).
- **External configuration only (no chart changes)** — Rejected because it shifts complexity to the user via raw `extraEnv` overrides, bypassing schema validation and making upgrades fragile.

## Decision Outcome

Azure Blob Storage was added as a first-class document handling storage backend in the Helm chart. Configuration is surfaced in `values.yaml`, validated via `values.schema.json`, and rendered into application config files for both the orchestration statefulset and the connectors deployment through shared helpers in `_helpers.tpl`.

### Positive Consequences

- Azure customers can configure document storage declaratively with full schema validation, reducing misconfiguration risk.
- Shared helper templates ensure orchestration and connectors receive identical Azure configuration, eliminating drift between components.
- Landing the change in both 8.9 and 8.10 provides a consistent upgrade path without backport pressure later.

### Negative Consequences

- Additional conditional logic in `_helpers.tpl` and application config templates increases template complexity and cognitive load for chart maintainers.
- Parallel changes across two chart versions create a maintenance surface that must be kept in sync until 8.9 reaches end-of-life.
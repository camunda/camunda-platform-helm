# Integrate Azure Blob Storage as a document handling backend in the Helm chart

- Status: accepted
- Date: 2026-03-19
- Decision-makers: Rei, Distribution team

## Context and Problem Statement

Camunda's document handling capability previously supported only a subset of storage backends. Customers deploying on Azure needed native Azure Blob Storage integration for document persistence without resorting to workarounds or S3-compatible shims. The Helm chart needed to expose Azure-specific configuration in a consistent, schema-validated manner across chart versions 8.9 and 8.10.

The Orchestration Cluster is the only component that reads Azure document-store configuration. Connectors does not access a document store directly: it sends documents through the Orchestration Cluster REST API. The application lines bind the configuration differently:

- **8.9** — the application reads `DOCUMENT_STORE_<ID>_*` environment variables. The Azure provider ships from application version 8.9.18.
- **8.10** — the application binds the map-keyed property `camunda.document.azure.<storeId>.*`.

### Applicability by version

Charts 8.9 and 8.10. `global.documentStore.type.azure` does not exist in charts 8.7 and 8.8.

## Decision Drivers

- **Cloud-native parity**: Azure customers expect first-class support equivalent to AWS/GCP storage backends, reducing operational friction in enterprise deployments.
- **Bind what the application reads**: Configuration that the pinned application cannot bind fails silently, so each setting must render at the path that the application line binds.
- **Schema-driven validation**: All new configuration must be captured in `values.schema.json` to provide early feedback on misconfiguration during `helm install/upgrade`.
- **Multi-version maintainability**: Changes must land in both 8.9 and 8.10 charts to support customers on current and next minor versions.

## Considered Options

- **S3-compatible proxy for Azure Blob Storage** — Rejected because it adds an extra network hop, increases operational complexity, and obscures Azure-native features (managed identity, SAS tokens).
- **External configuration only (no chart changes)** — Rejected because it shifts complexity to the user via raw `extraEnv` overrides, bypassing schema validation and making upgrades fragile.
- **First-class Azure backend, rendered for the Orchestration Cluster only (chosen).**

## Decision Outcome

Azure Blob Storage is a first-class document handling storage backend in the Helm chart. Configuration is surfaced in `values.yaml` and validated via `values.schema.json`. The following constraints are normative:

1. The chart MUST render Azure document-store configuration only for the Orchestration Cluster. It MUST NOT render it for Connectors.
2. The chart MUST render the connection string at the path that the pinned application line binds: `DOCUMENT_STORE_<ID>_CONNECTION_STRING` in 8.9, with the uppercased `global.documentStore.activeStoreId` as `<ID>`, and `camunda.document.azure.<storeId>.connection-string` in 8.10.
3. The connection string keeps the `inlineSecret` and `existingSecret` values interface in both chart versions.

The shared document-store rules of [ADR 0052](0052-centralize-document-store-configuration-via-shared.md) also apply.

### Positive Consequences

- Azure customers can configure document storage declaratively with full schema validation, reducing misconfiguration risk.
- Every rendered Azure setting binds in the pinned application, so no Azure configuration renders inert.
- Landing the change in both 8.9 and 8.10 provides a consistent upgrade path without backport pressure later.

### Negative Consequences

- Additional conditional logic in the orchestration templates increases template complexity and cognitive load for chart maintainers.
- The two chart versions render Azure through different mechanisms, so each change must be made and verified per version until 8.9 reaches end-of-life.

## Changelog

- 2026-08-28 — [#6933](https://github.com/camunda/camunda-platform-helm/pull/6933) — Render Azure configuration for the Orchestration Cluster only, at the path that each application line binds (was ADR 0097).
- 2026-09-25 — [#7175](https://github.com/camunda/camunda-platform-helm/pull/7175) — Bind the 8.9 connection string through `DOCUMENT_STORE_<ID>_CONNECTION_STRING`.

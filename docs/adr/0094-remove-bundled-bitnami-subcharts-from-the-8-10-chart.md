# Remove bundled Bitnami subcharts from the 8.10 chart and migrate CI to companion charts

- Status: accepted
- Date: 2026-06-01
- Decision-makers: Balázs

## Context and Problem Statement

Through 8.9 the Camunda Platform Helm chart bundled four Bitnami subcharts —
`identityKeycloak`, `identityPostgresql`, `webModelerPostgresql`, and `elasticsearch` —
so that a default install provisioned its own Keycloak, two PostgreSQL instances, and
Elasticsearch. In mid-2025 Bitnami moved the underlying container images to an
unmaintained `bitnamilegacy/*` namespace and stopped patching them. The chart therefore
shipped unmaintained, unpatchable third-party infrastructure inside a product marketed as
production-ready, and every release carried the resulting CVE exposure.

This was addressed as a multi-release deprecation: 8.8 disabled the bundled subcharts by
default, 8.9 became the migration runway (migration guide in
`camunda-docs/.../migration-from-bitnami/` and a toolkit in
`camunda-deployment-references/generic/kubernetes/migration/`), and 8.10 is the agreed
removal target per [PDP #3554](https://github.com/camunda/product-hub/issues/3554) and
EPIC [team-distribution#774](https://github.com/camunda/team-distribution/issues/774).
[ADR-0083](0083-deprecate-bitnami-subcharts-in-camunda-platform-helm-chart.md) recorded the
8.9 deprecation; this ADR records the 8.10 removal and the CI test migration that the
removal forced (delivered in [#6146](https://github.com/camunda/camunda-platform-helm/pull/6146)).

## Decision Drivers

- **Supply-chain security:** the bundled images were unmaintained (`bitnamilegacy/*`); continuing to ship them meant continuing to ship unpatched CVEs.
- **Contractual obligation, not a design choice:** removal is the settled end-state of the public 8.8 → 8.9 → 8.10 deprecation path, framed by PDP #3554. The question was how to remove cleanly, not whether to remove.
- **Explicit Self-Managed ownership boundary:** from 8.10, Camunda owns the workload chart; the customer owns the database (PostgreSQL), search (Elasticsearch/OpenSearch), and identity provider (Keycloak/IdP). This shrinks the supported surface area.
- **Test what we ship:** with no bundled infrastructure, CI scenarios must exercise the external-infrastructure path that the chart now mandates, not a bundled-subchart path that no longer exists.
- **Ship the breaking change early:** the chart is in 8.10 alpha, so the breaking removal lands as early in the cycle as possible to give consumers maximum runway.

## Considered Options

- **Hard removal with constraint failures (chosen)** — drop the four subcharts and fail `helm template` for any still-set removed key, pointing at the migration guide.
- **Soft deprecation shim into 8.10** — rejected: the deprecation window already spanned 8.8 and 8.9; another shim would defer the supply-chain fix and add the exact dual-code-path complexity the removal is meant to delete.
- **Replace with new bundled charts (e.g. an in-house Keycloak/PostgreSQL)** — rejected as a product default: it recreates the "Camunda owns infrastructure it cannot patch" problem. Minimal in-repo charts are introduced for CI only (see below), not as a supported install path.

For the test infrastructure that the removal invalidated:

- **Per-run companion charts (chosen)** — each scenario deploys lightweight charts alongside the Camunda release: `internal-postgresql` and `internal-keycloak-26` (added in-repo for CI), the `elastic/elasticsearch` (ECK) chart, and a CloudNativePG (CNPG) cluster for RDBMS/self-signed scenarios.
- **Keep relying on shared in-cluster CI services** — rejected: it re-couples tests to long-lived shared infrastructure (the ~107 GB Keycloak/ES/OS/PG footprint tracked for decommissioning in [team-distribution#796](https://github.com/camunda/team-distribution/issues/796)) and does not represent how a real 8.10 user wires external infrastructure.
- **Keep the Bitnami subcharts for tests only** — rejected: it would retain a dependency on the exact unmaintained images the release is removing, and tests would not exercise the supported external-infra surface.

## Decision Outcome

The four Bitnami subcharts and their supporting values (`values-bitnami-legacy.yaml`,
removed-key blocks) are removed from the 8.10 chart. The dual code paths that gated on the
subcharts' `*.enabled` flags collapse to the external-infrastructure path; identity Keycloak
helpers read from `global.identity.keycloak.url.*` and identity/web-modeler database helpers
read from the respective `externalDatabase.*` only. `camundaPlatform.keyRemoved` guards make
`helm template` fail with a migration-guide pointer when a removed key is still set.

CI integration tests are rebuilt around per-run companion charts: minimal in-repo
`internal-postgresql` and `internal-keycloak-26` charts, the ECK Elasticsearch chart, and a
CNPG cluster pre-install fixture for RDBMS/self-signed scenarios; the matrix runner gains
`processCompanionCharts` env substitution and version-gates the legacy Bitnami PG password
extraction off for ≥8.10. Bundled-only unit suites and golden snapshots
(`keycloak-statefulset`, `elasticsearch-statefulset`) and the external/self-signed scenario
value files are deleted. The migration guide and toolkit are not re-authored; they are linked
from the chart README and the upgrade documentation.

> **Amendment (2026-06-05, [#6339](https://github.com/camunda/camunda-platform-helm/pull/6339)):**
> The CNPG cluster pre-install fixture for the `rdbms` and `rdbms-self-signed`
> scenarios was superseded by `internal-postgresql` companion-chart profiles
> (`postgresql-rdbms` and a TLS-enabled `postgresql-tls`) because the CloudNativePG
> operator on the shared CI cluster intermittently stops reconciling, failing every
> dependent scenario ([#6338](https://github.com/camunda/camunda-platform-helm/issues/6338)).
> This narrows CI's reliance on per-run operator reconciliation; it does not change
> the ADR's decision to remove the bundled Bitnami subcharts. The `cnpg`
> dependency-profile is retained as an opt-in escape hatch for scenarios that
> explicitly want an operator-managed cluster, but no scenario references it by default.

### Positive Consequences

- The chart no longer ships unmaintained third-party infrastructure; the recurring Bitnami CVE surface is eliminated.
- The Self-Managed responsibility boundary is explicit and the template logic reads as an external-infrastructure-first chart rather than one with bundled fallbacks.
- CI scenarios provision infrastructure the same way a real 8.10 deployment does, improving test fidelity, and the per-run model unblocks decommissioning the shared CI infrastructure (team-distribution#796).

### Negative Consequences

- Direct breaking change: any 8.10 consumer still setting a removed key gets a hard `helm template` failure and must migrate to externally managed infrastructure (downstream consumers tracked under the EPIC; e.g. `camunda/camunda#54089`).
- Users who relied on the bundled subcharts for quick local/dev installs must now provision Keycloak, PostgreSQL, and Elasticsearch themselves (operator-based and companion examples are provided per [#5989](https://github.com/camunda/camunda-platform-helm/issues/5989)).
- The CI test surface gains companion-chart wiring whose per-scenario duplication needs follow-up consolidation ([#6273](https://github.com/camunda/camunda-platform-helm/issues/6273)).

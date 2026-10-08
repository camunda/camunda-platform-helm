# Centralize document-store configuration via shared ConfigMap with multi-backend support

- Status: accepted
- Date: 2025-02-21
- Decision-makers: Daniel Rodriguez, Distribution team

## Context and Problem Statement

Camunda's document store previously supported only a single storage backend, configured independently per component. As deployment requirements grow in complexity — multi-region architectures, hybrid storage strategies, and compliance-driven data residency — a single hardcoded backend becomes insufficient. The platform needed a declarative, Helm-native mechanism to configure multiple document-store backends consistently across all consuming services.

The components that read document-store configuration differ by chart version. The block that carries the configuration also carries a second, unrelated payload. Document configuration (the `DOCUMENT_*` / `DOCUMENT_STORE_*` keys) and ambient cloud credentials (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `GOOGLE_APPLICATION_CREDENTIALS` and its GCP volume) travel in one ConfigMap and one env block, but different code reads them for different reasons. Wiring every component by symmetry gave components configuration and secrets that they never read.

### Applicability by version

ConfigMap consumers, verified against the application source at the image tag that each chart version pins:

- **8.7** — `zeebe` StatefulSet, `zeebe-gateway`, `operate`, `tasklist`, `execution-identity`, and `web-modeler` restapi. `connectors` receives the cloud credentials only.
- **8.8** — `orchestration` StatefulSet and importer Deployment, `optimize`, and `web-modeler` restapi. `connectors` receives the cloud credentials only.
- **8.9** — `orchestration` StatefulSet, `optimize`, and `web-modeler` restapi. `connectors` receives the cloud credentials only.
- **8.10** — `orchestration` StatefulSet only. A component that sets `camunda.document.*` in its `extraConfiguration` does not get the ConfigMap `envFrom` ([ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md)).

Console, Web Modeler webapp, and Management Identity read neither payload and get no wiring.

In 8.8 and 8.9, `connectors`, `optimize`, and `web-modeler` restapi also use the document-store cloud credentials as ambient AWS or GCP configuration: AWS connector tasks, AWS OpenSearch request signing, and the Web Modeler document storage. These chart lines keep that injection to avoid a breaking change and render a deprecation warning. In 8.10 these components no longer receive the credentials.

## Decision Drivers

- **Configuration consistency:** Multiple components consume document-store config; independent per-component configuration creates drift risk and operational burden.
- **Multi-backend necessity:** Real-world deployments require simultaneous support for GCS, S3, and local filesystem backends to satisfy data residency and redundancy requirements.
- **Helm idiom compliance:** The solution must work within Helm's declarative model without introducing external runtime dependencies.
- **Operator simplicity:** Configuration should be defined once in `values.yaml` and distributed automatically to all relevant workloads.
- **Verified consumers only:** A component that receives configuration or secrets it never reads widens secret exposure and misleads reviewers, including automated review that reads `docs/adr/`.
- **Separate payloads:** Document configuration and ambient cloud credentials share a transport but have different readers, so each needs its own rule.

## Considered Options

- **Per-component configuration** — each deployment carries its own document-store config block. Rejected due to duplication across 7+ services and high drift risk during upgrades.
- **External configuration service (Consul/Vault)** — rejected as too heavy a dependency for a Helm-native deployment model that targets air-gapped and minimal environments.
- **Environment variables only** — rejected because multi-backend configuration with nested properties (bucket names, credentials, regions per backend) is too complex for flat environment variable schemas.
- **Wire every component by symmetry** — rejected because it copies configuration and secrets into components before anyone verifies that they read them.
- **Shared ConfigMap for verified consumers, with cloud credentials as a separate contract (chosen).**

## Decision Outcome

A shared ConfigMap (`configmap-documentstore.yaml`) at the platform level is the single rendering point for document-store configuration. The `values.yaml` schema supports an array of storage backend definitions, so operators declare multiple backends declaratively. The following constraints are normative for all supported chart versions:

1. **Consumer set.** The `-documentstore-env-vars` ConfigMap MUST be referenced only by components verified to read the `DOCUMENT_*` / `DOCUMENT_STORE_*` keys, or verified to read a key that only this block supplies. The per-version sets are under "Applicability by version".
2. **Credentials are a separate contract.** `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `GOOGLE_APPLICATION_CREDENTIALS`, and the GCP credentials volume are ambient cloud-SDK configuration, not document-store configuration. From 8.10, the `global.documentStore.*` credentials MUST reach only document-store consumers. A component that needs cloud access for another feature MUST get it from its own component configuration. Any analysis of this block MUST evaluate the two payload classes independently: "no `DOCUMENT_STORE_*` reader" MUST NOT be read as "no reader".
3. **Verification before wiring.** Adding a component to the document-store wiring, or adding a store backend to an already-wired component, MUST be justified by source-level verification against the image that the chart version pins. Symmetry with an already-wired component is not sufficient justification.
4. **Inert configuration is a defect, not a decision.** Where the chart renders document-store configuration that the pinned application cannot bind, the gap MUST be tracked as a bug against the chart or the application line, and MUST NOT be recorded as intended architecture.

### Positive Consequences

- **Single source of truth:** Every verified consumer of a chart version receives identical document-store configuration, eliminating drift between services.
- **Extensibility:** Adding a new storage backend requires no structural changes — only values and verified wiring.
- **Helm-native:** No external runtime dependencies; configuration correctness is validated at deploy time via JSON Schema.
- **Smaller secret exposure:** Fewer components carry a `secretKeyRef` they never read.

### Negative Consequences

- **Blast radius:** A misconfiguration in the shared ConfigMap affects all of its consumers simultaneously rather than being isolated to a single service.
- **Schema complexity:** Operators must understand the multi-backend configuration structure even for simple single-backend deployments, increasing the learning curve for initial adoption.
- **Verification cost:** Constraint 3 adds a step — read the pinned image — to every document-store change, which is slower than copying an existing component's block.
- **Per-version consumer sets:** Maintainers must check the chart version they edit, because no single consumer list holds for all versions.
- **Upgrade migration:** Operators who used the `global.documentStore.*` credentials for another component's cloud access must move that configuration to the component when they upgrade to 8.10.

## Links

- Builds on [ADR 0091](0091-adopt-component-extraconfiguration-as-the-standard-application-configuration-mechanism.md) — on 8.10 the ConfigMap `envFrom` is suppressed for a component that owns `camunda.document.*` via `extraConfiguration`.
- [ADR 0089](0089-integrate-azure-blob-storage-as-a-document-handling-backend.md) — the Azure backend and its rendering rules.

## Changelog

- 2026-08-28 — [#6933](https://github.com/camunda/camunda-platform-helm/pull/6933) — Narrow the consumer set to verified readers, separate the cloud credentials from document configuration, and require verification before wiring (was ADR 0097).
- 2026-09-23 — [#7200](https://github.com/camunda/camunda-platform-helm/pull/7200) — Stop injecting the document-store cloud credentials into non-document components in 8.10, and warn in 8.8 and 8.9.

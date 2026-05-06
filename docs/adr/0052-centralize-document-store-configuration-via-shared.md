# Centralize document-store configuration via shared ConfigMap with multi-backend support

- Status: accepted
- Date: 2025-02-21
- Decision-makers: Daniel Rodriguez

## Context and Problem Statement

Camunda's document store previously supported only a single storage backend, configured independently per component. As deployment requirements grow in complexity — multi-region architectures, hybrid storage strategies, and compliance-driven data residency — a single hardcoded backend becomes insufficient. The platform needed a declarative, Helm-native mechanism to configure multiple document-store backends consistently across all consuming services.

## Decision Drivers

- **Configuration consistency:** Multiple components consume document-store config; independent per-component configuration creates drift risk and operational burden.
- **Multi-backend necessity:** Real-world deployments require simultaneous support for GCS, S3, and local filesystem backends to satisfy data residency and redundancy requirements.
- **Helm idiom compliance:** The solution must work within Helm's declarative model without introducing external runtime dependencies.
- **Operator simplicity:** Configuration should be defined once in `values.yaml` and distributed automatically to all relevant workloads.

## Considered Options

- **Per-component configuration** — each deployment carries its own document-store config block. Rejected due to duplication across 7+ services and high drift risk during upgrades.
- **External configuration service (Consul/Vault)** — rejected as too heavy a dependency for a Helm-native deployment model that targets air-gapped and minimal environments.
- **Environment variables only** — rejected because multi-backend configuration with nested properties (bucket names, credentials, regions per backend) is too complex for flat environment variable schemas.

## Decision Outcome

A shared ConfigMap (`configmap-documentstore.yaml`) was introduced at the platform level and mounted into every service deployment that interacts with the document store. The `values.yaml` schema was extended to support an array of storage backend definitions, allowing operators to declare multiple backends declaratively. This single source of truth is rendered once by Helm and consumed uniformly by Connectors, Console, Core/Zeebe, Identity, Optimize, and Web Modeler.

### Positive Consequences

- **Single source of truth:** All components receive identical document-store configuration, eliminating drift between services.
- **Extensibility:** Adding new storage backends or new consuming services requires no structural changes — only values and volume mount additions.
- **Helm-native:** No external runtime dependencies; configuration correctness is validated at deploy time via JSON Schema.

### Negative Consequences

- **Blast radius:** A misconfiguration in the shared ConfigMap affects all components simultaneously rather than being isolated to a single service.
- **Schema complexity:** Operators must understand the multi-backend configuration structure even for simple single-backend deployments, increasing the learning curve for initial adoption.
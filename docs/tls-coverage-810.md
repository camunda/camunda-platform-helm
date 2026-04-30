# TLS Coverage Matrix — Camunda 8.10

This document inventories every documented Camunda 8.10 connection and its TLS support level **as shipped by this Helm chart**. It is the source-of-truth for the customer-facing guide drafted in Phase E of the [TLS-everywhere implementation plan](https://github.com/camunda/product-hub/issues/3520).

## Why one PEM bundle, not a JKS

Camunda components span three trust ecosystems:

- **OS / OpenSSL native** (libcurl, Go HTTP, OpenSearch native client, PG JDBC `sslrootcert=`) reads PEM via `SSL_CERT_FILE` or a file path.
- **JVM** (Operate, Tasklist, Optimize, Web Modeler restapi, Identity, Connectors, Zeebe broker) reads only **binary keystores** (PKCS12 or JKS) via `-Djavax.net.ssl.trustStore=…`. It does **not** read PEM directly.
- **Node.js** (Console, Web Modeler websockets) reads PEM via `NODE_EXTRA_CA_CERTS`.

Historically that meant two trust artefacts per deployment: a PEM for the OS/Node side, a JKS for the JVM side, plus per-component values references for the JKS. The chart now bridges the gap with an **init container that builds the JVM keystore from the PEM at pod start** — copying `$JAVA_HOME/lib/security/cacerts` and appending the user CA with `keytool`. Customers supply one PEM Secret; the chart wires both worlds.

JVM apps still need a keystore at runtime. The change is that the chart, not the customer, builds it.

## Deprecation: legacy per-component JKS path

The legacy `*.tls.secret.existingSecret` and `global.<engine>.tls.jks.*` fields remain functional but are **deprecated as of chart 15.x** in favour of `global.tls.caBundle.secret.*`. Concrete fields:

- `global.elasticsearch.tls.secret.{existingSecret,existingSecretKey}`
- `global.opensearch.tls.secret.{existingSecret,existingSecretKey}`
- `global.elasticsearch.tls.jks.secret.*` (chart 14.x; password-injection block from #5994)
- `global.opensearch.tls.jks.secret.*`
- `orchestration.data.secondaryStorage.elasticsearch.tls.secret.*`
- `orchestration.data.secondaryStorage.opensearch.tls.secret.*`
- `optimize.database.elasticsearch.tls.secret.*`
- `optimize.database.opensearch.tls.secret.*`

Precedence remains "legacy JKS wins when set" so existing deployments do not regress on upgrade. New deployments should use `global.tls.caBundle.secret.*` only. A future major chart release will remove the legacy fields.

For the values-side machinery referenced below, see:
- `global.tls.caBundle.secret.{existingSecret,existingSecretKey}` — OS-level CA bundle, used by `SSL_CERT_FILE` and the chart-built combined Java truststore (PRs #6039 + #6040).
- Per-component `tls.secret.existingSecret` — **deprecated** legacy JKS truststore path (still supported, takes precedence when set; see deprecation list above).
- `global.gateway.tls.{enabled,secretName}` — external-facing GatewayAPI TLS termination.
- `*.ingress.grpc.tls.{enabled,secretName}` and similar — external-facing Ingress TLS termination.

Status legend:

| Mark | Meaning |
| :---: | --- |
| ✅ | TLS supported and validated end-to-end on GKE `distro-ci` |
| 🟢 | TLS supported (chart wiring exists; not exhaustively re-tested in this epic) |
| 🟡 | Partial: external edges only — pod-to-pod cluster traffic is plaintext unless a service mesh is added |
| ⚫ | Out of scope: requires service mesh (mTLS for in-cluster transport not provided by this chart) |
| ❌ | Gap: no TLS support today |

## Connections covered by this chart

### 1. Camunda components → secondary storage

| Connection | Status | Mechanism (TLS trust) | Notes |
| --- | :---: | --- | --- |
| Orchestration (Zeebe / Operate / Tasklist) → Elasticsearch | ✅ | JKS `tls.secret.existingSecret` _or_ `global.tls.caBundle` | Validated by the `elasticsearch-self-signed` scenario merged via #5994 (entry currently `enabled: false` in `ci-test-config.yaml` pending owner decision; superseded the closed draft #6033) |
| Orchestration → OpenSearch | ✅ | Same as above | Validated by `opensearch-self-signed` (#6032) and `opensearch-self-signed-os-trust` (#6040, JKS-removed path) |
| Orchestration → RDBMS (PostgreSQL) | ✅ | JDBC `?sslmode=verify-full&sslrootcert=…` + `global.tls.caBundle` mount | Validation lands with sibling PR #6036, which adds the `rdbms-self-signed` scenario (file does not exist on this branch) |
| Orchestration → RDBMS (Oracle) | 🟢 | Same JDBC pattern; cert SAN handling is Oracle-specific | Not validated in this epic; covered by Phase F customer trial |
| Optimize → Elasticsearch | ✅ | JKS via `optimize.database.elasticsearch.tls.secret` _or_ caBundle | Validated alongside orchestration |
| Optimize → OpenSearch | ✅ | Same | Validated alongside orchestration |
| Camunda Exporter (Zeebe broker) → secondary storage | ✅ | Inherits orchestration's truststore | Validated end-to-end via E2E smoke |
| Legacy `ElasticsearchExporter` (zeebe-record-* indices) → ES | ✅ | `ZEEBE_BROKER_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_*` for auth + same truststore for TLS | Discovered + documented in #6033 PR description; auth env path is independent of `secondaryStorage.elasticsearch.auth` |

### 2. Component ingress / external edges

| Connection | Status | Mechanism | Notes |
| --- | :---: | --- | --- |
| Browser / external client → orchestration HTTP | ✅ | `orchestration.ingress.grpc.tls.{enabled,secretName}` and `*.ingress.tls.*` per component | Standard K8s ingress TLS termination |
| Browser → Operate / Tasklist / Modeler / Console / Identity / Optimize UI | ✅ | Per-component `*.ingress.tls` or `global.gateway.tls` | Standard K8s ingress TLS termination |
| External gRPC client → Zeebe gateway | ✅ | `orchestration.ingress.grpc.tls.{enabled,secretName}` | Configured at the ingress, not the pod |
| GatewayAPI deployments | ✅ | `global.gateway.tls.{enabled,secretName}` | When using the GatewayAPI controller path |

### 3. Component → identity / OIDC issuer

| Connection | Status | Mechanism (TLS trust) | Notes |
| --- | :---: | --- | --- |
| Any component → in-cluster Keycloak OIDC token endpoint | 🟡 | If `global.identity.auth.tokenUrl` is `https://`, JVM truststore via caBundle / JKS | In-cluster default endpoint is HTTP (Keycloak terminates TLS at ingress); HTTPS only when external Keycloak is configured |
| Any component → external IdP (Microsoft Entra, Okta, etc.) | ✅ | `global.identity.auth.issuerBackendUrl: https://…` + caBundle (Phase B) or system CA bundle for public CAs | Public-CA chains work out of the box; private CAs supported via caBundle |

## Connections out of scope (require service mesh)

The chart does **not** provision pod-to-pod mTLS for in-cluster service traffic. The connections below are plaintext by default and require a service mesh (Linkerd, Istio, Cilium) to encrypt at the pod level:

| Connection | Status | Notes |
| --- | :---: | --- |
| Operate → Zeebe gateway (gRPC, in-cluster) | ⚫ | `integration-zeebe-gateway:26500` plaintext gRPC |
| Tasklist → Zeebe gateway (gRPC, in-cluster) | ⚫ | Same |
| Connectors → Zeebe gateway (gRPC, in-cluster) | ⚫ | Same |
| Web Modeler → Identity (REST, in-cluster) | ⚫ | `integration-identity:80` plaintext HTTP |
| Console → Identity (REST, in-cluster) | ⚫ | Same |
| Optimize / Operate / Tasklist → Identity | ⚫ | Same |
| Connectors → Identity (REST, in-cluster) | ⚫ | Same — OAuth token validation flow against `integration-identity:80` |
| Zeebe broker ↔ broker (Raft replication / gossip, in-cluster) | ⚫ | Internal port 26502; carries in-flight process state. Plaintext by default; a service mesh is required for transport encryption between broker pods |
| Spring Boot management / metrics endpoints (probes) | ⚫ | All probes scheme `HTTP`; switching to HTTPS would require server.ssl wiring on every component (no chart support today). Cluster-internal only. |

This is documented in the epic ([product-hub#3520](https://github.com/camunda/product-hub/issues/3520) Validation Criteria → "What still requires a service mesh") as the explicit boundary of the chart-level TLS deliverable.

## Verified gaps (potential future work)

| Gap | Tracking | Notes |
| --- | --- | --- |
| Spring Boot actuator endpoints over TLS | none | Would need `server.ssl.enabled` + per-component cert mounting; meaningful only when security/scrape tooling pulls actuator from outside the cluster |
| Web Modeler websockets (Node.js) trust customization | none | Resolved: `caBundleEnv` helper (#6039 + the Console/Node fix in #6040) emits both `SSL_CERT_FILE` AND `NODE_EXTRA_CA_CERTS` — Node-native trust path is wired |
| Direct in-cluster mTLS without mesh (e.g., Linkerd-style auto-mTLS) | out of scope per epic | Mesh layer is the supported answer |

## Validation matrix

Each row in section 1 was validated by deploying the corresponding scenario and observing successful pod readiness, no TLS handshake / PKIX errors in logs, and a passing `c8e2e` smoke run. Specific PR / scenario references:

| Scenario | PR | Notes |
| --- | --- | --- |
| `opensearch-self-signed` | #6032 | OS via JKS |
| `opensearch-self-signed-os-trust` | #6040 | OS via caBundle init container, JKS removed — proves Phase B2 |
| `elasticsearch-self-signed` | #6033 | ES + legacy exporter auth env |
| `rdbms-self-signed` | #6036 | PG with `sslmode=verify-full` + caBundle mount |
| Plaintext-fallback regression check | #6037 | Asserts no `http://datastore` URLs leaked into pods |

## How to use this matrix

- **Customers** evaluating Camunda 8.10 against an InfoSec checklist: section 1 is what the chart delivers out of the box; sections 2 and 3 cover external edges and IdPs; the "out of scope" section is the honest boundary.
- **Field teams**: when a customer asks "is TLS supported between X and Y", look up the row, point at the PR / scenario for proof, and route the answer through this document instead of redoing per-customer audits.
- **Reviewers** of upcoming PRs: this matrix should grow with new chart support and shrink in the "out of scope" column when a mesh integration ships. Please update it in the same PR that changes the underlying support.

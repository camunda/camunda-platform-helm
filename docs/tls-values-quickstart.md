# `values-tls.yaml` quickstart — Camunda 8.10 TLS-everywhere

A two-step path from a fresh namespace to a fully TLS-enabled Camunda 8.10 deployment talking to TLS-secured Elasticsearch / OpenSearch / PostgreSQL with a private (self-signed or internal) CA. No init-container-as-root workarounds, no custom images.

## 1. Create the CA bundle secret

The single trust input is a PEM CA bundle that signs your datastore certs.

```bash
NAMESPACE=camunda

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NAMESPACE" create secret generic camunda-ca-bundle \
  --from-file=ca.crt=./your-ca-bundle.pem
```

If you use cert-manager, see the illustrative recipe at the bottom of [`charts/camunda-platform-8.10/values-tls.yaml`](../charts/camunda-platform-8.10/values-tls.yaml).

## 2. Apply the overlay

```bash
helm upgrade --install camunda camunda/camunda-platform \
  --version 15.x \
  --namespace "$NAMESPACE" \
  -f charts/camunda-platform-8.10/values-tls.yaml \
  -f your-values.yaml
```

`values-tls.yaml` references `camunda-ca-bundle` (the secret you created in step 1) and wires it into every Java component automatically:

- mounts the bundle at `/etc/camunda/tls/ca.crt` (read-only)
- sets `SSL_CERT_FILE` for libraries that honor it
- runs an init container per Java component that imports the CA into a JKS at runtime, then sets `-Djavax.net.ssl.trustStore=…` so JVM HTTP clients trust it

`your-values.yaml` is wherever you supply the rest: datastore URLs, credentials, ingress hosts, IdP config. The TLS overlay is additive — it doesn't replace your scenario or persistence settings, only adds the trust input.

## What's covered

| Connection | Covered by overlay |
| --- | :---: |
| Camunda → Elasticsearch (private CA, self-hosted or AWS) | ✅ |
| Camunda → OpenSearch (private CA, self-hosted or AWS-managed) | ✅ |
| Camunda → PostgreSQL JDBC (`sslmode=verify-full` + CA) | ✅ |
| Camunda → external OIDC issuer (Entra, Okta, internal Keycloak) with private CA | ✅ |
| Browser / external client → ingress / GatewayAPI | ✅ via standard K8s patterns |
| In-cluster pod-to-pod transport (Operate ↔ Zeebe gateway, Connectors ↔ gateway) | ❌ requires service mesh — see [`tls-coverage-810.md`](tls-coverage-810.md) |

## Verify the deployment

After helm install, run the plaintext-fallback regression check (added by sibling PR #6037; the script lands on `main` once that PR merges) against the namespace:

```bash
# Requires #6037 to have merged; before then, clone the script from that branch
# or skip this step and rely on the spot-check below.
scripts/check-no-plaintext-datastore.sh \
  --namespace "$NAMESPACE" \
  --kube-context "$KUBE_CONTEXT"
```

Exit code 0 + `[no-plaintext-check] PASS` means no Camunda pod is talking plaintext to a known datastore service name.

Spot-check a Camunda pod's truststore wiring:

```bash
kubectl -n "$NAMESPACE" get pod -l app.kubernetes.io/component=zeebe-broker -o yaml | \
  grep -A 1 'JAVA_TOOL_OPTIONS\|SSL_CERT_FILE\|ca-bundle'
```

You should see:
- `SSL_CERT_FILE: /etc/camunda/tls/ca.crt`
- `JAVA_TOOL_OPTIONS: … -Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit`
- A `ca-bundle` volume sourced from `camunda-ca-bundle`
- A `ca-bundle-truststore` emptyDir volume populated by the init container

## Updating the CA

To rotate the CA bundle, replace the secret and restart the Camunda workloads so the init container rebuilds the truststore from the new CA:

```bash
kubectl -n "$NAMESPACE" delete secret camunda-ca-bundle
kubectl -n "$NAMESPACE" create secret generic camunda-ca-bundle \
  --from-file=ca.crt=./new-ca-bundle.pem

kubectl -n "$NAMESPACE" rollout restart statefulset,deployment
```

### Optional: automatic rollout on `helm upgrade`

Set `global.tls.caBundle.autoRollout: true` to have the chart stamp a `checksum/ca-bundle`
pod annotation derived from the CA secret, so the next `helm upgrade` rolls the Java
components automatically when the CA changes — no manual `rollout restart`.

It's **off by default** because it uses Helm's `lookup` to read the secret at upgrade
time, which has two important constraints:

- **RBAC:** the identity running `helm upgrade` must have `get` on Secrets in the
  release namespace. Without it, `lookup` raises a `Forbidden` error that is **not**
  catchable in templates and **fails the `helm upgrade`**. Only enable `autoRollout`
  where the upgrader has Secret-read access.
- **GitOps:** Argo CD / Flux render with `helm template` (no cluster access for
  `lookup`), so the annotation stays constant and the rollout never fires. Drive the
  restart from your GitOps stack instead (an Argo CD `PostSync` hook or a Flux
  `kustomize` patch), or use the `kubectl rollout restart` above.

Trust itself never depends on this flag — it only controls the rollout convenience.

> **Reminder:** `global.tls.caBundle` provides CA **trust**, not encryption. It does
> not turn a plaintext datastore connection into TLS — point the datastore URL at
> `https://` (or set the JDBC `sslmode`) to actually encrypt the traffic. The chart
> emits an install-time warning if the bundle is set while a datastore URL is still
> `http://`.

## How the chart detects Orchestration TLS

Four rendered values depend on whether Orchestration REST or gRPC TLS is on: the
dedicated `/orchestration` Ingress and its `backend-protocol`, the exclusion of
`/orchestration` from the shared Ingress, the gRPC Ingress `GRPC` vs `GRPCS`
backend, and the in-cluster endpoint schemes handed to Web Modeler and Connectors.

The chart resolves that state from four sources, highest precedence first — the
same order Spring applies, since the first two reach the container as environment
variables and Spring ranks those above every `application.yaml` source:

| Precedence | Source | Key |
|---|---|---|
| 1 | `orchestration.env` (last duplicate wins) | `SERVER_SSL_ENABLED` / `CAMUNDA_API_GRPC_SSL_ENABLED` |
| 2 | `global.tls.orchestration.{rest,grpc}.enabled` | emitted as the env vars above |
| 3 | `orchestration.extraConfiguration` | `server.ssl.enabled` / `camunda.api.grpc.ssl.enabled` |
| 4 | `orchestration.configuration` | same keys |

`global.tls.orchestration.*` is the supported route and the only one that also
wires cert material, keystore passwords, volume mounts, and rotation rollout.

### What the detection cannot see

The two YAML sources are matched as **nested** keys, so these forms enable TLS in
the running container while the chart still derives plaintext:

- a flat dotted key, `server.ssl.enabled: true` written as one key
- a relaxed-binding camelCase spelling
- keys outside the first YAML document in a file
- entries marked `springImport: false`, which are skipped by design

`valueFrom` and `envFrom` are unresolvable at render time by definition: the chart
cannot read a ConfigMap or Secret while templating, so a toggle sourced that way
is not seen either.

A wrong derivation fails closed, not open — you get a TLS handshake failure or
`Bad Request: This combination of host and port requires TLS.`, never silent
plaintext against a listener the chart reported as encrypted. The chart emits an
install-time warning for the dotted-key and `valueFrom` cases rather than deriving
plaintext silently. If you hit one, set `global.tls.orchestration.*` or add the
matching `orchestration.env` entry.

### Connectors and Optimize follow the same rules

`camundaPlatform.connectorsTLSEnabled` and the Optimize equivalent resolve
`server.ssl.enabled` from the same four sources in the same order, substituting
`connectors.*` / `optimize.*` for `orchestration.*` and
`global.tls.connectors.enabled` / `global.tls.optimize.enabled` for the
Orchestration flag. Connectors TLS state drives the container probe schemes and
the in-cluster Connectors URL; Optimize TLS state drives its probe schemes and
its dedicated `/optimize` Ingress backend. The same non-detections and the same
install-time warnings apply to both.

### Migrating from a hand-managed `/orchestration` Ingress

Enabling REST TLS makes the chart render its own `/orchestration` Ingress. If you
already manage one by hand, delete it in the same change as the `helm upgrade`, or
the two resources collide on the same host and path and the controller picks
between them arbitrarily. The chart-managed resource inherits every
`global.ingress.annotations` and `global.ingress.labels` entry, so custom auth,
rate-limit, and security-header annotations carry over — except
`nginx.ingress.kubernetes.io/backend-protocol`, which is forced to `HTTPS`
because an HTTP backend against a TLS listener is the failure this Ingress exists
to prevent. The chart warns when it overrides an inherited value.

## Common gotchas

- **Java 21 defaults `trustStoreType` to PKCS12.** This overlay's init container copies the JDK system `cacerts` (PKCS12 on Java 21) and appends the user CA via `keytool -importcert` without changing the format — the chart-built truststore is PKCS12, and the chart helper relies on the JVM default by NOT setting `-Djavax.net.ssl.trustStoreType` for that path. If you instead supply your own legacy JKS via a per-component `tls.secret.existingSecret`, that path takes precedence and your `javaOpts` must set `-Djavax.net.ssl.trustStoreType=jks` explicitly.
- **Legacy Zeebe ES exporter** (`zeebe-record-*` indices) has its own auth env path (`ZEEBE_BROKER_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_*`) that the regular `secondaryStorage.elasticsearch.auth` does not fill. Set those env vars via `orchestration.env` if you use the legacy exporter — see #6033 for an example.
- **Bitnami PostgreSQL `tls.certCAFilename`** flips the server into mTLS mode. Customers running the chart's default `external` Postgres should configure server-side TLS via their cloud provider's tooling (e.g., RDS/Cloud SQL TLS settings), not the Bitnami subchart's `tls.*` keys.
- **Web Modeler websockets and Console are Node.js.** The chart helper now emits both `SSL_CERT_FILE` and `NODE_EXTRA_CA_CERTS` on every Node component automatically — no manual `webModeler.websockets.env` override needed. (Adding it manually would create a duplicate env entry, which Kubernetes treats as undefined behavior.)

## Related

- Coverage matrix: [`tls-coverage-810.md`](tls-coverage-810.md)
- Foundational caBundle wiring: [PR #6039](https://github.com/camunda/camunda-platform-helm/pull/6039)
- JVM truststore init container: [PR #6040](https://github.com/camunda/camunda-platform-helm/pull/6040)
- Plaintext-fallback regression check: [PR #6037](https://github.com/camunda/camunda-platform-helm/pull/6037)
- Validated CI scenarios: #6032 (OS), #6033 (ES), #6036 (RDBMS)

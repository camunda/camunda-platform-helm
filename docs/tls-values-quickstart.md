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

To rotate the CA bundle, replace the secret and bounce the affected pods:

```bash
kubectl -n "$NAMESPACE" delete secret camunda-ca-bundle
kubectl -n "$NAMESPACE" create secret generic camunda-ca-bundle \
  --from-file=ca.crt=./new-ca-bundle.pem

kubectl -n "$NAMESPACE" rollout restart statefulset,deployment
```

The init container re-runs on each pod start and imports the new CA into a fresh truststore.

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

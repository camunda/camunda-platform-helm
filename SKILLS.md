# Operational Skills

Instructions for using the primary operational tools in this repository: the `deploy-camunda` CLI and `kubectl`. Use these to deploy, debug, and test Camunda on Kubernetes.

Throughout this document, `$NS` refers to a Kubernetes namespace and `$RELEASE` refers to a Helm release name.

---

## deploy-camunda CLI

The `deploy-camunda` CLI orchestrates Camunda Platform Helm deployments. It resolves layered values files, performs environment variable substitution, manages Keycloak realms and Elasticsearch index prefixes, and supports parallel multi-scenario deployments.

**Source:** `scripts/deploy-camunda/`

**Full flag reference:** Run `deploy-camunda --help`, `deploy-camunda matrix run --help`, or `deploy-camunda prepare-values --help`.

### Installation

```bash
# Recommended: install all Go CLI tools
make install.dx-tooling

# Or just deploy-camunda
cd scripts/deploy-camunda && go build -o deploy-camunda . && mv deploy-camunda $GOPATH/bin/
```

### Deploy a Single Scenario

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS \
  --release $RELEASE \
  --scenario chart-full-setup
```

### Deploy with Selection + Composition

The preferred way to configure deployments is through selection flags that compose layered values:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup \
  --identity keycloak-external \
  --persistence opensearch \
  --features multitenancy,documentstore \
  --qa
```

Available selections:

| Flag | Values |
|------|--------|
| `--identity` | `keycloak`, `keycloak-external`, `oidc`, `basic`, `hybrid` |
| `--persistence` | `elasticsearch`, `opensearch`, `rdbms`, `rdbms-oracle` |
| `--test-platform` | `gke`, `eks`, `openshift` |
| `--features` | `multitenancy`, `rba`, `documentstore` |
| `--qa` | (boolean) Enable QA configuration |
| `--upgrade-flow` | (boolean) Enable upgrade flow configuration |

Layer resolution order (last wins):

```
base.yaml -> base-upgrade.yaml (if upgrade) -> identity -> persistence -> platform -> features -> QA -> image-tags
```

Layered values live in: `charts/<version>/test/integration/scenarios/chart-full-setup/values/`

### Configuration Profiles

Create a config file at `.camunda-deploy.yaml` (project root) or `~/.config/camunda/deploy.yaml`:

```yaml
current: dev
repoRoot: /path/to/repo

deployments:
  dev:
    chartPath: ./charts/camunda-platform-8.9
    namespace: dev-test
    release: camunda
    scenario: chart-full-setup

matrix:
  maxParallel: 33
  namespacePrefix: distribution
  ensureDockerHub: true
  ensureDockerRegistry: true
  kubeContexts:
    gke: gke_camunda-distribution_europe-west1-b_distro-ci
  ingressBaseDomains:
    gke: ci.distro.ultrawombat.com
  envFiles:
    "8.9": .env.89
```

Manage profiles: `deploy-camunda config create|use|show|set`.

### Matrix Operations

The matrix manages the CI test matrix — all scenario/version/platform combinations defined in each chart's `ci-test-config.yaml`.

```bash
# List all scenarios
deploy-camunda matrix list --repo-root . --versions 8.9

# Filter by shortname
deploy-camunda matrix list --repo-root . --shortname-filter eske

# Dry-run: preview what would be deployed
deploy-camunda matrix run --repo-root . --versions 8.9 --shortname-filter eske --dry-run

# Run a specific scenario on GKE
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.9 \
  --shortname-filter eske \
  --flow-filter upgrade-minor \
  --platform gke \
  --delete-namespace \
  --timeout 15 \
  --yes
```

**Important: Docker credentials are required.** The matrix runner creates K8s pull secrets. Before running, ask the user to ensure these environment variables are set:

- **Harbor**: `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD` and `TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD`
- **Docker Hub**: `TEST_DOCKER_USERNAME` and `TEST_DOCKER_PASSWORD`

**Upgrade flows are two-step** (handled automatically by `matrix run`):
1. Install the previous version's chart from the Helm repo (e.g., `camunda/camunda-platform@13.5.2` for 8.8)
2. `helm upgrade --force` to the local chart with `base-upgrade.yaml` included

The namespace convention is: `<prefix>-<version>-<shortname>-<flow>` (e.g., `distribution-89-eske-upgm-gke`).

### Render Without Deploying

Debug values merging without touching the cluster:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --render-templates --render-output-dir ./debug-output
```

### Prepare Values Standalone

```bash
deploy-camunda prepare-values \
  --chart-path ./charts/camunda-platform-8.9 \
  --identity keycloak --persistence elasticsearch \
  --interactive=false
# Returns path to merged values file
```

---

## kubectl — Debugging Deployments

### Check Deployment Health

```bash
kubectl get pods -n $NS -o wide
kubectl get deployments,statefulsets -n $NS
helm list -n $NS
helm get values $RELEASE -n $NS
```

**Pod states:**

| Status | Next Step |
|--------|-----------|
| `Pending` | `kubectl describe pod` — check resources, PVC, node selector |
| `CrashLoopBackOff` | `kubectl logs --previous` — check config, env vars, OOM |
| `ImagePullBackOff` | `kubectl describe pod` — check image name, registry secret |
| `Running` but not `Ready` | `kubectl describe pod` — check readiness probe |

### Debug Failing Pods

```bash
kubectl describe pod <pod-name> -n $NS          # Events section at bottom
kubectl logs <pod-name> -n $NS                   # Main container logs
kubectl logs <pod-name> -n $NS --previous        # Previous crash logs
kubectl logs <pod-name> -n $NS --all-containers  # All containers
kubectl get events -n $NS --sort-by=.lastTimestamp
```

Pod naming pattern (8.8+):
```
$RELEASE-zeebe-0/1/2          # Orchestration StatefulSet
$RELEASE-connectors-<hash>    # Connectors
$RELEASE-identity-<hash>      # Identity
$RELEASE-optimize-<hash>      # Optimize
$RELEASE-web-modeler-restapi-<hash>     # Pre-8.10
$RELEASE-web-modeler-websockets-<hash>  # Pre-8.10
$RELEASE-hub-<hash>                     # 8.10+
$RELEASE-hub-websockets-<hash>          # 8.10+
$RELEASE-console-<hash>
$RELEASE-keycloak-0
$RELEASE-postgresql-0
```

### Port-Forward to Services

```bash
kubectl port-forward svc/$RELEASE-zeebe-gateway 26500:26500 -n $NS  # gRPC
kubectl port-forward svc/$RELEASE-identity 8084:80 -n $NS
kubectl port-forward svc/$RELEASE-optimize 8083:80 -n $NS
kubectl port-forward svc/$RELEASE-connectors 8085:8080 -n $NS
kubectl port-forward svc/$RELEASE-console 8088:80 -n $NS
kubectl port-forward svc/$RELEASE-keycloak 18080:80 -n $NS
kubectl port-forward svc/$RELEASE-elasticsearch 9200:9200 -n $NS
```

### Manage Secrets

```bash
kubectl get secrets -n $NS
kubectl get secret <name> -n $NS -o jsonpath="{.data.<key>}" | base64 -d
kubectl get secret <name> -n $NS -o json | jq '.data | map_values(@base64d)'
```

### Namespace Lifecycle

```bash
kubectl create ns $NS --dry-run=client -o yaml | kubectl apply -f -
kubectl delete namespace $NS --wait=true
```

### Post-Uninstall Cleanup

```bash
helm uninstall $RELEASE -n $NS
kubectl delete pvc -l app.kubernetes.io/instance=$RELEASE -n $NS
```

---

## Debugging Spring Boot Config via `/actuator/configprops`

Most Camunda components (Orchestration/Zeebe, Operate, Tasklist, Identity, Optimize, Connectors, Web Modeler) are Spring Boot apps. When a configmap or env var isn't doing what you expect, verify the effective bound `@ConfigurationProperties` via the `/actuator/configprops` endpoint — this is the source of truth for what Spring actually resolved, and will reveal typos in env var names, wrong relaxed-binding forms, and values that never made it into a bean.

### Enable the endpoint + show values

By default `configprops` is not exposed and values are masked. Add these env vars to the target container:

```yaml
        - name: MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE
          value: health,info,metrics,prometheus,configprops
        - name: MANAGEMENT_ENDPOINT_CONFIGPROPS_SHOW_VALUES
          value: ALWAYS
```

> `SHOW_VALUES=ALWAYS` exposes passwords, tokens, and connection strings in the response. Use only for local/debug namespaces — never leave this on a shared or production cluster.

### Patch a running workload

StatefulSets and Deployments restart their pods automatically when `spec.template` changes.

```bash
# StatefulSet (8.8+ orchestration/zeebe, keycloak, postgresql)
kubectl edit statefulset $RELEASE-zeebe -n $NS

# Deployment (identity, optimize, connectors, console, web-modeler-*)
kubectl edit deployment $RELEASE-identity -n $NS
```

Add both entries under `spec.template.spec.containers[0].env`. Or patch non-interactively:

```bash
kubectl patch statefulset $RELEASE-zeebe -n $NS --type=json -p='[
  {"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE","value":"health,info,metrics,prometheus,configprops"}},
  {"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"MANAGEMENT_ENDPOINT_CONFIGPROPS_SHOW_VALUES","value":"ALWAYS"}}
]'

kubectl rollout status statefulset/$RELEASE-zeebe -n $NS
```

Note: a subsequent `helm upgrade` will revert these edits.

### Query the endpoint

Most components expose actuator on a dedicated management port (verified from `charts/camunda-platform-8.10/values.yaml`):

| Component | Management Port |
|-----------|-----------------|
| Orchestration / Zeebe | `9600` |
| Optimize | `8092` |
| Console | `9100` |
| Web Modeler REST API | `8091` |
| Identity / Connectors | same as app port (`8080` / `8082`) — check the container's `ports:` |

Exec into the pod (no port-forward needed):

```bash
kubectl exec -n $NS $RELEASE-zeebe-0 -- \
  curl -s http://localhost:9600/actuator/configprops | jq .

# Filter to a specific prefix (e.g., camunda.*)
kubectl exec -n $NS $RELEASE-zeebe-0 -- \
  curl -s http://localhost:9600/actuator/configprops \
  | jq '.. | objects | select(.prefix? | strings | startswith("camunda"))'
```

Or port-forward:

```bash
kubectl port-forward -n $NS statefulset/$RELEASE-zeebe 9600:9600
curl -s http://localhost:9600/actuator/configprops | jq .
```

**Always try both with and without a context path** — it's inconsistent across components. Some serve actuator at the root (`/actuator/...`), others behind a server or management context path (e.g. `/operate/actuator/...`, `/tasklist/actuator/...`, `/identity/actuator/...`, `/optimize/api/actuator/...`). If one 404s, try the other before assuming the endpoint isn't enabled:

```bash
# Probe root first, then common context paths
for p in "" "/operate" "/tasklist" "/identity" "/optimize/api" "/console" "/modeler"; do
  echo "== ${p}/actuator/configprops =="
  kubectl exec -n $NS <pod> -- curl -s -o /dev/null -w "%{http_code}\n" \
    "http://localhost:<mgmt-port>${p}/actuator/configprops"
done
```

You can also discover the correct prefix from `/actuator` itself (the discovery endpoint), which lists the absolute `href` for every exposed endpoint:

```bash
kubectl exec -n $NS <pod> -- curl -s http://localhost:<mgmt-port>/actuator | jq '._links'
# If that 404s, retry with each candidate context path above.
```

### Workflow for diagnosing a misconfiguration

1. Identify the Spring property you believe you're setting (e.g., `camunda.database.url`).
2. Apply the env vars above and wait for the pod to restart.
3. Hit `/actuator/configprops` and search for the prefix — confirm the value bound to the bean matches what you expect.
4. If it's missing or wrong, the env var name is likely mis-cased or using the wrong relaxed-binding form (Spring maps `CAMUNDA_DATABASE_URL` → `camunda.database.url`).
5. Revert the debug env vars (especially `SHOW_VALUES=ALWAYS`) once done.

---

## Headless JVM Remote Debugging with `jdb`

When a stacktrace in the logs tells you *where* code failed but not *why* — the values of locals, fields, or caller arguments at that line — attach a headless debugger to the running pod. JDWP lets you set a breakpoint and inspect state without rebuilding or redeploying.

All Camunda components listed under the configprops section above are JVM apps and work with this.

### Step 1 — Enable JDWP on the target

Prefer a **single-replica Deployment** — patching a StatefulSet triggers a rolling restart of every replica.

Add the JDWP agent via `JAVA_TOOL_OPTIONS` and expose a container port. `suspend=n` is critical — `suspend=y` hangs the JVM at startup until a debugger attaches.

```bash
kubectl -n $NS patch deploy integration-optimize --type=json -p='[
  {"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"JAVA_TOOL_OPTIONS","value":"-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005"}},
  {"op":"add","path":"/spec/template/spec/containers/0/ports/-","value":{"containerPort":5005,"name":"jdwp","protocol":"TCP"}}
]'
kubectl -n $NS rollout status deploy/integration-optimize
```

For StatefulSets (`zeebe`, `keycloak`, `postgresql`), substitute `statefulset/<name>`.

A subsequent `helm upgrade` reverts these edits. Also note: **JDWP exposes every local and field to anyone on the port — passwords, tokens, connection strings included.** Only port-forward to localhost; never expose via a Service or Ingress. Revert the patch when done.

### Step 2 — Port-forward and attach

```bash
POD=$(kubectl -n $NS get pod -l app.kubernetes.io/name=optimize -o jsonpath='{.items[0].metadata.name}')
kubectl -n $NS port-forward $POD 15005:5005 &

# -sourcepath enables jdb's `list` command; `print`/`where`/`locals` work without it
SRC=/path/to/camunda/<module>/src/main/java
jdb -sourcepath "$SRC" -attach localhost:15005
```

### Step 3 — Set a breakpoint and inspect

```
stop in <fqcn>.<method>     # breakpoint by method (matches any overload)
stop at <fqcn>:<line>       # breakpoint by line
where                        # current thread's stack
locals                       # method arguments + local vars in frame 1
print <expr>                 # evaluate; use `this.<field>` for instance fields
dump <expr>                  # print with nested fields expanded
threads                      # list all threads
thread <id>                  # switch current thread
clear <fqcn>.<method>        # remove breakpoint — do this BEFORE `cont` on hot paths
cont                         # resume
exit                         # detach; VM keeps running because suspend=n
```

Method invocation (`print service.someCall()`, `print this.getIndexAlias()`) requires the **current thread** to be suspended at the breakpoint. If jdb reports `IncompatibleThreadStateException` or `Thread not suspended`, the thread already resumed — on hot paths the BP gets re-hit on many threads and "current thread" shifts out from under you. Clear the breakpoint immediately after the first hit to avoid this.

### Driving `jdb` headlessly — use `expect`

Piping commands into `jdb` via `tail -f cmdfile | jdb` or a heredoc **races the VM state**: jdb consumes queued commands from stdin faster than the VM transitions between running and suspended, so `clear` + `cont` execute before your `print` commands and you get `Current thread isn't suspended` / `Thread not suspended` errors on every inspection.

The clean fix is to install `expect` and pattern-match on `Breakpoint hit` / the thread-suspended prompt (`<thread-name>[1]`) before sending each dump command. Without `expect`, fall back to: (a) poll the jdb output file for `Breakpoint hit` before sending any `print`; (b) send prints one at a time with short waits between; (c) send `clear` and `cont` last. Interactive `jdb` works fine and is simpler for one-off debug sessions — reach for scripting only when you need to automate repeated runs.

### Revert when done

```bash
# list the env/port array indices first so you remove the right ones
kubectl -n $NS get deploy integration-optimize -o json \
  | jq '.spec.template.spec.containers[0] | {env, ports}'
# then patch with `"op":"remove"` on the specific indices, or just wait for the next helm upgrade
```

---

## Running E2E Tests

### Generate a per-environment .env file

After deploying a namespace, generate the `.env.test` file that the Playwright test suite needs. This file contains the ingress hostname, resolved credentials (from Kubernetes secrets), Keycloak URLs, and feature flags — all auto-resolved from the live cluster.

**Via `deploy-camunda`:**

```bash
# Generate .env.test alongside the deployment
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --output-test-env

# Custom path (useful for multiple environments side by side)
deploy-camunda ... --output-test-env --output-test-env-path .env.test.eske
```

For multi-scenario parallel deployments, each scenario gets its own `.env.test.{scenario}` file automatically.

**Standalone against an existing namespace:**

```bash
./scripts/render-e2e-env.sh \
  --absolute-chart-path $PWD/charts/camunda-platform-8.9 \
  --namespace $NS \
  --output .env.test \
  --kube-context $CTX        # optional: target a specific cluster
  # --opensearch             # set IS_OPENSEARCH=true
  # --rba                    # set IS_RBA=true
  # --mt                     # set IS_MT=true
  # --run-smoke-tests        # set IS_SMOKE=true
  # -v                       # verbose: show resolved values
```

### Run the tests

**Via `deploy-camunda` (deploy + test in one step):**

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup --identity keycloak --persistence elasticsearch \
  --test-e2e          # e2e tests after deploy
  # --test-it         # integration tests instead
  # --test-all        # both
```

**Via `c8e2e` (distributed Playwright runner on Kubernetes):**

`c8e2e` (`@camunda/c8e2e`) launches sharded Playwright test pods directly on the cluster — faster and more reliable than running locally. Point it at a deployed environment by its ingress URL:

```bash
# Run against a deployed namespace
c8e2e test \
  --target SM-8.9 \
  --endpoint https://$NS.ci.distro.ultrawombat.com \
  --feature-flags smoke \
  --follow

# Full test suite with sharding
c8e2e test \
  --target SM-8.9 \
  --endpoint https://my-env.ci.distro.ultrawombat.com \
  --shards 4 \
  --follow

# Filter to specific tests
c8e2e test \
  --target SM-8.9 \
  --endpoint https://my-env.ci.distro.ultrawombat.com \
  --grep "Basic Navigation"

# OpenSearch environment with multitenancy
c8e2e test \
  --target SM-8.9 \
  --endpoint https://os-env.ci.distro.ultrawombat.com \
  --feature-flags opensearch,mt \
  --follow
```

Manage running tests:

```bash
c8e2e list                    # List active test runs
c8e2e status <run-id>         # Check status
c8e2e logs <run-id>           # Stream logs
c8e2e results <run-id>        # Download results
c8e2e cancel <run-id>         # Cancel a run
```

### Multiple environments in parallel

Deploy multiple namespaces with unique subdomains, then run `c8e2e` against each:

```bash
# Deploy two environments with unique hostnames
deploy-camunda --chart-path ./charts/camunda-platform-8.9 \
  --namespace test-es --release camunda --ingress-subdomain test-es \
  --identity keycloak --persistence elasticsearch

deploy-camunda --chart-path ./charts/camunda-platform-8.9 \
  --namespace test-os --release camunda --ingress-subdomain test-os \
  --identity keycloak --persistence opensearch

# Run e2e against each in parallel
c8e2e test --target SM-8.9 --endpoint https://test-es.ci.distro.ultrawombat.com --feature-flags smoke --follow &
c8e2e test --target SM-8.9 --endpoint https://test-os.ci.distro.ultrawombat.com --feature-flags smoke,opensearch --follow &
wait
```

---

## Reproducing a CI Test Failure Locally

See [docs/reproducing-ci-e2e-failures.md](docs/reproducing-ci-e2e-failures.md) for the step-by-step guide to pulling logs, downloading artifacts, decoding CI scenario shortnames, and spinning up an identical local environment.

---

## Troubleshooting

### Deployment fails

```
1. kubectl get pods -n $NS -o wide
2. kubectl describe pod <failing-pod> -n $NS
3. kubectl logs <pod> -n $NS --previous
4. kubectl get events -n $NS --sort-by=.lastTimestamp
5. helm status $RELEASE -n $NS
```

### Values don't look right

```
1. deploy-camunda --render-templates --render-output-dir ./debug ...
2. Or: deploy-camunda prepare-values --chart-path ... --interactive=false
3. Compare: helm get values $RELEASE -n $NS
```

### Scenario not found

```
1. ls charts/camunda-platform-8.9/test/integration/scenarios/chart-full-setup/values/
2. deploy-camunda matrix list --repo-root . --versions 8.9
```

---

## Debugging Helm Subchart Env Var Conflicts

Bitnami subcharts (Elasticsearch, PostgreSQL, Keycloak) set env vars from multiple sources in a fixed order inside the statefulset template. When the parent chart's `values.yaml` also defaults an `extraEnvVars` entry, duplicates can occur. Kubernetes uses the **last** env var when names collide, and some platforms (AKS) reject duplicates outright.

### Detect duplicates via template rendering

Always render the template and count occurrences before deploying:

```bash
helm template integration charts/camunda-platform-8.X \
  -f <your-values.yaml> \
  --show-only charts/elasticsearch/templates/master/statefulset.yaml \
  | grep -c 'YOUR_ENV_VAR_NAME'
# Should be exactly 1. If >1, trace where each occurrence comes from.
```

### Trace the sources

In a Bitnami statefulset template, env vars appear in this order:

1. **Security helper** (`elasticsearch.configure.security`) — from `security.*` values
2. **Role-level `extraEnvVars`** (e.g., `master.extraEnvVars`) — from parent chart defaults or overlays
3. **Top-level `extraEnvVars`** — from parent chart defaults or overlays

To find the parent chart's defaults, read `charts/camunda-platform-8.X/values.yaml` and search for the subchart's `extraEnvVars` entries. To inspect the subchart's helper, extract the tgz:

```bash
mkdir /tmp/subchart && tar -xzf charts/camunda-platform-8.X/charts/<subchart>.tgz -C /tmp/subchart
grep -n 'YOUR_ENV_VAR' /tmp/subchart/<subchart>/templates/_helpers.tpl
```

### Neutralize a parent chart default array

Helm deep-merges maps but **replaces arrays entirely**. To remove a parent's default `extraEnvVars` array, set it to empty in your values overlay:

```yaml
elasticsearch:
  master:
    extraEnvVars: []   # Clears the parent's default entries
```

This is cleaner than re-adding with a different value, which creates a duplicate.

---

## Bundled Elasticsearch with TLS — Auth Workaround

The `global.elasticsearch.external` flag gates ES auth env var injection in most Camunda components (8.7 and 8.8). A hard constraint in `constraints.tpl` blocks setting `external=true` when the bundled subchart is active (`elasticsearch.enabled=true`). This means **you cannot use the normal auth injection path with the bundled subchart**.

**Workaround:** Inject auth credentials via component-level `env` overrides. The exact env var names differ per version — 8.7 uses per-component names (e.g., `CAMUNDA_OPERATE_ELASTICSEARCH_USERNAME`), 8.8 uses `VALUES_ELASTICSEARCH_PASSWORD` for the unified orchestration component. See the `elasticsearch-self-signed` persistence layer values files for the complete set.

Note: `global.elasticsearch.tls.existingSecret` triggers TLS truststore volume mounts and is NOT behind the `external` gate — it works with the bundled subchart.

---

## Java 21 JKS Truststore Fix

Camunda 8.7+ containers use Java 21, which defaults `keystore.type` to PKCS12. A JKS-format truststore fails with `trustAnchors parameter must be non-empty` unless you set:

```yaml
javaOpts: "-Djavax.net.ssl.trustStoreType=jks -Djavax.net.ssl.trustStorePassword=changeit"
```

Truststore mount paths differ by component:
- Orchestration / most components: `/usr/local/camunda/certificates/externaldb.jks`
- Optimize: `/optimize/certificates/externaldb.jks`
- Connectors: `/usr/local/connectors/certificates/externaldb.jks` (must set full `JAVA_TOOL_OPTIONS` manually)

In 8.8+, the `optimize.java_tool_options_tls_env` helper auto-resolves the password from the JKS secret, so `optimize.javaOpts` only needs `-Djavax.net.ssl.trustStoreType=jks`.

---

## Upgrade Flow Pre-Cleanup (8.7 to 8.8)

When upgrading from 8.7 to 8.8 via `deploy-camunda matrix run`, certain Kubernetes resources must be deleted between steps because Helm's strategic merge patch cannot reconcile them:

| Resource | Why |
|----------|-----|
| Identity Deployment | Port naming conflict: 8.7 uses `containerPort: 8080, name: http`; 8.8 uses `8084, name: http`. Strategic merge keeps both, causing duplicate name error. |
| Ingresses | 8.7 ingresses route to services (`/operate`, `/tasklist`) that don't exist in 8.8's unified architecture. |
| PostgreSQL StatefulSets | Bitnami version changes require recreation (PVCs are preserved). |

This cleanup lives in `pre-upgrade-minor.sh` in the target version's `pre-setup-scripts/` directory.

---

## deploy-camunda Gotchas

### Env var scanning in YAML comments

The `prepare-values` command scans ALL layer files — including YAML comments — for `${...}` patterns and attempts environment variable substitution. Avoid `${...}` syntax even in comments.

### Migrator feature for upgrade scenarios

Always add `features: [migrator]` explicitly in `ci-test-config.yaml` for `upgrade-minor` scenarios. The automatic `needsMigrator()` function only activates when `ChartVersion` starts with "13", but the matrix runner doesn't set `ChartVersion`.

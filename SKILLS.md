# Operational Skills

Instructions for the primary operational tools and workflows in this repository: the `deploy-camunda` CLI, `kubectl`, debugging recipes, and the PR Ready-for-Review / review workflows.

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

### Pre-Flight Checklist

> **Shortcut:** `deploy-camunda doctor` runs the checks below automatically and prints a
> ✓/✗ checklist (config resolved, kube-context reachable, docker creds present, every var
> in the vault mapping, the scenario's values, and companion charts set). It only flags
> vars you must supply — deploy-computed ones like `CAMUNDA_HOSTNAME`/`KEYCLOAK_REALM` are
> recognized as satisfied. `deploy-camunda doctor --fix` prompts for anything missing and
> writes it to `.env`. First-time setup? `deploy-camunda config init` scaffolds config +
> `.env` (including Postgres/RDBMS dev creds) and ends with the same checklist. See
> `scripts/deploy-camunda/README.md` for the full secrets/env model. A direct
> `deploy-camunda` run also fails fast on missing inputs before touching the cluster
> (bypass with `--skip-preflight`); matrix entries do too.

Before deploying, verify these requirements. Skipping any of these is the most common source
of wasted time (pods stuck in `ImagePullBackOff`, missing ingress, helm errors):

1. **Docker credentials** — the matrix runner creates K8s pull secrets from env vars. Without them,
   pods will fail with `ImagePullBackOff` after deployment appears to succeed.
   ```bash
   # Harbor (required for all deployments)
   echo $TEST_DOCKER_USERNAME_CAMUNDA_CLOUD   # should be: ci-distribution
   echo $TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD   # should be non-empty

   # Docker Hub (only if using --ensure-docker-hub)
   echo $TEST_DOCKER_USERNAME
   echo $TEST_DOCKER_PASSWORD
   ```

2. **kubectl context** — confirm you're targeting the right cluster.
   ```bash
   kubectl config current-context
   # Expected for GKE: gke_camunda-distribution_europe-west1-b_distro-ci
   ```

3. **Helm dependencies** — must be up to date for the target chart version.
   ```bash
   make helm.dependency-update chartPath=charts/camunda-platform-8.10
   ```

4. **Ingress hostname** — for matrix deploys this is computed automatically. For single deploys,
   you must provide it via `CAMUNDA_HOSTNAME` env var or `--ingress-hostname` flag.
   ```bash
   # Matrix deploy: namespace becomes the hostname prefix automatically
   # e.g., matrix-810-keyco-inst-gke.ci.distro.ultrawombat.com

   # Single deploy: set explicitly
   export CAMUNDA_HOSTNAME=my-test-ns.ci.distro.ultrawombat.com
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
  --identity keycloak \
  --persistence opensearch-embedded \
  --features multitenancy,documentstore \
  --qa
```

Available selections:

| Flag | Values |
|------|--------|
| `--identity` | `keycloak`, `oidc`, `basic`, `hybrid` |
| `--persistence` | `elasticsearch`, `opensearch`, `rdbms`, `rdbms-oracle` |
| `--test-platform` | `gke`, `eks` |
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

**Deploying with SNAPSHOT image tags** (nightly CI pattern):

```bash
# Create a .env file with the SNAPSHOT image tags
cat > /tmp/snapshot-tags.env <<'EOF'
E2E_TESTS_ORCHESTRATION_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_CONNECTORS_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_OPTIMIZE_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_IDENTITY_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_CONSOLE_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_WEBMODELER_IMAGE_TAG=8.8
E2E_TESTS_SEARCH_ENGINE=opensearch
EOF

# Deploy a QA OpenSearch scenario with SNAPSHOT image tags
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.8 \
  --shortname-filter qaos \
  --platform gke \
  --env-file /tmp/snapshot-tags.env
```

The `qa-*` scenarios have `image-tags: true` in `ci-test-config.yaml`, which
includes `base-image-tags.yaml` (with `$E2E_TESTS_*_IMAGE_TAG` placeholders)
and excludes `values-digest.yaml`. The `--env-file` provides the actual values
for substitution via `buildScenarioEnv()`. In CI, the workflow converts the
`VALUES_CONFIG` JSON to a `.env` file using `jq` before calling `deploy-camunda`.

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

### Watch a Deploy with a Local Agent (`deploy-camunda watch`)

When a Helm install gets stuck, the default `helm install --wait --timeout 10m`
hides every signal until the timeout fires. `deploy-camunda watch` polls the
cluster on a short cadence and hands each snapshot to a local agent CLI
(Claude Code or opencode) for live diagnosis.

**ALWAYS run `deploy-camunda watch` in a second terminal alongside any deployment.** It
detects CrashLoopBackOff, ImagePullBackOff, and other failures in real time, instead
of waiting for the full helm timeout to expire.

**Quick start:**

```bash
# Terminal 1: deploy via matrix run
deploy-camunda matrix run --repo-root . --versions 8.10 \
  --shortname-filter keyco --platform gke --delete-namespace --timeout 10 --yes

# Terminal 2: watch (start immediately, it waits for pods to appear)
deploy-camunda watch \
  --namespace matrix-810-keyco-inst-gke \
  --release integration \
  --interval 30
```

For single (non-matrix) deploys:

```bash
# Terminal 1: deploy
deploy-camunda --chart-path ./charts/camunda-platform-8.10 \
  --namespace my-test --release integration --scenario chart-full-setup

# Terminal 2: watch
deploy-camunda watch --namespace my-test --release integration --interval 30
```

The watcher prints a diagnosis on each tick. Common verdicts:
- **wait** — pods are starting normally, keep polling.
- **investigate** — something looks off (slow startup, pending PVCs), diagnosis printed.
- **abort** — unrecoverable failure detected (wrong image, missing secret). Use
  `--abort-confidence 0.85` to auto-exit when the agent is confident.

**Prerequisites:** `claude` or `opencode` must be on `PATH`. The watcher does
NOT call any API directly — it shells out to whichever CLI is installed and
uses that CLI's existing auth and model configuration.

```bash
# Run alongside an active install (separate terminal):
deploy-camunda watch \
  --namespace my-test-ns \
  --release int \
  --interval 60 \
  --abort-confidence 0.85 \
  --corpus-dir ~/eval/snapshots
```

**What the watcher does each tick:**

1. `kubectl get pods,events,pvcs -n <ns> -o json` + `helm status -o json`.
2. Pipes the snapshot JSON to the agent CLI in headless mode with the
   `debug-failing-pods` skill prompt.
3. Parses the verdict JSON. Acts on `recommended_action`:
   - `wait` — keep polling silently.
   - `investigate` — print diagnosis, keep polling.
   - `abort` — print diagnosis; auto-exit non-zero only if `confidence` is
     at or above `--abort-confidence` (default 0 disables auto-abort).

**Verdict schema** (the skill must produce exactly this shape):

```json
{
  "diagnosis": "<one paragraph>",
  "causal_chain": ["t+12s FailedMount(secret/...)", "t+30s CrashLoopBackOff"],
  "confidence": 0.92,
  "recommended_action": "abort",
  "evidence": ["pod=keycloak-0", "event=FailedMount"]
}
```

**Eval workflow.** When `--corpus-dir` is set, every tick is persisted as a
JSON file containing the snapshot, raw agent output, and parsed verdict.
Replay a saved corpus to regression-test prompt or model changes:

```bash
deploy-camunda watch replay ~/eval/snapshots
# Prints a per-tick diff between recorded and freshly-replayed verdicts.
# Exits non-zero (with --strict, default) if any action class regresses.
```

Build the corpus by running `watch --corpus-dir` on at least 5 deliberately
broken installs (delete a referenced secret, mistype an image tag, undersize
a quota, set too-small JVM heap, break a CRD reference) before promoting
auto-abort to actionable.

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
  --identity keycloak --persistence opensearch-embedded

# Run e2e against each in parallel
c8e2e test --target SM-8.9 --endpoint https://test-es.ci.distro.ultrawombat.com --feature-flags smoke --follow &
c8e2e test --target SM-8.9 --endpoint https://test-os.ci.distro.ultrawombat.com --feature-flags smoke,opensearch --follow &
wait
```

---

## Reproducing a CI Test Failure Locally

See [docs/reproducing-ci-e2e-failures.md](docs/reproducing-ci-e2e-failures.md) for the step-by-step guide to pulling logs, downloading artifacts, decoding CI scenario shortnames, and spinning up an identical local environment.

---

## PR Ready-for-Review Validation

PR CI runs **tier-1 only** (~5 deploys, the `eske` baseline). The full matrix (~33 deploys) runs in the **merge queue** and rejects any PR whose diff exercises a non-baseline variant (OpenSearch, RDBMS, auth, document store, hub-legacy, ARM Elasticsearch, no-secondary-storage) and fails. Run the minimum correct scenario set locally before marking the PR Ready-for-Review.

### Prerequisites

- `deploy-camunda` — `make install.dx-tooling`
- `helm`, `kubectl` — `asdf install` (see `.tool-versions`)
- `gh` — PR and workflow-run inspection
- `crev` — automated PR reviewer at [github.com/camunda/crev](https://github.com/camunda/crev); see [docs/contribution-and-collaboration.md](docs/contribution-and-collaboration.md) and [.github/escalation-policy.md](.github/escalation-policy.md) for the project workflow
- `actionlint` — `brew install actionlint` (macOS) or `go install github.com/rhysd/actionlint/cmd/actionlint@latest`

### Identify the Surface

1. **Chart versions touched** — which `charts/camunda-platform-<v>/`.
2. **Variant axes touched** — `persistence` (ES/OS/RDBMS), `auth` (keycloak/basic/none/orgs/multitenancy), `feature` (docstr, huble, migrator), `infra` (arm, platform).
3. **Whether `eske` covers it** — `eske` = elasticsearch + keycloak on GKE. If yes, tier-1 alone is enough.

### Tier Reference

Authoritative source: `charts/camunda-platform-<v>/test/ci-test-config.yaml` (`tier:`, `enabled:`). The table below is a snapshot — re-derive with:

```bash
awk '/shortname:/{s=$2} /enabled:/{e=$2} /tier:/{t=$2; print s, e, "tier", t}' \
  charts/camunda-platform-<v>/test/ci-test-config.yaml | grep "true tier 2"
```

**Tier 1:** `eske` on every version. 8.9 covers both `install` and `upgrade-minor`.

**Enabled tier-2 (merge-queue set):**

| Version | Shortnames |
|---|---|
| 8.7  | `kemt`, `kerba`, `esoi`, `keyc`, `osem`, `entv` |
| 8.8  | `esoi`, `esarm`, `osem`, `docstr`, `entv` |
| 8.9  | `osem`, `esoi`, `kemt`, `kerba`, `keorg`, `gatkc`, `esarm`, `nosec`, `docstr`, `rdbms`, `entv` |
| 8.10 | `osem`, `keorg`, `gatkc`, `esarm`, `nosec`, `docstr`, `rdbms`, `huble`, `entv` |

`osex` (external AWS OpenSearch, #6119) and `oske` (Bitnami OpenSearch subchart, #6121) are defined but currently disabled.

**Variant decoder:**

| Shortname | Meaning |
|---|---|
| `osem` | OpenSearch embedded (OS analog to `eske`) |
| `esoi` | Elasticsearch OIDC |
| `rdbms` | RDBMS persistence |
| `ke*` | Keycloak variants: `kemt` `keycloak-mt`, `kerba` `keycloak-rba`, `keorg` `keycloak-original`, `keyc` `keycloak` (plain) |
| `gatkc` | gateway + Keycloak auth path |
| `nosec` | `noSecondaryStorage` (no Elasticsearch; `persistence: no-elasticsearch`, still uses Keycloak auth) |
| `esarm` | ARM Elasticsearch |
| `docstr` | document store feature |
| `huble` | hub-legacy feature |
| `entv` | enterprise values overlay (`values-enterprise.yaml`) |

### Select Scenarios

Default: tier-1 on every affected version. Add tier-2 entries only when the diff exercises that variant.

| Diff | Scenarios |
|---|---|
| `templates/orchestration/operate/` on 8.10 | `eske` 8.10 |
| OpenSearch values rework on 8.9 + 8.10 | `eske` + `osem` per version |
| RDBMS migrator change on 8.10 | `eske` + `rdbms` 8.10 |
| Keycloak helper change 8.9–8.10 | `eske` + each version's `ke*` set |
| Document store feature 8.8+ | `eske` + `docstr` per version |
| Hub change on 8.10 | `eske` + `huble` |
| `_helpers.tpl` change | tier-1 all versions + `nosec`, `docstr` |
| `values-enterprise.yaml` or enterprise image tags | `entv` on each version where it is defined (8.10 only until backports land) |

**Skip the matrix** for `.github/workflows/*` (run `actionlint`), `scripts/` Go tooling (`make go.test`), Dockerfile-only (`hadolint`, `docker build --target`), compose-only (`docker compose config`), docs-only.

### Run via deploy-camunda

CI uses `deploy-camunda matrix run` (Taskfile orchestration removed in PR #6016).

**Rebuild after every pull.** `deploy-camunda` tracks chart-side changes; a stale binary silently rejects new flags with `unknown flag`. The binary exposes no way to print its own build version (the existing `--version` / `-v` flag selects a *chart* version, not the binary version), so rebuild unconditionally rather than attempting to compare versions.

```bash
make install.dx-tooling
asdf reshim golang   # if using asdf
```

```bash
# PR-CI baseline
deploy-camunda matrix list --tier 1 --versions 8.10

# Full merge-queue set
deploy-camunda matrix list --versions 8.10

# Run one scenario
deploy-camunda matrix run \
  --versions 8.10 --shortname-filter eske \
  --flow-filter install --platform gke

# CI-parity overrides: --extra-helm-arg, --extra-helm-set, --namespace-override
```

#### Flow Semantics

- **`modular-upgrade-minor` is single-step** and assumes a prior install in the namespace (matches CI staging).
- **`upgrade-minor` is two-step.** Step 1 installs the *remote* previous chart `camunda/camunda-platform` from the public Helm repo (`https://helm.camunda.io`) at the previous version (e.g., `<latest-8.8>` for `--versions 8.9 --flow-filter upgrade-minor`), pinned via `versionmatrix.DefaultHelmChartRef`. The *values* are still resolved from the previous version's local layers under `charts/camunda-platform-8.8/test/integration/scenarios/chart-full-setup/`. Step 2 then `helm upgrade --force`s to the local chart.

### RFR Checklist

- [ ] Affected chart versions enumerated.
- [ ] Tier-1 (`eske`) passes on each affected version.
- [ ] Each diff-implied tier-2 scenario passes.
- [ ] `make go.update-golden-only chartPath=...` executed if templates changed, and updated goldens committed.
- [ ] `make precommit.chores` clean.
- [ ] Optional: `crev` dry-run produces no findings (see below).

### Optional Pre-RFR Self-Check (`crev`)

`crev` ([github.com/camunda/crev](https://github.com/camunda/crev)) runs automatically on every PR per [docs/contribution-and-collaboration.md](docs/contribution-and-collaboration.md) and [.github/escalation-policy.md](.github/escalation-policy.md). After review it posts a `crev/escalation` commit status plus a `human-review-required` or `ai-review-sufficient` label.

Running `crev` locally before flipping draft → Ready-for-Review is optional, not required, and can surface findings before reviewers see them:

```bash
crev <pr-url> --single --dry-run
```

`crev` defaults to dry-run and does not post comments. A typical run takes 1-5 minutes. Output terminates in a JSON object (`schema: "crev/v1"`, `findings: [...]`, `summary: "..."`). `findings.length == 0` is the green signal.

- Multi-PR sibling discovery is enabled by default; use `--single` for unrelated PRs.
- The cache key includes head SHAs and reviewer configuration, so pushing new commits invalidates the cache automatically.
- Matrix scenarios prove chart correctness; `crev` is the domain-aware review pass and the primary automated signal for non-chart PRs.

### Anti-Patterns

- Treating PR CI green as sufficient when the diff exercises a non-baseline variant; the merge queue will reject the PR.
- Hand-editing golden files instead of running `make go.update-golden-only`.
- Adding speculative tier-2 entries not exercised by the diff (consumes local capacity without coverage gain).
- Reading the tier list above without re-checking `ci-test-config.yaml`; the YAML is authoritative.
- Using the merge queue as a discovery mechanism for predictable variant breakage.
- Running the matrix on PRs that change no rendering output (workflow, Dockerfile, compose, docs, Go-tooling).

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

## End-to-End Fix Verification Workflow

Complete workflow for deploying a scenario, generating credentials, and running E2E tests
to verify a fix. This is the standard procedure for validating changes before creating a PR.

### 1. Pre-flight

```bash
# Confirm docker credentials (ask user if not set)
echo $TEST_DOCKER_USERNAME_CAMUNDA_CLOUD
echo $TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD

# Confirm kubectl context
kubectl config current-context

# Update chart dependencies
make helm.dependency-update chartPath=charts/camunda-platform-8.10
```

### 2. Deploy the scenario

Use `deploy-camunda matrix run` to deploy the exact CI scenario. Run `deploy-camunda watch`
in a second terminal to get real-time pod health diagnosis instead of manually polling
with `kubectl get pods`.

```bash
# Terminal 1: deploy
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.10 \
  --shortname-filter keyco \
  --platform gke \
  --delete-namespace \
  --timeout 10 \
  --yes

# Terminal 2: watch (start immediately)
deploy-camunda watch \
  --namespace matrix-810-keyco-inst-gke \
  --release integration \
  --interval 30
```

The namespace naming convention is `matrix-<version>-<shortname>-<flow>-<platform>`.
Use `deploy-camunda matrix list --repo-root . --versions 8.10` to find the exact
namespace for a given shortname.

### 3. Generate credentials

```bash
HELM_REPO=$(pwd)
bash "$HELM_REPO/scripts/render-e2e-env.sh" \
  --absolute-chart-path "$HELM_REPO/charts/camunda-platform-8.10" \
  --namespace matrix-810-keyco-inst-gke \
  --output /path/to/e2e-repo/.env \
  --not-ci --run-smoke-tests --verbose
```

Or use `deploy-camunda --output-test-env` to generate credentials as part of the deploy step.

### 4. Run tests (reproduce then verify)

```bash
E2E_REPO=/path/to/c8-cross-component-e2e-tests

# First: reproduce failure on main
cd $E2E_REPO && git checkout main
npx playwright test --project=chromium tests/SM-8.10/smoke-tests.spec.ts --trace on

# Then: verify fix on your branch
git checkout fix/your-branch-name
npx playwright test --project=chromium tests/SM-8.10/smoke-tests.spec.ts --trace on
```

### 5. Clean up

```bash
kubectl delete namespace matrix-810-keyco-inst-gke
```

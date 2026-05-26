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

## Runtime Remote Debugging with `jdb` and `setup-debugger`

When logs alone can't answer the question, attach a headless debugger to a running pod. JDWP exposes every local, field, and method on a Java thread; the [setup-debugger](scripts/setup-debugger/main.go) Go tool automates the patch + port-forward dance for the four components in an `integration-*` Camunda 8 release.

The `setup-debugger` workflow also enables Spring Boot's `/actuator/configprops` endpoint and snapshots the response to `configprops-<pod>.json` per component — useful for "is this property *actually* bound?" questions, often without needing JDB at all.

### When to reach for it

Four runtime scenarios where JDB pays off — and trigger phrases the LLM should pattern-match against:

- **Configuration not being read.** Logs claim a value was applied, but observed behavior contradicts it. Trigger phrases: *"is X actually being read"*, *"why isn't my config taking effect"*, *"the env var is set but…"*. (Often resolved by `configprops-*.json` alone; reach for JDB if the property isn't even declared as a `@ConfigurationProperties` bean.)
- **Logs aren't deep enough.** You need values of locals on a specific stack frame, not just what the app chose to log. Trigger phrases: *"print local variables from this stack"*, *"what's the value of X at this point"*, a stacktrace pasted with no other context.
- **Stuck or long-running process.** The app is alive but it's unclear what it's doing. Trigger phrases: *"the app is hanging"*, *"stuck in a loop"*, *"what's it doing right now"*.
- **Surprising or undocumented configuration.** You suspect an option is silently shaping behavior. Trigger phrases: *"what config options are even active"*, *"is something else overriding this"*, *"undocumented setting"*. The orchestration `standardFlowEnabled` regression caught via JDB ([branch hackday-jdb-skill](scripts/setup-debugger/main.go)) is the canonical example: an unexpected `KEYCLOAK_CLIENTS_0_TYPE=M2M` env var was overwriting Spring config from a later initializer — invisible in logs.

**When NOT to use it:** the fix is obvious from logs; the issue reproduces locally without a cluster; the question is about static code (use Read/Grep). JDB is a runtime-state tool only.

### Prerequisites

- A deployed Camunda 8 release using the `integration-*` workload naming convention. Components and their hardcoded mapping live in [scripts/setup-debugger/main.go](scripts/setup-debugger/main.go) — Identity, Optimize, Connectors as Deployments; Zeebe as a StatefulSet.
- `kubectl` context and namespace already set. Verify with `kubectl config current-context` and `kubectl config view --minify -o jsonpath='{..namespace}'`. The script reads both from the active kubeconfig and aborts if the resolved namespace is empty.
- `jdb` on PATH (ships with the JDK).
- `skopeo` on PATH (used to fetch image-revision labels for log output — non-fatal if missing).
- Build the binary once: `make install.setup-debugger` puts `setup-debugger` on `$GOPATH/bin`.

### Run it

```bash
setup-debugger
```

The tool patches each workload to inject `JAVA_TOOL_OPTIONS=-agentlib:jdwp=...:5005` plus the two Spring `MANAGEMENT_ENDPOINT_*` vars that expose `/actuator/configprops`, scales the pod 0→1 to apply the env vars, port-forwards JDWP and the management port, fetches `/actuator/configprops`, and writes `configprops-<pod>.json` to the working directory. Port-forwards stay open until SIGINT.

> **Security: JDWP is unauthenticated remote code execution.** It exposes every local, field, and method — passwords, tokens, connection strings included — and lets any client invoke arbitrary methods on the JVM.
>
> The agent is started with `address=*:5005`, so it binds **all interfaces inside the pod**, not loopback. That means:
> - Any pod in the same namespace (and any namespace lacking a NetworkPolicy) can reach `5005/tcp` and own the JVM.
> - Anyone with `kubectl exec` or `port-forward` rights to a Camunda pod can reach it from outside the cluster.
>
> Only run this against dev/integration clusters you trust. Never run it against production or against any cluster where the namespace lacks a deny-by-default NetworkPolicy. Never expose port 5005 via a Service, Ingress, or LoadBalancer. Always run `setup-debugger -cleanup` when you are done — see the Cleanup section below; leaving JDWP listening is a persistent RCE foothold.

### Local port mapping

| Component  | Local debug port | Local mgmt port | Mgmt context path |
|------------|------------------|-----------------|-------------------|
| Zeebe      | 5006             | 9600            | `/orchestration`  |
| Connectors | 5007             | 8080            | `/connectors`     |
| Optimize   | 5008             | 8092            | `/optimize`       |
| Identity   | 5009             | 8082            | `/identity`       |

All pods listen for JDWP on container port 5005; local ports are unique per component so all four can be debugged simultaneously.

### Driving a JDB session (LLM guidance)

The minimal kickoff prompt that worked in real sessions:

> "I set up a remote debugging port on **\<port\>** that you can use JDB to connect to. Use the runtime to confirm application behavior."

Canonical headless invocation:

```bash
{
  echo "stop at <fully.qualified.Class>:<line>"
  sleep 2
  echo "resume"
  sleep 25                      # wait for breakpoint to bind + hit
  echo "print someVar"
  sleep 1
  echo "print someObj.method()"
  sleep 1
  echo "where"                  # capture call stack — reveals which caller hit the bp
  sleep 2
  echo "clear <fully.qualified.Class>:<line>"  # before `cont` on hot paths
  echo "resume"
  echo "quit"
} | jdb -connect com.sun.jdi.SocketAttach:hostname=localhost,port=5006
```

Lessons from prior sessions — every one cost a retry the first time:

- **Breakpoint lines must contain executable bytecode.** A line that holds only a method signature or a closing brace yields `No code at line N`. Use the body of the method, not its declaration.
- **`sleep` is required between commands.** `jdb`'s stdin is line-buffered; piping commands without delays drops them on the floor before the breakpoint binds. Use `sleep 2` after `stop at`, `sleep 20–30` after `resume`, `sleep 1–2` between `print` calls.
- **Boxed `Boolean` getters use the `is` form.** `client.isStandardFlowEnabled()` works; `getStandardFlowEnabled()` does not. JDB's expression parser is also limited — chained constructor calls and static field references often fail with `ParseException: Name unknown`.
- **`where` is the secret weapon for ordering bugs.** When the same breakpoint hits multiple times, the call stack reveals which initializer/caller triggered it — that's how a Spring `@Order(5)` initializer was caught silently overwriting the work of an `@Order(3)` one.
- **Method invocation requires the current thread to be suspended.** `IncompatibleThreadStateException` / `Thread not suspended` means the thread already resumed. On hot paths, `clear` the breakpoint immediately after the first hit to keep it suspended.
- **For repeatable automation, use `expect`.** Piping `echo` + `sleep` races the VM state. `expect` pattern-matching on `Breakpoint hit` / the thread-suspended prompt is more robust. For one-off debugging, interactive `jdb` is simpler — reach for scripting only when you need repeated runs.

### Using the `configprops-*.json` artifacts

Each file is a Spring Boot `/actuator/configprops` snapshot for one pod. Useful jq idiom:

```bash
jq '.contexts.application.beans | to_entries[] | select(.key | test("keycloak"; "i"))' \
   configprops-integration-identity-*.json
```

When the question is *"what config did this pod actually receive?"*, this is faster than attaching JDB — provided the answer is in a declared `@ConfigurationProperties` bean.

### Cleanup

**Cleanup is a security obligation, not housekeeping.** `Ctrl-C` only stops the script's port-forwards. The injected env vars **persist on the workload** until reverted, so JDWP stays open inside every patched pod and remains reachable from any pod in the namespace until you act. Forgetting to clean up turns the debug session into a long-lived, unauthenticated RCE foothold on the cluster. Run the revert before you walk away.

**Preferred: scripted revert.**

```bash
setup-debugger -cleanup                       # remove debug env vars + restart pods
setup-debugger -cleanup -delete-configprops   # also remove configprops-*.json files
```

The cleanup path is **idempotent** — components without the debug env vars are skipped (no scale-down, no errors). Only the three vars this tool injects (`JAVA_TOOL_OPTIONS`, `MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE`, `MANAGEMENT_ENDPOINT_CONFIGPROPS_SHOW_VALUES`) are removed; any other env vars on the workload are preserved.

**Manual fallback** (workload outside `integration-*` naming, or when the script can't run):

```bash
NS=<namespace>
for kind_name in deployment/integration-identity deployment/integration-optimize \
                 deployment/integration-connectors statefulset/integration-zeebe; do
  kubectl -n "$NS" get "$kind_name" -o json \
    | jq '(.spec.template.spec.containers[0].env) |= map(select(.name | IN(
        "JAVA_TOOL_OPTIONS",
        "MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE",
        "MANAGEMENT_ENDPOINT_CONFIGPROPS_SHOW_VALUES") | not))' \
    | kubectl apply -f -
  kubectl -n "$NS" rollout restart "$kind_name"
done
```

**Last resort:** redeploy the helm release. That fully restores the chart's intended container spec.

**Verify clean state:**

```bash
kubectl -n <namespace> get deployment integration-identity \
  -o jsonpath='{.spec.template.spec.containers[0].env[*].name}' \
  | tr ' ' '\n' | grep -E 'JAVA_TOOL_OPTIONS|CONFIGPROPS' || echo "clean"
```

Repeat for the other three workloads. Each should print `clean`.

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
| 8.7  | `kemt`, `kerba`, `esoi`, `keyc`, `osem` |
| 8.8  | `esoi`, `esarm`, `osem`, `docstr` |
| 8.9  | `osem`, `esoi`, `kemt`, `kerba`, `keorg`, `gatkc`, `esarm`, `nosec`, `docstr`, `rdbms` |
| 8.10 | `osem`, `keorg`, `gatkc`, `esarm`, `nosec`, `docstr`, `rdbms`, `huble` |

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

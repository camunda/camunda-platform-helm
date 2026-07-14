---
name: cluster-debugging
description: Debug a deployed Camunda cluster — kubectl recipes for failing pods, port-forwarding, secrets; inspecting effective Spring Boot config via /actuator/configprops; headless JVM remote debugging with jdb over JDWP. Use when pods are Pending/CrashLoopBackOff/ImagePullBackOff, a configmap or env var is not taking effect, or you need to inspect live JVM state in a pod.
---

# Debugging Deployed Camunda Clusters

Throughout: `$NS` = Kubernetes namespace, `$RELEASE` = Helm release name.

## Check Deployment Health

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

## Debug Failing Pods

```bash
kubectl describe pod <pod-name> -n $NS          # Events section at bottom
kubectl logs <pod-name> -n $NS                   # Main container logs
kubectl logs <pod-name> -n $NS --previous        # Previous crash logs
kubectl logs <pod-name> -n $NS --all-containers  # All containers
kubectl get events -n $NS --sort-by=.lastTimestamp
```

Standard triage order when a deployment fails:

```
1. kubectl get pods -n $NS -o wide
2. kubectl describe pod <failing-pod> -n $NS
3. kubectl logs <pod> -n $NS --previous
4. kubectl get events -n $NS --sort-by=.lastTimestamp
5. helm status $RELEASE -n $NS
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

## Port-Forward to Services

```bash
kubectl port-forward svc/$RELEASE-zeebe-gateway 26500:26500 -n $NS  # gRPC
kubectl port-forward svc/$RELEASE-identity 8084:80 -n $NS
kubectl port-forward svc/$RELEASE-optimize 8083:80 -n $NS
kubectl port-forward svc/$RELEASE-connectors 8085:8080 -n $NS
kubectl port-forward svc/$RELEASE-console 8088:80 -n $NS
kubectl port-forward svc/$RELEASE-keycloak 18080:80 -n $NS
kubectl port-forward svc/$RELEASE-elasticsearch 9200:9200 -n $NS
```

## Manage Secrets

```bash
kubectl get secrets -n $NS
kubectl get secret <name> -n $NS -o jsonpath="{.data.<key>}" | base64 -d
kubectl get secret <name> -n $NS -o json | jq '.data | map_values(@base64d)'
```

## Namespace Lifecycle

```bash
kubectl create ns $NS --dry-run=client -o yaml | kubectl apply -f -
kubectl delete namespace $NS --wait=true
```

## Post-Uninstall Cleanup

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

Don't pipe commands into `jdb` (`tail -f cmdfile | jdb`, heredocs) — it races VM-state transitions: jdb consumes queued commands from stdin faster than the VM switches between running and suspended, so `clear` + `cont` execute before your `print` commands and every inspection fails with `Current thread isn't suspended` / `Thread not suspended`.

Fix: use `expect` and pattern-match on `Breakpoint hit` / the thread-suspended prompt (`<thread-name>[1]`) before sending each dump command. Without `expect`, fall back to: (a) poll the jdb output file for `Breakpoint hit` before sending any `print`; (b) send prints one at a time with short waits between; (c) send `clear` and `cont` last. Interactive `jdb` works fine and is simpler for one-off debug sessions — reach for scripting only when you need to automate repeated runs.

### Revert when done

```bash
# list the env/port array indices first so you remove the right ones
kubectl -n $NS get deploy integration-optimize -o json \
  | jq '.spec.template.spec.containers[0] | {env, ports}'
# then patch with `"op":"remove"` on the specific indices, or just wait for the next helm upgrade
```

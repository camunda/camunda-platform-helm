---
name: jdb-remote-debug
description: Attach a JDWP debugger to running Camunda 8 pods (Zeebe, Identity, Optimize, Connectors) using the `setup-debugger` Go tool. Use for runtime questions logs cannot answer — config not being read, locals on a stack frame, stuck/hanging processes, surprising configuration. Tool patches workloads, port-forwards JDWP + management ports, snapshots `/actuator/configprops` per component, and supports a scripted cleanup. Includes JDB session driving guidance and security warnings about JDWP exposure.
---

## Runtime Remote Debugging with `jdb` and `setup-debugger`

When logs alone can't answer the question, attach a headless debugger to a running pod. JDWP exposes every local, field, and method on a Java thread; the [setup-debugger](../../../scripts/setup-debugger/main.go) Go tool automates the patch + port-forward dance for the four components in an `integration-*` Camunda 8 release.

The `setup-debugger` workflow also enables Spring Boot's `/actuator/configprops` endpoint and snapshots the response to `configprops-<pod>.json` per component — useful for "is this property *actually* bound?" questions, often without needing JDB at all.

### When to reach for it

Four runtime scenarios where JDB pays off — and trigger phrases the LLM should pattern-match against:

- **Configuration not being read.** Logs claim a value was applied, but observed behavior contradicts it. Trigger phrases: *"is X actually being read"*, *"why isn't my config taking effect"*, *"the env var is set but…"*. (Often resolved by `configprops-*.json` alone; reach for JDB if the property isn't even declared as a `@ConfigurationProperties` bean.)
- **Logs aren't deep enough.** You need values of locals on a specific stack frame, not just what the app chose to log. Trigger phrases: *"print local variables from this stack"*, *"what's the value of X at this point"*, a stacktrace pasted with no other context.
- **Stuck or long-running process.** The app is alive but it's unclear what it's doing. Trigger phrases: *"the app is hanging"*, *"stuck in a loop"*, *"what's it doing right now"*.
- **Surprising or undocumented configuration.** You suspect an option is silently shaping behavior. Trigger phrases: *"what config options are even active"*, *"is something else overriding this"*, *"undocumented setting"*. The orchestration `standardFlowEnabled` regression caught via JDB on branch [`hackday-jdb-skill`](https://github.com/camunda/camunda-platform-helm/tree/hackday-jdb-skill) is the canonical example: an unexpected `KEYCLOAK_CLIENTS_0_TYPE=M2M` env var was overwriting Spring config from a later initializer — invisible in logs.

**When NOT to use it:** the fix is obvious from logs; the issue reproduces locally without a cluster; the question is about static code (use Read/Grep). JDB is a runtime-state tool only.

### Prerequisites

- A deployed Camunda 8 release using the `integration-*` workload naming convention. Components and their hardcoded mapping live in [scripts/setup-debugger/main.go](../../../scripts/setup-debugger/main.go) — Identity, Optimize, Connectors as Deployments; Zeebe as a StatefulSet.
- `kubectl` context and namespace already set. Verify with `kubectl config current-context` and `kubectl config view --minify -o jsonpath='{..namespace}'`. The script reads both from the active kubeconfig and aborts if the resolved namespace is empty.
- `jdb` on PATH (ships with the JDK).
- `skopeo` on PATH (used to fetch image-revision labels for log output — non-fatal if missing).
- Build the binary once: `make install.setup-debugger` puts `setup-debugger` on `$GOPATH/bin`.

### Run it

```bash
setup-debugger
```

The tool patches each workload to inject `JAVA_TOOL_OPTIONS=-agentlib:jdwp=...:5005` plus the two Spring `MANAGEMENT_ENDPOINT_*` vars that expose `/actuator/configprops`, scales the workload to 0 then back to its original replica count to apply the env vars, port-forwards JDWP and the management port, fetches `/actuator/configprops`, and writes `configprops-<pod>.json` to the working directory. Port-forwards stay open until SIGINT.

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
  sleep 25                      # tune to breakpoint binding + hit latency
  echo "print someVar"
  sleep 1
  echo "print someObj.method()"
  sleep 1
  echo "where"                  # capture call stack — reveals which caller hit the bp
  sleep 2
  echo "clear <fully.qualified.Class>:<line>"  # before resume on hot paths
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

**Manual fallback** (when the script can't run — e.g. binary missing, kubeconfig drift). Adjust the `kind_name` list if the release uses a non-`integration-*` prefix:

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
NS=<namespace>
for kind_name in deployment/integration-identity deployment/integration-optimize \
                 deployment/integration-connectors statefulset/integration-zeebe; do
  out=$(kubectl -n "$NS" get "$kind_name" \
    -o jsonpath='{.spec.template.spec.containers[0].env[*].name}' \
    | tr ' ' '\n' | grep -E 'JAVA_TOOL_OPTIONS|CONFIGPROPS')
  echo "$kind_name: ${out:-clean}"
done
```

Each line should end in `clean`.

### Troubleshooting

- **`Unable to attach to target VM` / connection refused.** Port-forward died. Check `setup-debugger` is still running; restart it. Confirm `kubectl -n <ns> get pod` shows the target pod `Running` 1/1.
- **Breakpoint never hits.** Either the line has no bytecode (use the method body, not signature/brace), or class is loaded lazily and isn't on the classpath yet — trigger the codepath via an HTTP call. JDB's `classes` command lists loaded classes.
- **`No code at line N`.** Pick a line inside the method body that contains an executable statement. JDB binds bytecode-bearing lines only.
- **JVM warmup.** First attach right after pod start may need 10–30s extra before classes load.
- **Finding class:line for a breakpoint.** Read the source from this repo, the upstream Camunda repo (`camunda/camunda`, `camunda/identity`, `camunda/connectors`), or unpack the JAR inside the pod (`kubectl exec ... -- jar tf /app/<artifact>.jar`). Match the `appVersion` reported in the pod's image label — `setup-debugger` prints it on attach.

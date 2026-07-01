<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# templates

## Purpose

This directory contains all Helm templates for Camunda 8.9 Self-Managed, organized by component. Each component directory contains the Kubernetes resources (Deployments, StatefulSets, Services, ConfigMaps, Secrets, RBAC, etc.) needed to deploy that service. The `common/` directory provides shared helper functions and global resources used across all components.

## Structure

```
templates/
├── common/                    # Shared helpers and global resources
│   ├── _helpers.tpl          # Top-level template functions (labels, annotations, selectors)
│   ├── _utilz.tpl            # Utility functions (environment variables, auth config, TLS)
│   ├── constraints.tpl        # Business logic constraints (e.g., external ES + bundled ES check)
│   ├── configmap-*.yaml      # Shared ConfigMaps (identity auth, release info, document store)
│   ├── ingress-*.yaml        # HTTP and gRPC ingress definitions
│   ├── gateway.yaml          # Gateway API resources (alpha)
│   ├── referencegrant.yaml   # Cross-namespace reference grants
│   └── extra-manifest.yaml   # Optional custom manifests
│
├── orchestration/            # Zeebe (orchestration engine)
│   ├── _helpers.tpl          # Component-specific helpers
│   ├── statefulset.yaml      # Zeebe StatefulSet (clustering, persistence, init containers)
│   ├── service.yaml          # ClusterIP service (inter-pod communication)
│   ├── service-headless.yaml # Headless service (DNS SRV records)
│   ├── configmap.yaml        # Zeebe configuration (broker settings, data retention)
│   ├── serviceaccount.yaml   # RBAC identity
│   ├── grpcroute.yaml        # Gateway API gRPC routing
│   ├── poddisruptionbudget.yaml
│   ├── persistentvolumeclaim.yaml
│   └── files/                # Template files embedded as ConfigMap data
│
├── identity/                 # Identity service (Keycloak wrapper)
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── keycloak-service.yaml # Exposes Keycloak subchart service
│   ├── configmap.yaml
│   ├── serviceaccount.yaml
│   ├── persistentvolumeclaim.yaml
│   ├── httproute.yaml
│   └── constraints.tpl       # Identity-specific constraints (e.g., Keycloak DB validation)
│
├── console/                  # Camunda Console (management UI)
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── serviceaccount.yaml
│   └── httproute.yaml
│
├── connectors/               # Connectors runtime (extensibility)
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── service-headless.yaml
│   ├── configmap.yaml
│   ├── serviceaccount.yaml
│   ├── persistentvolumeclaim.yaml
│   ├── httproute.yaml
│   └── files/                # Connector configuration templates
│
├── optimize/                 # Optimize (analytics and business insights)
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── serviceaccount.yaml
│   ├── persistentvolumeclaim.yaml
│   ├── httproute.yaml
│   └── files/
│
├── web-modeler/              # Web Modeler (process & decision modeling)
│   ├── _helpers.tpl
│   ├── deployment-restapi.yaml      # REST API deployment
│   ├── deployment-websockets.yaml   # WebSocket service deployment
│   ├── service-restapi.yaml
│   ├── service-websockets.yaml
│   ├── configmap-restapi.yaml
│   ├── configmap-websockets.yaml
│   ├── configmap-shared.yaml        # Shared Web Modeler config
│   ├── secret-shared.yaml           # Shared secrets (e.g., encryption keys)
│   ├── serviceaccount.yaml
│   ├── persistentvolumeclaim-restapi.yaml
│   └── httproute.yaml
│
├── service-monitor/          # Prometheus monitoring (optional)
│   ├── *-service-monitor.yaml       # ServiceMonitor for each component
│
└── z/                        # Special: Helm ordering hack (DO NOT EDIT)
    └── 1/2/3/4/5/6/7/8/      # Nested dirs force alphabetical ordering of manifests
```

## Key Patterns

### Component Directory Pattern

Most component directories follow this standard:

- **`_helpers.tpl`** — Component-specific template helpers (e.g., container environment, probes, security context). Imports from `common/_helpers.tpl`.
- **`deployment.yaml` or `statefulset.yaml`** — Main workload resource. Defines containers, volumes, init containers, health checks, and resource requests.
- **`service.yaml`** — ClusterIP service for inter-pod communication.
- **`serviceaccount.yaml`** — RBAC: ServiceAccount and associated Roles/RoleBindings (if needed).
- **`configmap.yaml`** — Application configuration (typically a properties or YAML file).
- **`persistentvolumeclaim.yaml`** — Data persistence (optional; only if component needs stateful storage).
- **`httproute.yaml` or `grpcroute.yaml`** — Gateway API routing (optional; requires Kubernetes 1.25+).

### Common/ Directory Special Purpose

The `common/` directory is shared by all components and provides:

- **Global helpers** (`_helpers.tpl`) — Release name, labels, annotations, selectors, image pull secrets.
- **Utility functions** (`_utilz.tpl`) — Environment variable construction, auth config injection, TLS setup.
- **Constraints** (`constraints.tpl`) — Business logic validation (e.g., "Elasticsearch cannot be both external and bundled").
- **Global ConfigMaps** — Identity auth config (`configmap-identity-auth.yaml`), release info (`configmap-release.yaml`), document store setup.
- **Networking** — Ingress and Gateway API resources for HTTP/gRPC traffic.

### The `z/` Directory (Helm Ordering Hack)

The nested `z/1/2/3/4/5/6/7/8/` directory is a workaround for Helm's manifest ordering. Helm sorts templates alphabetically and renders them in that order. By placing templates in `z/<number>/`, you force a specific render order **without renaming files**. This is critical for Zeebe DNS SRV records, which must be created after services.

**Do not edit or move files in `z/`.** The structure is intentional and fragile.

## For AI Agents

### Working In This Directory

1. **Always read the component's `_helpers.tpl` first** — it defines the local context and how the component integrates with global helpers.

2. **Understand template gating** — Many blocks are conditionally rendered based on `values.yaml` flags:
   - `global.elasticsearch.external` — Controls ES auth injection (see `constraints.tpl`)
   - `global.elasticsearch.tls.existingSecret` — Triggers TLS truststore setup
   - `global.multitenancy.enabled` — Activates tenant isolation
   - Component-specific toggles (e.g., `identity.enabled`, `optimize.enabled`) — Enable/disable entire components

3. **Test after edits** — Use the Go unit tests in `../test/unit/<component>/`:
   ```bash
   cd ../test/unit
   go test ./<component>/... -run <TestName>
   ```

4. **Update golden files only for intentional changes**:
   ```bash
   make go.update-golden-only chartPath=../..
   ```

5. **Keep diffs focused** — Edit one component or helper at a time. Version-scoped changes are easier to review and test.

6. **Render templates locally to verify**:
   ```bash
   helm template integration ../.. -f <values-file> | grep -A 10 <resource-name>
   ```

### Common Editing Tasks

#### Adding an Environment Variable to a Component

1. Open the component's `_helpers.tpl` and locate the `<component>.env` helper.
2. Add your variable using the pattern established in that file (typically with `tpl` to allow interpolation).
3. Add the corresponding value to `../values.yaml` under the component's config section.
4. Test with `go test ./<component>/...` to ensure the variable renders correctly.

**Example:**
```yaml
# In identity/_helpers.tpl, inside the containerEnv helper
- name: MY_NEW_VAR
  value: {{ .Values.identity.myNewVar | quote }}
```

#### Adding Startup/Liveness Probes

1. Open the component's `deployment.yaml` or `statefulset.yaml`.
2. Add `startupProbe`, `livenessProbe`, or `readinessProbe` blocks under the container spec.
3. Test the probe endpoint to ensure the component responds correctly before committing.

#### Configuring TLS for a Component

1. The component's template typically includes a conditional block gated by `global.elasticsearch.tls.existingSecret`.
2. Add volume mounts for the TLS secret and set `JAVA_TOOL_OPTIONS` or equivalent environment variable to point to the truststore.
3. See `common/_utilz.tpl` for the `javaToolOptions` helper function.

#### Adding a Gateway API Route

1. Create an `httproute.yaml` or `grpcroute.yaml` in the component directory.
2. Use the global `gateway` values and the component's service name.
3. Include proper parent references and hostname matching.
4. Test with `helm template` and verify the route is created.

### Common Patterns in Templates

**Conditional Rendering (Feature Gates):**
```yaml
{{- if .Values.identity.enabled }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "orchestration.fullname" . }}-identity
{{- end }}
```

**Importing Helpers:**
```yaml
{{- include "common.labels" . | nindent 4 }}
{{- include "orchestration.env" . | nindent 12 }}
```

**Environment Variable Construction (Tpl + Interpolation):**
```yaml
env:
  - name: RELEASE_NAME
    value: {{ .Release.Name }}
  - name: SECRET_REF
    valueFrom:
      secretKeyRef:
        name: {{ include "common.fullname" . }}-secret
        key: password
```

**PVC Template Pattern:**
```yaml
{{- if and .Values.persistence.enabled (eq .Values.persistence.type "dynamic") }}
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: {{ .Values.persistence.storageClass }}
  resources:
    requests:
      storage: {{ .Values.persistence.size }}
{{- end }}
```

### Testing Guidelines

**Before committing any template changes:**

1. **Lint the chart**:
   ```bash
   make helm.lint chartPath=../..
   ```

2. **Run component-specific tests**:
   ```bash
   cd ../test/unit && go test ./<component>/...
   ```

3. **Render and inspect output**:
   ```bash
   helm template my-release ../.. -f <values-overlay> | kubectl apply --dry-run=client -f -
   ```

4. **Run the full test suite** (before PR):
   ```bash
   cd ../test/unit && go test ./...
   ```

### Debugging Template Issues

**Template rendering errors:**
```bash
helm template my-release ../.. 2>&1 | grep -i error
```

**Find where a value is used:**
```bash
grep -r "\.Values\.myKey" .
```

**Check Helm debug output:**
```bash
helm template my-release ../.. --debug | head -50
```

**Verify function helpers exist:**
```bash
grep "define.*myHelper" common/_helpers.tpl
```

## Component-Specific Notes

### Orchestration (Zeebe)

- **Largest and most complex component** — handles clustering, persistence, init containers, and monitoring.
- StatefulSet with multiple init containers to set up data directories and cluster formation.
- Headless service required for DNS SRV records (Zeebe broker discovery).
- Data persisted to Elasticsearch; auth and TLS configuration critical.
- Test file: `../test/unit/orchestration/statefulset_test.go` — study this first.

### Identity (Keycloak)

- Wraps the Bitnami Keycloak subchart.
- Keycloak runs as a separate Deployment with its own service.
- PostgreSQL database is a dependency (separate subchart).
- Realm and client setup automated via `common/configmap-identity-auth.yaml`.
- See `constraints.tpl` for validation rules (e.g., Keycloak must have DB).

### Web Modeler

- **Only component with two Deployments** — REST API and WebSocket server.
- Separate ConfigMaps for REST API, WebSocket, and shared config.
- Shared secrets for encryption keys.
- Both deployments share a PostgreSQL database.

### Service-Monitor

- Prometheus ServiceMonitor resources (only created if monitoring is enabled).
- One monitor per component (Zeebe, Identity, Connectors, Optimize, Operate, Tasklist).
- Requires Prometheus Operator to be installed in the cluster.

## Dependencies

### Local Helpers

- `common/_helpers.tpl` — Always imported by component templates. Provides `common.labels`, `common.fullname`, `common.annotations`, etc.
- `common/_utilz.tpl` — Imported for utility functions like `camundaPlatform.javaToolOptions`, `camundaPlatform.auth`, etc.

### Subchart Dependencies

Referenced in `../Chart.yaml`:

- **keycloak** (alias: `identityKeycloak`) — Identity provider. Deployed as separate Deployment; Identity component wraps it.
- **postgresql** (aliases: `identityPostgresql`, `webModelerPostgresql`) — Databases for Identity and Web Modeler.
- **elasticsearch** — Search and indexing for Operate, Tasklist, Optimize. Optional (can be external).
- **common** — Bitnami library chart with shared helpers (vendored).

### External Kubernetes APIs

- **apps/v1** — Deployment, StatefulSet
- **v1** — Service, ConfigMap, Secret, ServiceAccount, PersistentVolumeClaim
- **rbac.authorization.k8s.io/v1** — Role, RoleBinding
- **gateway.networking.k8s.io/v1** — HTTPRoute, GRPCRoute (optional; Kubernetes 1.25+)
- **monitoring.coreos.com/v1** — ServiceMonitor (optional; requires Prometheus Operator)

## Troubleshooting

**"Template not rendering"**
- Check for syntax errors: `helm template my-release ../.. 2>&1 | head -20`
- Ensure helper function is defined: `grep "define.*helperName" common/_helpers.tpl`

**"Wrong value injected"**
- Verify the value path: `helm template my-release ../.. --values <file> | grep -C 3 <key>`
- Check for typos in `values.yaml` keys.

**"PVC not created"**
- Check `persistence.enabled` in values.
- Verify storage class exists in the cluster: `kubectl get storageclass`

**"Service not found by DNS"**
- Ensure Service resource was rendered: `helm template my-release ../.. | grep "kind: Service"`
- Check service name matches in Deployments: `grep "serviceName:" orchestration/statefulset.yaml`

---

**Last updated:** 2026-07-01  
**Parent:** `../AGENTS.md` (camunda-platform-8.9 chart-level guidance)  
**Related:** `../test/unit/AGENTS.md` (test patterns), Helm documentation

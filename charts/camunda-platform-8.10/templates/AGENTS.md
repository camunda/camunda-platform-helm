<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# templates

## Purpose

This directory contains all Helm templates for the Camunda Platform 8.10 chart. Templates are organized by component (orchestration, identity, web-modeler, connectors, optimize, console) plus shared utilities. Each component defines Deployments/StatefulSets, Services, ConfigMaps, ServiceAccounts, and optional routing (HTTPRoute, GRPCRoute). The `common/` directory provides global helpers and base templates; `z/` is a depth-hack for controlling template rendering order.

## Subdirectories

| Directory | Component | Key Templates | Purpose |
|-----------|-----------|---------------|---------|
| **orchestration/** | Zeebe + Operate + Tasklist | `statefulset.yaml` (19KB), `_helpers.tpl`, `service.yaml`, `service-headless.yaml`, `configmap.yaml`, `grpcroute.yaml`, `httproute.yaml`, `poddisruptionbudget.yaml`, `serviceaccount.yaml`, `files/` | Core process engine and task management. StatefulSet pattern with init containers and shared data volumes. |
| **identity/** | Keycloak + Identity Service | `deployment.yaml` (14KB), `_helpers.tpl`, `configmap.yaml` (17KB), `service.yaml`, `httproute.yaml`, `persistentvolumeclaim.yaml`, `serviceaccount.yaml`, `constraints.tpl`, `keycloak-service.yaml` | OAuth2/OIDC provider and user/role management. Includes Keycloak init containers and federation setup. |
| **web-modeler/** | Web Modeler (REST + WebSocket) | `deployment-restapi.yaml` (16KB), `deployment-websockets.yaml` (12KB), `_helpers.tpl`, `configmap-restapi.yaml`, `configmap-websockets.yaml`, `configmap-shared.yaml`, `service-restapi.yaml`, `persistentvolumeclaim-restapi.yaml`, `secret-shared.yaml`, `httproute.yaml` | BPMN/DMN diagram editor with two-tier architecture. Separate REST API and WebSocket deployments. |
| **connectors/** | Connector Runtime | `deployment.yaml` (9.9KB), `_helpers.tpl`, `configmap.yaml`, `service.yaml`, `service-headless.yaml`, `httproute.yaml`, `persistentvolumeclaim.yaml`, `serviceaccount.yaml`, `files/` | Integration runtime for outbound connectors (HTTP, Slack, Google Sheets). |
| **optimize/** | Optimize (Analytics) | `deployment.yaml` (22KB), `_helpers.tpl`, `configmap.yaml`, `service.yaml`, `httproute.yaml`, `persistentvolumeclaim.yaml`, `serviceaccount.yaml`, `files/` | Business process analytics and reporting. Connects to Elasticsearch for data queries. |
| **console/** | Console (Multi-Cluster Management) | `deployment.yaml` (12KB), `_helpers.tpl`, `configmap.yaml` (1.9KB), `service.yaml`, `httproute.yaml`, `serviceaccount.yaml` | Management UI for multi-cluster and multi-tenant scenarios. |
| **camunda-hub/** | Camunda Hub | `_helpers.tpl` (1KB) | Element templates and extensions. Minimal configuration. |
| **common/** | Shared Utilities | `_helpers.tpl` (47KB), `_utilz.tpl`, `constraints.tpl` (34KB), `configmap-documentstore.yaml`, `configmap-identity-auth.yaml`, `configmap-release.yaml`, `gateway.yaml`, `ingress-grpc.yaml`, `ingress-http.yaml`, `referencegrant.yaml`, `extra-manifest.yaml` | Global helper functions for labels, selectors, annotations. Elasticsearch/TLS gating. Ingress and gateway configuration. |
| **service-monitor/** | Prometheus Metrics | `connectors-service-monitor.yaml`, `console-service-monitor.yaml`, `identity-service-monitor.yaml`, `optimize-service-monitor.yaml`, `orchestration-service-monitor.yaml`, `web-modeler-service-monitor.yaml` | ServiceMonitor CRDs for metrics scraping (Prometheus operator). One per component. |
| **z/** | Depth-Hack (Load Order) | `1/2/3/4/5/6/7/8/z_compatibility_helpers.tpl` | Nested directories force Helm to render in controlled order. Not functional templates — do not edit. |

## For AI Agents

### Working In This Directory

1. **Never edit or reason about the z/ directory.** It's a namespace-depth hack to force Helm template rendering order. The actual code (`z_compatibility_helpers.tpl`) is a compatibility layer; changes to z/ structure break the ordering mechanism.

2. **Component templates are independent.** Each component (orchestration, identity, web-modeler, etc.) can be enabled/disabled via `.Values.<component>.enabled`. A template guarded by `{{- if .Values.orchestration.enabled }}` will not render if the flag is false.

3. **Use component fullname helpers consistently.** Every component defines a helper that follows the pattern:
   ```yaml
   {{ define "orchestration.fullname" -}}
   {{ include "camundaPlatform.componentFullname" (dict
       "componentName" "zeebe"
       "componentValues" .Values.orchestration
       "context" $
   ) -}}
   {{ end }}
   ```
   Always use `{{ include "orchestration.fullname" . }}` for resource names, not hardcoded strings.

4. **Read component _helpers.tpl before editing templates.** Each component's `_helpers.tpl` defines:
   - Fullname and naming conventions (e.g., `orchestration.fullname`, `orchestration.brokerName`, `orchestration.gatewayName`).
   - Version and component labels (with backward-compatibility notes for 8.7→8.8 migration).
   - Security context helpers.
   - Custom env var injection patterns.

5. **ConfigMaps in templates are often generated — do not hand-edit.** Many ConfigMaps (e.g., `orchestration/configmap.yaml`, `identity/configmap.yaml`) template large YAML blocks or properties files. Changes to template syntax or values can break them. Always verify with `make helm.template` or `make helm.dry-run` before committing.

6. **Constraints are enforced in common/constraints.tpl.** Hard constraints like "cannot set `elasticsearch.external: true` if `elasticsearch.enabled: true`" are defined in `common/constraints.tpl`, not in individual component templates. Violations cause rendering to fail (intentional).

7. **TLS and Elasticsearch gating patterns are in common/_helpers.tpl.** Global helpers define:
   - `global.elasticsearch.external` — blocks ES auth env var injection if bundled subchart is active.
   - `global.elasticsearch.tls.existingSecret` — triggers TLS truststore mounts and `JAVA_TOOL_OPTIONS` injection in most components.
   - These gates are evaluated in component templates, not in helpers, so check the actual component template (e.g., `orchestration/statefulset.yaml`) to see where the gate is applied.

8. **HTTPRoute and GRPCRoute are conditional.** Ingress is controlled by `global.ingress.enabled` (Ingress) or `global.ingress.className` (Kubernetes native). These routes may be Kubernetes Gateway API routes (for modern clusters) or fallback to Ingress. Check the actual template to see which is used.

9. **Service discovery is via DNS names, not IPs.** Kubernetes services are referenced by their fully qualified DNS name: `<service-name>.<namespace>.svc.cluster.local`. All service references in ConfigMaps and env vars should use these DNS names, not hardcoded IPs.

10. **Never assume template paths are stable across versions.** This is version 8.10; template organization may differ in 8.11+. Always verify relative paths before referencing.

### Common Patterns

#### Template Naming Convention
```
{{ include "<component>.fullname" . }}
  Examples:
  - {{ include "orchestration.fullname" . }}  → "release-zeebe"
  - {{ include "identity.fullname" . }}        → "release-identity"
  - {{ include "connectors.fullname" . }}      → "release-connectors"
```

#### Conditional Component Rendering
```yaml
{{- if .Values.orchestration.enabled }}
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ include "orchestration.fullname" . }}
  namespace: {{ .Release.Namespace | quote }}
  # ...
{{- end }}
```

#### Label Pattern (All Resources)
```yaml
metadata:
  labels:
    {{- include "camundaPlatform.componentLabels" (dict
        "componentName" "orchestration"
        "componentValuesKey" "orchestration"
        "context" $
    ) | nindent 4 }}
```

#### StatefulSet Init Container Pattern (Orchestration)
```yaml
spec:
  template:
    spec:
      initContainers:
        - name: setup-zeebe-data
          image: busybox
          command: ['sh', '-c', 'mkdir -p /var/lib/zeebe/data && chown -R 1000:1000 /var/lib/zeebe']
          volumeMounts:
            - name: zeebe-data
              mountPath: /var/lib/zeebe/data
      containers:
        - name: zeebe
          # ...
        - name: operate
          # ...
```

#### ConfigMap Env Var Reference
```yaml
envFrom:
  - configMapRef:
      name: {{ include "orchestration.fullname" . }}-config
```

#### Secret Env Var Reference
```yaml
env:
  - name: ZEEBE_SECURITY_TLSENABLED
    valueFrom:
      secretKeyRef:
        name: {{ .Values.global.tls.secretName }}
        key: tlsEnabled
```

#### TLS Mount Pattern (When `global.elasticsearch.tls.existingSecret` is set)
```yaml
{{- if .Values.global.elasticsearch.tls.existingSecret }}
volumeMounts:
  - name: es-tls-ca
    mountPath: /etc/elasticsearch/tls
    readOnly: true
volumes:
  - name: es-tls-ca
    secret:
      secretName: {{ .Values.global.elasticsearch.tls.existingSecret }}
      items:
        - key: ca.crt
          path: ca.crt
{{- end }}
```

#### Service Discovery (DNS-based)
```yaml
# In ConfigMap or env var, reference other services by DNS name:
ELASTICSEARCH_URL: "http://elasticsearch.{{ .Release.Namespace }}.svc.cluster.local:9200"
KEYCLOAK_URL: "http://{{ include "identity.fullname" . }}.{{ .Release.Namespace }}.svc.cluster.local:8080"
```

### Template Editing Workflow

1. **Identify target component** — e.g., `orchestration`, `identity`, `web-modeler`.
2. **Read component _helpers.tpl** — understand naming, labels, and custom helpers.
3. **Read the actual template** — e.g., `orchestration/statefulset.yaml` — to see how values are applied.
4. **Make focused edits** — change only what's needed; preserve existing patterns and whitespace trimming.
5. **Verify locally:**
   ```bash
   make helm.dependency-update chartPath=charts/camunda-platform-8.10
   make helm.template chartPath=charts/camunda-platform-8.10
   # or
   make helm.dry-run chartPath=charts/camunda-platform-8.10
   ```
6. **Run unit tests** — snapshot tests will catch rendering changes:
   ```bash
   cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/... -v
   ```
7. **Update golden files only if intentional:**
   ```bash
   make go.update-golden-only chartPath=charts/camunda-platform-8.10
   ```

### Key Helpers in common/_helpers.tpl

| Helper | Purpose | Example |
|--------|---------|---------|
| `camundaPlatform.name` | Chart name | `camunda-platform` |
| `camundaPlatform.fullname` | Full app name | `release-camunda-platform` |
| `camundaPlatform.componentFullname` | Component-scoped full name | `release-zeebe` |
| `camundaPlatform.labels` | Global labels (includes chart version) | Assigned to all resources |
| `camundaPlatform.matchLabels` | Selector-safe labels (no transient fields) | Used in `spec.selector.matchLabels` |
| `camundaPlatform.componentLabels` | Component + global labels | Component resource labels |
| `camundaPlatform.componentExtraLabels` | Component name + version | Added to resource labels |
| `camundaPlatform.versionLabel` | Version label value | Component version from chart or values |

### Gating Patterns

**Elasticsearch External:**
```yaml
# In component template:
{{- if .Values.global.elasticsearch.external }}
# Use external ES auth
env:
  - name: ZEEBE_ELASTICSEARCH_USERNAME
    valueFrom:
      secretKeyRef:
        name: {{ .Values.global.elasticsearch.auth.secretName }}
        key: username
{{- end }}
```

**Elasticsearch TLS:**
```yaml
# In component template:
{{- if .Values.global.elasticsearch.tls.existingSecret }}
# Mount TLS cert, inject JAVA_TOOL_OPTIONS
volumeMounts:
  - name: es-tls-ca
    mountPath: /etc/elasticsearch/tls
{{- end }}
```

**Component Enabled:**
```yaml
# In component root template file:
{{- if .Values.orchestration.enabled }}
apiVersion: apps/v1
kind: StatefulSet
# ...
{{- end }}
```

### Files Subdirectory

Some components have a `files/` subdirectory (e.g., `orchestration/files/`, `connectors/files/`, `optimize/files/`). These contain static config files that are mounted into containers via ConfigMap:

```yaml
# In component template:
volumes:
  - name: config-files
    configMap:
      name: {{ include "orchestration.fullname" . }}-config-files
      items:
        - key: application.properties
          path: application.properties
```

The actual files are templated via Helm's `tpl` function if they contain dynamic values.

### Pre-rendered Ingress vs Gateway API

The `common/` directory provides both:
- `ingress-http.yaml` — Traditional Kubernetes Ingress (deprecated but widely supported).
- `ingress-grpc.yaml` — Ingress for gRPC traffic (if applicable).
- Component-level `httproute.yaml` / `grpcroute.yaml` — Gateway API routes (modern Kubernetes 1.25+).

Check `global.ingress.className` and `global.ingress.enabled` in values to understand which path is active.

<!-- MANUAL: Update this file when new components are added, template patterns change, or gating logic is modified. -->

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# camunda-platform-8.10

## Purpose

This is the **Camunda 8.10 alpha Helm chart** — the bleeding-edge version where new features land first.
It deploys the complete Camunda 8 Self-Managed platform: orchestration (Zeebe/Operate/Tasklist), Identity (Keycloak), Web Modeler, Connectors, Optimize, and Console.
All Camunda components are internalized; only Keycloak, PostgreSQL, and Elasticsearch are managed as sub-charts.

Chart version: `15.0.0-alpha0` | App version: `8.10.x`

## Key Files

- **Chart.yaml** — Chart metadata, dependencies (keycloak-24, identity-postgresql-16, web-modeler-postgresql-15, elasticsearch-21, common-2), and version info.
- **values.yaml** — Primary configuration (386KB). Defines all component settings, subchart overrides, and feature flags.
- **values.schema.json** — JSON Schema validation for values (generated; do not edit by hand).
- **values.schema.extra.json** — Custom schema extensions and property descriptions (merge into schema.json for validation).
- **values-enterprise.yaml** — Enterprise-only feature flags and settings (Optimize, Identity advanced options).
- **values-latest.yaml** — Pins latest app versions for each component (useful for pre-release testing).
- **values-digest.yaml** — Component digest/image tag overrides for air-gapped or custom registry scenarios.
- **values-bitnami-legacy.yaml** — Bitnami subchart compatibility overrides (legacy, rarely needed).
- **values-local.yaml** — Local development defaults (minimal resource requests, small replicas).
- **README.md** — User-facing documentation: architecture, installation, configuration notes, parameters table.
- **RELEASE-NOTES.md** — Generated changelog (Conventional Commits format).

## Subdirectories

### `templates/`
Helm templates grouped by component:

- **orchestration/** — Zeebe, Operate, and Tasklist (unified in 8.8+).
  - `statefulset.yaml` — Core orchestration StatefulSet with shared init containers and sidecar patterns.
  - `_helpers.tpl` — Reusable template helpers for orchestration (security contexts, env vars, TLS configs).
  - Other files: service, headless service, ConfigMaps, gRPC/HTTP routes, PDB.

- **identity/** — Identity service and Keycloak integration.
  - `deployment.yaml`, `service.yaml`, `configmap.yaml`, `serviceaccount.yaml`.
  - Manages user/role federation, OAuth2/OIDC provider setup.

- **web-modeler/** — Web Modeler (diagram editor).
  - `deployment-restapi.yaml`, `deployment-websockets.yaml` — Two-tier architecture (REST + WebSocket).
  - `service-restapi.yaml`, `service-websockets.yaml`.
  - PostgreSQL init jobs and secrets management.

- **connectors/** — Connector runtime.
  - `deployment.yaml`, `service.yaml`, `configmap.yaml`, `serviceaccount.yaml`.

- **optimize/** — Optimize analytics and reporting.
  - `deployment.yaml`, `service.yaml`, `configmap.yaml`, `serviceaccount.yaml`.

- **console/** — Console management UI.
  - `deployment.yaml`, `service.yaml`, `configmap.yaml`, `serviceaccount.yaml`.

- **camunda-hub/** — Camunda Hub integration (element templates, extensions).
  - `configmap.yaml`.

- **common/** — Shared utilities and templates (inherited from common-2 subchart).
  - `_helpers.tpl` — Global helpers (labels, selectors, annotations).
  - `secrets.yaml` — Certificate/TLS secret management.

- **service-monitor/** — Prometheus ServiceMonitor for metrics scraping (optional).
  - `servicemonitor.yaml`.

- **z/** — Utility templates (load NOTES.txt).
  - `NOTES.txt` — Post-install instructions and quick-start guide.

### `test/`
Test suite for the chart:

- **unit/** — Go-based snapshot tests using `testify/suite` and golden file comparisons.
  - `common/` — Tests for shared template helpers.
  - `connectors/` — Connector deployment and config tests.
  - `console/` — Console component tests.
  - `identity/` — Identity and Keycloak tests.
  - `optimize/` — Optimize deployment tests.
  - `orchestration/` — Zeebe/Operate/Tasklist StatefulSet and config tests (largest suite).
  - `web-modeler/` — Web Modeler deployment and PostgreSQL tests.
  - `testhelpers/` — Reusable test utilities (golden file paths, assertions).
  - `utils/` — Kubernetes object builders and validators.
  - `README.md` — Test harness documentation.

- **integration/** — Integration and E2E test scenarios.
  - `scenarios/` — Reusable value sets for different deployment patterns.
    - `chart-full-setup/values/` — Complete platform configuration templates.
      - `persistence/` — Storage backend configurations (elasticsearch, postgresql for web-modeler).
      - `features/` — Optional feature toggles (migrator, tls, etc.).
    - `pre-setup-scripts/` — Pre-install and pre-upgrade shell scripts (TLS secrets, data migrations).

- **e2e/** — End-to-end tests (if present; validates full deployment lifecycle).

- **ci-test-config.yaml** — CI matrix definition.
  - Defines test scenarios: identity type, persistence backend, Kubernetes platforms, upgrade flows, features.
  - Each scenario maps to a name, enabled flag, persistence choice, platform targets, flows (install/upgrade), and optional features.

### `openshift/`
OpenShift-specific overrides:

- **values.yaml** — OpenShift networking (routes instead of Ingress), security context constraints (SCC).
- **openshift-tuned.yaml** — Performance tuning profiles for OpenShift clusters.

### `charts/`
Vendored subchart dependencies:

- **keycloak-24/** — Keycloak (Identity Provider).
- **identity-postgresql-16/** — PostgreSQL for Identity.
- **web-modeler-postgresql-15/** — PostgreSQL for Web Modeler.
- **elasticsearch-21/** — Elasticsearch (search/analytics backend).
- **common-2/** — Shared Helm utilities.

These are managed via `Chart.yaml` dependencies and `make helm.dependency-update`.

## For AI Agents

### Working In This Directory

1. **Always check the target version first.** 8.10 has unified orchestration templates (8.8+ pattern), not separate component directories.
2. **Understand gating patterns.** Many template blocks are conditional:
   - `global.elasticsearch.external` gates ES auth env vars (hard constraint: cannot set `external=true` when `elasticsearch.enabled=true`).
   - `global.elasticsearch.tls.existingSecret` triggers TLS truststore mounts and `JAVA_TOOL_OPTIONS` injection (works with bundled subchart).
   - `identity.enabled` gates Identity/Keycloak deployment and OAuth2 provider setup.
   - `webModeler.enabled` gates Web Modeler REST and WebSocket deployments.

3. **Subchart value merging is deep-merge for maps, full-replace for arrays.** Overriding an `extraEnvVars` array? You must replace it fully, not append.

4. **Template path patterns:**
   - Orchestration: `templates/orchestration/statefulset.yaml`, `_helpers.tpl`.
   - Other components: `templates/<component>/deployment.yaml`, `service.yaml`, `configmap.yaml`.
   - Shared: `templates/common/_helpers.tpl`, `templates/common/secrets.yaml`.

5. **Read the actual template before writing values overrides.** Template behavior differs subtly across 8.7→8.8→8.10:
   - 8.10 Operate init container may or may not apply `tpl` to environment variables — check before assuming string interpolation.
   - Keycloak init containers, roles, and federation setup differ between chart versions.

6. **Never edit golden files by hand.** Use `make go.update-golden-only chartPath=charts/camunda-platform-8.10`.

7. **Keep diffs small and version-scoped.** If editing templates, touch only the target component and version directory.

### Testing Requirements

#### Unit Tests (Go)
Run snapshot tests after any template change:

```bash
# Full chart test suite
make go.test chartPath=charts/camunda-platform-8.10

# Single component (e.g., orchestration)
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/... -v

# Single test by name
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/... -run TestStatefulSetTemplate
```

Tests use golden files in `test/unit/<component>/golden/` as snapshots. When template output changes intentionally:

```bash
# Update only golden files (leaves Go code unchanged)
make go.update-golden-only chartPath=charts/camunda-platform-8.10

# Faster update without cleanup
make go.update-golden-only-lite chartPath=charts/camunda-platform-8.10
```

#### Dependency Update (Required Before Tests)
Sub-charts must be fetched and locked before testing:

```bash
make helm.dependency-update chartPath=charts/camunda-platform-8.10
```

This generates/updates `Chart.lock` and populates `charts/` with vendored sub-charts.

#### Linting
```bash
# Lint chart (strict mode)
make helm.lint chartPath=charts/camunda-platform-8.10

# Check Go formatting
make go.fmt

# Check/add Apache license headers to Go test files
make go.addlicense-check chartPath=charts/camunda-platform-8.10
make go.addlicense-run chartPath=charts/camunda-platform-8.10
```

#### Local Template Rendering
```bash
# Render all templates locally
make helm.template chartPath=charts/camunda-platform-8.10

# Dry-run install (does not actually install)
make helm.dry-run chartPath=charts/camunda-platform-8.10
```

### Common Patterns

#### Conditional Component Rendering
Most components are gated by top-level `enabled` flags in `values.yaml`:

```yaml
# values.yaml
identity:
  enabled: true
  # ... identity config
webModeler:
  enabled: true
  # ... web modeler config
```

Templates check these flags:
```yaml
{{- if .Values.identity.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "camunda-platform.identity.fullname" . }}
# ...
{{- end }}
```

#### StatefulSet Orchestration Pattern
The orchestration StatefulSet (Zeebe + Operate + Tasklist) uses a unified template with init containers:

- **Init containers:** Database setup, schema migration, configuration bootstrap.
- **Main containers:** Zeebe broker, Operate, Tasklist (often multiple per pod for resource efficiency).
- **Volumes:** Zeebe data, config maps, secrets, TLS mounts.
- **Affinity:** Hard pod anti-affinity (spread across nodes) or soft (zone spread).

Example from `templates/orchestration/statefulset.yaml`:
```yaml
initContainers:
  - name: setup-zeebe-data
    image: busybox
    command: ['sh', '-c', 'mkdir -p /var/lib/zeebe/data && chown -R 1000:1000 /var/lib/zeebe']
    volumeMounts:
      - name: zeebe-data
        mountPath: /var/lib/zeebe/data
containers:
  - name: zeebe
    image: {{ .Values.zeebe.image }}
    # ...
  - name: operate
    image: {{ .Values.operate.image }}
    # ...
```

#### Environment Variable Injection
Components receive environment variables from multiple sources:

1. **From ConfigMap:**
   ```yaml
   envFrom:
     - configMapRef:
         name: {{ include "camunda-platform.identity.fullname" . }}-config
   ```

2. **From Secret (sensitive values):**
   ```yaml
   env:
     - name: ZEEBE_SECURITY_TLSENABLED
       valueFrom:
         secretKeyRef:
           name: {{ .Values.global.tls.secretName }}
           key: tlsEnabled
   ```

3. **Direct env vars:**
   ```yaml
   env:
     - name: CAMUNDA_OPTIMIZE_ELASTICSEARCH_URL
       value: "{{ .Values.elasticsearch.url }}"
   ```

**Gotcha:** Bitnami subcharts inject env vars in a fixed order (security helper → role-specific → top-level). Last one wins if there are duplicates. Always check the actual template to see where your override needs to go.

#### TLS Certificate Injection
When `global.elasticsearch.tls.existingSecret` is set:

```yaml
{{- if .Values.global.elasticsearch.tls.existingSecret }}
volumeMounts:
  - name: es-tls-ca
    mountPath: /etc/elasticsearch/tls
    readOnly: true
env:
  - name: JAVA_TOOL_OPTIONS
    value: "-Djavax.net.ssl.trustStore=/etc/elasticsearch/tls/ca.crt"
volumes:
  - name: es-tls-ca
    secret:
      secretName: {{ .Values.global.elasticsearch.tls.existingSecret }}
      items:
        - key: ca.crt
          path: ca.crt
{{- end }}
```

This works with both bundled and external Elasticsearch.

#### PostgreSQL SubChart Overrides
Identity and Web Modeler use PostgreSQL subcharts. Override via parent chart values:

```yaml
# values.yaml
identityPostgresql:
  enabled: true
  auth:
    username: identity-user
    password: "{{ secretKeyRef }}"  # Or reference a pre-created secret
  primary:
    persistence:
      size: 20Gi
```

Helm deep-merges map values, so you can override individual keys without losing parent defaults. **But arrays are replaced entirely.** If the parent's `extraEnvVars` has a default, setting `extraEnvVars: []` removes it completely.

#### Elasticsearch External vs Bundled
Two deployment modes:

1. **Bundled subchart (default):**
   ```yaml
   elasticsearch:
     enabled: true
   global:
     elasticsearch:
       external: false
   ```

2. **External Elasticsearch:**
   ```yaml
   elasticsearch:
     enabled: false
   global:
     elasticsearch:
       external: true
       url: "https://elasticsearch.external:9200"
       auth:
         username: elastic
         password: "{{ secretKeyRef }}"
   ```

**Hard constraint:** Cannot set `external: true` while `elasticsearch.enabled: true`. The template enforces this via `constraints.tpl`.

### Integration Test Scenarios
Test scenarios are defined in `test/ci-test-config.yaml`:

```yaml
- name: elasticsearch-self-signed-upgrade
  enabled: false
  identity: keycloak
  persistence: elasticsearch-self-signed
  platforms: [gke]
  flows: [upgrade-minor]
  features: [migrator]
  shortname: esss
```

**Key fields:**
- `name` — Scenario identifier.
- `enabled` — Whether CI runs this scenario.
- `identity` — Identity provider setup (keycloak, ldap, etc.).
- `persistence` — Storage backend (values file: `test/integration/scenarios/chart-full-setup/values/persistence/<name>.yaml`).
- `platforms` — Target Kubernetes platforms (gke, eks, aks, openshift).
- `flows` — Test flows (install, upgrade-minor, upgrade-major).
- `features` — Optional value file overrides (mapped to `values/features/<name>.yaml`). Common: `migrator` (adds migration jobs for upgrades).
- `shortname` — 4-char abbreviation for namespace generation.

**Pre-install scripts:** If a scenario needs namespace-level setup before `helm install` (e.g., TLS secrets), create a shell script:
```bash
charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/pre-install-<scenario-name>.sh
```

The runner detects and executes it after namespace creation, before install.

**Pre-upgrade scripts:** For cleanup between upgrade steps, create:
```bash
charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/pre-upgrade-minor.sh
```

Example: deleting 8.7 Identity deployments (port conflict: 8.7 uses `containerPort: 8080`, 8.8 uses `8084`, both named `http`). Strategic merge patch keeps both, causing duplicate port name error. Pre-upgrade script deletes the old deployment.

## Dependencies

### Helm SubCharts (Vendored)
- **keycloak-24** (`identityKeycloak.enabled`) — Identity provider, OAuth2/OIDC server.
- **identity-postgresql-16** (`identityPostgresql.enabled`) — Identity service database.
- **web-modeler-postgresql-15** (`webModelerPostgresql.enabled`) — Web Modeler database.
- **elasticsearch-21** (`elasticsearch.enabled`) — Search and analytics backend (Zeebe events, Operate/Optimize data).
- **common-2** — Shared Helm utilities (always included).

All are managed via `Chart.yaml` dependency declarations and `Chart.lock` pinning.

### Kubernetes Version
- **Minimum:** 1.20+
- **Tested:** Typically 1.27+ (as of 2026-07).

### External Services (if not using bundled subcharts)
- **Keycloak:** For external Identity provider setup (if `identityKeycloak.enabled: false`).
- **PostgreSQL:** For external Identity or Web Modeler databases.
- **Elasticsearch:** For external search backend.

### Camunda Components (Internalized)
All internalized — no external sub-chart dependencies:
- **Zeebe** — Orchestration engine (process execution, event streaming).
- **Operate** — Human-centric task management, process monitoring.
- **Tasklist** — Task inbox and assignment interface.
- **Identity** — User/role management, federation.
- **Web Modeler** — BPMN/DMN diagram editor.
- **Connectors** — Integration runtime (HTTP, Slack, Google Sheets, etc.).
- **Optimize** — Business analytics and reporting.
- **Console** — Multi-cluster management console.

### Related Documentation
- **Parent AGENTS.md:** `../AGENTS.md` — Repository-wide build, test, and code style guidelines.
- **CI/CD Agents:** `.github/AGENTS.md` — GitHub Actions workflows, matrix generation, release process.
- **User Documentation:** `README.md` — Installation, configuration, parameters.
- **Camunda Docs:** https://docs.camunda.io/docs/self-managed/about-self-managed/ — Official deployment and architecture guides.
- **Helm Chart Repo:** https://github.com/camunda/camunda-platform-helm — Source repository.

<!-- MANUAL: Update this file when chart version increments, new components are added, or test scenarios change. -->

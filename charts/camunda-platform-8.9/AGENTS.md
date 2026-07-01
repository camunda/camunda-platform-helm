<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# camunda-platform-8.9

## Purpose

This is the Camunda 8.9 Self-Managed Helm chart (v14.1.0), one of the two most actively maintained chart versions in the repository. It packages all Camunda 8.9 components (Zeebe, Operate, Tasklist, Connectors, Optimize, Identity, Web Modeler, Console) with vendored Keycloak as the identity provider, Elasticsearch for data persistence, and optional PostgreSQL for Identity and Web Modeler databases.

## Key Facts

- **Chart Version:** 14.1.0
- **Camunda Version:** 8.9.x (app version varies by component, see RELEASE-NOTES.md)
- **Components:** Zeebe (StatefulSet), Operate, Tasklist, Connectors, Optimize, Identity, Web Modeler, Console
- **Dependencies:** Keycloak (IDP), PostgreSQL (Identity, Web Modeler), Elasticsearch (search/index)
- **Configuration Surface:** 3,529 lines in `values.yaml`, validated by 7,164-line JSON Schema in `values.schema.json`
- **Test Coverage:** Go unit tests (organized by component), e2e Playwright tests, CI scenario matrix with 10+ integration scenarios
- **OpenShift Support:** Template overrides in `openshift/` for restricted-v2 SCC compatibility

## Key Files

- **Chart.yaml** — Chart metadata, dependencies (Keycloak, PostgreSQL, Elasticsearch, common helpers)
- **values.yaml** — Primary configuration surface with extensive documentation and component-scoped defaults
- **values.schema.json** — JSON Schema validation for values (enforced in CI, IDE integration)
- **Chart.lock** — Locked subchart versions for reproducible deployments
- **CHANGELOG.md** — Version history and breaking changes (85KB)
- **RELEASE-NOTES.md** — Current release info: image versions, supported platforms, version matrix
- **test/ci-test-config.yaml** — CI scenario matrix: defines unit test packages and integration test scenarios
- **test/unit/** — Go unit test packages organized by component
- **test/integration/scenarios/** — Integration test setup: values overlays, pre-install/upgrade scripts
- **test/e2e/** — Playwright e2e tests (if present)

## Subdirectories

### templates/

Helm templates organized by component. Each component has its own directory with resources (Deployments, StatefulSets, Services, ConfigMaps, Secrets, etc.):

- **templates/orchestration/** — Zeebe StatefulSet, Init containers, configuration, monitoring
- **templates/identity/** — Identity Deployment, Keycloak integration, LDAP/OIDC setup
- **templates/console/** — Camunda Console Deployment
- **templates/operate/** — Operate Deployment, data volume setup
- **templates/tasklist/** — Tasklist Deployment
- **templates/connectors/** — Connectors Deployment, extensibility
- **templates/optimize/** — Optimize Deployment
- **templates/web-modeler/** — Web Modeler REST API and WebSocket Deployments
- **templates/service-monitor/** — Prometheus ServiceMonitor resources (if monitoring enabled)
- **templates/common/** — Shared helpers and utilities
  - `_helpers.tpl` — Top-level template functions (release name, labels, annotations, etc.)
  - `_utilz.tpl` — Utility functions for environment variables, auth config, TLS setup
  - `constraints.tpl` — Business logic constraints (e.g., Elasticsearch external mode incompatible with bundled subchart)
  - `configmap-*.yaml` — Shared ConfigMaps (identity auth, release info, document store)
  - `ingress-*.yaml`, `gateway.yaml` — Networking (HTTP/gRPC ingress, API Gateway)
  - `extra-manifest.yaml` — Optional custom manifests
  - `referencegrant.yaml` — Cross-namespace reference grants (Gateway API support)
- **templates/z/** — Nested "Z" directory structure (legacy organizational pattern, used for DNS SRV records and other advanced features)

### test/

Go-based test suite with unit and integration tests:

- **test/unit/** — Go unit tests (table-driven, snapshot-based)
  - **common/** — Common/global configuration tests
  - **identity/** — Identity component tests (Keycloak, auth modes, env setup)
  - **console/** — Console component tests
  - **orchestration/** — Zeebe StatefulSet tests (most complex; init containers, clustering, persistence)
  - **connectors/** — Connectors Deployment tests
  - **optimize/** — Optimize tests
  - **web-modeler/** — Web Modeler REST API and WebSocket deployment tests
  - **testhelpers/** — Shared test utilities and assertions
  - **utils/** — Test helpers for parsing, validation, golden file comparison
- **test/integration/** — Integration test scenarios and fixtures
  - **scenarios/** — Values overlays and pre-setup scripts for each test scenario
  - **scenarios/chart-full-setup/values/persistence/** — Persistence layer overlays (elasticsearch, opensearch, rdbms, etc.)
  - **scenarios/pre-setup-scripts/** — Pre-install and pre-upgrade scripts for scenario-specific prerequisites (TLS secrets, namespace setup, etc.)
- **test/e2e/** — End-to-end Playwright tests (if present)
- **test/ci-test-config.yaml** — Matrix definition: unit test packages and integration test scenarios

### charts/

Pre-packaged subchart dependencies (vendored as tarballs):

- **common-2.37.0.tgz** — Common helper chart (labels, annotations, resource defaults)
- **elasticsearch-21.6.3.tgz** — Elasticsearch subchart (optional, can be disabled for external ES)
- **keycloak-24.9.1.tgz** — Keycloak subchart (vendored, used as Identity provider via `identityKeycloak` alias)
- **postgresql-16.7.27.tgz** — PostgreSQL for Identity (optional, can be disabled for external DB)
- **web-modeler-postgresql-15.5.28.tgz** — PostgreSQL for Web Modeler (optional)

**Note:** These are Bitnami subcharts. Helm's values merge for subcharts is a **deep merge for maps but full replace for arrays**. See parent AGENTS.md for env var patterns and array override gotchas.

### openshift/

Template overrides and compatibility layer for OpenShift deployment:

- Security context adaptations (remove `runAsUser`, `runAsGroup`, `fsGroup` for restricted-v2 SCC compatibility)
- Component-specific OpenShift templates (if needed)
- Enable via `global.compatibility.openshift.adaptSecurityContext: force`

## For AI Agents

### Working In This Directory

1. **Always identify your target version first** — this is 8.9. Never assume templates are identical to 8.8, 8.10, or other versions.
2. **Run `make helm.dependency-update chartPath=charts/camunda-platform-8.9`** before testing, linting, or rendering templates. Subcharts must be extracted.
3. **Read the specific version's templates before editing** — template logic differs between versions (8.7 vs. 8.8+ have structural differences in orchestration, env var injection, etc.).
4. **Prefer `make` targets** — they ensure local behavior matches CI:
   - Linting: `make helm.lint chartPath=charts/camunda-platform-8.9`
   - Testing: `make go.test chartPath=charts/camunda-platform-8.9`
   - Rendering: `make helm.template chartPath=charts/camunda-platform-8.9`
5. **Keep diffs small and version-scoped** — changes to templates, values, or tests should be isolated to this chart version unless the issue is cross-version.
6. **Never edit golden files by hand** — use `make go.update-golden-only chartPath=charts/camunda-platform-8.9` to regenerate snapshots after intentional template changes.
7. **Verify constraints and gating patterns** — many template blocks are gated behind value flags (e.g., `global.elasticsearch.external`, `global.elasticsearch.tls.existingSecret`, multitenancy, OpenShift mode). Understand the coupling before editing.

### Testing Requirements

**Unit Tests (Go):**

```bash
# Run all unit tests for this chart
cd /Users/eamonn.moloney/workspaces/camunda-platform-helm/charts/camunda-platform-8.9/test/unit
go test ./...

# Run tests for a single component package
go test ./orchestration/...   # Zeebe orchestration (largest/most complex)
go test ./identity/...         # Identity/Keycloak
go test ./connectors/...       # Connectors

# Run a single test by name (most important pattern for iteration)
go test ./orchestration/... -run TestStatefulSetTemplate

# Update golden snapshots (required after intentional template changes)
make go.update-golden-only chartPath=charts/camunda-platform-8.9
```

**Linting:**

```bash
# Lint this chart (strict mode)
make helm.lint chartPath=charts/camunda-platform-8.9

# Check Go formatting
make go.fmt

# Add Apache license headers to new Go test files
make go.addlicense-run chartPath=charts/camunda-platform-8.9
```

**Rendering:**

```bash
# Render all templates with defaults
make helm.template chartPath=charts/camunda-platform-8.9

# Dry-run an install (validates resource structure)
make helm.dry-run chartPath=charts/camunda-platform-8.9
```

**Integration Tests (if running CI scenarios locally):**

```bash
# View available scenarios
cat test/ci-test-config.yaml | grep -A 5 "name:"

# Deploy a scenario (requires a cluster and deploy-camunda CLI)
# Example: deploy-camunda run elasticsearch keycloak gke
```

### Common Patterns

**Template Gating (Conditional Rendering):**

Many template blocks are gated behind value flags to handle feature toggles and deployment modes:

- **`global.elasticsearch.external: true/false`** — Controls whether Elasticsearch auth env vars are injected. When `true`, Elasticsearch is external (auth required). When `false`, the bundled ES subchart is used. **Hard constraint:** `constraints.tpl` forbids `external=true` when `elasticsearch.enabled=true` (cannot use both).
- **`global.elasticsearch.tls.existingSecret`** — When set, configures TLS truststore volume mounts and `JAVA_TOOL_OPTIONS` injection for secure ES communication. Works with both bundled and external ES.
- **`global.multitenancy.enabled: true/false`** — Enables multitenancy features across applicable components (auth, data isolation, etc.).
- **`global.compatibility.openshift.adaptSecurityContext`** — Toggles OpenShift SCC adaptation (`force`, `disabled`).
- **Authentication method** — `global.security.authentication.method` (`basic`, `oidc`, etc.) gates which auth components are deployed.

**Helm Subchart Value Merging:**

Helm performs **deep merge for maps** but **full array replacement** for array values. This is critical for Bitnami subcharts (Elasticsearch, PostgreSQL, Keycloak):

```yaml
# Parent chart (values.yaml default)
elasticsearch:
  master:
    extraEnvVars:
      - name: SOME_VAR
        value: "default"

# Your overlay attempting to override
elasticsearch:
  master:
    extraEnvVars: [{name: SOME_VAR, value: "override"}]
    # Result: the entire array is replaced — parent default is gone.
    # To add to the array, list all entries.
```

**Environment Variable Chains in Bitnami StatefulSets:**

Bitnami subcharts apply env vars in a fixed order (security helpers → role-level extras → global extras). When duplicate env var names exist, **the last one wins**. To diagnose:

```bash
helm template integration charts/camunda-platform-8.9 \
  -f your-values.yaml \
  --show-only charts/elasticsearch/templates/master/statefulset.yaml \
  | grep -c 'ELASTICSEARCH_ENABLE_REST_TLS'
# Should be exactly 1. If >1, there's a duplicate.
```

**Component Enable/Disable:**

Many components have a top-level toggle (e.g., `identity.enabled: true/false`, `optimize.enabled: true/false`). Disabling a component skips all its templates. This is useful for minimal deployments.

**Test Golden Files:**

Unit tests use snapshot (golden file) comparison. Golden files live in `test/unit/<component>/golden/` and are auto-updated when you run:

```bash
make go.update-golden-only chartPath=charts/camunda-platform-8.9
```

After changes, always run the full test suite before committing:

```bash
cd charts/camunda-platform-8.9/test/unit && go test ./...
```

**Values Documentation:**

Every value in `values.yaml` has a comment block starting with `@param` or similar annotations. These annotations feed into the JSON Schema (`values.schema.json`) and Helm Hub documentation. Always update comments when adding/changing values.

## Dependencies

### Direct Chart Dependencies

Declared in `Chart.yaml`:

1. **keycloak** (alias: `identityKeycloak`, v24.x.x)
   - File path: `../keycloak-24`
   - Condition: `identityKeycloak.enabled`
   - Role: Identity provider (OIDC, LDAP support)
   - Vendored: Yes (tgz in `charts/`)

2. **postgresql** (alias: `identityPostgresql`, v16.x.x)
   - File path: `../identity-postgresql-16`
   - Condition: `identityPostgresql.enabled`
   - Role: Database for Keycloak and Identity component
   - Vendored: Yes

3. **web-modeler-postgresql** (v15.x.x)
   - File path: `../web-modeler-postgresql-15`
   - Condition: `webModelerPostgresql.enabled`
   - Role: Database for Web Modeler
   - Vendored: Yes

4. **elasticsearch** (v21.6.3)
   - File path: `../elasticsearch-21`
   - Condition: `elasticsearch.enabled`
   - Role: Search and indexing for Operate, Tasklist, Optimize
   - Vendored: Yes

5. **common** (v2.x.x, helper chart)
   - File path: `../common-2`
   - Role: Shared Helm helpers (labels, annotations, etc.)
   - Vendored: Yes

### Transitive Image Dependencies

Non-Camunda images (used by subcharts):

- **bitnamilegacy/elasticsearch:8.18.0** — Search engine (bundled ES subchart)
- **bitnamilegacy/postgresql:14.18.0-debian-12-r0** — Identity database
- **bitnamilegacy/postgresql:15.10.0-debian-12-r2** — Web Modeler database
- **bitnamilegacy/os-shell:12-debian-12-r43** — Init/helper containers
- **busybox:1.36** — Lightweight utilities

**Enterprise alternatives** (Camunda Enterprise only):
- **registry.camunda.cloud/vendor-ee/elasticsearch:8.19.13**
- **registry.camunda.cloud/vendor-ee/postgresql:18.3.0-debian-12-r***
- **registry.camunda.cloud/keycloak-ee/keycloak:26.5.6**

### Camunda Component Images

Versioned in RELEASE-NOTES.md:

- **docker.io/camunda/camunda:8.9.2** — Zeebe (orchestration engine)
- **docker.io/camunda/connectors-bundle:8.9.2** — Connectors runtime
- **docker.io/camunda/console:8.9.32** — Console (management UI)
- **docker.io/camunda/identity:8.9.2** — Identity service
- **docker.io/camunda/optimize:8.9.2** — Optimize (analytics)
- **docker.io/camunda/web-modeler-restapi:8.9.2** — Web Modeler REST API
- **docker.io/camunda/web-modeler-websockets:8.9.2** — Web Modeler WebSocket service
- **registry.camunda.cloud/camunda/keycloak:26.3.3** — Keycloak (IDP)

### Build and CI Tools

Defined in parent repo `.tool-versions`:

- **Go 1.26.1** — Test and tooling language
- **Helm 3.20.1** — Chart templating and deployment
- **kubectl 1.27.16** — Kubernetes client (validation, context)
- **kind 0.31.0** — Local Kubernetes for testing (optional)
- **kustomize 5.8.1** — Overlay tooling (optional, used in some tests)
- **yq 4.52.5** — YAML manipulation
- **jq 1.8.1** — JSON querying
- **yamllint 1.38.0** — YAML linting
- **bats 1.11.0** — Bash integration tests

Install all tools:

```bash
make tools.asdf-install
```

<!-- MANUAL: Add chart-specific gotchas, upgrade patterns, known issues below this line -->

## Chart-Specific Patterns & Gotchas

### Orchestration (Zeebe) StatefulSet

The `orchestration/` package is the largest and most complex in this chart:

- **Init containers** — Prepare Zeebe data directories, handle cluster formation
- **Clustering** — Replication factor, broker IDs, election settings
- **Data persistence** — Elasticsearch external auth config, TLS setup
- **Monitoring** — ServiceMonitor integration, JMX exporter setup

When editing orchestration templates, always check the test file `test/unit/orchestration/statefulset_test.go` first to understand expected behavior.

### Component-Specific Auth Modes

Each component can override the global authentication method:

- **Identity** — `identity.enabled`, `identity.auth` (basic, keycloak, oidc hybrid)
- **Operate/Tasklist** — `operate.auth`, `tasklist.auth` (inherit global default or override)
- **Console** — `console.auth` (same)

Template gating differs per component. Always check the component's `_helpers.tpl` or test file before adding auth logic.

### Keycloak (identityKeycloak) Subchart

Keycloak is used as the OIDC/LDAP provider. Key points:

- Configured via `identityKeycloak` section in values.yaml
- Uses PostgreSQL (identityPostgresql) for state
- Realm and client setup is automated (see `common/configmap-identity-auth.yaml`)
- Default credentials are in `identityKeycloak.auth` (change in production)

### Elasticsearch Subchart Array Gotchas

When using bundled Elasticsearch, be careful with array overrides in values:

```yaml
elasticsearch:
  master:
    extraEnvVars: []          # ← Replaces parent default array entirely
  coordinating:
    extraEnvVars: []          # ← Same
```

To add to (not replace) a Bitnami subchart array, list all entries or use the parent chart's merge logic (deploy-camunda CLI does this for you). Direct Helm values overlay will replace the whole array.

### Multitenancy Mode

When `global.multitenancy.enabled: true`:

- Identity/Keycloak must be deployed (Identity provides tenant isolation)
- Zeebe, Operate, Tasklist all participate in tenant isolation
- Some features are disabled or reconfigured (e.g., RBA gets special handling)
- Integration tests include multitenancy scenarios (grep for `-mt` in `ci-test-config.yaml`)

### OpenShift Compatibility Mode

When `global.compatibility.openshift.adaptSecurityContext: force`:

- Security context fields are stripped from Deployments/StatefulSets
- OpenShift will assign UIDs/GIDs automatically (restricted-v2 SCC)
- Test with: `make helm.template chartPath=charts/camunda-platform-8.9` and grep for `securityContext`

### Pre-Install and Pre-Upgrade Scripts

For scenarios requiring prerequisites:

1. **Pre-install scripts** — Executed after namespace creation, before `helm install`
   - Example: `test/integration/scenarios/pre-setup-scripts/pre-install-elasticsearch-self-signed.sh`
2. **Pre-upgrade scripts** — Executed between Step 1 (old version) and Step 2 (new version) in upgrade flows
   - Example: `pre-upgrade-minor.sh` may delete Identity Deployment (port naming conflict in 8.7→8.8)

Discovery is automatic via `versionmatrix.HasPreInstallScript()` in Go code.

### Values Schema Validation

The `values.schema.json` is auto-validated in CI. When adding or changing values:

1. Update the comment in `values.yaml` with `@param` or similar
2. Regenerate schema (depends on external tooling; check CI for command)
3. Verify schema is valid JSON
4. Test with: `helm template` on your values overlay (Helm validates against schema)

---

**Last updated:** 2026-07-01  
**Maintained by:** Camunda platform team  
**Related docs:** `../AGENTS.md` (repo-level), `RELEASE-NOTES.md`, `CHANGELOG.md`, [Camunda 8.9 docs](https://docs.camunda.io/)

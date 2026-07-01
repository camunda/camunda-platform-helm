<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# charts

## Purpose

This directory contains all Helm charts for Camunda 8 Self-Managed platform:
- **Versioned platform charts** (`camunda-platform-8.X`): Complete deployments for each minor version (8.3–8.10).
- **Dependency/subchart vendors** (`common-2`, `elasticsearch-21`, `keycloak-24`, etc.): Shared helpers and external integrations.
- **Version registry** (`chart-versions.yaml`): Support matrix (alpha, standard, extended, end-of-life).

Each versioned chart is **self-contained and must never be assumed identical to others**. Always verify the target chart before editing.

## Key Files/Dirs

| Path | Purpose |
|------|---------|
| `camunda-platform-8.{3..10}/` | Standalone chart for that minor version. Contains `Chart.yaml`, `values.yaml` (3.5KB+), `values.schema.json` (388KB), `templates/`, `test/`, `openshift/`. |
| `common-2/` | Bitnami library chart (v2.37.0) providing shared helpers. Vendored as dependency. |
| `elasticsearch-21/` | Bitnami Elasticsearch chart (v21.6.3, app v8.18.0). Vendored as optional dependency. |
| `keycloak-24/` | Keycloak chart (v24.x.x). Identity provider for Camunda. Vendored as optional. |
| `identity-postgresql-{15,16}/` | PostgreSQL variants for Identity service. Bitnami subcharts. |
| `web-modeler-postgresql{,-14,-15}/` | PostgreSQL variants for Web Modeler. Bitnami subcharts. |
| `chart-versions.yaml` | Version support matrix: alpha (8.10), standard (8.9, 8.8, 8.7), extended (8.6, 8.5, 8.4, 8.3), eol (8.2, 8.1, 8.0). |

## For AI Agents

### Working In This Directory

1. **Always identify the target chart version first.** Do not assume `8.10` changes apply to `8.9` or earlier. Check the target Chart.yaml and read the specific version's `values.yaml` and templates.

2. **Run dependency updates before testing or linting:**
   ```bash
   make helm.dependency-update chartPath=charts/camunda-platform-8.X
   ```
   This resolves all `file://` references to vendored subcharts and generates `Chart.lock`.

3. **Keep diffs version-scoped.** Use separate commits for separate versions. Example:
   - Commit 1: `feat(8.10): add new podAnnotation to statefulset`
   - Commit 2: `feat(8.9): add new podAnnotation to statefulset` (if needed in prior version)

4. **Lint before commit:**
   ```bash
   make helm.lint chartPath=charts/camunda-platform-8.X
   ```

5. **Never edit golden snapshot files by hand.** Use the Makefile:
   ```bash
   make go.update-golden-only chartPath=charts/camunda-platform-8.X
   ```

6. **Test single functions first, then the full chart:**
   ```bash
   # Single test by name (fastest feedback)
   cd charts/camunda-platform-8.X/test/unit && go test ./orchestration/... -run TestStatefulSetTemplate
   
   # Full chart tests
   make go.test chartPath=charts/camunda-platform-8.X
   ```

### Common Patterns

#### Template Gating (Conditional Rendering)

Many template blocks are gated behind feature flags. Always read the specific version's template to understand the gate logic:

- **`global.elasticsearch.external`**: Controls ES authentication injection (8.7, 8.8). Hard constraint in `constraints.tpl` blocks setting `external=true` when bundled subchart is active. For auth with bundled ES, use component-level `env` overrides instead.
- **`global.elasticsearch.tls.existingSecret`**: Triggers TLS truststore mounts and `JAVA_TOOL_OPTIONS` in most components. Works with both external and bundled ES.
- Template blocks **differ per version** (e.g., 8.7 Operate init container applies `toYaml` but NOT `tpl` on `operate.env`, so `{{ .Release.Name }}` in `valueFrom.secretKeyRef.name` is literal). Always read the actual version's template.

#### Helm Subchart Value Overrides

Helm deep-merges maps but **replaces arrays wholesale**. This matters for vendor chart overrides:

```yaml
# Parent chart default:
elasticsearch:
  master:
    extraEnvVars:
      - name: SOME_VAR
        value: "default"

# Your overlay:
elasticsearch:
  master:
    extraEnvVars: []        # Replaces entire array — parent default is gone
```

To neutralize a parent's default array, set it to `[]`. To add entries, replace the entire array.

#### Environment Variable Precedence in Statefulsets

Bitnami charts set env vars in order:
1. Security helper (from `security.*` values)
2. `<role>.extraEnvVars` (e.g., `master.extraEnvVars`)
3. Top-level `extraEnvVars`

**Last one wins.** If duplicates exist, diagnose with:
```bash
helm template integration charts/camunda-platform-8.X \
  -f values.yaml \
  --show-only charts/elasticsearch/templates/master/statefulset.yaml \
  | grep -c 'ELASTICSEARCH_ENABLE_REST_TLS'
# Should be exactly 1
```

#### Version-Specific Template Layout

- **8.8+**: Unified `templates/orchestration/` subdirectory.
- **8.7 and earlier**: Separate component template directories (`templates/operate/`, `templates/zeebe/`, etc.).
- Never assume paths or helper names are identical across versions.

### Test/Integration Scenarios

Scenarios are defined per chart version in `test/ci-test-config.yaml`. Each scenario specifies identity, persistence backend, cloud platforms, upgrade flows, and optional features.

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

The `features` array maps to `values/features/<name>.yaml`. Pre-install scripts live in `test/integration/scenarios/pre-setup-scripts/pre-install-<scenario-name>.sh` and receive `$NAMESPACE`, `$RELEASE`, `$KUBE_CONTEXT`.

## Dependencies

### External

- **Bitnami Common (common-2)**: Library chart for shared helpers and functions.
- **Bitnami Elasticsearch (elasticsearch-21)**: Optional search backend for audit logs. Version 21.6.3 (app 8.18.0).
- **Keycloak (keycloak-24)**: OIDC/OAuth2 identity provider. Version 24.x.x.
- **Bitnami PostgreSQL (identity-postgresql-{15,16}, web-modeler-postgresql*)**: Database backends. Versions vary per subchart.

### Internal References

- Parent doc: `../AGENTS.md` — build, lint, test, and Go tooling commands.
- CI/CD context: `../.github/AGENTS.md` — CI pipeline, release workflows, matrix testing.
- Deployment patterns: `../SKILLS.md` — deploy-camunda CLI, kubectl usage.
- Session state: `../STATE.md` (gitignored) — session continuity, discoveries, remaining work.
- MCP server: `../helm-values-mcp/` — TypeScript MCP exposing chart values schema.
- Unused values scanner: `../scripts/helm_unused_values/` — Find declared but unreferenced values.

## Version Support and Maintenance

- **Alpha (8.10)**: Most actively developed, breaking changes possible.
- **Standard Support (8.9, 8.8, 8.7)**: Maintained, safe for production.
- **Extended Support (8.6, 8.5, 8.4, 8.3)**: Maintenance mode, only critical backports.
- **End-of-Life (8.2, 8.1, 8.0)**: Not supported, do not edit.

Before editing a chart, confirm its support status in `chart-versions.yaml`.

## Critical Rules (Summary)

1. **NEVER assume templates are identical across versions.** Always verify the target Chart.yaml and version-specific templates before editing.
2. **NEVER edit golden files by hand.** Use `make go.update-golden-only chartPath=...`.
3. **ALWAYS run `make helm.dependency-update chartPath=...` before testing a chart.**
4. **ALWAYS keep diffs small and version-scoped.** One chart version per commit.
5. **ALWAYS use Conventional Commits** (feat, fix, test, docs, etc.) in PR titles.

<!-- MANUAL: Add chart-specific discoveries, common gotchas, or patterns discovered during development. -->

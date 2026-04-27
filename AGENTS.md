# Agent Instructions

You are an expert Helm chart and Go engineer working on the Camunda 8 Self-Managed platform.
This repository contains Helm charts, chart tests, and Go-based tooling.
Use this file as the practical guide. For architecture and CI context, also read `.github/AGENTS.md`.

## Critical Rules
- NEVER assume templates are identical across chart versions — always check the target version first.
- NEVER edit golden files by hand — use `make go.update-golden-only chartPath=...`.
- NEVER implement CI logic (>20 lines) in bash — use Go scripts in `scripts/` with unit tests.
- ALWAYS run `make helm.dependency-update chartPath=...` before testing/linting a chart.
- ALWAYS keep diffs small and version-scoped.
- ALWAYS preserve existing patterns before introducing new abstractions.
- ALWAYS use Conventional Commits for PR titles.

## Quick Start

- Read `STATE.md` at repo root if it exists (session continuity file, gitignored).
- Identify the target chart version before editing.
- Prefer `make` targets so local behavior matches CI.

## Build Commands
```bash
# Install all Go CLIs into $GOPATH/bin
make install.dx-tooling

# Build all Go CLIs
make build.dx-tooling

# Build individual Go tools
make build.deployer
make build.prepare-helm-values
make build.deploy-camunda
make build.vault-secret-mapper

# Build TypeScript MCP server
cd helm-values-mcp && npm run build
```

## Lint Commands
```bash
# Lint one chart (strict)
make helm.lint chartPath=charts/camunda-platform-8.10

# Lint all matching charts
make helm.lint-all

# Enforce Go formatting (fails if gofmt would change files)
make go.fmt

# Check Apache headers on Go test files
make go.addlicense-check chartPath=charts/camunda-platform-8.10

# Add missing Apache headers
make go.addlicense-run chartPath=charts/camunda-platform-8.10

# Maintainer precommit chores
make precommit.chores

# Find values defined in values.yaml that are not referenced by any template
# Build first: cd scripts/helm_unused_values && go build -o helm-unused-values
./helm-unused-values charts/camunda-platform-8.10/templates
```

## Test Commands
```bash
# Run all Go unit tests for all matched chart versions
make go.test

# Run Go unit tests for a single chart version
make go.test chartPath=charts/camunda-platform-8.10

# Run all tests in one Go package
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/...

# Run a single Go test by name (most important single-test pattern)
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/... -run TestStatefulSetTemplate

# Update only golden snapshots
make go.update-golden-only chartPath=charts/camunda-platform-8.10

# Faster golden update without cleanup
make go.update-golden-only-lite chartPath=charts/camunda-platform-8.10

# Bash tests
bats test/scripts/
bats test/scripts/generate_chart_matrix.bats

# TypeScript tests
cd helm-values-mcp && npm test

# Run a single vitest file
cd helm-values-mcp && npx vitest run src/path/to/file.test.ts

# Run a single vitest test by name
cd helm-values-mcp && npx vitest run -t "test name"
```

Single-test guidance: Go uses `go test <package> -run <regex>` from `charts/<version>/test/unit`; Vitest uses `vitest run <file>` and/or `-t <name>`.

## Helm and Dependency Commands
```bash
# Required before many chart test/lint operations
make helm.dependency-update chartPath=charts/camunda-platform-8.10

# Render templates locally
make helm.template chartPath=charts/camunda-platform-8.10

# Dry-run local install
make helm.dry-run chartPath=charts/camunda-platform-8.10
```

## Toolchain
Pinned in `.tool-versions`, managed via `asdf`.

- Go `1.26.1`, Helm `3.20.1`, kubectl `1.27.16`
- kind `0.31.0`, kustomize `5.8.1`
- yq `4.52.5`, jq `1.8.1`, yamllint `1.38.0`, bats `1.11.0`

Install all pinned tools:
```bash
make tools.asdf-install
```

## Code Style Guidelines

### General
- Keep diffs small and version-scoped.
- Preserve existing patterns before introducing new abstractions.
- Respect per-version layout differences across chart directories.

### Imports
- Go imports: standard library, third-party, local packages.
- Let `gofmt` manage order and grouping.
- Reuse established aliases (for example `corev1`, `appsv1`) where already used.
- TypeScript uses ESM imports at top-level.

### Formatting
- Go must be gofmt-clean (`make go.fmt`).
- Helm templates/YAML use 2-space indentation.
- Use Helm whitespace trimming (`{{-`, `-}}`) consistently.

### Types and Naming
- Go tests generally use `testify/suite` and table-driven helpers.
- Prefer explicit TypeScript types when inference is unclear.
- Helm helper names follow `<component>.<camelCase>` or `camundaPlatform.<camelCase>`.
- Go test suite names follow `<Resource>Test`; entry function style is `Test<Resource>Template`.

### Error Handling
- In Go tests, prefer `require.NoError` for setup/fail-fast checks.
- Use `assert` for additional non-fatal assertions.
- Do not swallow errors; return/assert with useful context.

### Golden Files
- Template output changes often require golden snapshot updates.
- During iteration use `make go.update-golden-only-lite chartPath=...`.
- Before finalizing, run `make go.test chartPath=...`.

## Version-Aware Rules
- `8.8+` uses unified `templates/orchestration/`.
- `8.7` and older use separate component template directories.
- Never assume paths/components are identical across versions.

### Template Gating Patterns
Many template blocks are gated behind value flags that create implicit coupling:

- **`global.elasticsearch.external`** gates ES auth env var injection in most components (8.7 and 8.8). A hard constraint in `constraints.tpl` blocks setting `external=true` when the bundled subchart is active (`elasticsearch.enabled=true`). To inject auth when using the bundled subchart, use component-level `env` overrides instead.
- **`global.elasticsearch.tls.existingSecret`** triggers TLS truststore volume mounts and `JAVA_TOOL_OPTIONS` injection in most components. This is NOT behind the `external` gate — it works with the bundled subchart.
- Template blocks often differ between versions (e.g., 8.7 Operate init container renders `operate.env` with `toYaml` but NOT `tpl`, so `{{ .Release.Name }}` in `valueFrom.secretKeyRef.name` is literal). Always read the specific version's template before writing env overrides.

### Helm Subchart Value Override Patterns
Helm's values merge is a **deep merge for maps** but a **full replace for arrays**. This matters for subchart overrides:

```yaml
# Parent chart values.yaml default:
elasticsearch:
  master:
    extraEnvVars:           # <-- array
      - name: SOME_VAR
        value: "default"

# Your overlay:
elasticsearch:
  master:
    extraEnvVars: []        # Replaces the entire array — parent default is gone
```

This is the correct way to neutralize a parent chart's default array value. Setting `extraEnvVars: []` removes the parent's entries entirely. Setting `extraEnvVars: [{name: SOME_VAR, value: "override"}]` replaces the array with your single entry.

**Contrast with deploy-camunda's merge:** The `deploy-camunda` CLI uses name-keyed deep merge for `env` arrays (matching on `name` field). But Helm itself does NOT — Helm replaces arrays wholesale. Know which merge strategy applies at each layer.

### Bitnami Subchart Env Var Chains
Bitnami charts often set env vars from multiple sources in a fixed order within the statefulset template:

1. Security helper (from `security.*` values)
2. `<role>.extraEnvVars` (e.g., `master.extraEnvVars`)
3. Top-level `extraEnvVars`

When Kubernetes encounters duplicate env var names, **the last one wins**. If the parent chart's `values.yaml` defaults an `extraEnvVars` entry that conflicts with a security helper value, you must either override or clear the array. To diagnose:

```bash
# Render and count occurrences of a suspicious env var:
helm template integration charts/camunda-platform-8.X \
  -f <your-values.yaml> \
  --show-only charts/elasticsearch/templates/master/statefulset.yaml \
  | grep -c 'ELASTICSEARCH_ENABLE_REST_TLS'
# Should be exactly 1. If >1, there's a duplicate.
```

## Commit and Branch Conventions
- Branches: `issueId-description` (example `123-adding-bpel-support`).
- Commit/PR titles: Conventional Commits.
- Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `style`, `build`, `ci`, `chore`, `perf`, `deps`.
- Use present tense; keep subject under 120 characters.

## Additional Agent Context
- `CLAUDE.md` — thin redirect for Claude Code (redirects to this file, AGENTS.md)
- `.github/AGENTS.md` — CI/CD architecture, repo structure, values files
- `SKILLS.md` — deploy-camunda CLI patterns, kubectl usage
- `STATE.md` — session continuity (gitignored, read on session start)
- `helm-values-mcp/` — MCP server exposing chart values schema. Tools: `list_versions`, `list_components`, `search_configs`, `get_config_info`, `generate_values_example`.
- `scripts/helm_unused_values/` — CLI to find values declared in `values.yaml` but never referenced in templates.

## Recommended Agent Workflow
1. Select target chart version/component.
2. Run dependency update for that chart.
3. Make focused edits using existing helpers/patterns.
4. Run single package/test first, then chart-scoped test run.
5. Update golden files only for intentional rendering changes.
6. Record discoveries and remaining work in `STATE.md`.

## Adding New Persistence Layers and Scenarios

### New Persistence Layer
A persistence layer is a values file at `charts/<version>/test/integration/scenarios/chart-full-setup/values/persistence/<name>.yaml`. To add one:

1. Create the YAML file with the values needed for the data backend.
2. Add the name to `validPersistence` in `scripts/camunda-core/pkg/scenarios/scenarios.go`.
3. Update help text and shell completions in `scripts/deploy-camunda/cmd/prepare_values.go` and `scripts/deploy-camunda/cmd/root.go`.

### New CI Test Scenario
Scenarios are defined in `charts/<version>/test/ci-test-config.yaml`. Each entry specifies identity, persistence, platforms, flows, and optional features. Example:

```yaml
- name: elasticsearch-self-signed-upgrade
  enabled: false                    # set true when ready for CI
  identity: keycloak
  persistence: elasticsearch-self-signed
  platforms: [gke]
  flows: [upgrade-minor]
  features: [migrator]              # includes values/features/migrator.yaml
  shortname: esss                   # 4-char, used in namespace generation
```

The `features` array maps to `values/features/<name>.yaml`. The `migrator` feature enables identity and data migration jobs during upgrades — use it for any `upgrade-minor` scenario. Note: the automatic `needsMigrator()` function in `scenarios.go` only activates when `ChartVersion` starts with "13", but the matrix runner does not set `ChartVersion`, so always use `features: [migrator]` explicitly.

### Pre-Install Hooks (Scenario-Specific)
When a scenario needs prerequisites in the namespace before `helm install` (e.g., TLS secrets), use a pre-install script:

1. Create `charts/<version>/test/integration/scenarios/pre-setup-scripts/pre-install-<scenario-name>.sh`.
2. The script receives `NAMESPACE`, `RELEASE`, and `KUBE_CONTEXT` as environment variables.
3. Discovery is automatic: `versionmatrix.HasPreInstallScript(repoRoot, appVersion, scenario)` checks for the file.
4. The runner calls it after namespace creation but before `helm install`, in both single-step and two-step upgrade flows.

For reusable logic (e.g., cert generation), create a separate script and have the pre-install wrapper `exec` into it:
```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/create-elasticsearch-tls-secrets.sh"
```

### Pre-Upgrade Scripts
For cleanup needed between Step 1 (old version) and Step 2 (new version) of an upgrade flow, use `pre-upgrade-minor.sh` in the target version's `pre-setup-scripts/`. Common needs for 8.7→8.8:
- Delete identity deployment (port naming conflict: 8.7 uses `containerPort: 8080`, 8.8 uses `8084`, both named `http` — Kubernetes strategic merge patch keeps both, causing a duplicate name error).
- Delete stale 8.7 ingresses that route to non-existent services.
- Delete PostgreSQL statefulsets (Bitnami version changes).

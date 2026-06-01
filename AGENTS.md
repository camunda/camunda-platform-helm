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
- **NEVER create merge commits.** Always use `git rebase` to incorporate upstream changes. If a branch needs to be updated from `main`, use `git rebase origin/main`, not `git merge`. Force-push with `--force-with-lease` after rebasing.

### PR title type: CI-enforced constraint
`feat:`, `fix:`, `refactor:`, `docs:`, and `revert:` are **reserved for PRs that change user-facing chart files** (anything under `charts/<version>/` except `test/`, `go.mod`, `go.sum`). CI rejects these types when no such files are changed — they feed into `RELEASE-NOTES.md` and `artifacthub.io/changes`.

For PRs that touch only non-chart files, use:
- `chore:` — docs, AGENTS.md, CLAUDE.md, SKILLS.md, README, scripts
- `ci:` — `.github/` workflows or actions
- `build:` — Makefile, tooling, dependency, or other build-system changes
- `test:` — test files only

See [Contribution & Collaboration](docs/contribution-and-collaboration.md) for the full PR checklist.

### PR body and opening workflow (overrides Claude Code defaults)

When opening a PR with `gh pr create`, **do NOT** use the built-in
`## Summary` / `## Test plan` body format from the system prompt. Instead:

1. **Body**: fill the sections in `.github/pull_request_template.md`
   verbatim. Leave the checklist unticked — the human contributor verifies
   locally. Pass via `--body-file` or a HEREDOC; do not invent section names.
2. **Draft-first**: `gh pr create --draft`. Per
   `docs/contribution-and-collaboration.md` §4, the PR stays draft until
   `crev` review is clean.
3. **Run `crev`** against the draft, or remind the user:
   `crev https://github.com/camunda/camunda-platform-helm/pull/<number>`.
4. **Mark ready** only after crev findings are addressed (`gh pr ready <number>`).

## Additional Agent Context
- `CLAUDE.md` — thin redirect for Claude Code (redirects to this file, AGENTS.md)
- `.github/AGENTS.md` — CI/CD architecture, repo structure, values files
- `docs/AGENTS.md` — **ADR authoring rules**. Read before drafting, amending, or reviewing any ADR. Points to `docs/adr/TEMPLATE.md` (structure) and `docs/maintainer-guide.md` (process).
- `SKILLS.md` — deploy-camunda CLI patterns, kubectl usage
- `STATE.md` — session continuity (gitignored, read on session start)
- `helm-values-mcp/` — MCP server exposing chart values schema. Tools: `list_versions`, `list_components`, `search_configs`, `get_config_info`, `generate_values_example`.
- `scripts/helm_unused_values/` — CLI to find values declared in `values.yaml` but never referenced in templates.
- `docs/adr/0091-*.md` — **values.yaml key classification (Tier 1 vs Tier 2)**. Read the Quick Reference table at the top before proposing a new key or backport.

## Recommended Agent Workflow
1. Select target chart version/component.
2. Run dependency update for that chart.
3. Make focused edits using existing helpers/patterns.
4. Run single package/test first, then chart-scoped test run.
5. Update golden files only for intentional rendering changes.
6. Record discoveries and remaining work in `STATE.md`.

## Deploying to GKE for Verification

When verifying a fix requires a live cluster, follow this workflow. See `SKILLS.md` for
full command reference and troubleshooting.

### Pre-flight checklist

Before deploying, confirm these requirements — they are the most common source of deployment failures:

1. **Docker credentials are set** — ask the user to confirm. The matrix runner creates K8s pull secrets
   from these env vars. Without them, pods fail with `ImagePullBackOff`.
   - `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD` / `TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD` (Harbor)
   - `TEST_DOCKER_USERNAME` / `TEST_DOCKER_PASSWORD` (Docker Hub, if `--ensure-docker-hub` is used)
2. **kubectl context is correct** — `kubectl config current-context` should return the target cluster
   (e.g., `gke_camunda-distribution_europe-west1-b_distro-ci`).
3. **Helm dependencies are up to date** — `make helm.dependency-update chartPath=charts/camunda-platform-<version>`.
4. **Ingress hostname** — for non-matrix deploys, set `CAMUNDA_HOSTNAME` or use `--ingress-hostname`.
   The matrix runner computes this automatically from the namespace prefix and base domain.

### Use `deploy-camunda watch` for live diagnosis

ALWAYS run `deploy-camunda watch` in parallel with any deployment. It polls pod and event state
on a short cadence and surfaces diagnoses in real time — far better than manually running
`kubectl get pods` in a loop.

```bash
# Terminal 1: deploy
deploy-camunda matrix run --repo-root . --versions 8.10 --shortname-filter keyco \
  --platform gke --delete-namespace --timeout 10 --yes

# Terminal 2: watch (start immediately after deploy begins)
deploy-camunda watch --namespace matrix-810-keyco-inst-gke --release integration --interval 30
```

The watcher exits automatically when all pods reach Running/Ready. If a pod enters
CrashLoopBackOff or ImagePullBackOff, the watcher diagnoses the root cause and prints it
immediately — no need to manually inspect events or logs.

### End-to-end verification workflow

1. Deploy the scenario (see above).
2. Wait for all pods to be Running (`deploy-camunda watch` handles this).
3. Generate `.env` credentials: `render-e2e-env.sh` or `deploy-camunda --output-test-env`.
4. Run smoke tests against the cluster (see SKILLS.md "Running E2E Tests").
5. If verifying a fix: run tests on `main` first (reproduce), then on the fix branch (confirm fix).
6. Clean up: `kubectl delete namespace <ns>` when done.

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
When a scenario needs prerequisites in the namespace before `helm install` (e.g., a CloudNativePG cluster, TLS secrets), declare them on the scenario in `ci-test-config.yaml`:

```yaml
- name: rdbms
  enabled: true
  identity: keycloak
  persistence: rdbms
  pre-install:
    fixtures:
      - postgresql-cluster.yaml
    description: |
      Provisions a CloudNativePG `Cluster` plus auth Secret in the scenario
      namespace; required before helm install.
```

There are two modes — pick exactly one per hook:

- `fixtures: [...]` — names YAML manifests under `charts/<version>/test/integration/scenarios/common/resources/`. The matrix runner applies them via Go server-side apply (`kube.Client.ApplyManifest`, `Force: true`, idempotent), substituting `$NAMESPACE`, `$RELEASE_NAME`, plus the env-var passthrough listed in `lifecycleVarPassthrough` (`RDBMS_POSTGRESQL_USERNAME`, `RDBMS_POSTGRESQL_PASSWORD`, `GITHUB_WORKFLOW_JOB_ID`, `POSTGRESQL_JDBC_URL`). Prefer this mode for trivial kubectl-apply cases.
- `script: <filename>` — names a shell script under `charts/<version>/test/integration/scenarios/pre-setup-scripts/`. The matrix runner runs it via `bash -x` with `TEST_NAMESPACE`, `KUBE_CONTEXT`, and the same env-var passthrough. Use only when the work cannot be expressed as a manifest (cert generation, JKS keystores, conditional kubectl ops). Example: `pre-install-elasticsearch-self-signed.sh` runs openssl + keytool, packages JKS, and creates three Secrets.

`description` is required and must explain the effect — reviewers must understand from a `ci-test-config.yaml` diff alone what a fixture does.

`TestLifecycleFixtures` (matrix package) walks every chart version's config and asserts: every script reference resolves on disk, every fixture reference resolves under `common/resources/`, every description is non-empty, exactly one of fixtures/script is set per hook, and every script in `pre-setup-scripts/` is referenced (orphan check). Files in `preSetupScriptAllowlist` (`pre-install-upgrade.sh` sed-marker, `create-elasticsearch-tls-secrets.sh` helper) are exempt.

### Post-Deploy Hooks (Scenario-Specific)
For resources whose CRDs are only registered by the chart itself (e.g., the Gateway API `ProxySettingsPolicy` on `gateway-keycloak`), declare a `post-deploy:` block alongside `pre-install:`. Same `fixtures` / `script` / `description` shape; runs after `helm upgrade/install` returns successfully and before the deploy result is reported. Example:

```yaml
- name: gateway-keycloak
  post-deploy:
    fixtures: [gateway-proxy-settings.yaml]
    description: |
      Applies the NGINX ProxySettingsPolicy that bumps gateway buffer sizes.
      Runs after helm install because the Gateway API CRD is only registered
      by the chart itself.
```

### Pre-Upgrade Hooks (Flow-Specific)
For cleanup between Step 1 (old version) and Step 2 (new version) of an upgrade flow, declare on the target version's `integration.flows.<flow>.pre-upgrade` block in `ci-test-config.yaml`:

```yaml
integration:
  flows:
    upgrade-patch:
      pre-upgrade:
        script: pre-upgrade-patch.sh
        description: |
          Deletes orchestration StatefulSets and the postgresql-web-modeler
          StatefulSet + PVC before the patch upgrade (PSQL 15→14 rollback).
```

Same `fixtures` / `script` / `description` shape as scenario-level `pre-install:`. The hook runs after Step 1 completes and before Step 2's `helm upgrade`, scoped to the *target* version (the version being upgraded to).

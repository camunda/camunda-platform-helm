---
applyTo: "**"
---

# Code Review — Scoped Instructions

## Overview

This file guides AI agents performing code review on pull requests in the camunda-platform-helm
repository. Reviews should focus on correctness, backward compatibility, security, and Helm
best practices. Surface only issues that genuinely matter — bugs, security risks, breaking
changes, or logic errors. Do not comment on formatting (enforced by `gofmt` / `helm lint`),
trivial style preferences, or cosmetic whitespace. Always verify that template changes are
accompanied by updated golden files and that new values fields are documented. Check that Go
test coverage exists for any new template behaviour. Flag concerns about cross-version
compatibility (8.7 vs 8.8+ layouts differ).

### Chart Design Principles (from `docs/index.md`)

These are first-class review criteria. Flag PRs that violate them:

- **Minimal & common** — only expose configuration that is common, minimal, and useful. Reject
  additions that expose arbitrary or exhaustive application configuration.
- **User-driven** — every new field or feature must stem from a conscious, user-driven decision
  validated by product management. Reject opinionated solutions from individual engineers.
- **Generic extensibility** — provide generic, composable mechanisms (`extraConfiguration`,
  `extraEnv`, `extraVolumes`) rather than embedding specific integrations (monitoring stacks,
  custom security policies, identity management solutions).
- **No external bundling** — do not add dependencies on external components not part of Camunda
  core (e.g., OpenSearch, Bitnami sub-charts beyond what already exists).
- **No workarounds** — the chart must not patch or work around application-level issues or
  technical debt. If an application bug requires a workaround, fix the application instead.
---

## Critical Rules

### NEVER
- **NEVER** approve a PR that removes or renames an existing values field without a deprecation
  notice or migration path — this is a breaking API change for users.
- **NEVER** approve a PR that adds secrets or credentials as default values in `values.yaml`.
- **NEVER** approve a PR that hardcodes image names or tags instead of using
  `camundaPlatform.imageByParams`.
- **NEVER** approve a PR that uses mutable action tags (e.g., `@v4`) in GitHub Actions workflows.
- **NEVER** raise style or formatting comments — these are enforced by automated tools.
- **NEVER** approve changes to `values-digest.yaml` that appear to be manually edited.

### ALWAYS
- **ALWAYS** check that template changes have corresponding golden file updates in
  `test/unit/<component>/golden/`.
- **ALWAYS** verify that new `values.yaml` fields have `## @param` documentation comments.
- **ALWAYS** check for the `{{- if .Values.<component>.enabled -}}` guard in new resource files.
- **ALWAYS** verify Go test files have an Apache 2.0 license header.
- **ALWAYS** flag changes that only apply to one chart version when the same change may be
  needed across multiple versions (8.8, 8.9, 8.10, etc.).
- **ALWAYS** check that `extraConfiguration` support follows the `kindIs "slice"` dual-form pattern.

---

## Core Review Checklist

### Chart Design Principle Violations

- [ ] Does the PR add a values field that exposes arbitrary or exhaustive application configuration
  with no clear user-driven rationale?
- [ ] Does the PR implement an opinionated solution (bundled monitoring, hard-coded security policy,
  specific identity integration) instead of a generic, composable mechanism?
- [ ] Does the PR bundle a dependency on an external component (e.g., a new Bitnami sub-chart)
  not already present in the chart?
- [ ] Does the PR use a Helm values abstraction that breaks the 1:1 mapping with application config?
- [ ] Does the PR serve as a workaround for an application bug or technical debt rather than a
  genuine chart improvement?

```yaml
# GOOD — generic extensibility via extraConfiguration
connectors:
  extraConfiguration:
    monitoring.yaml: |
      management.metrics.export.prometheus.enabled=true

# BAD — opinionated, hard-coded monitoring integration
connectors:
  prometheus:
    enabled: true
    port: 9090
    path: /metrics
```

### Template Rendering Validation

- [ ] Does every new resource file have the component `enabled` guard?
- [ ] Are `{{-` / `-}}` whitespace trimmers used correctly on block directives?
- [ ] Is `nindent` (not `indent`) used when piping `include` results into YAML?
- [ ] Are user-supplied label/annotation maps rendered with `tpl` to support template expressions?
- [ ] Is `checksum/config` annotation present when the resource depends on a ConfigMap?
- [ ] Is image resolution done via `camundaPlatform.imageByParams`?
- [ ] Is `extraConfiguration` support using `camundaPlatform.renderExtraConfiguration`?

```yaml
# GOOD — guard present, labels use nindent, tpl wraps podLabels
{{- if .Values.connectors.enabled -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    {{- include "connectors.labels" . | nindent 4 }}
spec:
  template:
    metadata:
      labels:
        {{- include "connectors.labels" . | nindent 8 }}
        {{- if .Values.connectors.podLabels }}
        {{- tpl (toYaml .Values.connectors.podLabels) $ | nindent 8 }}
        {{- end }}
{{- end }}

# BAD — missing guard, missing nindent, missing tpl
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    {{ include "connectors.labels" . }}
```

### Unit Test Coverage

- [ ] Is there a unit test for each new rendering path (e.g., enabled/disabled, with/without
  optional field)?
- [ ] Does each new template file have a corresponding golden file test in
  `test/unit/<component>/goldenfiles_test.go`?
- [ ] Are golden files in `test/unit/<component>/golden/` updated to match the new output?
- [ ] Do new test files have Apache 2.0 license headers?
- [ ] Is `require.NoError` used for setup failures (not `assert.NoError`)?
- [ ] Is `t.Parallel()` present in the suite entry function?

```go
// GOOD — require for setup, parallel, proper unmarshal
func TestDeploymentTemplate(t *testing.T) {
    t.Parallel()
    chartPath, err := filepath.Abs("../../../")
    require.NoError(t, err)  // fail fast on setup error
    suite.Run(t, &DeploymentTest{ chartPath: chartPath, ... })
}

// BAD — assert allows test to continue with bad state
func TestDeploymentTemplate(t *testing.T) {
    chartPath, err := filepath.Abs("../../../")
    assert.NoError(t, err)  // test continues even on error
    suite.Run(t, &DeploymentTest{ chartPath: chartPath, ... })
}
```

### Security Review

- [ ] Are any secrets or credentials hardcoded in `values.yaml` defaults?
- [ ] Does a new workflow step use Vault (`hashicorp/vault-action`) rather than plain `secrets:`
  references for sensitive values?
- [ ] Are all GitHub Actions `uses:` references pinned to full commit SHAs?
- [ ] Does a new template expose any unnecessary environment variables (API keys, tokens) that
  should be in Secrets, not ConfigMaps?
- [ ] Are `containerSecurityContext` and `podSecurityContext` fields preserved, not removed?

### Workflow Automation Implementation

- [ ] Does the PR introduce new non-trivial workflow automation logic in Bash (`run:` blocks or
  `scripts/*.sh`) instead of Go?
- [ ] If the workflow logic calls external APIs, parses JSON, or contains branching/orchestration,
  is that logic implemented in Go under `scripts/<feature>/` (or existing Go tooling)?
- [ ] If Bash remains, is it only thin glue (roughly <=20 lines), with business logic moved to Go?

```yaml
# GOOD — action pinned to SHA with version comment
- uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6

# BAD — mutable tag, supply-chain risk
- uses: actions/checkout@v4
```

### Backward Compatibility

- [ ] Does the PR rename or remove any field from `values.yaml`? If so, is there a deprecation
  comment and a transition path?
- [ ] Does the PR change a helper name (`define "foo.bar"`)? All existing callers across all
  chart versions must be updated.
- [ ] Does the PR change the default value of an existing field in a way that alters runtime
  behaviour for users who do not override it?
- [ ] If the change is version-specific (e.g., 8.10 only), is the scope clearly limited and
  not accidentally applied to shared templates?
- [ ] Does the PR update the `CHANGELOG.md` for user-visible changes?

### Helm Best Practices

- [ ] Are named templates (`define`) defined in `_helpers.tpl` files, not in resource templates?
- [ ] Are component names derived from helpers (not hardcoded)?
- [ ] Is `values.schema.json` updated when new required or constrained fields are added?
- [ ] Is `helm lint` expected to pass? (run `make helm.lint chartPath=...` to verify)
- [ ] Are multi-line configuration strings indented with `indent N | trim`?

---

## Common Mistakes to Flag in PRs

1. **Chart principle violations** — PRs that add opinionated integrations, arbitrary config
   fields, or external component dependencies violate `docs/index.md`. These should be flagged
   with a clear explanation of which principle is violated and why.

2. **Golden files not updated** — when a template changes but golden files are not regenerated,
   CI will fail. Check that `test/unit/<component>/golden/*.golden.yaml` files are committed
   alongside template changes.

2. **Undocumented values field** — a new `values.yaml` field without `## @param` will be
   invisible in generated docs. Flag as a required fix.

3. **Missing `kindIs "slice"` check on `extraConfiguration`** — the field supports both map
   and list forms. Handlers that only loop with `range $k, $v := ...` will panic on list input.

4. **`indent` without `trim` on multiline strings** — produces a leading newline before the
   content block, causing YAML parse errors at helm install time.

5. **Cross-version scope creep** — a fix applied to `8.10` templates may also be needed in
   `8.8` and `8.9`. Flag if the PR description doesn't address this.

6. **Inline bash >20 lines** — complex shell logic without tests. Flag and suggest moving to
   a Go script in `scripts/` with unit tests per `.github/AGENTS.md` policy.

7. **`fail-fast: true` (or default) on test matrix** — in GitHub Actions workflows, the default
   for `fail-fast` is `true`. Test matrices should set `fail-fast: false`.

8. **Using `assert` instead of `require` for fatal Go test setup** — allows tests to continue
   with bad state and produces confusing failure messages.

9. **Non-trivial workflow automation in Bash** — API orchestration, JSON parsing, and branching
  added in `run:` blocks or `scripts/*.sh` should be flagged and migrated to Go for testability
  and maintainability.

---

## Resources

- Helm chart best practices: <https://helm.sh/docs/chart_best_practices/>
- Chart design principles (what the chart IS and IS NOT): `docs/index.md`
- Integration test scenario resolution: `docs/integration-test-scenario-resolution.md`
- Conventional Commits: <https://www.conventionalcommits.org/>
- Kubernetes API conventions: <https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md>
- Repository AI guidance: `AGENTS.md` and `.github/AGENTS.md`
- Run full test suite: `make go.test chartPath=charts/camunda-platform-8.10`
- Run helm lint: `make helm.lint chartPath=charts/camunda-platform-8.10`

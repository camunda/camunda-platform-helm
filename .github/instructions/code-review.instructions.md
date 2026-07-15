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

### Chart Design Principles

The chart design principles in `docs/index.md` (canonical) are first-class review criteria —
flag PRs that violate them: minimal & common, user-driven, generic extensibility
(`extraConfiguration`/`extraEnv`/`extraVolumes` over embedded integrations), no external-component
bundling, no workarounds for application-level issues, 1:1 mapping to application config.

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

Most flag-worthy mistakes restate the rules and checklists above (principle violations, golden
files not regenerated — CI fails, undocumented `@param` fields — invisible in generated docs,
`kindIs "slice"` panics on list input, `indent` without `trim` — YAML parse errors at install
time, cross-version scope creep, bash >20 lines). Additional pitfalls not covered above:

1. **`fail-fast: true` (or default) on test matrix** — in GitHub Actions workflows, the default
   for `fail-fast` is `true`. Test matrices should set `fail-fast: false`.

---

## Resources

- Helm chart best practices: <https://helm.sh/docs/chart_best_practices/>
- Chart design principles (what the chart IS and IS NOT): `docs/index.md`
- Breaking changes policy (canonical): `docs/policies/breaking-changes.md`
- Backporting policy: `docs/policies/backporting.md`
- Integration test scenario resolution: `docs/skills/integration-test-scenario-resolution.md`
- Conventional Commits: <https://www.conventionalcommits.org/>
- Kubernetes API conventions: <https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md>
- Repository AI guidance: `AGENTS.md` and `.github/AGENTS.md`
- Run full test suite: `make go.test chartPath=charts/camunda-platform-8.10`
- Run helm lint: `make helm.lint chartPath=charts/camunda-platform-8.10`

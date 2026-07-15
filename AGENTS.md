# Agent Instructions

You are an expert Helm chart and Go engineer working on the Camunda 8 Self-Managed platform.
This repository contains Helm charts, chart tests, and Go-based tooling.
Use this file as the practical guide. For architecture and CI context, also read `.github/AGENTS.md`.

## Critical Rules
- NEVER assume templates are identical across chart versions — always check the target version first.
- NEVER hand-edit generated artifacts — regenerate them (see Generated Artifacts below).
- NEVER implement CI logic (>20 lines) in bash — use Go scripts in `scripts/` with unit tests.
- NEVER write reasoning/"why"/narration comments — comments explain only non-obvious HOW. Architectural rationale belongs in an ADR under `docs/adr/`, human-authored; tactical rationale (bug-fix defaults, timeouts, label choices) goes in the PR body or commit message. Agents NEVER create or edit ADRs proactively. Keep only required structured comments: Apache license headers, `## @param`/`## @extra`, the `{{- /* NOTE */ -}}` helper convention, and lint/build pragmas (`//nolint`, `//go:build`, `# yamllint disable`, `# yamllint disable-line`, `# shellcheck disable`).
- ALWAYS run `make helm.dependency-update chartPath=...` before testing/linting a chart.
- ALWAYS keep diffs small and version-scoped.
- ALWAYS preserve existing patterns before introducing new abstractions.
- ALWAYS use Conventional Commits for PR titles.

## Quick Start

- Read `STATE.md` at repo root if it exists (session continuity file, gitignored). Record discoveries and remaining work there as you go, under `## Goal / Instructions / Discoveries / Accomplished / Not Yet Done / Relevant Files` — keep it useful to a fresh session.
- Identify the target chart version before editing.
- Prefer `make` targets so local behavior matches CI.
- Run a single package/test first, then the chart-scoped run; update golden files only for intentional rendering changes.

## Build Commands
```bash
make install.dx-tooling               # Install all Go CLIs into $GOPATH/bin
make build.dx-tooling                 # Build all Go CLIs
make build.prepare-helm-values        # Individual Go tools
make build.deploy-camunda
make build.vault-secret-mapper
cd helm-values-mcp && npm run build   # TypeScript MCP server
```

## Lint Commands
```bash
make helm.lint chartPath=charts/camunda-platform-8.10       # Lint one chart (strict)
make helm.lint-all                                           # Lint all matching charts
make go.fmt                                                  # Enforce Go formatting
make go.addlicense-check chartPath=charts/camunda-platform-8.10  # Check Apache headers
make go.addlicense-run chartPath=charts/camunda-platform-8.10    # Add missing headers
make precommit.chores                                        # Maintainer precommit chores

# Find values defined in values.yaml but not referenced by any template
# Build first: cd scripts/helm_unused_values && go build -o helm-unused-values
./helm-unused-values charts/camunda-platform-8.10/templates
```

## Test Commands
```bash
make go.test                                                 # All chart versions
make go.test chartPath=charts/camunda-platform-8.10          # One chart version

# One Go package / single test by name (run from charts/<version>/test/unit)
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/...
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/... -run TestStatefulSetTemplate

bats test/scripts/                                           # Bash tests

cd helm-values-mcp && npm test                               # TypeScript tests
cd helm-values-mcp && npx vitest run src/path/to/file.test.ts   # Single vitest file
cd helm-values-mcp && npx vitest run -t "test name"             # Single vitest test
```

## Helm and Dependency Commands
```bash
make helm.dependency-update chartPath=charts/camunda-platform-8.10  # Required before test/lint
make helm.template chartPath=charts/camunda-platform-8.10           # Render locally
make helm.dry-run chartPath=charts/camunda-platform-8.10            # Dry-run install
```

## Toolchain

Tool versions are pinned in `.tool-versions` (the ground truth — do not trust prose copies), managed via `asdf`. Install all: `make tools.asdf-install`.

## Generated Artifacts

Never hand-edit any of these — regenerate:

| Artifact | Regenerate | Verify |
|---|---|---|
| Chart template goldens (`charts/<v>/test/unit/**/testdata/`, `**/golden/`) | `make go.update-golden-only chartPath=...` (or `-lite` during iteration) | `make go.test chartPath=...` |
| Registry snapshot (`charts/<v>/test/ci/registry-snapshot.yaml`) | `make go.update-registry-golden` | `make go.test` |
| `values.schema.json` | `make helm.schema-update` (part of `precommit.chores`) | `make helm.schema-validate-values` |
| `values-digest.yaml` | release/CI automation — no local command | — |
| `Chart.yaml` `artifacthub.io/changes` | release automation, generated from Conventional-Commit PR titles (`scripts/camunda-core/pkg/releasenotes`) — manual edits are overwritten | — |

Notes:
- The registry snapshot is the compiled `CITestConfig` view of the composable scenario registry (`test/ci/registry/`) — use it as a diff target when editing scenarios/hooks. The matrix-package suite in `make go.test` runs across all chart versions regardless of `chartPath=` (fast — YAML parse only).
- Because `artifacthub.io/changes` derives from PR titles only, leave `Chart.yaml` untouched in every PR.

## Code Style

- Helm templates/YAML: 2-space indentation; use `{{-` / `-}}` whitespace trimming consistently.
- Helm helper names: `<component>.<camelCase>` or `camundaPlatform.<camelCase>`.
- Go tests: `testify/suite` with table-driven helpers; suite `<Resource>Test`, entry `Test<Resource>Template`; `require.NoError` for setup/fail-fast, `assert` for non-fatal checks; don't swallow errors.
- Go: gofmt-clean (`make go.fmt`); reuse established import aliases (`corev1`, `appsv1`).
- TypeScript: ESM imports; explicit types when inference is unclear.

## Version-Aware Rules
- `8.8+` uses unified `templates/orchestration/`.
- `8.7` and older use separate component template directories.
- Never assume paths/components are identical across versions.

### Template Gating Patterns
Many template blocks are gated behind value flags that create implicit coupling:

- **`global.elasticsearch.external`** gates ES auth env var injection in most components (8.7 and 8.8). A hard constraint in `constraints.tpl` blocks setting `external=true` when the bundled subchart is active (`elasticsearch.enabled=true`). To inject auth when using the bundled subchart, use component-level `env` overrides instead.
- **`global.elasticsearch.tls.existingSecret`** triggers TLS truststore volume mounts and `JAVA_TOOL_OPTIONS` injection in most components. This is NOT behind the `external` gate — it works with the bundled subchart.
- Template blocks often differ between versions (e.g., 8.7 Operate init container renders `operate.env` with `toYaml` but NOT `tpl`, so `{{ .Release.Name }}` in `valueFrom.secretKeyRef.name` is literal). Always read the specific version's template before writing env overrides.

### Subchart Values Gotchas
- Helm merge: deep for maps, **full replace for arrays** — `extraEnvVars: []` in an overlay removes a parent chart's default array entirely.
- `deploy-camunda` merges `env` arrays name-keyed (unlike Helm) — know which merge applies at each layer.
- Bitnami charts emit env vars from several sources in fixed order; on duplicate names Kubernetes takes the last.

Full examples and the diagnose recipe: `.claude/skills/helm-values-debugging/SKILL.md`.

## Commit and Branch Conventions
- Branches: `issueId-description` (example `123-adding-bpel-support`).
- Commit/PR titles: Conventional Commits, present tense, subject under 120 characters. CI enforces the format and the allowed type list via `.github/workflows/repo-pr-conventions.yaml`.
- **NEVER create merge commits.** Always use `git rebase` to incorporate upstream changes (`git rebase origin/main`, not `git merge`); force-push with `--force-with-lease` after rebasing.

### PR title type: CI-enforced constraint
`feat:`, `fix:`, `refactor:`, `docs:`, and `revert:` are **reserved for PRs that change user-facing chart files** (anything under `charts/<version>/` except `test/`, `go.mod`, `go.sum`). CI rejects these types when no such files are changed — they feed into `RELEASE-NOTES.md` and `artifacthub.io/changes`.

For PRs that touch only non-chart files, use:
- `chore:` — docs, AGENTS.md, skills, README, scripts
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

## Skills (operational runbooks)

Procedures live as on-demand skills under `.claude/skills/<name>/SKILL.md` (Agent Skills format — plain markdown, readable by any agent). `SKILLS.md` at repo root is the index.

- `deploy-camunda` — deploy scenarios, matrix operations, values-layer composition, `watch`
- `gke-verification` — verify a fix on a live GKE cluster end-to-end (pre-flight → deploy → test → clean up)
- `rfr-validation` — tier-1/tier-2 scenario selection and local validation before marking a PR ready
- `cluster-debugging` — kubectl triage, Spring Boot `/actuator/configprops`, headless `jdb` debugging
- `e2e-testing` — run Playwright/smoke tests via `c8e2e`, generate `.env` credentials, reproduce CI failures
- `ci-scenario-authoring` — add scenarios, persistence layers, and pre-install/post-deploy/pre-upgrade hooks
- `helm-values-debugging` — Helm vs deploy-camunda merge semantics, Bitnami env-var chains

## Additional Agent Context
- `CLAUDE.md` — thin redirect for Claude Code (imports this file)
- `.github/instructions/*.instructions.md` — **authoritative path-scoped chart-coding conventions.** These files carry Copilot/VS Code `applyTo:` globs that are NOT auto-applied by Claude Code, so read the matching guide explicitly BEFORE editing: `values-yaml.instructions.md` (values.yaml authoring — `@param` conjunctions, secret-block shape), `helm-templates.instructions.md` (templates, NOTES.txt, constraints/warnings), `code-review.instructions.md`, `go-tests.instructions.md`, `scripting.instructions.md`, `github-actions.instructions.md`.
- `.github/AGENTS.md` — CI/CD architecture, repo structure, values files
- `docs/AGENTS.md` — **ADR authoring rules**. Read before drafting, amending, or reviewing any ADR.
- `STATE.md` — session continuity (gitignored, read on session start)
- `helm-values-mcp/` — MCP server exposing chart values schema; tool list and setup in `helm-values-mcp/README.md`.
- `scripts/helm_unused_values/` — CLI to find values declared in `values.yaml` but never referenced in templates.
- `docs/adr/0091-*.md` — **values.yaml key classification (Tier 1 vs Tier 2)**. Read the Quick Reference table at the top before proposing a new key or backport; operational restatement in `docs/policies/values-yaml-policy.md`.

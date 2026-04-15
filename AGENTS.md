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

## Recommended Agent Workflow
1. Select target chart version/component.
2. Run dependency update for that chart.
3. Make focused edits using existing helpers/patterns.
4. Run single package/test first, then chart-scoped test run.
5. Update golden files only for intentional rendering changes.
6. Record discoveries and remaining work in `STATE.md`.

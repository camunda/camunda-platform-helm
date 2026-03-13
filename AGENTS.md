# Agent Instructions

This is the **Camunda 8 Self-Managed Helm Charts** repository. See `.github/AGENTS.md` for full architecture, CI matrix, and operational context. This file focuses on build/lint/test commands and code style.

## Build Commands

```bash
# Install all Go CLI tools to $GOPATH/bin
make install.dx-tooling

# Build individual tools
make build.deployer
make build.prepare-helm-values
make build.deploy-camunda
make build.vault-secret-mapper

# Update Helm chart dependencies (required before running tests)
make helm.dependency-update chartPath=charts/camunda-platform-8.10

# Update all chart dependencies
make helm.dependency-update
```

## Lint Commands

```bash
# Lint all Helm charts (helm lint --strict)
make helm.lint

# Lint a specific chart
make helm.lint chartPath=charts/camunda-platform-8.10

# Check Go formatting (fails if any .go files are dirty)
make go.fmt

# Check Apache license headers on .go files
make go.addlicense-check

# Add missing license headers
make go.addlicense-run

# YAML lint (applied to test/unit directories)
yamllint -c .github/config/yamllint.yaml charts/camunda-platform-8.10/test/unit/

# Run all pre-commit chores (lint + readme + schema + golden files)
make precommit.chores
```

## Test Commands

```bash
# Run all unit tests across all chart versions
make go.test

# Run unit tests for a specific chart version
make go.test chartPath=charts/camunda-platform-8.10

# Run a single test package (e.g., orchestration)
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/...

# Run a single test by name
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/... -run TestStatefulSetTemplate

# Run multiple packages (as CI does it)
cd charts/camunda-platform-8.10/test/unit && go test ./orchestration/ ./connectors/ ./identity/

# Run golden file tests and update snapshots
make go.update-golden-only chartPath=charts/camunda-platform-8.10

# Update golden files without deleting first (faster, additive only)
make go.update-golden-only-lite chartPath=charts/camunda-platform-8.10

# Run bats tests for shell scripts
bats test/scripts/

# Run a single bats test file
bats test/scripts/generate_chart_matrix.bats

# TypeScript tests (helm-values-mcp)
cd helm-values-mcp && npm test

# TypeScript tests in watch mode
cd helm-values-mcp && npm run test:watch
```

## Tool Versions

Managed by `asdf`. Install all with `asdf install`. Pinned in `.tool-versions`.

| Tool  | Version | Tool    | Version |
|-------|---------|---------|---------|
| Go    | 1.26    | helm    | 3.20    |
| bats  | 1.11.0  | kubectl | 1.27.16 |
| yq    | 4.52.4  | jq      | 1.8.1   |

## Code Style: Helm Templates

- **Helper naming:** Prefix all `define` blocks with the component name: `orchestration.fullname`, `camundaPlatform.imageByParams`.
- **Indentation:** 2 spaces. Sequences indented under their parent key.
- **Conditionals:** Use `{{- if .Values.component.enabled -}}` guards at the top of each template file.
- **Whitespace control:** Use `{{-` / `-}}` to strip surrounding whitespace except where output formatting matters.
- **Helpers file:** Component-level helpers live in `templates/<component>/_helpers.tpl`. Shared helpers live in `templates/common/_helpers.tpl`.
- **Backward compat:** Annotate cross-version compatibility notes inline: `{{- /* NOTE: backward compat between 8.7 and 8.8 */ -}}`.
- **YAML lint rules:** 2-space indent, no line-length limit, `document-start: disable`, `truthy: warning` level. See `.github/config/yamllint.yaml`.
- **No raw image tags:** Always use the `camundaPlatform.imageByParams` helper rather than hardcoding image references.

## Code Style: Go

- **License header:** Every `.go` file must have the Apache 2.0 license header. Enforced by `make go.addlicense-check`. Run `make go.addlicense-run` to add missing headers.
- **Formatting:** `gofmt` enforced. Run `make go.fmt` before committing. No `.golangci.yml` — `gofmt` is the primary formatter.
- **Imports:** Standard library first, then third-party, then local packages. Use aliased imports for Kubernetes types: `appsv1 "k8s.io/api/apps/v1"`, `corev1 "k8s.io/api/core/v1"`.
- **Test structure:** Use `testify/suite` with a struct embedding `suite.Suite`. Entry point is a plain `func TestXxx(t *testing.T)` that calls `suite.Run`.
- **Table-driven tests:** Use `testhelpers.TestCase` slices with `Name`, `Values` (map[string]string for `--set`), `ValuesFiles`, and a `Verifier` func. See `test/unit/testhelpers/testhelpers.go`.
- **Golden files:** Snapshot outputs live in `test/unit/<component>/golden/<name>.golden.yaml`. Regenerate with `go test ./... -args -update-golden` or via `make go.update-golden-only`.
- **Error handling:** Use `require.NoError(t, err)` in tests (fails fast). Production code uses structured logging via `github.com/rs/zerolog`.
- **CLI tooling:** Use `github.com/spf13/cobra` for CLI commands. Local module dependencies use `replace` directives in `go.mod`.
- **Bash vs. Go:** Any CI logic over ~20 lines must be a Go script in `scripts/`, not bash. All Go scripts must have unit tests.
- **Module structure:** Each chart version has its own `go.mod` under `charts/<version>/`. Each script tool has its own `go.mod` under `scripts/<tool>/`.

## Code Style: TypeScript (`helm-values-mcp/`)

- Language: TypeScript 5.x, compiled with `tsc`. Runtime: Node.js with `tsx` for development.
- Test runner: `vitest`. Config in `package.json`. Run with `npm test`.
- ESLint configured in `test/e2e/` directories with `typescript-eslint`.

## Naming Conventions

| Context            | Convention                                                            |
|--------------------|-----------------------------------------------------------------------|
| Helm helper names  | `<component>.camelCase` or `camundaPlatform.camelCase`                |
| Helm values keys   | `camelCase` (e.g., `orchestration.podLabels`)                         |
| Go test suites     | `<Resource>Test` struct, `Test<Resource>Template` entry function      |
| Go test cases      | `"TestVerbNoun"` string in `Name` field (e.g., `"TestContainerSetPodLabels"`) |
| Go packages        | Match the component directory name (e.g., package `orchestration`)   |
| Git branches       | `<issueId>-short-description` (e.g., `123-adding-bpel-support`)       |
| Commit messages    | Conventional Commits: `feat(scope): description` (present tense, ≤120 chars) |

Valid commit types: `feat`, `fix`, `refactor`, `revert`, `test`, `docs`, `style`, `build`, `ci`, `cd`, `chore`, `chore(deps)`, `chore(release)`, `deps`, `perf`.

## Session State

Use `STATE.md` (repo root, gitignored) to persist context across sessions. Read it on start; update it after meaningful progress. See `.github/AGENTS.md` for the full format specification.

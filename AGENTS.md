# AGENTS.md

Agent guide for working in `camunda-platform-helm`.

## Priority

1. Follow this file.
2. Follow `README.md` and `CONTRIBUTING.md`.
3. Treat `.github/workflows/*.yaml` as the source of truth for CI behavior.

## Repository Layout

- Root automation: `Makefile`
- Charts: `charts/camunda-platform-*`
- Chart unit tests (Go): `charts/camunda-platform-*/test/unit`
- Integration scenario taskfiles: `test/integration/scenarios`
- Go helper tools/scripts: `scripts/*`
- CI config: `.github/config`

## Toolchain

Install versions pinned in `.tool-versions`:

```bash
make tools.asdf-install
```

Important tools used by local and CI-like flows:

- `golang 1.26.1`
- `helm 3.20.0`
- `helm-ct 3.11.0`
- `yamllint 1.38.0`
- `kind 0.31.0`
- `kubectl 1.27.16`
- `task 3.30.1`
- `jq 1.8.1`
- `yq 4.52.4`
- `bats 1.11.0`

## Build Commands

Run from repository root:

```bash
make build.deployer
make build.prepare-helm-values
make build.deploy-camunda
make build.vault-secret-mapper
make build.dx-tooling
make install.dx-tooling
```

## Lint and Validation

Prepare chart repos/dependencies:

```bash
make helm.repos-add
make helm.dependency-update
```

Lint a single chart:

```bash
chartPath=charts/camunda-platform-8.9 make helm.lint
```

Lint all matching charts:

```bash
make helm.lint-all
```

Go format and license checks:

```bash
make go.fmt
make go.addlicense-install
make go.addlicense-check
```

CI-equivalent lint commands:

```bash
ct lint --charts charts/camunda-platform-8.9 --lint-conf .github/config/chart-testing.yaml --config .github/config/chart-testing.yaml
$(asdf which yamllint) -c .github/config/yamllint.yaml ./charts/camunda-platform-8.9/test/unit
```

## Test Commands

Run all Go tests selected by `chartPath`:

```bash
make go.test
```

Run one chart package tests:

```bash
cd charts/camunda-platform-8.9/test/unit
go test ./common
```

Run a single Go test function:

```bash
cd charts/camunda-platform-8.9/test/unit
go test ./common -run TestConfigMapTemplate
```

Run a single Go subtest:

```bash
cd charts/camunda-platform-8.9/test/unit
go test ./common -run 'TestConfigMapTemplate/TestConfigMapIdentityIssuerURL'
```

Run tests in script modules:

```bash
cd scripts/helm_unused_values
go test ./...
go test ./pkg/search -run TestSearchKeyByPatternInTemplates
```

Run shell tests:

```bash
bats test/scripts/*.bats
```

Run Playwright integration tests:

```bash
cd charts/camunda-platform-8.9/test/integration/testsuites
npx playwright test
npx playwright test tests/identity.spec.ts
npx playwright test --grep "login"
```

## Single-Test Quick Reference

- Go function: `go test ./<pkg> -run TestName`
- Go subtest: `go test ./<pkg> -run 'TestName/Subtest'`
- Playwright file: `npx playwright test tests/<file>.spec.ts`
- Playwright grep: `npx playwright test --grep "case"`
- Bats file: `bats test/scripts/<file>.bats`

## Golden and Generated Artifacts

```bash
make go.test-golden-updated
make go.update-golden-only
chartPath=charts/camunda-platform-8.9 make helm.readme-update
chartPath=charts/camunda-platform-8.9 make helm.schema-update
```

## Code Style Guidelines

- Use `gofmt`; do not hand-format Go code.
- Keep imports grouped by gofmt conventions.
- Prefer typed structs over untyped maps unless data is dynamic/unstructured.
- Keep functions focused; extract helpers for repeated logic.
- Use table-driven tests for behavior matrices.
- Name tests `TestXxx` with clear subtest names.
- Wrap errors with context via `%w`.
- Avoid panic in production paths; return actionable errors.
- Keep Helm template logic readable and test-backed.
- Update/add unit tests whenever template behavior changes.
- Follow `.github/config/yamllint.yaml` (2-space indent, no duplicate keys, unix newlines).
- Naming: exported `PascalCase`, unexported `camelCase`, test files `*_test.go`.

## Error Handling and Assertions

- Include operation and resource context in returned errors.
- Preserve root cause with `%w` wrapping.
- In CLI paths, fail fast with remediation hints.
- In tests, use `require` for hard preconditions.

## PR/Commit and CI Rules (from `.github/AGENTS.md`)

- PR title format: `<type>[optional scope]: <description>`
- Commit format: `<type>[optional scope]: <description>`
- Valid types: `feat`, `fix`, `refactor`, `revert`, `test`, `docs`, `style`, `build`, `ci`, `cd`, `chore`, `chore(deps)`, `chore(release)`, `deps`
- Complex CI logic (>20 lines) should be implemented in Go under `scripts/`, not Bash.
- Any new Go script logic must include unit tests.

## Cursor/Copilot Rules Check

Checked these locations and found no files:

- `.cursor/rules/`
- `.cursorrules`
- `.github/copilot-instructions.md`

If these files are added later, merge their instructions into this guide.

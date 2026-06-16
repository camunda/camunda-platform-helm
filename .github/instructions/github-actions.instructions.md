---
applyTo: ".github/workflows/**"
---

# GitHub Actions — Scoped Instructions

## Overview

Workflows live in `.github/workflows/`. They follow a template + runner pattern: reusable
template workflows (named `*-template.yaml`) are called by trigger workflows (named
`*-runner.yaml` or per-event files). All workflows use pinned action SHAs (not mutable tags)
for security. Vault is the source of truth for secrets — never hard-code credentials. The
`hashicorp/vault-action` step imports secrets as environment variables before any step that
needs them. Complex logic (>20 lines of shell) must be implemented as a Go script, not inline
bash or standalone bash scripts. Commit messages from automated workflow steps follow Conventional Commits with the
`chore(ci):` type. Always set `permissions:` explicitly to the minimum required.

---

## Critical Rules

### NEVER
- **NEVER** use a mutable action tag (e.g., `actions/checkout@v4`) — always pin to a full commit
  SHA with a `# vX.Y.Z` comment.
- **NEVER** store secrets in workflow files, environment variables in plain text, or repository
  variables. All secrets come from Vault via `hashicorp/vault-action`.
- **NEVER** write inline bash scripts longer than ~20 lines — implement as a Go script with tests
  in `scripts/` instead.
- **NEVER** add new bash business-logic scripts in `scripts/` for workflow automation when logic
  exceeds ~20 lines, includes branching, calls APIs, or parses JSON.
- **NEVER** omit `concurrency:` on workflows triggered by `push` or `pull_request` — duplicate
  runs waste CI resources.
- **NEVER** use `strategy.fail-fast: true` for integration test matrices — one flaky test must
  not cancel all other matrix legs.
- **NEVER** use `secrets: inherit` without justification — list required secrets explicitly.

### ALWAYS
- **ALWAYS** set `permissions:` at the workflow or job level with only the minimum required scopes.
- **ALWAYS** pin all `uses:` references to a full SHA: `uses: actions/checkout@<sha> # vN`.
- **ALWAYS** use `workflow_call` inputs for reusable template workflows so callers can parameterise
  them without editing the template.
- **ALWAYS** use `cancel-in-progress: true` in `concurrency:` for PR-triggered workflows.
- **ALWAYS** use `make` targets (not raw commands) so local and CI behaviour match.
- **ALWAYS** install tools via `.github/actions/install-tool-versions` before using `go`, `helm`,
  `kubectl`, etc. — this respects the pinned versions in `.tool-versions`.
- **ALWAYS** implement new non-trivial workflow automation logic in Go under `scripts/<feature>/`
  (or an existing Go tooling module), with tests where practical.

---

## Core Patterns with Code Examples

### 1. Minimal Permissions Declaration

```yaml
permissions:
  contents: read
```

Or for workflows that push commits or comment on PRs:

```yaml
permissions:
  contents: write
  pull-requests: write
```

### 2. Vault Secrets Import

All secrets come from Vault. Use `hashicorp/vault-action` **before** any step that needs them.
Always set `exportEnv: false` so the values are **not** written to the job-wide `$GITHUB_ENV`.
Give the step an `id:` and pass each secret only into the specific step that consumes it, via
`${{ steps.<id>.outputs.<NAME> }}`:

```yaml
- name: Import Vault secrets
  uses: hashicorp/vault-action@892a26828f195e65540a40b4768ae4571f51ebfc # v4.0.0
  id: vault
  with:
    url: ${{ secrets.VAULT_ADDR }}
    method: approle
    roleId: ${{ secrets.VAULT_ROLE_ID }}
    secretId: ${{ secrets.VAULT_SECRET_ID }}
    secrets: |
      secret/data/products/distribution/ci GH_APP_ID_DISTRO_CI;
      secret/data/products/distribution/ci GH_APP_PRIVATE_KEY_DISTRO_CI;
    exportEnv: false

# Consume in a GitHub Action via its `with:` inputs — no env var needed:
- name: Generate GitHub token
  uses: tibdex/github-app-token@3beb63f4bd073e61482598c45c71c1019b59b73a # v2
  with:
    app_id: ${{ steps.vault.outputs.GH_APP_ID_DISTRO_CI }}
    private_key: ${{ steps.vault.outputs.GH_APP_PRIVATE_KEY_DISTRO_CI }}

# Consume in a shell `run:` step via a step-scoped `env:` block:
- name: Use a secret in a script
  env:
    GH_APP_ID_DISTRO_CI: ${{ steps.vault.outputs.GH_APP_ID_DISTRO_CI }}
  run: |
    echo "app id is ${GH_APP_ID_DISTRO_CI}"
```

Why `exportEnv: false`: the action default (`true`) injects every imported secret into
`$GITHUB_ENV`, making it readable by **all** subsequent steps in the job — including
third-party actions that have no need for it. Scoping each secret to its consumer keeps the
blast radius minimal.

For composite actions: secrets cannot be handed to the calling job via `$GITHUB_ENV` without
leaking them job-wide. Expose them as the composite's `outputs:` and let the caller wire each
consuming step's `env:` from `${{ steps.<setup-id>.outputs.<NAME> }}`.

### 3. Reusable Template Workflow Structure

```yaml
name: "Test - Unit - Template"

on:
  workflow_call:
    inputs:
      identifier:
        description: Unique identifier for the deployment hostname.
        required: true
        type: string
      camunda-helm-git-ref:
        required: false
        default: main
        type: string
      camunda-helm-dir:
        required: true
        type: string

concurrency:
  group: ${{ github.workflow }}-${{ inputs.identifier }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  unit:
    name: Unit - ${{ matrix.test.name }}
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        test: ${{ fromJson(needs.init.outputs.unitTestMatrix) }}
    env:
      chartPath: "charts/${{ inputs.camunda-helm-dir }}"
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6
      - name: Install tools
        uses: ./.github/actions/install-tool-versions
        with:
          tools: |
            golang
            helm
      - name: Add helm repos
        run: make helm.repos-add
      - name: Get Helm dependency
        run: make helm.dependency-update
      - name: Run tests
        run: |
          cd charts/${{ inputs.camunda-helm-dir }}/test/unit
          go test $(printf "./%s " ${{ matrix.test.packages }})
```

### 4. Init Job + Matrix Pattern

```yaml
jobs:
  init:
    name: Generate matrix
    runs-on: ubuntu-latest
    outputs:
      testEnabled: ${{ steps.vars.outputs.testEnabled }}
      testMatrix: ${{ steps.vars.outputs.testMatrix }}
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6
      - name: Get test matrix
        id: vars
        uses: ./.github/actions/test-type-vars
        with:
          chart-dir: "${{ inputs.camunda-helm-dir }}"

  run:
    name: Run - ${{ matrix.test.name }}
    needs: init
    if: needs.init.outputs.testEnabled == 'true'
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        test: ${{ fromJson(needs.init.outputs.testMatrix) }}
```

### 5. Automated Commit Format

Workflow steps that commit changes (e.g., updating docs, bumping versions) must use
Conventional Commits format with the `chore(ci):` type:

```yaml
- name: Commit changes
  run: |
    git config user.name "github-actions[bot]"
    git config user.email "github-actions[bot]@users.noreply.github.com"
    git add .
    git commit -m "chore(ci): update generated files [skip ci]"
    git push
```

Other valid CI commit types: `chore(release):`, `chore(deps):`, `docs:`.

### 6. Tool Installation

```yaml
- name: Install tools
  uses: ./.github/actions/install-tool-versions
  with:
    tools: |
      golang
      helm
      kubectl
```

After installing Go tools, reshim asdf if using custom Go binaries:

```yaml
- name: Install license tool
  run: |
    make go.addlicense-install
    asdf reshim golang
```

### 7. Concurrency for PR Workflows

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true
```

### 8. Conditional Job Execution

```yaml
jobs:
  chores:
    needs: [init]
    if: needs.init.outputs.should-run == 'true'
```

---

## Common Mistakes

1. **Mutable action tags** — `uses: actions/checkout@v4` is mutable and a supply-chain risk.
   Always use the full commit SHA pinned to a specific version tag comment.

2. **Hardcoded secrets** — putting `password: ${{ secrets.MY_SECRET }}` directly in a step
   instead of importing from Vault makes secret rotation harder and violates the security model.

3. **Inline bash >20 lines** — complex logic in `run:` blocks has no tests and is hard to
   maintain. Move it to a Go script in `scripts/` with unit tests.

4. **Standalone bash automation scripts for business logic** — adding new `scripts/*.sh` files
  that perform API orchestration, JSON parsing, and branching duplicates logic that should live in
  Go and be testable.

5. **Missing `fail-fast: false`** — default is `true`, which cancels all matrix jobs when one
   fails. This is almost never wanted for test matrices.

6. **Skipping `concurrency:`** — PR workflows without concurrency run duplicate jobs on rapid
   force-pushes, wasting CI minutes.

7. **Not using `make` targets** — running `go test ./...` directly instead of `make go.test`
   skips pre-test steps (license checks, formatting, dependency updates).

8. **Missing `id-token: write`** — OIDC-based Vault auth requires `permissions.id-token: write`
   at job or workflow level.

9. **Skipping debug output** — add an info step (`echo "output: ${{ steps.id.outputs.val }}"`)
   after complex composite actions so failures are easier to diagnose.

10. **`exportEnv: true` on `vault-action`** — the default injects every imported secret into
    job-wide `$GITHUB_ENV`, exposing it to all subsequent steps (and any third-party action)
    in the job. Always set `exportEnv: false` and consume via `${{ steps.<id>.outputs.<NAME> }}`
    in the consuming step's `with:` input or a step-scoped `env:` block (see Pattern 2).

---

## Resources

- GitHub Actions security hardening: <https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions>
- Conventional Commits: <https://www.conventionalcommits.org/>
- Vault action: <https://github.com/hashicorp/vault-action>
- Reusable workflows: <https://docs.github.com/en/actions/using-workflows/reusing-workflows>
- `.github/AGENTS.md` — CI architecture and integration test patterns
- `.tool-versions` — pinned tool versions for `asdf`

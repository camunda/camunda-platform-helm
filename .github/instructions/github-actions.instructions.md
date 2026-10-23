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
needs them. `run:` blocks have a complexity limit (see Critical Rules). Commit messages from
automated workflow steps follow Conventional Commits with the `chore(ci):` type. Always set `permissions:` explicitly to the minimum required.

---

## Critical Rules

### NEVER
- **NEVER** use a mutable action tag (e.g., `actions/checkout@v4`) — always pin to a full commit
  SHA with a `# vX.Y.Z` comment.
- **NEVER** store secrets in workflow files, environment variables in plain text, or repository
  variables. All secrets come from Vault via `hashicorp/vault-action`.
- **NEVER** add a `run:` block above bash cyclomatic complexity 10 (1, plus 1 for each `if`/`elif`,
  `case` item, loop, `&&` and `||`) or 100 lines of code (blank and comment-only lines do not
  count), in `.github/workflows/` or
  `.github/actions/`, and never make a block that is already above more complex or longer.
  `make go.complexity` fails the change, and `test-release-tooling.yaml` runs it on every PR.
  The same check runs shellcheck on the `run:` blocks of the composite actions that a change
  touches and fails on any finding.
  `cd scripts/complexity-gate && go run . --repo-root ../.. --all` lists the blocks that are
  already above. To pass, delete dead branches and duplicated commands, or split the step where
  no shell variable crosses the split. Do not add a `GITHUB_ENV` or `GITHUB_OUTPUT` write, a `.sh`
  file, a `${{ }}` expression or an `if:` condition only to pass the check, and do not rewrite the
  block in another language to pass it. If a fix must grow a block that is already above, say so
  in the PR body.
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

---

## Test Modified Release Workflows

The release workflows are `chart-build-dev.yaml`, `chart-promote-rc.yaml`, and
`chart-release-public.yaml`. They do not run on `pull_request`. A green PR does not prove that
they work. Run each modified release workflow on the PR branch before the PR is ready.

A release workflow is modified when the PR changes one of these:

- The workflow file.
- A composite action, script, Go tool, or Makefile target that the workflow calls.

### 1. Find the modified release workflows

Search for each changed file name or Makefile target:

```bash
grep -ln '<file-or-target-name>' .github/workflows/chart-build-dev.yaml .github/workflows/chart-promote-rc.yaml .github/workflows/chart-release-public.yaml
```

If a composite action calls the changed file, search for the action name.

### 2. Run each modified release workflow on the PR branch

Read the `if:` conditions of the changed steps and their jobs. Select inputs that make the
changed steps run. The release workflows write to Harbor. Use these commands:

| Workflow | Command | Effect |
|---|---|---|
| `chart-build-dev.yaml` | `gh workflow run chart-build-dev.yaml --ref <branch> -f chart-version=<8.x>` | Pushes `{version}-dev-{sha}`. Moves `{major}-dev-latest` to the branch build. The next `main` build moves it back. |
| `chart-promote-rc.yaml` | `gh workflow run chart-promote-rc.yaml --ref <branch> -f dev-tag=<major>-dev-latest -f dry-run=true` | Writes only `-rc-dryrun` tags. Creates no release PR. |
| `chart-release-public.yaml` | `gh workflow run chart-release-public.yaml --ref <branch> -f rc-tag=<major>-rc-latest -f dry-run=true` | Publishes nothing. |

`<major>` is the chart major version, for example `15` for 8.10. If `chart-promote-rc.yaml`
fails with "is not on main branch", use the dev tag of the last `main` build.

- Build Dev runs the steps behind `HAS_IMAGE_OVERRIDES` only when the run sets an `*-image-tag` input.
  If you change one of these steps, set one input, for example
  `-f orchestration-image-tag=<tag from values.yaml>`.
- A dry run of `chart-release-public.yaml` skips the `post-release` and `ensure-release-pr-merges`
  jobs. A dry run cannot test a change in these jobs. Tell the user. Do not mark the PR ready
  until the user gives a test plan.
- If a release workflow has a `dry-run` input, always test it with `dry-run=true`. Never test it
  with `dry-run=false`.

### 3. Check the run

A green run is not a test when the run skipped a changed step. For example, a push run of Build Dev
is green, but it skips the `HAS_IMAGE_OVERRIDES` steps.

List the step results:

```bash
gh run view <run-id> --json jobs --jq '.jobs[] | .name as $job | .steps[] | "\(.conclusion)\t\($job) / \(.name)"'
```

- Each changed step must show `success`.
- A changed step that shows `skipped` means that the inputs are wrong. Run the workflow again
  with other inputs.
- A failure that the change did not cause, for example a network error, needs a rerun. It is
  not a test.

### 4. Record the run

Add the run URL for each modified release workflow to the PR body, under "What's in this PR?".

### 5. Do not skip the test

The test is mandatory. A user request does not change this.

- You can push the branch and open a draft PR before the test.
- Do not mark the PR ready, merge it, or enable auto-merge before each modified release workflow
  has a passing run.
- If the user tells you to skip the test, refuse. Name the untested release workflow. Give the
  exact command that runs it.
- Refuse again each time that the user asks.
- "CI will catch it" and "it is a small change" are not reasons to skip the test.
- If the user gives a run URL, check it with step 3 before you accept it.

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
  uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
  with:
    app-id: ${{ steps.vault.outputs.GH_APP_ID_DISTRO_CI }}
    private-key: ${{ steps.vault.outputs.GH_APP_PRIVATE_KEY_DISTRO_CI }}

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
blast radius minimal. **Exception:** the per-scenario mapping step in `integration-test-setup`
keeps `exportEnv: true` because the following `vault-secret-mapper` `go run` resolves the
mapped secrets through `os.Getenv`; `exportEnv: false` there would yield an empty Secret manifest.

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

For SHA pinning, Vault-only secrets, `exportEnv: false` (and its one exception), over-limit
`run:` blocks, `fail-fast: false`, and `concurrency:`, see Critical Rules and Pattern 2 above. Additional pitfalls:

1. **Not using `make` targets** — running `go test ./...` directly instead of `make go.test`
   skips pre-test steps (license checks, formatting, dependency updates).

2. **Missing `id-token: write`** — OIDC-based Vault auth requires `permissions.id-token: write`
   at job or workflow level.

3. **Skipping debug output** — add an info step (`echo "output: ${{ steps.id.outputs.val }}"`)
   after complex composite actions so failures are easier to diagnose.

---

## Resources

- GitHub Actions security hardening: <https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions>
- Conventional Commits: <https://www.conventionalcommits.org/>
- Vault action: <https://github.com/hashicorp/vault-action>
- Reusable workflows: <https://docs.github.com/en/actions/using-workflows/reusing-workflows>
- `.github/AGENTS.md` — CI architecture and integration test patterns
- `.tool-versions` — pinned tool versions for `asdf`

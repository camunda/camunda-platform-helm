# Reproducing a CI E2E Test Failure Locally

When a PR check fails (e.g. `Playwright e2e after upgrade - upgrade-minor on gke - eske`), this guide walks you through pulling the logs and artifacts, decoding the scenario, and spinning up an identical local environment so you can iterate without waiting on CI.

Designed for application developers: you do not need deep Kubernetes knowledge — `deploy-camunda` handles cluster setup, and `gh` handles artifact retrieval.

Repo slug used throughout: `camunda/camunda-platform-helm`.

## Prerequisites

Two cold-start requirements for anyone new to this workflow:

```bash
# 1. Authenticate the GitHub CLI (used for check runs, logs, artifacts)
gh auth login

# 2. Point kubectl at the distro-ci GKE cluster
gcloud container clusters get-credentials distro-ci \
  --zone europe-west1-b \
  --project camunda-distribution
# Verify:
kubectl config current-context   # gke_camunda-distribution_europe-west1-b_distro-ci
```

If you don't have access to the `distro-ci` cluster yet, request it in [#ask-self-managed](https://camunda.slack.com/archives/C03UR0V2R2M) on Slack.

For the rest (Go/helm/asdf toolchain → `make install.dx-tooling`, Docker creds → Step 5, Playwright browsers → Gotchas) see the linked sections.

## Step 1 — Find the Failing Check Run

From the PR's commit SHA (or `git rev-parse HEAD` if checked out locally):

```bash
COMMIT=<sha>
gh api repos/camunda/camunda-platform-helm/commits/$COMMIT/check-runs --paginate \
  -q '.check_runs[] | select(.conclusion=="failure") | {id, name, html_url}'
```

Note the check-run `id` and the workflow run id (from `html_url`, the number after `/actions/runs/`).

## Step 2 — Pull the Job Log

```bash
mkdir -p test-artifacts
JOB_ID=<check-run id from step 1>
gh api repos/camunda/camunda-platform-helm/actions/jobs/$JOB_ID/logs \
  > test-artifacts/job.log
```

For Playwright failures, search the log for `✘`, `Error:`, `expect(`, or `timed out`.

## Step 3 — Download Artifacts

```bash
RUN_ID=<workflow run id from step 1>
gh api repos/camunda/camunda-platform-helm/actions/runs/$RUN_ID/artifacts \
  -q '.artifacts[] | {id, name}'

# For each artifact of interest:
gh api repos/camunda/camunda-platform-helm/actions/artifacts/<id>/zip \
  > test-artifacts/<name>.zip
unzip test-artifacts/<name>.zip -d test-artifacts/<name>/
```

Artifacts to prioritize for Playwright e2e failures:

| Artifact suffix | Why you want it |
|-----------------|-----------------|
| `*-playwright-results-json` | Machine-readable test outcomes + error messages |
| `*-e2e-html-report` | Interactive report with screenshots, videos, traces |
| `*-playwright-report-runner` | Runner-level summary |
| `*-blob-report` | Raw Playwright blobs (large; only if merging reports) |

## Step 4 — Decode the Scenario Shortname

CI job names encode scenarios with shortnames like `eske` (Elasticsearch + Keycloak) or `kemt` (Keycloak + multitenancy). Decode them to `deploy-camunda` flags:

```bash
deploy-camunda matrix list --repo-root . --versions 8.9 --shortname-filter eske
```

Output shows the chart version, scenario, identity, persistence, platform, and which flows (install, upgrade-minor, upgrade-patch) are permitted.

## Step 5 — Deploy an Identical Environment

**Install-only flows** (single-version, no upgrade):

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace test-eske \
  --release integration \
  --scenario chart-full-setup \
  --identity keycloak \
  --persistence elasticsearch
```

**Upgrade flows** (install previous version, then upgrade to local chart — handled automatically):

```bash
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.9 \
  --shortname-filter eske \
  --flow-filter upgrade-minor \
  --platform gke \
  --delete-namespace \
  --timeout 15 --yes
```

Docker credentials must be exported first (`TEST_DOCKER_USERNAME{,_CAMUNDA_CLOUD}` and matching `_PASSWORD`).

## Step 6 — Run the E2E Tests Locally

Fastest path — let `deploy-camunda` run them after deploy:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace test-eske \
  --release integration \
  --scenario chart-full-setup \
  --identity keycloak --persistence elasticsearch \
  --test-e2e
```

Or against an already-deployed namespace:

```bash
./scripts/run-e2e-tests.sh \
  --absolute-chart-path $PWD/charts/camunda-platform-8.9 \
  --namespace test-eske
```

## Step 7 — Clean Up

```bash
helm uninstall integration -n test-eske
kubectl delete namespace test-eske --wait=true
```

## Gotchas

- **`deploy-camunda: command not found` / `No version is set for command`** — the `asdf` shim didn't activate Go. Build directly: `make build.deploy-camunda`, then invoke via `./scripts/deploy-camunda/deploy-camunda`.
- **Playwright can't find a browser** — run `npx playwright install chromium`, or point at a system binary with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium-browser`.
- **Tests pass locally but failed in CI** — compare chart image tags. CI uses `values-latest.yaml` / `values-digest.yaml`; local defaults may lag. Run with `--qa` to match CI image selection.
- **Reproducing confirms the failure is upstream** (test suite / product bug, not your PR) — capture the `deploy-camunda` invocation and the failing test name in the PR thread so the next reviewer doesn't redo the work.

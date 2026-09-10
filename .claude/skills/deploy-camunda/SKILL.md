---
name: deploy-camunda
description: Deploy Camunda 8 Self-Managed to Kubernetes with the deploy-camunda CLI — single scenarios, layered values composition, configuration profiles, CI matrix operations, SNAPSHOT image tags, offline rendering, and live deploy watching. Use when deploying a chart to a cluster, running or listing matrix scenarios, debugging values-layer merging, or watching a stuck install.
---

# deploy-camunda CLI

The `deploy-camunda` CLI orchestrates Camunda Platform Helm deployments. It resolves layered values files, performs environment variable substitution, manages Keycloak realms and Elasticsearch index prefixes, and supports parallel multi-scenario deployments.

Throughout: `$NS` = Kubernetes namespace, `$RELEASE` = Helm release name.

**Source:** `scripts/deploy-camunda/` — see its `README.md` for the full secrets/env model, scenario catalog, operators, and watch internals.

**Full flag reference:** `deploy-camunda --help`, `deploy-camunda matrix run --help`, `deploy-camunda prepare-values --help`.

## Installation

```bash
# Recommended: install all Go CLI tools
make install.dx-tooling

# Or just deploy-camunda
cd scripts/deploy-camunda && go build -o deploy-camunda . && mv deploy-camunda $GOPATH/bin/
```

**Rebuild after every pull.** `deploy-camunda` tracks chart-side changes; a stale binary silently rejects new flags with `unknown flag`. The binary exposes no way to print its own build version (the existing `--version` / `-v` flag selects a *chart* version, not the binary version), so rebuild unconditionally: `make install.dx-tooling && asdf reshim golang`.

## Pre-Flight Checklist

> **Shortcut:** `deploy-camunda doctor` runs the checks below automatically and prints a
> ✓/✗ checklist (config resolved, kube-context reachable, docker creds present, every var
> in the vault mapping, the scenario's values, and companion charts set). It only flags
> vars you must supply — deploy-computed ones like `CAMUNDA_HOSTNAME`/`KEYCLOAK_REALM` are
> recognized as satisfied. `deploy-camunda doctor --fix` prompts for anything missing and
> writes it to `.env`. First-time setup? `deploy-camunda config init` scaffolds config +
> `.env` (including Postgres/RDBMS dev creds) and ends with the same checklist. A direct
> `deploy-camunda` run also fails fast on missing inputs before touching the cluster
> (bypass with `--skip-preflight`); matrix entries do too.

Before deploying, verify these requirements. Skipping any of these is the most common source
of wasted time (pods stuck in `ImagePullBackOff`, missing ingress, helm errors):

1. **Docker credentials** — the matrix runner creates K8s pull secrets from env vars. Without them,
   pods will fail with `ImagePullBackOff` after deployment appears to succeed. Ask the user to
   confirm; do not attempt to extract credentials automatically.
   ```bash
   # Harbor (required for all deployments)
   echo $TEST_DOCKER_USERNAME_CAMUNDA_CLOUD   # should be: ci-distribution
   echo $TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD   # should be non-empty

   # Docker Hub (only if using --ensure-docker-hub)
   echo $TEST_DOCKER_USERNAME
   echo $TEST_DOCKER_PASSWORD
   ```
   Both are needed when `ensureDockerHub` and `ensureDockerRegistry` are `true` in `.deploy-camunda.yaml`.

2. **kubectl context** — confirm you're targeting the right cluster.
   ```bash
   kubectl config current-context
   # Expected for GKE: gke_camunda-distribution_europe-west1-b_distro-ci
   ```

3. **Helm dependencies** — must be up to date for the target chart version.
   ```bash
   make helm.dependency-update chartPath=charts/camunda-platform-8.10
   ```

4. **Ingress hostname** — for matrix deploys this is computed automatically. For single deploys,
   you must provide it via `CAMUNDA_HOSTNAME` env var or `--ingress-hostname` flag.
   ```bash
   # Matrix deploy: namespace becomes the hostname prefix automatically
   # e.g., matrix-810-keyco-inst-gke.ci.distro.ultrawombat.com

   # Single deploy: set explicitly
   export CAMUNDA_HOSTNAME=my-test-ns.ci.distro.ultrawombat.com
   ```

## Deploy a Single Scenario

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS \
  --release $RELEASE \
  --scenario chart-full-setup
```

## Deploy with Selection + Composition

The preferred way to configure deployments is through selection flags that compose layered values:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup \
  --identity keycloak \
  --persistence opensearch-embedded \
  --features multitenancy,documentstore \
  --qa
```

Available selections:

| Flag | Values |
|------|--------|
| `--identity` | `keycloak`, `oidc`, `basic`, `hybrid` |
| `--persistence` | `elasticsearch`, `opensearch`, `rdbms`, `rdbms-oracle` |
| `--test-platform` | `gke`, `eks` |
| `--features` | `multitenancy`, `rba`, `documentstore` |
| `--qa` | (boolean) Enable QA configuration |
| `--upgrade-flow` | (boolean) Enable upgrade flow configuration |

Layer resolution order (last wins):

```
base.yaml -> base-upgrade.yaml (if upgrade) -> identity -> persistence -> platform -> features -> QA -> image-tags
```

Layered values live in: `charts/<version>/test/integration/scenarios/chart-full-setup/values/`.
Full resolution semantics per chart version: `docs/skills/integration-test-scenario-resolution.md`.

## Configuration Profiles

Create a config file at `.deploy-camunda.yaml` (project root) or `~/.config/camunda/deploy.yaml`:

```yaml
current: dev
repoRoot: /path/to/repo

deployments:
  dev:
    chartPath: ./charts/camunda-platform-8.9
    namespace: dev-test
    release: camunda
    scenario: chart-full-setup

matrix:
  maxParallel: 33
  namespacePrefix: distribution
  ensureDockerHub: true
  ensureDockerRegistry: true
  kubeContexts:
    gke: gke_camunda-distribution_europe-west1-b_distro-ci
  ingressBaseDomains:
    gke: ci.distro.ultrawombat.com
  envFiles:
    "8.9": .env.89
```

Manage profiles: `deploy-camunda config create|use|show|set`.

## Matrix Operations

The matrix manages the CI test matrix — all scenario/version/platform combinations defined per chart version in the composable registry (`test/ci/registry/`).

```bash
# List all scenarios
deploy-camunda matrix list --repo-root . --versions 8.9

# Filter by shortname
deploy-camunda matrix list --repo-root . --shortname-filter eske

# Dry-run: preview what would be deployed
deploy-camunda matrix run --repo-root . --versions 8.9 --shortname-filter eske --dry-run

# Run a specific scenario on GKE
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.9 \
  --shortname-filter eske \
  --flow-filter upgrade-minor \
  --platform gke \
  --delete-namespace \
  --timeout 15 \
  --yes

# CI-parity overrides: --extra-helm-arg, --extra-helm-set
```

For local runs, use `--namespace-prefix` and let `deploy-camunda` create the namespace and secrets. `--namespace-override` is for CI jobs that pre-provision the namespace; local live runs require `--namespace-prepared` because this mode disables automatic ExternalSecrets.

GKE matrix runs require `--ingress-base-domain-gke ci.distro.ultrawombat.com` — the host is computed per-namespace from that base domain; without it, `${CAMUNDA_HOSTNAME}` substitution in `base.yaml` fails with `missing required environment variables: CAMUNDA_HOSTNAME`.

The namespace convention is: `<prefix>-<version>-<shortname>-<flow>` (e.g., `distribution-89-eske-upgm-gke`).

**Upgrade flows are two-step** (handled automatically by `matrix run`):
1. Install the previous version's chart from the Helm repo (e.g., `camunda/camunda-platform@13.5.2` for 8.8)
2. `helm upgrade --force` to the local chart with `base-upgrade.yaml` included

**Flow semantics:**
- **`modular-upgrade-minor` is single-step** and assumes a prior install in the namespace (matches CI staging).
- **`upgrade-minor` is two-step.** Step 1 installs the *remote* previous chart `camunda/camunda-platform` from the public Helm repo (`https://helm.camunda.io`) at the previous version (e.g., `<latest-8.8>` for `--versions 8.9 --flow-filter upgrade-minor`), pinned via `versionmatrix.DefaultHelmChartRef`. The *values* are still resolved from the previous version's local layers under `charts/camunda-platform-8.8/test/integration/scenarios/chart-full-setup/`. Step 2 then `helm upgrade --force`s to the local chart.

**Deploying with SNAPSHOT image tags** (nightly CI pattern):

```bash
# Create a .env file with the SNAPSHOT image tags
cat > /tmp/snapshot-tags.env <<'EOF'
E2E_TESTS_ORCHESTRATION_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_CONNECTORS_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_OPTIMIZE_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_IDENTITY_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_CONSOLE_IMAGE_TAG=8.8-SNAPSHOT
E2E_TESTS_WEBMODELER_IMAGE_TAG=8.8
E2E_TESTS_SEARCH_ENGINE=opensearch
EOF

# Deploy a QA OpenSearch scenario with SNAPSHOT image tags
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.8 \
  --shortname-filter qaos \
  --platform gke \
  --env-file /tmp/snapshot-tags.env
```

The `qa-*` scenarios have `image-tags: true`, which includes `base-image-tags.yaml` (with `$E2E_TESTS_*_IMAGE_TAG` placeholders) and excludes `values-digest.yaml`. The `--env-file` provides the actual values for substitution via `buildScenarioEnv()`. In CI, the workflow converts the `VALUES_CONFIG` JSON to a `.env` file using `jq` before calling `deploy-camunda`.

## Extended-Support Versions (Opt-In)

The default matrix includes only `chartAutomation.routineVersions` from `charts/chart-versions.yaml`, independently of support-lifecycle metadata. A version outside that list is reachable when named explicitly **and** its chart dir has a CI scenario registry (`test/ci/registry/manifest.yaml`). **8.6** uses this opt-in path. A lifecycle `eolSince` entry blocks matrix execution, including explicit requests.

Every 8.6 scenario is `enabled: false`, so `--include-disabled` is mandatory — without it the run yields no entries.

```bash
# Validate a patched image against the 8.6 chart
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.6 \
  --include-disabled \
  --shortname-filter es \
  --shortname-exact \
  --flow-filter install \
  --platform gke \
  --ingress-base-domain-gke ci.distro.ultrawombat.com \
  --extra-values /tmp/patched-image.yaml \
  --ensure-docker-registry \
  --docker-username "$HARBOR_USERNAME" \
  --docker-password "$HARBOR_PASSWORD" \
  --yes
```

Deliberately no `--cleanup`: that is the flag that deletes the namespace *after* the run, and a verification run needs it to survive so the pod can be inspected afterwards (below). Delete it yourself when done — `kubectl delete namespace <ns>`.

`--delete-namespace` is a different thing: it deletes the namespace *before* deploying each entry, for a clean slate (internally `DeleteNamespaceFirst`). Omit it when an existing deployment must be preserved — `upgrade-patch` relies on that, and `runner_upgrade.go:320` forces it false for Step 2 because the namespace must persist from Step 1. So a namespace surviving an `upgrade-patch` run is expected when `--cleanup` is not set, not a bug.

Expect the install to take ~4-5 minutes and to look broken in the middle of it: optimize's `migration` init container enters `CrashLoopBackOff` with `java.net.ConnectException: Connection refused` until the Elasticsearch cluster reports `status: green`. There is no readiness gate between them, so it retries until ES is up and then proceeds. Confirm ES before treating an optimize crashloop as a real failure:

```bash
kubectl exec -n $NS integration-elasticsearch-master-0 -- \
  curl -s localhost:9200/_cluster/health | jq .status
```

8.6 shortnames: `es` (Keycloak + Elasticsearch), `os` (Keycloak + OpenSearch), `mt` (adds the `multitenancy` feature). `es` also carries `upgrade-patch` on `gke` and `eks`; the others are `install`/`gke` only.

**`kcor` does not exist for 8.6 and is not needed.** The `keycloak-original-install` scenario (`kcor`/`keyco`) is defined only in the 8.8 and 8.9 registries. Use `es` instead — it selects the same configuration that matters for validating an image: Keycloak identity with Elasticsearch persistence. `--shortname-filter` matches by substring, so pair it with `--shortname-exact`. On 8.9 a bare `--shortname-filter es` returns 9 entries (`eske`, `esoi`, `esss`, `esot`, `esarm`); against 8.6's current three shortnames it happens to be unambiguous, but the exact flag keeps the invocation correct if the registry gains one.

**Image override:** put the component's full image coordinates in the `--extra-values` file, including `pullSecrets`.

```yaml
identity:
  image:
    registry: registry.camunda.cloud
    repository: team-identity/identity
    tag: <patched-tag>
    pullSecrets:
      - name: index-docker-io
      - name: registry-camunda-cloud
global:
  image:
    pullSecrets:
      - name: index-docker-io
      - name: registry-camunda-cloud
```

Component-level pull-secret resolution is **exclusive**, not merged: setting `identity.image.pullSecrets` makes the chart ignore `global.image.pullSecrets` for that component. A global-only override therefore drops the secret and the pod lands in `ImagePullBackOff`.

A digest overlay cannot shadow the tag here — 8.6 simply ships no `values-digest.yaml` (only 8.8/8.9/8.10 do). Matrix runs do select the digest overlay by default (`resolveChartRootOverlays`, `matrix/runner.go:79`), so on the versions that have one, `neutralizeOverriddenDigests` (`deploy/digest_overlay.go:51`, wired at `deploy/values.go:890`) strips the digest of any component whose image coordinates `--extra-values` overrides, so the tag still wins.

**These scenarios never run e2e, even with `--test-e2e` or `--test-all`.** All three set `skip-e2e: true`, and the runner computes `RunE2ETests: (opts.TestE2E || opts.TestAll) && !entry.SkipE2E` (`matrix/runner_execute.go:250`) — the scenario flag wins unconditionally. This is deliberate: the cross-component e2e suite ships no `SM-8.6` fixture directory, and its Keycloak admin-console login locators do not match 8.6's older console. Verify a deploy by inspecting the running pod instead — image contents, startup logs, and the OIDC discovery document:

```bash
NS=<namespace>
kubectl get pod -n $NS -l app.kubernetes.io/component=identity \
  -o jsonpath='{range .items[*]}{.status.containerStatuses[0].image}{"\n"}{.status.containerStatuses[0].imageID}{"\n"}{end}'
kubectl logs -n $NS -l app.kubernetes.io/component=identity | grep -iE "error|exception"
curl -sk "https://$NS.ci.distro.ultrawombat.com/auth/realms/camunda-platform/.well-known/openid-configuration" | jq .issuer
```

The OIDC path uses the fixed `camunda-platform` realm — `values/identity/keycloak.yaml` hardcodes `publicIssuerUrl: .../auth/realms/camunda-platform` on every chart version. The per-scenario `realm:` value the run summary prints (e.g. `elasticsearch-0760899c`) is not a Keycloak realm path; using it returns `Realm does not exist`.

For an extended-support version with **no** registry, `matrix run` fails with an error naming the missing manifest and pointing at the single-scenario path, which needs no registry:

```bash
deploy-camunda \
  --scenario charts/camunda-platform-8.5/test/integration/scenarios/chart-full-setup \
  --identity keycloak --persistence elasticsearch \
  --namespace $NS --release $RELEASE
```

## Render Without Deploying

Debug values merging without touching the cluster:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --render-templates --render-output-dir ./debug-output
```

## Prepare Values Standalone

```bash
deploy-camunda prepare-values \
  --chart-path ./charts/camunda-platform-8.9 \
  --identity keycloak --persistence elasticsearch \
  --interactive=false
# Returns path to merged values file
```

## Watch a Deploy (`deploy-camunda watch`)

When a Helm install gets stuck, the default `helm install --wait --timeout 10m` hides every signal until the timeout fires. `deploy-camunda watch` polls the cluster on a short cadence and hands each snapshot to a local agent CLI (Claude Code or opencode) for live diagnosis.

**ALWAYS run `deploy-camunda watch` in a second terminal alongside any deployment.** It detects CrashLoopBackOff, ImagePullBackOff, and other failures in real time, instead of waiting for the full helm timeout to expire.

```bash
# Terminal 1: deploy via matrix run
deploy-camunda matrix run --repo-root . --versions 8.10 \
  --shortname-filter keyco --platform gke --delete-namespace --timeout 10 --yes

# Terminal 2: watch (start immediately, it waits for pods to appear)
deploy-camunda watch \
  --namespace matrix-810-keyco-inst-gke \
  --release integration \
  --interval 30

# For single (non-matrix) deploys, same shape: watch --namespace <ns> --release <rel> --interval 30
```

The watcher prints a diagnosis on each tick and exits when all pods reach Running/Ready. Verdicts:
- **wait** — pods are starting normally, keep polling.
- **investigate** — something looks off (slow startup, pending PVCs), diagnosis printed.
- **abort** — unrecoverable failure detected (wrong image, missing secret). Use `--abort-confidence 0.85` to auto-exit when the agent is confident (default 0 disables auto-abort).

**Prerequisites:** `claude` or `opencode` must be on `PATH`. The watcher does NOT call any API directly — it shells out to whichever CLI is installed and uses that CLI's existing auth and model configuration.

Watcher internals (per-tick mechanics, verdict JSON schema, `--corpus-dir` eval/replay workflow): `scripts/deploy-camunda/README.md` → "Watch internals".

## Troubleshooting

**Values don't look right:**

```
1. deploy-camunda --render-templates --render-output-dir ./debug ...
2. Or: deploy-camunda prepare-values --chart-path ... --interactive=false
3. Compare: helm get values $RELEASE -n $NS
```

**Scenario not found:**

```
1. ls charts/camunda-platform-8.9/test/integration/scenarios/chart-full-setup/values/
2. deploy-camunda matrix list --repo-root . --versions 8.9
```

**Failing pods after deploy:** see the `cluster-debugging` skill.

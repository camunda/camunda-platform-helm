---
name: gke-verification
description: End-to-end fix verification on a live GKE cluster — pre-flight checklist (Docker credentials, kubectl context, helm dependencies, ingress hostname), deploy the exact CI scenario, watch it, generate credentials, reproduce the failure on main, verify the fix on the branch, clean up. Use when a chart change needs live-cluster verification before a PR, or when asked to reproduce/verify a fix against GKE.
---

# Deploying to GKE for Verification

When verifying a fix requires a live cluster, follow this workflow. This is the standard procedure for validating changes before creating a PR.

## 1. Pre-flight checklist

Confirm these requirements before deploying — they are the most common source of deployment failures:

1. **Docker credentials are set** — ask the user to confirm; do not attempt to extract credentials
   automatically. The matrix runner creates K8s pull secrets from these env vars. Without them,
   pods fail with `ImagePullBackOff`.
   - `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD` / `TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD` (Harbor, `registry.camunda.cloud`)
   - `TEST_DOCKER_USERNAME` / `TEST_DOCKER_PASSWORD` (Docker Hub, if `--ensure-docker-hub` is used)

   ```bash
   echo $TEST_DOCKER_USERNAME_CAMUNDA_CLOUD   # should be: ci-distribution
   echo $TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD   # should be non-empty
   ```

2. **kubectl context is correct:**
   ```bash
   kubectl config current-context
   # Expected for GKE: gke_camunda-distribution_europe-west1-b_distro-ci
   ```

3. **Helm dependencies are up to date:**
   ```bash
   make helm.dependency-update chartPath=charts/camunda-platform-<version>
   ```

4. **Ingress hostname** — for non-matrix deploys, set `CAMUNDA_HOSTNAME` or use `--ingress-hostname`.
   Matrix runs require `--ingress-base-domain-gke ci.distro.ultrawombat.com` (host computed
   per-namespace from that base domain; without it, values substitution fails on `CAMUNDA_HOSTNAME`).

`deploy-camunda doctor` automates these checks — see the `deploy-camunda` skill.

## 2. Deploy the scenario — always with `watch`

Use `deploy-camunda matrix run` to deploy the exact CI scenario. ALWAYS run `deploy-camunda watch`
in a second terminal — it polls pod and event state and surfaces root-cause diagnoses in real time,
far better than manually running `kubectl get pods` in a loop.

```bash
# Terminal 1: deploy
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.10 \
  --shortname-filter keyco \
  --platform gke \
  --ingress-base-domain-gke ci.distro.ultrawombat.com \
  --delete-namespace \
  --timeout 10 \
  --yes

# Terminal 2: watch (start immediately after deploy begins)
deploy-camunda watch \
  --namespace matrix-810-keyco-inst-gke \
  --release integration \
  --interval 30
```

The watcher exits automatically when all pods reach Running/Ready. If a pod enters
CrashLoopBackOff or ImagePullBackOff, the watcher diagnoses the root cause and prints it
immediately — no need to manually inspect events or logs.

The namespace naming convention is `matrix-<version>-<shortname>-<flow>-<platform>`.
Use `deploy-camunda matrix list --repo-root . --versions 8.10` to find the exact
namespace for a given shortname.

To validate tier-2 scenarios a PR adds or changes before merge, see the `rfr-validation` skill.

## 3. Generate credentials

```bash
HELM_REPO=$(pwd)
bash "$HELM_REPO/scripts/render-e2e-env.sh" \
  --absolute-chart-path "$HELM_REPO/charts/camunda-platform-8.10" \
  --namespace matrix-810-keyco-inst-gke \
  --output /path/to/e2e-repo/.env \
  --not-ci --run-smoke-tests --verbose
```

Or use `deploy-camunda --output-test-env` to generate credentials as part of the deploy step
(see the `e2e-testing` skill for details and `c8e2e` usage).

## 4. Run tests (reproduce then verify)

If verifying a fix: run tests on `main` first (reproduce), then on the fix branch (confirm fix).

```bash
E2E_REPO=/path/to/c8-cross-component-e2e-tests

# First: reproduce failure on main
cd $E2E_REPO && git checkout main
npx playwright test --project=chromium tests/SM-8.10/smoke-tests.spec.ts --trace on

# Then: verify fix on your branch
git checkout fix/your-branch-name
npx playwright test --project=chromium tests/SM-8.10/smoke-tests.spec.ts --trace on
```

## 5. Clean up

```bash
kubectl delete namespace matrix-810-keyco-inst-gke
```

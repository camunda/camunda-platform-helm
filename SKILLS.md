# Operational Skills

Instructions for using the primary operational tools in this repository: the `deploy-camunda` CLI and `kubectl`. Use these to deploy, debug, and test Camunda on Kubernetes.

Throughout this document, `$NS` refers to a Kubernetes namespace and `$RELEASE` refers to a Helm release name.

---

## deploy-camunda CLI

The `deploy-camunda` CLI orchestrates Camunda Platform Helm deployments. It resolves layered values files, performs environment variable substitution, manages Keycloak realms and Elasticsearch index prefixes, and supports parallel multi-scenario deployments.

**Source:** `scripts/deploy-camunda/`

**Full flag reference:** Run `deploy-camunda --help`, `deploy-camunda matrix run --help`, or `deploy-camunda prepare-values --help`.

### Installation

```bash
# Recommended: install all Go CLI tools
make install.dx-tooling

# Or just deploy-camunda
cd scripts/deploy-camunda && go build -o deploy-camunda . && mv deploy-camunda $GOPATH/bin/
```

### Deploy a Single Scenario

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS \
  --release $RELEASE \
  --scenario chart-full-setup
```

### Deploy with Selection + Composition

The preferred way to configure deployments is through selection flags that compose layered values:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE \
  --scenario chart-full-setup \
  --identity keycloak-external \
  --persistence opensearch \
  --features multitenancy,documentstore \
  --qa
```

Available selections:

| Flag | Values |
|------|--------|
| `--identity` | `keycloak`, `keycloak-external`, `oidc`, `basic`, `hybrid` |
| `--persistence` | `elasticsearch`, `opensearch`, `rdbms`, `rdbms-oracle` |
| `--test-platform` | `gke`, `eks`, `openshift` |
| `--features` | `multitenancy`, `rba`, `documentstore` |
| `--qa` | (boolean) Enable QA configuration |
| `--upgrade-flow` | (boolean) Enable upgrade flow configuration |

Layer resolution order (last wins):

```
base.yaml -> base-upgrade.yaml (if upgrade) -> identity -> persistence -> platform -> features -> QA -> image-tags
```

Layered values live in: `charts/<version>/test/integration/scenarios/chart-full-setup/values/`

### Configuration Profiles

Create a config file at `.camunda-deploy.yaml` (project root) or `~/.config/camunda/deploy.yaml`:

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

### Matrix Operations

The matrix manages the CI test matrix — all scenario/version/platform combinations defined in each chart's `ci-test-config.yaml`.

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
```

**Important: Docker credentials are required.** The matrix runner creates K8s pull secrets. Before running, ask the user to ensure these environment variables are set:

- **Harbor**: `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD` and `TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD`
- **Docker Hub**: `TEST_DOCKER_USERNAME` and `TEST_DOCKER_PASSWORD`

**Upgrade flows are two-step** (handled automatically by `matrix run`):
1. Install the previous version's chart from the Helm repo (e.g., `camunda/camunda-platform@13.5.2` for 8.8)
2. `helm upgrade --force` to the local chart with `base-upgrade.yaml` included

The namespace convention is: `<prefix>-<version>-<shortname>-<flow>` (e.g., `distribution-89-eske-upgm-gke`).

### Render Without Deploying

Debug values merging without touching the cluster:

```bash
deploy-camunda \
  --chart-path ./charts/camunda-platform-8.9 \
  --namespace $NS --release $RELEASE --scenario chart-full-setup \
  --render-templates --render-output-dir ./debug-output
```

### Prepare Values Standalone

```bash
deploy-camunda prepare-values \
  --chart-path ./charts/camunda-platform-8.9 \
  --identity keycloak --persistence elasticsearch \
  --interactive=false
# Returns path to merged values file
```

---

## kubectl — Debugging Deployments

### Check Deployment Health

```bash
kubectl get pods -n $NS -o wide
kubectl get deployments,statefulsets -n $NS
helm list -n $NS
helm get values $RELEASE -n $NS
```

**Pod states:**

| Status | Next Step |
|--------|-----------|
| `Pending` | `kubectl describe pod` — check resources, PVC, node selector |
| `CrashLoopBackOff` | `kubectl logs --previous` — check config, env vars, OOM |
| `ImagePullBackOff` | `kubectl describe pod` — check image name, registry secret |
| `Running` but not `Ready` | `kubectl describe pod` — check readiness probe |

### Debug Failing Pods

```bash
kubectl describe pod <pod-name> -n $NS          # Events section at bottom
kubectl logs <pod-name> -n $NS                   # Main container logs
kubectl logs <pod-name> -n $NS --previous        # Previous crash logs
kubectl logs <pod-name> -n $NS --all-containers  # All containers
kubectl get events -n $NS --sort-by=.lastTimestamp
```

Pod naming pattern (8.8+):
```
$RELEASE-zeebe-0/1/2          # Orchestration StatefulSet
$RELEASE-connectors-<hash>    # Connectors
$RELEASE-identity-<hash>      # Identity
$RELEASE-optimize-<hash>      # Optimize
$RELEASE-web-modeler-restapi-<hash>
$RELEASE-web-modeler-websockets-<hash>
$RELEASE-console-<hash>
$RELEASE-keycloak-0
$RELEASE-postgresql-0
```

### Port-Forward to Services

```bash
kubectl port-forward svc/$RELEASE-zeebe-gateway 26500:26500 -n $NS  # gRPC
kubectl port-forward svc/$RELEASE-identity 8084:80 -n $NS
kubectl port-forward svc/$RELEASE-optimize 8083:80 -n $NS
kubectl port-forward svc/$RELEASE-connectors 8085:8080 -n $NS
kubectl port-forward svc/$RELEASE-console 8088:80 -n $NS
kubectl port-forward svc/$RELEASE-keycloak 18080:80 -n $NS
kubectl port-forward svc/$RELEASE-elasticsearch 9200:9200 -n $NS
```

### Manage Secrets

```bash
kubectl get secrets -n $NS
kubectl get secret <name> -n $NS -o jsonpath="{.data.<key>}" | base64 -d
kubectl get secret <name> -n $NS -o json | jq '.data | map_values(@base64d)'
```

### Namespace Lifecycle

```bash
kubectl create ns $NS --dry-run=client -o yaml | kubectl apply -f -
kubectl delete namespace $NS --wait=true
```

### Post-Uninstall Cleanup

```bash
helm uninstall $RELEASE -n $NS
kubectl delete pvc -l app.kubernetes.io/instance=$RELEASE -n $NS
```

---

## Troubleshooting

### Deployment fails

```
1. kubectl get pods -n $NS -o wide
2. kubectl describe pod <failing-pod> -n $NS
3. kubectl logs <pod> -n $NS --previous
4. kubectl get events -n $NS --sort-by=.lastTimestamp
5. helm status $RELEASE -n $NS
```

### Values don't look right

```
1. deploy-camunda --render-templates --render-output-dir ./debug ...
2. Or: deploy-camunda prepare-values --chart-path ... --interactive=false
3. Compare: helm get values $RELEASE -n $NS
```

### Scenario not found

```
1. ls charts/camunda-platform-8.9/test/integration/scenarios/chart-full-setup/values/
2. deploy-camunda matrix list --repo-root . --versions 8.9
```

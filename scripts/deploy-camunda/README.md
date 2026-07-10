# deploy-camunda

`deploy-camunda` is a Go/Cobra CLI that stands up Camunda 8 Self-Managed on
a Kubernetes cluster from this repository's Helm charts. It hides the
scenario/flow/values-file wiring that CI uses so you can deploy a full
Camunda cluster with one command instead of tracing a workflow file.

## Overview

The tool exposes the scenario catalog that Camunda CI already exercises
(`charts/<version>/test/ci/registry/scenarios/*.yaml`). The mental model is:

> **Pick a scenario → point it at your cluster → let deploy-camunda wire
> everything (companion charts, secrets, ingress, values overlays) for you.**

Scenarios are the source of truth for a "what does a Camunda deployment
of type X look like" question. Each one declares:

- an identity provider (`keycloak`, `oidc`, `basic`, `auth0`),
- a persistence backend (`elasticsearch`, `opensearch`, `rdbms`, …),
- one or more flows (`install`, `upgrade-minor`, `modular-upgrade-minor`),
- the platforms it runs on (`gke`, `eks`), and
- companion charts it needs (Keycloak, PostgreSQL, Elasticsearch, …).

You point `deploy-camunda` (or `deploy-camunda matrix run`) at a scenario
via `--identity` + `--persistence` + optional `--features`, or by its
canonical name via `--shortname-filter`. Anything not covered by an
existing scenario is a couple of YAML lines in your own registry (see
[Customising a scenario](#customising-a-scenario)).

Who this is for: reliability engineers, load-test engineers, and any
external Camunda team who needs a repeatable "give me a working Camunda"
button without becoming a Helm-values expert.

## Prerequisites

Before your first run:

- **`kubectl`** on your PATH, pointed at the cluster you want to deploy
  into. Confirm with `kubectl config current-context`.
- **Helm 3.x** on your PATH (`helm version`).
- **`deploy-camunda` binary installed** — from the repo root:

  ```bash
  make install.deploy-camunda   # installs into $GOPATH/bin via `go install .`
  # or, without installing globally:
  make build.deploy-camunda     # produces ./scripts/deploy-camunda/deploy-camunda
  ```

- **Docker Hub credentials (optional but recommended).** Docker Hub
  applies anonymous pull-rate limits per source IP and CI nodes hit
  them regularly. Set `DOCKERHUB_USERNAME` / `DOCKERHUB_PASSWORD` in
  your environment (or via `--dockerhub-username` / `--dockerhub-password`)
  and pass `--ensure-docker-hub` so the pull secret is created in the
  namespace before pods try to pull.
- **Operator CRDs pre-installed (optional).** If your scenario relies on
  an operator-managed dependency — for example, the ECK operator
  provisioning an Elasticsearch cluster — install the operator's CRDs
  before running `deploy-camunda`. For ECK, follow Elastic's
  "Cloud on Kubernetes → Install" documentation (search "ECK operator
  install").
  See [Using operators](#using-operators) for how to wire an
  operator-provisioned dependency into a scenario.

## Quick start

The minimum path from nothing to a running Camunda cluster:

```bash
# 1. Install the binary.
make install.deploy-camunda

# 2. Bootstrap a config file. Interactive; ends with a doctor checklist.
deploy-camunda config init
#   or, non-interactively, drop the annotated starter into place:
deploy-camunda config init --from-example getting-started

# 3. Edit .deploy-camunda.yaml.
#    - set `kubeContext` to your cluster context
#    - set `ingressBaseDomain` to your ingress DNS zone
#    - set `chartPath` to the version you want (e.g. charts/camunda-platform-8.9)

# 4. Verify preflight is green.
deploy-camunda doctor

# 5. Deploy the default scenario (keycloak + elasticsearch).
deploy-camunda
```

`deploy-camunda` reads your `.deploy-camunda.yaml` on every run — you
don't need to repeat the flags on the command line. Precedence is
`CLI flag  >  active profile in config file  >  root config  >  defaults`,
so ad-hoc overrides still work when you need them.

To run a specific canonical scenario by shortname (equivalent to what
CI does), use `matrix run`:

```bash
deploy-camunda matrix run \
  --repo-root . \
  --versions 8.10 \
  --shortname-filter keycloak-original \
  --ingress-base-domain <your-ingress-zone>
```

(Note the plural `--versions` — accepts a comma-separated list.)

## Scenario catalog

Scenarios live under
`charts/<chart-version>/test/ci/registry/scenarios/`, one YAML per
scenario definition. Each YAML declares `name`, `identity`, `persistence`,
`flows`, `platforms`, and the companion chart `dependencies` it needs.

The most common starting points on chart 8.10:

| Scenario file | Best for | Identity | Persistence | Flows |
|---|---|---|---|---|
| `keycloak-original-install-tier1.yaml` | **Default "I want a Camunda cluster" — Keycloak + bundled Elasticsearch.** | keycloak | elasticsearch | install |
| `elasticsearch.yaml` | Slightly modernised variant of the above. | keycloak | elasticsearch | install |
| `elasticsearch-basic.yaml` | **Simplest possible — basic auth, no Keycloak. Good for local dev.** | basic | elasticsearch | install |
| `opensearch.yaml` | OpenSearch instead of Elasticsearch (bundled, no TLS). | keycloak | opensearch-embedded | install |
| `opensearch-self-signed.yaml` | OpenSearch with an in-scenario TLS trust chain. | keycloak | opensearch-self-signed | install |
| `rdbms.yaml` | Secondary-storage RDBMS (PostgreSQL) alongside Elasticsearch. | keycloak | rdbms | install |
| `rdbms-external.yaml` | RDBMS pointing at a cluster you provision separately. | keycloak | rdbms-external | install |
| `rdbms-self-signed.yaml` | RDBMS with self-signed PostgreSQL TLS. | keycloak | rdbms-self-signed | install |
| `oidc.yaml` | External OIDC IdP (Entra) instead of Keycloak. | oidc | elasticsearch | upgrade-minor |
| `auth0.yaml` | Auth0 as the IdP. | auth0 | elasticsearch | install |
| `keycloak-mt.yaml` | Multi-tenancy enabled. | keycloak | elasticsearch | upgrade-minor |
| `keycloak-rba.yaml` | Role-based authorization enabled. | keycloak | elasticsearch | upgrade-minor |
| `documentstore.yaml` | Document store feature enabled. | keycloak | elasticsearch | — |
| `gateway-keycloak.yaml` | Gateway-API based Keycloak (post-deploy hook installs the CRD). | keycloak | elasticsearch | install |
| `orchestration-tls.yaml` | Full mTLS between orchestration components. | keycloak | elasticsearch | install |
| `no-secondary-storage.yaml` | Camunda without an Elasticsearch back-end. | keycloak | no-elasticsearch | install |
| `alwaysgreen.yaml` | Canary/smoke scenario. | keycloak | elasticsearch | install |

The `qa-*` variants (many of them) mirror these onto the QA node pool
(via `nodeSelector` / `tolerations` in the companion values files under
`test/integration/companion-values/`) and are typically run only by CI.

**"I am a bank of X type" mental model.** Pick the scenario closest to
your production shape and layer overrides on top — don't try to compose
one from scratch:

- *"I want the plainest Camunda I can get."* → `elasticsearch-basic`.
- *"I use Keycloak."* → `keycloak-original-install-tier1`.
- *"I use Entra ID / another OIDC provider."* → `oidc`.
- *"I use PostgreSQL as secondary storage."* → `rdbms` or `rdbms-external`.
- *"I use OpenSearch with our own certs."* → `opensearch-self-signed`.
- *"I need multi-tenancy."* → `keycloak-mt`.

Full list (47 scenarios on 8.10):

```bash
ls charts/camunda-platform-8.10/test/ci/registry/scenarios/
```

## Customising a scenario

Three layers of override, in increasing scope:

### 1. Override values for the Camunda platform chart

Use `--extra-values` (`extraValues:` in the config file) to append your
own values file to the merge. It's applied **last**, so it wins over
scenario defaults:

```bash
deploy-camunda \
  --shortname-filter keycloak-original \
  --extra-values ./my-overrides.yaml
```

Or in `.deploy-camunda.yaml`:

```yaml
deployments:
  local-dev:
    identity: keycloak
    persistence: elasticsearch
    extraValues:
      - ./my-overrides.yaml
```

For chart-root overlays (files named `values-<name>.yaml` at the top of
`charts/camunda-platform-<v>/`), use `--values-preset` (comma-separated
or repeatable), e.g. `--values-preset enterprise,digest`.

### 2. Override values for a companion chart (Elasticsearch, Keycloak, …)

Companion charts (external persistence and IdP) use their own values
files at `test/integration/companion-values/`. The starting files —
`elasticsearch.yaml`, `keycloak.yaml`, `postgresql.yaml`, `opensearch.yaml`
— are the ones a scenario picks up by default. To override, either edit
the values file directly (fine for local testing) or add a new variant
alongside and reference it from a new scenario (see below).

Example: change Elasticsearch replica count.
Edit `test/integration/companion-values/elasticsearch.yaml`:

```yaml
replicas: 3
```

`deploy-camunda` picks it up on the next run — no CLI flag needed.
See [`test/integration/companion-values/README.md`](../../test/integration/companion-values/README.md)
for which files are external-user defaults vs CI-only variants.

### 3. Add a new scenario

To add a scenario for a shape you frequently deploy:

1. Create `charts/<v>/test/ci/registry/scenarios/<my-scenario>.yaml`:

   ```yaml
   name: my-scenario
   auth: keycloak
   flows:
     - install
   identity: keycloak
   persistence: elasticsearch
   platforms:
     - gke
   infra-type:
     gke: distroci
   dependencies:
     - keycloak
     - elasticsearch
     - postgresql
   ```
2. If it needs a values overlay unique to your setup, add one under
   `charts/<v>/test/integration/scenarios/chart-full-setup/values/` and
   reference it via the registry entry's identity/persistence keys.
3. Re-run `make go.update-registry-golden` to refresh the CI registry
   snapshot so `TestLifecycleFixtures` stays green.

`deploy-camunda matrix run --shortname-filter my-scenario` now picks it
up. When you're done iterating, if it's useful to more than one person,
send a PR.

## Using operators

Some persistence and IdP options are best provisioned by a Kubernetes
**operator** (e.g. the Elastic ECK operator instead of the bundled
`elastic/elasticsearch` Helm chart). deploy-camunda exposes three
lifecycle hook types so a scenario can wire an operator-managed
dependency without leaking that logic into the platform chart.

### Lifecycle hooks

Every scenario or flow can declare one of three hooks, each shaped
either as a set of `fixtures:` (YAML manifests server-side-applied by
the runner) or a `script:` (bash run with a curated env). All three
run **inside the deploy for a single matrix entry**, scoped to that
entry's namespace:

| Hook | When it runs | Declared at | Typical use |
|---|---|---|---|
| `pre-install` | After namespace + pull-secret creation, before `helm install` of the Camunda chart. | Scenario (`charts/<v>/test/ci/registry/scenarios/<name>.yaml`) | Provision external infra the chart depends on — a CNPG cluster, a TLS keystore, a JKS. |
| `post-infra` | After the runner has stood up companion Helm releases (Keycloak, ES, PG) and they're ready, but before the Camunda chart install. | Scenario | Bootstrap seed data into the companion infra (e.g. a Keycloak realm before the chart binds to it). |
| `post-deploy` | After the Camunda chart's `helm install` returns successfully. | Scenario | Register CRDs whose types the chart itself brings — the `gateway-keycloak` scenario uses this to apply a Gateway API `ProxySettingsPolicy`. |
| `pre-upgrade` | Between step 1 and step 2 of a two-step upgrade flow. | Flow (`charts/<v>/test/ci/registry/manifest.yaml` under `integration.flows.<flow>`) | Delete stateful resources that must be recreated on upgrade (e.g. StatefulSets + PVCs on major version bumps). |

Hooks are executed by
[`scripts/deploy-camunda/matrix/lifecycle_hook.go`](matrix/lifecycle_hook.go);
scripts receive `TEST_NAMESPACE`, `KUBE_CONTEXT`, and the same
env-var passthrough the runner uses elsewhere:
`RDBMS_POSTGRESQL_USERNAME`, `RDBMS_POSTGRESQL_PASSWORD`,
`GITHUB_WORKFLOW_JOB_ID`, `POSTGRESQL_JDBC_URL`.

### Wiring the Elastic (ECK) operator as a fixture

There is no ECK-backed persistence scenario shipped today (all 8.10
Elasticsearch scenarios use the `elastic/elasticsearch` Helm chart).
The path to add one:

1. Install the ECK operator + CRDs once, cluster-wide. This is a
   prerequisite; deploy-camunda does not manage the operator itself.
   Follow Elastic's "Cloud on Kubernetes → Install" documentation.
2. Add a fixture manifest under
   `charts/<v>/test/integration/scenarios/common/resources/` that
   declares the `Elasticsearch` custom resource your scenario needs.
   Use `$NAMESPACE` / `$RELEASE_NAME` for substitution:

   ```yaml
   apiVersion: elasticsearch.k8s.elastic.co/v1
   kind: Elasticsearch
   metadata:
     name: $RELEASE_NAME
     namespace: $NAMESPACE
   spec:
     version: 8.14.0
     nodeSets:
       - name: default
         count: 1
         config:
           node.store.allow_mmap: false
   ```
3. Reference it from a new scenario file with a `pre-install` hook:

   ```yaml
   name: elasticsearch-eck
   auth: keycloak
   flows: [install]
   identity: keycloak
   persistence: elasticsearch-external
   platforms: [gke]
   infra-type:
     gke: distroci
   dependencies:
     - keycloak
     - postgresql
   pre-install:
     fixtures:
       - eck-elasticsearch.yaml
     description: |
       Provisions an ECK-managed Elasticsearch cluster in the scenario
       namespace before helm-installing the Camunda chart. Requires the
       ECK operator's CRDs to be installed cluster-wide already.
   ```
4. Point the Camunda chart at the ECK service via a companion values
   file overlay (`persistence: elasticsearch-external`).

For a full worked example of a load-tested ECK setup, see
[camunda/camunda#57203](https://github.com/camunda/camunda/pull/57203)
(Jonathan Ballet — load-test docs including ES operator usage).

### Testing an operator-scenario locally

Because operators provision resources async, the pre-install hook is
just fire-and-forget from the runner's perspective. Add a readiness
gate to the manifest (e.g. an `Elasticsearch.status.health: green`
polling loop) if your Camunda chart install fails against a not-yet-ready
ECK cluster. The runner logs from `lifecycle_hook.go` show applied
resources under the `hook:` log prefix.

## Registry & credentials

Two registries commonly show up on a Camunda deploy: **Docker Hub**
(for community images pulled by `docker.io`) and **Harbor / Nexus /
Camunda Harbor** (for internal images pulled by `registry.camunda.cloud`).

### Docker Hub

Anonymous rate limits will bite in shared clusters. Supply credentials
one of these ways (first non-empty wins):

- `--dockerhub-username` / `--dockerhub-password` CLI flags
- `DOCKERHUB_USERNAME` / `DOCKERHUB_PASSWORD` env
- `TEST_DOCKER_USERNAME` / `TEST_DOCKER_PASSWORD` env (CI legacy names)

Set `--ensure-docker-hub` (or `ensureDockerHub: true` in the config file)
so the runner creates a pull secret in the target namespace before
pods try to pull.

### Harbor / Camunda Cloud

For images from `registry.camunda.cloud` (Harbor):

- `--docker-username` / `--docker-password` CLI flags
- `HARBOR_USERNAME` / `HARBOR_PASSWORD` env
- `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD` / `TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD` env
- `NEXUS_USERNAME` / `NEXUS_PASSWORD` env

Set `--ensure-docker-registry` (or `ensureDockerRegistry: true`) to
create the Harbor pull secret. `deploy-camunda doctor` reports both
registries' credentials as ✓/✗ and only fails when the matching
`--ensure-docker-*` flag makes the pull secret mandatory.

### Persisting credentials

Never commit credentials to `.deploy-camunda.yaml`. Persist them in
`.env` (which `deploy-camunda config init` writes for you) or set them
via your shell's env-manager. `deploy-camunda config env --show-origin`
prints which layer each variable resolved from — process env vs `.env`
vs per-entry override.

## Environment & secret model

For configuration fields (chart, namespace, platform, kube-context, …):

```
CLI flags  >  active deployment in config file  >  root config  >  built-in defaults
```

The config file is resolved in this order (first that exists wins):

1. `--config <path>` if given
2. `.deploy-camunda.yaml` in the current directory
3. `.deploy-camunda.yaml` in the repo root
4. `~/.config/camunda/deploy.yaml`

`CAMUNDA_*` environment variables override config-file values (e.g.
`CAMUNDA_PLATFORM`, `CAMUNDA_LOG_LEVEL`).

For **environment variables** substituted into scenario values files
(`$VAR` / `${VAR}` placeholders) and packed into secrets:

```
process environment  <  .env file  <  per-entry overrides (ExtraEnv)
```

Later layers win. `deploy-camunda config env --show-origin` prints
exactly which layer each variable resolved from. The `.env` file is
read without mutating your shell, and `config init` /
`--interactive` / `--auto-generate-secrets` write back to it
format-preservingly (comments and ordering are kept).

> Never commit `.env`. Generate it on demand (`config init`,
> `--auto-generate-secrets`, or `scripts/render-e2e-env.sh`).

### Vault secret mapping

`--vault-secret-mapping` (or the `vaultSecretMapping` config field)
lists which env vars get packed into a Kubernetes `Secret`. Format:

```
vault/path VAR1,VAR2 | ALIAS1,ALIAS2;
vault/path VAR3;
```

- Unset vars are **omitted with a warning** (the Secret simply lacks
  those keys). Pass `--strict-secrets` to fail instead — useful in CI
  so an incomplete Secret never ships and surfaces later as a
  crashing pod.
- `deploy-camunda doctor` lists any mapping vars that are currently unset.

The mapping used by `--auto-generate-secrets` / `config init` is data,
not code — edit
[`deploy/data/test-secret-mapping.yaml`](deploy/data/test-secret-mapping.yaml)
to change which variables are scaffolded, no Go changes required.

### Companion-chart env vars

Some scenarios deploy companion charts whose values files require
their own env vars — `RDBMS_POSTGRESQL_USERNAME` /
`RDBMS_POSTGRESQL_PASSWORD` for the `elasticsearch`/`rdbms` family, for
example. The preflight validates these too (the "companion env vars"
check), so a missing `RDBMS_POSTGRESQL_*` is reported up front rather
than failing partway through value preparation. `config init` offers
to scaffold local dev credentials, and `doctor --fix` prompts for any
that are still missing.

### Deploy-computed variables

Some `$PLACEHOLDER`s in scenario values are filled in by the deploy
machinery itself, not by you: `CAMUNDA_HOSTNAME`, `KEYCLOAK_REALM`,
the `*_INDEX_PREFIX` vars, `FLOW`, `KEYCLOAK_EXT_*`, `VENOM_CLIENT_ID`,
`CONNECTORS_CLIENT_ID`. The preflight evaluates placeholders against
the same environment the deploy will build (`buildScenarioEnv`), so
these are correctly recognised as satisfied and never flagged — only
variables you actually need to provide are reported.

### Layered values are scanned in full

A scenario's values are composed from layers — `base` + `identity/<id>`
+ `persistence/<backend>` + `platform/<p>` + `features/<f>`. The
preflight resolves the same layered set the deploy composes and scans
every layer for placeholders, so a `$VAR` introduced by a persistence
or identity layer (e.g. `$VENOM_CLIENT_ID` in
`values/persistence/elasticsearch.yaml`) is caught up front rather
than surfacing mid-deploy.

## CI vs local usage

Some files in the repo look tempting to edit but are consumed by CI
and should be left alone by external users. The three that trip people
up:

| File | Audience | External users |
|---|---|---|
| `.deploy-camunda.yaml.template` (repo root) | Advanced reference — every field, plus matrix config. | Read for reference; copy the [`getting-started`](examples/getting-started.deploy-camunda.yaml) starter instead. |
| `charts/chart-versions.yaml` | CI/release tooling + Renovate. | Read via `deploy-camunda matrix list`; never edit. |
| `charts/<v>/test/ci-test-config.yaml` | GitHub Actions unit-test workflow. | Never edit. |

Everything under `scripts/deploy-camunda/examples/` is for external
users to copy. Everything under `charts/<v>/test/ci/registry/` is
part of the scenario catalog — external users add new scenarios there
by PR, but should not edit existing entries without owning the change.

## Troubleshooting

The three issues that come up most often on first contact:

### `docker.io` anonymous rate limit / `TooManyRequests`

Symptom: pods stuck in `ImagePullBackOff` with
`toomanyrequests: You have reached your unauthenticated pull rate limit`.

Fix: pass Docker Hub credentials **and** `--ensure-docker-hub` so the
runner creates a pull secret in the target namespace:

```bash
DOCKERHUB_USERNAME=... DOCKERHUB_PASSWORD=... \
deploy-camunda matrix run \
  --repo-root . --versions 8.10 --shortname-filter keycloak-original \
  --ingress-base-domain <zone> --ensure-docker-hub
```

Or persist the credentials in `.env` (via `deploy-camunda config init`)
and put `ensureDockerHub: true` in the deployment profile in
`.deploy-camunda.yaml`.

### Ingress domain not found / `CAMUNDA_HOSTNAME` empty

Symptom: preflight reports `CAMUNDA_HOSTNAME` unresolved, or Keycloak
issues callbacks to `null` / `localhost`.

Fix: `--ingress-base-domain` (or `ingressBaseDomain:` in the config
file) is required. deploy-camunda computes
`CAMUNDA_HOSTNAME = <namespace>.<ingress-base-domain>` (or
`<ingress-subdomain>.<base>` when both are set) and feeds it into
scenario values, Keycloak issuer URLs, and the ingress objects
themselves. Set it to the DNS zone your cluster's ingress controller
serves — for example `ci.distro.ultrawombat.com` in Camunda CI or
`apps.mycompany.example` in your own cluster.

### Operator CRDs missing

Symptom: `no matches for kind "Elasticsearch" in version
"elasticsearch.k8s.elastic.co/v1"` (or similar) when the deploy
applies a pre-install fixture.

Fix: install the operator + CRDs cluster-wide **before** running
`deploy-camunda`. Operators are a
[prerequisite](#prerequisites), not something deploy-camunda
provisions for you. See [Using operators](#using-operators) for how
to wire an operator-provisioned dependency into a scenario.

### General diagnostics

Two commands cover most first-pass debugging:

- `deploy-camunda doctor` — read-only preflight checklist: config,
  kube-context reachability, docker creds, vault-mapping vars,
  scenario `$PLACEHOLDER`s, companion vars. `--fix` prompts for
  missing vars and writes them to `.env`. Exits non-zero on any ✗.
- `deploy-camunda config env --show-origin` — effective env table
  with source per key; secrets masked unless `--unmask`.

For a full command reference and operational patterns, see
[`../../SKILLS.md`](../../SKILLS.md).

## Command reference

| Command | Purpose |
| --- | --- |
| `deploy-camunda` | Deploy a single scenario using the active profile in `.deploy-camunda.yaml` (or CLI flags). |
| `deploy-camunda matrix list` | Preview the matrix of `(version, scenario, flow)` combinations without deploying. |
| `deploy-camunda matrix run` | Deploy every entry the matrix would generate (filter with `--versions`, `--shortname-filter`, `--flow-filter`). |
| `deploy-camunda config init [--from-example <name>]` | Interactive first-run setup, or drop an embedded starter file into place. |
| `deploy-camunda config init --list-examples` | List the embedded starter templates. |
| `deploy-camunda doctor [--fix]` | Preflight checklist. |
| `deploy-camunda config env [--show-origin] [--unmask]` | Show effective env variables with provenance. |
| `deploy-camunda config set/get/list/use/create/show` | Manage deployment profiles. |
| `deploy-camunda watch --namespace <ns>` | Poll a running deploy and diagnose CrashLoopBackOff / ImagePullBackOff live. |

# deploy-camunda — secrets & environment model

`deploy-camunda` deploys Camunda 8 Self-Managed scenarios and runs the CI matrix.
This document covers the part that trips people up most: **where secrets and
environment variables come from, and how to make sure they're set before you
deploy.** For the full command reference and operational patterns, see
[`../../SKILLS.md`](../../SKILLS.md).

## TL;DR — first run

```bash
deploy-camunda config init      # interactive: scaffolds .camunda-deploy.yaml + .env, ends with a checklist
deploy-camunda doctor           # re-run the checklist any time
deploy-camunda config env       # show the effective env and where each value came from (secrets masked)
```

A normal `deploy-camunda` run now runs the same preflight automatically and
**fails fast** if a required input is missing — before creating a namespace or
calling helm — so you no longer wait for an `ImagePullBackOff` to discover a
missing credential. Pass `--skip-preflight` to bypass, or `--interactive`
(default) to be prompted for missing scenario variables instead.

## Where values come from (precedence)

For configuration fields (chart, namespace, platform, kube-context, …):

```
CLI flags  >  active deployment in config file  >  root config  >  built-in defaults
```

The config file is resolved in this order (first that exists wins):

1. `--config <path>` if given
2. `.camunda-deploy.yaml` in the current directory
3. `.camunda-deploy.yaml` in the repo root
4. `~/.config/camunda/deploy.yaml`

`CAMUNDA_*` environment variables override config-file values (e.g.
`CAMUNDA_PLATFORM`, `CAMUNDA_LOG_LEVEL`).

For **environment variables** substituted into scenario values files
(`$VAR` / `${VAR}` placeholders) and packed into secrets:

```
process environment  <  .env file  <  per-entry overrides (ExtraEnv)
```

Later layers win. `deploy-camunda config env --show-origin` prints exactly which
layer each variable resolved from. The `.env` file is read without mutating your
shell, and `config init` / `--interactive` / `--auto-generate-secrets` write back
to it format-preservingly (comments and ordering are kept).

> Never commit `.env`. Generate it on demand (`config init`, `--auto-generate-secrets`,
> or `scripts/render-e2e-env.sh`).

## Secrets

### Docker registry credentials

Required when pods pull images (`--ensure-docker-registry`, and
`--ensure-docker-hub` for Docker Hub). Resolution order:

- **Harbor:** `--docker-username/--docker-password`, then `HARBOR_USERNAME`/`HARBOR_PASSWORD`,
  then `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD`/`TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD`,
  then `NEXUS_USERNAME`/`NEXUS_PASSWORD`.
- **Docker Hub:** `--dockerhub-username/--dockerhub-password`, then
  `DOCKERHUB_USERNAME`/`DOCKERHUB_PASSWORD`, then `TEST_DOCKER_USERNAME`/`TEST_DOCKER_PASSWORD`.

`deploy-camunda doctor` reports these as ✓/✗ and only fails when the matching
`--ensure-docker-*` flag makes the pull secret mandatory.

### Vault secret mapping

`--vault-secret-mapping` (or the `vaultSecretMapping` config field) lists which
env vars get packed into a Kubernetes `Secret`. Format:

```
vault/path VAR1,VAR2 | ALIAS1,ALIAS2;
vault/path VAR3;
```

- Unset vars are **omitted with a warning** (the Secret simply lacks those keys).
  Pass `--strict-secrets` to fail instead — useful in CI so an incomplete Secret
  never ships and surfaces later as a crashing pod.
- `deploy-camunda doctor` lists any mapping vars that are currently unset.

The mapping used by `--auto-generate-secrets` / `config init` is data, not code:
edit [`deploy/data/test-secret-mapping.yaml`](deploy/data/test-secret-mapping.yaml)
to change which variables are scaffolded — no need to touch Go.

## Commands for setup & inspection

| Command | Purpose |
| --- | --- |
| `config init` | Interactive first-run setup (config profile, docker creds, test secrets, **Postgres/RDBMS dev creds**) ending in a checklist. `--non-interactive` just verifies + prints. |
| `doctor [--fix]` | Read-only preflight checklist: config, kube-context reachability, docker creds, vault-mapping vars, scenario `$PLACEHOLDER`s, companion vars. `--fix` prompts for missing vars and writes them to `.env`. Exits non-zero on any ✗. |
| `config env [--show-origin] [--unmask]` | Effective env table with source per key; secrets masked unless `--unmask`. |
| `config set/get/list/use/create` | Manage deployment profiles in the config file. |

## Companion charts (matrix scenarios)

Some scenarios deploy companion charts (e.g. an external PostgreSQL) whose values
files require their own env vars — `RDBMS_POSTGRESQL_USERNAME` /
`RDBMS_POSTGRESQL_PASSWORD` for the `elasticsearch`/`rdbms` family, for example.
These are declared per scenario as `dependencies` in `ci-test-config.yaml` (each
with an `envVars` allowlist) and are substituted into the companion values file
at deploy time. The preflight validates them too (the "companion env vars"
check), so a missing `RDBMS_POSTGRESQL_*` is reported up front rather than
failing partway through value preparation. `config init` offers to scaffold
local `RDBMS_POSTGRESQL_USERNAME`/`RDBMS_POSTGRESQL_PASSWORD` dev credentials, and
`doctor --fix` will prompt for any that are still missing.

### Deploy-computed variables are not "missing"

Some `$PLACEHOLDER`s in scenario values are filled in by the deploy machinery
itself, not by you: `CAMUNDA_HOSTNAME` (from the ingress subdomain + base
domain), `KEYCLOAK_REALM`, the `*_INDEX_PREFIX` vars, `FLOW`, and
`KEYCLOAK_EXT_*`. The preflight evaluates placeholders against the **same
environment the deploy will build** (`buildScenarioEnv`), so these are correctly
recognized as satisfied and never flagged — only variables you actually need to
provide are reported.

`VENOM_CLIENT_ID` and `CONNECTORS_CLIENT_ID` are also deploy-supplied: they
default to the Keycloak mapping-rule client IDs `venom` / `connectors`, mirroring
the workflow env defaults in
[`test-integration-runner.yaml`](../../.github/workflows/test-integration-runner.yaml)
so local keycloak deploys match CI. OIDC/Entra entries override them with the
provisioned app GUIDs (via per-entry `ExtraEnv`), and your process env / `.env`
override the defaults too. Without this, keycloak + Elasticsearch scenarios would
fail locally on the `$VENOM_CLIENT_ID` / `$CONNECTORS_CLIENT_ID` placeholders that
live in the persistence/identity values layers.

### Layered values are scanned in full

A scenario's values are composed from layers — `base` + `identity/<id>` +
`persistence/<backend>` + `platform/<p>` + `features/<f>`. The preflight resolves
the **same layered set** the deploy composes (via the scenario's deployment
config) and scans every layer for placeholders, not just the top-level scenario
file. A `$VAR` introduced by a persistence or identity layer (e.g.
`$VENOM_CLIENT_ID` in `values/persistence/elasticsearch.yaml`) is therefore caught
up front rather than surfacing mid-deploy as a `failed to process layered values
file` error.

## Matrix runs

Each matrix entry runs the fail-fast preflight at the start of its deploy (the
runner's own pre-dispatch docker login and kube-context warmup still happen; the
preflight's cluster reachability probe is skipped to avoid duplicate network
calls). A missing scenario or companion variable fails that entry immediately
with the full list. Set the required vars in your `.env` (or environment) — see
the companion-chart vars above for `rdbms`/`elasticsearch` scenarios.

`matrix run` inherits the **active `config init` deployment profile** for its
shared infra fields — `ingressBaseDomain`, `kubeContext`, `platform`, `repoRoot`,
`envFile`. Precedence is: CLI flag > `matrix:` config block > root config >
active deployment profile. So once `config init` has set your base domain and
context, you don't need to repeat `--ingress-base-domain-gke` / `--kube-context`
on every matrix run. (Without a base domain the deploy can't compute
`CAMUNDA_HOSTNAME`, which is why an unconfigured matrix run flagged it.)

**Preview before deploying:** `matrix run --dry-run` now prints a per-entry
preflight checklist (secrets/env/companion vars), so you can confirm an entry
will pass — including computed vars like `CAMUNDA_HOSTNAME` — without touching a
cluster.

# Companion values

Values files for the "companion" charts (PostgreSQL, Keycloak, Elasticsearch,
OpenSearch, OpenLDAP) that CI scenarios stand up alongside the Camunda
Platform chart. Every file here is referenced by explicit path from a scenario
under `charts/<version>/test/ci/registry/dependencies/`; renaming or merging
them will break the matrix runner.

## Audience

These files are **deploy-camunda internals**, not templates for users to copy.
`deploy-camunda` selects the right file based on your `identity` and
`persistence` selection flags — do not add them to your `.deploy-camunda.yaml`
or `extraValues:` list by hand. If you're looking for a starting configuration,
see the [Getting started](#first-time-users) section below.

| Bucket | Files | Notes |
|---|---|---|
| **Base scenario dependencies** | `postgresql.yaml`, `elasticsearch.yaml`, `opensearch.yaml` | Minimal single-node setups with security relaxed for development. Placeholders like `$RDBMS_POSTGRESQL_USERNAME` are resolved from your `.env` or process env at deploy time. `config init` scaffolds those local dev credentials. |
| **CI-tuned (require CI-provisioned secrets)** | `keycloak.yaml` | Consumes `existingSecret: integration-test-credentials`, a Kubernetes Secret that the CI matrix runner creates in each entry's namespace before helm-installing. `deploy-camunda config init` does NOT create this Secret — it only writes env vars into `.env`. Copying `keycloak.yaml` in isolation will result in a missing-secret pod error. |
| **CI variants — QA node-pool targeting** | `elasticsearch-qa.yaml`, `keycloak-qa.yaml`, `postgresql-qa.yaml` | Add `nodeSelector` / `tolerations` targeting the `qa-workloads` node pool. Used only by QA-flagged scenarios. |
| **CI variants — TLS / RDBMS / regression** | `opensearch-tls.yaml`, `postgresql-tls.yaml`, `postgresql-rdbms.yaml`, `openldap.yaml` | Load-bearing per-scenario overrides (TLS trust chains, secondary databases, LDAP regression fixtures). Changing any of these directly alters CI behaviour for the scenario that references them. |

## Wiring

Each companion values file is opted into by a scenario via the registry
entries at `charts/<version>/test/ci/registry/dependencies/<name>.yaml`, and
that registry entry declares which env vars the file needs (the `envVars`
allowlist). `deploy-camunda`'s preflight validates those vars up front — see
[`scripts/deploy-camunda/README.md`](../../../scripts/deploy-camunda/README.md)
for the substitution model and the `companion env vars` check.

## First-time users

If you are looking for a starting configuration to deploy Camunda 8 to your
own cluster, don't edit the files in this directory. Copy the starter config
instead:

- [`scripts/deploy-camunda/examples/getting-started.deploy-camunda.yaml`](../../../scripts/deploy-camunda/examples/getting-started.deploy-camunda.yaml)

That starter drives `deploy-camunda`, which picks the appropriate companion
values file for you based on your `identity` and `persistence` selections.

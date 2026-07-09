# Companion values

Values files for the "companion" charts (PostgreSQL, Keycloak, Elasticsearch,
OpenSearch, OpenLDAP) that CI scenarios stand up alongside the Camunda
Platform chart. Every file here is referenced by explicit path from a scenario
under `charts/<version>/test/ci/registry/dependencies/`; renaming or merging
them will break the matrix runner.

## Audience

| Bucket | Files | Notes |
|---|---|---|
| **Sensible defaults — safe to use as a starting point for local dev** | `postgresql.yaml`, `keycloak.yaml`, `elasticsearch.yaml`, `opensearch.yaml` | Minimal single-node setups with security relaxed for development. Placeholders like `$RDBMS_POSTGRESQL_USERNAME` are resolved from your `.env` or process env at deploy time. |
| **CI variants — do not edit unless changing that specific scenario** | `elasticsearch-qa.yaml`, `keycloak-qa.yaml`, `postgresql-qa.yaml` | Add `nodeSelector` / `tolerations` targeting the `qa-workloads` node pool. Used only by QA-flagged scenarios. |
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

- [`scripts/deploy-camunda/examples/getting-started.camunda-deploy.yaml`](../../../scripts/deploy-camunda/examples/getting-started.camunda-deploy.yaml)

That starter drives `deploy-camunda`, which picks the appropriate companion
values file for you based on your `identity` and `persistence` selections.

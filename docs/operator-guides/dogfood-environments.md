---
title: Operate the dogfood environments
---

# Operate the dogfood environments

The `dogfood` topology runs one long-lived Hub plane with three Orchestration
Clusters on GKE, for internal dogfooding of Hub, Physical Tenants, and
multi-tenancy — shapes that do not exist in SaaS.

It is a normal chart topology deployed by
[`.github/workflows/dogfood-environment.yaml`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/dogfood-environment.yaml),
not a CI test. The scenario is registered with `enabled: false`, so it never
joins the PR or merge-queue matrix; the workflow reaches it with
`--include-disabled`.

## Shape

| Release | Namespace | What it runs |
|---|---|---|
| hub | `<env>-hub` | Management Identity, Camunda Hub, Web Modeler, Keycloak, the shared Elasticsearch |
| orchestration `plain` | `<env>-plain` | No Physical Tenants, no multi-tenancy |
| orchestration `mt` | `<env>-mt` | No Physical Tenants, logical multi-tenancy enabled |
| orchestration `pt` | `<env>-pt` | Physical Tenants `tenanta` and `tenantb` beside `default`; logical multi-tenancy inside `tenanta` only |
| optimize | `<env>-optplain` | Optimize for `plain` |
| optimize | `<env>-optmt` | Optimize for `mt` |
| optimize | `<env>-optptdef` | Optimize for the `pt` cluster's `default` tenant |
| optimize | `<env>-optptta` | Optimize for `tenanta` |
| optimize | `<env>-optpttb` | Optimize for `tenantb` |

`<env>` is the workflow's `environment` input, `dogfood` by default.

Every Optimize runs as its own release because one Optimize reads one tenant's
index prefix. All five are served on the Hub host under their own context path
(`/optimize-plain`, `/optimize-pt-tenanta`, …); the Orchestration Clusters are
served on their own hosts. All three clusters share the Hub namespace's
Elasticsearch, separated by index prefix.

## Run the workflow

Actions/Dogfood - Environment → Run workflow.

| Action | Effect |
|---|---|
| `deploy` | Installs, or upgrades in place if the environment already exists |
| `upgrade` | Upgrades in place; fails when the environment is not deployed |
| `status` | Prints namespaces, pod readiness, and URLs; changes nothing |
| `uninstall` | Deletes every namespace; requires `confirm` to repeat the environment name |

A nightly schedule runs `upgrade` at 02:00 UTC so the environments track the
chart, and posts to the Distribution Slack channel when it fails.

`deploy` and `upgrade` are the same Helm call — the deploy path is
`helm upgrade --install` and every namespace, Keycloak realm, and index prefix
is derived from the stable base namespace, so re-running is an in-place upgrade
rather than a reinstall. The two actions differ only in the pre-flight: `upgrade`
refuses to create an environment that is not already there.

## Local equivalent

Requires a `gcloud` session for the CI GKE project and Harbor credentials in the
Docker keychain (see the `gke-verification` skill for the pre-flight).

```bash
deploy-camunda matrix run \
  --repo-root . \
  --include-disabled \
  --scenario-filter dogfood \
  --flow-filter install \
  --versions 8.10 \
  --platform gke \
  --namespace-override dogfood \
  --ingress-base-domain-gke ci.distro.ultrawombat.com \
  --timeout 25
```

Then exempt the namespaces from the cluster cleaner, which otherwise reaps them
within the hour:

```bash
deploy-camunda topology persist --version 8.10 --scenario dogfood --base dogfood
```

Other lifecycle subcommands work against any topology scenario, not just this one:

```bash
deploy-camunda topology namespaces --version 8.10 --scenario dogfood --base dogfood
deploy-camunda topology status     --version 8.10 --scenario dogfood --base dogfood \
  --ingress-base-domain ci.distro.ultrawombat.com
deploy-camunda topology uninstall  --version 8.10 --scenario dogfood --base dogfood \
  --confirm dogfood
```

## Change the shape

The topology lives in
`charts/camunda-platform-8.10/test/ci/registry/scenarios/dogfood.yaml`; each
release's values live in
`charts/camunda-platform-8.10/test/integration/scenarios/chart-full-setup/values/features/dogfood-*.yaml`.

To add an Orchestration Cluster: add an `orchestration` release with a unique
`namespace-suffix` (12 characters or fewer), `modeler-cluster-id`, and
`modeler-cluster-name`; add its `dogfood-orchestration-<id>.yaml` feature layer;
add a matching cluster record to `dogfood-hub.yaml`; and give it at least one
`optimize` release. Regenerate the registry snapshot with
`make go.update-registry-golden` and commit it.

To add a Physical Tenant to the `pt` cluster, follow the order in
[Run one Hub with multiple clusters and Optimize releases](hub-and-physical-tenant-optimize.md):
add its Hub record, add its tenant and exporter in
`dogfood-orchestration-pt.yaml`, then add its Optimize release. Removing a tenant
runs the reverse order, and Identity reconciliation is additive — its Keycloak
client, resource server, and role survive the removal and need explicit cleanup.

Resource-Based Access is not covered: it cannot be combined with multi-tenancy,
so it needs a fourth cluster of its own.

## Constraints to keep in mind

- **Index prefixes are the isolation boundary.** Every cluster and tenant needs a
  unique orchestration prefix, exporter writer prefix, and Optimize application
  prefix, and each Optimize's reader prefix must exactly equal the writer prefix
  of the tenant it serves. Authentication does not isolate shared storage.
- **The `pt` cluster pins its OIDC issuer.** Declaring Physical Tenants makes the
  cluster validate `iss` on every token, and the chart will not infer the value;
  the Hub's Keycloak is started with a fixed hostname so in-cluster callers and
  browsers present the same issuer.
- **Uninstall leaves data behind.** Helm uninstall does not drop Elasticsearch
  indices, and removing a cluster or tenant record does not delete its Identity
  or Keycloak objects. Clean both explicitly once no release reads them.

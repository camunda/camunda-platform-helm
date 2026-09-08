---
title: Run one Hub with multiple clusters and Optimize releases
---

# Run one Hub with multiple clusters and Optimize releases

This guide covers a fresh Camunda 8.10 deployment with:

- one Hub release
- any number of Orchestration Cluster releases
- one separate Optimize release for every tenant, including `default`

Use `global.topology.mode: hub` for the Hub, `orchestration` for each cluster, and
`optimize` for each Optimize release. Keep release values in separate files so each
release has one owner and lifecycle.

## Map tenants explicitly

In the Hub release, map the default tenant through the cluster-level
`components.optimize` record. Map each non-default Physical Tenant through a
`physicalTenants` entry. The tenant ID must exactly match the ID under
`camunda.physical-tenants` in the Orchestration Cluster configuration.

```yaml
global:
  topology:
    mode: hub
    clusters:
      - id: production-a
        # namespace, releaseName, host, version, contextPaths, and other components omitted
        components:
          optimize: # Optimize for the default tenant
            enabled: true
            clientId: optimize-production-a-default
            audience: optimize-production-a-default-api
            roleName: Optimize production-a default
            redirectUrl: https://production-a.example.com/optimize-default
            secret:
              existingSecret: optimize-production-a-default-oidc
              existingSecretKey: client-secret
        physicalTenants:
          - id: tenanta
            components:
              optimize:
                enabled: true
                clientId: optimize-production-a-tenanta
                audience: optimize-production-a-tenanta-api
                roleName: Optimize production-a tenanta
                redirectUrl: https://production-a.example.com/optimize-tenanta
                secret:
                  existingSecret: optimize-production-a-tenanta-oidc
                  existingSecretKey: client-secret
```

Give every Optimize release its own OIDC client ID, audience, role name, redirect
URL, and secret. Set the same client ID, audience, redirect URL, and secret on that
tenant's Optimize release under `optimize.security.authentication.oidc`. Setting a
dedicated `roleName` avoids adding the audience to the shared `Optimize` role.

## Isolate all four index prefix families

Authentication does not isolate shared Elasticsearch or OpenSearch storage. Assign
unique prefixes for every cluster and tenant.

| Prefix family | Configuration | Requirement |
|---|---|---|
| Orchestration application indices | Default: `orchestration.index.prefix`; Physical Tenant: `camunda.physical-tenants.<id>.data.secondary-storage.<backend>.index-prefix` | Unique per cluster and tenant |
| Legacy exporter writer | Default: `orchestration.exporters.zeebe.index.prefix` and an explicit `default` exporter in `camunda.physical-tenants`; Physical Tenant: its exporter `args.index.prefix` | Unique per cluster and tenant |
| Optimize reader | `optimize.database.elasticsearch.prefix` or `optimize.database.opensearch.prefix` | Must exactly equal that tenant's legacy exporter writer prefix |
| Optimize application indices | `CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX` or `CAMUNDA_OPTIMIZE_OPENSEARCH_SETTINGS_INDEX_PREFIX` in `optimize.env` | Unique per Optimize release and different from all writer prefixes |

Declaring any Physical Tenant stops `default` from inheriting the root exporters.
Define an exporter for `default` explicitly, as well as one for every Physical
Tenant. A writer/reader mismatch starts Optimize against the wrong or empty record
set; similar-looking prefixes are not sufficient.

The tenant-specific part of an Optimize-only release has this shape. Provider-wide
OIDC values such as the issuer and JWKS URL are omitted. Use the OpenSearch
equivalents when applicable.

```yaml
global:
  topology:
    mode: optimize
  noSecondaryStorage: false
  identity:
    service:
      url: http://identity.hub.svc:80/identity

optimize:
  enabled: true
  contextPath: /optimize-tenanta
  security:
    authentication:
      method: oidc
      oidc:
        clientId: optimize-production-a-tenanta
        audience: optimize-production-a-tenanta-api
        redirectUrl: https://production-a.example.com/optimize-tenanta
        secret:
          existingSecret: optimize-production-a-tenanta-oidc
          existingSecretKey: client-secret
  database:
    elasticsearch:
      enabled: true
      external: true
      url:
        host: elasticsearch.example.com
      prefix: production-a-tenanta-records # exact exporter writer prefix
  env:
    - name: CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX
      value: production-a-tenanta-optimize
```

## Operate releases in dependency order

Use these orders for both Helm and GitOps reconciliation:

| Operation | Order |
|---|---|
| Install | Hub, each Orchestration Cluster, then its default and Physical Tenant Optimize releases |
| Add a tenant | Add its Hub record, add its Orchestration tenant and exporter, then install its Optimize release |
| Disable or remove a tenant | Stop or uninstall its Optimize release, remove its Orchestration tenant and exporter, then disable or remove its Hub record |
| Re-enable a tenant | Restore its Hub record, restore its Orchestration tenant and exporter, then reinstall its Optimize release with the same prefixes |
| Upgrade | Hub first, then Orchestration Clusters, then their Optimize releases; complete and verify each layer before continuing |
| Uninstall the topology | All Optimize releases, all Orchestration Cluster releases, then the Hub release |

Identity and Keycloak initialization is additive. Removing or disabling a cluster or
tenant record does not delete its client, resource server, permissions, or role.
Inventory and clean those objects explicitly after dependent releases have stopped.
Do not delete an object still used by another release.

Helm uninstall also leaves Elasticsearch and OpenSearch indices intact. This permits
re-enabling with the same prefixes, but it can expose old data to a newly mapped
tenant if prefixes are reused. Apply the storage system's retention and deletion
process separately after confirming that no release reads or writes those indices.

## Scope and limits

- This procedure applies to fresh Camunda 8.10 topology deployments. It does not
  define migration from a combined production release to split releases.
- Hub topology connections and Physical Tenants require OIDC. This guide does not
  cover basic authentication.
- The chart does not configure cross-cluster DNS, routing, TLS trust, firewall rules,
  or external identity-provider objects. All configured URLs must be reachable from
  the release that uses them.
- One Optimize release serves one tenant. Sharing an Optimize release between the
  default tenant and Physical Tenants is outside this guide.
- Separate OIDC credentials do not isolate storage. Prefixes and backend access
  controls remain operator responsibilities.
- Supported cluster and tenant scale limits have not been established by this
  deployment contract.

See the [8.10 chart values](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.10/values.yaml)
for the complete topology, Physical Tenant, authentication, and backend
configuration.

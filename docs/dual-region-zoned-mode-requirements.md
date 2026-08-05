# 8.10 Multi-Region Zoned Mode Requirements

## Scope

These requirements apply to the Camunda Platform Helm chart 8.10 orchestration
component. The chart must support the existing legacy multi-region behavior and
the new application-supported zone-aware behavior.

## Modes

`global.multiregion.mode` selects the behavior:

- `legacy` is the default.
- `zoned` enables zone-aware clustering.
- Any mode other than `zoned` follows the legacy rendering path.

The default must preserve existing 8.10 users that do not configure the new
values.

## Zoned Values

The user configures the local zone and the complete cluster topology under the
global multi-region namespace:

```yaml
global:
  multiregion:
    mode: zoned
    zone: region-a
    zones:
      - name: region-a
        numberOfBrokers: 2
        numberOfReplicas: 2
        priority: 100
      - name: region-b
        numberOfBrokers: 3
        numberOfReplicas: 3
        priority: 50
```

The `zones` list may contain N zones. The expected common cases are one, two,
or three zones.

The zone fields map to the Camunda application properties as follows:

| Helm value | Application property |
| --- | --- |
| `name` | `name` |
| `numberOfBrokers` | `number-of-brokers` |
| `numberOfReplicas` | `number-of-replicas` |
| `priority` | `priority` |

## Zoned Rendering

When `mode: zoned`:

- `camunda.cluster.partitioning.scheme` renders as `ZONE_AWARE`.
- `camunda.cluster.partitioning.zone-aware.zones` renders the complete zone list.
- `camunda.cluster.size` is the sum of all `numberOfBrokers` values.
- `camunda.cluster.replication-factor` is the sum of all `numberOfReplicas` values.
- The StatefulSet replica count is the local zone's `numberOfBrokers` value.
- The pod receives `CAMUNDA_CLUSTER_ZONE` with the configured local zone.
- The chart does not render `camunda.cluster.node-id`.
- The chart does not compute or set the node ID. The Camunda application owns
  the zone-aware node ID computation.
- Initial contact points are not generated. Users provide them through the
  supported application environment variable mechanism when required.
- Zoned mode is treated as multi-region for the advertised broker hostname.
- Legacy Elasticsearch and OpenSearch exporter checks must not treat zoned mode
  as a single-region deployment.

## Legacy Compatibility

When `mode` is omitted or is `legacy`:

- `global.multiregion.regions` continues to control the number of regions.
- `global.multiregion.regionId` continues to control the legacy region ID.
- StatefulSet replicas remain `orchestration.clusterSize / regions`.
- The startup script computes the numeric node ID using the existing formula:
  `pod ordinal * regions + regionId`.
- `camunda.cluster.node-id` remains backed by
  `VALUES_ORCHESTRATION_NODE_ID`.
- Single-region initial contact points continue to be generated.
- Multi-region initial contact points continue to be left for the user to
  configure.
- Single-region and multi-region advertised broker hostnames remain unchanged.
- Legacy exporter behavior remains unchanged.
- `orchestration.configuration` remains a complete replacement for the
  generated application configuration.

The existing legacy values are not removed or renamed.

## Legacy Values Incompatibility

When `mode: zoned`, the legacy `regions` and `regionId` settings cannot be used.
The chart must fail when either value is configured with a non-default value:

- `regions` other than `1`
- `regionId` other than `0`

Helm values do not provide a reliable way to distinguish an explicitly supplied
default from the chart default. Therefore, the implementation rejects
non-default legacy values and leaves the default-valued fields inert in zoned
mode.

## Custom Application Configuration

`orchestration.configuration` remains authoritative and is not merged with the
chart-generated application configuration.

If a user supplies `orchestration.configuration`, the chart does not inject the
zone-aware `camunda.cluster.partitioning` settings into that content. Users are
expected to provide application properties through their custom configuration or
environment variables as appropriate.

The chart still injects `CAMUNDA_CLUSTER_ZONE` into the pod environment in zoned
mode because it is a per-deployment broker setting.

## Validation Scope

The Helm chart does not currently validate the zone topology. The application is
responsible for validating, among other things:

- zone names and duplicates
- local-zone membership
- broker and replica counts
- topology totals
- application-level zone constraints

Helm-level zone validation may be added later.

## Verification Matrix

The 8.10 unit tests must cover:

| Path | Required assertions |
| --- | --- |
| Default legacy mode | Numeric node ID, single-region advertised hostname, no zone env, no `ZONE_AWARE` config |
| Explicit legacy mode | Numeric node ID, multi-region advertised hostname, no zone env, no `ZONE_AWARE` config |
| Legacy StatefulSet scaling | Replica division for one and multiple regions |
| Legacy initial contacts | Generated for one region; omitted for multiple regions |
| Legacy custom configuration | Custom application configuration replaces generated application configuration |
| Zoned mode | Local replica count, computed cluster totals, zone env, `ZONE_AWARE` topology, no Helm node ID |
| Zoned contact points | Generated contact points remain disabled |
| Zoned exporter gating | Zoned mode does not activate legacy single-region exporter behavior |
| Zoned/legacy conflict | Non-default legacy region settings fail in zoned mode |

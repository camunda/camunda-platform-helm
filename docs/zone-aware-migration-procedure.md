# Migrate an existing Orchestration cluster to zone-aware brokers

This procedure migrates an existing numbered-broker Orchestration cluster to zone-aware broker identities on Camunda 8.10.

The migration replaces brokers instead of changing their persisted identities. Existing numbered brokers and replacement zoned brokers run together until partition distribution and cluster membership have been updated through the Orchestration management API.

Each Helm release manages one Kubernetes cluster and one local zone. Repeat the procedure one zone at a time for a multi-zone deployment.

## Prerequisites

- Confirm that the Orchestration image supports zone-aware clustering and the management API operations used below.
- Prepare the complete, identical `orchestration.multiregion.zones` topology for every participating release.
- Set `orchestration.multiregion.zone` to the local zone for the release being upgraded.
- Ensure the local Kubernetes cluster has enough capacity for both broker generations and their persistent volumes.
- Back up the Helm values and confirm that the existing numbered StatefulSet and its PVCs are healthy.
- For zones in different Kubernetes clusters, provide externally resolvable contact points through `CAMUNDA_CLUSTER_INITIALCONTACTPOINTS` instead of relying on the generated in-cluster DNS names:

  ```yaml
  orchestration:
    env:
      - name: CAMUNDA_CLUSTER_INITIALCONTACTPOINTS
        value: <fully-qualified-broker-addresses-for-all-zones>
  ```

## 1. Start the migration for one zone

Set zoned mode and retain the existing numbered brokers:

```yaml
orchestration:
  multiregion:
    mode: zoned
    zone: zone-a
    zones:
      - name: zone-a
        numberOfBrokers: 3
        numberOfReplicas: 3
        priority: 100
      - name: zone-b
        numberOfBrokers: 3
        numberOfReplicas: 3
        priority: 90
    keepUnzonedBrokers: true
```

Keep the previous `regions` and `regionId` values while the numbered StatefulSet is retained. Upgrade the existing release:

```bash
helm upgrade <release> camunda/camunda-platform \
  --namespace <namespace> \
  --values <values-file>
```

The release should now contain both the zone-suffixed StatefulSet and the retained numbered StatefulSet. The shared client-facing Services must select both broker generations, while the zone-specific StatefulSet and headless Service select only the local zoned brokers.

During this coexistence phase, the zoned configuration's `cluster.size` and `replication-factor` describe the zoned topology. The retained numbered brokers are temporary migration members and are not added to those derived values.

Verify that the existing numbered pods were not restarted and that the new zoned pods become ready before continuing.

## 2. Move the local zone through the management API

Use the Orchestration management API to move the local zone's partition distribution and broker membership to the zoned brokers:

- Update partition distribution with `PUT /actuator/cluster/partition-distribution`.
- Update zone membership with `PUT /actuator/cluster/zones`.

Use the API documentation and the response from the running Orchestration cluster to construct the request bodies. Do not remove the numbered brokers from Kubernetes until the zoned brokers are ready and the management API reports that the local partitions and broker membership have moved.

Repeat the management API operation until the local numbered brokers no longer own partitions and are no longer members of the logical cluster.

## 3. Remove the retained numbered resources

After the management API migration is complete, set:

```yaml
orchestration:
  multiregion:
    keepUnzonedBrokers: false
```

The old `regions` and `regionId` values are no longer used by the zoned resources and may be removed or reset in the same upgrade. Upgrade the release again:

```bash
helm upgrade <release> camunda/camunda-platform \
  --namespace <namespace> \
  --values <values-file>
```

This removes the retained numbered StatefulSet, ConfigMap, ServiceAccount, and PDB. The zoned StatefulSet pod template and configuration checksum must remain unchanged, so the zoned brokers must not restart solely because retention was disabled.

Do not delete the old PVCs yet. Confirm through the management API and the cluster state that every corresponding numbered broker has left the logical cluster, then remove the old PVCs manually according to the storage policy for the deployment.

## 4. Repeat for the next zone

For the next Kubernetes cluster:

1. Use the same complete zone topology.
2. Set `orchestration.multiregion.zone` to the next local zone.
3. Set `keepUnzonedBrokers: true` for that release.
4. Run the management API migration for the local zone.
5. Set `keepUnzonedBrokers: false` after the local numbered brokers leave membership.
6. Clean up the old PVCs only after the migration is verified.

Do not migrate multiple zones concurrently unless the deployment has an independently verified operational procedure for that topology.

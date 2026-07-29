# Automatic Capacity Manager

The capacity manager is an opt-in 8.10 component that coordinates physical orchestration capacity
with Camunda's dynamic cluster topology.

It supports three policy modes:

- `recommend` observes and reports a desired broker count without applying it.
- `scheduled` applies an explicit target or an RFC3339 capacity window.
- `automatic` uses a Prometheus pressure query with independent scale-up and scale-down stabilization.

## Safety Model

For scale-up, the manager starts the next StatefulSet pod before dry-running and applying a Camunda
cluster topology change. For scale-down, it evacuates the broker and waits for the topology change to
complete before reducing StatefulSet replicas.

The component does not change partition count. Size logical partitions for expected peak throughput;
the manager spreads those partitions across more brokers during peaks and consolidates them off peak.

Capacity management is disabled by default and currently supports fresh single-region deployments
with `orchestration.clusterSize: "1"`. Set `capacityManager.replicaOwnership: capacityManager` to
explicitly transfer StatefulSet replica ownership from Helm. Advisor-only mode leaves the value empty
and retains normal Helm ownership.

Returning ownership is an explicit handback using `capacityManager.replicaOwnership: helm`. It
requires live cluster access and succeeds only when the durable target, durable completion marker,
live replicas, and configured cluster size all match. Offline handback is rejected. Rolling back to a
chart version that predates this contract is unsupported until the cluster has been safely contracted.

## Operator Integration

The implementation under `scripts/capacity-manager` is an importable Go package. Kubernetes access,
Camunda topology access, policy loading, pressure measurement, and planning are interfaces. A future
operator can provide a CRD-backed policy source and status adapter while reusing the same planner and
reconciliation state machine.

## Validation

The disabled `capacity-manager` GKE scenario starts with two logical partitions on one broker and a
50 process-instances-per-second load generator. Automatic mode directly measures the broker's real
record-appended rate, scales to two brokers, then scales safely back to one after the verifier removes
the load generator.

The GKE validation used a 5-second reconciliation interval, three scale-up samples, and six
scale-down samples. A 50 process-instances-per-second noop workload exceeded the configured
20 records-per-second scale-up threshold. After removing load, measured background activity was
approximately 0.4-3.6 records per second, so the scenario uses a calibrated 5 records-per-second
scale-down threshold. The complete matrix scenario passed in 7m03s and retained the removed broker's
PVC.

## Partition Advisor

The recommendation-only partition advisor can be enabled independently of broker autoscaling. It
uses the runtime topology's unique partition count and sustained metric evidence to recommend a
small partition increase. Recommendations never call the partition-scaling API.

The GKE advisor scenario runs one broker and one partition under a 50 process-instances-per-second
noop workload. A per-partition appended-record rate above 20 records per second for three consecutive
samples produces an incremental recommendation. The verifier asserts that StatefulSet replicas,
active broker count, runtime partition count, pending topology change, and last topology change ID
remain unchanged. Advisor-only RBAC cannot patch StatefulSets.

The GKE matrix scenario passed in 2m40s. Under 50 process instances per second, the measured
partition record rate was approximately 258 records per second and produced a high-confidence
incremental recommendation from one to two partitions. Runtime topology remained at one broker and
one partition, no topology change was created, and Kubernetes authorization confirmed the advisor
service account cannot patch the orchestration StatefulSet.

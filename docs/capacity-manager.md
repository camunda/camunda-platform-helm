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
with `orchestration.clusterSize: "1"`. When enabled, Helm omits `spec.replicas`, giving the manager
exclusive ownership of runtime replicas across upgrades. Disable or rollback only after setting the
policy target to one and waiting for the manager to complete safe topology contraction.
The chart rejects a normal disable while the durable target is greater than one. Rolling back to a
chart version that predates capacity management cannot enforce this guard and is unsupported until
the cluster has been safely contracted.

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

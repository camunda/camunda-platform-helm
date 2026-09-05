# Camunda Capacity Manager

The capacity manager coordinates Kubernetes orchestration replicas with Camunda's dynamic cluster
topology. It never reduces the logical partition count.

## Modes

- `recommend` reports decisions without changing capacity.
- `scheduled` applies an explicit target or an active scheduled minimum.
- `automatic` evaluates a Prometheus pressure query with separate scale-up and scale-down
  stabilization windows.

The policy is read from `/etc/capacity-manager/policy.json` on every reconciliation. The Helm chart
renders this file from `capacityManager.policy`.

## Safety Order

Scale-up creates the next StatefulSet pod, waits for it to start, dry-runs the Camunda topology
change, and then adds the broker. Scale-down dry-runs and completes broker evacuation before reducing
the StatefulSet replica count.

The planner and reconciler depend on interfaces rather than Kubernetes resource types. An operator
can provide a CRD-backed `PolicySource` and use the same `Manager` without reimplementing scaling
logic.

The StatefulSet annotation `capacity-manager.camunda.io/target-brokers` persists the intended
capacity so a restarted manager can distinguish an unfinished join from a completed topology
contraction.

## Status

The process exposes:

- `/healthz` for health probes.
- `/status` for the current decision, phase, capacity and blocking reason.

## Local Validation

```bash
make test.capacity-manager
make build.capacity-manager
make build-image.capacity-manager
```

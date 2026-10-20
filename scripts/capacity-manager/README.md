# Camunda Capacity Manager

The capacity manager coordinates Kubernetes orchestration replicas with Camunda's dynamic cluster
topology. It never reduces the logical partition count.

## Modes

- `recommend` reports decisions without changing capacity.
- `scheduled` applies an explicit target or an active scheduled minimum.
- `automatic` scales on named Prometheus signals (see below).

`scheduledMinimumBrokers` requires both `scheduleStartsAt` and `scheduleEndsAt` (RFC 3339).

The policy is read from `/etc/capacity-manager/policy.json` on every reconciliation. The Helm chart
renders this file from `capacityManager.extraConfiguration` and defaults to a read-only `recommend`
policy.

## Automatic Scaling Signals

More brokers only help when the cluster is saturated, the brokers themselves are the bottleneck,
and there is room to spread work: partitions are fixed, so a broker beyond the total number of
partition replicas hosts nothing. Throughput metrics such as
`zeebe_log_appender_record_appended_total` or `zeebe_flow_control_partition_load` measure load,
not saturation, and a partition at its write limit needs more partitions, not more brokers.

`automatic` mode therefore requires a Prometheus server that scrapes every broker (`--prometheus-url`;
scraping a broker actuator directly is rejected) and evaluates three groups of signals:

- `scaleUpSignals`: every signal must hold to add a broker.
- `scaleDownSignals`: every signal must hold to remove a broker.
- `vetoSignals`: any signal that holds blocks every change.

Each signal has a `name`, a PromQL `query`, an `operator` (`>=` or `<=`) and a `threshold`. The
query result is reduced to its maximum series, so `>=` means "any series reached the threshold" and
`<=` means "every series is at or below it". `NaN` samples are ignored. The placeholders
`$NAMESPACE`, `$STATEFULSET` and `$BROKERS` (current active brokers) are substituted before the
query runs.

A signal without data, a failed query, or a veto without data makes the planner hold and restart
both stabilization streaks. Write vetoes so they return a value when healthy, for example with
`or vector(0)`. Independently of signals, the planner never exceeds `maxBrokers` or the topology's
partition replica count, never scales below `minBrokers` or an active scheduled minimum, waits
`cooldown` after each change, and requires `scaleUpSamples`/`scaleDownSamples` consecutive
evaluations held for `scaleUpStabilization`/`scaleDownStabilization` (Go durations).

Recommended signals (selector `{namespace="$NAMESPACE",pod=~"$STATEFULSET-[0-9]+"}`
abbreviated as `{S}`):

| Role | Name | Query | Condition | Why |
|---|---|---|---|---|
| up | `backpressure` | `max(sum by (partition) (rate(zeebe_dropped_request_count_total{S}[2m])) / clamp_min(sum by (partition) (rate(zeebe_received_request_count_total{S}[2m])), 1))` | `>= 0.05` | The request limiter (on by default) is rejecting user commands. |
| up | `broker-cpu` | `max(avg_over_time(process_cpu_usage{S}[2m]))` | `>= 0.75` | The brokers themselves are busy. `process_cpu_usage` is relative to the CPUs available to the JVM, which follows the container CPU limit rounded up to whole CPUs. |
| down | `no-backpressure` | same as `backpressure` | `<= 0.001` | Nothing is being rejected. |
| down | `fits-on-fewer-brokers` | `sum(avg_over_time(process_cpu_usage{S}[5m])) / ($BROKERS - 1)` | `<= 0.5` | The current work fits on one broker fewer with headroom. |
| veto | `exporter-backlog` | `max(max by (partition) (zeebe_log_appender_last_committed_position{S}) - min by (partition) (max by (partition, exporter) (zeebe_exporter_last_exported_position{S}))) or vector(0)` | `>= 100000` | Exporters (and secondary storage) are behind; brokers are not the bottleneck and moving partitions adds load. |
| veto | `partition-unhealthy` | `min(zeebe_health{S})` | `<= 0` | A partition is unhealthy or dead. |

Exporter or secondary-storage lag does not raise backpressure unless write-rate throttling is
enabled, which is why it is a separate veto. Optional vetoes worth adding when the metrics exist:
`min(zeebe_cluster_partition_balanced{S}) <= 0` (leadership is off its preferred broker; rebalance
before adding capacity) and `max(kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes)`
for broker volumes (disk near the watermark). The status endpoint reports every reading and the
signals behind each decision; `/metrics` exports them as `camunda_capacity_manager_signal`.

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
- `/status` for the current decision, phase, capacity, signal readings, blocking reason, and
  `lastChange` (time, broker counts, reason and signal readings of the last decision that started a
  change).
- `/metrics` for current/desired brokers, replicas, phase and signal readings.

## Local Validation

```bash
make test.capacity-manager
make build.capacity-manager
make build-image.capacity-manager
```

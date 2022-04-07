# Migration

  * [Migrating from zeebe-full-helm](#migrating-from-zeebe-full-helm)
  * [Migrating from zeebe-cluster-helm](#migrating-from-zeebe-cluster-helm)

## Migrating from zeebe-full-helm

If you're running an earlier `zeebe-full-helm` chart release, and you want to migrate to `camunda-platform` you need to
adjust your `values.yaml` file, since the dependencies (sub-charts) have been renamed.

| Old Chart Names | New Sub-Chart Names |
|-----------|-----------|
| zeebe-cluster-helm | zeebe |
| zeebe-operate-helm | operate |
| zeebe-tasklist-helm | tasklist |

Some properties have been renamed or removed, but also better documented. Please check the [README](https://github.com/camunda-community-hub/camunda-platform-helm/blob/main/charts/camunda-platform/README.md) for more details.

**Example:**

_Old:_
```
global:
  zeebe: "{{ .Release.Name }}-zeebe"

tasklist:
  enabled: false

zeebe-cluster-helm: {}

zeebe-operate-helm: {}

zeebe-tasklist-helm: {}
```
_New:_
```
global:
  zeebeClusterName: "{{ .Release.Name }}-zeebe"

zeebe: {}

operate: {}

tasklist: {}
```

## Migrating from zeebe-cluster-helm

Be aware there is no longer a zeebe only helm chart. If you want to migrate to `camunda-platform` you have to move most of the
properties under the `zeebe` object.

> **Note:** There is also a new sub-chart for the standalone gateway. This means some configurations need to be done now
> on the gateway chart.

For more details please check the [README](https://github.com/camunda-community-hub/camunda-platform-helm/blob/main/charts/camunda-platform/README.md),
which documents all possible values for each chart.

#### Example:

**_Old:_**

```yaml
image:
  repository: camunda/zeebe
  tag: SNAPSHOT
  pullPolicy: Always

clusterSize: 3
partitionCount: 3
replicationFactor: 3
cpuThreadCount: 4
ioThreadCount: 4

gateway:
  replicas: 1
  logLevel: debug
  env:
    - name: ZEEBE_LOG_APPENDER
      value: Stackdriver
    - name: ZEEBE_LOG_STACKDRIVER_SERVICENAME
      value: zeebe-gateway
    - name: ZEEBE_LOG_STACKDRIVER_SERVICEVERSION
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: ATOMIX_LOG_LEVEL
      value: INFO
    - name: ZEEBE_GATEWAY_MONITORING_ENABLED
      value: "true"
    - name: ZEEBE_GATEWAY_THREADS_MANAGEMENTTHREADS
      value: "4"

JavaOpts: >-
  -XX:MaxRAMPercentage=25.0
  -XX:+ExitOnOutOfMemoryError
  -XX:+HeapDumpOnOutOfMemoryError
  -XX:HeapDumpPath=/usr/local/zeebe/data
  -XX:ErrorFile=/usr/local/zeebe/data/zeebe_error%p.log
  -Xlog:gc*:file=/usr/local/zeebe/data/gc.log:time:filecount=7,filesize=8M

# Environment variables
env:
  - name: ZEEBE_BROKER_EXECUTION_METRICS_EXPORTER_ENABLED
    value: "true"
  - name: ATOMIX_LOG_LEVEL
    value: INFO

# RESOURCES
resources:
  limits:
    cpu: 5
    memory: 12Gi
  requests:
    cpu: 5
    memory: 12Gi
```

**_New:_**

> **Be aware** you need to disable tasklist and operate, since they are enabled per default.

As described earlier the gateway object was moved into an own chart, called now `zeebe-gateway`.

```yaml
global:
  image:
    repository: camunda/zeebe
    tag: SNAPSHOT
    pullPolicy: Always

zeebe:
  clusterSize: 3
  partitionCount: 3
  replicationFactor: 3
  cpuThreadCount: 4
  ioThreadCount: 4

  JavaOpts: >-
    -XX:MaxRAMPercentage=25.0
    -XX:+ExitOnOutOfMemoryError
    -XX:+HeapDumpOnOutOfMemoryError
    -XX:HeapDumpPath=/usr/local/zeebe/data
    -XX:ErrorFile=/usr/local/zeebe/data/zeebe_error%p.log
    -Xlog:gc*:file=/usr/local/zeebe/data/gc.log:time:filecount=7,filesize=8M

  env:
    - name: ZEEBE_BROKER_EXECUTION_METRICS_EXPORTER_ENABLED
      value: "true"
    - name: ATOMIX_LOG_LEVEL
      value: INFO

  resources:
    limits:
      cpu: 5
      memory: 12Gi
    requests:
      cpu: 5
      memory: 12Gi

zeebe-gateway:
  replicas: 1
  logLevel: debug
  env:
    - name: ZEEBE_LOG_APPENDER
      value: Stackdriver
    - name: ZEEBE_LOG_STACKDRIVER_SERVICENAME
      value: zeebe-gateway
    - name: ZEEBE_LOG_STACKDRIVER_SERVICEVERSION
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: ATOMIX_LOG_LEVEL
      value: INFO
    - name: ZEEBE_GATEWAY_MONITORING_ENABLED
      value: "true"
    - name: ZEEBE_GATEWAY_THREADS_MANAGEMENTTHREADS
      value: "4"

tasklist:
  enabled: false

operate:
  enabled: false
```

As an example migration you can check this [commit](https://github.com/camunda-cloud/zeebe/commit/814d6ce58d7827960f47ae5296ac014873a3092c) for the zeebe benchmarks.

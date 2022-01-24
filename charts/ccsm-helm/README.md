[![Community Extension](https://img.shields.io/badge/Community%20Extension-An%20open%20source%20community%20maintained%20project-FF4700)](https://github.com/camunda-community-hub/community)[![Lifecycle: Incubating](https://img.shields.io/badge/Lifecycle-Incubating-blue)](https://github.com/Camunda-Community-Hub/community/blob/main/extension-lifecycle.md#incubating-)[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

# Camunda Cloud Self-Managed Helm Chart

This functionality is in beta and is subject to change. The design and code is less mature than official GA features and is provided as-is with no warranties. Beta features are not subject to the support SLA of official GA features.

## Requirements

* [Helm](https://helm.sh/) >= 3.x +
* Kubernetes >= 1.20+
* Minimum cluster requirements include the following to run this chart with default settings. All of these settings are configurable.
  * Three Kubernetes nodes to respect the default "hard" affinity settings
  * 2GB of RAM for the JVM heap


## Installing

* Add the official CCSM helm charts repo

  ```shell
  helm repo add ccsm https://helm.camunda.io
  ```

* Install it

  ```shell
  helm install camunda-cloud ccsm/ccsm-helm
  ```

 ## Configuration
  | Parameter                     | Description                                                                                                                                                                                                                                                                                                                | Default                                                                                                                   |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `labels`                 | labels to be applied to the StatefulSet and Service                                                                                                                                | `app: zeebe`                                                                                                           |
| `annotations`                 | annotations to be applied to the StatefulSet and Service                                                                                                                                | ``                                                                                                           |
| `podAnnotations`                 | annotations to be applied to the StatefulSet pod Template                                                                                                                                | ``                                                                                                           |
| `global.elasticsearch.disableExporter`                 | Disable [Elasticsearch Exporter](https://github.com/camunda-cloud/zeebe/tree/develop/exporters/elasticsearch-exporter) in Zeebe                                                                                                                                | `false`                                                                                                           |
| `global.elasticsearch.host`         | ElasticSearch host to use in Elasticsearch Exporter connection  | `elasticsearch-master` |
| `global.elasticsearch.port`         | ElasticSearch port to use in Elasticsearch Exporter connection | `9200` |
| `global.elasticsearch.url`         | ElasticSearch full url to use in Elasticsearch Exporter connection. This config overrides the `host` and `port` above.  |  |
| `elasticsearch.enabled`                 | Enable ElasticSearch deployment as part of the Zeebe Cluster                                                                                                                                | `true`                                                                                                           |
| `kibana.enabled`                 | Enable Kibana deployment as part of the Zeebe Cluster                                                                                                                                | `false`                                                                                                           |
| `prometheus.enabled`                 | Enable Prometheus operator as part of the Zeebe Cluster                                                                                                                          | `false`                                                                                                           |
| `prometheus.servicemonitor.enabled`                 | Deploy a `ServiceMonitor` for your Zeebe Cluster                                                                                                                                 | `false`                                                                                                           |
| `clusterSize`                 | Set the Zeebe Cluster Size and the number of replicas of the replica set                                                                                                                                | `3`
| `partitionCount`                 | Set the Zeebe Cluster partition count                                                                                                                                | `3`
| `replicationFactor`                 | Set the Zeebe Cluster replication factor                                                                                                                                | `3`
| `cpuThreadCount`                 | Set the Zeebe Cluster CPU thread count                                                                                                                                | `2`
| `ioThreadCount`                 | Set the Zeebe Cluster IO thread count                                                                                                                                | `2`
| `zeebeCfg`                 | Can be used to set several zeebe configuration options.                                                                                                                                | `null`
| `logLevel`                 | Sets the log level for io.zeebe packages; must be one of: ERROR, WARN, INFO, DEBUG, TRACE | `info`
| `log4j2`                   | Log4J 2.x XML configuration; if provided, the contents given will be written to file and will overwrite the distribution's default `/usr/local/zeebe/config/log4j2.xml` | ``
| `gatewayMetrics`                 | Enables the exporting of the gateway prometheus metrics                                                                                                                                | `false`
| `JavaOpts`                 | Set the Zeebe Cluster Broker JavaOpts. This is where you should configure the jvm heap size.                                                                                                                                | `-XX:MaxRAMPercentage=25.0 -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/zeebe/data -XX:ErrorFile=/usr/local/zeebe/data/zeebe_error%p.log -XX:+ExitOnOutOfMemoryError`
| `resources`                 | Set the Zeebe Cluster Broker Kubernetes Resource Request and Limits                                                                                                                                | `requests:`<br>  `cpu: 500m`<br>  ` memory: 1Gi`<br>`limits:`<br>  ` cpu: 1000m`<br>  ` memory: 2Gi`
| `env`                       |  Pass additional environment variables to the Zeebe broker pods; <br> variables should be specified using standard Kubernetes raw YAML format. See below for an example.| `[]`
| `podDisruptionBudget.enabled`         | Create a podDisruptionBudget for the broker pods | `false`
| `podDisruptionBudget.minAvailable`         | Minimum number of available broker pods for PodDisruptionBudget |
| `podDisruptionBudget.maxUnavailable`       | Maximum number of unavailable broker pods for PodDisruptionBudget | `1`
| `podSecurityContext` | Sets the [securityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/) for the Zeebe pod. Can hold pod-level security attributes and common container settings. |  {}
| `pvcSize`                 | Set the Zeebe Cluster Persistence Volume Claim Request storage size                                                                                                                                | `10Gi`
| `pvcAccessModes`                 | Set the Zeebe Cluster Persistence Volume Claim Request accessModes                                                                                                                                | `[ "ReadWriteOnce" ]`
| `pvcStorageClassName`                 | Set the Zeebe Cluster Persistence Volume Claim Request storageClassName                                                                                                                                | ``
| `extraInitContainers`                 | add extra initContainers sections to StatefulSet                                                                                                                                | ``
| `nodeSelector`                 | Node selection constraint to schedule Zeebe on specific nodes                                                                                                                                | {}
| `priorityClassName`                 | Name of the priority class to assign on Zeebe pods                                                                                                                                | ``
| `tolerations`                 | Tolerations to allow Zeebe to run on dedicated nodes                                                                                                                                | []
| `affinity`                 | Use affinity constraints to schedule Zeebe on specific nodes                                                                                                                                | {}
| `gateway.replicas`         | The number of standalone gateways that should be deployed | `1`
| `gateway.priorityClassName`                 | Name of the priority class to assign on Zeebe gateway pods                                                                                                                                | ``
| `gateway.logLevel`         | The log level of the gateway, one of: ERROR, WARN, INFO, DEBUG, TRACE | `info`
| `gateway.log4j2`           | Log4J 2.x XML configuration; if provided, the contents given will be written to file and will overwrite the distribution's default `/usr/local/zeebe/config/log4j2.xml` | ``
| `gateway.env`         |  Pass additional environment variables to the Zeebe broker pods; <br> variables should be specified using standard Kubernetes raw YAML format. See below for an example.| `[]`
| `gateway.podAnnotations`         | Annotations to be applied to the gateway Deployment pod template | ``
| `gateway.podDisruptionBudget.enabled`         | Create a PodDisruptionBudget for the gateway pods | `false`
| `gateway.podDisruptionBudget.minAvailable`         | minimum number of available gateway pods for PodDisruptionBudget | `1`
| `gateway.podDisruptionBudget.maxUnavailable`       | maximum number of unavailable gateway pods for PodDisruptionBudget |
| `serviceType`         | The type of cluster service | `ClusterIP`
| `serviceGatewayType`         | The type of cluster gateway service | `ClusterIP`
| `serviceHttpPort`         | The http port used by the brokers and the gateway| `9600`
| `serviceGatewayPort`         | The gateway port used by the gateway | `26500`
| `serviceInternalPort`         | The internal port used by the brokers and the gateway | `26502`
| `serviceCommandPort`         | The command port used the brokers | `26501`
| `serviceHttpName`         | The http port name used by the brokers and the gateway| `http`
| `serviceGatewayName`         | The gateway port name used by the gateway | `gateway`
| `serviceInternalName`         | The internal port name used by the brokers and the gateway | `internal`
| `serviceCommandName`         | The command port name used the brokers | `command`

## Examples

### Env Example
```yaml

env:
  - name: ZEEBE_GATEWAY_MONITORING_ENABLED
    value: "true"
```


## Adding dynamic exporters to Zeebe Brokers

This chart supports the addition of Zeebe Exporters by using initContainer as shown in the following example:

```
extraInitContainers: |
  - name: init-exporters-hazelcast
    image: busybox:1.28
    command: ['/bin/sh', '-c']
    args: ['wget --no-check-certificate https://repo1.maven.org/maven2/io/zeebe/hazelcast/zeebe-hazelcast-exporter/0.8.0-alpha1/zeebe-hazelcast-exporter-0.8.0-alpha1-jar-with-dependencies.jar -O /exporters/zeebe-hazelcast-exporter.jar; ls -al']
    volumeMounts:
    - name: exporters
      mountPath: /exporters/
  - name: init-exporters-kafka
    image: busybox:1.28
    command: ['/bin/sh', '-c']
    args: ['wget --no-check-certificate https://github.com/zeebe-io/zeebe-kafka-exporter/releases/download/1.1.0/zeebe-kafka-exporter-1.1.0-uber.jar -O /exporters/zeebe-kafka-exporter.jar; ls -al']
    volumeMounts:
    - name: exporters
      mountPath: /exporters/
env:
  ZEEBE_BROKER_EXPORTERS_HAZELCAST_JARPATH: exporters/zeebe-hazelcast-exporter.jar
  ZEEBE_BROKER_EXPORTERS_HAZELCAST_CLASSNAME: io.zeebe.hazelcast.exporter.HazelcastExporter
  ZEEBE_HAZELCAST_REMOTE_ADDRESS: "{{ .Release.Name }}-hazelcast"
```
This example is downloading the exporters Jar from an URL and adding the Jars to the `exporters` directory that will be scanned for jars and added to the zeebe broker classpath. Then with `environment variables` you can configure the exporter parameters.

## Dependencies

This chart currently depends on the following charts:

* [ElasticSearch Helm Chart](https://github.com/elastic/helm-charts/blob/master/elasticsearch/README.md)
* [Kibana Helm Chart](https://github.com/elastic/helm-charts/tree/master/kibana)
* [Prometheus Operator Helm Chart](https://github.com/helm/charts/tree/master/stable/prometheus-operator)

These dependencies can be turned on or off and parameters can be overiden from these dependent charts by changing the `values.yaml` file. For example:

```yaml
elasticsearch:
  enabled: true
  imageTag: <YOUR VERSION HERE>
kibana:
  enabled: false
```

## Development

For development purpose you might want to deploy and test the charts without creating a new release. In order to do this you can run the following:

```sh
 helm install <RELEASENAME> charts/ccsm-helm/
```

If you see errors like:

```sh
Error: found in Chart.yaml, but missing in charts/ directory: elasticsearch, kibana, kube-prometheus-stack
```

Then you need to download the dependencies first. You can do this via:

```sh
$ helm dependency update charts/ccsm-helm/
Getting updates for unmanaged Helm repositories...
...Successfully got an update from the "https://helm.elastic.co" chart repository
...Successfully got an update from the "https://helm.elastic.co" chart repository
...Successfully got an update from the "https://prometheus-community.github.io/helm-charts" chart repository
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "ccsm" chart repository
...Successfully got an update from the "stable" chart repository
Update Complete. ⎈Happy Helming!⎈
Saving 3 charts
Downloading elasticsearch from repo https://helm.elastic.co
Downloading kibana from repo https://helm.elastic.co
Downloading kube-prometheus-stack from repo https://prometheus-community.github.io/helm-charts
Deleting outdated charts
```

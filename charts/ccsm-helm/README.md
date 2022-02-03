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

```shell
  # Add the official Camunda Cloud helm charts repo
  helm repo add camunda-cloud https://helm.camunda.io
  # Install the Camunda Cloud Self Managed chart
  helm install camunda-cloud camunda-cloud/ccsm-helm
```

## Configuration

The following sections contain the configuration values for the chart and each sub chart. All of them can be overwritten via an separate `values.yaml` file.

Check out the default [values.yaml](values.yaml) file, which contains the same content and documentation.

### Global 

| Section | Parameter | Description | Default |
|-|-|-|-|
| `global` | | Global variables which can be accessed by all sub charts | |
| | `annotations` | Can be used to define common annotations, which should be applied to all deployments | |
| | `labels` | Can be used to define common labels, which should be applied to all deployments | |
| | `image.tag` | Defines the tag / version which should be used in the chart. | |
| | `image.pullPolicy` | Defines the [image pull policy](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy) which should be used. | IfNotPresent |
| | `image.pullSecrets` | Can be used to configure [image pull secrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod) which should be used. | `[]` |
| | `elasticsearch.disableExporter` | If true, disables [Elasticsearch Exporter](https://github.com/camunda-cloud/zeebe/tree/develop/exporters/elasticsearch-exporter) in Zeebe | `false` |
| | `elasticsearch.url` | Can be used to configure the URL to access Elasticsearch, if not set services fallback to host and port configuration. | |
| | `elasticsearch.host` | Defines the Elasticsearch host, ideally the service name inside the namespace. | `elasticsearch-master` |
| | `elasticsearch.port` | Defines the Elasticsearch port, under which Elasticsearch can be accessed | `9200` |
| | `elasticsearch.clusterName` | Defines the cluster name which is used by Elasticsearch. | `elasticsearch` |
| | `elasticsearch.prefix` | Defines the prefix which is used by the Zeebe Elasticsearch Exporter to create Elasticsearch indexes | `zeebe-record` |
| | `zeebeClusterName` | Defines the cluster name for the Zeebe cluster. All pods get this prefix in their name. | `{{ .Release.Name }}-zeebe` |
| | `zeebePort` | Defines the port which is used for the Zeebe Gateway. This port accepts the GRPC Client messages and forwards them to the Zeebe Brokers. | 26500 |
| `elasticsearch`| `enabled` | Enable ElasticSearch deployment as part of the Zeebe Cluster | `true` |
| `kibana`| `enabled` | Enable Kibana deployment as part of the Zeebe Cluster | `false` |
| `prometheus`| `enabled` | Enable Prometheus operator as part of the Zeebe Cluster | `false` |
| | `servicemonitor.enabled` | Deploy a `ServiceMonitor` for your Zeebe Cluster | `false` |

### Zeebe

| Section | Parameter | Description | Default |
|-|-|-|-|
| `zeebe` | Configuration for the Zeebe sub chart. Contains configuration for the Zeebe broker and related resources. | |
| | `clusterSize` | Defines the amount of brokers (=replicas), which are deployed via helm | `3` |
| | `partitionCount` | Defines how many Zeebe partitions are set up in the cluster | `3` |
| | `replicationFactor` | Defines how each partition is replicated, the value defines the number of nodes | `3` |
| | `env` | Can be used to set extra environment variables in each Zeebe broker container | `[ ]` |
| | `logLevel` | Defines the log level which is used by the Zeebe brokers; must be one of: ERROR, WARN, INFO, DEBUG, TRACE | `info` |
| | `log4j2` | Can be used to overwrite the Log4J 2.x XML configuration. If provided, the contents given will be written to file and will overwrite the distribution's default `/usr/local/zeebe/config/log4j2.xml` | `` |
| | `JavaOpts` | Can be used to set the Zeebe Broker JavaOpts. This is where you should configure the jvm heap size. | `-XX:MaxRAMPercentage=25.0 -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/zeebe/data -XX:ErrorFile=/usr/local/zeebe/data/zeebe_error%p.log -XX:+ExitOnOutOfMemoryError` |
| |`prometheusServiceMonitor.enabled`| If true, then a service monitor will be deployed, which allows an installed Prometheus controller to scrape metrics from the broker pods | `false`|
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.httpPort` | Defines the port of the HTTP endpoint, where for example metrics are provided | `9600` |
| | `service.httpName` | Defines the name of the HTTP endpoint, where for example metrics are provided | `http` |
| | `service.commandPort` | Defines the port of the Command API endpoint, where the broker commands are sent to | `26501` |
| | `service.commandName` | Defines the name of the Command API endpoint, where the broker commands are sent to | `command` |
| | `service.internalPort` | Defines the port of the Internal API endpoint, which is used for internal communication | `26502` |
| | `service.internalName` | Defines the name of the Internal API endpoint, which is used for internal communication | `internal` |
| | `serviceAccount.enabled` | If true, enables the broker service account | `true` |
| | `serviceAccount.name` | Can be used to set the name of the broker service account | `""` |
| | `serviceAccount.annotations` | Can be used to set the annotations of the broker service account | `{ }` |
| | `cpuThreadCount` | Defines how many threads can be used for the processing on each broker pod | `2` |
| | `ioThreadCount` | Defines how many threads can be used for the exporting on each broker pod | `2` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 500m`<br> `  memory: 2Gi`<br>`limits:`<br>  ` cpu: 1000m`<br>  ` memory: 4Gi` |
| | `pvcSize` | Defines the [persistent volume claim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims) size, which is used by each broker pod | `10Gi` |
| | `pvcAccessModes` | Can be used to configure the [persistent volume claim access mode](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes) | `[ "ReadWriteOnce" ]` |
| | `pvcStorageClassName` | Can be used to set the storage class name which should be used by the persistent volume claim. It is recommended to use a storage class, which is backed with a SSD. | `` |
| | `extraVolumes` | Can be used to define extra volumes for the broker pods, useful for additional exporters | `{ }`|
| | `extraVolumeMounts` | Can be used to mount extra volumes for the broker pods, useful for additional exporters | `{ }` |
| | `extraInitContainers` | Can be used to set up extra init containers for the broker pods, useful for additional exporters | `{ }` |
| | `podAnnotations` | Can be used to define extra broker pod annotations | `{ }` |
| | `podLabels` | Can be used to define extra broker pod labels | `{ }` |
| | `podDisruptionBudget` | Configuration to configure a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) for the broker pods | |
| | `podDisruptionBudget.enabled` | If true a pod disruption budget is defined for the brokers | `false` |
| | `podDisruptionBudget.minAvailable` | Can be used to set how many pods should be available | `` |
| | `podDisruptionBudget.maxUnavailable` | Can be used to set how many pods should be at max. unavailable | `1` |
| | `podSecurityContext` | Defines the security options the broker and gateway container should be run with | |
| | `nodeSelector` | Can be used to define on which nodes the broker pods should run | `{ } ` |
| | `tolerations` | Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` | Can be used to define [pod affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |
| | `priorityClassName` | Can be used to define the broker [pods priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass) | `""` |
| | `readinessProbe` | Configuration for the Zeebe broker readiness probe | |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the Zeebe brokers | `/ready` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `10` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |

### Zeebe-gateway

| Section | Parameter | Description | Default |
|-|-|-|-|
| `zeebe-gateway`| | Configuration to define properties related to the Zeebe standalone gateway | |
| | `replicas` | Defines how many standalone gateways are deployed | `1` |
| | `podAnnotations` | Can be used to define extra gateway pod annotations | `{ }` |
| | `podLabels` | Can be used to define extra gateway pod labels | `{ }` |
| | `annotations` | Can be used to define gateway deployment annotations | `{ } `|
| | `logLevel` | Defines the log level which is used by the gateway | `info` |
| | `log4j2` | Can be used to overwrite the log4j2 configuration of the gateway | `""` |
| | `env` | Can be used to set extra environment variables in each gateway container | `[ ]` |
| | `podSecurityContext` | Defines the security options the gateway container should be run with | `{ } `|
| | `podDisruptionBudget` | Configuration to configure a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) for the broker pods | |
| | `podDisruptionBudget.enabled` | If true a pod disruption budget is defined for the brokers | `false` |
| | `podDisruptionBudget.minAvailable` | Can be used to set how many pods should be available | `` |
| | `podDisruptionBudget.maxUnavailable` | Can be used to set how many pods should be at max. unavailable | `1` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `{ }` |
| | `priorityClassName` | Can be used to define the broker [pods priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass) | `""` |
| | `nodeSelector` | Can be used to define on which nodes the gateway pods should run | `{ } ` |
| | `tolerations` | Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` | Can be used to define [pod affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |
| | `extraVolumeMounts` | Can be used to mount extra volumes for the gateway pods, useful for enabling TLS between gateway and broker | `{ }` |
| | `extraVolumes` | Can be used to define extra volumes for the gateway pods, useful for enabling TLS between gateway and broker | `{ }` |
| | `service` | Configuration for the gateway service | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.httpPort` | Defines the port of the HTTP endpoint, where for example metrics are provided | `9600` |
| | `service.httpName` | Defines the name of the HTTP endpoint, where for example metrics are provided | `http` |
| | `service.gatewayPort` | Defines the port of the gateway endpoint, where client commands (gRPC) are sent to | `26500` |
| | `service.gatewayName` | Defines the name of the gateway endpoint, where client commands (gRPC) are sent to | `gateway` |
| | `service.internalPort` | Defines the port of the Internal API endpoint, which is used for internal communication | `26502` |
| | `service.internalName` | Defines the name of the Internal API endpoint, which is used for internal communication | `internal` |
| | `serviceAccount` | Configuration for the service account where the gateway pods are assigned to | |
| | `serviceAccount.enabled` | If true, enables the gateway service account | `true` |
| | `serviceAccount.name` | Can be used to set the name of the gateway service account | `""` |
| | `serviceAccount.annotations` | Can be used to set the annotations of the gateway service account | `{ }` |

### Operate

| Section | Parameter | Description | Default |
|-|-|-|-|
| `operate` | | Configuration for the Operate sub chart. | |
| | `enabled` | If true, the Operate deployment and its related resources are deployed via a helm release | `true` |
| | `logging` | Configuration for the Operate logging. This template will be directly included in the Operate configuration yaml file | `level:` <br/> `ROOT: INFO` <br/> `org.camunda.operate: DEBUG` |
| | `service` | Configuration to configure the Operate service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.port` | Defines the port of the service, where the Operate web application will be available | `80` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 500m`<br> `  memory: 1Gi`<br>`limits:`<br> ` cpu: 1000m`<br> ` memory: 2Gi` |
| | `env` | Can be used to set extra environment variables in each operate container | `[ ]` |
| | `extraVolumes` | Can be used to define extra volumes for the Operate pods, useful for TLS and self-signed certificates | `{ }` |
| | `extraVolumeMounts` | Can be used to mount extra volumes for the broker pods, useful for TLS and self-signed certificates | `{ }` |
| | `serviceAccount` | Configuration for the service account where the Operate pods are assigned to | |
| | `serviceAccount.enabled` | If true, enables the Operate service account | `true` |
| | `serviceAccount.name` | Can be used to set the name of the Operate service account | `""` |
| | `serviceAccount.annotations` | Can be used to set the annotations of the Operate service account | `{ }` |
| | `ingress` | Configuration to configure the ingress resource | |
| | `ingress.enabled` | If true, an ingress resource is deployed with the Operate deployment. Only useful if an ingress controller is available, like nginx. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Operate service and port https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `ingress.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.tls.enabled` | If true, then TLS is configured on the ingress resource. If enabled the Ingress.host need to be defined. | `false` |
| | `ingress.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | `""` |
| | `podSecurityContext` | Defines the security options the Operate container should be run with | `{ }` |

### Tasklist

| Section | Parameter | Description | Default |
|-|-|-|-|
| | `tasklist` | Configuration for the Tasklist sub chart. | |
| | `enabled` | If true, the Tasklist deployment and its related resources are deployed via a helm release | `true` |
| | `service` | Configuration to configure the Tasklist service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.port` | Defines the port of the service, where the Tasklist web application will be available | `80` |
| | `springProfilesActive` | Can be used to set the active spring profiles used by Tasklist | `""` |
| | `graphqlPlaygroundEnabled` | If true, enables the GraphQl playground | `""` |
| | `graphqlPlaygroundEnabled` | Can be set to include the credentials in each request, should be set to "include" if GraphQl playground is enabled | `""` |
| | `podSecurityContext` | Defines the security options the Operate container should be run with | `{ }` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 500m`<br> `  memory: 1Gi`<br>`limits:`<br> ` cpu: 1000m`<br> ` memory: 2Gi` |
| | `ingress` | Configuration to configure the ingress resource | |
| | `ingress.enabled` | If true, an ingress resource is deployed with the Tasklist deployment. Only useful if an ingress controller is available, like nginx. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Tasklist service and port https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |

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
This example is downloading the exporters Jar from a URL and adding the Jars to the `exporters` directory that will be scanned for jars and added to the zeebe broker classpath. Then with `environment variables` you can configure the exporter parameters.

## Dependencies

This chart currently depends on the following charts:

* [ElasticSearch Helm Chart](https://github.com/elastic/helm-charts/blob/master/elasticsearch/README.md)
* *optional* [Kibana Helm Chart](https://github.com/elastic/helm-charts/tree/master/kibana)
* *optional* [Prometheus Operator Helm Chart](https://github.com/helm/charts/tree/master/stable/prometheus-operator)

These dependencies can be turned on or off and parameters can be overiden from these dependent charts by changing the `values.yaml` file. For example:

```yaml
elasticsearch:
  enabled: true
  imageTag: <YOUR VERSION HERE>
kibana:
  enabled: false
```

## Development

For development purpose you might want to deploy and test the charts without creating a new helm chart release. In order to do this you can run the following:

```sh
 helm install <RELEASENAME> --atomic --debug charts/ccsm-helm/
```

 * `--atomic if set, the installation process deletes the installation on failure. The --wait flag will be set automatically if --atomic is used`

 * `--debug enable verbose output`

To generate the resources / manifests without really install them you can use: 

 * `--dry-run simulate an install`

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

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)[![Go CI](https://github.com/camunda/camunda-platform-helm/actions/workflows/go.yml/badge.svg)](https://github.com/camunda/camunda-platform-helm/actions/workflows/go.yml)

# Camunda Platform Helm Chart

- [Camunda Platform Helm Chart](#camunda-platform-helm-chart)
  - [Requirements](#requirements)
  - [Installing](#installing)
  - [Configuration](#configuration)
    - [Global](#global)
    - [Camunda Platform](#camunda-platform)
    - [Zeebe](#zeebe)
    - [Zeebe Gateway](#zeebe-gateway)
    - [Operate](#operate)
    - [Tasklist](#tasklist)
    - [Optimize](#optimize)
    - [Identity](#identity)
    - [Elasticsearch](#elasticsearch)
  - [Adding dynamic exporters to Zeebe Brokers](#adding-dynamic-exporters-to-zeebe-brokers)
  - [Development](#development)
  - [Releasing the Charts](#releasing-the-charts)

<small><i><a href='http://ecotrust-canada.github.io/markdown-toc/'>Table of contents generated with markdown-toc</a></i></small>

## Requirements

* [Helm](https://helm.sh/) >= 3.x +
* Kubernetes >= 1.20+
* Minimum cluster requirements include the following to run this chart with default settings. All of these settings are configurable.
  * Three Kubernetes nodes to respect the default "hard" affinity settings
  * 2GB of RAM for the JVM heap

## Installing

The first command adds the official Camunda Platform Helm charts repo and the second installs the Camunda Platform chart to your current kubernetes context.

```shell
  helm repo add camunda https://helm.camunda.io
  helm install camunda-platform camunda/camunda-platform
```

## Configuration

The following sections contain the configuration values for the chart and each sub chart. All of them can be overwritten via a separate `values.yaml` file.

Check out the default [values.yaml](values.yaml) file, which contains the same content and documentation.

### Global 

| Section | Parameter | Description | Default |
|-|-|-|-|
| `global` | | Global variables which can be accessed by all sub charts | |
| | `annotations` | Can be used to define common annotations, which should be applied to all deployments | |
| | `labels` | Can be used to define common labels, which should be applied to all deployments | |
| | `image.tag` | Defines the tag / version which should be used in the chart. | |
| | `image.pullPolicy` | Defines the [image pull policy](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy) which should be used. | IfNotPresent |
| | `image.pullSecrets` | Can be used to configure [image pull secrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod). Also it could be configured per component separately. | `[]` |
| | `elasticsearch.disableExporter` | If true, disables the [Elasticsearch Exporter](https://github.com/camunda-cloud/zeebe/tree/develop/exporters/elasticsearch-exporter) in Zeebe | `false` |
| | `elasticsearch.url` | Can be used to configure the URL to access Elasticsearch. When not set, services fallback to host and port configuration. | |
| | `elasticsearch.host` | Defines the Elasticsearch host, ideally the service name inside the namespace. | `elasticsearch-master` |
| | `elasticsearch.port` | Defines the Elasticsearch port, under which Elasticsearch can be accessed | `9200` |
| | `elasticsearch.clusterName` | Defines the cluster name which is used by Elasticsearch. | `elasticsearch` |
| | `elasticsearch.prefix` | Defines the prefix which is used by the Zeebe Elasticsearch Exporter to create Elasticsearch indexes | `zeebe-record` |
| | `zeebeClusterName` | Defines the cluster name for the Zeebe cluster. All pods get this prefix in their name. | `{{ .Release.Name }}-zeebe` |
| | `zeebePort` | Defines the port which is used for the Zeebe Gateway. This port accepts the GRPC Client messages and forwards them to the Zeebe Brokers. | 26500 |
| | `identity.auth.enabled` |  If true, enables the Identity authentication otherwise basic-auth will be used on all services. | `true` |
| | `identity.auth.publicIssuerUrl` | Defines the token issuer (Keycloak) URL, where the services can request JWT tokens. Should be public accessible, per default we assume a port-forward to Keycloak (18080) is created before login. Can be overwritten if, ingress is in use and an external IP is available. | `"http://localhost:18080/auth/realms/camunda-platform"` |
| | `identity.auth.operate.existingSecret` |  Can be used to reference an existing secret. If not set, a random secret is generated. The existing secret should contain an `operate-secret` field, which will be used as secret for the Identity-Operate communication. | `` |
| | `identity.auth.operate.redirectUrl` |  Defines the redirect URL, which is used by Keycloak to access Operate. Should be public accessible, the default value works if port-forward to operate is created to 8081. Can be overwritten if, ingress is in use and an external IP is available. | `"http://localhost:8081"` |
| | `identity.auth.tasklist.existingSecret` |  Can be used to reference an existing secret. If not set, a random secret is generated. The existing secret should contain an `tasklist-secret` field, which will be used as secret for the Identity-Tasklist communication. | ` ` |
| | `identity.auth.tasklist.redirectUrl` |  Defines the redirect URL, which is used by Keycloak to access Tasklist. Should be public accessible, the default value works if port-forward to Tasklist is created to 8082. Can be overwritten if, an Ingress is in use and an external IP is available. | `"http://localhost:8082"` |
| | `identity.auth.optimize.existingSecret` |  Can be used to reference an existing secret. If not set, a random secret is generated. The existing secret should contain an `optimize-secret` field, which will be used as secret for the Identity-Optimize communication. | ` ` |
| | `identity.auth.optimize.redirectUrl` |  Defines the redirect URL, which is used by Keycloak to access Optimize. Should be public accessible, the default value works if port-forward to Optimize is created to 8083. Can be overwritten if, an Ingress is in use and an external IP is available. | `"http://localhost:8083"` |
| `elasticsearch`| `enabled` | Enable Elasticsearch deployment as part of the Camunda Platform Cluster | `true` |

### Camunda Platform

| Section | Parameter | Description | Default |
|-|-|-|-|
| `retentionPolicy` | | Configuration to configure the Elasticsearch index retention policies | |
| | `enabled` | If true, Elasticsearch curator cronjob and configuration will be deployed. | `false` |
| | `schedule` | Defines how often/when the curator should run. | `"0 0 * * *"` |
| | `zeebeIndexTTL` | Defines after how many days a zeebe index can be deleted. | `1` |
| | `zeebeIndexMaxSize` | Can be set to configure the maximum allowed zeebe index size in gigabytes. After reaching that size, curator will delete that corresponding index on the next run. To benefit from that configuration the schedule needs to be configured small enough, like every 15 minutes. | `` |
| | `operateIndexTTL` | Defines after how many days an Operate index can be deleted. | `30` |
| | `tasklistIndexTTL` | Defines after how many days an Tasklist index can be deleted. | `30` |
| | `image.repository` | Defines which image repository to use. | `bitnami/elasticsearch-curator` |
| | `image.tag` | Defines the tag / version which should be used in the chart. | `5.8.4` |
| `prometheusServiceMonitor` | | Configuration to configure a prometheus service monitor | |
| | `enabled` | If true, then a service monitor will be deployed, which allows an installed prometheus controller to scrape metrics from the deployed pods. | `false`|
| | `labels` | Can be set to configure extra labels, which will be added to the servicemonitor and can be used on the prometheus controller for selecting the servicemonitors | `release: metrics` |
| | `scrapeInterval` | Can be set to configure the interval at which metrics should be scraped | `10s` |

### Zeebe

Information about Zeebe you can find [here](https://docs.camunda.io/docs/components/zeebe/zeebe-overview/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `zeebe` | Configuration for the Zeebe sub chart. Contains configuration for the Zeebe broker and related resources. | |
| | `image` | Configuration to configure the Zeebe image specifics. | |
| | `image.repository` | Defines which image repository to use. | `camunda/zeebe` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | ` ` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `clusterSize` | Defines the amount of brokers (=replicas), which are deployed via helm | `3` |
| | `partitionCount` | Defines how many Zeebe partitions are set up in the cluster | `3` |
| | `replicationFactor` | Defines how each partition is replicated, the value defines the number of nodes | `3` |
| | `env` | Can be used to set extra environment variables in each Zeebe broker container | `- name: ZEEBE_BROKER_DATA_SNAPSHOTPERIOD` </br>`  value: "5m"`</br>`- name: ZEEBE_BROKER_EXECUTION_METRICS_EXPORTER_ENABLED`</br>`  value: "true"`</br>`- name: ZEEBE_BROKER_DATA_DISKUSAGECOMMANDWATERMARK`</br>`  value: "0.85"`</br>`- name: ZEEBE_BROKER_DATA_DISKUSAGEREPLICATIONWATERMARK`</br>`  value: "0.87"` |
| | `configMap.defaultMode` | Can be used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. See [Api docs](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L1615-L1623) for more details. It is useful to configure it if you want to run the helm charts in OpenShift. | [`0744`](https://chmodcommand.com/chmod-744/) |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` | 
| | `logLevel` | Defines the log level which is used by the Zeebe brokers; must be one of: ERROR, WARN, INFO, DEBUG, TRACE | `info` |
| | `log4j2` | Can be used to overwrite the Log4J 2.x XML configuration. If provided, the contents given will be written to file and will overwrite the distribution's default `/usr/local/zeebe/config/log4j2.xml` | ` ` |
| | `JavaOpts` | Can be used to set the Zeebe Broker JavaOpts. This is where you should configure the jvm heap size. | `-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/zeebe/data -XX:ErrorFile=/usr/local/zeebe/data/zeebe_error%p.log -XX:+ExitOnOutOfMemoryError` |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.httpPort` | Defines the port of the HTTP endpoint, where for example metrics are provided | `9600` |
| | `service.httpName` | Defines the name of the HTTP endpoint, where for example metrics are provided | `http` |
| | `service.commandPort` | Defines the port of the Command API endpoint, where the broker commands are sent to | `26501` |
| | `service.commandName` | Defines the name of the Command API endpoint, where the broker commands are sent to | `command` |
| | `service.internalPort` | Defines the port of the Internal API endpoint, which is used for internal communication | `26502` |
| | `service.internalName` | Defines the name of the Internal API endpoint, which is used for internal communication | `internal` |
| | `service.extraPort` |  Exposes any other [ports](https://kubernetes.io/docs/concepts/services-networking/service/#multi-port-services) which are required. Can be useful for exporters | `[]` |
| | `serviceAccount.enabled` | If true, enables the broker service account | `true` |
| | `serviceAccount.name` | Can be used to set the name of the broker service account | `""` |
| | `serviceAccount.annotations` | Can be used to set the annotations of the broker service account | `{ }` |
| | `cpuThreadCount` | Defines how many threads can be used for the processing on each broker pod | `3` |
| | `ioThreadCount` | Defines how many threads can be used for the exporting on each broker pod | `3` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 800m`<br> `  memory: 1200Mi`<br>`limits:`<br>  ` cpu: 960m`<br>  ` memory: 1920Mi` |
| | `persistenceType` | defines the type of persistence which is used by Zeebe. Possible values are: disk, local and memory. <br/> disk  - means a persistence volume claim is configured and used <br/> local - means the data is stored into the container, no volumeMount nor volume nor claim is configured <br/> ram   - means zeebe uses a tmpfs for the data persistence, be aware that this takes the limits into account | `disk` |
| | `pvcSize` | Defines the [persistent volume claim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims) size, which is used by each broker pod | `32Gi` |
| | `pvcAccessModes` | Can be used to configure the [persistent volume claim access mode](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes) | `[ "ReadWriteOnce" ]` |
| | `pvcStorageClassName` | Can be used to set the storage class name which should be used by the persistent volume claim. It is recommended to use a storage class, which is backed with a SSD. | ` ` |
| | `extraVolumes` | Can be used to define extra volumes for the broker pods, useful for additional exporters | `[ ]`|
| | `extraVolumeMounts` | Can be used to mount extra volumes for the broker pods, useful for additional exporters | `[ ]` |
| | `extraInitContainers` | Can be used to set up extra init containers for the broker pods, useful for additional exporters | `[ ]` |
| | `podAnnotations` | Can be used to define extra broker pod annotations | `{ }` |
| | `podLabels` | Can be used to define extra broker pod labels | `{ }` |
| | `podDisruptionBudget` | Configuration to configure a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) for the broker pods | |
| | `podDisruptionBudget.enabled` | If true a pod disruption budget is defined for the brokers | `false` |
| | `podDisruptionBudget.minAvailable` | Can be used to set how many pods should be available. Be aware that if minAvailable is set, maxUnavailable will not be set (they are mutually exclusive). | `` |
| | `podDisruptionBudget.maxUnavailable` | Can be used to set how many pods should be at max. unavailable | `1` |
| | `containerSecurityContext` | Defines the security options the broker container should be run with | |
| | `nodeSelector` | Can be used to define on which nodes the broker pods should run | `{ } ` |
| | `tolerations` | Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` | Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity). The default defined PodAntiAffinity allows constraining on which nodes the [Zeebe pods are scheduled on](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity). It uses a hard requirement for scheduling and works based on the Zeebe pod labels. | `podAntiAffinity:</br>  requiredDuringSchedulingIgnoredDuringExecution:</br>  - labelSelector: </br>    matchExpressions:</br>    - key: "app.kubernetes.io/component"</br>    operator: In</br>    values:</br>    - zeebe-broker</br>  topologyKey: "kubernetes.io/hostname"` |
| | `priorityClassName` | Can be used to define the broker [pods priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass) | `""` |
| | `readinessProbe` | Configuration for the Zeebe broker readiness probe | |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the Zeebe brokers | `/ready` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `10` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |

### Zeebe Gateway

Information about the Zeebe Gateway you can find [here](https://docs.camunda.io/docs/components/zeebe/technical-concepts/architecture/#gateway).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `zeebe-gateway`| | Configuration to define properties related to the Zeebe standalone gateway | |
| | `replicas` | Defines how many standalone gateways are deployed | `1` |
| | `image` | Configuration to configure the zeebe-gateway image specifics. | |
| | `image.repository` | Defines which image repository to use. | `camunda/zeebe` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | ` ` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `podAnnotations` | Can be used to define extra gateway pod annotations | `{ }` |
| | `podLabels` | Can be used to define extra gateway pod labels | `{ }` |
| | `logLevel` | Defines the log level which is used by the gateway | `info` |
| | `log4j2` | Can be used to overwrite the log4j2 configuration of the gateway | `""` |
| | `JavaOpts` | Can be used to set the Zeebe Gateway JavaOpts. This is where you should configure the jvm heap size. | `-XX:+ExitOnOutOfMemoryError` |
| | `env` | Can be used to set extra environment variables in each gateway container | `[ ]` |
| | `configMap.defaultMode` | Can be used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. See [Api docs](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L1615-L1623) for more details. It is useful to configure it if you want to run the helm charts in OpenShift. | [`0744`](https://chmodcommand.com/chmod-744/) |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` |
| | `containerSecurityContext` | Defines the security options the gateway container should be run with | `{ } `|
| | `podDisruptionBudget` | Configuration to configure a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) for the broker pods. | |
| | `podDisruptionBudget.enabled` | If true a pod disruption budget is defined for the brokers | `false` |
| | `podDisruptionBudget.minAvailable` | Can be used to set how many pods should be available. Be aware that if minAvailable is set, maxUnavailable will not be set (they are mutually exclusive). | `` |
| | `podDisruptionBudget.maxUnavailable` | Can be used to set how many pods should be at max. unavailable | `1` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 400m`<br> `  memory: 450Mi`<br>`limits:`<br>  ` cpu: 400m`<br>  ` memory: 450Mi` |
| | `priorityClassName` | Can be used to define the broker [pods priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass) | `""` |
| | `nodeSelector` | Can be used to define on which nodes the gateway pods should run | `{ } ` |
| | `tolerations` | Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` | Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity). The default defined PodAntiAffinity allows constraining on which nodes the [Zeebe gateway pods are scheduled on](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity). It uses a hard requirement for scheduling and works based on the Zeebe gateway pod labels. | `podAntiAffinity:</br>  requiredDuringSchedulingIgnoredDuringExecution:</br>  - labelSelector: </br>    matchExpressions:</br>    - key: "app.kubernetes.io/component"</br>    operator: In</br>    values:</br>    - zeebe-gatway</br>  topologyKey: "kubernetes.io/hostname"` |
| | `extraVolumeMounts` | Can be used to mount extra volumes for the gateway pods, useful for enabling TLS between gateway and broker | `[ ]` |
| | `extraVolumes` | Can be used to define extra volumes for the gateway pods, useful for enabling TLS between gateway and broker | `[ ]` |
| | `extraInitContainers` | Can be used to set up extra init containers for the gateway pods, useful for adding interceptors | `[ ]` |
| | `service` | Configuration for the gateway service | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.loadBalancerIP` | Can be used to set [ip address](https://cloud.google.com/kubernetes-engine/docs/how-to/service-parameters#lb_ip) if service.type is LoadBalancer | `""` |
| | `service.loadBalancerSourceRanges` | Can be used to set list of allowed [source ip ranges](https://cloud.google.com/kubernetes-engine/docs/how-to/service-parameters#lb_source_ranges) if service.type is LoadBalancer | `[ ]` |
| | `service.httpPort` | Defines the port of the HTTP endpoint, where for example metrics are provided | `9600` |
| | `service.httpName` | Defines the name of the HTTP endpoint, where for example metrics are provided | `http` |
| | `service.gatewayPort` | Defines the port of the gateway endpoint, where client commands (gRPC) are sent to | `26500` |
| | `service.gatewayName` | Defines the name of the gateway endpoint, where client commands (gRPC) are sent to | `gateway` |
| | `service.annotations` | Defines annotations for the zeebe gateway service | `{ }` | 
| | `service.internalPort` | Defines the port of the Internal API endpoint, which is used for internal communication | `26502` |
| | `service.internalName` | Defines the name of the Internal API endpoint, which is used for internal communication | `internal` |
| | `serviceAccount` | Configuration for the service account where the gateway pods are assigned to | |
| | `serviceAccount.enabled` | If true, enables the gateway service account | `true` |
| | `serviceAccount.name` | Can be used to set the name of the gateway service account | `""` |
| | `serviceAccount.annotations` | Can be used to set the annotations of the gateway service account | `{ }` |

### Operate

Information about Operate you can find [here](https://docs.camunda.io/docs/components/operate/index/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `operate` | | Configuration for the Operate sub chart. | |
| | `enabled` | If true, the Operate deployment and its related resources are deployed via a helm release | `true` |
| | `image` | Configuration to configure the Operate image specifics. | |
| | `image.repository` | Defines which image repository to use. | `camunda/operate` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | `` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `podLabels` |  Can be used to define extra Operate pod labels | `{ }` |
| | `logging` | Configuration for the Operate logging. This template will be directly included in the Operate configuration yaml file | `level:` <br/> `ROOT: INFO` <br/> `org.camunda.operate: DEBUG` |
| | `service` | Configuration to configure the Operate service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.port` | Defines the port of the service, where the Operate web application will be available | `80` |
| | `service.annotations` | Defines annotations for the Operate service | `{ }` | 
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 600m`<br> `  memory: 400Mi`<br>`limits:`<br> ` cpu: 2000m`<br> ` memory: 2Gi` |
| | `env` | Can be used to set extra environment variables in each Operate container | `[ ]` |
| | `configMap.defaultMode` | Can be used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. See [Api docs](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L1615-L1623) for more details. It is useful to configure it if you want to run the helm charts in OpenShift. | [`0744`](https://chmodcommand.com/chmod-744/) |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` |
| | `extraVolumes` | Can be used to define extra volumes for the Operate pods, useful for TLS and self-signed certificates | `[ ]` |
| | `extraVolumeMounts` | Can be used to mount extra volumes for the broker pods, useful for TLS and self-signed certificates | `[ ]` |
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
| | `nodeSelector` |  Can be used to define on which nodes the Operate pods should run | `{ }` |
| | `tolerations` |  Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ] ` |
| | `affinity` |  Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |

### Tasklist

Information about Tasklist you can find [here](https://docs.camunda.io/docs/components/tasklist/introduction/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `tasklist` | | Configuration for the Tasklist sub chart. | |
| | `enabled` | If true, the Tasklist deployment and its related resources are deployed via a helm release | `true` |
| | `image` | Configuration to configure the Tasklist image specifics. | |
| | `image.repository` | Defines which image repository to use. | `camunda/tasklist` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | `` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `podLabels` |  Can be used to define extra Tasklist pod labels | `{ }` |
| | `configMap.defaultMode` | Can be used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. See [Api docs](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L1615-L1623) for more details. It is useful to configure it if you want to run the helm charts in OpenShift. | [`0744`](https://chmodcommand.com/chmod-744/) |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` |
| | `service` | Configuration to configure the Tasklist service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.port` | Defines the port of the service, where the Tasklist web application will be available | `80` |
| | `graphqlPlaygroundEnabled` | If true, enables the GraphQl playground | `""` |
| | `graphqlPlaygroundEnabled` | Can be set to include the credentials in each request, should be set to "include" if GraphQl playground is enabled | `""` |
| | `podSecurityContext` | Defines the security options the Tasklist container should be run with | `{ }` |
| | `nodeSelector` |  Can be used to define on which nodes the Tasklist pods should run | `{ }` |
| | `tolerations` |  Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` |  Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 400m`<br> `  memory: 1Gi`<br>`limits:`<br> ` cpu: 1000m`<br> ` memory: 2Gi` |
| | `ingress` | Configuration to configure the ingress resource | |
| | `ingress.enabled` | If true, an ingress resource is deployed with the Tasklist deployment. Only useful if an ingress controller is available, like nginx. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Tasklist [service and port](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `env` | Can be used to set extra environment variables on each Tasklist container | `[ ]` |

### Optimize

Information about Optimize you can find [here](https://docs.camunda.io/docs/components/optimize/what-is-optimize/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `optimize` | |  Configuration for the Optimize sub chart. | |
| | `enabled` |  If true, the Optimize deployment and its related resources are deployed via a helm release | `true` |
| | `image` |  Configuration for the Optimize image specifics | |
| | `image.repository` |  Defines which image repository to use | `camunda/optimize` |
| | `image.tag` |  Can be set to overwrite the global tag, which should be used in that chart | `3.8.0` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `podLabels` |  Can be used to define extra Optimize pod labels | `{ }` |
| | `partitionCount` |  Defines how many Zeebe partitions are set up in the cluster and which should be imported by Optimize | `"3"` |
| | `env` |  Can be used to set extra environment variables in each Optimize container | `[]` |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` |
| | `extraVolumes` |  Can be used to define extra volumes for the Optimize pods, useful for tls and self-signed certificates | `[]` |
| | `extraVolumeMounts` |  Can be used to mount extra volumes for the Optimize pods, useful for tls and self-signed certificates | `[]` |
| | `serviceAccount` |  Configuration for the service account where the Optimize pods are assigned to | |
| | `serviceAccount.enabled` |  If true, enables the Optimize service account | `true` |
| | `serviceAccount.name` |  Can be used to set the name of the Optimize service account | `""` |
| | `serviceAccount.annotations` |  Can be used to set the annotations of the Optimize service account | `{}` |
| | `service` |  Configuration for the Optimize service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.port` | Defines the port of the service, where the Optimize web application will be available | `80` |
| | `service.annotations` |  Can be used to define annotations, which will be applied to the Optimize service | `{}` |
| | `podSecurityContext` |  Defines the security options the operate container should be run with | `{}` |
| | `nodeSelector` |  Can be used to define on which nodes the Optimize pods should run | `{}` |
| | `tolerations` |  Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` |  Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 600m`<br> `  memory: 1Gi`<br>`limits:`<br> ` cpu: 2000m`<br> ` memory: 2Gi` || | `ingress` |  Configuration to configure the ingress resource | |
| | `ingress.enabled` |  If true, an ingress resource is deployed with the Optimize deployment. Only useful if an ingress controller is available, like nginx. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Optimize [service and port](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |

### Identity

Information about Identity you can find [here](https://docs.camunda.io/docs/self-managed/identity/what-is-identity/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `identity`| |  Configuration for the Identity sub chart. | |
| | `enabled` |  If true, the Identity deployment and its related resources are deployed via a helm release. <br/> Note: Identity is required by Optimize. If Identity is disabled, then Optimize will be unusable. If you don't need Optimize, then make sure to disable both: set global.identity.auth.enabled=false AND optimize.enabled=false. | `true` |
| | `firstUser.username` | Defines the username of the first user, needed to log in into the web applications | `demo` |
| | `firstUser.password` | Defines the password of the first user, needed to log in into the web applications | `demo` |
| | `image` |  Configuration to configure the Identity image specifics | |
| | `image.repository` |  Defines which image repository to use | `camunda/identity` |
| | `image.tag` |   Can be set to overwrite the global.image.tag | |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `service` |  Configuration to configure the Identity service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.port` |  Defines the port of the service, where the Identity web application will be available | `80` |
| | `service.annotations` |  Can be used to define annotations, which will be applied to the Identity service | `{}` |
| | `resources` |  Configuration to set request and limit configuration for the container [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 600m`<br> `  memory: 400Mi`<br>`limits:`<br> ` cpu: 2000m`<br> ` memory: 2Gi` |
| | `env` |  Can be used to set extra environment variables in each Identity container | `[]` |
| | `command` |  Can be used to override the default command provided by the container image. See [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | |
| | `extraVolumes` |  Can be used to define extra volumes for the Identity pods, useful for tls and self-signed certificates | `[]` |
| | `extraVolumeMounts` |  Can be used to mount extra volumes for the Identity pods, useful for tls and self-signed certificates | `[]` |
| | `keycloak` |  Configuration for the Keycloak dependency chart which is used by Identity | |
| | `keycloak.auth` |  Authentication parameters - see [admin-credentials](https://github.com/bitnami/bitnami-docker-keycloak#admin-credentials) | |
| | `keycloak.auth.adminUser` |  Defines the Keycloak administrator user | 'admin' |
| | `keycloak.auth.existingSecret` |  Can be used to reuse an existing secret containing authentication information. See [manage-passwords](https://docs.bitnami.com/kubernetes/apps/keycloak/configuration/manage-passwords/) for more details. | `""` |
| | `serviceAccount` |  Configuration for the service account where the Identity pods are assigned to | |
| | `serviceAccount.enabled` |  If true, enables the Identity service account | `true` |
| | `serviceAccount.name` |  Can be used to set the name of the Identity service account | `` |
| | `serviceAccount.annotations` |  Can be used to set the annotations of the Identity service account | `{}` |
| | `ingress.enabled` | If true, an ingress resource is deployed with the Identity deployment. Only useful if an ingress controller is available, like nginx. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Identity service, - see [ingress-rules](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `ingress.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.tls.enabled` | If true, then TLS is configured on the ingress resource. If enabled the Ingress.host need to be defined. | `false` |
| | `ingress.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | `""` |
| | `podSecurityContext` | Defines the security options the Identity container should be run with | `{ }` |

### Elasticsearch

This chart has a dependency to the [Elasticsearch Helm Chart](https://github.com/elastic/helm-charts/blob/master/elasticsearch/README.md). All variables related to Elasticsearch which can be found [here](https://github.com/elastic/helm-charts/blob/main/elasticsearch/values.yaml) can be set under `elasticsearch`.

| Section | Parameter | Description | Default |
|-|-|-|-|
| `elasticsearch`| `enabled` | If true, enables Elasticsearch deployment as part of the Camunda Platform Helm chart | `true` |

**Example:**

```yaml
elasticsearch:
  enabled: true
  imageTag: <YOUR VERSION HERE>
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

## Development

For development purpose you might want to deploy and test the charts without creating a new helm chart release. In order to do this you can run the following:

```sh
 helm install <RELEASENAME> --atomic --debug charts/camunda-platform/
```

 * `--atomic if set, the installation process deletes the installation on failure. The --wait flag will be set automatically if --atomic is used`

 * `--debug enable verbose output`

To generate the resources / manifests without really install them you can use: 

 * `--dry-run simulate an install`

If you see errors like:

```sh
Error: found in Chart.yaml, but missing in charts/ directory: elasticsearch
```

Then you need to download the dependencies first.

Run the following to add resolve the dependencies:

```sh
helm repo add elastic https://helm.elastic.co
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
```

After this you can run: `make deps`, which will update and download the dependencies for all charts.

The execution should look like this:
```
$ make deps
helm dependency update charts/camunda-platform
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "camunda-platform" chart repository
...Successfully got an update from the "elastic" chart repository
...Successfully got an update from the "bitnami" chart repository
Update Complete. ⎈Happy Helming!⎈
Saving 6 charts
Dependency zeebe did not declare a repository. Assuming it exists in the charts directory
Dependency zeebe-gateway did not declare a repository. Assuming it exists in the charts directory
Dependency operate did not declare a repository. Assuming it exists in the charts directory
Dependency tasklist did not declare a repository. Assuming it exists in the charts directory
Dependency identity did not declare a repository. Assuming it exists in the charts directory
Downloading elasticsearch from repo https://helm.elastic.co
Deleting outdated charts
helm dependency update charts/camunda-platform/charts/identity
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "camunda-platform" chart repository
...Successfully got an update from the "elastic" chart repository
...Successfully got an update from the "bitnami" chart repository
Update Complete. ⎈Happy Helming!⎈
Saving 2 charts
Downloading keycloak from repo https://charts.bitnami.com/bitnami
Downloading common from repo https://charts.bitnami.com/bitnami
```

## Releasing the Charts

In order to find out how to release the charts please see the corresponding [release guide](/RELEASE.md).

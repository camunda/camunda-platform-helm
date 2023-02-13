# Camunda Platform 8 Helm Chart

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Test - Unit](https://github.com/camunda/camunda-platform-helm/actions/workflows/test-unit.yml/badge.svg)](https://github.com/camunda/camunda-platform-helm/actions/workflows/test-unit.yml)
[![Camunda Platform 8](https://img.shields.io/badge/dynamic/yaml?label=Camunda%20Platform&query=version&url=https%3A%2F%2Fraw.githubusercontent.com%2Fcamunda%2Fcamunda-platform-helm%2Fmain%2Fcharts%2Fcamunda-platform%2FChart.yaml?style=plastic&logo=artifacthub&logoColor=white&labelColor=417598&color=2D4857)](https://artifacthub.io/packages/helm/camunda/camunda-platform)

- [Architecture](#architecture)
- [Versioning](#versioning)
- [Requirements](#requirements)
- [Dependencies](#dependencies)
- [Installation](#installation)
  - [Local Kubernetes](#local-kubernetes)
  - [OpenShift](#openshift)
- [Uninstalling Charts](#uninstalling-charts)
- [Configuration](#configuration)
  - [Global](#global)
  - [Camunda Platform](#camunda-platform)
  - [Zeebe](#zeebe)
  - [Zeebe Gateway](#zeebe-gateway)
  - [Operate](#operate)
  - [Tasklist](#tasklist)
  - [Optimize](#optimize)
  - [Identity](#identity)
  - [Web Modeler (Beta)](#web-modeler-beta)
  - [Elasticsearch](#elasticsearch)
  - [Keycloak](#keycloak)
- [Guides](#guides)
  - [Adding dynamic exporters to Zeebe Brokers](#adding-dynamic-exporters-to-zeebe-brokers)
- [Development](#development)
- [Releasing the Charts](#releasing-the-charts)

## Architecture

<p align="center">
  <img
    alt="Camunda Platform 8 Self-Managed Helm charts architecture diagram"
    src="../../imgs/camunda-platform-8-self-managed-architecture-diagram-combined-ingress.png"
    width="80%"
  />
</p>

## Versioning

Camunda Platform 8 **Helm chart** versions are only aligned with the minor version of [Camunda Platform 8](https://github.com/camunda/camunda-platform).
In other words, the `Camunda Platform 8 Helm chart` could have a patch version different from the `Camunda Platform 8`.

For example, the Camunda Platform 8 Helm chart could be on version `8.1.1`, but Camunda Platform 8 apps are on version `8.1.0`. Additionally, the Camunda Platform 8 Helm chart could be on version `8.1.1`, but Camunda Platform 8 apps are on version `8.1.2`.

We work to keep the Helm chart updated with the latest version of Camunda Platform 8, but currently this is not guaranteed. Note that the latest version of the Helm chart may not necessarily have the latest version of the Camunda Platform 8 apps. This should not be an issue unless you rely on a specific Camunda Platform 8 patch version. In that case, you can set the desired version via a custom values file.

## Requirements

* [Helm](https://helm.sh/) >= 3.9.x
* Kubernetes >= 1.20+
* Minimum cluster requirements include the following to run this chart with default settings.
  All of these settings are configurable.
  * Three Kubernetes nodes to respect the default "hard" affinity settings
  * 2GB of RAM for the JVM heap

## Dependencies

Camunda Platform 8 Helm chart is an umbrella chart for different components. Some are internal (sub-charts),
and some are external (third-party). The dependency management is fully automated and managed by Helm itself;
however, it's good to understand the dependency structure. This third-party dependency is reflected in the Helm chart
as follows:

```
camunda-platform
  |_ elasticsearch
  |_ identity
    |_ keycloak
      |_ postgresql
  |_ optimize
  |_ operate
  |_ tasklist
  |_ web-modeler
    |_ postgresql
  |_ zeebe
```

For example, Camunda Identity utilizes Keycloak and allows you to manage users, roles, and permissions
for Camunda Platform 8 components.

- Keycloak is a dependency for Camunda Identity, and PostgreSQL is a dependency for Keycloak.
- PostgreSQL is an optional dependency for Web Modeler.
- Elasticsearch is a dependency for the Camunda Platform chart, which is used in Zeebe, Operate, Tasklist, and Optimize.

The values for the dependencies Keycloak and PostgreSQL can be set in the same hierarchy:

```yaml
identity:
  [identity values]
  keycloak:
    [keycloak values]
    postgresql:
      [postgresql values]
web-modeler:
  [web-modeler values]
  postgresql:
    [postgresql values]
```

## Installation

The first command adds the official Camunda Platform Helm charts repo, and the second installs the Camunda Platform
chart to your current Kubernetes context.

```shell
  helm repo add camunda https://helm.camunda.io
  helm install camunda-platform camunda/camunda-platform
```

### Local Kubernetes

We recommend using Helm on KIND for local environments, as the Helm configurations are battle-tested
and much closer to production systems.

For more details, follow the Camunda Platform 8
[local Kubernetes cluster guide](https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/guides/local-kubernetes-cluster/).

### OpenShift

Check out [OpenShift Support](openshift/README.md) to get started with deploying the charts on Red Hat OpenShift. 

## Uninstalling Charts

You can remove these charts by running:

```sh
helm uninstall YOUR_RELEASE_NAME
```

> **Note**
>
> Notice that all the Services and Pods will be deleted, but not the PersistentVolumeClaims (PVC)
> which are used to hold the storage for the data generated by the cluster and Elasticsearch.

To free up the storage, you need to delete all the PVCs manually.

First, view the PVCs:

```sh
kubectl get pvc -l app.kubernetes.io/instance=YOUR_RELEASE_NAME
kubectl get pvc -l release=YOUR_RELEASE_NAME
```

Then delete the ones that you don't want to keep:

```sh
kubectl delete pvc -l app.kubernetes.io/instance=YOUR_RELEASE_NAME
kubectl delete pvc -l release=YOUR_RELEASE_NAME
```

Or you can delete the related Kubernetes namespace, which contains all PVCs.

## Configuration

The following sections contain the configuration values for the chart and each sub-chart. All of them can be overwritten
via a separate `values.yaml` file.

Check out the default [values.yaml](values.yaml) file, which contains the same content and documentation.

> **Note**
> For more details about deploying Camunda Platform 8 on Kubernetes, please visit the
> [Helm/Kubernetes installation instructions docs](https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/overview/).

### Global 

| Section | Parameter | Description | Default |
|-|-|-|-|
| `global` | | Global variables which can be accessed by all sub charts | |
| | `annotations` | Can be used to define common annotations, which should be applied to all deployments | |
| | `labels` | Can be used to define common labels, which should be applied to all deployments | |
| | `image.tag` | Defines the tag / version which should be used in the chart. | |
| | `image.pullPolicy` | Defines the [image pull policy](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy) which should be used. | IfNotPresent |
| | `image.pullSecrets` | Can be used to configure [image pull secrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod). Also it could be configured per component separately. | `[]` |
| | `ingress` | Configuration to configure the ingress resource | |
| | `ingress.enabled` | If true, an ingress resource is deployed. Only useful if an ingress controller is available, like Ingress-NGINX. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` <br/> `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"` |
| | `ingress.path` | Defines the path which is used as a base for all services | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `ingress.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.tls.enabled` | If true, then TLS is configured on the ingress resource. If enabled the Ingress.host needs to be defined. | `false` |
| | `ingress.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | `""` |
| | `elasticsearch.disableExporter` | If true, disables the [Elasticsearch Exporter](https://github.com/camunda-cloud/zeebe/tree/develop/exporters/elasticsearch-exporter) in Zeebe | `false` |
| | `elasticsearch.url` | Can be used to configure the URL to access Elasticsearch. When not set, services fallback to host and port configuration. | |
| | `elasticsearch.protocol` | Defines the elasticsearch access protocol, by default HTTP. | `http` |
| | `elasticsearch.host` | Defines the Elasticsearch host, ideally the service name inside the namespace. | `elasticsearch-master` |
| | `elasticsearch.port` | Defines the Elasticsearch port, under which Elasticsearch can be accessed | `9200` |
| | `elasticsearch.clusterName` | Defines the cluster name which is used by Elasticsearch. | `elasticsearch` |
| | `elasticsearch.prefix` | Defines the prefix which is used by the Zeebe Elasticsearch Exporter to create Elasticsearch indexes | `zeebe-record` |
| | `zeebeClusterName` | Defines the cluster name for the Zeebe cluster. All pods get this prefix in their name. | `{{ .Release.Name }}-zeebe` |
| | `zeebePort` | Defines the port which is used for the Zeebe Gateway. This port accepts the GRPC Client messages and forwards them to the Zeebe Brokers. | 26500 |
| | `identity.fullnameOverride` | can be used to override the full name of the identity resources | |
| | `identity.nameOverride` | can be used to partly override the name of the identity resources (names will still be prefixed with the release name) | |
| | `identity.service.port` | defines the port of the service on which the identity application will be available | `80` |
| | `identity.auth.enabled` |  If true, enables the Identity authentication otherwise basic-auth will be used on all services. | `true` |
| | `identity.auth.publicIssuerUrl` | Defines the token issuer (Keycloak) URL, where the services can request JWT tokens. Should be publicly accessible, per default we assume a port-forward to Keycloak (18080) is created before login. Can be overwritten if an Ingress is in use and an external IP is available. | `"http://localhost:18080/auth/realms/camunda-platform"` |
| | `identity.auth.operate.existingSecret` |  Can be used to reference an existing secret. If not set, a random secret is generated. The existing secret should contain an `operate-secret` field, which will be used as secret for the Identity-Operate communication. | `` |
| | `identity.auth.operate.redirectUrl` |  Defines the redirect URL, which is used by Keycloak to access Operate. Should be publicly accessible, the default value works if a port-forward to operate is created to 8081. Can be overwritten if an Ingress is in use and an external IP is available. | `"http://localhost:8081"` |
| | `identity.auth.tasklist.existingSecret` |  Can be used to reference an existing secret. If not set, a random secret is generated. The existing secret should contain an `tasklist-secret` field, which will be used as secret for the Identity-Tasklist communication. | ` ` |
| | `identity.auth.tasklist.redirectUrl` |  Defines the redirect URL, which is used by Keycloak to access Tasklist. Should be publicly accessible, the default value works if a port-forward to Tasklist is created to 8082. Can be overwritten if an Ingress is in use and an external IP is available. | `"http://localhost:8082"` |
| | `identity.auth.optimize.existingSecret` |  Can be used to reference an existing secret. If not set, a random secret is generated. The existing secret should contain an `optimize-secret` field, which will be used as secret for the Identity-Optimize communication. | ` ` |
| | `identity.auth.optimize.redirectUrl` |  Defines the redirect URL, which is used by Keycloak to access Optimize. Should be publicly accessible, the default value works if a port-forward to Optimize is created to 8083. Can be overwritten if an Ingress is in use and an external IP is available. | `"http://localhost:8083"` |
| | `identity.auth.webModeler.redirectUrl` | Defines the root URL which is used by Keycloak to access Web Modeler. Should be publicly accessible, the default value works if a port-forward to Web Modeler is created to 8084. Can be overwritten if an Ingress is in use and an external IP is available. | `"http://localhost:8084"` |
| | `identity.keycloak.legacy` | If true, it will configure Keycloak service name according to Keycloak v16. If false, then it will configure Keycloak service name according to Keycloak v19. This config is used when Keycloak v19 Helm chart is used. Note: This is just for config, it will not enable Keycloak v19). | `""` |
| | `identity.keycloak.internal` | If true, it will configure an extra service with type "ExternalName". It's useful for using existing Keycloak in another namespace with and access it with the combined Ingress. | `false` |
| | `identity.keycloak.fullname` |    Can be used to change the referenced Keycloak service name inside the sub-charts, like operate, optimize, etc. Subcharts can't access values from other sub-charts or the parent, global only. This is useful if the `identity.keycloak.fullnameOverride` is set, and specifies a different name for the Keycloak service | `""` |
| | `identity.keycloak.url` |    Can be used to customize the Identity Keycloak chart URL when "identity.keycloak.enabled: true", or to use already existing Keycloak instead of the one comes with Camunda Platform Helm chart when "identity.keycloak.enabled: false" | `{}` |
| | `identity.keycloak.url.protocol` |    Can be used to set existing Keycloak URL protocol | `` |
| | `identity.keycloak.url.host` |    Can be used to set existing Keycloak URL host | `` |
| | `identity.keycloak.url.port` |    Can be used to set existing Keycloak URL port | `` |
| | `identity.keycloak.contextPath` |   Defines the endpoint of Keycloak which varies between Keycloak versions. In Keycloak v16.x.x it's hard-coded as '/auth', but in v19.x.x it's '/' | `"/auth"` |
| | `identity.keycloak.realm` |   Defines Keycloak realm path used for Camunda Platform. | `"/realms/camunda-platform"` |
| | `identity.keycloak.auth` |   Can be used incorporate with "global.identity.keycloak.url" and "identity.keycloak.enabled: false" set existing Keycloak URL instead of the one comes with Camunda Platform Helm chart | `{}` |
| | `identity.keycloak.auth.adminUser` |    Can be used to configure admin user to access existing Keycloak | `""` |
| | `identity.keycloak.auth.existingSecret` |    Can be used to configure existing Secret object which has admin password to access existing Keycloak | `""` |
| | `identity.keycloak.auth.existingSecretKey` |    Can be used to configure the key inside existing Secret object which has admin password to access existing Keycloak | `"admin-password"` |
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
| | `image.registry` | Can be used to set container image registry. | `""` |
| | `image.repository` | Defines which image repository to use. | `bitnami/elasticsearch-curator` |
| | `image.tag` | Defines the tag / version which should be used in the chart. | `5.8.4` |
| `prometheusServiceMonitor` | | Configuration to configure a prometheus service monitor | |
| | `enabled` | If true, then a service monitor will be deployed, which allows an installed prometheus controller to scrape metrics from the deployed pods. | `false`|
| | `labels` | Can be set to configure extra labels, which will be added to the ServiceMonitor and can be used on the prometheus controller for selecting the ServiceMonitors | `release: metrics` |
| | `scrapeInterval` | Can be set to configure the interval at which metrics should be scraped. Should be *less* than 60s if the provided grafana dashboard is used, which can be found in [zeebe/monitor/grafana](https://github.com/camunda/zeebe/tree/main/monitor/grafana), otherwise it isn't able to show any metrics which is aggregated over 1 min. | `10s` |

### Zeebe

For more information about Zeebe, visit [Zeebe Overview](https://docs.camunda.io/docs/components/zeebe/zeebe-overview/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `zeebe` | Configuration for the Zeebe sub chart. Contains configuration for the Zeebe broker and related resources. | |
| | `image` | Configuration to configure the Zeebe image specifics. | |
| | `image.registry` | Can be used to set container image registry. | `""` |
| | `image.repository` | Defines which image repository to use. | `camunda/zeebe` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | ` ` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `clusterSize` | Defines the amount of brokers (=replicas), which are deployed via helm | `3` |
| | `partitionCount` | Defines how many Zeebe partitions are set up in the cluster | `3` |
| | `replicationFactor` | Defines how each partition is replicated, the value defines the number of nodes | `3` |
| | `env` | Can be used to set extra environment variables in each Zeebe broker container | `- name: ZEEBE_BROKER_DATA_SNAPSHOTPERIOD` </br>`  value: "5m"`</br>`- name: ZEEBE_BROKER_EXECUTION_METRICS_EXPORTER_ENABLED`</br>`  value: "true"`</br>`- name: ZEEBE_BROKER_DATA_DISKUSAGECOMMANDWATERMARK`</br>`  value: "0.85"`</br>`- name: ZEEBE_BROKER_DATA_DISKUSAGEREPLICATIONWATERMARK`</br>`  value: "0.87"` |
| | `configMap.defaultMode` | Can be used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. See [Api docs](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L1615-L1623) for more details. It is useful to configure it if you want to run the helm charts in OpenShift. | [`0754`](https://chmodcommand.com/chmod-0754/) |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` | 
| | `logLevel` | Defines the log level which is used by the Zeebe brokers; must be one of: ERROR, WARN, INFO, DEBUG, TRACE | `info` |
| | `log4j2` | Can be used to overwrite the Log4J 2.x XML configuration. If provided, the contents given will be written to file and will overwrite the distribution's default `/usr/local/zeebe/config/log4j2.xml` | ` ` |
| | `javaOpts` | Can be used to set the Zeebe Broker javaOpts. This is where you should configure the jvm heap size. | `-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/zeebe/data -XX:ErrorFile=/usr/local/zeebe/data/zeebe_error%p.log -XX:+ExitOnOutOfMemoryError` |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.httpPort` | Defines the port of the HTTP endpoint, where for example metrics are provided | `9600` |
| | `service.httpName` | Defines the name of the HTTP endpoint, where for example metrics are provided | `http` |
| | `service.commandPort` | Defines the port of the Command API endpoint, where the broker commands are sent to | `26501` |
| | `service.commandName` | Defines the name of the Command API endpoint, where the broker commands are sent to | `command` |
| | `service.internalPort` | Defines the port of the Internal API endpoint, which is used for internal communication | `26502` |
| | `service.internalName` | Defines the name of the Internal API endpoint, which is used for internal communication | `internal` |
| | `service.extraPorts` |  Exposes any other [ports](https://kubernetes.io/docs/concepts/services-networking/service/#multi-port-services) which are required. Can be useful for exporters | `[]` |
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
| | `podSecurityContext` | Defines the security options the Zeebe broker pod should be run with | `{ }` |
| | `containerSecurityContext` | Defines the security options the Zeebe broker container should be run with | `{ }` |
| | `startupProbe` | StartupProbe configuration | |
| | `startupProbe.enabled` | If true, the startup probe is enabled in app container | `true` |
| | `startupProbe.probePath` | Defines the startup probe route used on the app | `/startup` |
| | `startupProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `startupProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `startupProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `startupProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `startupProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `readinessProbe` | ReadinessProbe configuration | |
| | `readinessProbe.enabled` | If true, the readiness probe is enabled in app container | `true` |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the app | `/ready` |
| | `readinessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `30` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `livenessProbe` | LivenessProbe configuration | |
| | `livenessProbe.enabled` | If true, the liveness probe is enabled in app container | `false` |
| | `livenessProbe.probePath` | Defines the liveness probe route used on the app | `/health` |
| | `livenessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `livenessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `livenessProbe.successThreshold` | Defines how often it needs to be true to be considered successful after having failed | `1` |
| | `livenessProbe.failureThreshold` | Defines when the probe is considered as failed so the container will be restarted | `5` |
| | `livenessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `nodeSelector` | Can be used to define on which nodes the broker pods should run | `{ } ` |
| | `tolerations` | Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` | Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity). The default defined PodAntiAffinity allows constraining on which nodes the [Zeebe pods are scheduled on](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity). It uses a hard requirement for scheduling and works based on the Zeebe pod labels. To disable the default rule set `podAntiAffinity: null`. | `podAntiAffinity:`</br>`  requiredDuringSchedulingIgnoredDuringExecution:`</br>`  - labelSelector: `</br>`    matchExpressions:`</br>`    - key: "app.kubernetes.io/component"`</br>`    operator: In`</br>`    values:`</br>`    - zeebe-broker`</br>`  topologyKey: "kubernetes.io/hostname"` |
| | `priorityClassName` | Can be used to define the broker [pods priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass) | `""` |

### Zeebe Gateway

For more information about Zeebe Gateway, visit
[Zeebe Gateway Overview](https://docs.camunda.io/docs/components/zeebe/technical-concepts/architecture/#gateways).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `zeebe-gateway`| | Configuration to define properties related to the Zeebe standalone gateway | |
| | `replicas` | Defines how many standalone gateways are deployed | `1` |
| | `image` | Configuration to configure the zeebe-gateway image specifics. | |
| | `image.registry` | Can be used to set container image registry. | `""` |
| | `image.repository` | Defines which image repository to use. | `camunda/zeebe` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | ` ` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `podAnnotations` | Can be used to define extra gateway pod annotations | `{ }` |
| | `podLabels` | Can be used to define extra gateway pod labels | `{ }` |
| | `logLevel` | Defines the log level which is used by the gateway | `info` |
| | `log4j2` | Can be used to overwrite the log4j2 configuration of the gateway | `""` |
| | `javaOpts` | Can be used to set the Zeebe Gateway javaOpts. This is where you should configure the jvm heap size. | `-XX:+ExitOnOutOfMemoryError` |
| | `env` | Can be used to set extra environment variables in each gateway container | `[ ]` |
| | `configMap.defaultMode` | Can be used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. See [Api docs](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L1615-L1623) for more details. It is useful to configure it if you want to run the helm charts in OpenShift. | [`0744`](https://chmodcommand.com/chmod-744/) |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` |
| | `podDisruptionBudget` | Configuration to configure a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) for the broker pods. | |
| | `podDisruptionBudget.enabled` | If true a pod disruption budget is defined for the brokers | `false` |
| | `podDisruptionBudget.minAvailable` | Can be used to set how many pods should be available. Be aware that if minAvailable is set, maxUnavailable will not be set (they are mutually exclusive). | `` |
| | `podDisruptionBudget.maxUnavailable` | Can be used to set how many pods should be at max. unavailable | `1` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 400m`<br> `  memory: 450Mi`<br>`limits:`<br>  ` cpu: 400m`<br>  ` memory: 450Mi` |
| | `priorityClassName` | Can be used to define the broker [pods priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass) | `""` |
| | `podSecurityContext` | Defines the security options the Zeebe Gateway pod should be run with | `{ }` |
| | `containerSecurityContext` | Defines the security options the Zeebe Gateway container should be run with | `{ }` |
| | `startupProbe` | StartupProbe configuration | |
| | `startupProbe.enabled` | If true, the startup probe is enabled in app container | `true` |
| | `startupProbe.probePath` | Defines the startup probe route used on the app | `/actuator/health/startup` |
| | `startupProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `startupProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `startupProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `startupProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `startupProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `readinessProbe` | ReadinessProbe configuration | |
| | `readinessProbe.enabled` | If true, the readiness probe is enabled in app container | `false` |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the app | `/actuator/health` |
| | `readinessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `30` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `livenessProbe` | LivenessProbe configuration | |
| | `livenessProbe.enabled` | If true, the liveness probe is enabled in app container | `false` |
| | `livenessProbe.probePath` | Defines the liveness probe route used on the app | `/actuator/health/liveness` |
| | `livenessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `livenessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `livenessProbe.successThreshold` | Defines how often it needs to be true to be considered successful after having failed | `1` |
| | `livenessProbe.failureThreshold` | Defines when the probe is considered as failed so the container will be restarted | `5` |
| | `livenessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
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
| | `ingress` | Configuration to configure the ingress resource | |
| | `ingress.enabled` | If true, an ingress resource is deployed with the Zeebe gateway deployment. Only useful if an ingress controller is available, like Ingress-NGINX. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` <br/> `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"` |
| | `ingress.path` | Defines the path which is associated with the Zeebe Gateway [service and port](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `ingress.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.tls.enabled` | If true, then TLS is configured on the ingress resource. If enabled the Ingress.host needs to be defined. | `false` |
| | `ingress.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | `""` |

### Operate

For more information about Operate, visit
[Operate Introduction](https://docs.camunda.io/docs/components/operate/operate-introduction/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `operate` | | Configuration for the Operate sub chart. | |
| | `enabled` | If true, the Operate deployment and its related resources are deployed via a helm release | `true` |
| | `image` | Configuration to configure the Operate image specifics. | |
| | `image.registry` | Can be used to set container image registry. | `""` |
| | `image.repository` | Defines which image repository to use. | `camunda/operate` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | `` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `contextPath` |  Can be used to make Operate web application works on a custom sub-path. This is mainly used to run Camunda Platform web applications under a single domain. | |
| | `podAnnotations` | Can be used to define extra Operate pod annotations | `{ }` |
| | `podLabels` |  Can be used to define extra Operate pod labels | `{ }` |
| | `logging` | Configuration for the Operate logging. This template will be directly included in the Operate configuration yaml file | `level:` <br/> `ROOT: INFO` <br/> `io.camunda.operate: DEBUG` |
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
| | `ingress.enabled` | If true, an ingress resource is deployed with the Operate deployment. Only useful if an ingress controller is available, like Ingress-NGINX. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Operate [service and port](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `ingress.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.tls.enabled` | If true, then TLS is configured on the ingress resource. If enabled the Ingress.host needs to be defined. | `false` |
| | `ingress.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | `""` |
| | `podSecurityContext` | Defines the security options the Operate pod should be run with | `{ }` |
| | `containerSecurityContext` | Defines the security options the Operate container should be run with | `{ }` |
| | `startupProbe` | StartupProbe configuration | |
| | `startupProbe.enabled` | If true, the startup probe is enabled in app container | `true` |
| | `startupProbe.probePath` | Defines the startup probe route used on the app | `/ready` |
| | `startupProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `startupProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `startupProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `startupProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `startupProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `readinessProbe` | ReadinessProbe configuration | |
| | `readinessProbe.enabled` | If true, the readiness probe is enabled in app container | `false` |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the app | `/actuator/health/readiness` |
| | `readinessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `30` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `livenessProbe` | LivenessProbe configuration | |
| | `livenessProbe.enabled` | If true, the liveness probe is enabled in app container | `false` |
| | `livenessProbe.probePath` | Defines the liveness probe route used on the app | `/actuator/health/liveness` |
| | `livenessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `livenessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `livenessProbe.successThreshold` | Defines how often it needs to be true to be considered successful after having failed | `1` |
| | `livenessProbe.failureThreshold` | Defines when the probe is considered as failed so the container will be restarted | `5` |
| | `livenessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `nodeSelector` |  Can be used to define on which nodes the Operate pods should run | `{ }` |
| | `tolerations` |  Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ] ` |
| | `affinity` |  Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |

### Tasklist

For more information about Tasklist, visit
[Tasklist Introduction](https://docs.camunda.io/docs/components/tasklist/introduction-to-tasklist/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `tasklist` | | Configuration for the Tasklist sub chart. | |
| | `enabled` | If true, the Tasklist deployment and its related resources are deployed via a helm release | `true` |
| | `image` | Configuration to configure the Tasklist image specifics. | |
| | `image.registry` | Can be used to set container image registry. | `""` |
| | `image.repository` | Defines which image repository to use. | `camunda/tasklist` |
| | `image.tag` | Can be set to overwrite the global tag, which should be used in that chart. | `` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `contextPath` |  Can be used to make Tasklist web application works on a custom sub-path. This is mainly used to run Camunda Platform web applications under a single domain. | |
| | `podAnnotations` | Can be used to define extra Tasklist pod annotations | `{ }` |
| | `podLabels` |  Can be used to define extra Tasklist pod labels | `{ }` |
| | `configMap.defaultMode` | Can be used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. See [Api docs](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L1615-L1623) for more details. It is useful to configure it if you want to run the helm charts in OpenShift. | [`0744`](https://chmodcommand.com/chmod-744/) |
| | `command` | Can be used to [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | `[]` |
| | `service` | Configuration to configure the Tasklist service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.port` | Defines the port of the service, where the Tasklist web application will be available | `80` |
| | `graphqlPlaygroundEnabled` | If true, enables the GraphQl playground | `""` |
| | `graphqlPlaygroundEnabled` | Can be set to include the credentials in each request, should be set to "include" if GraphQl playground is enabled | `""` |
| | `extraVolumes` | Can be used to define extra volumes for the Tasklist pods, useful for tls and self-signed certificates | `[]` |
| | `extraVolumeMounts` | Can be used to mount extra volumes for the Tasklist pods, useful for tls and self-signed certificates | `[]` |
| | `podSecurityContext` | Defines the security options the Tasklist pod should be run with | `{ }` |
| | `containerSecurityContext` | Defines the security options the Tasklist container should be run with | `{ }` |
| | `startupProbe` | StartupProbe configuration | |
| | `startupProbe.enabled` | If true, the startup probe is enabled in app container | `true` |
| | `startupProbe.probePath` | Defines the startup probe route used on the app | `/ready` |
| | `startupProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `startupProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `startupProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `startupProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `startupProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `readinessProbe` | ReadinessProbe configuration | |
| | `readinessProbe.enabled` | If true, the readiness probe is enabled in app container | `false` |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the app | `/actuator/health/readiness` |
| | `readinessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `30` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `livenessProbe` | LivenessProbe configuration | |
| | `livenessProbe.enabled` | If true, the liveness probe is enabled in app container | `false` |
| | `livenessProbe.probePath` | Defines the liveness probe route used on the app | `/actuator/health/liveness` |
| | `livenessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `livenessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `livenessProbe.successThreshold` | Defines how often it needs to be true to be considered successful after having failed | `1` |
| | `livenessProbe.failureThreshold` | Defines when the probe is considered as failed so the container will be restarted | `5` |
| | `livenessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `nodeSelector` |  Can be used to define on which nodes the Tasklist pods should run | `{ }` |
| | `tolerations` |  Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` |  Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 400m`<br> `  memory: 1Gi`<br>`limits:`<br> ` cpu: 1000m`<br> ` memory: 2Gi` |
| | `ingress` | Configuration to configure the ingress resource | |
| | `ingress.enabled` | If true, an ingress resource is deployed with the Tasklist deployment. Only useful if an ingress controller is available, like Ingress-NGINX. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Tasklist [service and port](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `env` | Can be used to set extra environment variables on each Tasklist container | `[ ]` |

### Optimize

For more information, visit [Optimize Introduction](https://docs.camunda.io/optimize/components/what-is-optimize/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `optimize` | |  Configuration for the Optimize sub chart. | |
| | `enabled` |  If true, the Optimize deployment and its related resources are deployed via a helm release | `true` |
| | `image` |  Configuration for the Optimize image specifics | |
| | `image.registry` | Can be used to set container image registry. | `""` |
| | `image.repository` |  Defines which image repository to use | `camunda/optimize` |
| | `image.tag` |  Can be set to overwrite the global tag, which should be used in that chart | `3.8.0` |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `contextPath` |  Can be used to make Optimize web application works on a custom sub-path. This is mainly used to run Camunda Platform web applications under a single domain. | |
| | `podAnnotations` | Can be used to define extra Optimize pod annotations | `{ }` |
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
| | `service.managementPort` | Defines the management port, where actuator and the backupAPI will be available | `8092` |
| | `service.annotations` |  Can be used to define annotations, which will be applied to the Optimize service | `{}` |
| | `podSecurityContext` | Defines the security options the Optimize pod should be run with | `{ }` |
| | `containerSecurityContext` | Defines the security options the Optimize container should be run with | `{ }` |
| | `startupProbe` | StartupProbe configuration | |
| | `startupProbe.enabled` | If true, the startup probe is enabled in app container | `true` |
| | `startupProbe.probePath` | Defines the startup probe route used on the app | `/api/readyz` |
| | `startupProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `startupProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `startupProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `startupProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `startupProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `readinessProbe` | ReadinessProbe configuration | |
| | `readinessProbe.enabled` | If true, the readiness probe is enabled in app container | `false` |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the app | `/api/readyz` |
| | `readinessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `30` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `livenessProbe` | LivenessProbe configuration | |
| | `livenessProbe.enabled` | If true, the liveness probe is enabled in app container | `false` |
| | `livenessProbe.probePath` | Defines the liveness probe route used on the app | `/api/readyz` |
| | `livenessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `livenessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `livenessProbe.successThreshold` | Defines how often it needs to be true to be considered successful after having failed | `1` |
| | `livenessProbe.failureThreshold` | Defines when the probe is considered as failed so the container will be restarted | `5` |
| | `livenessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `nodeSelector` |  Can be used to define on which nodes the Optimize pods should run | `{}` |
| | `tolerations` |  Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ]` |
| | `affinity` |  Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |
| | `resources` | Configuration to set [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 600m`<br> `  memory: 1Gi`<br>`limits:`<br> ` cpu: 2000m`<br> ` memory: 2Gi` || | `ingress` |  Configuration to configure the ingress resource | |
| | `ingress.enabled` |  If true, an ingress resource is deployed with the Optimize deployment. Only useful if an ingress controller is available, like Ingress-NGINX. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Optimize [service and port](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `ingress.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.tls.enabled` | If true, then TLS is configured on the ingress resource. If enabled the Ingress.host needs to be defined. | `false` |
| | `ingress.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | `""` |

### Identity

For more information, visit [Identity Overview](https://docs.camunda.io/docs/self-managed/identity/what-is-identity/).

| Section | Parameter | Description | Default |
|-|-|-|-|
| `identity`| |  Configuration for the Identity sub chart. | |
| | `enabled` |  If true, the Identity deployment and its related resources are deployed via a helm release. <br/> Note: Identity is required by Optimize and Web Modeler. If Identity is disabled, both Optimize and Web Modeler will be unusable. If you need neither Optimize nor Web Modeler, make sure to disable both the Identity authentication and the applications by setting:<br/>`global.identity.auth.enabled: false`<br/>`optimize.enabled: false`<br/>`web-modeler.enabled: false` | `true` |
| | `firstUser.username` | Defines the username of the first user, needed to log in into the web applications | `demo` |
| | `firstUser.password` | Defines the password of the first user, needed to log in into the web applications | `demo` |
| | `firstUser.email` | Defines the email address of the first user; a valid email address is required to use Web Modeler | `demo@example.org` |
| | `firstUser.firstName` | Defines the first name of the first user; a name is required to use Web Modeler | `Demo` |
| | `firstUser.lastName` | Defines the last name of the first user; a name is required to use Web Modeler | `User` |
| | `image` |  Configuration to configure the Identity image specifics | |
| | `image.registry` | Can be used to set container image registry. | `""` |
| | `image.repository` |  Defines which image repository to use | `camunda/identity` |
| | `image.tag` |   Can be set to overwrite the global.image.tag | |
| | `image.pullSecrets` | Can be set to overwrite the global.image.pullSecrets | `{{ global.image.pullSecrets }}` |
| | `fullURL` |  Can be used when Ingress is configured (for both multi and single domain setup). <br/> Note: If the `ContextPath` is configured, then value of `ContextPath` should be included in the fullURL too. | |
| | `contextPath` |  Can be used to make Identity web application works on a custom sub-path. This is mainly used to run Camunda Platform web applications under a single domain. **Note:** Identity cannot be accessed over HTTP if a "contextPath" is configured. Which means that Identity cannot be configured in combined Ingress without HTTPS. To use Identity over HTTP, setup a separated Ingress using "identity.ingress" and don't set "contextPath". | `` |
| | `podAnnotations` | Can be used to define extra Identity pod annotations | `{ }` |
| | `service` |  Configuration to configure the Identity service. | |
| | `service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `service.annotations` |  Can be used to define annotations, which will be applied to the Identity service | `{}` |
| | `service.port` | Defines the port of the service on which the identity application will be available | `80` |
| | `service.metricsPort` | Defines the port of the service on which the identity metrics will be available | `82` |
| | `service.metricsName` | Defines the name of the service on which the identity metrics will be available | `metrics` |
| | `resources` |  Configuration to set request and limit configuration for the container [request and limit configuration for the container](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) | `requests:`<br>`  cpu: 600m`<br> `  memory: 400Mi`<br>`limits:`<br> ` cpu: 2000m`<br> ` memory: 2Gi` |
| | `nodeSelector` |  Can be used to define on which nodes the Identity pods should run | `{ }` |
| | `tolerations` |  Can be used to define [pod toleration's](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[ ] ` |
| | `affinity` |  Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{ }` |
| | `env` |  Can be used to set extra environment variables in each Identity container. See the [documentation](https://docs.camunda.io/docs/self-managed/identity/deployment/configuration-variables/) for more details. | `[]` |
| | `command` |  Can be used to override the default command provided by the container image. See [override the default command provided by the container image](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) | |
| | `extraVolumes` |  Can be used to define extra volumes for the Identity pods, useful for tls and self-signed certificates | `[]` |
| | `extraVolumeMounts` |  Can be used to mount extra volumes for the Identity pods, useful for tls and self-signed certificates | `[]` |
| | `keycloak` |  Configuration for the Keycloak dependency chart which is used by Identity. See the chart [documentation](https://github.com/bitnami/charts/tree/master/bitnami/keycloak#parameters) for more details. | |
| | `keycloak.auth` |  Authentication parameters - see [admin-credentials](https://github.com/bitnami/bitnami-docker-keycloak#admin-credentials) | |
| | `keycloak.auth.adminUser` |  Defines the Keycloak administrator user | 'admin' |
| | `keycloak.auth.existingSecret` |  Can be used to reuse an existing secret containing authentication information. See [manage-passwords](https://docs.bitnami.com/kubernetes/apps/keycloak/configuration/manage-passwords/) for more details. | `""` |
| | `serviceAccount` |  Configuration for the service account where the Identity pods are assigned to | |
| | `serviceAccount.enabled` |  If true, enables the Identity service account | `true` |
| | `serviceAccount.name` |  Can be used to set the name of the Identity service account | `` |
| | `serviceAccount.annotations` |  Can be used to set the annotations of the Identity service account | `{}` |
| | `ingress.enabled` | If true, an ingress resource is deployed with the Identity deployment. Only useful if an ingress controller is available, like Ingress-NGINX. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"` <br/> `nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.path` | Defines the path which is associated with the Identity service, - see [ingress-rules](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) | `/` |
| | `ingress.host` | Can be used to define the [host of the ingress rule.](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules) If not specified the rules applies to all inbound HTTP traffic, if specified the rule applies to that host. | `""` |
| | `ingress.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.tls.enabled` | If true, then TLS is configured on the ingress resource. If enabled the Ingress.host needs to be defined. | `false` |
| | `ingress.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | `""` |
| | `podSecurityContext` | Defines the security options the Identity pod should be run with | `{ }` |
| | `containerSecurityContext` | Defines the security options the Identity container should be run with | `{ }` |
| | `startupProbe` | StartupProbe configuration | |
| | `startupProbe.enabled` | If true, the startup probe is enabled in app container | `true` |
| | `startupProbe.probePath` | Defines the startup probe route used on the app | `/ready` |
| | `startupProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `startupProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `startupProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `startupProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `startupProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `readinessProbe` | ReadinessProbe configuration | |
| | `readinessProbe.enabled` | If true, the readiness probe is enabled in app container | `false` |
| | `readinessProbe.probePath` | Defines the readiness probe route used on the app | `/actuator/health` |
| | `readinessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `30` |
| | `readinessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `readinessProbe.successThreshold` | Defines how often it needs to be true to be marked as ready, after failure | `1` |
| | `readinessProbe.failureThreshold` | Defines when the probe is considered as failed so the Pod will be marked Unready | `5` |
| | `readinessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |
| | `livenessProbe` | LivenessProbe configuration | |
| | `livenessProbe.enabled` | If true, the liveness probe is enabled in app container | `false` |
| | `livenessProbe.probePath` | Defines the liveness probe route used on the app | `/ready` |
| | `livenessProbe.initialDelaySeconds` | Defines the number of seconds after the container has started before the probe is initiated | `5` |
| | `livenessProbe.periodSeconds` | Defines how often the probe is executed | `30` |
| | `livenessProbe.successThreshold` | Defines how often it needs to be true to be considered successful after having failed | `1` |
| | `livenessProbe.failureThreshold` | Defines when the probe is considered as failed so the container will be restarted | `5` |
| | `livenessProbe.timeoutSeconds` | Defines the seconds after the probe times out | `1` |

### Web Modeler (Beta)

> :warning: Web Modeler Self-Managed is currently offered as a [beta release](https://docs.camunda.io/docs/next/reference/early-access#beta) with limited availability for enterprise customers only. It is not recommended for production use, and there is no maintenance service guaranteed. Special [terms & conditions](https://camunda.com/legal/terms/camunda-platform/camunda-platform-8-self-managed/) apply. However, we encourage you to provide feedback via your designated support channel or the [Camunda Forum](https://forum.camunda.io/).

#### Docker registry
The Docker images for Web Modeler Beta are available in a private registry.
Enterprise customers either already have credentials to this registry, or they can request access to this registry through their CSM contact at Camunda.
To enable Kubernetes to pull the images from Camunda's registry, you'll need to:
- [create an image pull secret](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod) using the provided credentials
- configure the Web Modeler pods to use the secret:
  ```yaml
  web-modeler:
    image:
      pullSecrets:
        - name: <MY_SECRET_NAME>
   ```

#### Database
Web Modeler requires a PostgreSQL database to store the data.
You can either:
- deploy a PostgreSQL instance as part of the Helm release by setting `postgresql.enabled` to `true` (which will enable the `postgresql` chart dependency); this is the default setting
- configure a connection to an (existing) external database by setting `postgresql.enabled` to `false` and providing the values under `restapi.externalDatabase`

#### SMTP server
Web Modeler requires an SMTP server to send (notification) emails to users.
The SMTP connection can be configured with the values under `restapi.mail`.

#### Ingress
Running Web Modeler on a context path (like https://example.com/modeler) is not yet supported.
That is why the [global](#global) Ingress resource does not contain a rule for Web Modeler.
You can configure a separate Ingress resource for Web Modeler though using the values under `ingress` below.

#### Configuration values
| Section | Parameter | Description | Default |
|-|-|-|-|
| `web-modeler` | | Configuration of the Web Modeler subchart | |
| | `enabled` | If `true`, the Web Modeler deployment and its related resources are deployed via a helm release | `false` |
| | `image` | Configuration of the Web Modeler Docker images | |
| | `image.registry` | Can be used to set the Docker registry for the Web Modeler images (overwrites `global.image.registry`).<br/>Note: The images are not publicly available on Docker Hub, but only from Camunda's private registry. | `registry.camunda.cloud` |
| | `image.tag` | Can be used to set the Docker image tag for the Web Modeler images (overwrites `global.image.tag`) | |
| | `image.pullSecrets` | Can be used to configure [image pull secrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod).<br/>Note: A secret will be required, if the Web Modeler images are pulled directly from Camunda's private registry. | |
| | `restapi` | Configuration of the Web Modeler restapi component | |
| | `restapi.image` | Configuration of the restapi Docker image | |
| | `restapi.image.repository` | Defines which image repository to use for the restapi Docker image | `web-modeler-ee/modeler-restapi` |
| | `restapi.externalDatabase` | Can be used to configure a connection to an external database. This will only be applied if the postgresql dependency chart is disabled (by setting `postgresql.enabled` to `false`).<br/>Note: Currently, the only supported database system is PostgreSQL.| |
| | `restapi.externalDatabase.host` | Defines the host name of the database instance | |
| | `restapi.externalDatabase.port` | Defines the port number of the database instance | `5432` |
| | `restapi.externalDatabase.database` | Defines the database name | |
| | `restapi.externalDatabase.user` | Defines the database user | |
| | `restapi.externalDatabase.password` | Defines the database user's password | |
| | `restapi.mail` | Configuration for emails sent by Web Modeler | |
| | `restapi.mail.smtpHost` | Defines the host name of the SMTP server to be used by Web Modeler | |
| | `restapi.mail.smtpPort` | Defines the port number of the SMTP server | `587` |
| | `restapi.mail.smtpUser` | Can be used to provide a user for the SMTP server | |
| | `restapi.mail.smtpPassword` | Can be used to provide a password for the SMTP server | |
| | `restapi.mail.smtpTlsEnabled` | If `true`, enforces TLS encryption for SMTP connections (using STARTTLS) | `true` |
| | `restapi.mail.fromAddress` | Defines the email address that will be displayed as the sender of emails sent by Web Modeler | |
| | `restapi.mail.fromName` | Defines the name that will be displayed as the sender of emails sent by Web Modeler | Camunda Platform |
| | `restapi.podAnnotations` | Can be used to define extra restapi pod annotations | `{}` |
| | `restapi.podLabels` | Can be used to define extra restapi pod labels | `{}` |
| | `restapi.env` | Can be used to set extra environment variables in each restapi container | `[]` |
| | `restapi.command` | Can be used to [override the default command](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) provided by the container image | `[]` |
| | `restapi.extraVolumes` | Can be used to define extra volumes for the restapi pods, useful for TLS and self-signed certificates | `[]` |
| | `restapi.extraVolumeMounts` | Can be used to mount extra volumes for the restapi pods, useful for TLS and self-signed certificates | `[]` |
| | `restapi.podSecurityContext` | Can be used to define the security options the restapi pod should be run with | `{}` |
| | `restapi.containerSecurityContext` | Can be used to define the security options the restapi container should be run with | `{}` |
| | `restapi.nodeSelector` | Can be used to select the nodes the restapi pods should run on | `{}` |
| | `restapi.tolerations` | Can be used to define [pod tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[]` |
| | `restapi.affinity` | Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{}` |
| | `restapi.resources` | Configuration of [resource requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) for the container | `requests:`<br/>&nbsp;&nbsp;`cpu: 500m`<br/>&nbsp;&nbsp;`memory: 1Gi`<br/>`limits:`<br/>&nbsp;&nbsp;`cpu: 1000m`<br>&nbsp;&nbsp;`memory: 2Gi` |
| | `restapi.service` | Configuration of the Web Modeler restapi service | |
| | `restapi.service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `restapi.service.port` | Defines the default port of the service | `80` |
| | `restapi.service.managementPort` | Defines the management port of the service | `8091` |
| | `restapi.service.annotations` | Can be used to define annotations which will be applied to the service | `{}` |
| | `webapp` | Configuration of the Web Modeler webapp component | |
| | `webapp.image` | Configuration of the webapp Docker image | |
| | `webapp.image.repository` | Defines which image repository to use for the webapp Docker image | `web-modeler-ee/modeler-webapp` |
| | `webapp.podAnnotations` | Can be used to define extra webapp pod annotations | `{}` |
| | `webapp.podLabels` | Can be used to define extra webapp pod labels | `{}` |
| | `webapp.env` | Can be used to set extra environment variables in each webapp container | `[]` |
| | `webapp.command` | Can be used to [override the default command](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) provided by the container image | `[]` |
| | `webapp.extraVolumes` | Can be used to define extra volumes for the webapp pods, useful for TLS and self-signed certificates | `[]` |
| | `webapp.extraVolumeMounts` | Can be used to mount extra volumes for the webapp pods, useful for TLS and self-signed certificates | `[]` |
| | `webapp.podSecurityContext` | Can be used to define the security options the webapp pod should be run with | `{}` |
| | `webapp.containerSecurityContext` | Can be used to define the security options the webapp container should be run with | `{}` |
| | `webapp.nodeSelector` | Can be used to select the nodes the webapp pods should run on | `{}` |
| | `webapp.tolerations` | Can be used to define [pod tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[]` |
| | `webapp.affinity` | Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{}` |
| | `webapp.resources` | Configuration of [resource requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) for the container | `requests:`<br/>&nbsp;&nbsp;`cpu: 400m`<br/>&nbsp;&nbsp;`memory: 256Mi`<br/>`limits:`<br/>&nbsp;&nbsp;`cpu: 800m`<br>&nbsp;&nbsp;`memory: 512Mi` |
| | `webapp.service` | Configuration of the Web Modeler webapp service | |
| | `webapp.service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `webapp.service.port` | Defines the port of the service | `80` |
| | `webapp.service.annotations` | Can be used to define annotations which will be applied to the service | `{}` |
| | `websockets` | Configuration of the Web Modeler websockets component | |
| | `websockets.image` | Configuration of the websockets Docker image | |
| | `websockets.image.repository` | Defines which image repository to use for the websockets Docker image | `web-modeler-ee/modeler-websockets` |
| | `websockets.publicHost` | Can be used to define the host on which the WebSockets server can be reached from the Web Modeler client in the browser. The default value assumes that a port-forwarding to the websockets service has been created.<br/>Note: The host will only be used if the Ingress resource for Web Modeler is disabled. | `localhost` |
| | `websockets.publicPort` | Can be used to define the port number on which the WebSockets server can be reached from the Web Modeler client in the browser. The default value assumes that a port-forwarding to the websockets service on port `8085` has been created.<br/>Note: The port will only be used if the Ingress resource for Web Modeler is disabled. | `8085` |
| | `websockets.podAnnotations` | Can be used to define extra websockets pod annotations | `{}` |
| | `websockets.podLabels` | Can be used to define extra websockets pod labels | `{}` |
| | `websockets.env` | Can be used to set extra environment variables in each websockets container | `[]` |
| | `websockets.command` | Can be used to [override the default command](ttps://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/) provided by the container image | `[]` |
| | `websockets.podSecurityContext` | Can be used to define the security options the websockets pod should be run with | `{}` |
| | `websockets.containerSecurityContext` | Can be used to define the security options the websockets container should be run with | `{}` |
| | `websockets.nodeSelector` | Can be used to select the nodes the websockets pods should run on | `{}` |
| | `websockets.tolerations` | Can be used to define [pod tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) | `[]` |
| | `websockets.affinity` | Can be used to define [pod affinity or anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) | `{}` |
| | `websockets.resources` | Configuration of [resource requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits) for the container | `requests:`<br/>&nbsp;&nbsp;`cpu: 100m`<br/>&nbsp;&nbsp;`memory: 64Mi`<br/>`limits:`<br/>&nbsp;&nbsp;`cpu: 200m`<br>&nbsp;&nbsp;`memory: 128Mi` |
| | `websockets.service` | Configuration of the Web Modeler websockets service | |
| | `websockets.service.type` | Defines the [type of the service](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) | `ClusterIP` |
| | `websockets.service.port` | Defines the port of the service | `80` |
| | `websockets.service.annotations` | Can be used to define annotations which will be applied to the service | `{}` |
| | `serviceAccount` | Configuration for the service account the Web Modeler pods are assigned to | |
| | `serviceAccount.enabled` | If `true`, enables the Web Modeler service account | `true` |
| | `serviceAccount.name` | Can be used to set the name of the Web Modeler service account | |
| | `serviceAccount.annotations` | Can be used to set the annotations of the Web Modeler service account | `{}` |
| | `ingress` | Configuration of the Web Modeler ingress resource | |
| | `ingress.enabled` | If `true`, an Ingress resource will be deployed with the Web Modeler deployment. Only useful if an Ingress controller like NGINX is available. | `false` |
| | `ingress.className` | Defines the class or configuration of ingress which should be used by the controller | `nginx` |
| | `ingress.annotations` | Defines the ingress related annotations, consumed mostly by the ingress controller | `ingress.kubernetes.io/rewrite-target: "/"`<br/>`nginx.ingress.kubernetes.io/ssl-redirect: "false"` |
| | `ingress.webapp` | Configuration of the webapp ingress | |
| | `ingress.webapp.host` | Defines the [host of the ingress rule](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules); this is the host name on which the Web Modeler web application will be available.<br/>Note: The value must be different from `ingress.websockets.host`. | |
| | `ingress.webapp.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.webapp.tls.enabled` | If `true`, TLS will be configured on the ingress resource | `false` |
| | `ingress.webapp.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | |
| | `ingress.websockets` | Configuration of the websockets ingress | |
| | `ingress.websockets.host` | Defines the [host of the ingress rule](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-rules); this is the host name the Web Modeler client in the browser will use to connect to the WebSockets server.<br/>Note: The value must be different from `ingress.webapp.host`. | |
| | `ingress.websockets.tls` | Configuration for [TLS on the ingress resource](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls) | |
| | `ingress.websockets.tls.enabled` | If `true`, TLS will be configured on the ingress resource | `false` |
| | `ingress.websockets.tls.secretName` | Defines the secret name which contains the TLS private key and certificate | |
| | `postgresql` | Configuration for the postgresql dependency chart used by Web Modeler. See the [chart documentation](https://github.com/bitnami/charts/tree/master/bitnami/postgresql#parameters) for more details. | |
| | `postgresql.enabled` | If `true`, a PostgreSQL database will be deployed as part of the Helm release by using the dependency chart.<br/>Note: If set to `false`, a connection to an external database must be configured instead (see `restapi.externalDatabase`). | `true` |
| | `postgresql.nameOverride` | Defines the name of the Postgres resources (names will be [prefixed with the release name](https://github.com/bitnami/charts/tree/main/bitnami/postgresql#common-parameters)).<br/>Note: Must be different from the default value "postgresql" which is already used for Keycloak's database. | `postgresql-web-modeler` |
| | `postgresql.auth` | Configuration of the database authentication | |
| | `postgresql.auth.username` | Defines the name of the database user to be created for Web Modeler | `web-modeler` |
| | `postgresql.auth.password` | Defines the database user's password; a random password will be generated if left empty | |
| | `postgresql.auth.database` | Defines the name of the database to be created for Web Modeler | `web-modeler` |

### Elasticsearch

Camunda Platform 8 Helm chart has a dependency on the [Elasticsearch Helm Chart](https://github.com/elastic/helm-charts/blob/master/elasticsearch/README.md). All variables related to Elasticsearch can be found in [elastic/helm-charts/values.yaml](https://github.com/elastic/helm-charts/blob/main/elasticsearch/values.yaml) and can be set under `elasticsearch`.

| Section | Parameter | Description | Default |
|-|-|-|-|
| `elasticsearch`| `enabled` | If true, enables Elasticsearch deployment as part of the Camunda Platform Helm chart | `true` |

**Example:**

```yaml
elasticsearch:
  enabled: true
  imageTag: <YOUR VERSION HERE>
```

### Keycloak

When Camunda Platform 8 Identity component is enabled by default, and it depends on
[Bitnami Keycloak chart](https://github.com/bitnami/charts/tree/main/bitnami/keycloak).
Since Keycloak is a dependency for Identity, all variables related to Keycloak can be found in
[bitnami/keycloak/values.yaml](https://github.com/bitnami/charts/blob/main/bitnami/keycloak/values.yaml)
and can be set under `identity.keycloak`.

| Section | Parameter | Description | Default |
|-|-|-|-|
| `identity.keycloak`| `enabled` | If true, enables Keycloak chart deployment as part of the Camunda Platform Helm chart | `true` |

**Example:**

```yaml
identity:
  keycloak:
    enabled: true
```

#### Keycloak Theme

Camunda provides a custom theme for the login page used in all apps. The theme is copied from the Identity image.

The theme is added to Keycloak by default, however, since Helm v3 (latest checked 3.10.x) doesn't merge lists
with custom values files, then you will need to add this to your own values file if you override any of
`extraVolumes`, `initContainers`, or `extraVolumeMounts`.

```yaml
identity:
  keycloak:
    extraVolumes:
    - name: camunda-theme
      emptyDir:
        sizeLimit: 10Mi
    initContainers:
    - name: copy-camunda-theme
      image: >-
        {{- $identityImageParams := (dict "base" .Values.global "overlay" .Values.global.identity) -}}
        {{- include "camundaPlatform.imageByParams" $identityImageParams }}
      imagePullPolicy: "{{ .Values.global.image.pullPolicy }}"
      command: ["sh", "-c", "cp -a /app/keycloak-theme/* /mnt"]
      volumeMounts:
      - name: camunda-theme
        mountPath: /mnt
    extraVolumeMounts:
    - name: camunda-theme
      mountPath: /opt/bitnami/keycloak/themes/identity
```

## Guides

> **Note**
>
> For full list of guides list, please visit
> [Helm/Kubernetes Guides](https://docs.camunda.io/docs/next/self-managed/platform-deployment/helm-kubernetes/overview/)

### Adding dynamic exporters to Zeebe Brokers

This chart supports the addition of Zeebe Exporters by using `initContainer` as shown in the following example:

```
extraInitContainers:
  - name: init-exporters-hazelcast
    image: busybox:1.35
    command: ['/bin/sh', '-c']
    args: ['wget --no-check-certificate https://repo1.maven.org/maven2/io/zeebe/hazelcast/zeebe-hazelcast-exporter/0.8.0-alpha1/zeebe-hazelcast-exporter-0.8.0-alpha1-jar-with-dependencies.jar -O /exporters/zeebe-hazelcast-exporter.jar; ls -al /exporters']
    volumeMounts:
    - name: exporters
      mountPath: /exporters/
  - name: init-exporters-kafka
    image: busybox:1.35
    command: ['/bin/sh', '-c']
    args: ['wget --no-check-certificate https://github.com/zeebe-io/zeebe-kafka-exporter/releases/download/1.1.0/zeebe-kafka-exporter-1.1.0-uber.jar -O /exporters/zeebe-kafka-exporter.jar; ls -al /exporters']
    volumeMounts:
    - name: exporters
      mountPath: /exporters/
env:
  - name: ZEEBE_BROKER_EXPORTERS_HAZELCAST_JARPATH
    value: /exporters/zeebe-hazelcast-exporter.jar
  - name: ZEEBE_BROKER_EXPORTERS_HAZELCAST_CLASSNAME
    value: io.zeebe.hazelcast.exporter.HazelcastExporter
  - name: ZEEBE_HAZELCAST_REMOTE_ADDRESS
    value: "{{ .Release.Name }}-hazelcast"
```

This example is downloading the exporters' Jar from a URL and adding the Jars to the `exporters` directory,
which will be scanned for jars and added to the Zeebe broker classpath. Then with `environment variables`,
you can configure the exporter parameters.

## Development

For development purposes, you might want to deploy and test the charts without creating a new helm chart release.
To do this you can run the following:

```sh
 helm install YOUR_RELEASE_NAME --atomic --debug ./charts/camunda-platform
```

 * `--atomic if set, the installation process deletes the installation on failure. The --wait flag will be set automatically if --atomic is used`

 * `--debug enable verbose output`

To generate the resources/manifests without really installing them, you can use: 

 * `--dry-run simulate an install`

If you see errors like:

```sh
Error: found in Chart.yaml, but missing in charts/ directory: elasticsearch
```

Then you need to download the dependencies first.

Run the following to add resolve the dependencies:

```sh
make helm.repos-add
```

After this, you can run: `make helm.dependency-update`, which will update and download the dependencies for all charts.

The execution should look like this:
```
$ make helm.dependency-update
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

Please see the corresponding [release guide](../../RELEASE.md) to find out how to release the chart.

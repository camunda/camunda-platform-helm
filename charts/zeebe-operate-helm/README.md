[![Community Extension](https://img.shields.io/badge/Community%20Extension-An%20open%20source%20community%20maintained%20project-FF4700)](https://github.com/camunda-community-hub/community)[![Lifecycle: Incubating](https://img.shields.io/badge/Lifecycle-Incubating-blue)](https://github.com/Camunda-Community-Hub/community/blob/main/extension-lifecycle.md#incubating-)[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

# Zeebe Operate Helm Chart

This functionality is in beta and is subject to change. The design and code is less mature than official GA features and is being provided as-is with no warranties. Beta features are not subject to the support SLA of official GA features.

## Requirements

* [Helm](https://helm.sh/) >= 3.x +
* Kubernetes >= 1.8
* Minimum cluster requirements include the following to run this chart with default settings. All of these settings are configurable.
  * Three Kubernetes nodes to respect the default "hard" affinity settings
  * 1GB of RAM for the JVM heap

## Usage notes and getting started

## Installing

* Add the official zeebe helm charts repo
  ```
  helm repo add zeebe https://helm.camunda.io
  ```
* Install it
  ```
  helm install zeebe-operate zeebe/zeebe-operate --set global.zeebe=<YOUR ZEEBE CLUSTER NAME>
  ```

  Example if you installed the `zeebe-cluster-helm` chart manually with the name: `zb`

   ```
  helm install zeebe-operate zeebe/zeebe-operate --set global.zeebe=zb-zeebe
  ```

  > Note that you can find the Zeebe Cluster name by doing `kubectl get services` and copy the name of the Zeebe service, which will include the Helm Release name used to install the cluster. 

 ## Configuration
  | Parameter                     | Description                                                                                                                                                                                                                                                                                                                | Default                                                                                                                   |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `global.elasticsearch.host`         | ElasticSearch host to use in Elasticsearch Exporter connection  | `elasticsearch-master` |
| `global.elasticsearch.port`         | ElasticSearch port to use in Elasticsearch Exporter connection | `9200` |
| `global.elasticsearch.url`         | ElasticSearch full url to use in Elasticsearch Exporter connection. This config overrides the `host` and `port` above.  |  |
| `global.zeebe`                 | Zeebe Cluster to connect Operate to                                                                                                                               |                                                                                                            |
| `global.zeebePort` | Zeebe Cluster Port to connect Operate to | `26500` |
| `logging`               | Additional logging configuration                                                                                                                                  | `{level: { ROOT: INFO, org.camunda.operate: DEBUG }}`                                                     |
                                                                                                                                                                                                                                                                                                                


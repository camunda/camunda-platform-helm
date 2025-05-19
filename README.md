[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://opensource.org/licenses/Apache-2.0)
[![Artifact Hub](https://img.shields.io/badge/Artifact%20Hub-Camunda-417598?logo=artifacthub&logoColor=fff&style=flat-square)](https://artifacthub.io/packages/helm/camunda/camunda-platform)
[![Helm Chart Version Matrix](https://img.shields.io/badge/Helm_Chart-Version_Matrix-0F1689?logo=helm&logoColor=fff&style=flat-square)](https://helm.camunda.io/camunda-platform/version-matrix/)

<!-- omit in toc -->
# Camunda Platform Helm

The Camunda Platform Helm repo contains and hosts Camunda Platform Helm charts.

For more details about versions, check out the [full version matrix](./camunda-platform/version-matrix/).

The charts can be accessed by adding the following Helm repo to your Helm setup:

```shell
helm repo add camunda https://helm.camunda.io
helm repo update
```

<!-- omit in toc -->
## ToC

- [Architecture](#architecture)
- [Versioning](#versioning)
- [Installing Charts](#installing-charts)
- [Configure Charts](#configure-charts)
- [Uninstalling Charts](#uninstalling-charts)
- [Issues](#issues)

## Architecture

<p align="center">
  <img
    alt="Camunda Platform 8 Self-Managed Helm charts architecture diagram"
    src="imgs/camunda-platform-8-self-managed-architecture-diagram-combined-ingress.png"
    width="80%"
  />
</p>

## Versioning

Since the 8.4 release (January 2024), the Camunda Helm chart version has been decoupled from the version of the application (e.g., the chart version is 9.0.0 and the application version is 8.4.x).

For more details, check out the [full version matrix](./camunda-platform/version-matrix/).

## Installing Charts

Follow [the instructions in the Camunda Platform documentation](https://docs.camunda.io/docs/self-managed/setup/install/) to install the Camunda Platform to a K8s cluster.

Check the repo branch for more technical details:
[camunda/camunda-platform-helm](https://github.com/camunda/camunda-platform-helm)

## Configure Charts

Helm charts can be configured by using extra values files or directly via the `--set` option.
Ensure to check out the [Camunda Platform Helm Charts values](https://artifacthub.io/packages/helm/camunda/camunda-platform) for more information.

Example to enable the Prometheus ServiceMonitor for Zeebe:

```shell
helm install camunda camunda/camunda-platform --set zeebe.prometheusServiceMonitor.enabled=true
```

## Uninstalling Charts

You can remove these charts by running:

```shell
helm uninstall camunda
```

The command above will remove the Helm chart installation but will leave the PVCs which holds the data
for the applications. To remove the PVCs, please run the following:

> [!CAUTION]
> This will remove the installation data permenantly.

```shell
kubectl delete pvc -l app.kubernetes.io/instance=camunda
```

Or delete the related Kubernetes namespace, which contains all of the chart resources.

## Issues

Please create [new issues](https://github.com/camunda/camunda-platform-helm/issues) if you find problems with these charts. This repository is hosted using GitHub Pages, and the source code repository can be found [here](https://github.com/camunda/camunda-platform-helm).

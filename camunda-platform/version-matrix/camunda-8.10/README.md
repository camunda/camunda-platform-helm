<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->
[Back to version matrix index](../)

# Camunda 8.10 Helm Chart Version Matrix

Alpha

> Deploys Camunda Hub (the management plane replacing Web Modeler and Console SM). For the Orchestration Cluster versions Hub can manage, see the [Hub documentation](https://docs.camunda.io/docs/next/self-managed/components/hub/configuration/properties/).

| Helm Chart | Camunda | Released | Helm CLI | Helm Values | Release Notes |
|---|---|---|---|---|---|
| [15.0.0-alpha3](#helm-chart-1500-alpha3) | 8.10.0-alpha3 | 2026-07-09 | [4.1.4](https://github.com/helm/helm/releases/tag/v4.1.4) | [ArtifactHub](https://artifacthub.io/packages/helm/camunda/camunda-platform/15.0.0-alpha3?modal=values) | [Changelog](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-8.10-15.0.0-alpha3) |
| [15.0.0-alpha2](#helm-chart-1500-alpha2) | 8.10.0-alpha2 | 2026-06-05 | [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2) | [ArtifactHub](https://artifacthub.io/packages/helm/camunda/camunda-platform/15.0.0-alpha2?modal=values) | [Changelog](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-8.10-15.0.0-alpha2) |
| [15.0.0-alpha1](#helm-chart-1500-alpha1) | 8.10.0-alpha1 | 2026-05-12 | [3.19.4](https://github.com/helm/helm/releases/tag/v3.19.4) | [ArtifactHub](https://artifacthub.io/packages/helm/camunda/camunda-platform/15.0.0-alpha1?modal=values) | [Changelog](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-8.10-15.0.0-alpha1) |

_Enterprise images replace the matching Non-Camunda (Bitnami OSS) images when using Camunda Enterprise registry access — mirror the set that matches your configuration._


---

## Helm chart 15.0.0-alpha3

Supported versions:

- Camunda applications: [8.10](https://github.com/camunda/camunda/releases?q=tag%3A8.10&expanded=true)
- Camunda version matrix: [8.10](https://helm.camunda.io/camunda-platform/version-matrix/camunda-8.10)
- Helm values: [15.0.0-alpha3](https://artifacthub.io/packages/helm/camunda/camunda-platform/15.0.0-alpha3?modal=values)
- Helm CLI: [4.1.4](https://github.com/helm/helm/releases/tag/v4.1.4)

Camunda images:

- docker.io/camunda/camunda:8.10.0-alpha3
- docker.io/camunda/connectors-bundle:8.10.0-alpha3
- docker.io/camunda/hub-websockets:8.10.0-alpha3-rc1
- docker.io/camunda/hub:8.10.0-alpha3-rc1
- docker.io/camunda/identity:8.10.0-alpha3
- docker.io/camunda/optimize:8.10.0-alpha3

## Helm chart 15.0.0-alpha2

Supported versions:

- Camunda applications: [8.10](https://github.com/camunda/camunda/releases?q=tag%3A8.10&expanded=true)
- Camunda version matrix: [8.10](https://helm.camunda.io/camunda-platform/version-matrix/camunda-8.10)
- Helm values: [15.0.0-alpha2](https://artifacthub.io/packages/helm/camunda/camunda-platform/15.0.0-alpha2?modal=values)
- Helm CLI: [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2)

Camunda images:

- docker.io/camunda/camunda:8.10.0-alpha2
- docker.io/camunda/connectors-bundle:8.10.0-alpha2
- docker.io/camunda/console:8.10.0-alpha2
- docker.io/camunda/hub-websockets:8.10.0-alpha2
- docker.io/camunda/hub:8.10.0-alpha2
- docker.io/camunda/identity:8.10.0-alpha2
- docker.io/camunda/optimize:8.10.0-alpha2

Non-Camunda images:

- busybox:1.36

## Helm chart 15.0.0-alpha1

Supported versions:

- Camunda applications: [8.10](https://github.com/camunda/camunda/releases?q=tag%3A8.10&expanded=true)
- Camunda version matrix: [8.10](https://helm.camunda.io/camunda-platform/version-matrix/camunda-8.10)
- Helm values: [15.0.0-alpha1](https://artifacthub.io/packages/helm/camunda/camunda-platform/15.0.0-alpha1?modal=values)
- Helm CLI: [3.19.4](https://github.com/helm/helm/releases/tag/v3.19.4)

Camunda images:

- docker.io/camunda/camunda:8.10.0-alpha1
- docker.io/camunda/connectors-bundle:8.10.0-alpha1
- docker.io/camunda/console:8.10.0-alpha1
- docker.io/camunda/identity:8.10.0-alpha1
- docker.io/camunda/optimize:8.10.0-alpha1
- registry.camunda.cloud/camunda/keycloak:26.3.3

Non-Camunda images:

- busybox:1.36
- docker.io/bitnamilegacy/elasticsearch:8.18.0
- docker.io/bitnamilegacy/os-shell:12-debian-12-r43
- docker.io/bitnamilegacy/postgresql:14.18.0-debian-12-r0
- docker.io/bitnamilegacy/postgresql:15.10.0-debian-12-r2

Enterprise images ([Camunda Enterprise](https://docs.camunda.io/docs/8.10/self-managed/deployment/helm/configure/registry-and-images/install-bitnami-enterprise-images/)):

- registry.camunda.cloud/keycloak-ee/keycloak:26.5.6
- registry.camunda.cloud/vendor-ee/elasticsearch:8.19.13
- registry.camunda.cloud/vendor-ee/os-shell:12-debian-12-r43
- registry.camunda.cloud/vendor-ee/postgresql:18.3.0-debian-12-r0
- registry.camunda.cloud/vendor-ee/postgresql:18.3.0-debian-12-r2

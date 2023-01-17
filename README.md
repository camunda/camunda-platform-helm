# Camunda Platform 8 Helm

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Test - Unit](https://github.com/camunda/camunda-platform-helm/actions/workflows/test-unit.yml/badge.svg)](https://github.com/camunda/camunda-platform-helm/actions/workflows/test-unit.yml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/camunda)](https://artifacthub.io/packages/search?repo=camunda)


  - [Overview](#overview)
  - [Installation](#installation)
  - [Documentation](#documentation)
  - [Guides](#guides)
  - [Issues](#issues)
  - [Contributing](#contributing)
  - [Versioning](#versioning)
  - [Releasing](#releasing)
  - [Deprecation](#deprecation)
  - [License](#license)

## Overview

Camunda Platform 8 Self-Managed Helm charts repo. Camunda Platform 8 Helm chart is an umbrella chart
for different components. Some are internal (sub-charts), and some are external (third-party).
The dependency management is fully automated and managed by Helm itself.

<p align="center">
  <img
    alt="Camunda Platform 8 Self-Managed Helm charts architecture diagram"
    src="imgs/camunda-platform-8-self-managed-architecture-diagram-combined-ingress.png"
    width="60%"
  />
  <br/>
  Camunda Platform 8 architecture
</p>

## Installation

Find out more details about different installation and deployment options
on the [Camunda Platform 8 Helm chart readme](./charts/camunda-platform/README.md).

## Documentation

This repo is mainly for Helm charts documentation. For full details, please visit the official
[Camunda Platform 8 Self-Managed docs](https://docs.camunda.io/docs/self-managed/about-self-managed/).

## Guides

Default values cannot cover every use case, so we have
[Camunda Platform 8 deploy guides](https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/guides/).
The guides have detailed examples for different use cases like Ingress setup.

## Issues

Please create a [new issue](https://github.com/camunda/camunda-platform-helm/issues) if you found any problem
with the Camunda Platform 8 Helm charts.

## Contributing

We value all feedback and contributions. To start contributing to this project, please:

- **Don't create a PR without opening [an issue](https://github.com/camunda/camunda-platform-helm/issues/new/choose)
  and discussing it first.**
- Familiarize yourself with the
[contribution guide](https://github.com/camunda/camunda-platform-helm/blob/main/CONTRIBUTING.md).
- Find more information about configuring and deploying the Camunda Platform 8
  [Helm chart](./charts/camunda-platform/README.md).

## Versioning

Camunda Platform 8 **Helm chart** versions are only aligned with the minor version of [Camunda Platform 8](https://github.com/camunda/camunda-platform).
In other words, the `Camunda Platform 8 Helm chart` could have a patch version different from the `Camunda Platform 8`.

For example, the Camunda Platform 8 Helm chart could be on version `8.1.1`, but Camunda Platform 8 apps are on version `8.1.0`. Additionally, the Camunda Platform 8 Helm chart could be on version `8.1.1`, but Camunda Platform 8 apps are on version `8.1.2`.

We work to keep the Helm chart updated with the latest version of Camunda Platform 8, but currently this is not guaranteed. Note that the latest version of the Helm chart may not necessarily have the latest version of the Camunda Platform 8 apps. This should not be an issue unless you rely on a specific Camunda Platform 8 patch version. In that case, you can set the desired version via a custom values file.

## Releasing

Please visit the [Camunda Platform 8 release guide](./RELEASE.md) to find out how to release the charts.

## Deprecation

<!-- omit in toc -->
### Old Zeebe charts

With the creation of the Camunda Platform 8 Helm charts (previously known as `ccsm-helm`), the old `zeebe-*` charts
have been deprecated. That means they are no longer part of the repository and are no longer maintained.
However, the packaged charts are still available for download. But will be removed in the next releases.

The following charts are deprecated:

- zeebe-full-helm
- zeebe-cluster-helm
- zeebe-operate-helm
- zeebe-tasklist-helm

The new `camunda-platform` chart is a full replacement of `zeebe-full-helm` and replaces (contains) all other charts
as sub-charts. All sub-charts in the `camunda-platform` are enabled by default.

For a complete migration guide, visit [migration docs](./MIGRATION.md).

## License

Camunda Platform 8 Self-Managed Helm charts are licensed under the open-source Apache License 2.0.
Please see [LICENSE](LICENSE) for details.

For Camunda Platform 8 components, please visit
[licensing information page](https://docs.camunda.io/docs/reference/licenses).

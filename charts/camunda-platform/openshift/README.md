# OpenShift Support

The Camunda 8 Helm chart can be deployed to OpenShift using extra values file that unset the `securityContext`
according to OpenShift default Security Context Constraints (SCCs).

For full details, please check the official docs:
[Camunda 8 Self-Managed Red Hat OpenShift](https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/platforms/redhat-openshift/).


## Prerequisite

First, download the exact chart version you use and extract the OpenShift extra values file:

```shell
# Ensure set CHART_VERSION to match the chart you want to install.
helm pull camunda/camunda-platform --version CHART_VERSION --untar --untardir /tmp
```

## Normal setup

If you are using any Helm CLI **NOT** between the 3.1.4 and 3.1.12 (**a recommended version to use: 3.1.13**),
which is not affected by the [nested null bug](https://github.com/helm/helm/issues/9136),
then follow [normal installation flow](../README.md#installation) using OpenShift extra values file
(you don't need to edit that file!).

E.g.

```shell
helm install camunda camunda/camunda-platform --skip-crds \
    --values /tmp/camunda-platform/openshift/values.yaml
```

## Post-renderer setup

> **Warning**
> If using a post-renderer, you must use the post-renderer whenever you are updating your release,
> not only during the initial installation. If you do not, the default values will be used again,
> which will prevent some services from starting.

If you are using one of the Helm CLI version affected by the [nested null bug](https://github.com/helm/helm/issues/9136)
(Helm CLI **between** the 3.1.4 and 3.1.12), and cannot upgrade your Helm CLI for a reason or another,
then you need to Helm post-render with a patch script as following:

```shell
helm install camunda camunda/camunda-platform --skip-crds       \
    --values /tmp/camunda-platform/openshift/values.yaml        \
    --values /tmp/camunda-platform/openshift/values-patch.yaml  \
    --post-renderer /tmp/camunda-platform/openshift/patch.sh
```

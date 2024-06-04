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


## Post-renderer setup

> **Warning**
> If using a post-renderer, you must use the post-renderer whenever you are updating your release,
> not only during the initial installation. If you do not, the default values will be used again,
> which will prevent some services from starting.

If you are using one of the Helm CLI version affected by the nested null bug [1](https://github.com/helm/helm/issues/9136)
[2](https://github.com/helm/helm/issues/12490)
then you need to Helm post-render with a patch script as following:

```shell
helm install camunda camunda/camunda-platform --skip-crds       \
    --values /tmp/camunda-platform/openshift/values.yaml        \
    --post-renderer bash --post-renderer-args /tmp/camunda-platform/openshift/patch.sh
```


## Normal setup

If you are using any Helm CLI 3.1.4 or less, which is not affected by the nested null bug
[1](https://github.com/helm/helm/issues/9136) [2](https://github.com/helm/helm/issues/12490),
then follow [normal installation flow](../README.md#installation) using OpenShift extra values file
(you don't need to edit that file!).

E.g.

```shell
helm install camunda camunda/camunda-platform --skip-crds \
    --values /tmp/camunda-platform/openshift/values.yaml
```

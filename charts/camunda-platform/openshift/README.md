# OpenShift Support

The `camunda-platform` Helm chart can be deployed to Openshift with a few modifications.

## Getting started

In order to deploy the chart to a standard Openshift cluster with default policies, we need to configure each chart
ensure that none of the bundled applications run under a specific user and/or group. This means making sure that no
security context, whether pod or container specific, specifies a user via `runAsUser` or `fsGroup`.

The `Elasticsearch`, `Keycloak`, and `Postgresql` charts all specify default non-root users for security purposes. This
needs to be disabled by removing these. Because of this, it's unfortunately not possible to deploy the chart with Helm
3.2.x or greater and the `restricted` SCC without using a workaround. This is due to a longstanding bug in Helm, which
you can see [here](https://github.com/helm/helm/issues/9136). You can read more about this in the [Usage](#usage)
section.

## Compatibility

We test against the following Openshift versions, and guarantee compatibility with:

| Openshift Version |     Supported      |
|-------------------|--------------------|
| 4.10.x            | :white_check_mark: |

Any version not explicitly marked in the table above is not tested, and we cannot guarantee compatibility.

## Usage

You will find in this repository a sample `values.yaml` file to get you start on Openshift
[here](./values.yaml).

Before proceeding, make sure you've fulfilled all the requirements as described in the [README](/README.md), namely that
you have Helm installed, and you've added the chart's repository.

Finally, due to a longstanding bug in Helm (see [here](https://github.com/helm/helm/issues/9136)), the installation
instructions are a bit different depending on your Helm version. To find out, quickly run:

```shell
helm version
```

For example, if your version was "3.9.0", you would get something like this:

```shell
$ helm version
version.BuildInfo{Version:"v3.9.0", GitCommit:"7ceeda6c585217a19a1131663d8cd1f7d641b2a7", GitTreeState:"clean", GoVersion:"go1.17.5"}
```

If your version is greater than or equal to 3.2.0, refer to [this section](#helm-32x-and-greater). If it's lower, refer
to [this section](#helm-313-or-lower).

### Helm 3.1.3 or lower

If you're using Helm 3.1.3 or lower, you can simply install the chart as you normally would. Copy
the [values.yaml](/openshift/values.yaml) locally, or merge the values with your own values file.

> Make sure you've logged into your Openshift cluster, and have selected a project to deploy the chart into. You can
> login using `oc login`, and create a new project via `oc new-project myProject`.

```shell
helm install test camunda/camunda-platform --skip-crds -f values.yaml
```

If you wanted to use the chart with your own values, simply copy the [values.yaml](/openshift/values.yaml) locally, e.g.
as `openshift.yaml`, and run:

```shell
helm install test camunda/camunda-platform --skip-crds -f openshift.yaml -f values.yaml
```

By specifying your own values file last, you can then override any default values we set in the OpenShift specific file.

You can verify the installation as described in the [README.md](/README.md).

### Helm 3.2.x and greater

Because Helm 3.2.x or greater cannot unset default values in sub-charts, we have two options: allow usage of the
`anyuid` or `nonroot` SCC, or use a chart post-renderer.

#### anyuid or nonroot SCC

Under the `restricted` SCC, these charts (`elasticsearch`, `bitnami/keycloak`, and `bitnami/postgresql`) would fail to
deploy. The simplest method is thus to allow the user/service account which will deploy your chart to use the `anyuid`
or `nonroot` SCC. This will let sub-charts which define arbitrary UIDs/GIDs use these IDs.

For example, if you will use the user `deployer` to deploy the chart but still want to restrict running to nonroot
users:

```shell
oc adm policy add-scc-to-user nonroot deployer
```

When this is done, you do not need any special values file to install the charts.

#### Using a post-renderer

If you must use the `restricted` SCC and Helm 3.2.x, then you will need to use a post-renderer to install the charts. To
do so, you'll need to use both values file, [openshift/values.yaml](/openshift/values.yaml) and
[openshift/values-patch.yaml](/openshift/values-patch.yaml), and the companion script [patch.sh](/openshift/patch.sh),
a Helm post renderer.

> For this method, you will need to have `bash` and `sed` installed locally. After downloading the script, make sure it
> is executable by your user, e.g. `chmod u+x patch.sh`.

When everything is ready, you can now run the following:

```shell
helm install test camunda/camunda-platform --skip-crds -f values.yaml -f values-patch.yaml --post-renderer ./patch.sh
```

If you wanted to use this with your own values file, remember to place that one as the right most, but keeping the
above order as is. For example, let's say you download the files as `openshift.yaml` and `openshift-patch.yaml`:

```shell
helm install test camunda/camunda-platform --skip-crds -f openshift.yaml -f openshift-patch.yaml -f values.yaml --post-renderer ./patch.sh
```

You can verify the installation as described in the [README.md](/README.md).

##### Upgrade

Note that if you use the post-renderer, you will _also_ need to use it when upgrading your chart. For example:

```shell
helm install test camunda/camunda-platform --skip-crds -f values.yaml -f values-patch.yaml --post-renderer ./patch.sh
helm upgrade test camunda/camunda-platform --skip-crds --reuse-values -f values-patch.yaml --post-renderer ./patch.sh
```

Even if we use the flag `--reuse-values`, the default values from the sub-charts will be picked up again and need to be
nullified by the post-renderer again.

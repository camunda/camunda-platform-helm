# Openshift Support

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
[here](/openshift/values.yaml).

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
helm install quick-test camunda/camunda-platform --skip-crds --values values.yaml
```

You can verify the installation as described in the [README.md](/README.md).

### Helm 3.2.x and greater

Because Helm 3.2.x or greater cannot unset default values in sub-charts, we have two options: allow usage of the
`anyuid` or `nonroot` SCC, or use `kustomize` as a chart post-renderer.

#### anyuid or nonroot SCC

Under the `restricted` SCC, these charts (`elasticsearch`, `bitnami/keycloak`, and `bitnami/postgresql`) would fail to
deploy. The simplest method is thus to allow the user/service account which will deploy your chart to use the `anyuid`
or `nonroot` SCC. This will let sub-charts which define arbitrary UIDs/GIDs use these IDs.

For example, if you will use the user `deployer` to deploy the chart but still want to restrict running to nonroot
users:

```shell
oc adm policy add-scc-to-user nonroot deployer
```

#### kustomize post-renderer

If you must use the `restricted` SCC, then copy over the following files:
[kustomization.yaml](/openshift/kustomization.yaml) and [patch.sh](/openshift/patch.sh). Make sure that the downloaded
script, `patch.sh`, is executable (e.g. `chmod u+x patch.sh`).

> For this method, you will need to install `kustomize` locally. You can install it by
> [following these instructions](https://kubectl.docs.kubernetes.io/installation/kustomize/binaries/).
> Note that some Openshift client installations come with `kustomize` installed, which you can verify as by running
> `oc kustomize --help`.
> If you did install `kustomize` in a standalone way, make sure to prefix the `helm` command with
> `KUSTOMIZE=$(which kustomize)`, e.g. `KUSTOMIZE=$(which kustomize) helm install ...`

When everything is ready, you can now run the following:

```shell
helm install test camunda/camunda-platform --skip-crds --values values.yaml --post-renderer ./patch.sh
```

You can verify the installation as described in the [README.md](/README.md).

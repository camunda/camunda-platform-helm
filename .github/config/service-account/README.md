# Camunda Cloud Helm Service Account

This directory contains resources to create a serviceaccount, role and clusterrolebinding. These
have been used to create related resources in our gke. This was necessary to run our
automated integration tests in our gke cluster.

After the serviceaccount and related roles are created. Kubernetes will create a token for the related serviceaccount.
This token needs to be used in order to access the related kubernetes cluster.

To deploy it use:

```shell
kubectl kustomize .
```

To receive the token use:

```shell
kubectl -n ccsm-helm describe secrets ccsm-helm-sa-token-*
```

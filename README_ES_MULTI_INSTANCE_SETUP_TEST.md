# Helm Chart Installation Guide

This README provides instructions on how to install Helm charts with a custom configuration using the `dm-test-multi-es-setup-values.yaml` file.

## Prerequisites
Before proceeding, ensure you have the following installed:

- Helm 3.x or later
- Access to a Kubernetes cluster
- Kubernetes command-line tool (kubectl) configured to communicate with your cluster

## Installation Steps

1. **Create a Namespace (Optional)**
    If you want to install the chart in a separate namespace, create one:
    
    ```bash
    kubectl create namespace [NAMESPACE]
    ```
    Replace `[NAMESPACE]` with the desired namespace name.

2. **Update HELM dependencies**
   Form the root folder of current repo execute the following command to update helm dependencies:

   ```bash
    helm dependency update charts/camunda-platform && \
      helm dependency update charts/camunda-platform/charts/identity
   ```
3. **Install the Chart**
    Use the following command to install the Helm chart with the custom values from the `dm-test-multi-es-setup-values.yaml` file:
    
    ```bash
    helm install [RELEASE_NAME] charts/camunda-platform -f dm-test-multi-es-setup-values.yaml --namespace [NAMESPACE]
    ```
    
    Replace `[RELEASE_NAME]` and `[NAMESPACE]` with the actual release and namespace where you want to install the chart. 
    If you did not create a separate namespace, omit the `--namespace [NAMESPACE]` part.
    ```bash
    helm install dm-helm-test charts/camunda-platform -f kind/dm-test-multi-es-setup-values.yaml
    ```

4. **Verify the Installation**
   After the installation command completes, verify that the chart has been installed correctly:
    ```bash
    helm list --namespace [NAMESPACE]
    kubectl get all --namespace [NAMESPACE]
    ```
    This will show you the Helm releases in the specified namespace and the Kubernetes resources that have been deployed as part of the Helm chart.

## Troubleshooting

If you encounter any issues during the installation, you can check the status of the Helm release and the Kubernetes resources, or review the logs of the individual pods:

```bash
helm status [RELEASE_NAME] --namespace [NAMESPACE]
kubectl describe pod/[POD_NAME] --namespace [NAMESPACE]
kubectl logs pod/[POD_NAME] --namespace [NAMESPACE]
```

Replace `[RELEASE_NAME]` with your release name and `[POD_NAME]` with the name of the pod you wish to inspect.
```bash
helm status dm-helm-test
```

## Uninstallation

To uninstall the Helm chart, use the following command:

```bash
helm uninstall [RELEASE_NAME] --namespace [NAMESPACE] && \
  kubectl delete pvc -l app.kubernetes.io/instance=[RELEASE_NAME] && \
  kubectl delete pvc -l release=[RELEASE_NAME]
```

This will remove all the Kubernetes components associated with the chart and delete the release.
Remember to replace placeholders (e.g., [RELEASE_NAME], [NAMESPACE], etc.) with actual values:
```bash
helm uninstall dm-helm-test --namespace dm-helm-test && \
  kubectl delete pvc -l app.kubernetes.io/instance=dm-helm-test && \
  kubectl delete pvc -l release=dm-helm-test
```
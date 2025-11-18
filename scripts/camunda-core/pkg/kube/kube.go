package kube

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/logging"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

const fieldManagerName = "camunda-platform-helm"

func defaultApplyOptions() metav1.ApplyOptions {
	return metav1.ApplyOptions{
		FieldManager: fieldManagerName,
		Force:        true,
	}
}

func defaultPatchOptions() metav1.PatchOptions {
	return metav1.PatchOptions{
		FieldManager: fieldManagerName,
		Force:        func() *bool { b := true; return &b }(),
	}
}

type Client struct {
	clientset       kubernetes.Interface
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	kubeconfig      string
	kubeContext     string
}

func NewClient(kubeconfig, kubeContext string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	return &Client{
		clientset:       clientset,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		kubeconfig:      kubeconfig,
		kubeContext:     kubeContext,
	}, nil
}

func (c *Client) EnsureNamespace(ctx context.Context, namespace string) error {
	if namespace == "" {
		return errors.New("namespace must not be empty")
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("applying namespace")

	nsApply := corev1apply.Namespace(namespace)

	_, err := c.clientset.CoreV1().Namespaces().Apply(ctx, nsApply, defaultApplyOptions())
	if err != nil {
		return fmt.Errorf("failed to apply namespace %q (context=%q): %w", namespace, c.kubeContext, err)
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("namespace applied successfully")
	return nil
}

func (c *Client) EnsureDockerRegistrySecret(ctx context.Context, namespace, username, password string) error {
	if username == "" || password == "" {
		logging.Logger.Debug().Str("namespace", namespace).Msg("skipping docker registry secret creation (credentials not provided)")
		return nil
	}

	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("secret", "registry-camunda-cloud").
		Msg("creating/updating docker registry secret")

	dockerConfig := map[string]any{
		"auths": map[string]any{
			"registry.camunda.cloud": map[string]any{
				"username": username,
				"password": password,
				"auth":     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
			},
		},
	}

	dockerConfigJSON, err := json.Marshal(dockerConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal docker config: %w", err)
	}

	secretApply := corev1apply.Secret("registry-camunda-cloud", namespace).
		WithType(corev1.SecretTypeDockerConfigJson).
		WithData(map[string][]byte{
			corev1.DockerConfigJsonKey: dockerConfigJSON,
		})

	_, err = c.clientset.CoreV1().Secrets(namespace).Apply(
		ctx,
		secretApply,
		defaultApplyOptions(),
	)
	if err != nil {
		return fmt.Errorf("failed to apply docker registry secret in namespace %q (context=%q): %w", namespace, c.kubeContext, err)
	}

	return nil
}

func (c *Client) DeleteNamespace(ctx context.Context, namespace string) error {
	logging.Logger.Debug().Str("namespace", namespace).Msg("deleting namespace")

	err := c.clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logging.Logger.Debug().Str("namespace", namespace).Msg("namespace not found, nothing to delete")
			return nil
		}
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("waiting for namespace deletion to complete")

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}

func (c *Client) HasCRD(ctx context.Context, group, kind string) (bool, error) {
	_, apiResourceLists, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			logging.Logger.Debug().Err(err).Msg("partial discovery failure, continuing with available resources")
		} else {
			return false, fmt.Errorf("failed to discover API resources: %w", err)
		}
	}

	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}

		if gv.Group != group {
			continue
		}

		for _, resource := range apiResourceList.APIResources {
			if resource.Kind == kind {
				logging.Logger.Debug().
					Str("group", group).
					Str("kind", kind).
					Str("version", gv.Version).
					Msg("CRD found")
				return true, nil
			}
		}
	}

	logging.Logger.Debug().
		Str("group", group).
		Str("kind", kind).
		Msg("CRD not found")
	return false, nil
}

func (c *Client) ApplyConfigMap(ctx context.Context, namespace, name string, data map[string]string) error {
	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("configMap", name).
		Msg("applying ConfigMap")

	configMapApply := corev1apply.ConfigMap(name, namespace).
		WithData(data)

	_, err := c.clientset.CoreV1().ConfigMaps(namespace).Apply(
		ctx,
		configMapApply,
		defaultApplyOptions(),
	)
	if err != nil {
		return fmt.Errorf("failed to apply ConfigMap %q in namespace %q (context=%q): %w", name, namespace, c.kubeContext, err)
	}

	return nil
}

func (c *Client) ApplyManifest(ctx context.Context, namespace string, manifestData []byte) error {
	return applyManifestData(ctx, c, namespace, manifestData)
}

func (c *Client) SetLabelsAndAnnotations(ctx context.Context, namespace string, labels, annotations map[string]string) error {
	logging.Logger.Debug().
		Str("namespace", namespace).
		Msg("applying labels and annotations to namespace")

	nsApply := corev1apply.Namespace(namespace).
		WithLabels(labels).
		WithAnnotations(annotations)

	_, err := c.clientset.CoreV1().Namespaces().Apply(ctx, nsApply, defaultApplyOptions())
	if err != nil {
		return fmt.Errorf("failed to apply namespace %q labels/annotations (context=%q): %w", namespace, c.kubeContext, err)
	}

	return nil
}

const (
	platformGKE  = "gke"
	platformROSA = "rosa"
	platformEKS  = "eks"

	secretNameTLS = "aws-camunda-cloud-tls"
)

func ApplyExternalSecretsAndCerts(ctx context.Context, kubeconfig, kubeContext, platform, repoRoot, chartPath, namespace, namespacePrefix string) error {
	platform = strings.ToLower(strings.TrimSpace(platform))

	logging.Logger.Debug().
		Str("platform", platform).
		Str("namespace", namespace).
		Str("chartPath", chartPath).
		Str("repoRoot", repoRoot).
		Msg("applying external secrets/certs")

	client, err := NewClient(kubeconfig, kubeContext)
	if err != nil {
		return err
	}
	hasCRD, err := checkIfExternalSecretsCRDExists(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to check for ExternalSecrets CRD: %w", err)
	}
	if !hasCRD {
		logging.Logger.Error().Msg("ExternalSecrets CRD not installed. ExternalSecrets CRD is required for TLS certificates, integration test credentials and infra credentials.")
		return fmt.Errorf("ExternalSecrets CRD not installed")
	}

	provider, err := NewPlatformSecretsProvider(platform, repoRoot, chartPath, namespacePrefix)
	if err != nil {
		return err
	}

	return provider.Apply(ctx, client, namespace)
}

func computeEKSSourceNamespace(namespacePrefix string) string {
	prefix := strings.TrimSpace(namespacePrefix)
	if prefix == "" {
		return "certs"
	}
	return prefix + "-certs"
}

func applyManifestIfExists(ctx context.Context, client *Client, namespace, filePath, description string) error {
	if !fileExists(filePath) {
		logging.Logger.Debug().Str("file", filePath).Msgf("%s manifest not found (skipping)", description)
		return nil
	}

	logging.Logger.Debug().Str("file", filePath).Msgf("applying %s", description)
	if err := applyManifestFile(ctx, client, namespace, filePath); err != nil {
		return err
	}

	return nil
}

func applyManifestFile(ctx context.Context, client *Client, namespace, filePath string) error {
	if !fileExists(filePath) {
		return fmt.Errorf("manifest file not found: %s", filePath)
	}

	logging.Logger.Debug().Str("file", filePath).Str("namespace", namespace).Msg("applying manifest file")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	return applyManifestData(ctx, client, namespace, data)
}

func applyManifestData(ctx context.Context, client *Client, namespace string, data []byte) error {
	var obj map[string]any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("failed to parse manifest (YAML/JSON): %w", err)
	}

	unstructuredObj := &unstructured.Unstructured{Object: obj}
	gvk := unstructuredObj.GroupVersionKind()

	if unstructuredObj.GetAPIVersion() == "" || unstructuredObj.GetKind() == "" {
		return fmt.Errorf("manifest missing apiVersion or kind")
	}

	if unstructuredObj.GetNamespace() == "" {
		unstructuredObj.SetNamespace(namespace)
	}

	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	gvr.Group = gvk.Group
	gvr.Version = gvk.Version

	logging.Logger.Debug().
		Str("kind", gvk.Kind).
		Str("resource", gvr.Resource).
		Str("namespace", namespace).
		Msg("applying resource")

	data, err := json.Marshal(unstructuredObj.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal resource for apply: %w", err)
	}

	_, err = client.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
		ctx,
		unstructuredObj.GetName(),
		types.ApplyPatchType,
		data,
		defaultPatchOptions(),
	)
	if err != nil {
		return fmt.Errorf("failed to apply %s %q in namespace %q: %w", gvk.Kind, unstructuredObj.GetName(), namespace, err)
	}

	return nil
}

func checkIfExternalSecretsCRDExists(ctx context.Context, client *Client) (bool, error) {
	hasCRD, err := client.HasCRD(ctx, "external-secrets.io", "ExternalSecret")
	if err != nil {
		return false, fmt.Errorf("failed to check for ExternalSecrets CRD: %w", err)
	}
	return hasCRD, nil
}

func waitExternalSecretsReady(ctx context.Context, client *Client, namespace string, timeout time.Duration) error {
	externalSecretGVR := schema.GroupVersionResource{
		Group:    "external-secrets.io",
		Version:  "v1beta1",
		Resource: "externalsecrets",
	}

	list, err := client.dynamicClient.Resource(externalSecretGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ExternalSecrets: %w", err)
	}

	if len(list.Items) == 0 {
		logging.Logger.Debug().Str("namespace", namespace).Msg("no ExternalSecrets found; skipping wait")
		return nil
	}

	logging.Logger.Debug().Str("namespace", namespace).Int("count", len(list.Items)).Msg("waiting for ExternalSecrets readiness")

	for _, item := range list.Items {
		name := item.GetName()

		err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
			obj, err := client.dynamicClient.Resource(externalSecretGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			status, found, err := unstructured.NestedMap(obj.Object, "status")
			if err != nil || !found {
				return false, nil
			}

			conditions, found, err := unstructured.NestedSlice(status, "conditions")
			if err != nil || !found {
				return false, nil
			}

			for _, cond := range conditions {
				condMap, ok := cond.(map[string]any)
				if !ok {
					continue
				}

				condType, _, _ := unstructured.NestedString(condMap, "type")
				condStatus, _, _ := unstructured.NestedString(condMap, "status")

				if condType == "Ready" && condStatus == "True" {
					return true, nil
				}
			}

			return false, nil
		})
		if err != nil {
			return fmt.Errorf("ExternalSecret %s not ready: %w", name, err)
		}
	}

	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func DeleteNamespace(ctx context.Context, kubeconfig, kubeContext, namespace string) error {
	client, err := NewClient(kubeconfig, kubeContext)
	if err != nil {
		return err
	}
	return client.DeleteNamespace(ctx, namespace)
}
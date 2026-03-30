package kube

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const fieldManagerName = "camunda-platform-helm"

// formatNamespacePermissionError creates a user-friendly error message for namespace permission errors.
func formatNamespacePermissionError(operation, namespace, verb string, err error) error {
	// Namespaces are cluster-scoped resources, so kubectl commands don't include the namespace name
	// For Apply operations, we need both create and patch permissions
	var verifyCommands []string
	var additionalHint string

	if verb == "get" {
		additionalHint = "  2. Or request cluster-wide namespace read permissions\n"
		verifyCommands = []string{"kubectl auth can-i get namespaces"}
	} else if verb == "create" {
		// Apply can use either create or patch, so check both
		verifyCommands = []string{
			"kubectl auth can-i create namespaces",
			"kubectl auth can-i patch namespaces",
		}
		additionalHint = "  2. Note: Apply operations require either 'create' or 'patch' permissions\n"
	} else {
		verifyCommands = []string{fmt.Sprintf("kubectl auth can-i %s namespaces", verb)}
	}

	verifyCmdsStr := ""
	for i, cmd := range verifyCommands {
		if i > 0 {
			verifyCmdsStr += "\n"
		}
		verifyCmdsStr += "  " + cmd
	}

	return fmt.Errorf("permission denied: cannot %s namespace %q\n\n"+
		"Your Kubernetes user does not have permission to %s namespaces.\n"+
		"This is typically a Teleport or RBAC configuration issue.\n\n"+
		"To resolve this:\n"+
		"  1. Ask your Teleport/cluster admin to grant access to namespace %q\n"+
		"     in the \"kubernetes_resources\" field of your Teleport role\n"+
		"%s"+
		"To verify your permissions, run:\n"+
		"%s\n\n"+
		"Original error: %w", operation, namespace, verb, namespace, additionalHint, verifyCmdsStr, err)
}

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

	// Check if namespace exists and is terminating
	if err := c.waitForNamespaceNotTerminating(ctx, namespace, 5*time.Minute); err != nil {
		return err
	}

	// Check if namespace exists
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	namespaceExists := !apierrors.IsNotFound(err)
	if err != nil && !apierrors.IsNotFound(err) {
		// If it's a permission error checking existence, we'll try to create anyway
		if apierrors.IsForbidden(err) {
			namespaceExists = false // Assume it doesn't exist and try create
		} else {
			return fmt.Errorf("failed to check if namespace %q exists: %w", namespace, err)
		}
	}

	if !namespaceExists {
		// Namespace doesn't exist, use Create() which only requires create permission
		logging.Logger.Debug().Str("namespace", namespace).Msg("creating namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsForbidden(err) {
				return formatNamespacePermissionError("create", namespace, "create", err)
			}
			if apierrors.IsAlreadyExists(err) {
				// Namespace was created between our check and create attempt
				logging.Logger.Debug().Str("namespace", namespace).Msg("namespace already exists")
				return nil
			}
			return fmt.Errorf("failed to create namespace %q (context=%q): %w", namespace, c.kubeContext, err)
		}
		logging.Logger.Debug().Str("namespace", namespace).Msg("namespace created successfully")
		return nil
	}

	// Namespace exists, use Apply() to update it (requires patch permission)
	logging.Logger.Debug().Str("namespace", namespace).Msg("applying namespace")
	nsApply := corev1apply.Namespace(namespace)
	_, err = c.clientset.CoreV1().Namespaces().Apply(ctx, nsApply, defaultApplyOptions())
	if err != nil {
		if apierrors.IsForbidden(err) {
			return formatNamespacePermissionError("update", namespace, "patch", err)
		}
		return fmt.Errorf("failed to apply namespace %q (context=%q): %w", namespace, c.kubeContext, err)
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("namespace applied successfully")
	return nil
}

// waitForNamespaceNotTerminating checks if a namespace is terminating and waits for deletion to complete
func (c *Client) waitForNamespaceNotTerminating(ctx context.Context, namespace string, timeout time.Duration) error {
	ns, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Namespace doesn't exist, we can proceed
			return nil
		}
		if apierrors.IsForbidden(err) {
			// On Teleport-managed clusters, a freshly created namespace may return
			// Forbidden until RBAC propagates. Treat as "doesn't exist yet" and let
			// EnsureNamespace attempt the create — if that also fails, it's a real error.
			logging.Logger.Debug().
				Str("namespace", namespace).
				Msg("namespace get returned forbidden (possible Teleport RBAC propagation delay), proceeding")
			return nil
		}
		return fmt.Errorf("failed to check namespace status: %w", err)
	}

	// Check if namespace is terminating
	if ns.Status.Phase != corev1.NamespaceTerminating {
		// Namespace exists and is not terminating
		return nil
	}

	logging.Logger.Info().
		Str("namespace", namespace).
		Msg("Namespace is currently being deleted, waiting for deletion to complete...")

	// Wait for namespace to be fully deleted
	startTime := time.Now()
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			logging.Logger.Info().
				Str("namespace", namespace).
				Str("duration", time.Since(startTime).String()).
				Msg("Namespace deletion completed")
			return true, nil
		}
		if err != nil {
			if apierrors.IsForbidden(err) {
				return false, formatNamespacePermissionError("check status of", namespace, "get", err)
			}
			return false, err
		}

		elapsed := time.Since(startTime)
		if int(elapsed.Seconds())%10 == 0 {
			logging.Logger.Debug().
				Str("namespace", namespace).
				Str("elapsed", elapsed.String()).
				Msg("Still waiting for namespace deletion...")
		}
		return false, nil
	})
}

func (c *Client) EnsureDockerRegistrySecret(ctx context.Context, namespace, username, password string) error {
	return c.EnsureRegistrySecret(ctx, namespace, "registry-camunda-cloud", "registry.camunda.cloud", username, password)
}

// EnsureDockerHubSecret creates or updates the "index-docker-io" pull secret for Docker Hub.
func (c *Client) EnsureDockerHubSecret(ctx context.Context, namespace, username, password string) error {
	return c.EnsureRegistrySecret(ctx, namespace, "index-docker-io", "https://index.docker.io/v1/", username, password)
}

// EnsureRegistrySecret creates or updates a Docker registry pull secret with the given
// secret name and registry URL in the specified namespace. Credentials are required;
// if either username or password is empty the call returns an error.
//
// The function first attempts server-side apply (PATCH). If that fails with Forbidden
// (e.g. Teleport proxies that reject apply-patch content types), it falls back to a
// classic Get → Create/Update flow.
func (c *Client) EnsureRegistrySecret(ctx context.Context, namespace, secretName, registryURL, username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("cannot create docker registry secret %q in namespace %q: credentials not provided (both username and password are required)", secretName, namespace)
	}

	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("secret", secretName).
		Str("registry", registryURL).
		Msg("creating/updating docker registry secret")

	dockerConfigJSON, err := buildDockerConfigJSON(registryURL, username, password)
	if err != nil {
		return fmt.Errorf("failed to marshal docker config: %w", err)
	}

	secretApply := corev1apply.Secret(secretName, namespace).
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
		// Retry once for transient network/API-server hiccups.
		if isTransientKubeApplyError(err) {
			logging.Logger.Warn().
				Err(err).
				Str("namespace", namespace).
				Str("secret", secretName).
				Msg("transient error applying docker registry secret, retrying once")
			select {
			case <-ctx.Done():
				return fmt.Errorf("failed to apply docker registry secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, ctx.Err())
			case <-time.After(2 * time.Second):
			}
			_, err = c.clientset.CoreV1().Secrets(namespace).Apply(
				ctx,
				secretApply,
				defaultApplyOptions(),
			)
		}
	}
	if err != nil {
		if checkNamespaceTerminating(err) {
			return fmt.Errorf("failed to apply docker registry secret %q in namespace %q (context=%q): namespace is currently being deleted, please wait for deletion to complete or use a different namespace: %w", secretName, namespace, c.kubeContext, err)
		}
		// Fall back to Create/Update when server-side apply is rejected (e.g. Teleport proxy).
		if apierrors.IsForbidden(err) {
			logging.Logger.Warn().
				Err(err).
				Str("namespace", namespace).
				Str("secret", secretName).
				Msg("server-side apply forbidden (possible Teleport proxy limitation), falling back to create/update")
			return c.createOrUpdateSecret(ctx, namespace, secretName, corev1.SecretTypeDockerConfigJson, map[string][]byte{
				corev1.DockerConfigJsonKey: dockerConfigJSON,
			})
		}
		return fmt.Errorf("failed to apply docker registry secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, err)
	}

	return nil
}

func buildDockerConfigJSON(registryURL, username, password string) ([]byte, error) {
	dockerConfig := map[string]any{
		"auths": map[string]any{
			registryURL: map[string]any{
				"username": username,
				"password": password,
				"auth":     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
			},
		},
	}
	return json.Marshal(dockerConfig)
}

func checkNamespaceTerminating(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "is being terminated") || strings.Contains(msg, "because it is being terminated")
}

// createOrUpdateSecret performs a classic Get → Create/Update flow for a Secret.
// This is used as a fallback when server-side apply is rejected (e.g. by Teleport proxies
// that don't support the PATCH apply-patch+yaml content type).
func (c *Client) createOrUpdateSecret(ctx context.Context, namespace, secretName string, secretType corev1.SecretType, data map[string][]byte) error {
	secretsClient := c.clientset.CoreV1().Secrets(namespace)

	existing, err := secretsClient.Get(ctx, secretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
		// On Teleport-managed clusters, a freshly created namespace may return
		// Forbidden instead of NotFound until RBAC propagates. Treat both as
		// "secret doesn't exist yet" and attempt Create.
		if apierrors.IsForbidden(err) {
			logging.Logger.Debug().
				Str("namespace", namespace).
				Str("secret", secretName).
				Msg("get secret returned forbidden (possible Teleport RBAC propagation delay), attempting create")
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Type: secretType,
			Data: data,
		}
		_, createErr := secretsClient.Create(ctx, secret, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, createErr)
		}
		logging.Logger.Info().
			Str("namespace", namespace).
			Str("secret", secretName).
			Msg("created secret via fallback create")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, err)
	}

	existing.Data = data
	existing.Type = secretType
	_, updateErr := secretsClient.Update(ctx, existing, metav1.UpdateOptions{})
	if updateErr != nil {
		return fmt.Errorf("failed to update secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, updateErr)
	}
	logging.Logger.Info().
		Str("namespace", namespace).
		Str("secret", secretName).
		Msg("updated secret via fallback update")
	return nil
}

// EnsureOpaqueSecret creates or updates an Opaque secret with string data in the
// specified namespace. Attempts server-side apply first, falling back to Create/Update
// if Forbidden (e.g. Teleport proxy).
func (c *Client) EnsureOpaqueSecret(ctx context.Context, namespace, secretName string, stringData map[string]string) error {
	if secretName == "" {
		return errors.New("secret name must not be empty")
	}

	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("secret", secretName).
		Msg("creating/updating opaque secret")

	secretApply := corev1apply.Secret(secretName, namespace).
		WithType(corev1.SecretTypeOpaque).
		WithStringData(stringData)

	_, err := c.clientset.CoreV1().Secrets(namespace).Apply(
		ctx,
		secretApply,
		defaultApplyOptions(),
	)
	if err != nil {
		if checkNamespaceTerminating(err) {
			return fmt.Errorf("failed to apply opaque secret %q in namespace %q (context=%q): namespace is currently being deleted, please wait for deletion to complete or use a different namespace: %w", secretName, namespace, c.kubeContext, err)
		}
		if apierrors.IsForbidden(err) {
			logging.Logger.Warn().
				Err(err).
				Str("namespace", namespace).
				Str("secret", secretName).
				Msg("server-side apply forbidden, falling back to create/update")
			return c.createOrUpdateOpaqueSecret(ctx, namespace, secretName, stringData)
		}
		return fmt.Errorf("failed to apply opaque secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, err)
	}

	return nil
}

func (c *Client) createOrUpdateOpaqueSecret(ctx context.Context, namespace, secretName string, stringData map[string]string) error {
	secretsClient := c.clientset.CoreV1().Secrets(namespace)

	existing, err := secretsClient.Get(ctx, secretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
		// On Teleport-managed clusters, a freshly created namespace may return
		// Forbidden instead of NotFound until RBAC propagates. Treat both as
		// "secret doesn't exist yet" and attempt Create.
		if apierrors.IsForbidden(err) {
			logging.Logger.Debug().
				Str("namespace", namespace).
				Str("secret", secretName).
				Msg("get opaque secret returned forbidden (possible Teleport RBAC propagation delay), attempting create")
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Type:       corev1.SecretTypeOpaque,
			StringData: stringData,
		}
		_, createErr := secretsClient.Create(ctx, secret, metav1.CreateOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create opaque secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, createErr)
		}
		logging.Logger.Info().
			Str("namespace", namespace).
			Str("secret", secretName).
			Msg("created opaque secret via fallback create")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get opaque secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, err)
	}

	existing.StringData = stringData
	_, updateErr := secretsClient.Update(ctx, existing, metav1.UpdateOptions{})
	if updateErr != nil {
		return fmt.Errorf("failed to update opaque secret %q in namespace %q (context=%q): %w", secretName, namespace, c.kubeContext, updateErr)
	}
	logging.Logger.Info().
		Str("namespace", namespace).
		Str("secret", secretName).
		Msg("updated opaque secret via fallback update")
	return nil
}

// GetSecretData reads a Kubernetes Secret and returns its data values as decoded strings.
// If the secret does not exist, it returns (nil, nil) — callers should check for a nil map.
// Only keys with non-empty values are included.
func (c *Client) GetSecretData(ctx context.Context, namespace, secretName string) (map[string]string, error) {
	secret, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get secret %q in namespace %q: %w", secretName, namespace, err)
	}

	result := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		if len(v) > 0 {
			result[k] = string(v)
		}
	}
	return result, nil
}

func (c *Client) DeleteNamespace(ctx context.Context, namespace string) error {
	logging.Logger.Debug().Str("namespace", namespace).Msg("deleting namespace")

	err := c.clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logging.Logger.Debug().Str("namespace", namespace).Msg("namespace not found, nothing to delete")
			return nil
		}
		if apierrors.IsForbidden(err) {
			return formatNamespacePermissionError("delete", namespace, "delete", err)
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
			if apierrors.IsForbidden(err) {
				return false, formatNamespacePermissionError("check deletion status of", namespace, "get", err)
			}
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
		// Check if error is due to namespace termination
		if strings.Contains(err.Error(), "is being terminated") || strings.Contains(err.Error(), "because it is being terminated") {
			return fmt.Errorf("failed to apply ConfigMap %q in namespace %q (context=%q): namespace is currently being deleted, please wait for deletion to complete or use a different namespace: %w", name, namespace, c.kubeContext, err)
		}
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

	result, err := c.clientset.CoreV1().Namespaces().Apply(ctx, nsApply, defaultApplyOptions())
	if err != nil {
		logging.Logger.Warn().
			Err(err).
			Str("namespace", namespace).
			Str("errorType", fmt.Sprintf("%T", err)).
			Bool("isForbidden", apierrors.IsForbidden(err)).
			Bool("isUnauthorized", apierrors.IsUnauthorized(err)).
			Msg("server-side apply failed for namespace labels/annotations")
		// Check if error is due to namespace termination
		if strings.Contains(err.Error(), "is being terminated") || strings.Contains(err.Error(), "because it is being terminated") {
			return fmt.Errorf("failed to apply namespace %q labels/annotations (context=%q): namespace is currently being deleted, please wait for deletion to complete or use a different namespace: %w", namespace, c.kubeContext, err)
		}
		if apierrors.IsForbidden(err) {
			logging.Logger.Warn().
				Err(err).
				Str("namespace", namespace).
				Msg("server-side apply forbidden for namespace labels/annotations, falling back to JSON merge patch")
			return c.patchNamespaceLabelsAndAnnotations(ctx, namespace, labels, annotations)
		}
		return fmt.Errorf("failed to apply namespace %q labels/annotations (context=%q): %w", namespace, c.kubeContext, err)
	}

	logging.Logger.Info().
		Str("namespace", namespace).
		Interface("resultLabels", result.Labels).
		Interface("resultAnnotations", result.Annotations).
		Int("labelCount", len(labels)).
		Int("annotationCount", len(annotations)).
		Msg("server-side apply succeeded for namespace labels/annotations")

	return nil
}

func (c *Client) patchNamespaceLabelsAndAnnotations(ctx context.Context, namespace string, labels, annotations map[string]string) error {
	const maxRetries = 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ns, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get namespace %q for label/annotation update (context=%q): %w", namespace, c.kubeContext, err)
		}
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		for k, v := range labels {
			ns.Labels[k] = v
		}
		for k, v := range annotations {
			ns.Annotations[k] = v
		}
		_, err = c.clientset.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
		if err == nil {
			logging.Logger.Info().
				Str("namespace", namespace).
				Int("attempt", attempt).
				Msg("updated namespace labels/annotations via fallback GET+UPDATE")
			return nil
		}
		lastErr = err
		logging.Logger.Warn().
			Err(err).
			Str("namespace", namespace).
			Int("attempt", attempt).
			Int("maxRetries", maxRetries).
			Msg("fallback GET+UPDATE failed, retrying")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("failed to update namespace %q labels/annotations after %d attempts (context=%q): %w", namespace, maxRetries, c.kubeContext, lastErr)
}

const (
	platformGKE  = "gke"
	platformROSA = "rosa"
	platformEKS  = "eks"

	secretNameTLS = "aws-camunda-cloud-tls"
)

func ApplyExternalSecretsAndCerts(ctx context.Context, kubeconfig, kubeContext, platform, repoRoot, chartPath, namespace, namespacePrefix, externalSecretsStore string) error {
	platform = strings.ToLower(strings.TrimSpace(platform))

	logging.Logger.Debug().
		Str("platform", platform).
		Str("namespace", namespace).
		Str("chartPath", chartPath).
		Str("repoRoot", repoRoot).
		Str("externalSecretsStore", externalSecretsStore).
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
		logging.Logger.Warn().Msg("ExternalSecrets CRD not installed - skipping external secrets setup. TLS certificates and infra credentials will need to be configured manually.")
		return nil
	}

	provider, err := NewPlatformSecretsProvider(platform, repoRoot, chartPath, namespacePrefix, externalSecretsStore)
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
	// Use utilyaml.NewYAMLOrJSONDecoder to properly handle multi-document YAML files
	// This is the standard Kubernetes approach for parsing manifests (used by kubectl, etc.)
	appliedCount := 0
	decoder := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)

	for docNum := 1; ; docNum++ {
		var obj map[string]any
		err := decoder.Decode(&obj)

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to decode document %d: %w", docNum, err)
		}

		// Skip empty documents
		if len(obj) == 0 {
			continue
		}

		if err := applySingleManifestObject(ctx, client, namespace, obj, docNum); err != nil {
			return err
		}
		appliedCount++
	}

	logging.Logger.Debug().
		Int("documentsApplied", appliedCount).
		Str("namespace", namespace).
		Msg("successfully applied all manifest documents")

	return nil
}

func applySingleManifestObject(ctx context.Context, client *Client, namespace string, obj map[string]any, docNum int) error {
	unstructuredObj := &unstructured.Unstructured{Object: obj}
	gvk := unstructuredObj.GroupVersionKind()

	if unstructuredObj.GetAPIVersion() == "" || unstructuredObj.GetKind() == "" {
		return fmt.Errorf("document %d missing apiVersion or kind", docNum)
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
		Int("documentNumber", docNum).
		Msg("applying resource")

	data, err := json.Marshal(unstructuredObj.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal resource for apply (document %d): %w", docNum, err)
	}

	// Apply with retry logic for webhook availability errors
	// This handles the case where the external-secrets webhook is not yet ready
	const maxRetries = 5
	initialDelay := 10 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err = client.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
			ctx,
			unstructuredObj.GetName(),
			types.ApplyPatchType,
			data,
			defaultPatchOptions(),
		)
		if err == nil {
			if attempt > 1 {
				logging.Logger.Info().
					Str("kind", gvk.Kind).
					Str("name", unstructuredObj.GetName()).
					Str("namespace", namespace).
					Int("attempts", attempt).
					Msg("successfully applied resource after retries")
			}
			return nil
		}

		lastErr = err

		// Check if error is due to namespace termination (non-retryable)
		if strings.Contains(err.Error(), "is being terminated") || strings.Contains(err.Error(), "because it is being terminated") {
			return fmt.Errorf("failed to apply %s %q in namespace %q (document %d): namespace is currently being deleted, please wait for deletion to complete or use a different namespace: %w", gvk.Kind, unstructuredObj.GetName(), namespace, docNum, err)
		}

		// Determine if the error is retryable (webhook not ready OR transient network error)
		retryable := isWebhookNotReadyError(err) || isTransientKubeApplyError(err)
		if !retryable {
			// Non-retryable error, fail immediately
			return fmt.Errorf("failed to apply %s %q in namespace %q (document %d): %w", gvk.Kind, unstructuredObj.GetName(), namespace, docNum, err)
		}

		// Retryable error - retry with exponential backoff
		if attempt == maxRetries {
			// Exhausted all retries
			logging.Logger.Error().
				Str("kind", gvk.Kind).
				Str("name", unstructuredObj.GetName()).
				Str("namespace", namespace).
				Int("attempts", attempt).
				Msg("retryable error persists after all retry attempts")
			return fmt.Errorf("failed to apply %s %q in namespace %q (document %d) after %d attempts: %w", gvk.Kind, unstructuredObj.GetName(), namespace, docNum, maxRetries, lastErr)
		}

		delay := initialDelay * time.Duration(1<<(attempt-1)) // Exponential backoff: 10s, 20s, 40s, 80s, 160s
		logging.Logger.Warn().
			Str("kind", gvk.Kind).
			Str("name", unstructuredObj.GetName()).
			Str("namespace", namespace).
			Int("attempt", attempt).
			Int("maxRetries", maxRetries).
			Dur("retryDelay", delay).
			Err(err).
			Msg("retryable error applying resource, retrying...")

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting to retry applying %s %q: %w", gvk.Kind, unstructuredObj.GetName(), ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// This should not be reached, but handle it just in case
	return fmt.Errorf("failed to apply %s %q in namespace %q (document %d): %w", gvk.Kind, unstructuredObj.GetName(), namespace, docNum, lastErr)
}

// isWebhookNotReadyError checks if the error is due to a webhook not being ready.
// This typically happens when the external-secrets webhook hasn't registered its endpoints yet.
//
// Common error patterns include:
//   - "Internal error occurred: failed calling webhook ... no endpoints available for service"
//   - "failed to call webhook: Post ... connection refused"
//   - "failed to call webhook: Post ... service unavailable"
func isWebhookNotReadyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()

	// Must contain "webhook" to be a webhook-related error
	if !strings.Contains(errStr, "webhook") {
		return false
	}

	// Check for specific webhook unavailability patterns
	webhookUnavailablePatterns := []string{
		"no endpoints available",
		"connection refused",
		"failed to call webhook",
		"service unavailable",
		"Internal error occurred",
	}

	for _, pattern := range webhookUnavailablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
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
		Version:  "v1",
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

// DeleteNamespace deletes a Kubernetes namespace using kubectl.
// This ensures proper handling of Teleport and other proxy configurations
// that may not be fully supported by the Go client-go library.
func DeleteNamespace(ctx context.Context, kubeconfig, kubeContext, namespace string) error {
	logging.Logger.Debug().Str("namespace", namespace).Msg("deleting namespace via kubectl")

	// Build common kubectl connection args
	var kubeArgs []string
	if kubeconfig != "" {
		kubeArgs = append(kubeArgs, "--kubeconfig", kubeconfig)
	}
	if kubeContext != "" {
		kubeArgs = append(kubeArgs, "--context", kubeContext)
	}

	// First check if namespace exists using --ignore-not-found which won't error if missing
	// We use "get" with -o name to minimize output - it returns "namespace/<name>" if exists
	getArgs := append([]string{"get", "namespace", namespace, "--ignore-not-found", "-o", "name"}, kubeArgs...)
	getCmdStr := "kubectl " + strings.Join(getArgs, " ")

	logging.Logger.Debug().Str("command", getCmdStr).Msg("checking if namespace exists")

	getCmd := exec.CommandContext(ctx, "kubectl", getArgs...)
	getOutput, err := getCmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(getOutput))

	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("output", outputStr).
		Err(err).
		Msg("namespace existence check result")

	if err != nil {
		return fmt.Errorf("failed to check if namespace %q exists\n\n"+
			"To check manually, run:\n"+
			"  %s\n\n"+
			"Output:\n%s", namespace, getCmdStr, outputStr)
	}

	// Check if the output contains the namespace name pattern "namespace/<name>"
	// This is more robust than checking for empty string, as Teleport may add warnings
	expectedPattern := "namespace/" + namespace
	if !strings.Contains(outputStr, expectedPattern) {
		logging.Logger.Debug().
			Str("namespace", namespace).
			Str("output", outputStr).
			Str("expectedPattern", expectedPattern).
			Msg("namespace does not exist (pattern not found in output), nothing to delete")
		return nil
	}

	// Namespace exists, proceed with deletion
	logging.Logger.Debug().Str("namespace", namespace).Msg("namespace exists, proceeding with deletion")

	deleteArgs := append([]string{"delete", "namespace", namespace, "--wait=true", "--timeout=5m"}, kubeArgs...)
	deleteCmdStr := "kubectl " + strings.Join(deleteArgs, " ")

	logging.Logger.Debug().Str("command", deleteCmdStr).Msg("deleting namespace")

	deleteCmd := exec.CommandContext(ctx, "kubectl", deleteArgs...)
	deleteOutput, err := deleteCmd.CombinedOutput()

	if err != nil {
		deleteOutputStr := strings.TrimSpace(string(deleteOutput))

		if strings.Contains(strings.ToLower(deleteOutputStr), "deleted") || strings.Contains(strings.ToLower(deleteOutputStr), "not found") {
			logging.Logger.Debug().
				Str("namespace", namespace).
				Str("output", deleteOutputStr).
				Msg("namespace delete returned non-zero but namespace is already gone")
			return nil
		}

		return fmt.Errorf("failed to delete namespace %q\n\n"+
			"To delete manually, run:\n"+
			"  %s\n\n"+
			"Output:\n%s", namespace, deleteCmdStr, deleteOutputStr)
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("namespace deleted successfully via kubectl")
	return nil
}

// CheckConnectivity performs a lightweight API call (ServerVersion) to verify
// that the given kube context has valid credentials. For Teleport-managed
// clusters this triggers the interactive browser login if needed, ensuring
// subsequent parallel calls don't race on the login prompt.
func CheckConnectivity(ctx context.Context, kubeContext string) error {
	client, err := NewClient("", kubeContext)
	if err != nil {
		return fmt.Errorf("kube context %q: %w", kubeContext, err)
	}
	if _, err := client.clientset.Discovery().ServerVersion(); err != nil {
		return fmt.Errorf("kube context %q: failed to connect: %w", kubeContext, err)
	}
	return nil
}

func isTransientKubeApplyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	transientHints := []string{
		"connection reset by peer",
		"i/o timeout",
		"tls handshake timeout",
		"context deadline exceeded",
		"temporarily unavailable",
		"service unavailable",
		"too many requests",
		"eof",
	}
	for _, hint := range transientHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}
	return false
}

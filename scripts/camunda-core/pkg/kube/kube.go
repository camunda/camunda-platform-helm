package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"
	"strings"
	"syscall"
	"time"

	"github.com/jwalton/gchalk"
	"golang.org/x/term"
)

func EnsureNamespace(ctx context.Context, kubeconfig, kubeContext, namespace string) error {
	if namespace == "" {
		return errors.New("namespace must not be empty")
	}
	logging.Logger.Debug().Str("namespace", namespace).Msg("checking if namespace exists")
	args := append([]string{"get", "ns", namespace}, composeKubeArgs(kubeconfig, kubeContext)...)
	if err := exec.CommandContext(ctx, "kubectl", args...).Run(); err == nil {
		logging.Logger.Debug().Str("namespace", namespace).Msg("namespace already exists")
		return nil
	}
	logging.Logger.Debug().Str("namespace", namespace).Msg("creating namespace")
	createArgs := append([]string{"create", "ns", namespace}, composeKubeArgs(kubeconfig, kubeContext)...)
	return executil.RunCommand(ctx, "kubectl", createArgs, nil, "")
}

func LabelAndAnnotateNamespace(ctx context.Context, kubeconfig, kubeContext, namespace, identifier, flow, ttl string, ghRunID string, ghJobID string, ghOrg string, ghRepo string, workflowURL string) error {
	labels := []string{}
	if strings.TrimSpace(identifier) != "" {
		labels = append(labels, "github-id="+identifier)
	}
	if strings.TrimSpace(flow) != "" {
		labels = append(labels, "test-flow="+flow)
	}
	if strings.TrimSpace(ghRunID) != "" {
		labels = append(labels, "github-run-id="+ghRunID)
	}
	if strings.TrimSpace(ghJobID) != "" {
		labels = append(labels, "github-job-id="+ghJobID)
	}
	if strings.TrimSpace(ghOrg) != "" {
		labels = append(labels, "github-org="+ghOrg)
	}
	if strings.TrimSpace(ghRepo) != "" {
		labels = append(labels, "github-repo="+ghRepo)
	}
	if len(labels) > 0 {
		logging.Logger.Debug().
			Str("namespace", namespace).
			Strs("labels", labels).
			Msg("applying labels to namespace")
		args := append([]string{"label", "ns", namespace}, labels...)
		args = append(args, "--overwrite=true")
		args = append(args, composeKubeArgs(kubeconfig, kubeContext)...)
		if err := executil.RunCommand(ctx, "kubectl", args, nil, ""); err != nil {
			return err
		}
	}
	// TTL annotations and ephemeral flag
	if strings.TrimSpace(ttl) == "" {
		ttl = "12h"
	}
	ann := []string{
		"cleaner/ttl=" + ttl,
		"janitor/ttl=" + ttl,
		"camunda.cloud/ephemeral=true",
	}
	if strings.TrimSpace(workflowURL) != "" {
		ann = append(ann, "github-workflow-run-url="+workflowURL)
	}
	logging.Logger.Debug().
		Str("namespace", namespace).
		Strs("annotations", ann).
		Msg("applying annotations to namespace")
	args := append([]string{"annotate", "ns", namespace}, ann...)
	args = append(args, "--overwrite=true")
	args = append(args, composeKubeArgs(kubeconfig, kubeContext)...)
	return executil.RunCommand(ctx, "kubectl", args, nil, "")
}

func ComposeKubeArgs(kubeconfig, kubeContext string) []string {
	return composeKubeArgs(kubeconfig, kubeContext)
}

func composeKubeArgs(kubeconfig, kubeContext string) []string {
	args := make([]string, 0, 4)
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	return args
}

func FormatCmd(name string, args []string) string {
	return name + " " + fmt.Sprint(args)
}

func EnsureDockerRegistrySecret(ctx context.Context, kubeconfig, kubeContext, namespace string) error {
	return EnsureDockerRegistrySecretWithCreds(ctx, kubeconfig, kubeContext, namespace, "", "")
}

// EnsureDockerRegistrySecretWithCreds creates/updates docker registry secret with provided credentials
// If username/password are empty, checks environment variables
func EnsureDockerRegistrySecretWithCreds(ctx context.Context, kubeconfig, kubeContext, namespace, username, password string) error {
	if username == "" {
		username = firstNonEmpty(os.Getenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD"), os.Getenv("NEXUS_USERNAME"))
	}
	if password == "" {
		password = firstNonEmpty(os.Getenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"), os.Getenv("NEXUS_PASSWORD"))
	}
	if username == "" || password == "" {
		// Nothing to create; skip quietly
		logging.Logger.Debug().Str("namespace", namespace).Msg("skipping docker registry secret creation (credentials not provided)")
		return nil
	}
	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("secret", "registry-camunda-cloud").
		Msg("creating/updating docker registry secret")
	baseArgs := composeKubeArgs(kubeconfig, kubeContext)
	createArgs := []string{
		"create", "secret", "docker-registry", "registry-camunda-cloud",
		"--namespace", namespace,
		"--docker-server", "registry.camunda.cloud",
		"--docker-username", username,
		"--docker-password", password,
		"--dry-run=client", "-o", "yaml",
	}
	yamlOut, err := executil.RunCommandCapture(ctx, "kubectl", createArgs, nil, "")
	if err != nil {
		return err
	}
	applyArgs := []string{"apply", "-n", namespace, "-f", "-"}
	applyArgs = append(applyArgs, baseArgs...)
	return executil.RunCommandWithStdin(ctx, "kubectl", applyArgs, nil, "", yamlOut)
}

func EnsureTLSSecret(ctx context.Context, kubeconfig, kubeContext, namespace, name, certPath, keyPath string) error {
	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("secret", name).
		Str("cert", certPath).
		Str("key", keyPath).
		Msg("creating/updating TLS secret")
	baseArgs := composeKubeArgs(kubeconfig, kubeContext)
	createArgs := []string{
		"create", "secret", "tls", name,
		"--namespace", namespace,
		"--cert", certPath,
		"--key", keyPath,
		"--dry-run=client", "-o", "yaml",
	}
	yamlOut, err := executil.RunCommandCapture(ctx, "kubectl", createArgs, nil, "")
	if err != nil {
		return err
	}
	applyArgs := []string{"apply", "-n", namespace, "-f", "-"}
	applyArgs = append(applyArgs, baseArgs...)
	return executil.RunCommandWithStdin(ctx, "kubectl", applyArgs, nil, "", yamlOut)
}

func ApplyIntegrationTestCredentialsIfPresent(ctx context.Context, kubeconfig, kubeContext, namespace string) error {
	b64 := strings.TrimSpace(os.Getenv("INTEGRATION_TEST_CREDENTIALS"))
	if b64 == "" {
		logging.Logger.Debug().Str("namespace", namespace).Msg("skipping integration-test credentials (env not present)")
		return nil
	}
	logging.Logger.Debug().Str("namespace", namespace).Msg("applying integration-test credentials from env")
	decoded, err := decodeBase64(b64)
	if err != nil {
		return err
	}
	args := append([]string{"apply", "-n", namespace, "-f", "-"}, composeKubeArgs(kubeconfig, kubeContext)...)
	return executil.RunCommandWithStdin(ctx, "kubectl", args, nil, "", decoded)
}

const (
	platformGKE  = "gke"
	platformROSA = "rosa"
	platformEKS  = "eks"
	
	externalSecretsReadyTimeout = 300 * time.Second
	secretNameTLS               = "aws-camunda-cloud-tls"
)

// ApplyExternalSecretsAndCerts configures external secrets/certs for the given platform.
// Supports GKE, ROSA (ExternalSecrets-based), and EKS (secret copy-based).
func ApplyExternalSecretsAndCerts(ctx context.Context, kubeconfig, kubeContext, platform, repoRoot, chartPath, namespace, namespacePrefix string) error {
	platform = strings.ToLower(strings.TrimSpace(platform))
	
	logging.Logger.Debug().
		Str("platform", platform).
		Str("namespace", namespace).
		Str("chartPath", chartPath).
		Str("repoRoot", repoRoot).
		Msg("applying external secrets/certs")
	
	switch platform {
	case platformGKE, platformROSA:
		return applyExternalSecretsForGKERosa(ctx, kubeconfig, kubeContext, repoRoot, chartPath, namespace)
	case platformEKS:
		return applyTLSSecretForEKS(ctx, kubeconfig, kubeContext, namespace, namespacePrefix)
	default:
		return fmt.Errorf("unsupported platform %q (supported: gke, rosa, eks)", platform)
	}
}

// applyExternalSecretsForGKERosa applies ExternalSecret manifests for GKE/ROSA platforms
func applyExternalSecretsForGKERosa(ctx context.Context, kubeconfig, kubeContext, repoRoot, chartPath, namespace string) error {
	externalSecretDir := filepath.Join(repoRoot, ".github", "config", "external-secret")
	
	// 1. Apply certificates ExternalSecret
	if err := applyManifestIfExists(ctx, kubeconfig, kubeContext, namespace,
		filepath.Join(externalSecretDir, "external-secret-certificates.yaml"),
		"certificates external-secret"); err != nil {
		return fmt.Errorf("apply certificates: %w", err)
	}
	
	// 2. Apply infra secrets ExternalSecret
	if err := applyManifestIfExists(ctx, kubeconfig, kubeContext, namespace,
		filepath.Join(externalSecretDir, "external-secret-infra.yaml"),
		"infra-secrets external-secret"); err != nil {
		return fmt.Errorf("apply infra secrets: %w", err)
	}
	
	// 3. Apply integration test credentials (chart-specific or fallback)
	chartSpecific := filepath.Join(chartPath, "test", "integration", "external-secrets", "external-secret-integration-test-credentials.yaml")
	fallback := filepath.Join(externalSecretDir, "external-secret-integration-test-credentials.yaml")
	
	if fileExists(chartSpecific) {
		if err := applyManifestFile(ctx, kubeconfig, kubeContext, namespace, chartSpecific); err != nil {
			return fmt.Errorf("apply chart-specific integration-test credentials: %w", err)
		}
		logging.Logger.Debug().Str("file", chartSpecific).Msg("applied chart-specific integration-test external-secret")
	} else if fileExists(fallback) {
		if err := applyManifestFile(ctx, kubeconfig, kubeContext, namespace, fallback); err != nil {
			return fmt.Errorf("apply fallback integration-test credentials: %w", err)
		}
		logging.Logger.Debug().Str("file", fallback).Msg("applied fallback integration-test external-secret")
	} else {
		logging.Logger.Debug().Msg("no integration-test external-secret manifest found (optional, continuing)")
	}
	
	// 4. Wait for all ExternalSecrets to become Ready
	logging.Logger.Debug().Str("namespace", namespace).Msg("waiting for ExternalSecrets to become Ready")
	if err := waitExternalSecretsReady(ctx, kubeconfig, kubeContext, namespace, externalSecretsReadyTimeout); err != nil {
		return fmt.Errorf("wait for ExternalSecrets ready: %w", err)
	}
	
	return nil
}

// applyTLSSecretForEKS copies TLS secret from source namespace for EKS platform
func applyTLSSecretForEKS(ctx context.Context, kubeconfig, kubeContext, namespace, namespacePrefix string) error {
	srcNamespace := computeEKSSourceNamespace(namespacePrefix)
	
	logging.Logger.Debug().
		Str("srcNamespace", srcNamespace).
		Str("destNamespace", namespace).
		Str("secret", secretNameTLS).
		Msg("copying TLS secret for EKS")
	
	if err := copySecretBetweenNamespaces(ctx, kubeconfig, kubeContext, srcNamespace, secretNameTLS, namespace); err != nil {
		return fmt.Errorf("copy TLS secret from %s to %s: %w", srcNamespace, namespace, err)
	}
	
	return nil
}

// computeEKSSourceNamespace determines the source namespace for EKS secret copy
func computeEKSSourceNamespace(namespacePrefix string) string {
	prefix := strings.TrimSpace(namespacePrefix)
	if prefix == "" {
		return "certs"
	}
	return prefix + "-certs"
}

// applyManifestIfExists applies a manifest file if it exists, logging and skipping if not
func applyManifestIfExists(ctx context.Context, kubeconfig, kubeContext, namespace, filePath, description string) error {
	if !fileExists(filePath) {
		logging.Logger.Debug().Str("file", filePath).Msgf("%s manifest not found (skipping)", description)
		return nil
	}
	
	logging.Logger.Debug().Str("file", filePath).Msgf("applying %s", description)
	if err := applyManifestFile(ctx, kubeconfig, kubeContext, namespace, filePath); err != nil {
		return err
	}
	
	return nil
}

func applyManifestFile(ctx context.Context, kubeconfig, kubeContext, namespace, filePath string) error {
	if !fileExists(filePath) {
		return fmt.Errorf("manifest file not found: %s", filePath)
	}
	logging.Logger.Debug().Str("file", filePath).Str("namespace", namespace).Msg("kubectl apply manifest file")
	args := append([]string{"apply", "-n", namespace, "-f", filePath}, composeKubeArgs(kubeconfig, kubeContext)...)
	return executil.RunCommand(ctx, "kubectl", args, nil, "")
}

func waitExternalSecretsReady(ctx context.Context, kubeconfig, kubeContext, namespace string, timeout time.Duration) error {
	// List ExternalSecret names; if CRD absent or none present, skip quietly
	args := append([]string{"get", "externalsecret", "-n", namespace, "-o", "jsonpath={.items[*].metadata.name}"}, composeKubeArgs(kubeconfig, kubeContext)...)
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		// likely CRD not installed; skip
		logging.Logger.Debug().Str("namespace", namespace).Msg("external-secrets CRD not present or list failed; skipping wait")
		return nil
	}
	names := strings.Fields(string(out))
	if len(names) == 0 {
		logging.Logger.Debug().Str("namespace", namespace).Msg("no ExternalSecrets found; skipping wait")
		return nil
	}
	logging.Logger.Debug().Str("namespace", namespace).Int("count", len(names)).Msg("waiting for ExternalSecrets readiness")
	for _, name := range names {
		waitArgs := append([]string{"wait", "--for=condition=Ready", "externalsecret/" + name, "-n", namespace, fmt.Sprintf("--timeout=%ds", int(timeout.Seconds()))}, composeKubeArgs(kubeconfig, kubeContext)...)
		if err := executil.RunCommand(ctx, "kubectl", waitArgs, nil, ""); err != nil {
			return err
		}
	}
	return nil
}

func copySecretBetweenNamespaces(ctx context.Context, kubeconfig, kubeContext, srcNamespace, secretName, destNamespace string) error {
	// Get secret as JSON
	logging.Logger.Debug().
		Str("srcNamespace", srcNamespace).
		Str("destNamespace", destNamespace).
		Str("secret", secretName).
		Msg("copying secret JSON for transform")
	getArgs := append([]string{"get", "secret", "-n", srcNamespace, secretName, "-o", "json"}, composeKubeArgs(kubeconfig, kubeContext)...)
	data, err := exec.CommandContext(ctx, "kubectl", getArgs...).Output()
	if err != nil {
		return fmt.Errorf("failed to get secret %s in ns %s: %w", secretName, srcNamespace, err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	// Strip metadata fields
	if meta, ok := m["metadata"].(map[string]interface{}); ok {
		delete(meta, "uid")
		delete(meta, "resourceVersion")
		delete(meta, "creationTimestamp")
		delete(meta, "selfLink")
		delete(meta, "managedFields")
		meta["namespace"] = destNamespace
	}
	// Write to temp file and apply
	tmp, err := ioutil.TempFile("", "secret-copy-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	enc, _ := json.Marshal(m)
	if _, err := tmp.Write(enc); err != nil {
		return err
	}
	_ = tmp.Close()
	logging.Logger.Debug().
		Str("tempFile", tmp.Name()).
		Str("destNamespace", destNamespace).
		Msg("applying transformed secret to destination namespace")
	applyArgs := append([]string{"apply", "-n", destNamespace, "-f", tmp.Name()}, composeKubeArgs(kubeconfig, kubeContext)...)
	return executil.RunCommand(ctx, "kubectl", applyArgs, nil, "")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func decodeBase64(s string) ([]byte, error) {
	// Avoid importing encoding/base64 at top unnecessarily
	return exec.Command("bash", "-lc", fmt.Sprintf("printf %%s %q | base64 -d", s)).Output()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// DeleteNamespace deletes the given namespace and waits for completion.
func DeleteNamespace(ctx context.Context, kubeconfig, kubeContext, namespace string) error {
	logging.Logger.Debug().Str("namespace", namespace).Msg("deleting namespace with wait=true")
	args := append([]string{"delete", "ns", namespace, "--wait=true"}, composeKubeArgs(kubeconfig, kubeContext)...)
	return executil.RunCommand(ctx, "kubectl", args, nil, "")
}

type nsInfo struct {
	Metadata struct {
		Name              string            `json:"name"`
		CreationTimestamp string            `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
	} `json:"metadata"`
	Age string `json:"-"`
}

func getNamespaceInfo(ctx context.Context, kubeconfig, kubeContext, namespace string) (*nsInfo, error) {
	args := append([]string{"get", "ns", namespace, "-o", "json"}, composeKubeArgs(kubeconfig, kubeContext)...)
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, err
	}
	var ni nsInfo
	if err := json.Unmarshal(out, &ni); err != nil {
		return nil, err
	}
	if ni.Metadata.CreationTimestamp != "" {
		if dur := sinceRFC3339(ni.Metadata.CreationTimestamp); dur != "" {
			ni.Age = dur
		}
	}
	return &ni, nil
}

func sinceRFC3339(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	// Human-friendly, rounded to minutes if > 1m
	if d >= time.Minute {
		return (d - (d % time.Minute)).String()
	}
	return d.String()
}

func formatMap(m map[string]string) string {
	if len(m) == 0 {
		return "-"
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}

func selectKeys(m map[string]string, keys ...string) map[string]string {
	if len(m) == 0 {
		return m
	}
	out := map[string]string{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

// PromptDockerRegistryCredentials prompts the user for docker registry username and password
func PromptDockerRegistryCredentials(in io.Reader) (username, password string, err error) {
	fmt.Println()
	fmt.Println(logging.Emphasize("═══════════════════════════════════════════════════════════", gchalk.Cyan))
	fmt.Println(logging.Emphasize("Docker Registry Credentials Required", gchalk.Yellow))
	fmt.Println(logging.Emphasize("═══════════════════════════════════════════════════════════", gchalk.Cyan))
	fmt.Println()
	fmt.Printf("Registry: %s\n", logging.Emphasize("registry.camunda.cloud", gchalk.Green))
	fmt.Println()
	fmt.Println("These credentials are used to pull Enterprise images from the Camunda registry.")
	fmt.Println()
	fmt.Println(logging.Emphasize("Where to find your credentials:", gchalk.Cyan))
	fmt.Println("  • Use your Camunda LDAP credentials (same as Camunda Cloud/Zeebe Cloud)")
	fmt.Println("  • Or set environment variables to avoid prompts:")
	fmt.Printf("    %s\n", logging.Emphasize("export TEST_DOCKER_USERNAME_CAMUNDA_CLOUD=<your-username>", gchalk.Blue))
	fmt.Printf("    %s\n", logging.Emphasize("export TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD=<your-password>", gchalk.Blue))
	fmt.Println()
	fmt.Println("  • Alternative env vars also supported:")
	fmt.Printf("    %s / %s\n", logging.Emphasize("NEXUS_USERNAME", gchalk.Blue), logging.Emphasize("NEXUS_PASSWORD", gchalk.Blue))
	fmt.Println()
	fmt.Println(logging.Emphasize("═══════════════════════════════════════════════════════════", gchalk.Cyan))
	fmt.Println()
	fmt.Printf("%s ", logging.Emphasize("Docker registry username:", gchalk.Cyan))
	if _, err := fmt.Fscanln(in, &username); err != nil {
		return "", "", fmt.Errorf("failed to read username: %w", err)
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return "", "", errors.New("username cannot be empty")
	}

	fmt.Printf("%s ", logging.Emphasize("Docker registry password:", gchalk.Cyan))
	// Read password securely (hide input)
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", fmt.Errorf("failed to read password: %w", err)
	}
	password = strings.TrimSpace(string(passwordBytes))
	fmt.Println() // New line after password input
	if password == "" {
		return "", "", errors.New("password cannot be empty")
	}
	return username, password, nil
}

// GetDockerRegistryCredentials returns docker registry credentials from environment or prompts if missing
// Returns username, password, and whether they were prompted (true) or from env (false)
func GetDockerRegistryCredentials(in io.Reader, promptIfMissing bool) (username, password string, prompted bool, err error) {
	username = firstNonEmpty(os.Getenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD"), os.Getenv("NEXUS_USERNAME"))
	password = firstNonEmpty(os.Getenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"), os.Getenv("NEXUS_PASSWORD"))

	if username == "" || password == "" {
		if !promptIfMissing {
			errMsg := "docker registry credentials not found in environment variables\n\n"
			errMsg += "Registry: registry.camunda.cloud\n"
			errMsg += "Required environment variables:\n"
			errMsg += "  • TEST_DOCKER_USERNAME_CAMUNDA_CLOUD (or NEXUS_USERNAME)\n"
			errMsg += "  • TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD (or NEXUS_PASSWORD)\n\n"
			errMsg += "These should be your Camunda LDAP credentials (same as Camunda Cloud/Zeebe Cloud).\n"
			errMsg += "Set them before running the command, or run without --yes to be prompted interactively."
			return "", "", false, errors.New(errMsg)
		}
		username, password, err = PromptDockerRegistryCredentials(in)
		if err != nil {
			return "", "", false, err
		}
		return username, password, true, nil
	}
	return username, password, false, nil
}

// EnsureDockerLogin ensures docker is logged in to Docker Hub (docker.io)
// If username/password are empty, checks environment variables
func EnsureDockerLogin(ctx context.Context, username, password string) error {
	if username == "" {
		username = firstNonEmpty(os.Getenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD"), os.Getenv("NEXUS_USERNAME"))
	}
	if password == "" {
		password = firstNonEmpty(os.Getenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"), os.Getenv("NEXUS_PASSWORD"))
	}
	if username == "" || password == "" {
		// No credentials available, skip docker login
		logging.Logger.Debug().Msg("skipping docker login (credentials not provided)")
		return nil
	}

	logging.Logger.Debug().Str("registry", "docker.io").Msg("ensuring docker login")

	// Use --password-stdin for secure password input
	// Docker Hub is the default registry, so no need to specify it explicitly
	args := []string{"login", "--username", username, "--password-stdin"}
	return executil.RunCommandWithStdin(ctx, "docker", args, nil, "", []byte(password))
}

// LoadKeycloakRealmConfigMap creates a Keycloak realm ConfigMap from a realm.json file.
// It modifies the realm.json to use the specified realmName and creates a ConfigMap named 'realm-json'
// in the target namespace.
func LoadKeycloakRealmConfigMap(ctx context.Context, kubeconfig, kubeContext, chartPath, realmName, namespace string) error {
	logging.Logger.Debug().
		Str("chartPath", chartPath).
		Str("realmName", realmName).
		Str("namespace", namespace).
		Msg("loading Keycloak realm ConfigMap")

	// Locate the realm.json file
	realmJSONPath := filepath.Join(chartPath, "test", "integration", "realm.json")
	if _, err := os.Stat(realmJSONPath); err != nil {
		logging.Logger.Debug().Str("path", realmJSONPath).Msg("realm.json not found")
		return fmt.Errorf("realm.json not found at %s: %w", realmJSONPath, err)
	}

	logging.Logger.Debug().Str("path", realmJSONPath).Msg("found realm.json")

	// Read and parse the realm.json
	data, err := os.ReadFile(realmJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read realm.json: %w", err)
	}

	var realmConfig map[string]interface{}
	if err := json.Unmarshal(data, &realmConfig); err != nil {
		return fmt.Errorf("failed to parse realm.json: %w", err)
	}

	logging.Logger.Debug().Msg("updating realm configuration")

	// Update realm configuration with the specified realm name
	realmConfig["id"] = realmName
	realmConfig["realm"] = realmName

	// Update realm roles if they exist
	if roles, ok := realmConfig["roles"].(map[string]interface{}); ok {
		if realmRoles, ok := roles["realm"].([]interface{}); ok {
			for _, role := range realmRoles {
				if roleMap, ok := role.(map[string]interface{}); ok {
					roleMap["containerId"] = realmName
				}
			}
		}
	}

	// Update defaultRole if it exists
	if defaultRole, ok := realmConfig["defaultRole"].(map[string]interface{}); ok {
		defaultRole["containerId"] = realmName
	}

	// Marshal the modified config
	modifiedData, err := json.MarshalIndent(realmConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modified realm config: %w", err)
	}

	// Create a temporary file for the modified realm.json
	tmpFile, err := os.CreateTemp("", "realm-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(modifiedData); err != nil {
		return fmt.Errorf("failed to write temporary realm file: %w", err)
	}
	tmpFile.Close()

	logging.Logger.Debug().Str("tmpFile", tmpFile.Name()).Msg("created temporary realm.json")

	// Create the ConfigMap using kubectl
	logging.Logger.Debug().Str("configMap", "realm-json").Msg("creating ConfigMap")

	args := []string{
		"create", "configmap", "realm-json",
		"-n", namespace,
		"--from-file=realm.json=" + tmpFile.Name(),
		"--dry-run=client",
		"-o", "yaml",
	}
	args = append(args, ComposeKubeArgs(kubeconfig, kubeContext)...)

	// Execute kubectl create with dry-run
	output, err := executil.RunCommandCapture(ctx, "kubectl", args, nil, "")
	if err != nil {
		return fmt.Errorf("failed to generate ConfigMap YAML: %w", err)
	}

	// Apply the ConfigMap
	applyArgs := []string{"apply", "-f", "-"}
	applyArgs = append(applyArgs, ComposeKubeArgs(kubeconfig, kubeContext)...)

	if err := executil.RunCommandWithStdin(ctx, "kubectl", applyArgs, nil, "", output); err != nil {
		return fmt.Errorf("failed to apply ConfigMap: %w", err)
	}

	logging.Logger.Debug().
		Str("configMap", "realm-json").
		Str("namespace", namespace).
		Msg("Keycloak realm ConfigMap applied successfully")

	return nil
}

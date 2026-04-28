package types

import (
	"context"
	"time"
)

type ValuesInput struct {
	ChartPath string
	Scenarios []string
	Auth      string
}

type Options struct {
	Chart       string
	ChartPath   string
	Version     string
	RealmPath   string
	ReleaseName string
	Namespace   string
	Kubeconfig  string
	KubeContext string

	Timeout     time.Duration
	Wait        bool
	Atomic      bool
	IngressHost string

	ValuesFiles []string
	SetPairs    map[string]string
	ExtraArgs   []string

	Identifier string
	TTL        string

	// CI metadata (optional)
	CIMetadata CIMetadata

	// Registry/cluster behaviors
	EnsureDockerRegistry   bool   // Create registry-camunda-cloud pull secret (Harbor)
	DockerRegistryUsername string // Harbor username (falls back to HARBOR_USERNAME, TEST_DOCKER_USERNAME_CAMUNDA_CLOUD, NEXUS_USERNAME)
	DockerRegistryPassword string // Harbor password (falls back to HARBOR_PASSWORD, TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD, NEXUS_PASSWORD)
	EnsureDockerHub        bool   // Create index-docker-io pull secret (Docker Hub)
	DockerHubUsername      string // Docker Hub username (falls back to DOCKERHUB_USERNAME, TEST_DOCKER_USERNAME)
	DockerHubPassword      string // Docker Hub password (falls back to DOCKERHUB_PASSWORD, TEST_DOCKER_PASSWORD)
	SkipDockerLogin        bool
	SkipDependencyUpdate   bool
	ApplyIntegrationCreds  bool

	PostRendererPath string

	ExternalSecretsEnabled bool
	ExternalSecretsStore   string // external secrets store type (e.g., "vault-backend")
	Platform               string // gke|rosa|eks
	NamespacePrefix        string // for eks copy
	RepoRoot               string // repo base for manifests

	TLSSecretName string
	TLSCertPath   string
	TLSKeyPath    string

	LoadKeycloakRealm bool
	KeycloakRealmName string
	VaultSecretPath   string

	// Template rendering (no cluster changes)
	RenderTemplates bool
	RenderOutputDir string
	IncludeCRDs     bool

	// PreInstallHooks are functions called after the namespace and registry
	// secrets are set up but before helm upgrade/install. This allows callers
	// to create K8s resources (e.g., OIDC credential secrets) that must exist
	// in the namespace at install time but cannot be created earlier because
	// the namespace may not yet exist or may be recreated.
	PreInstallHooks []func(ctx context.Context) error

	// CompanionCharts are Helm charts deployed as separate releases in the
	// same namespace before the main Camunda chart. Each companion chart is
	// deployed with helm upgrade --install --wait to ensure it is ready
	// before the main chart deployment begins.
	CompanionCharts []CompanionChart
}

// CompanionChart represents a Helm chart that should be deployed as a
// separate release before the main Camunda chart.
type CompanionChart struct {
	// ChartRef is the Helm chart reference — either a repo/chart name
	// (e.g., "opensearch/opensearch") or an absolute local path.
	ChartRef string
	// Version is the chart version to install (e.g., "3.6.0").
	// Empty means latest (for remote) or ignore (for local).
	Version string
	// ReleaseName is the Helm release name for this companion chart.
	ReleaseName string
	// ValuesFile is the absolute path to a values file. Empty means use chart defaults.
	ValuesFile string
	// RepoName is the Helm repository name to register before installing
	// (e.g., "opensearch"). Empty means no repo registration is needed.
	RepoName string
	// RepoURL is the Helm repository URL
	// (e.g., "https://opensearch-project.github.io/helm-charts/").
	RepoURL string
}

type CIMetadata struct {
	GithubRunID string
	GithubJobID string
	GithubOrg   string
	GithubRepo  string
	WorkflowURL string
	Flow        string
}

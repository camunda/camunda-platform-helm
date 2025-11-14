package types

import "time"

// ValuesInput specifies how to build the layered values list
type ValuesInput struct {
	ChartPath string
	Scenarios []string
	Auth      string
}

// Options is the stable set of deployment options.
type Options struct {
	ChartPath   string
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
	EnsureDockerRegistry   bool
	DockerRegistryUsername string
	DockerRegistryPassword string
	SkipDockerLogin        bool
	SkipDependencyUpdate   bool
	ApplyIntegrationCreds  bool

	// Optional post-renderer script/path
	PostRendererPath string

	// Cluster secrets/certs setup
	ExternalSecretsEnabled bool
	Platform               string // gke|rosa|eks
	NamespacePrefix        string // for eks copy
	RepoRoot               string // repo base for manifests

	// Optional TLS secret creation from files
	TLSSecretName string
	TLSCertPath   string
	TLSKeyPath    string

	// Keycloak realm configuration
	LoadKeycloakRealm bool
	KeycloakRealmName string
}

// CIMetadata holds optional CI/CD workflow metadata
type CIMetadata struct {
	GithubRunID string
	GithubJobID string
	GithubOrg   string
	GithubRepo  string
	WorkflowURL string
	Flow        string
}


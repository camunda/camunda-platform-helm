package matrix

// RunOptions controls matrix execution.
type RunOptions struct {
	// DryRun logs what would be done without executing.
	DryRun bool
	// StopOnFailure stops the run on the first failure.
	// In parallel mode, this cancels in-flight entries and prevents new ones from starting.
	StopOnFailure bool
	// Cleanup deletes each entry's namespace immediately after its deployment
	// and tests complete (regardless of success or failure). This frees cluster
	// resources as early as possible rather than waiting for the entire run to finish.
	Cleanup bool
	// KubeContexts maps platform names to Kubernetes contexts, e.g.,
	// {"gke": "gke_my-project_us-east1_cluster", "eks": "arn:aws:eks:..."}
	// When an entry's platform matches a key, that context is used for deployment and cleanup.
	KubeContexts map[string]string
	// KubeContext is a fallback Kubernetes context used when no platform-specific
	// context is configured. If both KubeContexts and KubeContext are set, the
	// platform-specific context takes priority.
	KubeContext string
	// NamespacePrefix is prepended to generated namespaces.
	NamespacePrefix string
	// Platform overrides the platform for all entries.
	Platform string
	// MaxParallel controls how many entries run concurrently.
	// 0 or 1 means sequential execution (default). Values > 1 enable parallel execution
	// with at most MaxParallel entries running simultaneously.
	MaxParallel int
	// TestIT runs integration tests after each deployment.
	TestIT bool
	// TestE2E runs e2e tests after each deployment.
	TestE2E bool
	// TestAll runs both integration and e2e tests after each deployment.
	TestAll bool
	// RepoRoot is the repository root path.
	RepoRoot string
	// EnvFiles maps chart versions to .env file paths, e.g.,
	// {"8.9": ".env.89", "8.8": ".env.88"}
	// When an entry's version matches a key, that .env file is loaded before deployment.
	EnvFiles map[string]string
	// EnvFile is a fallback .env file used when no version-specific file is configured.
	// If both EnvFiles and EnvFile are set, the version-specific file takes priority.
	EnvFile string
	// KeycloakHost is the external Keycloak hostname.
	// Defaults to config.DefaultKeycloakHost when empty.
	KeycloakHost string
	// KeycloakProtocol is the protocol for the external Keycloak (e.g., "https").
	// Defaults to config.DefaultKeycloakProtocol when empty.
	KeycloakProtocol string
	// IngressBaseDomains maps platform names to ingress base domains, e.g.,
	// {"gke": "ci.distro.ultrawombat.com", "eks": "distribution.aws.camunda.cloud"}
	// When an entry's platform matches a key, that domain is used for ingress hostname construction.
	IngressBaseDomains map[string]string
	// IngressBaseDomain is a fallback base domain for ingress hosts used when no
	// platform-specific domain is configured. If both IngressBaseDomains and
	// IngressBaseDomain are set, the platform-specific domain takes priority.
	// Valid values: ci.distro.ultrawombat.com, distribution.aws.camunda.cloud
	IngressBaseDomain string
	// LogLevel controls the log verbosity for each entry's deployment.
	// Valid values: debug, info, warn, error. Defaults to "info" if empty.
	LogLevel string
	// SkipDependencyUpdate skips running "helm dependency update" before deploying.
	// Default is false, meaning dependency update runs for every entry.
	SkipDependencyUpdate bool
	// VaultBackedSecrets maps platform names to whether vault-backed secrets should be used, e.g.,
	// {"eks": true, "gke": false}
	// When an entry's platform matches a key, the corresponding value controls whether
	// the vault-backend ClusterSecretStore and -vault.yaml manifest variants are selected.
	VaultBackedSecrets map[string]bool
	// UseVaultBackedSecrets is a fallback for platforms not in VaultBackedSecrets.
	// If both VaultBackedSecrets and UseVaultBackedSecrets are set, the platform-specific
	// value takes priority.
	UseVaultBackedSecrets bool
	// DeleteNamespaceFirst deletes the namespace before deploying each matrix entry.
	// This ensures a clean-slate deployment by removing any existing resources in the namespace.
	DeleteNamespaceFirst bool
	// Coverage produces a layer-breakdown report showing what IS tested in the matrix.
	// Behaves like DryRun (no deployment), but outputs a focused table showing each
	// scenario's resolved layers (identity, persistence, platform, infra-type, features, flow).
	Coverage bool
	// UpgradeFromVersion overrides the auto-resolved "from" chart version for upgrade flows.
	// When set, this version is used instead of resolving from version-matrix JSON files.
	// Only applies to entries with upgrade flows (upgrade-patch, upgrade-minor, modular-upgrade-minor).
	UpgradeFromVersion string
	// HelmTimeout is the timeout in minutes for each Helm deployment.
	// Applies uniformly to all matrix entries (install, upgrade Step 1, upgrade Step 2).
	// When <= 0, deploy.Execute defaults to 5 minutes.
	HelmTimeout int
	// DockerUsername is the Harbor registry username for pulling images.
	// When empty, the deployer falls back to HARBOR_USERNAME, TEST_DOCKER_USERNAME_CAMUNDA_CLOUD, or NEXUS_USERNAME env vars.
	DockerUsername string
	// DockerPassword is the Harbor registry password for pulling images.
	// When empty, the deployer falls back to HARBOR_PASSWORD, TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD, or NEXUS_PASSWORD env vars.
	DockerPassword string
	// EnsureDockerRegistry creates a Harbor registry secret in each entry's namespace.
	// When true, the deployer performs docker login and creates a registry-camunda-cloud
	// Kubernetes secret of type kubernetes.io/dockerconfigjson.
	EnsureDockerRegistry bool
	// DockerHubUsername is the Docker Hub registry username.
	// When empty, the deployer falls back to DOCKERHUB_USERNAME or TEST_DOCKER_USERNAME env vars.
	DockerHubUsername string
	// DockerHubPassword is the Docker Hub registry password.
	// When empty, the deployer falls back to DOCKERHUB_PASSWORD or TEST_DOCKER_PASSWORD env vars.
	DockerHubPassword string
	// EnsureDockerHub creates a Docker Hub pull secret (index-docker-io) in each entry's namespace.
	// When true, the deployer performs docker login and creates an index-docker-io
	// Kubernetes secret of type kubernetes.io/dockerconfigjson.
	EnsureDockerHub bool
	// UseLatest applies values-latest.yaml from each chart root instead of values-digest.yaml.
	// This overrides the default digest-based image pinning with the latest available tags.
	UseLatest bool
	// UseQA forces the base-qa layer to be included for all entries, regardless of each
	// entry's per-scenario qa setting in ci-test-config.yaml.
	UseQA bool
	// OnEntryStart is called when a matrix entry begins execution.
	// The callback receives the entry and its resolved namespace.
	// Nil disables the callback (zero overhead for existing CLI behavior).
	OnEntryStart func(entry Entry, namespace string)
	// OnEntryComplete is called when a matrix entry finishes execution.
	// The callback receives the entry and its full result (including error and duration).
	// Nil disables the callback (zero overhead for existing CLI behavior).
	OnEntryComplete func(entry Entry, result RunResult)
	// OnPhaseChange is called when a matrix entry transitions to a new phase
	// (e.g., "preparing", "deploying", "step-1", "step-2", "testing", "cleanup").
	// Nil disables the callback.
	OnPhaseChange func(entry Entry, phase string)
	// LogDir is the directory for per-entry log files. When set, test script
	// output (IT/e2e) is redirected to per-entry files instead of the terminal.
	LogDir string
	// ExtraHelmArgs are appended to every helm command for every entry. CI uses
	// this to inject license-key --set-file flags whose values would otherwise be
	// shell-escaped incorrectly via --set.
	ExtraHelmArgs []string
	// ExtraHelmSets are key=value strings applied as extra --set pairs for every
	// entry. CI uses this for invariant flags like
	// orchestration.upgrade.allowPreReleaseImages=true.
	ExtraHelmSets []string
	// NamespaceOverride, when non-empty, replaces the computed namespace for
	// every entry. Used by per-scenario CI workflows that pre-create the
	// namespace (with vault secrets, TLS certs, docker pull-secrets) before
	// invoking matrix run, so the install lands in the same namespace.
	// Only meaningful when filters narrow the run to a single entry.
	NamespaceOverride string
	// ChartRef, when non-empty, overrides the chart source for helm install/upgrade.
	// This can be an OCI reference (e.g., "oci://registry.camunda.cloud/team-distribution/camunda-platform")
	// or a path to a local .tgz file. When set, the matrix runner uses this as the
	// chart argument for `helm upgrade --install` instead of the local chart directory.
	// The local chart directory (entry.ChartPath) is still used for values file resolution
	// (scenario layers, chart-root overlays). SkipDependencyUpdate is forced to true.
	ChartRef string
	// ChartRefVersion is the chart version to install from ChartRef (e.g., "13-rc-latest").
	// Only meaningful when ChartRef is set. Passed as --version to helm.
	ChartRefVersion string
	// ForceImageOverrides bypasses the OCI immutability guard. By default, an
	// external chart reference is deployed with the image versions baked into
	// that chart artifact, without chart-root image overlays or image-tag env
	// substitutions from the local git checkout.
	ForceImageOverrides bool
}

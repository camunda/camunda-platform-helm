package config

// MergeIntField applies the matrix/root value to target when the CLI flag was
// not explicitly set by the user. Pointer-typed config values (nil = unset)
// allow distinguishing "not configured" from zero.
func MergeIntField(target *int, matrixVal, rootVal *int, changedFlags map[string]bool, flagName string) {
	if changedFlags != nil && changedFlags[flagName] {
		return // CLI flag was explicitly set; do not override
	}
	if matrixVal != nil {
		*target = *matrixVal
	} else if rootVal != nil {
		*target = *rootVal
	}
}

// MergeStringMapField copies src into dst for any keys not already present.
// This is used for per-platform/per-version maps (kubeContexts, envFiles, etc.).
func MergeStringMapField(dst map[string]string, src map[string]string) {
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}

// MergeBoolMapField copies src into dst for any keys not already present.
// This is used for per-platform bool maps (vaultBackedSecrets).
func MergeBoolMapField(dst map[string]bool, src map[string]bool) {
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}

// MatrixListFlags holds all the local flag variables from the "matrix list" command
// that can be populated from config.
type MatrixListFlags struct {
	Versions        *[]string
	IncludeDisabled *bool
	ScenarioFilter  *string
	ShortnameFilter *string
	FlowFilter      *string
	OutputFormat    *string
	Platform        *string
	RepoRoot        *string
}

// ApplyMatrixListConfig merges config-file values into the matrix list command flags.
// Precedence: CLI flags (if in changedFlags) > matrix config > root config > flag defaults.
func ApplyMatrixListConfig(rc *RootConfig, changedFlags map[string]bool, f *MatrixListFlags) {
	if rc == nil {
		return
	}
	m := &rc.Matrix

	// Filtering fields — matrix-only, no root fallback
	MergeStringField(f.RepoRoot, m.RepoRoot, rc.RepoRoot, changedFlags, "repo-root")
	MergeStringField(f.Platform, m.Platform, rc.Platform, changedFlags, "platform")
	MergeStringField(f.ScenarioFilter, m.ScenarioFilter, "", changedFlags, "scenario-filter")
	MergeStringField(f.ShortnameFilter, m.ShortnameFilter, "", changedFlags, "shortname-filter")
	MergeStringField(f.FlowFilter, m.FlowFilter, "", changedFlags, "flow-filter")
	MergeStringField(f.OutputFormat, m.OutputFormat, "", changedFlags, "format")
	MergeBoolField(f.IncludeDisabled, m.IncludeDisabled, nil, changedFlags, "include-disabled")
	MergeStringSliceField(f.Versions, m.Versions, nil)
}

// MatrixRunFlags holds all the local flag variables from the "matrix run" command
// that can be populated from config. Callers pass pointers to their local variables.
type MatrixRunFlags struct {
	// Filtering & generation
	Versions        *[]string
	IncludeDisabled *bool
	ScenarioFilter  *string
	ShortnameFilter *string
	FlowFilter      *string
	Platform        *string
	RepoRoot        *string

	// Execution
	DryRun               *bool
	Coverage             *bool
	StopOnFailure        *bool
	Cleanup              *bool
	DeleteNamespace      *bool
	NamespacePrefix      *string
	MaxParallel          *int
	LogLevel             *string
	SkipDependencyUpdate *bool
	HelmTimeout          *int

	// Tests
	TestIT  *bool
	TestE2E *bool
	TestAll *bool

	// Kube contexts
	KubeContext    *string
	KubeContextGKE *string
	KubeContextEKS *string
	// KubeContexts is the assembled map (populated by the command before calling this)
	KubeContexts map[string]string

	// Ingress
	IngressBaseDomain    *string
	IngressBaseDomainGKE *string
	IngressBaseDomainEKS *string
	// IngressBaseDomains is the assembled map
	IngressBaseDomains map[string]string

	// Vault
	UseVaultBackedSecrets    *bool
	UseVaultBackedSecretsGKE *bool
	UseVaultBackedSecretsEKS *bool
	// VaultBackedSecrets is the assembled map
	VaultBackedSecrets map[string]bool

	// Env files
	EnvFile   *string
	EnvFile86 *string
	EnvFile87 *string
	EnvFile88 *string
	EnvFile89 *string
	// EnvFiles is the assembled map
	EnvFiles map[string]string

	// Docker
	DockerUsername       *string
	DockerPassword       *string
	EnsureDockerRegistry *bool
	DockerHubUsername    *string
	DockerHubPassword    *string
	EnsureDockerHub      *bool

	// Keycloak
	KeycloakHost     *string
	KeycloakProtocol *string

	// Upgrade
	UpgradeFromVersion *string
}

// ApplyMatrixRunConfig merges config-file values into the matrix run command flags.
// Precedence: CLI flags (if in changedFlags) > matrix config > root config > flag defaults.
//
// For per-platform maps (KubeContexts, IngressBaseDomains, VaultBackedSecrets) and
// per-version maps (EnvFiles), config values are merged into the already-assembled maps
// (keys from CLI flags take precedence over config keys).
func ApplyMatrixRunConfig(rc *RootConfig, changedFlags map[string]bool, f *MatrixRunFlags) {
	if rc == nil {
		return
	}
	m := &rc.Matrix

	// --- Filtering & generation ---
	MergeStringSliceField(f.Versions, m.Versions, nil)
	MergeBoolField(f.IncludeDisabled, m.IncludeDisabled, nil, changedFlags, "include-disabled")
	MergeStringField(f.ScenarioFilter, m.ScenarioFilter, "", changedFlags, "scenario-filter")
	MergeStringField(f.ShortnameFilter, m.ShortnameFilter, "", changedFlags, "shortname-filter")
	MergeStringField(f.FlowFilter, m.FlowFilter, "", changedFlags, "flow-filter")
	MergeStringField(f.Platform, m.Platform, rc.Platform, changedFlags, "platform")
	MergeStringField(f.RepoRoot, m.RepoRoot, rc.RepoRoot, changedFlags, "repo-root")

	// --- Execution ---
	MergeStringField(f.NamespacePrefix, m.NamespacePrefix, "", changedFlags, "namespace-prefix")
	MergeIntField(f.MaxParallel, m.MaxParallel, nil, changedFlags, "max-parallel")
	MergeBoolField(f.StopOnFailure, m.StopOnFailure, nil, changedFlags, "stop-on-failure")
	MergeBoolField(f.Cleanup, m.Cleanup, nil, changedFlags, "cleanup")
	MergeBoolField(f.DeleteNamespace, m.DeleteNamespace, nil, changedFlags, "delete-namespace")
	MergeBoolField(f.DryRun, m.DryRun, nil, changedFlags, "dry-run")
	MergeBoolField(f.Coverage, m.Coverage, nil, changedFlags, "coverage")
	MergeBoolField(f.SkipDependencyUpdate, m.SkipDependencyUpdate, rc.SkipDependencyUpdate, changedFlags, "skip-dependency-update")
	MergeIntField(f.HelmTimeout, m.HelmTimeout, nil, changedFlags, "timeout")
	MergeStringField(f.LogLevel, m.LogLevel, rc.LogLevel, changedFlags, "log-level")

	// --- Tests ---
	MergeBoolField(f.TestIT, m.TestIT, nil, changedFlags, "test-it")
	MergeBoolField(f.TestE2E, m.TestE2E, nil, changedFlags, "test-e2e")
	MergeBoolField(f.TestAll, m.TestAll, nil, changedFlags, "test-all")

	// --- Kube contexts ---
	// Scalar fallbacks (default context for all platforms)
	MergeStringField(f.KubeContext, m.KubeContext, rc.KubeContext, changedFlags, "kube-context")
	// Per-platform map: merge config map into the CLI-assembled map
	if f.KubeContexts != nil {
		MergeStringMapField(f.KubeContexts, m.KubeContexts)
	}

	// --- Ingress base domains ---
	MergeStringField(f.IngressBaseDomain, m.IngressBaseDomain, rc.IngressBaseDomain, changedFlags, "ingress-base-domain")
	if f.IngressBaseDomains != nil {
		MergeStringMapField(f.IngressBaseDomains, m.IngressBaseDomains)
	}

	// --- Vault-backed secrets ---
	MergeBoolField(f.UseVaultBackedSecrets, m.UseVaultBackedSecrets, nil, changedFlags, "use-vault-backed-secrets")
	if f.VaultBackedSecrets != nil {
		MergeBoolMapField(f.VaultBackedSecrets, m.VaultBackedSecrets)
	}

	// --- Env files ---
	MergeStringField(f.EnvFile, m.EnvFile, rc.EnvFile, changedFlags, "env-file")
	if f.EnvFiles != nil {
		MergeStringMapField(f.EnvFiles, m.EnvFiles)
	}

	// --- Docker ---
	MergeStringField(f.DockerUsername, m.DockerUsername, rc.DockerUsername, changedFlags, "docker-username")
	MergeStringField(f.DockerPassword, m.DockerPassword, rc.DockerPassword, changedFlags, "docker-password")
	MergeBoolField(f.EnsureDockerRegistry, m.EnsureDockerRegistry, rc.EnsureDockerRegistry, changedFlags, "ensure-docker-registry")
	MergeStringField(f.DockerHubUsername, m.DockerHubUsername, rc.DockerHubUsername, changedFlags, "dockerhub-username")
	MergeStringField(f.DockerHubPassword, m.DockerHubPassword, rc.DockerHubPassword, changedFlags, "dockerhub-password")
	MergeBoolField(f.EnsureDockerHub, m.EnsureDockerHub, rc.EnsureDockerHub, changedFlags, "ensure-docker-hub")

	// --- Keycloak ---
	MergeStringField(f.KeycloakHost, m.KeycloakHost, rc.Keycloak.Host, changedFlags, "keycloak-host")
	MergeStringField(f.KeycloakProtocol, m.KeycloakProtocol, rc.Keycloak.Protocol, changedFlags, "keycloak-protocol")

	// --- Upgrade ---
	MergeStringField(f.UpgradeFromVersion, m.UpgradeFromVersion, "", changedFlags, "upgrade-from-version")
}

// LoadMatrixConfig loads the config file and returns the parsed RootConfig
// suitable for use by matrix subcommands. Environment overrides are applied.
func LoadMatrixConfig(configPath string) (*RootConfig, error) {
	res, err := ResolvePath(configPath)
	if err != nil {
		return nil, err
	}
	return Read(res.Path, true)
}

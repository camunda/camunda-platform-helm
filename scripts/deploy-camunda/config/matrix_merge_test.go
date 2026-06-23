package config

import "testing"

// allStringPtrs returns a MatrixRunFlags with every *string/*int/*bool field
// pointing at fresh zero values, so ApplyMatrixRunConfig can dereference them
// without nil panics. Tests then inspect the ones they care about.
func newMatrixRunFlags() (*MatrixRunFlags, *string, *string, *string, *string, *string) {
	var platform, repoRoot, kubeContext, ingress, envFile string
	var (
		versions                                      []string
		includeDisabled, dryRun, coverage, stopOnFail bool
		cleanup, deleteNS, skipDep, testE2E           bool
		testAll, useVault, ensureReg, ensureHub       bool
		maxParallel, helmTimeout                      int
		scenarioFilter, shortnameFilter, flowFilter   string
		namespacePrefix, logLevel                     string
		kubeGKE, kubeEKS, ingressGKE, ingressEKS      string
		dockerUser, dockerPass, hubUser, hubPass      string
		keycloakHost, keycloakProto, upgradeFrom      string
	)
	f := &MatrixRunFlags{
		Versions: &versions, IncludeDisabled: &includeDisabled,
		ScenarioFilter: &scenarioFilter, ShortnameFilter: &shortnameFilter, FlowFilter: &flowFilter,
		Platform: &platform, RepoRoot: &repoRoot,
		DryRun: &dryRun, Coverage: &coverage, StopOnFailure: &stopOnFail, Cleanup: &cleanup,
		DeleteNamespace: &deleteNS, NamespacePrefix: &namespacePrefix, MaxParallel: &maxParallel,
		LogLevel: &logLevel, SkipDependencyUpdate: &skipDep, HelmTimeout: &helmTimeout,
		TestE2E: &testE2E, TestAll: &testAll,
		KubeContext: &kubeContext, KubeContextGKE: &kubeGKE, KubeContextEKS: &kubeEKS,
		IngressBaseDomain: &ingress, IngressBaseDomainGKE: &ingressGKE, IngressBaseDomainEKS: &ingressEKS,
		UseVaultBackedSecrets: &useVault,
		EnvFile:               &envFile,
		DockerUsername:        &dockerUser, DockerPassword: &dockerPass,
		EnsureDockerRegistry: &ensureReg, DockerHubUsername: &hubUser, DockerHubPassword: &hubPass,
		EnsureDockerHub: &ensureHub,
		KeycloakHost:    &keycloakHost, KeycloakProtocol: &keycloakProto,
		UpgradeFromVersion: &upgradeFrom,
	}
	return f, &platform, &kubeContext, &ingress, &repoRoot, &envFile
}

func TestApplyMatrixRunConfigUsesActiveDeploymentProfile(t *testing.T) {
	rc := &RootConfig{
		Current: "local",
		Deployments: map[string]DeploymentConfig{
			"local": {InfraConfig: InfraConfig{
				Platform:          "gke",
				KubeContext:       "gke-ctx-from-profile",
				IngressBaseDomain: "ci.distro.ultrawombat.com",
				RepoRoot:          "/repo/root",
				EnvFile:           ".env.profile",
			}},
		},
	}

	f, platform, kubeContext, ingress, repoRoot, envFile := newMatrixRunFlags()
	ApplyMatrixRunConfig(rc, map[string]bool{}, f)

	if *ingress != "ci.distro.ultrawombat.com" {
		t.Errorf("ingress = %q, want profile value", *ingress)
	}
	if *kubeContext != "gke-ctx-from-profile" {
		t.Errorf("kubeContext = %q, want profile value", *kubeContext)
	}
	if *platform != "gke" {
		t.Errorf("platform = %q, want gke", *platform)
	}
	if *repoRoot != "/repo/root" {
		t.Errorf("repoRoot = %q, want /repo/root", *repoRoot)
	}
	if *envFile != ".env.profile" {
		t.Errorf("envFile = %q, want .env.profile", *envFile)
	}
}

func TestApplyMatrixRunConfigPrecedenceOverProfile(t *testing.T) {
	rc := &RootConfig{
		Current: "local",
		Deployments: map[string]DeploymentConfig{
			"local": {InfraConfig: InfraConfig{IngressBaseDomain: "profile.example.com", KubeContext: "profile-ctx"}},
		},
	}
	rc.IngressBaseDomain = "root.example.com" // root beats profile
	rc.Matrix.KubeContext = "matrix-ctx"      // matrix block beats profile

	f, _, kubeContext, ingress, _, _ := newMatrixRunFlags()
	ApplyMatrixRunConfig(rc, map[string]bool{}, f)

	if *ingress != "root.example.com" {
		t.Errorf("ingress = %q, want root value (root beats profile)", *ingress)
	}
	if *kubeContext != "matrix-ctx" {
		t.Errorf("kubeContext = %q, want matrix value (matrix block beats profile)", *kubeContext)
	}
}

func TestApplyMatrixRunConfigCLIFlagBeatsProfile(t *testing.T) {
	rc := &RootConfig{
		Current: "local",
		Deployments: map[string]DeploymentConfig{
			"local": {InfraConfig: InfraConfig{IngressBaseDomain: "profile.example.com"}},
		},
	}

	f, _, _, ingress, _, _ := newMatrixRunFlags()
	*ingress = "cli.example.com" // user passed --ingress-base-domain
	ApplyMatrixRunConfig(rc, map[string]bool{"ingress-base-domain": true}, f)

	if *ingress != "cli.example.com" {
		t.Errorf("ingress = %q, want cli value (explicit flag wins)", *ingress)
	}
}

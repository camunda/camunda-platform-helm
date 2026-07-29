package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
	"scripts/prepare-helm-values/pkg/env"
)

func newMatrixDoctorCommand() *cobra.Command {
	var versions []string
	var scenarioFilter, shortnameFilter, flowFilter, platform, repoRoot string
	var namespacePrefix, kubeContext, kubeContextGKE, kubeContextEKS string
	var envFile, ingressBaseDomain, ingressBaseDomainGKE, ingressBaseDomainEKS string
	var envFile86, envFile87, envFile88, envFile89 string
	var dockerUsername, dockerPassword, dockerHubUsername, dockerHubPassword string
	var includeDisabled, shortnameExact, ensureDockerRegistry, ensureDockerHub bool
	var skipKube, generatePostgres, importDockerAuth bool
	var useLatest, useQA, forceImageOverrides, waitIngressReady bool
	var useVaultBackedSecrets, useVaultBackedSecretsGKE, useVaultBackedSecretsEKS bool
	var keycloakHost, keycloakProtocol, logLevel string
	var skipDependencyUpdate bool
	var helmTimeout int
	var tier, ingressReadyTimeout int
	var extraHelmArgs, extraHelmSets, extraValues []string
	var namespaceOverride, chartRef, chartRefVersion string
	var dockerConfigPath string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose every selected matrix entry before cluster mutation",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateChartRefFlags(chartRef, chartRefVersion)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			changedFlags := map[string]bool{}
			cmd.Flags().Visit(func(flag *pflag.Flag) { changedFlags[flag.Name] = true })
			kubeContexts := map[string]string{}
			envFiles := map[string]string{}
			vaultBackedSecrets := map[string]bool{}
			ingressDomains := map[string]string{}
			if kubeContextGKE != "" {
				kubeContexts["gke"] = kubeContextGKE
			}
			if kubeContextEKS != "" {
				kubeContexts["eks"] = kubeContextEKS
			}
			for version, path := range map[string]string{"8.6": envFile86, "8.7": envFile87, "8.8": envFile88, "8.9": envFile89} {
				if path != "" {
					envFiles[version] = path
				}
			}
			if cmd.Flags().Changed("use-vault-backed-secrets-gke") {
				vaultBackedSecrets["gke"] = useVaultBackedSecretsGKE
			}
			if cmd.Flags().Changed("use-vault-backed-secrets-eks") {
				vaultBackedSecrets["eks"] = useVaultBackedSecretsEKS
			}
			if ingressBaseDomainGKE != "" {
				ingressDomains["gke"] = ingressBaseDomainGKE
			}
			if ingressBaseDomainEKS != "" {
				ingressDomains["eks"] = ingressBaseDomainEKS
			}
			rc, cfgErr := config.LoadMatrixConfig(configFile)
			if cfgErr != nil {
				return cfgErr
			}
			config.ApplyMatrixRunConfig(rc, changedFlags, &config.MatrixRunFlags{
				Versions: &versions, IncludeDisabled: &includeDisabled, ScenarioFilter: &scenarioFilter,
				ShortnameFilter: &shortnameFilter, FlowFilter: &flowFilter, Platform: &platform, RepoRoot: &repoRoot,
				NamespacePrefix: &namespacePrefix, LogLevel: &logLevel, SkipDependencyUpdate: &skipDependencyUpdate,
				HelmTimeout: &helmTimeout, KubeContext: &kubeContext, KubeContextGKE: &kubeContextGKE,
				KubeContextEKS: &kubeContextEKS, KubeContexts: kubeContexts,
				IngressBaseDomain: &ingressBaseDomain, IngressBaseDomainGKE: &ingressBaseDomainGKE,
				IngressBaseDomainEKS: &ingressBaseDomainEKS, IngressBaseDomains: ingressDomains,
				UseVaultBackedSecrets: &useVaultBackedSecrets, UseVaultBackedSecretsGKE: &useVaultBackedSecretsGKE,
				UseVaultBackedSecretsEKS: &useVaultBackedSecretsEKS, VaultBackedSecrets: vaultBackedSecrets,
				EnvFile: &envFile, EnvFile86: &envFile86, EnvFile87: &envFile87, EnvFile88: &envFile88,
				EnvFile89: &envFile89, EnvFiles: envFiles, DockerUsername: &dockerUsername,
				DockerPassword: &dockerPassword, EnsureDockerRegistry: &ensureDockerRegistry,
				DockerHubUsername: &dockerHubUsername, DockerHubPassword: &dockerHubPassword,
				EnsureDockerHub: &ensureDockerHub, KeycloakHost: &keycloakHost, KeycloakProtocol: &keycloakProtocol,
			})
			envFileToLoad := envFile
			if envFileToLoad == "" {
				envFileToLoad = ".env"
			}
			_ = env.Load(envFileToLoad)
			if err := resolveMatrixDockerCredentialPairs(&dockerUsername, &dockerPassword, &dockerHubUsername, &dockerHubPassword, ensureDockerRegistry, ensureDockerHub); err != nil {
				return err
			}
			if repoRoot == "" {
				detected, err := config.DetectRepoRoot()
				if err != nil {
					return err
				}
				repoRoot = detected
			}
			entries, err := matrix.Generate(repoRoot, matrix.GenerateOptions{Versions: versions, IncludeDisabled: includeDisabled})
			if err != nil {
				return err
			}
			entries = matrix.Filter(entries, matrix.FilterOptions{
				ScenarioFilter: scenarioFilter, ShortnameFilter: shortnameFilter, ShortnameExact: shortnameExact,
				FlowFilter: flowFilter, Platform: platform, Tier: tier,
			})
			if err := resolveSelectedEnvFileCredentials(entries, envFiles, envFile, &dockerUsername, &dockerPassword, &dockerHubUsername, &dockerHubPassword, ensureDockerRegistry, ensureDockerHub); err != nil {
				return err
			}
			if err := resolveKeyringCredentialPairs(&dockerUsername, &dockerPassword, &dockerHubUsername, &dockerHubPassword, ensureDockerRegistry, ensureDockerHub); err != nil {
				return err
			}
			if len(entries) == 0 {
				return fmt.Errorf("no matrix entries matched the filters")
			}
			if importDockerAuth {
				if err := importMatrixDockerAuth(dockerConfigPath, &dockerUsername, &dockerPassword, &dockerHubUsername, &dockerHubPassword); err != nil {
					return err
				}
			}
			opts := matrix.RunOptions{
				RepoRoot: repoRoot, Platform: platform, NamespacePrefix: namespacePrefix,
				KubeContext: kubeContext, KubeContexts: kubeContexts, EnvFile: envFile, EnvFiles: envFiles,
				IngressBaseDomain: ingressBaseDomain, IngressBaseDomains: ingressDomains,
				DockerUsername: dockerUsername, DockerPassword: dockerPassword,
				DockerHubUsername: dockerHubUsername, DockerHubPassword: dockerHubPassword,
				EnsureDockerRegistry: ensureDockerRegistry, EnsureDockerHub: ensureDockerHub,
				GeneratePostgresCredentials: generatePostgres, VaultBackedSecrets: vaultBackedSecrets,
				UseVaultBackedSecrets: useVaultBackedSecrets, KeycloakHost: keycloakHost,
				KeycloakProtocol: keycloakProtocol, LogLevel: logLevel,
				SkipDependencyUpdate: skipDependencyUpdate, HelmTimeout: helmTimeout,
				UseLatest: useLatest, UseQA: useQA, ForceImageOverrides: forceImageOverrides,
				ExtraHelmArgs: extraHelmArgs, ExtraHelmSets: extraHelmSets, ExtraValues: extraValues,
				NamespaceOverride: namespaceOverride, ChartRef: chartRef, ChartRefVersion: chartRefVersion,
				WaitIngressReady: waitIngressReady, IngressReadyTimeoutMinutes: ingressReadyTimeout,
			}
			postgresUser, postgresPassword, err := matrix.ResolvePostgresCredentials(entries, opts)
			if err != nil {
				return err
			}
			opts.GeneratedPostgresUsername, opts.GeneratedPostgresPassword = postgresUser, postgresPassword
			cfgRes, _ := config.ResolvePath(configFile)
			doctorOpts := matrix.DoctorOptions{SkipKube: skipKube}
			if cfgRes != nil {
				doctorOpts.ConfigPath, doctorOpts.ConfigFound = cfgRes.Path, cfgRes.Found
			}
			report, err := matrix.Doctor(cmd.Context(), entries, opts, doctorOpts)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Shared checks:")
			renderChecks(cmd, &deploy.Report{Checks: report.Shared})
			for _, entry := range report.Entries {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%s):\n", matrix.EntryID(entry.Entry), entry.Namespace)
				renderChecks(cmd, entry.Report)
			}
			if generatePostgres {
				fmt.Fprintln(cmd.OutOrStdout(), "\nPostgreSQL credentials: generated in memory where required; explicit env values were preserved")
			}
			if !report.OK() {
				return fmt.Errorf("matrix preflight failed")
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&versions, "versions", nil, "Limit to chart versions")
	f.BoolVar(&includeDisabled, "include-disabled", false, "Include disabled scenarios")
	f.StringVar(&scenarioFilter, "scenario-filter", "", "Filter scenarios by substring")
	f.StringVar(&shortnameFilter, "shortname-filter", "", "Filter entries by shortname")
	f.BoolVar(&shortnameExact, "shortname-exact", false, "Require exact shortname matches")
	f.StringVar(&flowFilter, "flow-filter", "", "Filter entries by flow")
	f.StringVar(&platform, "platform", "", "Filter and target platform")
	f.IntVar(&tier, "tier", 0, "Filter entries by tier")
	f.StringVar(&repoRoot, "repo-root", "", "Repository root")
	f.StringVar(&namespacePrefix, "namespace-prefix", "matrix", "Generated namespace prefix")
	f.StringVar(&kubeContext, "kube-context", "", "Default Kubernetes context")
	f.StringVar(&kubeContextGKE, "kube-context-gke", "", "GKE Kubernetes context")
	f.StringVar(&kubeContextEKS, "kube-context-eks", "", "EKS Kubernetes context")
	f.StringVar(&envFile, "env-file", "", "Environment file")
	f.StringVar(&envFile86, "env-file-8.6", "", "Environment file for 8.6")
	f.StringVar(&envFile87, "env-file-8.7", "", "Environment file for 8.7")
	f.StringVar(&envFile88, "env-file-8.8", "", "Environment file for 8.8")
	f.StringVar(&envFile89, "env-file-8.9", "", "Environment file for 8.9")
	f.StringVar(&ingressBaseDomain, "ingress-base-domain", "", "Default ingress base domain")
	f.StringVar(&ingressBaseDomainGKE, "ingress-base-domain-gke", "", "GKE ingress base domain")
	f.StringVar(&ingressBaseDomainEKS, "ingress-base-domain-eks", "", "EKS ingress base domain")
	f.StringVar(&dockerUsername, "docker-username", "", "Harbor username")
	f.StringVar(&dockerPassword, "docker-password", "", "Harbor password")
	f.BoolVar(&ensureDockerRegistry, "ensure-docker-registry", false, "Require Harbor credentials")
	f.StringVar(&dockerHubUsername, "dockerhub-username", "", "Docker Hub username")
	f.StringVar(&dockerHubPassword, "dockerhub-password", "", "Docker Hub password")
	f.BoolVar(&ensureDockerHub, "ensure-docker-hub", false, "Require Docker Hub credentials")
	f.BoolVar(&useVaultBackedSecrets, "use-vault-backed-secrets", false, "Use vault-backed secrets")
	f.BoolVar(&useVaultBackedSecretsGKE, "use-vault-backed-secrets-gke", false, "Use GKE vault-backed secrets")
	f.BoolVar(&useVaultBackedSecretsEKS, "use-vault-backed-secrets-eks", false, "Use EKS vault-backed secrets")
	f.StringVar(&keycloakHost, "keycloak-host", "", "External Keycloak host")
	f.StringVar(&keycloakProtocol, "keycloak-protocol", "", "External Keycloak protocol")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level")
	f.BoolVar(&skipDependencyUpdate, "skip-dependency-update", false, "Skip Helm dependency updates")
	f.IntVar(&helmTimeout, "timeout", 10, "Helm timeout in minutes")
	f.BoolVar(&useLatest, "use-latest", false, "Use latest image overlays")
	f.BoolVar(&useQA, "use-qa", false, "Force QA values")
	f.BoolVar(&forceImageOverrides, "force-image-overrides", false, "Allow local image overrides with chart references")
	f.StringArrayVar(&extraHelmArgs, "extra-helm-arg", nil, "Extra Helm argument")
	f.StringSliceVar(&extraHelmSets, "extra-helm-set", nil, "Extra Helm set values")
	f.StringArrayVar(&extraValues, "extra-values", nil, "Extra values file")
	f.StringVar(&namespaceOverride, "namespace-override", "", "Override generated namespace")
	f.StringVar(&chartRef, "chart-ref", "", "External chart reference")
	f.StringVar(&chartRefVersion, "chart-version", "", "External chart version")
	f.BoolVar(&waitIngressReady, "wait-ingress-ready", false, "Validate public ingress readiness settings")
	f.IntVar(&ingressReadyTimeout, "ingress-ready-timeout", config.DefaultIngressReadyTimeoutMinutes, "Ingress readiness timeout")
	f.BoolVar(&skipKube, "skip-kube-check", false, "Skip cluster reachability checks")
	f.BoolVar(&generatePostgres, "generate-postgres-credentials", true, "Generate entry-local PostgreSQL credentials when required")
	f.BoolVar(&importDockerAuth, "import-docker-auth", false, "Import plaintext auths from Docker config; credential helpers are rejected")
	f.StringVar(&dockerConfigPath, "docker-config", "", "Docker config.json path")
	registerMatrixShortnameCompletion(cmd)
	registerMatrixVersionsCompletion(cmd)
	registerMatrixFlowCompletion(cmd)
	return cmd
}

func importMatrixDockerAuth(configPath string, harborUser, harborPassword, hubUser, hubPassword *string) error {
	auths, err := matrix.ImportPlaintextDockerAuth(configPath, "registry.camunda.cloud", "docker.io")
	if err != nil {
		return err
	}
	if auth, ok := auths["registry.camunda.cloud"]; ok {
		if *harborUser == "" && *harborPassword == "" {
			*harborUser, *harborPassword = auth.Username, auth.Password
		}
	}
	if auth, ok := auths["docker.io"]; ok {
		if *hubUser == "" && *hubPassword == "" {
			*hubUser, *hubPassword = auth.Username, auth.Password
		}
	}
	return nil
}

func resolveMatrixDockerCredentialPairs(harborUser, harborPassword, hubUser, hubPassword *string, requireHarbor, requireHub bool) error {
	if requireHarbor {
		if err := resolveCredentialPair("Harbor flags/config", harborUser, harborPassword, [][2]string{
			{"HARBOR_USERNAME", "HARBOR_PASSWORD"},
			{"TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"},
			{"NEXUS_USERNAME", "NEXUS_PASSWORD"},
		}); err != nil {
			return err
		}
	}
	if requireHub {
		return resolveCredentialPair("Docker Hub flags/config", hubUser, hubPassword, [][2]string{
			{"DOCKERHUB_USERNAME", "DOCKERHUB_PASSWORD"},
			{"TEST_DOCKER_USERNAME", "TEST_DOCKER_PASSWORD"},
		})
	}
	return nil
}

func resolveCredentialPair(source string, username, password *string, envPairs [][2]string) error {
	if *username != "" || *password != "" {
		if *username == "" || *password == "" {
			return fmt.Errorf("%s must provide both username and password", source)
		}
		return nil
	}
	for _, pair := range envPairs {
		user, pass := os.Getenv(pair[0]), os.Getenv(pair[1])
		if user == "" && pass == "" {
			continue
		}
		if user == "" || pass == "" {
			return fmt.Errorf("%s/%s must both be set", pair[0], pair[1])
		}
		*username, *password = user, pass
		return nil
	}
	return nil
}

func resolveSelectedEnvFileCredentials(entries []matrix.Entry, envFiles map[string]string, fallback string, harborUser, harborPassword, hubUser, hubPassword *string, requireHarbor, requireHub bool) error {
	paths := map[string]bool{}
	for _, entry := range entries {
		path := envFiles[entry.Version]
		if path == "" {
			path = fallback
		}
		if path != "" {
			paths[path] = true
		}
	}
	for path := range paths {
		values, err := env.ReadFile(path)
		if err != nil {
			continue
		}
		if requireHarbor && *harborUser == "" && *harborPassword == "" {
			if err := mergeCredentialPairFromMap("Harbor", values, harborUser, harborPassword, [][2]string{{"HARBOR_USERNAME", "HARBOR_PASSWORD"}, {"TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"}, {"NEXUS_USERNAME", "NEXUS_PASSWORD"}}); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
		if requireHub && *hubUser == "" && *hubPassword == "" {
			if err := mergeCredentialPairFromMap("Docker Hub", values, hubUser, hubPassword, [][2]string{{"DOCKERHUB_USERNAME", "DOCKERHUB_PASSWORD"}, {"TEST_DOCKER_USERNAME", "TEST_DOCKER_PASSWORD"}}); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	return nil
}

func mergeCredentialPairFromMap(name string, values map[string]string, username, password *string, pairs [][2]string) error {
	for _, pair := range pairs {
		user, pass := values[pair[0]], values[pair[1]]
		if user == "" && pass == "" {
			continue
		}
		if user == "" || pass == "" {
			return fmt.Errorf("%s credential pair %s/%s is incomplete", name, pair[0], pair[1])
		}
		if *username != "" && (*username != user || *password != pass) {
			return fmt.Errorf("selected env files contain conflicting %s credentials", name)
		}
		*username, *password = user, pass
		return nil
	}
	return nil
}

func renderChecks(cmd *cobra.Command, report *deploy.Report) {
	var buf bytes.Buffer
	report.Render(&buf)
	fmt.Fprint(cmd.OutOrStdout(), buf.String())
}

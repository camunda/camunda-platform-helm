package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/completion"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/camunda-deployer/pkg/types"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"
	"time"
	"vault-secret-mapper/pkg/mapper"

	"github.com/spf13/cobra"
)

var (
	chartPath            string
	chart                string
	namespace            string
	release              string
	scenario             string
	scenarioPath         string
	realmPath            string
	auth                 string
	platform             string
	logLevel             string
	skipDependencyUpdate bool
	externalSecrets      bool
	keycloakHost         string
	keycloakProtocol     string
	repoRoot             string
	flow                 string
	envFile              string
	interactive          bool
	vaultSecretMapping   string
	autoGenerateSecrets  bool
	deleteNamespaceFirst bool
	dockerUsername       string
	dockerPassword       string
	ensureDockerRegistry bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "deploy-camunda",
		Short: "Deploy Camunda Platform with prepared Helm values",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Try to load .env file
			if envFile != "" {
				_ = env.Load(envFile)
			} else {
				// Try default locations if not specified
				_ = env.Load(".env")
			}
			// Skip chart requirement for completion command
			if cmd != nil && cmd.Name() == "completion" {
				return nil
			}
			// Ensure at least one of chart-path or chart is provided
			if chartPath == "" && chart == "" {
				return fmt.Errorf("either --chart-path or --chart must be provided")
			}
			return nil
		},
		RunE: run,
	}

	// completion subcommand (bash|zsh|fish|powershell)
	completionCmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		Args:      cobra.ExactValidArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return nil
			}
		},
	}
	rootCmd.AddCommand(completionCmd)

	flags := rootCmd.Flags()
	flags.StringVar(&chartPath, "chart-path", "", "Path to the Camunda chart directory")
	flags.StringVar(&chart, "chart", "", "Chart name")
	flags.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	flags.StringVar(&release, "release", "", "Helm release name")
	flags.StringVar(&scenario, "scenario", "", "The name of the scneario to deploy")
	flags.StringVar(&scenarioPath, "scenario-path", "", "Path to scenario files")
	flags.StringVar(&realmPath, "realm-path", "", "Path to the keycloak realm file")
	flags.StringVar(&auth, "auth", "keycloak", "Auth scenario")
	flags.StringVar(&platform, "platform", "gke", "Target platform: gke, rosa, eks")
	flags.StringVar(&logLevel, "log-level", "info", "Log level")
	flags.BoolVar(&skipDependencyUpdate, "skip-dependency-update", true, "Skip Helm dependency update")
	flags.BoolVar(&externalSecrets, "external-secrets", true, "Enable external secrets")
	flags.StringVar(&keycloakHost, "keycloak-host", "keycloak-24-9-0.ci.distro.ultrawombat.com", "Keycloak external host")
	flags.StringVar(&keycloakProtocol, "keycloak-protocol", "https", "Keycloak protocol")
	flags.StringVar(&repoRoot, "repo-root", "", "Repository root path")
	flags.StringVar(&flow, "flow", "install", "Flow type")
	flags.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	flags.BoolVar(&interactive, "interactive", true, "Enable interactive prompts for missing variables")
	flags.StringVar(&vaultSecretMapping, "vault-secret-mapping", "", "Vault secret mapping content")
	flags.BoolVar(&autoGenerateSecrets, "auto-generate-secrets", false, "Auto-generate certain secrets for testing purposes")
	flags.BoolVar(&deleteNamespaceFirst, "delete-namespace", false, "Delete the namespace first, then deploy")
	flags.StringVar(&dockerUsername, "docker-username", "", "Docker registry username")
	flags.StringVar(&dockerPassword, "docker-password", "", "Docker registry password")
	flags.BoolVar(&ensureDockerRegistry, "ensure-docker-registry", false, "Ensure Docker registry secret is created")

	_ = rootCmd.MarkFlagRequired("namespace")
	_ = rootCmd.MarkFlagRequired("release")
	_ = rootCmd.MarkFlagRequired("scenario")

	completion.RegisterScenarioCompletion(rootCmd, "scenario", "chart-path")
	completion.RegisterScenarioCompletion(rootCmd, "auth", "chart-path")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func generateRandomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 8)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[num.Int64()]
	}
	return string(result)
}

func run(cmd *cobra.Command, args []string) error {
	// Setup logging
	if err := logging.Setup(logging.Options{
		LevelString:  logLevel,
		ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
	}); err != nil {
		return err
	}

	// Identifiers
	suffix := generateRandomSuffix()
	realmName := fmt.Sprintf("%s-%s", namespace, suffix)
	optimizePrefix := fmt.Sprintf("opt-%s", suffix)
	orchestrationPrefix := fmt.Sprintf("orch-%s", suffix)

	logging.Logger.Info().Str("realm", realmName).Str("optimize", optimizePrefix).Str("orchestration", orchestrationPrefix).Msg("Generated identifiers")

	// Temp directory
	tempDir, err := os.MkdirTemp("", "camunda-values-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	logging.Logger.Info().Str("dir", tempDir).Msg("Created temporary values directory")

	// Set Env Vars for prepare-helm-values
	os.Setenv("KEYCLOAK_REALM", realmName)
	os.Setenv("OPTIMIZE_INDEX_PREFIX", optimizePrefix)
	os.Setenv("ORCHESTRATION_INDEX_PREFIX", orchestrationPrefix)
	os.Setenv("FLOW", flow)

	// Keycloak Env Vars
	if keycloakHost != "" {
		// Hardcoded version from script
		kcVersionSafe := "24_9_0"
		kcHostVar := fmt.Sprintf("KEYCLOAK_EXT_HOST_%s", kcVersionSafe)
		kcProtoVar := fmt.Sprintf("KEYCLOAK_EXT_PROTOCOL_%s", kcVersionSafe)

		os.Setenv(kcHostVar, keycloakHost)
		os.Setenv(kcProtoVar, keycloakProtocol)
	}

	// Prepare Values
	processValues := func(scen string) error {
		opts := values.Options{
			ChartPath:   chartPath,
			Scenario:    scen,
			ScenarioDir: scenarioPath,
			OutputDir:   tempDir,
			Interactive: interactive,
			EnvFile:     envFile,
		}
		if opts.EnvFile == "" {
			opts.EnvFile = ".env"
		}

		file, err := values.ResolveValuesFile(opts)
		if err != nil {
			return err
		}
		_, _, err = values.Process(file, opts)
		return err
	}

	if auth != "" && auth != scenario {
		logging.Logger.Info().Str("scenario", auth).Msg("Preparing auth scenario")
		if err := processValues(auth); err != nil {
			return err
		}
	}

	logging.Logger.Info().Str("scenario", scenario).Msg("Preparing main scenario")
	if err := processValues(scenario); err != nil {
		return err
	}

	if autoGenerateSecrets {
		vaultSecretMapping = "ci/path DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET;"
		// This is overriding the .env values for testing purposes
		firstUserPwd := rand.Text()
		secondUserPwd := rand.Text()
		thirdUserPwd := rand.Text()
		keycloakClientsSecret := rand.Text()

		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", firstUserPwd)
		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", secondUserPwd)
		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", thirdUserPwd)
		os.Setenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", keycloakClientsSecret)

		// Persist the generated secrets to the .env file
		targetEnvFile := envFile
		if targetEnvFile == "" {
			targetEnvFile = ".env"
		}
		type pair struct{ key, val string }
		toPersist := []pair{
			{"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", firstUserPwd},
			{"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", secondUserPwd},
			{"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", thirdUserPwd},
			{"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", keycloakClientsSecret},
		}
		for _, p := range toPersist {
			if err := env.Append(targetEnvFile, p.key, p.val); err != nil {
				logging.Logger.Warn().Err(err).Str("key", p.key).Str("path", targetEnvFile).Msg("Failed to persist generated secret to .env")
			} else {
				logging.Logger.Info().Str("key", p.key).Str("path", targetEnvFile).Msg("Persisted generated secret to .env")
			}
		}
	}
	// Generate Vault Secrets
	var vaultSecretPath string
	if vaultSecretMapping != "" {
		vaultSecretPath = filepath.Join(tempDir, "vault-mapped-secrets.yaml")
		logging.Logger.Info().Msg("Generating vault secrets")

		if err := mapper.Generate(vaultSecretMapping, "vault-mapped-secrets", vaultSecretPath); err != nil {
			return fmt.Errorf("failed to generate vault secrets: %w", err)
		}
	}

	// Deploy
	if deleteNamespaceFirst {
		logging.Logger.Info().Str("namespace", namespace).Msg("Deleting namespace prior to deployment as requested")
		if err := kube.DeleteNamespace(context.Background(), "", "", namespace); err != nil {
			return fmt.Errorf("failed to delete namespace %q: %w", namespace, err)
		}
	}
	vals, err := deployer.BuildValuesList(tempDir, []string{scenario}, auth, false, false, nil)
	if err != nil {
		return err
	}

	deployOpts := types.Options{
		ChartPath: chartPath,
		Chart:     chart,
		RealmPath: realmPath,

		ReleaseName:            release,
		Namespace:              namespace,
		Wait:                   true,
		Atomic:                 true,
		Timeout:                15 * time.Minute,
		ValuesFiles:            vals,
		EnsureDockerRegistry:   ensureDockerRegistry,
		SkipDependencyUpdate:   skipDependencyUpdate,
		ExternalSecretsEnabled: externalSecrets,
		DockerRegistryUsername: dockerUsername,
		DockerRegistryPassword: dockerPassword,
		Platform:               platform,
		RepoRoot:               repoRoot,
		Identifier:             fmt.Sprintf("%s-%s", release, time.Now().Format("20060102150405")),
		TTL:                    "30m",
		LoadKeycloakRealm:      true,
		KeycloakRealmName:      realmName,
		CIMetadata: types.CIMetadata{
			Flow: flow,
		},
		ApplyIntegrationCreds: true,
		VaultSecretPath:       vaultSecretPath,
	}

	err = deployer.Deploy(context.Background(), deployOpts)
	if err != nil {
		return err
	}

	// Print out the details of the deployment including the auto generated secrets
	logging.Logger.Info().Str("realm", realmName).Str("optimize", optimizePrefix).Str("orchestration", orchestrationPrefix).Msg("Deployment completed successfully")
	logging.Logger.Info().Msg("🎉 Deployment completed successfully! Here are your credentials for the test users and Keycloak client:")
	logging.Logger.Info().Msgf("  - First user password:    %s", os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"))
	logging.Logger.Info().Msgf("  - Second user password:   %s", os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"))
	logging.Logger.Info().Msgf("  - Third user password:    %s", os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"))
	logging.Logger.Info().Msgf("  - Keycloak clients secret: %s", os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"))
	logging.Logger.Info().Msg("Please keep these credentials safe. If you have any questions, refer to the documentation or reach out for support. 🚀")
	return nil
}

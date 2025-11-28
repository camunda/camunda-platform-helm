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
	"strings"
	"time"
	"vault-secret-mapper/pkg/mapper"

	"github.com/jwalton/gchalk"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	chartPath            string
	chart                string
	chartVersion         string
	namespace            string
	release              string
	scenario             string
	scenarioPath         string
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
	renderTemplates      bool
	renderOutputDir      string
	extraValues          []string
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
			// Skip validations for shell completion invocations
			if cmd != nil {
				if cmd.Name() == "completion" ||
					cmd.Name() == cobra.ShellCompRequestCmd ||
					cmd.Name() == cobra.ShellCompNoDescRequestCmd {
					return nil
				}
			}
			// Ensure at least one of chart-path or chart is provided
			if chartPath == "" && chart == "" {
				return fmt.Errorf("either --chart-path or --chart must be provided")
			}
			// Validate --version compatibility
			if strings.TrimSpace(chartVersion) != "" && strings.TrimSpace(chartPath) != "" {
				return fmt.Errorf("--version can only be used with --chart, not with --chart-path")
			}
			if strings.TrimSpace(chartVersion) != "" && strings.TrimSpace(chart) == "" {
				return fmt.Errorf("--version requires --chart to be set")
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
	flags.StringVar(&chartVersion, "version", "", "Chart version (only valid with --chart; not allowed with --chart-path)")
	flags.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	flags.StringVar(&release, "release", "", "Helm release name")
	flags.StringVar(&scenario, "scenario", "", "The name of the scneario to deploy")
	flags.StringVar(&scenarioPath, "scenario-path", "", "Path to scenario files")
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
	flags.BoolVar(&renderTemplates, "render-templates", false, "Render manifests to a directory instead of installing")
	flags.StringVar(&renderOutputDir, "render-output-dir", "", "Output directory for rendered manifests (defaults to ./rendered/<release>)")
	flags.StringSliceVar(&extraValues, "extra-values", nil, "Additional Helm values files to apply last (comma-separated or repeatable)")

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

// maskIfSet returns a masked placeholder when a sensitive value is set.
// This prevents leaking secrets while still indicating that a value exists.
func maskIfSet(val string) string {
	if val == "" {
		return ""
	}
	return "***"
}

func run(cmd *cobra.Command, args []string) error {
	// Setup logging
	if err := logging.Setup(logging.Options{
		LevelString:  logLevel,
		ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
	}); err != nil {
		return err
	}

	// Log flags as a colored, multi-line list (sensitive values are masked).
	// Iterate over the Cobra flag set to avoid duplication and keep logs in sync with flags.
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	stylePwd := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleBool := func(s string) string {
		if strings.EqualFold(s, "true") || s == "1" {
			return logging.Emphasize("true", gchalk.Green)
		}
		return logging.Emphasize("false", gchalk.Red)
	}
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }

	var b strings.Builder
	b.WriteString(styleHead("Starting deployment with flags:"))
	b.WriteString("\n")

	printFlag := func(f *pflag.Flag) {
		name := f.Name // actual CLI flag name
		val := f.Value.String()
		typ := f.Value.Type()

		// Sensitive handling
		switch name {
		case "docker-password":
			val = stylePwd(maskIfSet(val))
		case "vault-secret-mapping":
			if strings.TrimSpace(val) != "" {
				val = styleVal("provided")
			} else {
				val = styleVal("not-provided")
			}
		default:
			if typ == "bool" {
				val = styleBool(val)
			} else {
				val = styleVal(val)
			}
		}
		fmt.Fprintf(&b, "  - %s: %s\n", styleKey(name), val)
	}

	// Visit all flags on the current command (sorted and without duplication)
	if cmd != nil && cmd.Flags() != nil {
		cmd.Flags().VisitAll(printFlag)
	}
	logging.Logger.Info().Msg(b.String())

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
	vals, err := deployer.BuildValuesList(tempDir, []string{scenario}, auth, false, false, extraValues)
	if err != nil {
		return err
	}

	deployOpts := types.Options{
		ChartPath: chartPath,
		Chart:     chart,
		Version:   chartVersion,
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
		RenderTemplates:        renderTemplates,
		RenderOutputDir:        renderOutputDir,
		IncludeCRDs:            true,
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

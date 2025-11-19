package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/spf13/cobra"

	"scripts/camunda-core/pkg/completion"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/camunda-deployer/pkg/types"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"
)

var (
	chartPath            string
	namespace            string
	release              string
	scenario             string
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
			return nil
		},
		RunE: run,
	}

	flags := rootCmd.Flags()
	flags.StringVar(&chartPath, "chart-path", "", "Path to the Camunda chart directory")
	flags.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	flags.StringVar(&release, "release", "", "Helm release name")
	flags.StringVar(&scenario, "scenario", "", "Scenario name")
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

	_ = rootCmd.MarkFlagRequired("chart-path")
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

	// Deploy
	vals, err := deployer.BuildValuesList(tempDir, []string{scenario}, auth, false, false, nil)
	if err != nil {
		return err
	}

	deployOpts := types.Options{
		ChartPath:              chartPath,
		ReleaseName:            release,
		Namespace:              namespace,
		Wait:                   true,
		Atomic:                 true,
		Timeout:                15 * time.Minute,
		ValuesFiles:            vals,
		EnsureDockerRegistry:   true,
		SkipDependencyUpdate:   skipDependencyUpdate,
		ExternalSecretsEnabled: externalSecrets,
		Platform:               platform,
		RepoRoot:               repoRoot,
		Identifier:             fmt.Sprintf("%s-%s", release, time.Now().Format("20060102150405")),
		TTL:                    "1h",
		LoadKeycloakRealm:      true,
		KeycloakRealmName:      realmName,
		CIMetadata: types.CIMetadata{
			Flow: flow,
		},
		ApplyIntegrationCreds: true,
	}

	return deployer.Deploy(context.Background(), deployOpts)
}


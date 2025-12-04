package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"strings"

	"github.com/jwalton/gchalk"
	"github.com/spf13/cobra"
)

// newInitCommand creates the init subcommand.
func newInitCommand() *cobra.Command {
	var initFlags struct {
		outputPath string
		force      bool
		minimal    bool
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new deploy-camunda configuration file",
		Long: `Create a new .camunda-deploy.yaml configuration file interactively.

This command guides you through setting up a deployment configuration by
prompting for common settings. You can also use --minimal to create a
bare-bones config file that you can edit manually.

The configuration file will be created at:
  - .camunda-deploy.yaml in the current directory (default)
  - A custom path if --output is specified
  - ~/.config/camunda/deploy.yaml if --output=global`,
		Example: `  # Interactive initialization in current directory
  deploy-camunda init

  # Create minimal config without prompts
  deploy-camunda init --minimal

  # Create config in global location
  deploy-camunda init --output=global

  # Force overwrite existing config
  deploy-camunda init --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine output path
			outputPath := initFlags.outputPath
			if outputPath == "" {
				outputPath = ".camunda-deploy.yaml"
			} else if outputPath == "global" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				outputPath = filepath.Join(home, ".config", "camunda", "deploy.yaml")
			}

			// Check if file exists
			if _, err := os.Stat(outputPath); err == nil && !initFlags.force {
				return fmt.Errorf("configuration file already exists at %s\nUse --force to overwrite", outputPath)
			}

			var rc *config.RootConfig
			var err error

			if initFlags.minimal {
				rc = createMinimalConfig()
			} else {
				rc, err = runInteractiveInit()
				if err != nil {
					return err
				}
			}

			rc.FilePath = outputPath

			// Ensure parent directory exists
			if dir := filepath.Dir(outputPath); dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			// Write config
			if err := config.Write(rc); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			printInitSuccess(outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&initFlags.outputPath, "output", "o", "",
		"Output path for config file. Use 'global' for ~/.config/camunda/deploy.yaml")
	cmd.Flags().BoolVar(&initFlags.force, "force", false,
		"Overwrite existing configuration file")
	cmd.Flags().BoolVar(&initFlags.minimal, "minimal", false,
		"Create minimal config without interactive prompts")

	return cmd
}

// createMinimalConfig creates a bare-bones configuration.
func createMinimalConfig() *config.RootConfig {
	return &config.RootConfig{
		Current:  "default",
		Platform: "gke",
		LogLevel: "info",
		Deployments: map[string]config.DeploymentConfig{
			"default": {
				Chart:     "camunda-platform-8.8",
				Namespace: "camunda",
				Release:   "camunda",
				Scenario:  "keycloak",
			},
		},
	}
}

// runInteractiveInit prompts the user for configuration values.
func runInteractiveInit() (*config.RootConfig, error) {
	reader := bufio.NewReader(os.Stdin)
	stylePrompt := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleDefault := func(s string) string { return logging.Emphasize(s, gchalk.Dim) }

	prompt := func(question, defaultVal string) string {
		if defaultVal != "" {
			fmt.Printf("%s %s: ", stylePrompt(question), styleDefault(fmt.Sprintf("[%s]", defaultVal)))
		} else {
			fmt.Printf("%s: ", stylePrompt(question))
		}

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			return defaultVal
		}
		return input
	}

	promptBool := func(question string, defaultVal bool) bool {
		defaultStr := "y/N"
		if defaultVal {
			defaultStr = "Y/n"
		}
		fmt.Printf("%s %s: ", stylePrompt(question), styleDefault(fmt.Sprintf("[%s]", defaultStr)))

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "" {
			return defaultVal
		}
		return input == "y" || input == "yes" || input == "true" || input == "1"
	}

	fmt.Println()
	fmt.Println(logging.Emphasize("Deploy-Camunda Configuration Setup", gchalk.Bold))
	fmt.Println(logging.Emphasize("===================================", gchalk.Dim))
	fmt.Println()

	// Deployment profile name
	deploymentName := prompt("Deployment profile name", "default")

	// Core settings
	fmt.Println()
	fmt.Println(logging.Emphasize("Chart Configuration", gchalk.Bold))

	// Check if we're in a repo with charts
	chartPath := ""
	if _, err := os.Stat("charts/camunda-platform-8.8"); err == nil {
		chartPath = prompt("Chart path (local)", "charts/camunda-platform-8.8")
	} else {
		chartPath = prompt("Chart path (local, or empty for remote)", "")
	}

	chart := ""
	version := ""
	if chartPath == "" {
		chart = prompt("Chart name (for remote charts)", "camunda-platform")
		version = prompt("Chart version", "8.8.0")
	}

	// Deployment identifiers
	fmt.Println()
	fmt.Println(logging.Emphasize("Deployment Settings", gchalk.Bold))

	namespace := prompt("Kubernetes namespace", "camunda")
	release := prompt("Helm release name", "camunda")
	scenario := prompt("Scenario name", "keycloak")

	// Platform
	fmt.Println()
	fmt.Println(logging.Emphasize("Platform Settings", gchalk.Bold))

	platform := prompt("Target platform (gke/eks/rosa)", "gke")

	// Repository root (try to detect)
	repoRoot := ""
	if cwd, err := os.Getwd(); err == nil {
		// Check if we're in a camunda-platform-helm repo
		if _, err := os.Stat(filepath.Join(cwd, "charts")); err == nil {
			repoRoot = cwd
		}
	}
	repoRoot = prompt("Repository root path", repoRoot)

	// Optional features
	fmt.Println()
	fmt.Println(logging.Emphasize("Optional Features", gchalk.Bold))

	externalSecrets := promptBool("Enable external secrets", true)
	interactive := promptBool("Enable interactive prompts", true)

	// Build config
	boolPtr := func(b bool) *bool { return &b }

	rc := &config.RootConfig{
		Current:         deploymentName,
		RepoRoot:        repoRoot,
		Platform:        platform,
		LogLevel:        "info",
		ExternalSecrets: externalSecrets,
		Deployments: map[string]config.DeploymentConfig{
			deploymentName: {
				Chart:       chart,
				ChartPath:   chartPath,
				Version:     version,
				Namespace:   namespace,
				Release:     release,
				Scenario:    scenario,
				Interactive: boolPtr(interactive),
			},
		},
	}

	// Ask if user wants to add another deployment
	fmt.Println()
	if promptBool("Add another deployment profile", false) {
		additionalName := prompt("Additional profile name", "staging")
		additionalNamespace := prompt("Namespace for this profile", additionalName)

		rc.Deployments[additionalName] = config.DeploymentConfig{
			Chart:       chart,
			ChartPath:   chartPath,
			Version:     version,
			Namespace:   additionalNamespace,
			Release:     release,
			Scenario:    scenario,
			Interactive: boolPtr(interactive),
		}
	}

	return rc, nil
}

// printInitSuccess displays a success message with next steps.
func printInitSuccess(path string) {
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleCmd := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	stylePath := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }

	fmt.Println()
	fmt.Println(styleOk("✓ Configuration file created successfully!"))
	fmt.Println()
	fmt.Printf("  Location: %s\n", stylePath(path))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Review and edit the configuration: %s\n", styleCmd(fmt.Sprintf("vim %s", path)))
	fmt.Printf("  2. List configured deployments:       %s\n", styleCmd("deploy-camunda config list"))
	fmt.Printf("  3. Validate the configuration:        %s\n", styleCmd("deploy-camunda validate"))
	fmt.Printf("  4. Preview the deployment:            %s\n", styleCmd("deploy-camunda --dry-run"))
	fmt.Printf("  5. Deploy:                            %s\n", styleCmd("deploy-camunda"))
	fmt.Println()
}


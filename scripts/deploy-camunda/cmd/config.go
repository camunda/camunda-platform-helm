package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/format"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newConfigCommand creates the config subcommand.
func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage deploy-camunda configuration and active deployment",
	}

	configCmd.AddCommand(newListCommand())
	configCmd.AddCommand(newShowCommand())
	configCmd.AddCommand(newUseCommand())
	configCmd.AddCommand(newSetCommand())
	configCmd.AddCommand(newGetCommand())
	configCmd.AddCommand(newCreateCommand())

	return configCmd
}

// newListCommand creates the list subcommand.
func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured deployments",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}
			rc, err := config.Read(cfgPath, false)
			if err != nil {
				return err
			}
			active := rc.Current
			for name := range rc.Deployments {
				marker := " "
				if name == active {
					marker = "*"
				}
				fmt.Fprintf(os.Stdout, "%s %s\n", marker, name)
			}
			if len(rc.Deployments) == 0 {
				fmt.Fprintln(os.Stdout, "(no deployments configured)")
			}
			return nil
		},
	}
}

// newShowCommand creates the show subcommand.
func newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "show [name|current]",
		Short:             "Show a deployment (merged with defaults)",
		Args:              cobra.RangeArgs(0, 1),
		ValidArgsFunction: completeDeploymentNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}
			rc, err := config.Read(cfgPath, false)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 0 || args[0] == "current" {
				name = rc.Current
			} else {
				name = args[0]
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("no deployment selected; set a current one with: deploy-camunda config use <name>")
			}
			dep, ok := rc.Deployments[name]
			if !ok {
				return fmt.Errorf("deployment %q not found", name)
			}

			return format.PrintDeploymentConfig(name, dep, *rc)
		},
	}
}

// newUseCommand creates the use subcommand.
func newUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "use <name>",
		Short:             "Set the active deployment",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeDeploymentNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}
			rc, err := config.Read(cfgPath, false)
			if err != nil {
				return err
			}
			if rc.Deployments == nil {
				return fmt.Errorf("no deployments configured in %q", cfgPath)
			}
			if _, ok := rc.Deployments[name]; !ok {
				return fmt.Errorf("deployment %q not found in %q", name, cfgPath)
			}
			if err := config.WriteCurrentOnly(cfgPath, name); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Now using deployment %q in %s\n", name, cfgPath)
			return nil
		},
	}
}

// completeDeploymentNames provides shell completion for deployment names.
func completeDeploymentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, err := config.ResolvePath(configFile)
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	rc, err := config.Read(cfgPath, false)
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	var names []string
	for name := range rc.Deployments {
		if toComplete == "" || strings.HasPrefix(name, toComplete) {
			names = append(names, name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// validConfigKeys lists all valid configuration keys for root and deployment configs.
var validConfigKeys = []string{
	"chart", "version", "chartPath", "namespace", "release", "scenario",
	"scenarioPath", "auth", "platform", "logLevel", "externalSecrets",
	"skipDependencyUpdate", "keycloakRealm", "optimizeIndexPrefix",
	"orchestrationIndexPrefix", "tasklistIndexPrefix", "operateIndexPrefix",
	"ingressHost", "flow", "envFile", "interactive", "vaultSecretMapping",
	"autoGenerateSecrets", "deleteNamespace", "dockerUsername", "dockerPassword",
	"ensureDockerRegistry", "renderTemplates", "renderOutputDir", "extraValues",
	"repoRoot", "scenarioRoot", "valuesPreset", "kubeContext", "ingressBaseDomain",
}

// newSetCommand creates the set subcommand.
func newSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value in the config file.

For root-level settings:
  deploy-camunda config set <key> <value>

For deployment-specific settings:
  deploy-camunda config set <deployment>.<key> <value>

Examples:
  deploy-camunda config set repoRoot /path/to/repo
  deploy-camunda config set platform eks
  deploy-camunda config set dev-cluster.kubeContext my-dev-context
  deploy-camunda config set dev-cluster.namespace my-namespace
  deploy-camunda config set dev-cluster.externalSecrets true`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeConfigKeys,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}

			if err := config.SetValue(cfgPath, key, value); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Set %s = %s in %s\n", key, value, cfgPath)
			return nil
		},
	}
}

// newGetCommand creates the get subcommand.
func newGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a configuration value from the config file.

For root-level settings:
  deploy-camunda config get <key>

For deployment-specific settings:
  deploy-camunda config get <deployment>.<key>

Examples:
  deploy-camunda config get repoRoot
  deploy-camunda config get platform
  deploy-camunda config get dev-cluster.kubeContext
  deploy-camunda config get dev-cluster.namespace`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeConfigKeys,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}

			value, err := config.GetValue(cfgPath, key)
			if err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, value)
			return nil
		},
	}
}

// newCreateCommand creates the create subcommand.
func newCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new deployment configuration",
		Long: `Create a new deployment configuration with the given name.

Examples:
  deploy-camunda config create dev-cluster
  deploy-camunda config create prod-eks`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}

			if err := config.CreateDeployment(cfgPath, name); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Created deployment %q in %s\n", name, cfgPath)
			fmt.Fprintf(os.Stdout, "Use 'deploy-camunda config set %s.<key> <value>' to configure it\n", name)
			return nil
		},
	}
}

// completeConfigKeys provides shell completion for config keys.
func completeConfigKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// If we have one argument (the key), provide value completions for specific keys
	if len(args) == 1 {
		key := args[0]
		return completeConfigValue(key, toComplete)
	}

	// If we have two or more arguments, no more completions
	if len(args) >= 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfgPath, err := config.ResolvePath(configFile)
	if err != nil {
		return validConfigKeys, cobra.ShellCompDirectiveNoFileComp
	}
	rc, err := config.Read(cfgPath, false)
	if err != nil {
		return validConfigKeys, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string

	// Add root-level keys
	for _, key := range validConfigKeys {
		if toComplete == "" || strings.HasPrefix(key, toComplete) {
			completions = append(completions, key)
		}
	}

	// Add deployment-prefixed keys
	for depName := range rc.Deployments {
		prefix := depName + "."
		if strings.HasPrefix(toComplete, prefix) {
			// User is typing a deployment-specific key
			keyPart := strings.TrimPrefix(toComplete, prefix)
			for _, key := range validConfigKeys {
				if keyPart == "" || strings.HasPrefix(key, keyPart) {
					completions = append(completions, prefix+key)
				}
			}
		} else if toComplete == "" || strings.HasPrefix(prefix, toComplete) {
			// Suggest the deployment prefix
			completions = append(completions, prefix)
		}
	}

	sort.Strings(completions)
	return completions, cobra.ShellCompDirectiveNoSpace
}

// completeConfigValue provides value completions for specific config keys.
func completeConfigValue(key, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Extract the actual key name (after the last dot for deployment-specific keys)
	keyName := key
	if idx := strings.LastIndex(key, "."); idx >= 0 {
		keyName = key[idx+1:]
	}

	switch keyName {
	case "kubeContext":
		return completeKubeContexts(toComplete)
	case "platform":
		return completePlatforms(toComplete)
	case "ingressBaseDomain":
		return completeIngressBaseDomains(toComplete)
	case "externalSecrets", "skipDependencyUpdate", "interactive",
		"autoGenerateSecrets", "deleteNamespace", "ensureDockerRegistry", "renderTemplates":
		return completeBooleans(toComplete)
	case "logLevel":
		return completeLogLevels(toComplete)
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeKubeContexts returns available kubectl contexts.
func completeKubeContexts(toComplete string) ([]string, cobra.ShellCompDirective) {
	// Run kubectl config get-contexts to get available contexts
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var contexts []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ctx := strings.TrimSpace(line)
		if ctx != "" && (toComplete == "" || strings.HasPrefix(ctx, toComplete)) {
			contexts = append(contexts, ctx)
		}
	}

	return contexts, cobra.ShellCompDirectiveNoFileComp
}

// completePlatforms returns available platform values.
func completePlatforms(toComplete string) ([]string, cobra.ShellCompDirective) {
	platforms := []string{"gke", "eks", "rosa"}
	var completions []string
	for _, p := range platforms {
		if toComplete == "" || strings.HasPrefix(p, toComplete) {
			completions = append(completions, p)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeBooleans returns boolean value completions.
func completeBooleans(toComplete string) ([]string, cobra.ShellCompDirective) {
	bools := []string{"true", "false"}
	var completions []string
	for _, b := range bools {
		if toComplete == "" || strings.HasPrefix(b, toComplete) {
			completions = append(completions, b)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeLogLevels returns log level completions.
func completeLogLevels(toComplete string) ([]string, cobra.ShellCompDirective) {
	levels := []string{"debug", "info", "warn", "error"}
	var completions []string
	for _, l := range levels {
		if toComplete == "" || strings.HasPrefix(l, toComplete) {
			completions = append(completions, l)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeIngressBaseDomains returns valid ingress base domain completions.
func completeIngressBaseDomains(toComplete string) ([]string, cobra.ShellCompDirective) {
	var completions []string
	for _, d := range config.ValidIngressBaseDomains {
		if toComplete == "" || strings.HasPrefix(d, toComplete) {
			completions = append(completions, d)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

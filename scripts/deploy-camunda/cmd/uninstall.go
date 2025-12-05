package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/prepare-helm-values/pkg/env"
	"strings"
	"sync"

	"github.com/jwalton/gchalk"
	"github.com/spf13/cobra"
)

// UninstallTarget represents a target to uninstall with validation status.
type UninstallTarget struct {
	Namespace       string `json:"namespace"`
	Release         string `json:"release"`
	NamespaceExists bool   `json:"namespaceExists"`
	ReleaseExists   bool   `json:"releaseExists"`
	Scenario        string `json:"scenario,omitempty"`
}

// UninstallResult represents the result of an uninstall operation.
type UninstallResult struct {
	Release           string `json:"release"`
	Namespace         string `json:"namespace"`
	Status            string `json:"status"` // success, failed, skipped
	NamespaceDeleted  bool   `json:"namespaceDeleted,omitempty"`
	Error             string `json:"error,omitempty"`
}

// MultiUninstallResult represents the result of multiple uninstall operations.
type MultiUninstallResult struct {
	Status       string            `json:"status"` // success, partial, failed
	TotalCount   int               `json:"totalCount"`
	SuccessCount int               `json:"successCount"`
	FailedCount  int               `json:"failedCount"`
	Results      []UninstallResult `json:"results"`
}

// newUninstallCommand creates the uninstall subcommand.
func newUninstallCommand() *cobra.Command {
	var uninstallFlags struct {
		deleteNamespace bool
		allScenarios    bool
		scenarios       string
		force           bool
		outputFormat    string
	}

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall a Camunda deployment",
		Long: `Uninstall a Camunda Platform deployment from Kubernetes.

This command removes the Helm release and optionally the namespace.
You can specify which scenarios to uninstall using --scenario, or use
--all-scenarios to clean up all scenarios from a parallel deployment.

Before uninstalling, the tool validates that the targets exist and shows
what will be deleted. For destructive operations (--delete-namespace),
a double confirmation is required.

EXAMPLES:
  # Uninstall using active config profile
  deploy-camunda uninstall

  # Uninstall specific scenarios
  deploy-camunda uninstall --scenario keycloak,keycloak-mt

  # Uninstall and delete namespace
  deploy-camunda uninstall --delete-namespace

  # Uninstall all scenarios from a parallel deployment
  deploy-camunda uninstall --all-scenarios

  # Force uninstall without confirmation
  deploy-camunda uninstall --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging
			if err := logging.Setup(logging.Options{
				LevelString:  flags.LogLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			// Load .env file
			if flags.EnvFile != "" {
				_ = env.Load(flags.EnvFile)
			} else {
				_ = env.Load(".env")
			}

			// Load config
			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			rc, err := config.Read(cfgPath, true)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			// Apply active deployment defaults
			if err := config.ApplyActiveDeployment(rc, rc.Current, &flags); err != nil {
				return err
			}

			// Require namespace
			if flags.Namespace == "" {
				return fmt.Errorf("namespace not set; provide --namespace or set in config")
			}
			if flags.Release == "" {
				flags.Release = deploy.DefaultReleaseName
			}

			// Get output format
			outputJSON := uninstallFlags.outputFormat == "json"

			// Resolve targets based on scenarios
			var targets []UninstallTarget
			var scenariosToUninstall []string

			// Determine which scenarios to uninstall
			if uninstallFlags.scenarios != "" {
				// User explicitly specified scenarios to uninstall
				for _, s := range strings.Split(uninstallFlags.scenarios, ",") {
					s = strings.TrimSpace(s)
					if s != "" {
						scenariosToUninstall = append(scenariosToUninstall, s)
					}
				}
			} else if uninstallFlags.allScenarios && flags.Scenario != "" {
				// Use all scenarios from config
				for _, s := range strings.Split(flags.Scenario, ",") {
					s = strings.TrimSpace(s)
					if s != "" {
						scenariosToUninstall = append(scenariosToUninstall, s)
					}
				}
			}

			// Build targets from scenarios
			if len(scenariosToUninstall) > 0 {
				for _, s := range scenariosToUninstall {
					targets = append(targets, UninstallTarget{
						Namespace: fmt.Sprintf("%s-%s", flags.Namespace, s),
						Release:   flags.Release,
						Scenario:  s,
					})
				}
			}

			// If no scenario targets, use single target (base namespace)
			if len(targets) == 0 {
				targets = []UninstallTarget{{
					Namespace: flags.Namespace,
					Release:   flags.Release,
				}}
			}

			// Validate targets exist
			ctx := cmd.Context()
			targets = validateUninstallTargets(ctx, targets)

			// Check if anything exists to uninstall
			hasExistingTargets := false
			for _, t := range targets {
				if t.NamespaceExists || t.ReleaseExists {
					hasExistingTargets = true
					break
				}
			}
			if !hasExistingTargets {
				if outputJSON {
					return printValidationResult(targets, "nothing_to_uninstall", outputJSON)
				}
				logging.Logger.Warn().Msg("No existing releases or namespaces found to uninstall")
				printTargetsStatus(targets)
				return nil
			}

			// Show what will be deleted
			if !outputJSON {
				printTargetsStatus(targets)
			}

			// Confirmation prompts (unless --force)
			if !uninstallFlags.force && !outputJSON {
				// First confirmation
				if !confirmUninstallFirst(targets, uninstallFlags.deleteNamespace) {
					logging.Logger.Info().Msg("Uninstall cancelled")
					return nil
				}

				// Second confirmation for destructive operations
				if uninstallFlags.deleteNamespace {
					if !confirmUninstallSecond(targets) {
						logging.Logger.Info().Msg("Uninstall cancelled at second confirmation")
						return nil
					}
				}
			}

			// Execute uninstalls
			if len(targets) > 1 {
				return uninstallMultipleTargets(ctx, targets, uninstallFlags.deleteNamespace, outputJSON)
			}

			// Single uninstall
			result := uninstallRelease(ctx, targets[0].Namespace, targets[0].Release, uninstallFlags.deleteNamespace)
			return printUninstallResult(result, outputJSON)
		},
	}

	cmd.Flags().StringVarP(&uninstallFlags.scenarios, "scenario", "s", "",
		"Scenario(s) to uninstall, comma-separated (e.g., keycloak,keycloak-mt)")
	cmd.Flags().BoolVar(&uninstallFlags.deleteNamespace, "delete-namespace", false,
		"Delete the namespace after uninstalling the release (requires double confirmation)")
	cmd.Flags().BoolVar(&uninstallFlags.allScenarios, "all-scenarios", false,
		"Uninstall all scenarios from config (alternative to --scenario)")
	cmd.Flags().BoolVar(&uninstallFlags.force, "force", false,
		"Skip confirmation prompts (use with caution)")
	cmd.Flags().StringVarP(&uninstallFlags.outputFormat, "output", "o", "text",
		"Output format: text (default) or json")

	return cmd
}

// validateUninstallTargets checks which targets actually exist.
func validateUninstallTargets(ctx context.Context, targets []UninstallTarget) []UninstallTarget {
	for i := range targets {
		// Check if namespace exists
		nsCmd := exec.CommandContext(ctx, "kubectl", "get", "namespace", targets[i].Namespace, "-o", "name")
		if err := nsCmd.Run(); err == nil {
			targets[i].NamespaceExists = true
		}

		// Check if release exists in namespace
		if targets[i].NamespaceExists {
			helmCmd := exec.CommandContext(ctx, "helm", "list", "-n", targets[i].Namespace, "-q", "-f", fmt.Sprintf("^%s$", targets[i].Release))
			output, err := helmCmd.Output()
			if err == nil && strings.TrimSpace(string(output)) != "" {
				targets[i].ReleaseExists = true
			}
		}
	}
	return targets
}

// printTargetsStatus displays the validation status of targets.
func printTargetsStatus(targets []UninstallTarget) {
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }

	var b strings.Builder
	b.WriteString(styleHead("Uninstall Targets"))
	b.WriteString("\n\n")

	for _, t := range targets {
		if t.Scenario != "" {
			fmt.Fprintf(&b, "  %s %s\n", styleKey("Scenario:"), styleVal(t.Scenario))
		}
		fmt.Fprintf(&b, "  %s %s", styleKey("Namespace:"), styleVal(t.Namespace))
		if t.NamespaceExists {
			fmt.Fprintf(&b, " %s", styleOk("(exists)"))
		} else {
			fmt.Fprintf(&b, " %s", styleErr("(not found)"))
		}
		b.WriteString("\n")

		fmt.Fprintf(&b, "  %s %s", styleKey("Release:"), styleVal(t.Release))
		if t.ReleaseExists {
			fmt.Fprintf(&b, " %s", styleOk("(exists)"))
		} else if t.NamespaceExists {
			fmt.Fprintf(&b, " %s", styleWarn("(not found)"))
		} else {
			fmt.Fprintf(&b, " %s", styleErr("(namespace missing)"))
		}
		b.WriteString("\n\n")
	}

	fmt.Print(b.String())
}

// printValidationResult outputs validation results in JSON format.
func printValidationResult(targets []UninstallTarget, status string, outputJSON bool) error {
	if outputJSON {
		result := map[string]interface{}{
			"status":  status,
			"targets": targets,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}
	return nil
}

// confirmUninstallFirst prompts for the first user confirmation.
func confirmUninstallFirst(targets []UninstallTarget, deleteNs bool) bool {
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }

	fmt.Println()
	if deleteNs {
		fmt.Println(styleErr("⚠️  WARNING: This will DELETE the namespace(s) and ALL resources within!"))
	} else {
		fmt.Println(styleWarn("This will uninstall the Helm release(s) listed above."))
	}

	fmt.Print("\nType 'yes' to continue: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "yes"
}

// confirmUninstallSecond prompts for the second confirmation for destructive operations.
func confirmUninstallSecond(targets []UninstallTarget) bool {
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }

	fmt.Println()
	fmt.Println(styleErr("⚠️  SECOND CONFIRMATION REQUIRED"))
	fmt.Println(styleErr("This action is IRREVERSIBLE. All data in the namespace(s) will be lost."))
	fmt.Println()

	// Get list of namespaces to delete
	var namespaces []string
	for _, t := range targets {
		if t.NamespaceExists {
			namespaces = append(namespaces, t.Namespace)
		}
	}

	if len(namespaces) == 1 {
		fmt.Printf("To confirm, type the namespace name %s: ", styleVal(namespaces[0]))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		return input == namespaces[0]
	}

	// Multiple namespaces - ask for "DELETE ALL"
	fmt.Printf("To confirm deletion of %d namespaces, type %s: ", len(namespaces), styleVal("DELETE ALL"))
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	return input == "DELETE ALL"
}

// uninstallRelease uninstalls a single Helm release.
func uninstallRelease(ctx context.Context, namespace, release string, deleteNs bool) *UninstallResult {
	result := &UninstallResult{
		Release:   release,
		Namespace: namespace,
	}

	// Run helm uninstall
	helmCmd := exec.CommandContext(ctx, "helm", "uninstall", release, "--namespace", namespace)
	output, err := helmCmd.CombinedOutput()
	if err != nil {
		// Check if release doesn't exist
		if strings.Contains(string(output), "not found") {
			result.Status = "skipped"
			result.Error = "release not found"
		} else {
			result.Status = "failed"
			result.Error = fmt.Sprintf("helm uninstall failed: %s", strings.TrimSpace(string(output)))
		}
		
		// Still try to delete namespace if requested and release not found
		if deleteNs && result.Status == "skipped" {
			if nsErr := deleteNamespace(ctx, namespace); nsErr == nil {
				result.NamespaceDeleted = true
			}
		}
		return result
	}

	result.Status = "success"

	// Delete namespace if requested
	if deleteNs {
		if nsErr := deleteNamespace(ctx, namespace); nsErr != nil {
			// Don't fail the whole operation, just note it
			logging.Logger.Warn().Err(nsErr).Str("namespace", namespace).Msg("Failed to delete namespace")
		} else {
			result.NamespaceDeleted = true
		}
	}

	return result
}

// deleteNamespace deletes a Kubernetes namespace.
func deleteNamespace(ctx context.Context, namespace string) error {
	kubectlCmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", namespace, "--ignore-not-found")
	return kubectlCmd.Run()
}

// uninstallMultipleTargets uninstalls multiple validated targets.
func uninstallMultipleTargets(ctx context.Context, targets []UninstallTarget, deleteNs bool, outputJSON bool) error {
	var wg sync.WaitGroup
	resultCh := make(chan *UninstallResult, len(targets))

	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Skip targets that don't exist
			if !target.NamespaceExists && !target.ReleaseExists {
				resultCh <- &UninstallResult{
					Release:   target.Release,
					Namespace: target.Namespace,
					Status:    "skipped",
					Error:     "namespace and release do not exist",
				}
				return
			}
			result := uninstallRelease(ctx, target.Namespace, target.Release, deleteNs)
			resultCh <- result
		}()
	}

	wg.Wait()
	close(resultCh)

	// Collect results
	results := make([]*UninstallResult, 0, len(targets))
	successCount := 0
	failedCount := 0
	for result := range resultCh {
		results = append(results, result)
		if result.Status == "success" || result.Status == "skipped" {
			successCount++
		} else {
			failedCount++
		}
	}

	// Determine overall status
	status := "success"
	if failedCount == len(results) {
		status = "failed"
	} else if failedCount > 0 {
		status = "partial"
	}

	multiResult := &MultiUninstallResult{
		Status:       status,
		TotalCount:   len(results),
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Results:      make([]UninstallResult, len(results)),
	}
	for i, r := range results {
		multiResult.Results[i] = *r
	}

	return printMultiUninstallResult(multiResult, outputJSON)
}

// printUninstallResult outputs the uninstall result.
func printUninstallResult(result *UninstallResult, outputJSON bool) error {
	if outputJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }

	var b strings.Builder

	switch result.Status {
	case "success":
		b.WriteString(styleOk("✓ Uninstall completed successfully"))
	case "skipped":
		b.WriteString(styleWarn("○ Uninstall skipped"))
	case "failed":
		b.WriteString(styleErr("✗ Uninstall failed"))
	}
	b.WriteString("\n\n")

	maxKey := 12
	fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Release")), styleVal(result.Release))
	fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(result.Namespace))

	if result.Error != "" {
		fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Note")), result.Error)
	}

	if result.NamespaceDeleted {
		fmt.Fprintf(&b, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleOk("deleted"))
	}

	logging.Logger.Info().Msg(b.String())
	return nil
}

// printMultiUninstallResult outputs multiple uninstall results.
func printMultiUninstallResult(result *MultiUninstallResult, outputJSON bool) error {
	if outputJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }

	var b strings.Builder

	// Overall status
	switch result.Status {
	case "success":
		b.WriteString(styleOk("✓ All uninstalls completed successfully"))
	case "partial":
		b.WriteString(styleWarn("○ Some uninstalls completed with issues"))
	case "failed":
		b.WriteString(styleErr("✗ Uninstalls failed"))
	}
	b.WriteString("\n\n")

	// Summary
	b.WriteString(styleHead("Summary"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Total: %d, Success: %s, Failed: %s\n\n",
		result.TotalCount,
		styleOk(fmt.Sprintf("%d", result.SuccessCount)),
		styleErr(fmt.Sprintf("%d", result.FailedCount)))

	// Individual results
	b.WriteString(styleHead("Results"))
	b.WriteString("\n")

	for _, r := range result.Results {
		var statusIcon string
		switch r.Status {
		case "success":
			statusIcon = styleOk("✓")
		case "skipped":
			statusIcon = styleWarn("○")
		case "failed":
			statusIcon = styleErr("✗")
		}

		fmt.Fprintf(&b, "  %s %s/%s", statusIcon, styleKey(r.Namespace), styleVal(r.Release))
		if r.Error != "" {
			fmt.Fprintf(&b, " - %s", r.Error)
		}
		if r.NamespaceDeleted {
			fmt.Fprintf(&b, " %s", styleOk("(namespace deleted)"))
		}
		b.WriteString("\n")
	}

	logging.Logger.Info().Msg(b.String())
	return nil
}


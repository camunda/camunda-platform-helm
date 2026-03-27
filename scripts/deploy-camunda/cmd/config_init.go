package cmd

import (
	"fmt"
	"os"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/wizard"

	"github.com/jwalton/gchalk"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newInitCommand creates the "config init" subcommand.
func newInitCommand() *cobra.Command {
	var (
		editMode   bool
		accessible bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive wizard to generate a deploy.yaml config file",
		Long: `Guided wizard that walks you through creating a deployment configuration.

Asks for platform, chart source, namespace, release, scenario, and optionally
ingress, auth, and secrets settings. Produces a valid deploy.yaml config file.

Examples:
  deploy-camunda config init                # create new config
  deploy-camunda config init --edit         # edit existing config
  deploy-camunda config init --accessible   # plain prompts (non-TTY friendly)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Detect non-TTY and fail gracefully
			if !accessible && !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("init requires an interactive terminal; use --accessible for plain prompts, or use 'config set' for scripted config generation")
			}

			ds := wizard.LiveDataSource{}

			// Load existing config if edit mode
			var existing *config.RootConfig
			if editMode {
				cfgPath, err := config.ResolvePath(configFile)
				if err != nil {
					return fmt.Errorf("failed to resolve config path: %w", err)
				}
				existing, err = config.Read(cfgPath, false)
				if err != nil {
					return fmt.Errorf("failed to read existing config: %w", err)
				}
			}

			w := wizard.NewWizard(ds, existing, accessible)
			rc, cfgPath, err := w.Run()
			if err != nil {
				return err
			}

			// Backup existing file before overwriting
			if _, statErr := os.Stat(cfgPath); statErr == nil {
				backupPath := cfgPath + ".bak"
				data, readErr := os.ReadFile(cfgPath)
				if readErr == nil {
					_ = os.WriteFile(backupPath, data, 0o644)
					fmt.Fprintf(os.Stdout, "%s Existing config backed up to %s\n",
						gchalk.Yellow("!"), backupPath)
				}
			}

			rc.FilePath = cfgPath
			if err := config.Write(rc); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			// Styled completion output
			printCompletion(rc, cfgPath)

			return nil
		},
	}

	cmd.Flags().BoolVar(&editMode, "edit", false, "Edit existing config (pre-populates from current config)")
	cmd.Flags().BoolVar(&accessible, "accessible", false, "Use plain prompts instead of TUI (for non-TTY environments)")

	return cmd
}

// printCompletion shows a styled success message after the wizard completes.
func printCompletion(rc *config.RootConfig, cfgPath string) {
	check := gchalk.Green("✓")
	label := func(s string) string { return gchalk.Cyan(s) }
	value := func(s string) string { return gchalk.Magenta(s) }

	fmt.Fprintf(os.Stdout, "\n%s Configuration saved to %s\n\n", check, gchalk.Bold(cfgPath))

	if rc.Current != "" {
		if dep, ok := rc.Deployments[rc.Current]; ok {
			fmt.Fprintf(os.Stdout, "  %s  %s\n", label("Profile:"), value(rc.Current))
			fmt.Fprintf(os.Stdout, "  %s %s\n", label("Platform:"), value(dep.Platform))
			if dep.ChartPath != "" {
				fmt.Fprintf(os.Stdout, "  %s    %s\n", label("Chart:"), value(dep.ChartPath))
			} else if dep.Chart != "" {
				v := dep.Chart
				if dep.Version != "" {
					v += "@" + dep.Version
				}
				fmt.Fprintf(os.Stdout, "  %s    %s\n", label("Chart:"), value(v))
			}
			fmt.Fprintf(os.Stdout, "  %s    %s/%s\n", label("Where:"), value(dep.Namespace), value(dep.Release))
			fmt.Fprintf(os.Stdout, "  %s %s\n", label("Scenario:"), value(dep.Scenario))
			fmt.Fprintf(os.Stdout, "  %s     %s\n", label("Flow:"), value(dep.Flow))
		}
	}

	// Matrix section
	if len(rc.Matrix.Versions) > 0 || rc.Matrix.MaxParallel != nil {
		fmt.Fprintf(os.Stdout, "\n  %s\n", gchalk.Bold("Matrix:"))
		if len(rc.Matrix.Versions) > 0 {
			fmt.Fprintf(os.Stdout, "  %s %s\n", label("Versions:"), value(fmt.Sprintf("%v", rc.Matrix.Versions)))
		}
		if rc.Matrix.MaxParallel != nil {
			fmt.Fprintf(os.Stdout, "  %s %s\n", label("Parallel:"), value(fmt.Sprintf("%d", *rc.Matrix.MaxParallel)))
		}
		if rc.Matrix.HelmTimeout != nil {
			fmt.Fprintf(os.Stdout, "  %s  %s min\n", label("Timeout:"), value(fmt.Sprintf("%d", *rc.Matrix.HelmTimeout)))
		}
	}

	fmt.Fprintf(os.Stdout, "\n%s\n", gchalk.Dim("Next steps:"))
	fmt.Fprintf(os.Stdout, "  %s    Review your config\n", gchalk.Cyan("deploy-camunda config show"))
	fmt.Fprintf(os.Stdout, "  %s                Start deployment\n", gchalk.Cyan("deploy-camunda"))
}

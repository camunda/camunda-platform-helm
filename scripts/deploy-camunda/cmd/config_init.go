package cmd

import (
	"fmt"
	"os"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/wizard"

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
					fmt.Fprintf(os.Stdout, "Backed up existing config to %s\n", backupPath)
				}
			}

			rc.FilePath = cfgPath
			if err := config.Write(rc); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Fprintf(os.Stdout, "\nConfig written to %s\n", cfgPath)
			fmt.Fprintf(os.Stdout, "Active deployment: %s\n", rc.Current)
			fmt.Fprintf(os.Stdout, "\nRun 'deploy-camunda config show' to review, or 'deploy-camunda' to deploy.\n")

			return nil
		},
	}

	cmd.Flags().BoolVar(&editMode, "edit", false, "Edit existing config (pre-populates from current config)")
	cmd.Flags().BoolVar(&accessible, "accessible", false, "Use plain prompts instead of TUI (for non-TTY environments)")

	return cmd
}

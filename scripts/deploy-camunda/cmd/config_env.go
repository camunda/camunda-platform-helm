package cmd

import (
	"fmt"
	"text/tabwriter"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/prepare-helm-values/pkg/values"

	"github.com/spf13/cobra"
)

// newEnvCommand creates `config env`: prints the effective environment a deploy
// would see and which layer each value came from, with secret values masked.
// Mirrors `git config --show-origin`.
func newEnvCommand() *cobra.Command {
	var showOrigin bool
	var unmask bool

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Show the effective deploy environment and where each value came from",
		Long: `Print the environment variables a deploy would resolve, layered as
process-env → .env file → per-entry overrides, annotated with the winning
source per key. Secret-looking values (key/secret/password/token) are masked
unless --unmask is passed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var flags config.RuntimeFlags
			if _, _, err := config.LoadAndMerge(configFile, true, &flags); err != nil {
				return err
			}

			entries := deploy.EnvProvenance(&flags)
			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			if showOrigin {
				fmt.Fprintln(w, "NAME\tORIGIN\tVALUE")
			} else {
				fmt.Fprintln(w, "NAME\tVALUE")
			}
			for _, e := range entries {
				val := e.Value
				if !unmask && values.IsSecretName(e.Name) {
					val = "***"
				}
				if showOrigin {
					fmt.Fprintf(w, "%s\t%s\t%s\n", e.Name, e.Origin, val)
				} else {
					fmt.Fprintf(w, "%s\t%s\n", e.Name, val)
				}
			}
			return w.Flush()
		},
	}

	cmd.Flags().BoolVar(&showOrigin, "show-origin", true, "Show which layer each value came from")
	cmd.Flags().BoolVar(&unmask, "unmask", false, "Show secret values in clear text (key/secret/password/token)")
	return cmd
}

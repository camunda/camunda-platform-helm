package cmd

import (
	"os"

	"scripts/camunda-core/pkg/ciworkflow"
	"scripts/camunda-core/pkg/ghactions"

	"github.com/spf13/cobra"
)

// newCICommand creates the "ci" parent command grouping subcommands that
// replace bash step logic inside GitHub Actions workflows.
func newCICommand() *cobra.Command {
	ciCmd := &cobra.Command{
		Use:   "ci",
		Short: "Compute CI workflow variables inside GitHub Actions",
	}

	ciCmd.AddCommand(newCITestTypeVarsCommand())

	return ciCmd
}

// newCITestTypeVarsCommand creates the "ci test-type-vars" subcommand. It
// replaces the shell body of the composite action
// .github/actions/test-type-vars: computed vars are appended to the files
// named by $GITHUB_ENV and $GITHUB_OUTPUT (stdout on local runs).
//
// Boolean-like inputs are string flags compared against "true" because the
// composite action forwards its inputs verbatim as separate arguments
// (--upgrade-step "true"), a shape cobra bool flags do not accept.
func newCITestTypeVarsCommand() *cobra.Command {
	var (
		chartDir         string
		flow             string
		prev             string
		upgradeStep      string
		valuesEnterprise string
		valuesDigest     string
	)

	cmd := &cobra.Command{
		Use:   "test-type-vars",
		Short: "Compute the CI test-type variables for a chart version",
		Long: `Compute the CI test-type variables for a chart version and write them
to $GITHUB_ENV / $GITHUB_OUTPUT.

Environment variables:
  GITHUB_WORKSPACE                              Absolute repo checkout path
  DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET   Passed through to $GITHUB_ENV`,
		RunE: func(cmd *cobra.Command, args []string) error {
			in := ciworkflow.TestTypeVarsInput{
				ChartDir:               chartDir,
				Flow:                   flow,
				CamundaVersionPrevious: prev,
				UpgradeStep:            upgradeStep == "true",
				ValuesEnterprise:       valuesEnterprise == "true",
				ValuesDigest:           valuesDigest == "true",
				GitHubWorkspace:        os.Getenv("GITHUB_WORKSPACE"),
				KeycloakClientsSecret:  os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"),
			}

			vars, err := ciworkflow.Compute(in)
			if err != nil {
				return err
			}
			return vars.Emit(ghactions.NewGitHubEnv(), ghactions.NewGitHubOutput())
		},
	}

	cmd.Flags().StringVar(&chartDir, "chart-dir", "", "chart directory name, e.g. camunda-platform-8.10")
	cmd.Flags().StringVar(&flow, "flow", "install", "setup flow: install, upgrade-patch, upgrade-minor")
	cmd.Flags().StringVar(&prev, "camunda-version-previous", "", "previous Camunda minor, e.g. 8.9")
	cmd.Flags().StringVar(&upgradeStep, "upgrade-step", "false", `"true" during the upgrade phase of an upgrade flow`)
	cmd.Flags().StringVar(&valuesEnterprise, "values-enterprise", "false", `"true" to enable enterprise values`)
	cmd.Flags().StringVar(&valuesDigest, "values-digest", "false", `"true" to enable digest values when present`)
	_ = cmd.MarkFlagRequired("chart-dir")

	return cmd
}

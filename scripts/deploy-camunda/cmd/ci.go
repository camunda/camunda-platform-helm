package cmd

import (
	"fmt"
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
	ciCmd.AddCommand(newCIIntegrationMatrixCommand())

	return ciCmd
}

// newCIIntegrationMatrixCommand creates the "ci integration-matrix"
// subcommand. It replaces the string-built yq select() filter in
// test-integration-template.yaml: the platform × flow matrix from
// .github/config/test-integration-matrix.yaml is filtered and written to
// $GITHUB_OUTPUT as 'matrix'.
func newCIIntegrationMatrixCommand() *cobra.Command {
	var (
		configPath string
		platforms  string
		flows      string
		matrixData string
	)

	cmd := &cobra.Command{
		Use:   "integration-matrix",
		Short: "Filter the integration-test platform × flow matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				matrixJSON string
				err        error
			)
			if matrixData != "" {
				matrixJSON, err = ciworkflow.CompactJSON(matrixData)
			} else {
				matrixJSON, err = ciworkflow.FilterIntegrationMatrix(configPath, platforms, flows)
			}
			if err != nil {
				return err
			}
			out := ghactions.NewGitHubOutput()
			if out.Path != "" {
				fmt.Fprintf(os.Stdout, "matrix=%s\n", matrixJSON)
			}
			return out.Set("matrix", matrixJSON)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", ".github/config/test-integration-matrix.yaml", "path to the integration matrix config")
	cmd.Flags().StringVar(&platforms, "platforms", "", "comma-separated platforms to keep, e.g. gke,eks")
	cmd.Flags().StringVar(&flows, "flows", "", "comma-separated flows to keep, e.g. install,upgrade-patch")
	cmd.Flags().StringVar(&matrixData, "matrix-data", "", "explicit matrix JSON override; skips filtering when set")
	_ = cmd.MarkFlagRequired("platforms")
	_ = cmd.MarkFlagRequired("flows")

	return cmd
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

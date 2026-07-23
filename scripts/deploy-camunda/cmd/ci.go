package cmd

import (
	"crypto/rand"
	"encoding/hex"
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
	ciCmd.AddCommand(newCIWorkflowVarsCommand())

	return ciCmd
}

// newCIWorkflowVarsCommand creates the "ci workflow-vars" subcommand. It
// replaces the shell bodies of the composite action
// .github/actions/workflow-vars: infra-config loading, namespace/identifier/
// ingress derivation, upgrade-flow chart-version resolution, and the CI
// snapshot version stamp on Chart.yaml.
func newCIWorkflowVarsCommand() *cobra.Command {
	var (
		platform            string
		setupFlow           string
		deploymentTTL       string
		identifierBase      string
		chartDir            string
		chartUpgradeVersion string
		prefix              string
		prNumber            string
		runID               string
		infraConfig         string
	)

	cmd := &cobra.Command{
		Use:   "workflow-vars",
		Short: "Compute the common CI workflow variables for a test deployment",
		Long: `Compute the common CI workflow variables (namespace, identifier, ingress
host, index prefixes, upgrade chart version) and write them to
$GITHUB_ENV / $GITHUB_OUTPUT, then stamp the chart's CI snapshot version.

Environment variables:
  FLOW   Ambient flow value re-emitted to $GITHUB_ENV (rewritten to "install"
         for the modular-upgrade-minor flow)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			infra, err := ciworkflow.LoadInfraConfig(infraConfig)
			if err != nil {
				return err
			}

			randomID, err := randomHex(3)
			if err != nil {
				return err
			}
			in := ciworkflow.WorkflowVarsInput{
				Platform:       platform,
				SetupFlow:      setupFlow,
				DeploymentTTL:  deploymentTTL,
				IdentifierBase: identifierBase,
				Prefix:         prefix,
				PRNumber:       prNumber,
				RunID:          runID,
				Flow:           os.Getenv("FLOW"),
				RandomID:       randomID,
			}
			vars, err := ciworkflow.ComputeWorkflowVars(in, infra)
			if err != nil {
				return err
			}

			env, out := ghactions.NewGitHubEnv(), ghactions.NewGitHubOutput()
			if err := vars.Emit(env, out); err != nil {
				return err
			}

			version, resolved, err := ciworkflow.ResolveChartVersion(ciworkflow.ResolveChartVersionInput{
				SetupFlow:           setupFlow,
				ChartDir:            chartDir,
				ChartUpgradeVersion: chartUpgradeVersion,
			}, ciworkflow.ExecGit{})
			if err != nil {
				return err
			}
			if resolved {
				if err := env.Set("TEST_CHART_VERSION", version); err != nil {
					return err
				}
			}

			return ciworkflow.StampChartVersion("", chartDir)
		},
	}

	cmd.Flags().StringVar(&platform, "platform", "", "deployment platform, e.g. gke (first entry of a comma-separated list is used)")
	cmd.Flags().StringVar(&setupFlow, "setup-flow", "install", "setup flow: install, upgrade-patch, upgrade-minor, modular-upgrade-minor")
	cmd.Flags().StringVar(&deploymentTTL, "deployment-ttl", "", "deployment lifespan; empty selects the per-run hashed namespace")
	cmd.Flags().StringVar(&identifierBase, "identifier-base", "", "fixed identifier part (PR number or name)")
	cmd.Flags().StringVar(&chartDir, "chart-dir", "", "chart directory name, e.g. camunda-platform-8.10")
	cmd.Flags().StringVar(&chartUpgradeVersion, "chart-upgrade-version", "", "explicit chart version for upgrade flows")
	cmd.Flags().StringVar(&prefix, "prefix", "", "namespace prefix override")
	cmd.Flags().StringVar(&prNumber, "pr-number", "", "github.event.pull_request.number; empty on non-PR events")
	cmd.Flags().StringVar(&runID, "run-id", "", "github.run_id")
	cmd.Flags().StringVar(&infraConfig, "infra-config", ".github/config/infra.yaml", "path to the infra config file")
	_ = cmd.MarkFlagRequired("platform")
	_ = cmd.MarkFlagRequired("chart-dir")
	_ = cmd.MarkFlagRequired("run-id")

	return cmd
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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

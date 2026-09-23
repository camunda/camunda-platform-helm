// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"scripts/camunda-core/pkg/ciworkflow"
	"scripts/camunda-core/pkg/ghactions"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/e2erun"
	"scripts/deploy-camunda/matrix"

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
	ciCmd.AddCommand(newCIIntegrationMatrixCommand())
	ciCmd.AddCommand(newCIE2EMatrixCommand())
	ciCmd.AddCommand(newCIE2ERunCommand())

	return ciCmd
}

// newCIE2ERunCommand creates the "ci e2e-run" subcommand. It runs the e2e legs
// of a scenario inside the job that deployed it, so re-running the failed job
// redeploys before testing again.
//
// Without a mode flag it runs every leg and reports. In CI the workflow runs
// --plan, then one --leg-index step per leg with cluster credentials refreshed
// in between (the kubeconfig token expires after an hour), then --report.
func newCIE2ERunCommand() *cobra.Command {
	var (
		in           e2erun.PlanInput
		scenario     string
		auth         string
		exclude      string
		artifactsDir string
		scrubMapping string
		planOnly     bool
		legIndex     int
		reportOnly   bool
		maxLegs      int
		jobStatus    string
	)

	cmd := &cobra.Command{
		Use:   "e2e-run",
		Short: "Run a scenario's e2e legs against its deployed namespace",
		Long: `Run a scenario's e2e legs against its deployed namespace.

Modes:
  (none)          run every leg sequentially, then report
  --plan          write 'count' and a JSON 'blocking' array to $GITHUB_OUTPUT
  --leg-index N   run leg N and record its result; exits 1 when the leg failed
  --report        aggregate recorded results; exits 1 when a blocking leg
                  failed or never recorded a result

Reports are moved out of the Playwright suite directory after each leg into
--artifacts-dir (blob-report/, test-results/<leg>/, diagnostics/<leg>.txt,
results/<N>.json). The report writes blocking-failed and non-blocking-failed
to $GITHUB_OUTPUT and a leg table to $GITHUB_STEP_SUMMARY.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			modes := 0
			for _, set := range []bool{planOnly, legIndex >= 0, reportOnly} {
				if set {
					modes++
				}
			}
			if modes > 1 {
				return fmt.Errorf("--plan, --leg-index and --report are mutually exclusive")
			}
			if in.RepoRoot == "" {
				detected, err := config.DetectRepoRoot()
				if err != nil {
					return err
				}
				in.RepoRoot = detected
			}
			root, err := filepath.Abs(in.RepoRoot)
			if err != nil {
				return err
			}
			in.RepoRoot = root
			if artifactsDir, err = filepath.Abs(artifactsDir); err != nil {
				return err
			}

			legs, err := e2erun.Plan(in)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			switch {
			case planOnly:
				if maxLegs > 0 && len(legs) > maxLegs {
					return fmt.Errorf("%d e2e legs planned but the workflow runs at most %d; add leg steps to test-integration-runner.yaml", len(legs), maxLegs)
				}
				for i, leg := range legs {
					fmt.Fprintf(cmd.OutOrStdout(), "leg %d: %s (namespace %s, blocking %t)\n", i, leg.ID, leg.Namespace, leg.Blocking)
				}
				blocking := make([]bool, len(legs))
				for i, leg := range legs {
					blocking[i] = leg.Blocking
				}
				blockingJSON, err := json.Marshal(blocking)
				if err != nil {
					return err
				}
				out := ghactions.NewGitHubOutput()
				if err := out.Set("count", strconv.Itoa(len(legs))); err != nil {
					return err
				}
				return out.Set("blocking", string(blockingJSON))
			case reportOnly:
				return reportE2ERun(e2erun.LoadResults(artifactsDir, legs, jobStatus))
			}

			runner := e2erun.Runner{
				RepoRoot:     in.RepoRoot,
				ArtifactsDir: artifactsDir,
				Scenario:     scenario,
				Auth:         auth,
				Exclude:      exclude,
				Exec:         e2erun.ScriptExec(in.RepoRoot, os.Stdout, os.Stderr),
				Diagnostics:  e2erun.DeployCamundaDiagnostics(),
				Log:          cmd.OutOrStdout(),
				Preflight:    e2erun.KubectlPreflight(),
				ScrubEnv:     e2erun.MappedEnvNames(scrubMapping),
			}

			if legIndex >= 0 {
				if legIndex >= len(legs) {
					return fmt.Errorf("--leg-index %d out of range: %d legs planned", legIndex, len(legs))
				}
				lr := runner.RunLeg(ctx, legs[legIndex])
				if err := e2erun.SaveResult(artifactsDir, legIndex, lr); err != nil {
					return fmt.Errorf("record result of leg %s: %w", lr.Leg.ID, err)
				}
				if lr.Failed() {
					return fmt.Errorf("e2e leg %s %s: %w", lr.Leg.ID, lr.Category, lr.Err)
				}
				return nil
			}

			if len(legs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no e2e legs to run")
				return nil
			}
			return reportE2ERun(runner.Run(ctx, legs))
		},
	}

	f := cmd.Flags()
	f.StringVar(&in.RepoRoot, "repo-root", "", "repository root; auto-detected when empty")
	f.StringVar(&in.ChartDir, "chart-dir", "", "chart directory name, e.g. camunda-platform-8.10")
	f.StringVar(&in.Namespace, "namespace", "", "deployed namespace; the base namespace for topology legs")
	f.StringVar(&in.Stage, "stage", "after-install", "test stage used in leg ids: after-install or after-upgrade")
	f.StringVar(&in.SuiteLegs, "legs", "", "suite legs JSON from `ci e2e-matrix`")
	f.StringVar(&in.TopologyLegs, "topology-legs", "", "topology smoke matrix JSON from `matrix plan`")
	f.StringVar(&in.TopologyHubSuffix, "topology-hub-suffix", "", "namespace suffix of the topology Hub release")
	f.BoolVar(&in.Shadow, "shadow", false, "append the non-blocking shadow full-suite leg")
	f.StringVar(&scenario, "scenario", "", "scenario name; selects scenario-specific run-e2e-tests.sh flags")
	f.StringVar(&auth, "auth", "", "authentication type exported as TEST_AUTH_TYPE")
	f.StringVar(&exclude, "exclude", "", "test suites to exclude, exported as TEST_EXCLUDE")
	f.StringVar(&artifactsDir, "artifacts-dir", "e2e-artifacts", "directory collecting reports of every leg")
	f.StringVar(&scrubMapping, "scrub-vault-mapping", "", "vault-action secrets list whose exported variables are removed from the test environment")
	f.BoolVar(&planOnly, "plan", false, "only write the planned leg count to $GITHUB_OUTPUT")
	f.IntVar(&legIndex, "leg-index", -1, "run only this leg (0-based) and record its result")
	f.BoolVar(&reportOnly, "report", false, "aggregate recorded leg results")
	f.IntVar(&maxLegs, "max-legs", 0, "with --plan, fail when more legs are planned than this")
	f.StringVar(&jobStatus, "job-status", "", "with --report, the workflow job.status; labels legs without results as cancelled when it is 'cancelled'")
	_ = cmd.MarkFlagRequired("chart-dir")
	_ = cmd.MarkFlagRequired("namespace")

	return cmd
}

func reportE2ERun(result e2erun.Result) error {
	for _, line := range e2erun.Annotations(result) {
		fmt.Fprintln(os.Stdout, line)
	}
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			_, _ = f.WriteString(e2erun.Summary(result))
			_ = f.Close()
		}
	}
	out := ghactions.NewGitHubOutput()
	if err := out.Set("blocking-failed", strconv.FormatBool(result.BlockingFailed())); err != nil {
		return err
	}
	if err := out.Set("non-blocking-failed", strconv.FormatBool(result.NonBlockingFailed())); err != nil {
		return err
	}
	if result.BlockingFailed() {
		return fmt.Errorf("one or more blocking e2e legs failed")
	}
	return nil
}

// newCIE2EMatrixCommand creates the "ci e2e-matrix" subcommand. It resolves the
// scenario's e2e legs from the chart version's CI scenario registry and writes
// them to $GITHUB_OUTPUT as the JSON matrix test-integration-runner.yaml feeds
// to `matrix.include`, so the Playwright project and the blocking behavior of
// each leg come from the registry instead of a scenario-name comparison.
func newCIE2EMatrixCommand() *cobra.Command {
	var (
		repoRoot  string
		chartDir  string
		shortname string
		scenario  string
	)

	cmd := &cobra.Command{
		Use:   "e2e-matrix",
		Short: "Resolve a scenario's e2e matrix legs from the CI scenario registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := repoRoot
			if root == "" {
				detected, err := config.DetectRepoRoot()
				if err != nil {
					return err
				}
				root = detected
			}

			legs, err := matrix.ResolveE2ELegs(root, chartDir, shortname, scenario)
			if err != nil {
				return err
			}
			legsJSON, err := matrix.E2ELegsJSON(legs)
			if err != nil {
				return err
			}

			out := ghactions.NewGitHubOutput()
			if out.Path == "" {
				fmt.Fprintf(os.Stdout, "e2e-matrix=%s\n", legsJSON)
				return nil
			}
			fmt.Fprintf(os.Stdout, "e2e-matrix=%s\n", legsJSON)
			return out.Set("e2e-matrix", legsJSON)
		},
	}

	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "repository root; auto-detected when empty")
	cmd.Flags().StringVar(&chartDir, "chart-dir", "", "chart directory name, e.g. camunda-platform-8.10")
	cmd.Flags().StringVar(&shortname, "shortname", "", "scenario shortname; preferred over --scenario because scenario names are not unique")
	cmd.Flags().StringVar(&scenario, "scenario", "", "scenario name as declared in the registry; used when --shortname matches nothing")
	_ = cmd.MarkFlagRequired("chart-dir")
	_ = cmd.MarkFlagRequired("scenario")

	return cmd
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

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newDoctorCommand creates the `doctor` subcommand: a read-only preflight that
// reports whether the secrets/env setup a deploy needs is in place, with a
// remediation hint for anything missing. Mirrors `flyctl doctor`.
func newDoctorCommand() *cobra.Command {
	var skipKube bool
	var fix bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose deploy setup: config, kube-context, docker creds, required secrets/env",
		Long: `Run read-only preflight checks and print a ✓/✗ checklist.

Resolves config and .env exactly as a deploy would, then verifies the kube
context is reachable, docker credentials are present, every variable in the
vault secret mapping is set, and every $PLACEHOLDER in the selected scenario's
values is satisfied — so a missing input surfaces here instead of mid-deploy.

Exits non-zero if any required check fails.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := logging.Setup(logging.Options{
				LevelString:  flags.LogLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			// Record explicitly-set flags so config merge won't clobber them.
			flags.ChangedFlags = make(map[string]bool)
			cmd.Flags().Visit(func(f *pflag.Flag) {
				flags.ChangedFlags[f.Name] = true
			})

			_, cfgRes, err := config.LoadAndMerge(configFile, true, &flags)
			if err != nil {
				return err
			}
			if flags.Chart.RepoRoot == "" {
				if detected, _ := config.DetectRepoRoot(); detected != "" {
					flags.Chart.RepoRoot = detected
				}
			}

			report := deploy.Preflight(ctx, &flags, deploy.PreflightOptions{
				ConfigPath:           cfgRes.Path,
				ConfigFound:          cfgRes.Found,
				SkipKubeReachability: skipKube,
			})

			var buf bytes.Buffer
			report.Render(&buf)
			fmt.Fprint(os.Stdout, buf.String())

			// --fix: prompt for the missing vars, persist to .env, and re-check.
			if fix && !report.OK() {
				if n, err := deploy.ResolveMissingInteractively(ctx, report, &flags); err != nil {
					return err
				} else if n > 0 {
					report = deploy.Preflight(ctx, &flags, deploy.PreflightOptions{
						ConfigPath:           cfgRes.Path,
						ConfigFound:          cfgRes.Found,
						SkipKubeReachability: skipKube,
					})
					var after bytes.Buffer
					report.Render(&after)
					fmt.Fprintf(os.Stdout, "\nAfter --fix:\n%s", after.String())
				}
			}

			if !report.OK() {
				return fmt.Errorf("preflight failed: fix the ✗ checks above (run `deploy-camunda doctor --fix` to be prompted, or `deploy-camunda config init`)")
			}
			return nil
		},
	}

	// Inputs that scope the checks. Bound to the shared flags struct so config
	// merging fills in anything not passed explicitly.
	f := cmd.Flags()
	f.StringVar(&flags.Chart.ChartPath, "chart-path", "", "Path to the Camunda chart directory")
	f.StringVarP(&flags.Chart.Chart, "chart", "c", "", "Chart name")
	f.StringVarP(&flags.Chart.ChartVersion, "version", "v", "", "Chart version")
	f.StringVarP(&flags.Deployment.Scenario, "scenario", "s", "", "Scenario to check (comma-separated for multiple)")
	f.StringVar(&flags.Deployment.ScenarioPath, "scenario-path", "", "Path to scenario files")
	f.StringVar(&flags.Test.KubeContext, "kube-context", "", "Kubernetes context to check")
	f.StringVar(&flags.EnvFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	f.StringVar(&flags.Secrets.VaultSecretMapping, "vault-secret-mapping", "", "Vault secret mapping content")
	f.StringVar(&flags.Docker.DockerUsername, "docker-username", "", "Harbor registry username")
	f.StringVar(&flags.Docker.DockerPassword, "docker-password", "", "Harbor registry password")
	f.BoolVar(&flags.Docker.EnsureDockerRegistry, "ensure-docker-registry", false, "Treat Harbor pull secret as required")
	f.BoolVar(&flags.Docker.EnsureDockerHub, "ensure-docker-hub", false, "Treat Docker Hub pull secret as required")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info", "Log level")
	f.BoolVar(&skipKube, "skip-kube-check", false, "Skip the cluster reachability probe")
	f.BoolVar(&fix, "fix", false, "Prompt for missing variables and write them to the .env file")

	return cmd
}

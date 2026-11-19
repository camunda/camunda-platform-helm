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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"scripts/camunda-core/pkg/ghactions"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/matrix"
	"scripts/prepare-helm-values/pkg/env"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newMatrixCommand creates the matrix parent command with list and run subcommands.
func newMatrixCommand() *cobra.Command {
	matrixCmd := &cobra.Command{
		Use:   "matrix",
		Short: "Generate and run the CI test matrix across all active chart versions",
	}

	matrixCmd.AddCommand(newMatrixListCommand())
	matrixCmd.AddCommand(newMatrixRunCommand())
	matrixCmd.AddCommand(newMatrixPlanCommand())

	return matrixCmd
}

// validateTierFlag accepts 0 (no tier filter), 1, and 2; every other value is
// an error. Without this, a positive out-of-range tier silently filters the
// matrix down to nothing and a negative one disables filtering entirely.
func validateTierFlag(tier int) error {
	switch tier {
	case 0, 1, 2:
		return nil
	default:
		return fmt.Errorf("invalid --tier %d: supported values are 1 (PR CI), 2 (merge-queue only), or 0 for no filter", tier)
	}
}

// newMatrixPlanCommand creates the "matrix plan" subcommand. It replaces
// scripts/generate-chart-matrix.sh + generate-chart-matrix.jq: it decides
// which chart versions a change set affects and emits the GitHub Actions
// build matrix to $GITHUB_OUTPUT (stdout on local runs).
func newMatrixPlanCommand() *cobra.Command {
	var (
		activeVersions   string
		allModifiedFiles string
		manualTrigger    string
		manualScenario   string
		manualFlow       string
		tier             string
		repoRoot         string
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Compute the chart build matrix for a change set",
		Long: `Compute the chart build matrix for a change set and write it to
$GITHUB_OUTPUT as 'matrix' ({"include": [...]}) and 'camunda-versions'
(unique version array).

Which versions build: a manual trigger ("all" or a single version) wins;
otherwise changed files under .github/workflows, .github/actions,
.github/config (except release-please), or scripts/ build every active
version, and chart-only changes build just the affected versions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repoRoot == "" {
				detected, err := config.DetectRepoRoot()
				if err != nil {
					return err
				}
				repoRoot = detected
			}
			if repoRoot == "" {
				return fmt.Errorf("--repo-root is required (or run from within the repo)")
			}

			tierValue := 0
			if tier != "" {
				parsed, err := strconv.Atoi(tier)
				if err != nil {
					return fmt.Errorf("invalid --tier %q: %w", tier, err)
				}
				tierValue = parsed
			}
			if err := validateTierFlag(tierValue); err != nil {
				return err
			}

			result, err := matrix.Plan(repoRoot, matrix.PlanOptions{
				ActiveVersions: strings.Fields(strings.ReplaceAll(activeVersions, ",", " ")),
				ChangedFiles:   allModifiedFiles,
				ManualTrigger:  manualTrigger,
				ManualScenario: manualScenario,
				ManualFlow:     manualFlow,
				Tier:           tierValue,
			})
			if err != nil {
				return err
			}

			matrixJSON, err := result.MatrixJSON()
			if err != nil {
				return err
			}
			versionsJSON, err := result.VersionsJSON()
			if err != nil {
				return err
			}
			if len(result.Include) == 0 {
				fmt.Fprintln(os.Stdout, "No matching chart changes detected; emitting empty matrix and skipping downstream jobs.")
			}

			out := ghactions.NewGitHubOutput()
			if out.Path != "" {
				fmt.Fprintf(os.Stdout, "matrix=%s\ncamunda-versions=%s\n", matrixJSON, versionsJSON)
			}
			if err := out.Set("matrix", matrixJSON); err != nil {
				return err
			}
			return out.Set("camunda-versions", versionsJSON)
		},
	}

	f := cmd.Flags()
	f.StringVar(&activeVersions, "active-versions", "", "active chart versions (space- or comma-separated, e.g. \"8.7 8.8 8.9 8.10\")")
	f.StringVar(&allModifiedFiles, "all-modified-files", "", "changed files/dirs (whitespace-separated, as emitted by tj-actions/changed-files)")
	f.StringVar(&manualTrigger, "manual-trigger", "none", "\"none\", \"all\", or a single chart version to build")
	f.StringVar(&manualScenario, "manual-scenario", "none", "keep only this exact scenario (\"none\"/\"all\" keep everything)")
	f.StringVar(&manualFlow, "manual-flow", "none", "override flows (comma-separated: install, upgrade-patch, upgrade-minor)")
	f.StringVar(&tier, "tier", "", "filter scenarios by tier (1=PR CI, 2=merge-queue only; empty or 0=all)")
	f.StringVar(&repoRoot, "repo-root", "", "repository root path (default: auto-detect)")
	_ = cmd.MarkFlagRequired("active-versions")

	return cmd
}

// newMatrixListCommand creates the "matrix list" subcommand.
func newMatrixListCommand() *cobra.Command {
	var (
		versions        []string
		includeDisabled bool
		scenarioFilter  string
		shortnameFilter string
		shortnameExact  bool
		flowFilter      string
		outputFormat    string
		platform        string
		repoRoot        string
		tier            int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the CI test matrix for routine chart versions",
		Long: `List the full CI test matrix generated from chart-versions.yaml,
ci-test-config.yaml (PR scenarios only), and permitted-flows.yaml.

This command does not require cluster access.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Track which CLI flags were explicitly set so config merging
			// does not overwrite them.
			changedFlags := make(map[string]bool)
			cmd.Flags().Visit(func(f *pflag.Flag) {
				changedFlags[f.Name] = true
			})

			rc, err := config.LoadMatrixConfig(configFile)
			if err != nil {
				return err
			}
			infra := rc.ResolveInfra(true, "", "", matrixCLIInfra(cmd.Flags()))
			platform, repoRoot = infra.Platform, infra.RepoRoot
			m := rc.Matrix
			config.MergeStringSliceField(&versions, m.Versions, nil)
			config.MergeBoolField(&includeDisabled, m.IncludeDisabled, nil, changedFlags, "include-disabled")
			config.MergeStringField(&scenarioFilter, m.ScenarioFilter, "", changedFlags, "scenario-filter")
			config.MergeStringField(&shortnameFilter, m.ShortnameFilter, "", changedFlags, "shortname-filter")
			config.MergeStringField(&flowFilter, m.FlowFilter, "", changedFlags, "flow-filter")
			config.MergeStringField(&outputFormat, m.OutputFormat, "", changedFlags, "format")

			if repoRoot == "" {
				detected, err := config.DetectRepoRoot()
				if err != nil {
					return err
				}
				repoRoot = detected
			}
			if repoRoot == "" {
				return fmt.Errorf("--repo-root is required (or set repoRoot in config, or run from within the repo)")
			}

			if err := validateTierFlag(tier); err != nil {
				return err
			}

			entries, err := matrix.Generate(repoRoot, matrix.GenerateOptions{
				Versions:        versions,
				IncludeDisabled: includeDisabled,
				Platform:        platform,
			})
			if err != nil {
				return err
			}

			entries = matrix.Filter(entries, matrix.FilterOptions{
				ScenarioFilter:  scenarioFilter,
				ShortnameFilter: shortnameFilter,
				ShortnameExact:  shortnameExact,
				FlowFilter:      flowFilter,
				Platform:        platform,
				Tier:            tier,
			})

			output, err := matrix.Print(entries, outputFormat)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, output)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringSliceVar(&versions, "versions", nil, "Limit to specific chart versions (comma-separated, e.g., 8.8,8.9)")
	f.BoolVar(&includeDisabled, "include-disabled", false, "Include disabled scenarios in the output")
	f.StringVar(&scenarioFilter, "scenario-filter", "", "Filter scenarios by substring match (comma-separated for multiple, e.g. elasticsearch,opensearch)")
	f.StringVar(&shortnameFilter, "shortname-filter", "", "Filter entries by shortname substring match (comma-separated for multiple, e.g. eske,eshy)")
	f.BoolVar(&shortnameExact, "shortname-exact", false, "Treat each --shortname-filter value as an exact match instead of a substring (recommended for per-scenario CI use)")
	f.StringVar(&flowFilter, "flow-filter", "", "Filter entries by exact flow name")
	f.StringVar(&outputFormat, "format", "table", "Output format: table, json")
	f.StringVar(&platform, "platform", "", "Filter entries to those supporting this platform")
	f.StringVar(&repoRoot, "repo-root", "", "Repository root path (or set repoRoot in config)")
	f.IntVar(&tier, "tier", 0, "Filter entries by tier (1=PR CI, 2=merge-queue only; 0=all)")

	registerMatrixShortnameCompletion(cmd)
	registerMatrixVersionsCompletion(cmd)
	registerMatrixFlowCompletion(cmd)

	return cmd
}

// newMatrixRunCommand creates the "matrix run" subcommand.
func newMatrixRunCommand() *cobra.Command {
	return (&matrixRunFlags{}).command()
}

type matrixRunFlags struct {
	matrix.RunOptions
	filter            matrix.FilterOptions
	versions          []string
	includeDisabled   bool
	namespacePrepared bool
	yes               bool
}

func (r *matrixRunFlags) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the CI test matrix against a live cluster",
		Long: `Deploy the CI test matrix — the cartesian product of chart versions,
scenarios, and flows declared in charts/<v>/test/ci-test-config.yaml — to a
live cluster. Each entry gets its own namespace (<prefix>-<version>-<shortname>)
so entries stay isolated and can run in parallel with --max-parallel.

FILTER FIRST. Without --shortname-filter or --versions the runner attempts
every enabled scenario across every active chart version, which is not what
you want interactively. Common first-run pattern:

  # Deploy a single 8.10 scenario, scoping by shortname:
  deploy-camunda matrix run \
    --repo-root . \
    --versions 8.10 \
    --shortname-filter keyco \
    --ingress-base-domain ci.distro.ultrawombat.com \
    --platform gke

Docker Hub is required whenever a scenario pulls from docker.io — supply
credentials via env or flags, and set --ensure-docker-hub so the pull
secret is created before the deploy:

  DOCKERHUB_USERNAME=... DOCKERHUB_PASSWORD=... \
  deploy-camunda matrix run \
    --repo-root . --versions 8.10 --shortname-filter keyco \
    --ingress-base-domain ci.distro.ultrawombat.com \
    --ensure-docker-hub

Prefer a config file over a long flag list — copy the getting-started
starter and iterate from there:

  deploy-camunda config init --from-example getting-started
  # edit .deploy-camunda.yaml to set kubeContext / ingressBaseDomain
  deploy-camunda matrix run --versions 8.10 --shortname-filter keyco

Use --cleanup to delete each entry's namespace after its deployment and
tests complete, or --delete-namespace to start clean before each entry.

Under the hood this invokes deploy.Execute() for each matrix entry.`,
		Example: `  # Minimal single-scenario run:
  deploy-camunda matrix run \
    --repo-root . --versions 8.10 --shortname-filter keyco \
    --ingress-base-domain ci.distro.ultrawombat.com --platform gke

  # With Docker Hub credentials for docker.io images:
  DOCKERHUB_USERNAME=... DOCKERHUB_PASSWORD=... \
  deploy-camunda matrix run \
    --repo-root . --versions 8.10 --shortname-filter keyco \
    --ingress-base-domain ci.distro.ultrawombat.com --ensure-docker-hub

  # Config-file driven (recommended for repeat use):
  deploy-camunda config init --from-example getting-started
  deploy-camunda matrix run --versions 8.10 --shortname-filter keyco`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateIngressReadyFlags(r.WaitIngressReady, r.IngressReadyTimeoutMinutes); err != nil {
				return err
			}
			return validateChartRefFlags(r.ChartRef, r.ChartRefVersion)
		},
		RunE: r.run,
	}

	f := cmd.Flags()
	f.StringSliceVar(&r.versions, "versions", nil, "Limit to specific chart versions (comma-separated, e.g., 8.8,8.9)")
	f.BoolVar(&r.includeDisabled, "include-disabled", false, "Include disabled scenarios in the output")
	f.StringVar(&r.filter.ScenarioFilter, "scenario-filter", "", "Filter scenarios by substring match (comma-separated for multiple, e.g. elasticsearch,opensearch)")
	f.StringVar(&r.filter.ShortnameFilter, "shortname-filter", "", "Filter entries by shortname substring match (comma-separated for multiple, e.g. eske,eshy)")
	f.BoolVar(&r.filter.ShortnameExact, "shortname-exact", false, "Treat each --shortname-filter value as an exact match instead of a substring (recommended for per-scenario CI use)")
	f.StringVar(&r.filter.FlowFilter, "flow-filter", "", "Filter entries by exact flow name")
	f.StringVar(&r.Platform, "platform", "", "Filter entries to those supporting this platform (also sets deploy platform)")
	f.StringVar(&r.RepoRoot, "repo-root", "", "Repository root path (or set repoRoot in config)")
	f.BoolVar(&r.DryRun, "dry-run", false, "Log what would be deployed without actually deploying")
	f.BoolVar(&r.Coverage, "coverage", false, "Show a layer-breakdown report of what is tested in the matrix (no deployment)")
	f.BoolVar(&r.TestE2E, "test-e2e", false, "Run e2e tests after each deployment")
	f.BoolVar(&r.TestAll, "test-all", false, "Run all e2e tests after each deployment")
	f.BoolVar(&r.StopOnFailure, "stop-on-failure", false, "Stop the run on the first failure")
	f.StringVar(&r.NamespacePrefix, "namespace-prefix", "matrix", "Prefix for generated namespaces")
	f.BoolVar(&r.Cleanup, "cleanup", false, "Delete each entry's namespace after its deployment and tests complete")
	f.BoolVar(&r.DeleteNamespaceFirst, "delete-namespace", false, "Delete the namespace before deploying each entry (clean-slate deployment)")
	f.String("kube-context", "", "Default Kubernetes context for all platforms (overridden by --kube-context-gke/--kube-context-eks)")
	f.String("kube-context-gke", "", "Kubernetes context for GKE entries")
	f.String("kube-context-eks", "", "Kubernetes context for EKS entries")
	f.String("ingress-base-domain", "", "Fallback base DNS zone used to compute each entry's public URL — joined into CAMUNDA_HOSTNAME as <namespace>.<base>. Set to the DNS zone the target cluster's ingress controller serves, e.g. `ci.distro.ultrawombat.com` (Camunda CI) or `apps.mycompany.example`. Overridden per-platform by --ingress-base-domain-gke/--ingress-base-domain-eks.")
	f.String("ingress-base-domain-gke", "", "Ingress base domain for GKE entries (e.g., ci.distro.ultrawombat.com)")
	f.String("ingress-base-domain-eks", "", "Ingress base domain for EKS entries (e.g., distribution.aws.camunda.cloud)")
	f.IntVar(&r.MaxParallel, "max-parallel", 1, "Maximum number of entries to run concurrently (1 = sequential)")
	f.String("env-file", "", "Default .env file for all versions (overridden by --env-file-X.Y)")
	f.String("env-file-8.6", "", "Path to .env file for 8.6 entries")
	f.String("env-file-8.7", "", "Path to .env file for 8.7 entries")
	f.String("env-file-8.8", "", "Path to .env file for 8.8 entries")
	f.String("env-file-8.9", "", "Path to .env file for 8.9 entries")
	f.StringVarP(&r.LogLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.BoolVar(&r.SkipDependencyUpdate, "skip-dependency-update", false, "Skip helm dependency update before deploying")
	f.Bool("use-vault-backed-secrets", false, "Use vault-backed external secrets for all platforms (overridden by --use-vault-backed-secrets-gke/--use-vault-backed-secrets-eks)")
	f.Bool("use-vault-backed-secrets-gke", false, "Use vault-backed external secrets for GKE entries")
	f.Bool("use-vault-backed-secrets-eks", false, "Use vault-backed external secrets for EKS entries")
	f.StringVar(&r.KeycloakHost, "keycloak-host", "", "Keycloak external host")
	f.StringVar(&r.KeycloakProtocol, "keycloak-protocol", "", "Keycloak protocol (defaults to "+config.DefaultKeycloakProtocol+")")
	f.StringVar(&r.UpgradeFromVersion, "upgrade-from-version", "", "Override the auto-resolved 'from' chart version for upgrade flows (e.g., 13.5.0)")
	f.IntVar(&r.HelmTimeout, "timeout", 10, "Timeout in minutes for Helm deployment (applies to all entries)")
	bindRegistryFlags(f, &r.DockerFlags)
	f.BoolVar(&r.UseLatest, "use-latest", false, "Use values-latest.yaml from each chart root instead of values-digest.yaml")
	f.BoolVar(&r.UseQA, "use-qa", false, "Force the base-qa layer to be included for all entries, regardless of per-scenario qa config")
	f.BoolVar(&r.ForceImageOverrides, "force-image-overrides", false, "Bypass OCI immutability guard: apply chart-root image overlays and scenario image-tags on top of --chart-ref.")
	f.BoolVarP(&r.yes, "yes", "y", false, "Skip confirmation prompts (e.g., e2e threshold warning)")
	f.StringVar(&r.LogDir, "log-dir", "", "Write logs to this directory and show a live status table (auto-generated when running in a TTY)")
	f.StringArrayVar(&r.ExtraHelmArgs, "extra-helm-arg", nil, "Extra argument appended to every helm command (repeatable, e.g. --extra-helm-arg=--set-file=global.license.secret.inlineSecret=/tmp/license.txt)")
	f.StringSliceVar(&r.ExtraHelmSets, "extra-helm-set", nil, "Extra helm --set key=value pair applied to every entry (comma-separated or repeatable, e.g. orchestration.upgrade.allowPreReleaseImages=true)")
	f.StringArrayVar(&r.ExtraValues, "extra-values", nil, "Additional Helm values files appended last for every entry (repeatable; not comma-split — use the flag multiple times for multiple files). Ranked by the image resolver; prefer over --extra-helm-arg=--values=. In two-step upgrade flows, applied to Step 2 only.")
	f.StringArrayVar(&r.ImageOverrides, "image-override", nil, "Pin one component's image above every values file for every entry (repeatable), e.g. orchestration=registry.camunda.cloud/team-camunda/camunda:8.10.0-SNAPSHOT. In two-step upgrade flows, applied to Step 2 only.")
	f.BoolVar(&r.AllowDigestShadow, "allow-digest-shadow", false, "Keep a pinned digest when a later values file changes only the image registry/repository (e.g. a pull-through mirror)")
	f.StringVar(&r.NamespaceOverride, "namespace-override", "", "Deploy into one exact namespace that CI or another external process already provisioned. Disables automatic ExternalSecrets; local runs should use --namespace-prefix.")
	f.BoolVar(&r.namespacePrepared, "namespace-prepared", false, "Confirm the exact namespace selected by --namespace-override already has required secrets, TLS, and pull secrets.")
	f.StringVar(&r.ChartRef, "chart-ref", "", "Override chart source with an OCI reference or .tgz path (e.g., oci://registry.camunda.cloud/team-distribution/camunda-platform). Values are still resolved from the local repo via --repo-root.")
	f.StringVar(&r.ChartRefVersion, "chart-version", "", "Chart version to install from --chart-ref (e.g., 13-rc-latest). Only meaningful when --chart-ref is set.")
	f.IntVar(&r.filter.Tier, "tier", 0, "Filter entries by tier (1=PR CI, 2=merge-queue only; 0=all)")
	f.BoolVar(&r.WaitIngressReady, "wait-ingress-ready", false, "After a successful helm install/upgrade, fail the entry unless its ingress host becomes publicly DNS-resolvable and HTTP-reachable within --ingress-ready-timeout")
	f.IntVar(&r.IngressReadyTimeoutMinutes, "ingress-ready-timeout", config.DefaultIngressReadyTimeoutMinutes, "Timeout in minutes for --wait-ingress-ready")

	registerMatrixShortnameCompletion(cmd)
	registerMatrixVersionsCompletion(cmd)
	registerMatrixFlowCompletion(cmd)
	registerIngressBaseDomainCompletion(cmd)
	registerIngressBaseDomainCompletionForFlag(cmd, "ingress-base-domain-gke")
	registerIngressBaseDomainCompletionForFlag(cmd, "ingress-base-domain-eks")
	registerKubeContextCompletion(cmd)
	registerKubeContextCompletionForFlag(cmd, "kube-context-gke")
	registerKubeContextCompletionForFlag(cmd, "kube-context-eks")
	_ = cmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeLogLevels(toComplete)
	})

	annotateFlagGroups(cmd, matrixRunFlagGroups())

	return cmd
}

func (r *matrixRunFlags) configure(cmd *cobra.Command) error {
	changedFlags := make(map[string]bool)
	cmd.Flags().Visit(func(f *pflag.Flag) { changedFlags[f.Name] = true })
	rc, err := config.LoadMatrixConfig(configFile)
	if err != nil {
		return err
	}
	r.Config, r.Infra = rc, matrixCLIInfra(cmd.Flags())
	infra := rc.ResolveInfra(true, "", "", r.Infra)
	r.Platform, r.RepoRoot = infra.Platform, infra.RepoRoot
	r.NamespacePrefix, r.LogLevel = config.FirstNonEmpty(infra.NamespacePrefix, r.NamespacePrefix), config.FirstNonEmpty(infra.LogLevel, r.LogLevel)
	r.DockerUsername, r.DockerPassword = infra.DockerUsername, infra.DockerPassword
	r.DockerHubUsername, r.DockerHubPassword = infra.DockerHubUsername, infra.DockerHubPassword
	for target, value := range map[*bool]*bool{&r.SkipDependencyUpdate: infra.SkipDependencyUpdate, &r.DeleteNamespaceFirst: infra.DeleteNamespace, &r.EnsureDockerRegistry: infra.EnsureDockerRegistry, &r.EnsureDockerHub: infra.EnsureDockerHub} {
		if value != nil {
			*target = *value
		}
	}
	m := rc.Matrix
	config.MergeStringSliceField(&r.versions, m.Versions, nil)
	config.MergeBoolField(&r.includeDisabled, m.IncludeDisabled, nil, changedFlags, "include-disabled")
	config.MergeStringField(&r.filter.ScenarioFilter, m.ScenarioFilter, "", changedFlags, "scenario-filter")
	config.MergeStringField(&r.filter.ShortnameFilter, m.ShortnameFilter, "", changedFlags, "shortname-filter")
	config.MergeStringField(&r.filter.FlowFilter, m.FlowFilter, "", changedFlags, "flow-filter")
	config.MergeIntField(&r.MaxParallel, m.MaxParallel, nil, changedFlags, "max-parallel")
	config.MergeBoolField(&r.StopOnFailure, m.StopOnFailure, nil, changedFlags, "stop-on-failure")
	config.MergeBoolField(&r.Cleanup, m.Cleanup, nil, changedFlags, "cleanup")
	config.MergeBoolField(&r.DryRun, m.DryRun, nil, changedFlags, "dry-run")
	config.MergeBoolField(&r.Coverage, m.Coverage, nil, changedFlags, "coverage")
	config.MergeIntField(&r.HelmTimeout, m.HelmTimeout, nil, changedFlags, "timeout")
	config.MergeBoolField(&r.TestE2E, m.TestE2E, nil, changedFlags, "test-e2e")
	config.MergeBoolField(&r.TestAll, m.TestAll, nil, changedFlags, "test-all")
	config.MergeStringField(&r.KeycloakHost, m.KeycloakHost, rc.Keycloak.Host, changedFlags, "keycloak-host")
	config.MergeStringField(&r.KeycloakProtocol, m.KeycloakProtocol, rc.Keycloak.Protocol, changedFlags, "keycloak-protocol")
	config.MergeStringField(&r.UpgradeFromVersion, m.UpgradeFromVersion, "", changedFlags, "upgrade-from-version")

	if err := logging.Setup(logging.Options{
		LevelString: r.LogLevel, ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
	}); err != nil {
		return err
	}
	warnDeprecatedInfra(rc)
	envFile := infra.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	logging.Logger.Debug().Str("envFile", envFile).Msg("Loading environment file")
	if err := env.Load(envFile); err != nil {
		logging.Logger.Warn().Err(err).Str("envFile", envFile).Msg("Failed to load environment file")
	}
	if r.RepoRoot == "" {
		r.RepoRoot, err = config.DetectRepoRoot()
		if err != nil {
			return err
		}
	}
	if r.RepoRoot == "" {
		return fmt.Errorf("--repo-root is required (or set repoRoot in config, or run from within the repo)")
	}
	if err := validateNamespaceOverride(r.NamespaceOverride, r.namespacePrepared, r.DryRun || r.Coverage, os.Getenv("GITHUB_ACTIONS")); err != nil {
		return err
	}
	for _, domain := range []struct{ flag, value string }{
		{"ingress-base-domain", infra.IngressBaseDomain},
		{"ingress-base-domain-gke", r.Infra.IngressBaseDomains["gke"]},
		{"ingress-base-domain-eks", r.Infra.IngressBaseDomains["eks"]},
	} {
		if domain.value != "" && !config.IsValidIngressBaseDomain(domain.value) {
			return fmt.Errorf("--%s must be one of: %s", domain.flag, strings.Join(config.ValidIngressBaseDomains, ", "))
		}
	}
	return validateTierFlag(r.filter.Tier)
}

func (r *matrixRunFlags) entries() ([]matrix.Entry, error) {
	opts := matrix.GenerateOptions{Versions: r.versions, IncludeDisabled: r.includeDisabled, Platform: r.Platform}
	entries, err := matrix.Generate(r.RepoRoot, opts)
	if err != nil {
		return nil, err
	}
	r.filter.Platform = r.Platform
	entries = matrix.Filter(entries, r.filter)
	if len(entries) == 0 && !r.includeDisabled {
		opts.IncludeDisabled = true
		withDisabled, err := matrix.Generate(r.RepoRoot, opts)
		if err != nil {
			return nil, err
		}
		if matches := matrix.Filter(withDisabled, r.filter); len(matches) > 0 {
			return nil, fmt.Errorf("no enabled matrix entries matched the filters (versions=%v); matching scenarios are disabled; re-run with --include-disabled to include them", r.versions)
		}
	}
	return entries, nil
}

func (r *matrixRunFlags) resolveCredentials(entries []matrix.Entry, hasTopology bool) error {
	docker := r.DockerFlags
	docker.EnsureDockerRegistry = docker.EnsureDockerRegistry || hasTopology
	envFiles := make(map[string]string)
	for _, entry := range entries {
		envFiles[entry.Version] = r.Config.ResolveInfra(true, "", entry.Version, r.Infra).EnvFile
	}
	if err := resolveRegistryCredentialsFromEnvFiles(&docker, entries, envFiles); err != nil {
		return err
	}
	r.DockerUsername, r.DockerPassword = docker.DockerUsername, docker.DockerPassword
	r.DockerHubUsername, r.DockerHubPassword = docker.DockerHubUsername, docker.DockerHubPassword
	return nil
}

func (r *matrixRunFlags) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := r.configure(cmd); err != nil {
		return err
	}
	allEntries, err := r.entries()
	if err != nil {
		return err
	}
	var entries, topologyEntries []matrix.Entry
	for _, entry := range allEntries {
		if entry.Topology != nil {
			topologyEntries = append(topologyEntries, entry)
		} else {
			entries = append(entries, entry)
		}
	}
	allEntries = append(append([]matrix.Entry{}, entries...), topologyEntries...)
	if err := r.resolveCredentials(allEntries, len(topologyEntries) > 0); err != nil {
		return err
	}
	if err := matrix.RunTopologies(ctx, topologyEntries, r.RunOptions); err != nil {
		return err
	}
	if len(entries) == 0 {
		if r.NamespaceOverride != "" || r.filter.ShortnameFilter != "" || r.filter.ScenarioFilter != "" || r.filter.FlowFilter != "" {
			if len(topologyEntries) > 0 {
				return nil
			}
			return fmt.Errorf("no matrix entries matched the filters (versions=%v, scenario-filter=%q, shortname-filter=%q, flow-filter=%q, platform=%q); check ci-test-config.yaml has an entry for this scenario+flow combination",
				r.versions, r.filter.ScenarioFilter, r.filter.ShortnameFilter, r.filter.FlowFilter, r.Platform)
		}
		fmt.Fprintln(os.Stdout, "No matrix entries matched the filters.")
		return nil
	}
	if err := validateChartRefVersionSpan(r.ChartRef, entries); err != nil {
		return err
	}
	if err := r.confirmE2E(len(entries)); err != nil {
		return err
	}
	return runMatrixEntries(ctx, entries, r.RunOptions)
}

func (r *matrixRunFlags) confirmE2E(entries int) error {
	const e2eWarnThreshold = 5
	if (!r.TestE2E && !r.TestAll) || entries <= e2eWarnThreshold || r.yes {
		return nil
	}
	logging.Logger.Warn().
		Int("entries", entries).
		Int("threshold", e2eWarnThreshold).
		Msg("Running e2e tests on many entries — Playwright spawns a browser per test which can exhaust machine resources. Consider using --scenario-filter or --shortname-filter to reduce the set.")
	fmt.Fprintf(os.Stderr, "\nProceed with e2e tests on %d entries? [y/N] ", entries)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("aborted: e2e run with %d entries not confirmed (use --yes to skip this prompt)", entries)
	}
	return nil
}

func runMatrixEntries(ctx context.Context, entries []matrix.Entry, opts matrix.RunOptions) error {
	if !opts.DryRun && !opts.Coverage {
		output, _ := matrix.Print(entries, "table")
		fmt.Fprintln(os.Stdout, output)
	}
	var statusDisplay *matrix.StatusDisplay
	var logFile io.Closer
	stdoutIsTerminal := logging.IsTerminal(os.Stdout.Fd())
	if opts.LogDir != "" {
		opts.LogDir = filepath.Join(opts.LogDir, time.Now().Format("20060102-150405"))
	}
	if opts.LogDir == "" && stdoutIsTerminal && !opts.DryRun && !opts.Coverage {
		opts.LogDir = filepath.Join(os.TempDir(), "matrix-logs", time.Now().Format("20060102-150405"))
	}
	if opts.LogDir != "" && !opts.DryRun && !opts.Coverage {
		if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
			return fmt.Errorf("failed to create log directory %q: %w", opts.LogDir, err)
		}
		latestLink := filepath.Join(filepath.Dir(opts.LogDir), "latest")
		_ = os.Remove(latestLink)
		_ = os.Symlink(opts.LogDir, latestLink)
		f, err := os.Create(filepath.Join(opts.LogDir, "matrix-run.log"))
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}
		logFile = f
		if err := logging.Setup(logging.Options{
			LevelString: opts.LogLevel, ColorEnabled: false, Writer: f,
		}); err != nil {
			f.Close()
			return err
		}
		statusDisplay = matrix.NewStatusDisplay(os.Stdout, entries, stdoutIsTerminal, opts.LogDir)
		opts.OnEntryStart = statusDisplay.OnEntryStart
		opts.OnEntryComplete = statusDisplay.OnEntryComplete
		opts.OnPhaseChange = statusDisplay.OnPhaseChange
	}

	runStart := time.Now()
	results, err := matrix.Run(ctx, entries, opts)
	if logFile != nil {
		logFile.Close()
	}
	if !opts.DryRun && !opts.Coverage {
		if statusDisplay != nil {
			statusDisplay.Stop()
			statusDisplay.Clear()
			logging.ColorEnabled = stdoutIsTerminal
		}
		fmt.Fprintln(os.Stdout, matrix.PrintRunSummary(results, time.Since(runStart), opts.LogDir))
	}
	if err != nil {
		return err
	}
	if failed := countFailedResults(results); failed > 0 {
		return fmt.Errorf("matrix run: %d entr%s failed", failed, pluralEntry(failed))
	}
	return nil
}

// registerMatrixShortnameCompletion adds tab completion for the --shortname-filter flag.
// It generates the matrix from config files and offers unique shortnames, supporting
// comma-separated multi-select.
func registerMatrixShortnameCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("shortname-filter", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		repoRoot, _ := cmd.Flags().GetString("repo-root")
		repoRoot = resolveRepoRoot(repoRoot)
		if repoRoot == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --repo-root or configure repoRoot in your deployment config"), cobra.ShellCompDirectiveNoFileComp
		}

		entries, err := matrix.Generate(repoRoot, matrix.GenerateOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Collect unique shortnames in order of appearance.
		seen := make(map[string]bool)
		var shortnames []string
		for _, e := range entries {
			if e.Shortname != "" && !seen[e.Shortname] {
				seen[e.Shortname] = true
				shortnames = append(shortnames, e.Shortname)
			}
		}

		return completeMultiSelect(toComplete, shortnames)
	})
}

// registerMatrixVersionsCompletion adds tab completion for the --versions flag.
// It reads chart-versions.yaml and offers versions selected for routine automation.
func registerMatrixVersionsCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("versions", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		repoRoot, _ := cmd.Flags().GetString("repo-root")
		repoRoot = resolveRepoRoot(repoRoot)
		if repoRoot == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --repo-root or configure repoRoot in your deployment config"), cobra.ShellCompDirectiveNoFileComp
		}

		cv, err := matrix.LoadChartVersions(repoRoot)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return cv.ActiveVersions(), cobra.ShellCompDirectiveNoFileComp
	})
}

// registerMatrixFlowCompletion adds tab completion for the --flow-filter flag.
// It reads permitted-flows.yaml and offers the default flows list.
func registerMatrixFlowCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("flow-filter", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		repoRoot, _ := cmd.Flags().GetString("repo-root")
		repoRoot = resolveRepoRoot(repoRoot)
		if repoRoot == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --repo-root or configure repoRoot in your deployment config"), cobra.ShellCompDirectiveNoFileComp
		}

		pf, err := matrix.LoadPermittedFlows(repoRoot)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return pf.Defaults.Flows, cobra.ShellCompDirectiveNoFileComp
	})
}

// registerKubeContextCompletionForFlag adds tab completion for a named kube-context flag.
func registerKubeContextCompletionForFlag(cmd *cobra.Command, flagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		contexts, err := getKubeContexts()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return filterByPrefix(contexts, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
}

// resolveRepoRoot resolves the repository root from the flag, config file,
// or auto-detection via git.
func resolveRepoRoot(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if rc, err := config.LoadMatrixConfig(configFile); err == nil {
		if repoRoot := rc.ResolveInfra(true, "", "", config.InfraOverride{}).RepoRoot; repoRoot != "" {
			return repoRoot
		}
	}

	// Fall back to auto-detection from CWD (errors silently swallowed —
	// completion should never crash the shell).
	if detected, _ := config.DetectRepoRoot(); detected != "" {
		return detected
	}

	return ""
}

// matrixCLIInfra collects the infra flags set on the command line.
func matrixCLIInfra(flags *pflag.FlagSet) config.InfraOverride {
	text := func(name string) string {
		if !flags.Changed(name) {
			return ""
		}
		value, _ := flags.GetString(name)
		return value
	}
	toggle := func(name string) *bool {
		if !flags.Changed(name) {
			return nil
		}
		value, _ := flags.GetBool(name)
		return &value
	}
	scalars := func(text func(string) string) config.InfraConfig {
		return config.InfraConfig{
			Platform: text("platform"), RepoRoot: text("repo-root"), NamespacePrefix: text("namespace-prefix"), LogLevel: text("log-level"),
			EnvFile: text("env-file"), IngressBaseDomain: text("ingress-base-domain"), KubeContext: text("kube-context"),
			DockerUsername: text("docker-username"), DockerPassword: text("docker-password"),
			DockerHubUsername: text("dockerhub-username"), DockerHubPassword: text("dockerhub-password"),
			SkipDependencyUpdate: toggle("skip-dependency-update"), DeleteNamespace: toggle("delete-namespace"),
			EnsureDockerRegistry: toggle("ensure-docker-registry"), EnsureDockerHub: toggle("ensure-docker-hub"),
			UseVaultBackedSecrets: toggle("use-vault-backed-secrets"),
		}
	}
	cli := config.InfraOverride{InfraConfig: scalars(text), Given: scalars(func(name string) string {
		if !flags.Changed(name) {
			return ""
		}
		return name
	})}
	cli.KubeContexts = map[string]string{"gke": text("kube-context-gke"), "eks": text("kube-context-eks")}
	cli.IngressBaseDomains = map[string]string{"gke": text("ingress-base-domain-gke"), "eks": text("ingress-base-domain-eks")}
	cli.EnvFiles = map[string]string{"8.6": text("env-file-8.6"), "8.7": text("env-file-8.7"), "8.8": text("env-file-8.8"), "8.9": text("env-file-8.9")}
	cli.VaultBackedSecrets = map[string]bool{}
	for _, platform := range []string{"gke", "eks"} {
		if enabled := toggle("use-vault-backed-secrets-" + platform); enabled != nil {
			cli.VaultBackedSecrets[platform] = *enabled
		}
	}
	return cli
}

// warnDeprecatedInfra logs the root and matrix: infra keys the config file sets.
func warnDeprecatedInfra(rc *config.RootConfig) {
	if len(rc.DeprecatedInfra) > 0 {
		logging.Logger.Warn().Strs("keys", rc.DeprecatedInfra).Str("config", rc.FilePath).
			Msg("Root and matrix: platform, repoRoot, kubeContext, ingressBaseDomain and envFile are deprecated; set them in a deployments.<name> profile, the matrix: per-platform maps, or flags")
	}
}

// validateChartRefFlags rejects inconsistent --chart-ref / --chart-version
// combinations before any matrix entries run, so misconfiguration surfaces as
// a clear CLI error rather than a confusing helm failure.
//
// Rules:
//   - --chart-version requires --chart-ref (it has no meaning otherwise).
//   - --chart-ref must be either an OCI reference (oci://...) or a path to a
//     packaged chart (*.tgz). Bare directory paths are rejected because
//     deploy-camunda already supports local-directory installs via the normal
//     (non-overridden) chart path.
//   - When --chart-ref is an OCI reference, --chart-version is required —
//     otherwise helm would resolve to an arbitrary tag.
func validateChartRefFlags(chartRef, chartRefVersion string) error {
	if chartRef == "" {
		if chartRefVersion != "" {
			return fmt.Errorf("--chart-version requires --chart-ref")
		}
		return nil
	}

	isOCI := strings.HasPrefix(chartRef, "oci://")
	isTGZ := strings.HasSuffix(chartRef, ".tgz")
	if !isOCI && !isTGZ {
		return fmt.Errorf("--chart-ref must be an OCI reference (oci://...) or a packaged chart (.tgz), got %q", chartRef)
	}

	if isOCI && chartRefVersion == "" {
		return fmt.Errorf("--chart-version is required when --chart-ref is an OCI reference")
	}

	return nil
}

func validateIngressReadyFlags(waitIngressReady bool, timeoutMinutes int) error {
	if waitIngressReady && timeoutMinutes < 1 {
		return fmt.Errorf("--ingress-ready-timeout must be at least 1 minute with --wait-ingress-ready, got %d", timeoutMinutes)
	}
	return nil
}

func validateNamespaceOverride(namespaceOverride string, namespacePrepared, readOnly bool, githubActions string) error {
	if namespaceOverride == "" || namespacePrepared || readOnly || strings.EqualFold(githubActions, "true") {
		return nil
	}
	return fmt.Errorf("--namespace-override is only for an exact namespace that CI or another external process already provisioned; it disables automatic ExternalSecrets and expects required secrets, TLS, and pull secrets to exist. For a normal local run, replace --namespace-override NAME with --namespace-prefix NAME so deploy-camunda computes and provisions the namespace. If the exact namespace is already fully provisioned, add --namespace-prepared")
}

// validateChartRefVersionSpan rejects a --chart-ref override that would span
// more than one chart version. An external chart artifact (OCI ref or .tgz)
// corresponds to a single Camunda version, so applying it across multiple
// resolved matrix versions would install the wrong chart on every entry but the
// matching one. Multiple entries that share one version (scenarios/flows) are
// allowed — that is the normal RC-validation workflow.
func validateChartRefVersionSpan(chartRef string, entries []matrix.Entry) error {
	if chartRef == "" {
		return nil
	}
	seen := map[string]struct{}{}
	order := []string{}
	for _, e := range entries {
		if _, ok := seen[e.Version]; !ok {
			seen[e.Version] = struct{}{}
			order = append(order, e.Version)
		}
	}
	if len(order) > 1 {
		return fmt.Errorf(
			"--chart-ref applies a single external chart artifact, but the resolved matrix spans %d versions (%s); "+
				"narrow the run to one version with --versions (e.g. --versions %s)",
			len(order), strings.Join(order, ", "), order[0])
	}
	return nil
}

// countFailedResults returns the number of entries that finished with an error.
// Entries cancelled by --stop-on-failure are excluded so the count reflects
// real deployment failures rather than skipped work.
func countFailedResults(results []matrix.RunResult) int {
	failed := 0
	for _, r := range results {
		if r.Error == nil {
			continue
		}
		if r.Duration == 0 && strings.Contains(r.Error.Error(), "skipped") {
			continue
		}
		failed++
	}
	return failed
}

func pluralEntry(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

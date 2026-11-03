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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"scripts/camunda-core/pkg/ghactions"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
	"scripts/prepare-helm-values/pkg/env"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
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
	if err := runMatrixTopologies(ctx, topologyEntries, r.RunOptions); err != nil {
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

func runMatrixTopologies(ctx context.Context, entries []matrix.Entry, opts matrix.RunOptions) error {
	if len(entries) == 0 {
		return nil
	}
	if opts.DryRun || opts.Coverage {
		label := "dry-run"
		if !opts.DryRun && opts.Coverage {
			label = "coverage"
		}
		fmt.Fprintf(os.Stdout, "\n=== Topology entries (%s — listing only, no deploy) ===\n", label)
		for _, entry := range entries {
			fmt.Fprintf(os.Stdout, "%s/%s (%s): %d releases\n", entry.Version, entry.Scenario, entry.Shortname, len(entry.Topology.Releases))
		}
		return nil
	}
	opts.EnsureDockerRegistry = true
	for _, entry := range entries {
		if err := runTopologyEntry(ctx, entry, opts); err != nil {
			return fmt.Errorf("topology entry %s/%s failed: %w", entry.Version, entry.Scenario, err)
		}
	}
	return nil
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

// runTopologyEntry drives a topology-bearing matrix.Entry's multi-namespace
// fan-out: one matrix.Entry becomes N Helm releases (one per topology
// release) instead of matrix.Run's usual one-namespace-per-entry deploy.
//
// opts is the SAME fully-built matrix.RunOptions the normal (single-namespace)
// `matrix run` path constructs (see the matrix.Run(...) call below) — every
// per-release RunOptions is a copy of it with only Platform/NamespaceOverride
// overridden (see synthesizeReleaseOpts). Passing the whole struct through,
// rather than hand-picking a subset into a bespoke options type, is what
// prevents fields from silently dropping for topology deploys (this class of
// bug has already bitten IngressBaseDomains, HelmTimeout, and
// DeleteNamespaceFirst).
//
// Each release gets its OWN identity/persistence/features/dependencies layer
// selection (per matrix.TopologyRelease) rather than the scenario's uniform
// layers - e.g. the Hub release uses the bundled-Keycloak identity
// layer and deploys keycloak/postgresql/elasticsearch, while orchestration
// releases use the external-Keycloak layer and deploy no companions of their
// own. For each release, a matrix.Entry is synthesized carrying that
// release's layer selection + its own namespace, and
// matrix.BuildEntryFlags — the SAME flags/companion-chart/ingress-hostname/
// vault-mapping assembly a normal matrix entry gets — builds its
// RuntimeFlags, so nothing is hand-rolled or silently omitted.
//
// Every release's namespace and Keycloak realm are computed UP FRONT (via
// deploy.GenerateTopologyContexts — pure, no cluster access) since all
// namespaces are known before any release is deployed. The resulting
// cross-ref env (HUB_NAMESPACE, KEYCLOAK_REALM, EXTERNAL_ELASTICSEARCH_*) is
// injected into EVERY release's ExtraEnv, including the Hub release
// itself — its inherited placeholders (it has none today, but future layers
// might) resolve harmlessly since the values are self-consistent.
//
// Deploy order is Hub-first: its Keycloak/Identity/shared-storage
// companions must be reachable before orchestration releases render (their
// external-Keycloak layer points straight at the Hub namespace) or
// start.

// buildOrchestrationZeebeEnv derives ORCH_ZEEBE_GRPC/ORCH_ZEEBE_REST from the
// orchestration release's context: its Zeebe gateway Service is reachable
// cross-namespace at "<release>-zeebe-gateway.<namespace>.svc.cluster.local"
// (orchestration.serviceName in templates/orchestration/_helpers.tpl), on the
// gRPC (26500) and REST (8080) ports from
// orchestration.service.{grpcPort,httpPort}.
func buildOrchestrationZeebeEnv(orchestrationCtx *deploy.ScenarioContext) map[string]string {
	zeebeGatewayHost := fmt.Sprintf("%s-zeebe-gateway.%s.svc.cluster.local", orchestrationCtx.Release, orchestrationCtx.Namespace)
	return map[string]string{
		"ORCH_ZEEBE_GRPC": fmt.Sprintf("grpc://%s:26500", zeebeGatewayHost),
		"ORCH_ZEEBE_REST": fmt.Sprintf("http://%s:8080", zeebeGatewayHost),
	}
}

// applyTopologyCredentialsManifest points a topology release's deploy at the
// topology's credentials-manifest (repository-relative), rendered for the base
// namespace, so the deployer applies it instead of the CI-wide
// integration-test credentials. With no manifest declared the flags are left
// unchanged. The returned function removes the rendered file.
func applyTopologyCredentialsManifest(flags *config.RuntimeFlags, repoRoot, manifest, base string) (func(), error) {
	if manifest == "" {
		return func() {}, nil
	}
	path, remove, err := renderTopologyCredentialsManifest(filepath.Join(repoRoot, manifest), base)
	if err != nil {
		return func() {}, err
	}
	flags.Secrets.CredentialsManifest = path
	return remove, nil
}

// renderTopologyCredentialsManifest writes the topology's credentials-manifest,
// with the base namespace substituted, to a temporary file the deployer can
// apply, and returns a function that removes it.
func renderTopologyCredentialsManifest(path, base string) (string, func(), error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", func() {}, fmt.Errorf("read credentials manifest: %w", err)
	}
	rendered, err := matrix.RenderCredentialsManifest(content, base)
	if err != nil {
		return "", func() {}, err
	}
	f, err := os.CreateTemp("", "credentials-manifest-*.yaml")
	if err != nil {
		return "", func() {}, err
	}
	remove := func() { _ = os.Remove(f.Name()) }
	if _, err := f.Write(rendered); err != nil {
		f.Close()
		remove()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		remove()
		return "", func() {}, err
	}
	return f.Name(), remove, nil
}

// buildTopologyReleaseEnv layers a release's substitution namespace: shared
// cross-release variables first, then the release's own env, and finally the
// keys this driver derives from the topology declaration. The derived keys go
// last on purpose - they are the declaration's only copy, and a release env
// entry that shadowed one would deploy a context path or reader prefix that
// disagrees with optimize-context-path/serves and with the smoke matrix.
// matrix.Topology.Validate rejects such an entry outright; this ordering keeps
// the generated value authoritative even so.
func buildTopologyReleaseEnv(shared map[string]string, release matrix.TopologyRelease) map[string]string {
	env := make(map[string]string, len(shared)+len(release.Env)+4)
	maps.Copy(env, shared)
	if host := topologyReleaseServedHost(shared, release); host != "" {
		env["CAMUNDA_HOSTNAME"] = host
	}
	maps.Copy(env, release.Env)
	if release.Role == "orchestration" {
		token := matrix.TopologyEnvToken(release.NamespaceSuffix)
		env["ORCH_NAMESPACE"] = env[token+"_NAMESPACE"]
		env["ORCH_HOST"] = env[token+"_HOST"]
		env["ORCH_ZEEBE_GRPC"] = env[token+"_ZEEBE_GRPC"]
		env["ORCH_ZEEBE_REST"] = env[token+"_ZEEBE_REST"]
	}
	if release.Role == "optimize" {
		tenant := release.Tenant
		if tenant == "" {
			tenant = "default"
		}
		env["RELEASE_TENANT_ID"] = tenant
		// RELEASE_-prefixed: buildScenarioEnv seeds this namespace from the process
		// environment, where OPTIMIZE_CONTEXT_PATH is a name the Playwright suite reads
		// (pages/SM-8.10/NavigationPage.ts). Keep the two namespaces from sharing a key.
		env["RELEASE_OPTIMIZE_CONTEXT_PATH"] = release.OptimizeContextPath
		if release.Serves != "" {
			token := matrix.TopologyEnvToken(release.Serves)
			env["SERVED_NAMESPACE"] = env[token+"_NAMESPACE"]
			env["SERVED_HOST"] = env[token+"_HOST"]
			env["SERVED_ORCHESTRATION_INDEX_PREFIX"] = env[token+"_ORCHESTRATION_INDEX_PREFIX"]
		}
	}
	return env
}

// topologyReleaseServedHost returns the public host a release is served on:
// its own <TOKEN>_HOST for orchestration, HUB_HOST for hub and optimize.
func topologyReleaseServedHost(shared map[string]string, release matrix.TopologyRelease) string {
	if release.Role == "orchestration" {
		return shared[matrix.TopologyEnvToken(release.NamespaceSuffix)+"_HOST"]
	}
	return shared["HUB_HOST"]
}

// resolveSharedStorageServiceName resolves the Kubernetes Service name of the
// shared storage backend for a topology: it prefers the explicit
// topo.SharedStorageService, falling back to the ReleaseName of the resolved
// dependency matching topo.SharedStorage, or "" if neither is found.
func resolveSharedStorageServiceName(topo *matrix.Topology, resolvedDeps []matrix.ChartDependency) string {
	if topo.SharedStorageService != "" {
		return topo.SharedStorageService
	}
	for _, r := range resolvedDeps {
		if r.ReleaseName == topo.SharedStorage {
			return r.ReleaseName
		}
	}
	return ""
}

// extractHelmSetValue scans "key=value" helm-set pairs (the same shape
// matrix.RunOptions.ExtraHelmSets and parseHelmSetPairs use) for key and
// returns its value, or "" if key isn't present. The last matching entry
// wins, mirroring how repeated --extra-helm-set flags are applied.
func extractHelmSetValue(pairs []string, key string) string {
	value := ""
	for _, p := range pairs {
		idx := strings.Index(p, "=")
		if idx <= 0 {
			continue
		}
		if p[:idx] == key {
			value = p[idx+1:]
		}
	}
	return value
}

var prepareScenarioFn = deploy.PrepareScenario

var executePreparedFn = deploy.ExecutePrepared

var dependencyUpdateFn = helm.DependencyUpdate

// preparedTopologyRelease pairs a topology release with the flags and prepared scenario built for
// it, so the deploy loop and the topology-level post-deploy hook can both address it.
type preparedTopologyRelease struct {
	release   matrix.TopologyRelease
	flags     *config.RuntimeFlags
	namespace string
	prepared  *deploy.PreparedScenario
}

// topologyChartPaths returns the distinct local chart directories the topology's
// releases render from, in deploy order. A release pinning its own chart-version
// contributes a second directory, which is why this is not simply the entry's
// chart path. Releases rendered from an external chart reference (OCI or .tgz)
// have nothing to vendor locally and are skipped.
func topologyChartPaths(releases []preparedTopologyRelease) []string {
	seen := make(map[string]struct{}, len(releases))
	paths := make([]string, 0, len(releases))
	for _, release := range releases {
		if release.flags == nil || release.flags.Chart.Chart != "" || release.flags.Chart.ChartPath == "" {
			continue
		}
		chartPath := release.flags.Chart.ChartPath
		if _, ok := seen[chartPath]; ok {
			continue
		}
		seen[chartPath] = struct{}{}
		paths = append(paths, chartPath)
	}
	return paths
}

func runTopologyEntry(ctx context.Context, entry matrix.Entry, opts matrix.RunOptions) error {
	platform := entry.Platform
	if platform == "" {
		platform = opts.Platform
	}
	if platform == "" {
		platform = "gke"
	}

	if entry.Auth != "keycloak" || entry.Flow != "install" {
		return fmt.Errorf("topology entry %s/%s: multi-namespace topology currently supports only auth=keycloak and flow=install (got auth=%q, flow=%q)", entry.Version, entry.Scenario, entry.Auth, entry.Flow)
	}

	if opts.ChartRef != "" {
		return fmt.Errorf("topology entry %s/%s: --chart-ref is not supported on the multi-namespace topology path yet (applyChartRefOverride runs only in executeEntry); tracked in #6656", entry.Version, entry.Scenario)
	}
	if opts.Cleanup {
		return fmt.Errorf("topology entry %s/%s: --cleanup is not supported on the multi-namespace topology path yet (no per-release namespace teardown); tracked in #6656", entry.Version, entry.Scenario)
	}

	releases := make([]deploy.TopologyRelease, 0, len(entry.Topology.Releases))
	for _, r := range entry.Topology.Releases {
		releases = append(releases, deploy.TopologyRelease{
			Role:            r.Role,
			NamespaceSuffix: r.NamespaceSuffix,
			DependsOn:       r.DependsOn,
		})
	}

	// The "as if single-namespace" base namespace this entry would otherwise
	// get — every release's namespace is "<this>-<namespace-suffix>".
	baseOpts := opts
	baseOpts.Platform = platform
	baseNamespace := matrix.ResolveNamespace(baseOpts, entry)

	contexts, err := deploy.GenerateTopologyContexts(entry.Scenario, releases, &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{Namespace: baseNamespace},
	})
	if err != nil {
		return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}

	hubIdx := -1
	var orchestrationIndices []int
	var optimizeIndices []int
	for i, r := range entry.Topology.Releases {
		if r.Role == "hub" {
			hubIdx = i
		}
		if r.Role == "orchestration" {
			orchestrationIndices = append(orchestrationIndices, i)
		}
		if r.Role == "optimize" {
			optimizeIndices = append(optimizeIndices, i)
		}
	}
	if hubIdx == -1 {
		return fmt.Errorf("topology entry %s/%s: topology has no \"hub\" release", entry.Version, entry.Scenario)
	}

	sharedStorageService := resolveSharedStorageServiceName(entry.Topology, entry.Topology.Releases[hubIdx].ResolvedDependencies)

	crossRefEnv := deploy.BuildTopologyCrossRefEnv(contexts[hubIdx], sharedStorageService, "9200", "http")

	// ORCH_ZEEBE_GRPC/ORCH_ZEEBE_REST let the Hub release (Web
	// Modeler) register the orchestration release's Zeebe gateway as a
	// cluster, reaching it cross-namespace by FQDN — the same
	// "<release>-zeebe-gateway.<namespace>.svc.cluster.local" Service the
	// orchestration release itself exposes (orchestration.serviceName in
	// templates/orchestration/_helpers.tpl), on the gRPC (26500) and REST
	// (8080) ports from orchestration.service.{grpcPort,httpPort}.
	if err := addTopologyIngressHosts(crossRefEnv, opts, platform, contexts[hubIdx], entry.Topology.Releases, contexts); err != nil {
		return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}
	for _, i := range orchestrationIndices {
		token := matrix.TopologyEnvToken(entry.Topology.Releases[i].NamespaceSuffix)
		crossRefEnv[token+"_NAMESPACE"] = contexts[i].Namespace
		crossRefEnv[token+"_ORCHESTRATION_INDEX_PREFIX"] = contexts[i].OrchestrationIndexPrefix
		for key, value := range buildOrchestrationZeebeEnv(contexts[i]) {
			crossRefEnv[token+strings.TrimPrefix(key, "ORCH")] = value
		}
	}
	for _, i := range optimizeIndices {
		release := entry.Topology.Releases[i]
		token := matrix.TopologyEnvToken(release.NamespaceSuffix)
		crossRefEnv[token+"_NAMESPACE"] = contexts[i].Namespace
		crossRefEnv[token+"_OPTIMIZE_CONTEXT_PATH"] = release.OptimizeContextPath
	}
	if len(orchestrationIndices) == 1 {
		i := orchestrationIndices[0]
		crossRefEnv["ORCH_NAMESPACE"] = contexts[i].Namespace
		crossRefEnv["ORCH_ORCHESTRATION_INDEX_PREFIX"] = contexts[i].OrchestrationIndexPrefix
		maps.Copy(crossRefEnv, buildOrchestrationZeebeEnv(contexts[i]))
	}

	levels, err := topologyDeployLevels(entry.Topology.Releases)
	if err != nil {
		return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}
	levelOf := make([]int, len(entry.Topology.Releases))
	order := make([]int, 0, len(entry.Topology.Releases))
	for level, indices := range levels {
		for _, i := range indices {
			levelOf[i] = level
			order = append(order, i)
		}
	}

	preparedReleases := make([]preparedTopologyRelease, 0, len(order))
	preparedLevels := make([][]preparedTopologyRelease, len(levels))
	var releaseErrs []error
	defer func() {
		for _, release := range preparedReleases {
			release.prepared.Cleanup()
		}
	}()

	for _, i := range order {
		rel := entry.Topology.Releases[i]
		releaseCtx := contexts[i]

		releaseEntry := synthesizeReleaseEntry(opts.RepoRoot, entry, rel, platform)
		releaseOpts := synthesizeReleaseOpts(opts, platform, releaseCtx.Namespace)
		releaseHost := topologyReleaseHost(crossRefEnv, rel, len(orchestrationIndices))
		if releaseHost != "" {
			releaseOpts.ExtraHelmSets = append(releaseOpts.ExtraHelmSets, "global.host="+releaseHost)
		}

		flags, namespace, _, _, buildErr := matrix.BuildEntryFlags(releaseEntry, releaseOpts)
		if buildErr != nil {
			releaseErrs = append(releaseErrs, fmt.Errorf("topology release %s/%s (namespace-suffix %q): build flags: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, buildErr))
			continue
		}

		applyTopologyReleaseOverrides(flags, buildTopologyReleaseEnv(crossRefEnv, rel))
		applyTopologyReleaseHostname(flags, releaseHost)
		removeManifest, err := applyTopologyCredentialsManifest(flags, opts.RepoRoot, entry.Topology.CredentialsManifest, baseNamespace)
		if err != nil {
			releaseErrs = append(releaseErrs, fmt.Errorf("topology release %s/%s (namespace-suffix %q): %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err))
			continue
		}
		defer removeManifest()
		if err := matrix.RegisterDeclarativePostInfraHook(flags, releaseEntry.PostInfra, opts.RepoRoot, releaseEntry.Version, releaseEntry.Scenario); err != nil {
			releaseErrs = append(releaseErrs, fmt.Errorf("topology release %s/%s (namespace-suffix %q): register post-infra hook: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err))
			continue
		}
		if err := matrix.RegisterDeclarativePostDeployHook(flags, releaseEntry.PostDeploy, opts.RepoRoot, releaseEntry.Version, releaseEntry.Scenario); err != nil {
			releaseErrs = append(releaseErrs, fmt.Errorf("topology release %s/%s (namespace-suffix %q): register post-deploy hook: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err))
			continue
		}
		releaseCtx.IngressHost = flags.ResolveIngressHostname()
		prepared, prepareErr := prepareScenarioFn(ctx, releaseCtx, flags)
		if prepareErr != nil {
			releaseErrs = append(releaseErrs, fmt.Errorf("topology release %s/%s (namespace-suffix %q) prepare failed: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, prepareErr))
			continue
		}
		release := preparedTopologyRelease{release: rel, flags: flags, namespace: namespace, prepared: prepared}
		preparedReleases = append(preparedReleases, release)
		preparedLevels[levelOf[i]] = append(preparedLevels[levelOf[i]], release)
	}
	if len(releaseErrs) > 0 {
		return errors.Join(releaseErrs...)
	}

	for _, chartPath := range topologyChartPaths(preparedReleases) {
		if err := helm.EnsureDependencies(ctx, chartPath); err != nil {
			return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
		}
	}

	rendered := make([]matrix.RenderedTopologyRelease, 0, len(preparedReleases))
	for _, release := range preparedReleases {
		manifest, err := deploy.RenderPreparedTopologyContract(ctx, release.prepared, release.flags)
		if err != nil {
			return fmt.Errorf("topology release %s/%s contract render failed: %w", entry.Scenario, release.release.Role, err)
		}
		var contract matrix.TopologyContract
		// Charts older than 8.10 publish no contract template, so a release pinned to one
		// renders nothing here. Record the zero contract: ValidateRendered reads these only
		// to check optimize releases, and reports a mismatch if one actually needed this data.
		if len(bytes.TrimSpace(manifest)) > 0 {
			var document struct {
				Data map[string]string `yaml:"data"`
			}
			if err := yaml.Unmarshal(manifest, &document); err != nil {
				return fmt.Errorf("topology release %s/%s contract manifest decode failed: %w", entry.Scenario, release.release.Role, err)
			}
			if err := json.Unmarshal([]byte(document.Data["contract.json"]), &contract); err != nil {
				return fmt.Errorf("topology release %s/%s contract decode failed: %w", entry.Scenario, release.release.Role, err)
			}
		}
		rendered = append(rendered, matrix.RenderedTopologyRelease{Release: release.release, Contract: contract})
	}
	if err := entry.Topology.ValidateRendered(fmt.Sprintf("topology entry %s/%s", entry.Version, entry.Scenario), rendered); err != nil {
		return err
	}

	if err := updateTopologyChartDependencies(ctx, preparedReleases); err != nil {
		return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}
	if err := deployTopologyLevels(ctx, entry.Scenario, preparedLevels); err != nil {
		return err
	}

	if err := runTopologyPostDeployHook(ctx, entry, opts, preparedReleases); err != nil {
		return err
	}

	return runTopologyE2ELegs(ctx, entry, opts, platform, contexts, crossRefEnv, orchestrationIndices)
}

func updateTopologyChartDependencies(ctx context.Context, releases []preparedTopologyRelease) error {
	updated := make(map[string]bool, len(releases))
	for _, release := range releases {
		if release.flags == nil || release.flags.Chart.SkipDependencyUpdate || release.flags.Chart.Chart != "" || release.flags.Chart.ChartPath == "" {
			continue
		}
		chartPath := release.flags.Chart.ChartPath
		if !updated[chartPath] {
			if err := dependencyUpdateFn(ctx, chartPath); err != nil {
				return err
			}
			updated[chartPath] = true
		}
		release.flags.Chart.SkipDependencyUpdate = true
	}
	return nil
}

func deployTopologyLevels(ctx context.Context, scenario string, levels [][]preparedTopologyRelease) error {
	for _, level := range levels {
		levelErrs := make([]error, len(level))
		var wg sync.WaitGroup
		for i, release := range level {
			wg.Go(func() {
				deployErr := executePreparedFn(ctx, release.prepared, release.flags)

				status := "OK"
				if deployErr != nil {
					status = fmt.Sprintf("FAILED: %v", deployErr)
					levelErrs[i] = fmt.Errorf("topology release %s/%s (namespace-suffix %q) deploy failed: %w", scenario, release.release.Role, release.release.NamespaceSuffix, deployErr)
				}
				fmt.Fprintf(os.Stdout, "topology release %s/%s (namespace %s): %s\n", scenario, release.release.Role, release.namespace, status)
			})
		}
		wg.Wait()
		if err := errors.Join(levelErrs...); err != nil {
			return err
		}
	}
	return nil
}

// runTopologyPostDeployHook runs the scenario's post-deploy hook once, after every release in the
// topology has deployed. synthesizeReleaseEntry deliberately clears PostDeploy on each synthesized
// release, so without this the hook is registered against nothing and never runs: the lifecycle
// boundary of a topology hook is the whole topology, not any one release in it.
//
// It runs before, and independently of, the e2e legs. A topology that sets skip-e2e still has to
// prove its post-deploy assertions - for physicaltenants the hook is the only per-tenant Optimize
// coverage there is, because the browser suite cannot express it yet.
func runTopologyPostDeployHook(ctx context.Context, entry matrix.Entry, opts matrix.RunOptions, releases []preparedTopologyRelease) error {
	if entry.PostDeploy == nil {
		return nil
	}
	flags := topologyHookFlags(releases)
	if flags == nil {
		return fmt.Errorf("topology entry %s/%s: post-deploy hook declared but the topology prepared no release to run it against", entry.Version, entry.Scenario)
	}
	if err := matrix.RunDeclarativePostDeployHook(ctx, flags, entry.PostDeploy, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
		return fmt.Errorf("topology entry %s/%s: post-deploy hook: %w", entry.Version, entry.Scenario, err)
	}
	return nil
}

// topologyHookFlags picks the release a topology-level hook runs against. Hook scripts read
// TEST_NAMESPACE as the namespace to inspect workloads in, and they inspect the orchestration
// release (Zeebe partitions, exporters), so the orchestration release's flags are the ones to use;
// its ExtraEnv already carries the whole cross-reference env, so the Hub and per-Optimize values
// the scripts also read resolve from it. With more than one orchestration release the first in
// deploy order wins, matching ORCH_NAMESPACE, which is only set for a single-orchestration topology.
func topologyHookFlags(releases []preparedTopologyRelease) *config.RuntimeFlags {
	for _, release := range releases {
		if release.release.Role == "orchestration" {
			return release.flags
		}
	}
	if len(releases) > 0 {
		return releases[0].flags
	}
	return nil
}

// runTopologyE2ELegs runs e2e once the entire topology is deployed. Reaching this point means every
// release's helm install returned successfully, and helm --wait already gated each one on workload
// readiness, so the topology is up.
//
// Legs come from matrix.TopologyE2ELegs, the same computation that produces the CI smoke matrix, so
// a local run and a CI run agree on how many legs a topology has and which namespaces each targets.
// Legs run sequentially and every failure is collected: one tenant's failure must not hide another's
// result.
func runTopologyE2ELegs(
	ctx context.Context,
	entry matrix.Entry,
	opts matrix.RunOptions,
	platform string,
	contexts []*deploy.ScenarioContext,
	crossRefEnv map[string]string,
	orchestrationIndices []int,
) error {
	if !opts.TestE2E && !opts.TestAll {
		return nil
	}
	if entry.SkipE2E {
		fmt.Fprintf(os.Stdout, "topology %s: e2e skipped (skip-e2e)\n", entry.Scenario)
		return nil
	}

	legs := matrix.TopologyE2ELegs(entry.Version, entry.Topology)
	if len(legs) == 0 {
		return nil
	}

	// Namespaces are only knowable per release index, so map each release's suffix to its context.
	nsBySuffix := map[string]string{}
	relBySuffix := map[string]matrix.TopologyRelease{}
	hubNamespace := ""
	for i, rel := range entry.Topology.Releases {
		nsBySuffix[rel.NamespaceSuffix] = contexts[i].Namespace
		relBySuffix[rel.NamespaceSuffix] = rel
		if rel.Role == "hub" {
			hubNamespace = contexts[i].Namespace
		}
	}
	// Without the Hub namespace, run-e2e-tests.sh renders the env from the orchestration namespace
	// alone, so Web Modeler, Management Identity and Keycloak resolve to the orchestration host where
	// they do not exist and the setup project times out before any test runs.
	if hubNamespace == "" {
		return fmt.Errorf("topology %s: no hub release, so the e2e env cannot resolve Identity, Keycloak or Web Modeler", entry.Scenario)
	}

	var failures []string
	for _, leg := range legs {
		orchestrationNamespace := nsBySuffix[leg.OrchestrationSuffix]
		if orchestrationNamespace == "" {
			failures = append(failures, fmt.Sprintf("leg %q: no deployed namespace for that orchestration release", leg.OrchestrationSuffix))
			continue
		}
		optimizeNamespace := ""
		if leg.OptimizeSuffix != "" {
			optimizeNamespace = nsBySuffix[leg.OptimizeSuffix]
			if optimizeNamespace == "" {
				failures = append(failures, fmt.Sprintf("leg %q: no deployed namespace for optimize release %q", leg.OrchestrationSuffix, leg.OptimizeSuffix))
				continue
			}
		}

		rel := relBySuffix[leg.OrchestrationSuffix]
		releaseEntry := synthesizeReleaseEntry(opts.RepoRoot, entry, rel, platform)
		releaseOpts := synthesizeReleaseOpts(opts, platform, orchestrationNamespace)
		releaseHost := topologyReleaseHost(crossRefEnv, rel, len(orchestrationIndices))
		if releaseHost != "" {
			releaseOpts.ExtraHelmSets = append(releaseOpts.ExtraHelmSets, "global.host="+releaseHost)
		}

		flags, namespace, _, _, buildErr := matrix.BuildEntryFlags(releaseEntry, releaseOpts)
		if buildErr != nil {
			failures = append(failures, fmt.Sprintf("leg %q: build flags: %v", leg.OrchestrationSuffix, buildErr))
			continue
		}
		applyTopologyReleaseOverrides(flags, buildTopologyReleaseEnv(crossRefEnv, rel))
		applyTopologyReleaseHostname(flags, releaseHost)
		// synthesizeReleaseEntry disables e2e for the deploy loop; re-enable it for this leg only.
		flags.Test.RunE2ETests = true
		flags.Test.HubNamespace = hubNamespace
		flags.Test.OptimizeNamespace = optimizeNamespace
		flags.Test.OptimizeContextPath = leg.OptimizeContextPath
		flags.Test.ModelerClusterName = leg.ModelerClusterName
		flags.Test.PlaywrightProject = topologyPlaywrightProject(leg)
		flags.Test.PhysicalTenantID = leg.TenantID

		testErr := deploy.RunTests(ctx, flags, namespace)

		label := leg.OrchestrationSuffix
		if leg.OptimizeSuffix != "" {
			label = fmt.Sprintf("%s+%s", leg.OrchestrationSuffix, leg.OptimizeSuffix)
		}
		status := "OK"
		if testErr != nil {
			status = fmt.Sprintf("FAILED: %v", testErr)
			failures = append(failures, fmt.Sprintf("leg %q: %v", label, testErr))
		}
		fmt.Fprintf(os.Stdout, "topology e2e %s/%s (namespace %s): %s\n", entry.Scenario, label, namespace, status)
	}

	if len(failures) > 0 {
		return fmt.Errorf("topology %s: e2e failed for %d of %d legs:\n  - %s", entry.Scenario, len(failures), len(legs), strings.Join(failures, "\n  - "))
	}
	return nil
}

// topologyReleaseHost returns the ingress host the topology assigns to a
// release, or "" when the release keeps BuildEntryFlags' default host.
func topologyReleaseHost(crossRefEnv map[string]string, release matrix.TopologyRelease, orchestrationCount int) string {
	key := topologyReleaseHostKey(release.Role, release.NamespaceSuffix, orchestrationCount)
	if key == "" {
		return ""
	}
	return crossRefEnv[key]
}

// applyTopologyReleaseHostname makes a topology-assigned host the release's
// CAMUNDA_HOSTNAME as well as its global.host. Scenario values files take the
// ingress host from $CAMUNDA_HOSTNAME, and charts before 8.10 read it from
// global.ingress.host, which the global.host override does not reach (8.9
// prefers global.ingress.host; 8.8 and 8.7 read nothing else). Without this,
// every pre-8.10 orchestration release in a multi-orchestration topology kept
// the one CI-wide hostname, so they all published the same external-dns name
// instead of the per-release host the Hub inventory and e2e legs use.
// flags.ExtraEnv outranks the scenario-derived CAMUNDA_HOSTNAME when values
// files are rendered, so setting it here is enough.
func applyTopologyReleaseHostname(flags *config.RuntimeFlags, host string) {
	if host == "" {
		return
	}
	if flags.ExtraEnv == nil {
		flags.ExtraEnv = map[string]string{}
	}
	flags.ExtraEnv["CAMUNDA_HOSTNAME"] = host
}

func topologyPlaywrightProject(leg matrix.TopologyE2ELeg) string {
	if leg.AdditionalPlaywrightProject != "" {
		return leg.AdditionalPlaywrightProject
	}
	return leg.PlaywrightProject
}

// topologyReleaseHostKey names the crossRefEnv key whose value must be pushed
// into a release's global.host, or "" to leave global.host to BuildEntryFlags'
// namespace-derived default.
//
// An "optimize" release is always pinned to the Hub host: its values layer
// registers an OIDC redirect URL under HUB_HOST, so a namespace-derived host
// would break the callback. Other roles keep the pre-existing behavior of
// overriding only when several orchestration releases must be split across
// distinct hosts.
func topologyReleaseHostKey(role, namespaceSuffix string, orchestrationCount int) string {
	if role == "optimize" {
		return "HUB_HOST"
	}
	if orchestrationCount <= 1 {
		return ""
	}
	if role == "orchestration" {
		return matrix.TopologyEnvToken(namespaceSuffix) + "_HOST"
	}
	return "HUB_HOST"
}

func addTopologyIngressHosts(crossRefEnv map[string]string, opts matrix.RunOptions, platform string, hubCtx *deploy.ScenarioContext, releases []matrix.TopologyRelease, contexts []*deploy.ScenarioContext) error {
	orchestrationCount := 0
	for _, release := range releases {
		if release.Role == "orchestration" {
			orchestrationCount++
		}
	}
	sharedHost := extractHelmSetValue(opts.ExtraHelmSets, "global.host")
	if sharedHost != "" && orchestrationCount <= 1 {
		crossRefEnv["HUB_HOST"] = sharedHost
		for _, release := range releases {
			if release.Role == "orchestration" {
				crossRefEnv[matrix.TopologyEnvToken(release.NamespaceSuffix)+"_HOST"] = sharedHost
				crossRefEnv["ORCH_HOST"] = sharedHost
			}
		}
		return nil
	}

	baseDomain := matrix.EntryInfra(opts, platform, "").IngressBaseDomain
	if baseDomain == "" {
		if sharedHost != "" {
			return fmt.Errorf("global.host=%s is shared, but this topology has %d orchestration releases that each need their own host; set an ingress base domain instead", sharedHost, orchestrationCount)
		}
		return nil
	}
	crossRefEnv["HUB_HOST"] = (&config.IngressFlags{
		IngressSubdomain:  hubCtx.Namespace,
		IngressBaseDomain: baseDomain,
	}).ResolveIngressHostname()
	for i, release := range releases {
		if release.Role != "orchestration" {
			continue
		}
		host := (&config.IngressFlags{
			IngressSubdomain:  contexts[i].Namespace,
			IngressBaseDomain: baseDomain,
		}).ResolveIngressHostname()
		crossRefEnv[matrix.TopologyEnvToken(release.NamespaceSuffix)+"_HOST"] = host
		if orchestrationCount == 1 {
			crossRefEnv["ORCH_HOST"] = host
		}
	}
	return nil
}

func topologyDeployLevels(releases []matrix.TopologyRelease) ([][]int, error) {
	roleIdx := make(map[string]int, len(releases))
	suffixIdx := make(map[string]int, len(releases))
	for i, r := range releases {
		if _, seen := roleIdx[r.Role]; r.Role != "" && !seen {
			roleIdx[r.Role] = i
		}
		suffixIdx[r.NamespaceSuffix] = i
	}
	deps := make([][]int, len(releases))
	for i, r := range releases {
		if r.DependsOn != "" {
			dep, ok := roleIdx[r.DependsOn]
			if !ok {
				return nil, fmt.Errorf("release[%d] (role %q) depends-on %q which is not a declared role", i, r.Role, r.DependsOn)
			}
			deps[i] = append(deps[i], dep)
		}
		if r.Serves != "" {
			dep, ok := suffixIdx[r.Serves]
			if !ok {
				return nil, fmt.Errorf("release[%d] (role %q) serves %q which is not a declared namespace-suffix", i, r.Role, r.Serves)
			}
			deps[i] = append(deps[i], dep)
		}
	}
	deployed := make([]bool, len(releases))
	var levels [][]int
	for remaining := len(releases); remaining > 0; {
		var level []int
		for i := range releases {
			if deployed[i] {
				continue
			}
			ready := true
			for _, dep := range deps[i] {
				ready = ready && deployed[dep]
			}
			if ready {
				level = append(level, i)
			}
		}
		if len(level) == 0 {
			return nil, fmt.Errorf("topology depends-on graph has a cycle or unresolvable dependency")
		}
		for _, i := range level {
			deployed[i] = true
		}
		remaining -= len(level)
		levels = append(levels, level)
	}
	return levels, nil
}

// synthesizeReleaseEntry builds the matrix.Entry for one topology release,
// carrying THAT release's own identity/persistence/features/dependencies
// layer selection instead of the scenario-level (uniform) ones — the core of
// the per-release layer fix. Extracted as a pure function for testability.
func synthesizeReleaseEntry(repoRoot string, entry matrix.Entry, rel matrix.TopologyRelease, platform string) matrix.Entry {
	// Feature layers go through the same env-var substitution pipeline as the
	// identity and persistence layers (scenarios.BuildDeploymentConfig →
	// values.Process), so a release's ${...} placeholders resolve before Helm
	// sees them.
	features := append([]string(nil), rel.Features...)

	// A release may pin its own chart-version, so a topology can mix chart
	// versions (e.g. an 8.10 Hub serving an 8.9 orchestration release). Empty
	// inherits the parent matrix entry's version, which is the uniform case.
	version := rel.ChartVersion
	if version == "" {
		version = entry.Version
	}

	releaseEntry := matrix.Entry{
		Version:      version,
		ChartPath:    filepath.Join(repoRoot, "charts", "camunda-platform-"+version),
		Scenario:     entry.Scenario,
		Shortname:    entry.Shortname,
		Auth:         entry.Auth,
		Flow:         "install",
		Platform:     platform,
		InfraType:    entry.InfraType,
		Tier:         entry.Tier,
		Identity:     rel.Identity,
		Persistence:  rel.Persistence,
		Features:     features,
		Dependencies: rel.ResolvedDependencies,
	}
	// The scenario's post-infra hook provisions the shared infrastructure the whole
	// topology then deploys onto, so it belongs to the Hub release: that is the one
	// release every other one depends on, and running it per release would repeat
	// the provisioning once per namespace. PostDeploy is deliberately NOT carried
	// here - runTopologyPostDeployHook runs it once after the whole topology is up.
	if rel.Role == "hub" {
		releaseEntry.PostInfra = entry.PostInfra
	}
	// e2e is a topology-level concern, not a per-release one: a release's deploy returns while later
	// releases are still undeployed, so testing here would test a partial topology (and would repeat
	// for every orchestration release). runTopologyEntry runs the legs once the whole topology is up.
	releaseEntry.SkipE2E = true
	return releaseEntry
}

// synthesizeReleaseOpts builds the matrix.RunOptions for one topology
// release: identical to the base topology entry's options, except
// NamespaceOverride pins this release to its own precomputed namespace.
// Extracted as a pure function for testability.
// synthesizeReleaseOpts returns a copy of base (the same fully-built
// matrix.RunOptions passed into runTopologyEntry) with ONLY Platform and
// NamespaceOverride overridden for one topology release. Every other field
// — DeleteNamespaceFirst, EnvFile/EnvFiles, VaultBackedSecrets/
// UseVaultBackedSecrets, ExtraValues, ExtraHelmArgs/ExtraHelmSets,
// SkipDependencyUpdate, KeycloakHost/KeycloakProtocol, UseLatest, UseQA,
// LogDir, HelmTimeout, IngressBaseDomains/IngressBaseDomain, etc. — passes
// through unchanged. RunOptions is passed and returned by value; its map
// fields (KubeContexts, IngressBaseDomains, EnvFiles, VaultBackedSecrets)
// are shared with base, which is safe because callers only ever READ them
// (via BuildEntryFlags) — never mutate a map on the returned copy.
func synthesizeReleaseOpts(base matrix.RunOptions, platform string, namespaceOverride string) matrix.RunOptions {
	cp := base
	cp.Platform = platform
	cp.NamespaceOverride = namespaceOverride
	return cp
}

// applyTopologyReleaseOverrides applies the topology-specific adjustments
// every release's flags need on top of matrix.BuildEntryFlags' normal
// assembly, mutating flags in place:
//
//   - ExtraEnv: the cross-ref env (HUB_NAMESPACE, KEYCLOAK_REALM,
//     EXTERNAL_ELASTICSEARCH_*, HUB_HOST, ORCH_HOST, ORCH_ZEEBE_GRPC,
//     ORCH_ZEEBE_REST) computed up front, MERGED into EVERY
//     release (Hub included) before render/preflight, on top of
//     any pre-existing ExtraEnv (cross-ref wins on key conflict). The
//     topology path currently bypasses executeEntry (where per-entry
//     client-IDs like VENOM_CLIENT_ID/CONNECTORS_CLIENT_ID and
//     Entra/Auth0 vars are injected) and is guarded to auth=keycloak/
//     flow=install; replacing the map wholesale would silently drop any
//     such per-entry ExtraEnv if that guard is ever relaxed or if
//     BuildEntryFlags starts populating ExtraEnv. Merging is
//     future-proof and functionally identical today.
//
//   - Secrets.ExternalSecrets: forced to true for every release.
//     BuildEntryFlags sets ExternalSecrets = (NamespaceOverride == ""),
//     which is a normal-path optimization: per-scenario CI workflows that
//     pre-create the namespace via --namespace-override already ran
//     .github/actions/cluster-setup-secrets, so re-applying external
//     secrets would be redundant (and can fail on EKS RBAC, see
//     BuildEntryFlags' comment). The topology path ALWAYS sets
//     NamespaceOverride (one per release namespace) but has NO equivalent
//     cluster-setup-secrets action run for it — so the topology driver
//     itself must own provisioning integration-test-credentials (and other
//     external secrets) into every release namespace from the same Vault
//     path, or dependent companions like the Hub release's bundled
//     Keycloak CreateContainerConfigError on the missing secret.
//     NOTE for Stage 3 (CI wiring): when a topology scenario is wired into
//     CI, the workflow must NOT also invoke cluster-setup-secrets for these
//     namespaces — this driver already provisions them, and doing both
//     would double-provision (and reintroduces the EKS RBAC risk
//     BuildEntryFlags' comment warns about).
func applyTopologyReleaseOverrides(flags *config.RuntimeFlags, crossRefEnv map[string]string) {
	if flags.ExtraEnv == nil {
		flags.ExtraEnv = make(map[string]string, len(crossRefEnv))
	}
	maps.Copy(flags.ExtraEnv, crossRefEnv)

	flags.Secrets.ExternalSecrets = true

	// The topology driver provisions the registry-camunda-cloud pull secret into each
	// release namespace (creds resolve from HARBOR_USERNAME/TEST_DOCKER_USERNAME_CAMUNDA_CLOUD
	// env via the deployer fallback), so the deploy is self-sufficient locally and in CI
	// without relying on node image cache.
	flags.Docker.EnsureDockerRegistry = true
}

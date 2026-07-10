package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagGroupAnnotation is the pflag annotation key used to bucket flags into
// help-output groups. Kept namespaced so we don't collide with third-party
// annotations Cobra may attach in the future.
const flagGroupAnnotation = "deploy-camunda:group"

// Group labels rendered as section headings in --help output. Order below
// controls display order.
const (
	grpInfrastructure = "Infrastructure"
	grpAuth           = "Authentication"
	grpScenario       = "Scenario"
	grpRegistry       = "Registry"
	grpDeployment     = "Deployment behaviour"
	grpLogging        = "Logging"
	grpOther          = "Other"
)

var groupOrder = []string{
	grpInfrastructure, grpAuth, grpScenario, grpRegistry,
	grpDeployment, grpLogging, grpOther,
}

// annotateFlagGroups tags each named flag with the given group. Any local flag
// on the command that isn't listed in groups is silently bucketed into "Other"
// so the help output never drops a flag. Unknown flag names in groups are
// ignored — that lets the caller reference deprecated flags without breaking
// when they're removed.
func annotateFlagGroups(cmd *cobra.Command, groups map[string][]string) {
	for group, names := range groups {
		for _, n := range names {
			f := cmd.Flags().Lookup(n)
			if f == nil {
				continue
			}
			if f.Annotations == nil {
				f.Annotations = map[string][]string{}
			}
			f.Annotations[flagGroupAnnotation] = []string{group}
		}
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if _, ok := f.Annotations[flagGroupAnnotation]; ok {
			return
		}
		if f.Annotations == nil {
			f.Annotations = map[string][]string{}
		}
		f.Annotations[flagGroupAnnotation] = []string{grpOther}
	})
	cmd.SetUsageFunc(groupedUsageFunc)
}

// groupedUsageFunc renders the command usage with flags bucketed into the
// groups declared via annotateFlagGroups. Falls back to a plain flag dump for
// commands where no grouping was applied.
func groupedUsageFunc(cmd *cobra.Command) error {
	w := cmd.OutOrStderr()

	fmt.Fprintf(w, "Usage:\n  %s", cmd.UseLine())
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(w, "\n  %s [command]", cmd.CommandPath())
	}
	fmt.Fprintln(w)

	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(w, "\nAliases:\n  %s\n", cmd.NameAndAliases())
	}
	if cmd.HasExample() {
		fmt.Fprintf(w, "\nExamples:\n%s\n", cmd.Example)
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintln(w, "\nAvailable Commands:")
		maxLen := 0
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() || sub.IsAdditionalHelpTopicCommand() {
				continue
			}
			if l := len(sub.Name()); l > maxLen {
				maxLen = l
			}
		}
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() || sub.IsAdditionalHelpTopicCommand() {
				continue
			}
			fmt.Fprintf(w, "  %-*s  %s\n", maxLen, sub.Name(), sub.Short)
		}
	}

	printGroupedFlags(w, cmd)

	if cmd.HasAvailableInheritedFlags() {
		fmt.Fprintln(w, "\nGlobal Flags:")
		fmt.Fprint(w, cmd.InheritedFlags().FlagUsagesWrapped(120))
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(w, "\nUse \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	}
	return nil
}

// rootFlagGroups is the flag → group mapping applied to the root
// `deploy-camunda` command. Unknown or removed flag names are silently
// tolerated by annotateFlagGroups.
func rootFlagGroups() map[string][]string {
	return map[string][]string{
		grpInfrastructure: {
			"namespace", "namespace-prefix", "platform", "repo-root",
			"kube-context", "ingress-subdomain", "ingress-base-domain",
			"ingress-hostname",
		},
		grpAuth: {
			"auth", "keycloak-host", "keycloak-protocol", "keycloak-realm",
			"use-vault-backed-secrets",
		},
		grpScenario: {
			"chart-path", "chart", "version", "scenario", "scenario-path",
			"extra-values", "values-preset", "identity", "persistence",
			"test-platform", "features", "qa", "image-tags",
		},
		grpRegistry: {
			"docker-username", "docker-password", "ensure-docker-registry",
			"dockerhub-username", "dockerhub-password", "ensure-docker-hub",
		},
		grpDeployment: {
			"release", "flow", "ttl", "skip-preflight",
			"skip-dependency-update", "delete-namespace", "render-templates",
			"render-output-dir", "timeout", "test-e2e", "test-all",
			"upgrade-flow",
		},
		grpLogging: {
			"log-level",
		},
	}
}

// matrixRunFlagGroups is the flag → group mapping applied to
// `deploy-camunda matrix run`. Per-platform variants sit alongside their base
// flag in the same group so users see them together.
func matrixRunFlagGroups() map[string][]string {
	return map[string][]string{
		grpInfrastructure: {
			"platform", "repo-root", "namespace-prefix",
			"kube-context", "kube-context-gke", "kube-context-eks",
			"ingress-base-domain", "ingress-base-domain-gke", "ingress-base-domain-eks",
			"namespace-override",
		},
		grpAuth: {
			"use-vault-backed-secrets", "use-vault-backed-secrets-gke", "use-vault-backed-secrets-eks",
			"keycloak-host", "keycloak-protocol",
		},
		grpScenario: {
			"versions", "include-disabled", "scenario-filter",
			"shortname-filter", "shortname-exact", "flow-filter",
			"upgrade-from-version", "use-latest", "use-qa",
			"extra-helm-arg", "extra-helm-set", "extra-values",
			"chart-ref", "chart-version", "tier",
		},
		grpRegistry: {
			"docker-username", "docker-password", "ensure-docker-registry",
			"dockerhub-username", "dockerhub-password", "ensure-docker-hub",
		},
		grpDeployment: {
			"dry-run", "coverage", "test-e2e", "test-all",
			"stop-on-failure", "cleanup", "delete-namespace",
			"max-parallel", "skip-dependency-update", "timeout",
		},
		grpLogging: {
			"log-level", "log-dir",
		},
	}
}

// printGroupedFlags writes local flags to w, one section per group in
// groupOrder. Empty groups are skipped so a command that only touches a few
// groups doesn't get empty headers.
func printGroupedFlags(w io.Writer, cmd *cobra.Command) {
	byGroup := map[string][]*pflag.Flag{}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		g := grpOther
		if v, ok := f.Annotations[flagGroupAnnotation]; ok && len(v) > 0 {
			g = v[0]
		}
		byGroup[g] = append(byGroup[g], f)
	})

	for _, g := range groupOrder {
		fs := byGroup[g]
		if len(fs) == 0 {
			continue
		}
		sort.Slice(fs, func(i, j int) bool { return fs[i].Name < fs[j].Name })
		tmp := pflag.NewFlagSet(g, pflag.ContinueOnError)
		for _, f := range fs {
			tmp.AddFlag(f)
		}
		fmt.Fprintf(w, "\n%s Flags:\n", g)
		fmt.Fprint(w, tmp.FlagUsagesWrapped(120))
	}
}

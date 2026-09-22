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
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"scripts/camunda-core/pkg/kube"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
)

// persistedTTL is stamped onto a long-lived topology's namespaces in place of
// the deploy's ephemeral TTL. The cluster cleaner and janitor both read their
// own annotation, and camunda.cloud/ephemeral is flipped to "false" so a sweep
// keyed on that label leaves the namespaces alone too.
const persistedTTL = "8760h"

// topologyRelease is one resolved release of a scenario's topology: what the
// registry declared, paired with the namespace a deploy of that scenario into
// baseNamespace derives for it.
type topologyRelease struct {
	Role      string
	Suffix    string
	Namespace string
}

// resolveTopologyReleases reads the scenario's topology out of the chart
// version's CI registry and derives each release's namespace from
// baseNamespace, applying the same truncation the deploy path applies.
func resolveTopologyReleases(repoRoot, version, scenario, baseNamespace string) ([]topologyRelease, error) {
	chartDir := filepath.Join(repoRoot, "charts", fmt.Sprintf("camunda-platform-%s", version))
	cfg, err := matrix.LoadRegistry(chartDir)
	if err != nil {
		return nil, fmt.Errorf("load CI registry for version %s: %w", version, err)
	}

	for _, s := range cfg.Integration.Case.PR.Scenarios {
		if s.Name != scenario {
			continue
		}
		if s.Topology == nil {
			return nil, fmt.Errorf("scenario %q in version %s declares no topology", scenario, version)
		}
		releases := make([]topologyRelease, 0, len(s.Topology.Releases))
		for _, r := range s.Topology.Releases {
			namespace, err := deploy.DeriveReleaseNamespace(baseNamespace, r.NamespaceSuffix)
			if err != nil {
				return nil, err
			}
			releases = append(releases, topologyRelease{Role: r.Role, Suffix: r.NamespaceSuffix, Namespace: namespace})
		}
		return releases, nil
	}

	return nil, fmt.Errorf("scenario %q not found in version %s registry", scenario, version)
}

// teardownOrder returns releases with every non-hub release before the hub, so
// a teardown removes dependents before the Management Identity they registered
// against. Declaration order is preserved within each group.
func teardownOrder(releases []topologyRelease) []topologyRelease {
	ordered := make([]topologyRelease, 0, len(releases))
	for _, r := range releases {
		if r.Role != "hub" {
			ordered = append(ordered, r)
		}
	}
	for _, r := range releases {
		if r.Role == "hub" {
			ordered = append(ordered, r)
		}
	}
	return ordered
}

// podReadiness counts pods whose every container is ready, and names those that
// are not, capped so one broken namespace cannot flood a status report.
func podReadiness(pods []corev1.Pod) (ready int, notReady []string) {
	for _, pod := range pods {
		allReady := len(pod.Status.ContainerStatuses) > 0
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			ready++
			continue
		}
		if len(notReady) < 5 {
			notReady = append(notReady, pod.Name)
		}
	}
	sort.Strings(notReady)
	return ready, notReady
}

type topologyLifecycleFlags struct {
	repoRoot    string
	version     string
	scenario    string
	base        string
	kubeContext string
}

func (f *topologyLifecycleFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.repoRoot, "repo-root", ".", "repository root")
	cmd.Flags().StringVar(&f.version, "version", "", "chart version, e.g. 8.10")
	cmd.Flags().StringVar(&f.scenario, "scenario", "", "registry scenario declaring the topology")
	cmd.Flags().StringVar(&f.base, "base", "", "base namespace; each release lands in <base>-<suffix>")
	cmd.Flags().StringVar(&f.kubeContext, "kube-context", "", "kubectl context (defaults to current)")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("scenario")
	_ = cmd.MarkFlagRequired("base")
}

func (f *topologyLifecycleFlags) releases() ([]topologyRelease, error) {
	return resolveTopologyReleases(f.repoRoot, f.version, f.scenario, f.base)
}

func newTopologyNamespacesCommand() *cobra.Command {
	var f topologyLifecycleFlags

	cmd := &cobra.Command{
		Use:   "namespaces",
		Short: "Print every namespace a topology scenario deploys into, in deploy order",
		RunE: func(cmd *cobra.Command, args []string) error {
			releases, err := f.releases()
			if err != nil {
				return err
			}
			for _, r := range releases {
				fmt.Fprintln(cmd.OutOrStdout(), r.Namespace)
			}
			return nil
		},
	}

	f.bind(cmd)
	return cmd
}

func newTopologyPersistCommand() *cobra.Command {
	var (
		f   topologyLifecycleFlags
		ttl string
	)

	cmd := &cobra.Command{
		Use:   "persist",
		Short: "Exempt a topology's namespaces from the cluster cleaner so the environment outlives a CI TTL",
		RunE: func(cmd *cobra.Command, args []string) error {
			releases, err := f.releases()
			if err != nil {
				return err
			}
			client, err := kube.NewClient("", f.kubeContext)
			if err != nil {
				return err
			}
			annotations := map[string]string{
				"cleaner/ttl":             ttl,
				"janitor/ttl":             ttl,
				"camunda.cloud/ephemeral": "false",
			}
			ctx := cmd.Context()
			for _, r := range releases {
				if err := client.SetLabelsAndAnnotations(ctx, r.Namespace, nil, annotations); err != nil {
					return fmt.Errorf("persist namespace %s: %w", r.Namespace, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "persisted %s (ttl %s)\n", r.Namespace, ttl)
			}
			return nil
		},
	}

	f.bind(cmd)
	cmd.Flags().StringVar(&ttl, "ttl", persistedTTL, "TTL stamped on cleaner/janitor annotations")
	return cmd
}

func newTopologyStatusCommand() *cobra.Command {
	var (
		f             topologyLifecycleFlags
		ingressHost   string
		failOnUnready bool
		failOnMissing bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report namespace presence and pod readiness for every release in a topology",
		RunE: func(cmd *cobra.Command, args []string) error {
			releases, err := f.releases()
			if err != nil {
				return err
			}
			client, err := kube.NewClient("", f.kubeContext)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			missing, unready := writeTopologyStatus(ctx, cmd.OutOrStdout(), client, releases, ingressHost)
			if failOnMissing && missing > 0 {
				return fmt.Errorf("%d of %d release namespaces do not exist; deploy the environment before upgrading it", missing, len(releases))
			}
			if failOnUnready && (missing > 0 || unready > 0) {
				return fmt.Errorf("%d of %d releases are missing or not fully ready", missing+unready, len(releases))
			}
			return nil
		},
	}

	f.bind(cmd)
	cmd.Flags().StringVar(&ingressHost, "ingress-base-domain", "", "base domain used to print each release's URL")
	cmd.Flags().BoolVar(&failOnUnready, "fail-on-not-ready", false, "exit non-zero when any release namespace is missing or has an unready pod")
	cmd.Flags().BoolVar(&failOnMissing, "fail-on-missing", false, "exit non-zero only when a release namespace does not exist")
	return cmd
}

// writeTopologyStatus prints one Markdown table row per release and returns how
// many release namespaces are absent or unreadable, and how many present ones
// have an unready or absent pod.
func writeTopologyStatus(ctx context.Context, out io.Writer, client *kube.Client, releases []topologyRelease, ingressBaseDomain string) (missing, unready int) {
	fmt.Fprintln(out, "| Role | Namespace | Pods ready | Not ready | URL |")
	fmt.Fprintln(out, "|---|---|---|---|---|")

	for _, r := range releases {
		url := ""
		if ingressBaseDomain != "" {
			url = fmt.Sprintf("https://%s.%s", r.Namespace, ingressBaseDomain)
		}
		pods, err := client.ListPods(ctx, r.Namespace)
		if err != nil {
			state := "error"
			if errors.IsNotFound(err) {
				state = "absent"
			}
			fmt.Fprintf(out, "| %s | `%s` | %s | — | %s |\n", r.Role, r.Namespace, state, url)
			missing++
			continue
		}
		ready, notReady := podReadiness(pods.Items)
		if len(notReady) > 0 || len(pods.Items) == 0 {
			unready++
		}
		notReadyCell := "—"
		if len(notReady) > 0 {
			notReadyCell = "`" + strings.Join(notReady, "`, `") + "`"
		}
		fmt.Fprintf(out, "| %s | `%s` | %d/%d | %s | %s |\n", r.Role, r.Namespace, ready, len(pods.Items), notReadyCell, url)
	}
	return missing, unready
}

func newTopologyUninstallCommand() *cobra.Command {
	var (
		f       topologyLifecycleFlags
		confirm string
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Delete every namespace of a topology, dependents before the hub",
		RunE: func(cmd *cobra.Command, args []string) error {
			if confirm != f.base {
				return fmt.Errorf("--confirm must repeat the base namespace %q to delete it, got %q", f.base, confirm)
			}
			releases, err := f.releases()
			if err != nil {
				return err
			}
			client, err := kube.NewClient("", f.kubeContext)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			for _, r := range teardownOrder(releases) {
				if err := client.DeleteNamespace(ctx, r.Namespace); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("delete namespace %s: %w", r.Namespace, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", r.Namespace)
			}
			return nil
		},
	}

	f.bind(cmd)
	cmd.Flags().StringVar(&confirm, "confirm", "", "must repeat --base exactly")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Minute, "overall deletion timeout")
	_ = cmd.MarkFlagRequired("confirm")
	return cmd
}

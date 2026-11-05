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

package matrix

import (
	"context"
	"fmt"
	"os"
	"scripts/deploy-camunda/deploy"
	"strings"
)

// runTopologyE2ELegs runs e2e once the entire topology is deployed. Reaching this point means every
// release's helm install returned successfully, and helm --wait already gated each one on workload
// readiness, so the topology is up.
//
// Legs come from matrix.TopologyE2ELegs, the same computation that produces the CI smoke matrix, so
// a local run and a CI run agree on how many legs a topology has and which namespaces each targets.
// Legs run sequentially and every failure is collected: one tenant's failure must not hide another's
// result.
func (t *topologyRun) runTopologyE2ELegs(ctx context.Context) error {
	entry, opts, contexts := t.entry, t.opts, t.contexts
	if !opts.TestE2E && !opts.TestAll {
		return nil
	}
	if entry.SkipE2E {
		fmt.Fprintf(os.Stdout, "topology %s: e2e skipped (skip-e2e)\n", entry.Scenario)
		return nil
	}

	legs := TopologyE2ELegs(entry.Version, entry.Topology)
	if len(legs) == 0 {
		return nil
	}

	// Namespaces are only knowable per release index, so map each release's suffix to its context.
	nsBySuffix := map[string]string{}
	relBySuffix := map[string]TopologyRelease{}
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

		release, _, buildErr := t.releaseFlags(relBySuffix[leg.OrchestrationSuffix], orchestrationNamespace)
		if buildErr != nil {
			failures = append(failures, fmt.Sprintf("leg %q: build flags: %v", leg.OrchestrationSuffix, buildErr))
			continue
		}
		flags, namespace := release.flags, release.namespace
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

func topologyPlaywrightProject(leg TopologyE2ELeg) string {
	if leg.AdditionalPlaywrightProject != "" {
		return leg.AdditionalPlaywrightProject
	}
	return leg.PlaywrightProject
}

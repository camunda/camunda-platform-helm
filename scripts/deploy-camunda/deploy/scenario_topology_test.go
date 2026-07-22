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

package deploy

import (
	"testing"

	"scripts/deploy-camunda/config"
)

func topologyTestReleases() []TopologyRelease {
	return []TopologyRelease{
		{Role: "management", NamespaceSuffix: "mgmt", Values: "multinamespace/management.yaml"},
		{Role: "orchestration", NamespaceSuffix: "orcha", Values: "multinamespace/orchestration.yaml", DependsOn: "management"},
		{Role: "orchestration", NamespaceSuffix: "orchb", Values: "multinamespace/orchestration.yaml", DependsOn: "management"},
	}
}

func TestGenerateTopologyContexts_ThreeContexts(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-810-mns",
		},
	}

	contexts, err := generateTopologyContexts("multinamespace", topologyTestReleases(), flags)
	if err != nil {
		t.Fatalf("generateTopologyContexts returned error: %v", err)
	}
	if len(contexts) != 3 {
		t.Fatalf("expected 3 contexts, got %d", len(contexts))
	}
}

func TestGenerateTopologyContexts_DistinctNamespaces(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-810-mns",
		},
	}

	contexts, err := generateTopologyContexts("multinamespace", topologyTestReleases(), flags)
	if err != nil {
		t.Fatalf("generateTopologyContexts returned error: %v", err)
	}

	seen := map[string]bool{}
	for _, c := range contexts {
		if seen[c.Namespace] {
			t.Fatalf("duplicate namespace %q across topology contexts", c.Namespace)
		}
		seen[c.Namespace] = true
		if c.Release != "integration" {
			t.Errorf("expected release \"integration\", got %q for namespace %q", c.Release, c.Namespace)
		}
	}

	want := map[string]bool{
		"matrix-810-mns-mgmt":  true,
		"matrix-810-mns-orcha": true,
		"matrix-810-mns-orchb": true,
	}
	for ns := range want {
		if !seen[ns] {
			t.Errorf("expected namespace %q, not found in %v", ns, seen)
		}
	}
}

func TestGenerateTopologyContexts_DistinctOrchestrationPrefixes(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-810-mns",
		},
	}

	contexts, err := generateTopologyContexts("multinamespace", topologyTestReleases(), flags)
	if err != nil {
		t.Fatalf("generateTopologyContexts returned error: %v", err)
	}

	prefixes := map[string]bool{}
	for _, c := range contexts {
		if prefixes[c.OrchestrationIndexPrefix] {
			t.Fatalf("duplicate orchestration index prefix %q across topology contexts", c.OrchestrationIndexPrefix)
		}
		prefixes[c.OrchestrationIndexPrefix] = true
	}
	if len(prefixes) != 3 {
		t.Fatalf("expected 3 distinct orchestration index prefixes, got %d: %v", len(prefixes), prefixes)
	}
}

func TestGenerateTopologyContexts_EmptyReleasesErrors(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{Namespace: "matrix-810-mns"},
	}
	if _, err := generateTopologyContexts("multinamespace", nil, flags); err == nil {
		t.Fatal("expected error for empty topology releases")
	}
}

func TestDeriveReleaseNamespace_TruncatesToLimit(t *testing.T) {
	base := "distribution-810-multinamespace-scenario-with-a-very-long-name"
	suffix := "orchestration"

	got := DeriveReleaseNamespace(base, suffix)

	if len(got) > 63 {
		t.Fatalf("DeriveReleaseNamespace(%q, %q) = %q (len %d), want len <= 63", base, suffix, got, len(got))
	}
	if got[len(got)-len(suffix):] != suffix {
		t.Fatalf("DeriveReleaseNamespace(%q, %q) = %q, want suffix %q preserved", base, suffix, got, suffix)
	}
}

func TestDeriveReleaseNamespace_DistinctSuffixesYieldDistinctNamespaces(t *testing.T) {
	base := "distribution-810-multinamespace-scenario-with-a-very-long-name"

	orcha := DeriveReleaseNamespace(base, "orcha")
	orchb := DeriveReleaseNamespace(base, "orchb")

	if orcha == orchb {
		t.Fatalf("expected distinct namespaces for distinct suffixes, got %q for both", orcha)
	}
}

func TestDeriveReleaseNamespace_NoTruncationWhenWithinLimit(t *testing.T) {
	got := DeriveReleaseNamespace("matrix-810-mns", "mgmt")
	want := "matrix-810-mns-mgmt"
	if got != want {
		t.Fatalf("DeriveReleaseNamespace() = %q, want %q", got, want)
	}
}

// TestGenerateScenarioContext_Unaffected pins down that the existing
// single-namespace path is untouched by the topology addition.
func TestGenerateScenarioContext_Unaffected(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-810-eske",
			Scenarios: []string{"elasticsearch"},
		},
	}

	ctx, err := generateScenarioContext("elasticsearch", flags)
	if err != nil {
		t.Fatalf("generateScenarioContext returned error: %v", err)
	}
	if ctx.Namespace != "matrix-810-eske" {
		t.Errorf("expected single-namespace path to keep the base namespace, got %q", ctx.Namespace)
	}
	if ctx.Release != "integration" {
		t.Errorf("expected release \"integration\", got %q", ctx.Release)
	}
}

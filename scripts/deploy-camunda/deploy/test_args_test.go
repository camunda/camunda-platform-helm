// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"testing"
)

func hasFlagValue(args []string, flag, value string) bool {
	for i, a := range args {
		if a == flag {
			return i+1 < len(args) && args[i+1] == value
		}
	}
	return false
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// A topology leg must forward every namespace it was given. Dropping --hub-namespace rendered the e2e
// env from the orchestration namespace alone, so Web Modeler, Management Identity and Keycloak
// resolved to the orchestration host where they do not exist, and the setup project timed out with no
// hint that an argument was missing.
func TestE2EScriptArgs_ForwardsTopologyNamespaces(t *testing.T) {
	args := e2eScriptArgs(
		"/repo/charts/camunda-platform-8.10",
		"ns-orcha",
		"gke-ctx",
		"",
		"elasticsearch-external",
		topologyTarget{
			HubNamespace:        "ns-hub",
			OptimizeNamespace:   "ns-opta",
			OptimizeContextPath: "/optimize-orcha",
			ModelerClusterName:  "Orchestration A",
		},
	)

	for _, tc := range []struct{ flag, value string }{
		{"--namespace", "ns-orcha"},
		{"--hub-namespace", "ns-hub"},
		{"--optimize-namespace", "ns-opta"},
		{"--optimize-context-path", "/optimize-orcha"},
		{"--modeler-cluster-name", "Orchestration A"},
		{"--kube-context", "gke-ctx"},
	} {
		if !hasFlagValue(args, tc.flag, tc.value) {
			t.Errorf("%s %s missing from args: %v", tc.flag, tc.value, args)
		}
	}
}

// A single-release deployment must not gain topology flags: --hub-namespace switches
// run-e2e-tests.sh onto the merge path, which needs a Hub namespace to read from.
func TestE2EScriptArgs_OmitsTopologyFlagsForSingleRelease(t *testing.T) {
	args := e2eScriptArgs("/repo/charts/camunda-platform-8.10", "ns", "", "", "elasticsearch", topologyTarget{})

	for _, flag := range []string{"--hub-namespace", "--optimize-namespace", "--optimize-context-path", "--modeler-cluster-name"} {
		if hasFlag(args, flag) {
			t.Errorf("%s must be omitted for a single-release deployment, got %v", flag, args)
		}
	}
}

// 8.10+ runs the full suite; older charts are restricted to the smoke project.
func TestE2EScriptArgs_SmokeOnlyBelow810(t *testing.T) {
	if hasFlag(e2eScriptArgs("/repo/charts/camunda-platform-8.10", "ns", "", "", "", topologyTarget{}), "--run-smoke-tests") {
		t.Error("8.10 must run the full suite")
	}
	if !hasFlag(e2eScriptArgs("/repo/charts/camunda-platform-8.9", "ns", "", "", "", topologyTarget{}), "--run-smoke-tests") {
		t.Error("8.9 must be restricted to the smoke project")
	}
}

func TestE2EScriptArgs_OpenSearchFromPersistence(t *testing.T) {
	if !hasFlag(e2eScriptArgs("/c/camunda-platform-8.10", "ns", "", "", "opensearch-self-signed", topologyTarget{}), "--opensearch") {
		t.Error("--opensearch must be derived from the persistence layer name")
	}
	if hasFlag(e2eScriptArgs("/c/camunda-platform-8.10", "ns", "", "", "elasticsearch", topologyTarget{}), "--opensearch") {
		t.Error("--opensearch must not be set for an elasticsearch persistence layer")
	}
}

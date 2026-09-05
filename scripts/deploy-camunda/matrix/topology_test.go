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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"scripts/deploy-camunda/pkg/deployer"
	"scripts/deploy-camunda/pkg/types"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeValuesFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, "values", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func writeLayer(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, "values", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// optimizeTopology returns a minimal valid hub+orchestration+optimize topology
// whose optimize release carries the given declaration.
func optimizeTopology(serves, contextPath string) *Topology {
	return &Topology{
		Name: "optimize-layer-crosscheck",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "orchestration", NamespaceSuffix: "orchb", Features: []string{"orchestration"}, ModelerClusterID: "orchb", ModelerClusterName: "Orchestration B", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: serves, OptimizeContextPath: contextPath, Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}
}

const optimizeLayerFollowingDeclaration = `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`

func writeDepFile(t *testing.T, depsDir, id string) {
	t.Helper()
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(depsDir, id+".yaml"), []byte("release-name: "+id+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func validatePreparedTest(t *testing.T, top *Topology, dir string) error {
	t.Helper()
	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		return err
	}
	tempDir := t.TempDir()
	shared := map[string]string{}
	for _, release := range top.Releases {
		if release.Role == "optimize" {
			shared[TopologyEnvToken(release.NamespaceSuffix)+"_OPTIMIZE_CONTEXT_PATH"] = release.OptimizeContextPath
		}
	}
	rendered := make([]RenderedTopologyRelease, 0, len(top.Releases))
	for i, release := range top.Releases {
		paths := []string{filepath.Join(dir, "values", "base.yaml")}
		if release.Identity != "" {
			paths = append(paths, filepath.Join(dir, "values", "identity", release.Identity+".yaml"))
		}
		if release.Persistence != "" {
			paths = append(paths, filepath.Join(dir, "values", "persistence", release.Persistence+".yaml"))
		}
		for _, feature := range release.Features {
			paths = append(paths, filepath.Join(dir, "values", "features", feature+".yaml"))
		}
		var files []string
		for j, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content = []byte(os.Expand(string(content), func(name string) string {
				if value, ok := shared[name]; ok {
					return value
				}
				switch name {
				case "RELEASE_OPTIMIZE_CONTEXT_PATH":
					return release.OptimizeContextPath
				case "SERVED_ORCHESTRATION_INDEX_PREFIX":
					return "job-" + release.Serves
				}
				return "${" + name + "}"
			}))
			processed := filepath.Join(tempDir, fmt.Sprintf("%d-%d.yaml", i, j))
			if err := os.WriteFile(processed, content, 0o644); err != nil {
				t.Fatal(err)
			}
			files = append(files, processed)
		}
		var extraArgs []string
		if release.Role == "orchestration" {
			extraArgs = []string{"--set", "orchestration.exporters.zeebe.index.prefix=job-" + release.NamespaceSuffix}
		}
		extraArgs = append([]string{"--set", "global.topology.mode=combined"}, extraArgs...)
		rendered = append(rendered, RenderedTopologyRelease{Release: release, Contract: renderContractTest(t, files, extraArgs)})
	}
	for i, release := range top.Releases {
		if release.Role != "hub" {
			continue
		}
		declared := false
		for _, feature := range release.Features {
			content, _ := os.ReadFile(filepath.Join(dir, "values", "features", feature+".yaml"))
			declared = declared || strings.Contains(string(content), "clusters:")
		}
		rendered[i].Contract.Hub.ClustersDeclared = declared
	}
	return top.ValidateRendered("ctx", rendered)
}

func renderContractTest(t *testing.T, valuesFiles, extraArgs []string) TopologyContract {
	t.Helper()
	chartPath, err := filepath.Abs(filepath.Join("..", "..", "..", "charts", "camunda-platform-8.10"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := deployer.RenderTopologyContract(context.Background(), types.Options{
		ChartPath: chartPath, ReleaseName: "integration", Namespace: "test", ValuesFiles: valuesFiles,
		ExtraArgs: append([]string{"--skip-schema-validation", "--set", "orchestration.data.secondaryStorage.type=elasticsearch"}, extraArgs...),
	})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(manifest, &document); err != nil {
		t.Fatal(err)
	}
	var contract TopologyContract
	if err := json.Unmarshal([]byte(document.Data["contract.json"]), &contract); err != nil {
		t.Fatal(err)
	}
	return contract
}

func TestTopologyValidateRenderedUsesContractOnly(t *testing.T) {
	top := optimizeTopology("orcha", "/optimize-orcha")
	secret := TopologyContractSecret{Kind: "ref", Name: "oidc", Key: "optimize"}
	optimize := TopologyContractOptimize{
		Enabled: true, ContextPath: "/optimize-orcha", Backend: "elasticsearch", IndexPrefix: "job-orcha",
		ClientID: "optimize-orcha", Audience: "optimize-orcha-api", RedirectURL: "https://hub.test/optimize-orcha", Secret: secret,
	}
	var hub, orchestration TopologyContract
	hub.Hub.AuthType = "KEYCLOAK"
	hub.Hub.ClustersDeclared = true
	hub.Hub.Clusters = []TopologyContractCluster{{ID: "orcha", OptimizeContextPath: optimize.ContextPath, Optimize: optimize}}
	orchestration.Orchestration.ElasticsearchIndexPrefix = "job-orcha"

	rendered := []RenderedTopologyRelease{
		{Release: top.Releases[0], Contract: hub},
		{Release: top.Releases[1], Contract: orchestration},
		{Release: top.Releases[2]},
		{Release: top.Releases[3], Contract: TopologyContract{Optimize: optimize}},
	}
	if err := top.ValidateRendered("ctx", rendered); err != nil {
		t.Fatalf("matching contracts should validate: %v", err)
	}
	rendered[3].Contract.Optimize.IndexPrefix = "wrong"
	if err := top.ValidateRendered("ctx", rendered); err == nil || !strings.Contains(err.Error(), "writer prefix") {
		t.Fatalf("expected contract prefix mismatch, got %v", err)
	}
}

func TestTopologyValidateRenderedFallsBackToStandaloneIdentityClient(t *testing.T) {
	top := optimizeTopology("orcha", "/optimize-orcha")
	second := top.Releases[3]
	second.NamespaceSuffix = "optb"
	second.OptimizeContextPath = "/optimize-b"
	top.Releases = append(top.Releases, second)

	secret := TopologyContractSecret{Kind: "inline", Token: "redacted-hash"}
	first := TopologyContractOptimize{Enabled: true, ContextPath: "/optimize-orcha", Backend: "elasticsearch", IndexPrefix: "job-orcha", ClientID: "optimize-a", Audience: "api", RedirectURL: "https://hub.test/optimize-orcha", Secret: secret}
	secondOptimize := first
	secondOptimize.ContextPath = "/optimize-b"
	secondOptimize.ClientID = "optimize-b"
	secondOptimize.RedirectURL = "https://hub.test/optimize-b"
	var hub, orchestration TopologyContract
	hub.Hub.AuthType = "KEYCLOAK"
	hub.Hub.ClustersDeclared = true
	hub.Hub.Clusters = []TopologyContractCluster{{ID: "orcha", OptimizeContextPath: first.ContextPath, Optimize: first}}
	hub.Hub.IdentityClients = []TopologyContractIdentityClient{{ID: "optimize-b", RootURL: secondOptimize.RedirectURL, ResourceServerIDs: []string{"api"}, Secret: secret}}
	orchestration.Orchestration.ElasticsearchIndexPrefix = "job-orcha"
	rendered := []RenderedTopologyRelease{{Release: top.Releases[0], Contract: hub}, {Release: top.Releases[1], Contract: orchestration}, {Release: top.Releases[2]}, {Release: top.Releases[3], Contract: TopologyContract{Optimize: first}}, {Release: top.Releases[4], Contract: TopologyContract{Optimize: secondOptimize}}}
	if err := top.ValidateRendered("ctx", rendered); err != nil {
		t.Fatalf("standalone identity client should provision the second Optimize: %v", err)
	}
	hub.Hub.IdentityClients[0].Secret.Token = "different-redacted-hash"
	rendered[0].Contract = hub
	err := top.ValidateRendered("ctx", rendered)
	if err == nil || !strings.Contains(err.Error(), "compared as redacted hashes") {
		t.Fatalf("expected redacted secret mismatch, got %v", err)
	}
	if strings.Contains(err.Error(), "redacted-hash") {
		t.Fatal("validation error exposed a secret token")
	}
}

func TestTopologyValidate_NilIsNoop(t *testing.T) {
	var top *Topology
	if err := top.Validate("ctx", t.TempDir(), t.TempDir()); err != nil {
		t.Fatalf("nil Topology should be a no-op, got: %v", err)
	}
}

func TestTopologyValidate_Valid(t *testing.T) {
	dir := t.TempDir()
	depsDir := filepath.Join(t.TempDir(), "dependencies")
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "identity/keycloak.yaml")
	writeValuesFile(t, dir, "identity/keycloak-external.yaml")
	writeValuesFile(t, dir, "persistence/elasticsearch-external.yaml")
	writeDepFile(t, depsDir, "keycloak")
	writeDepFile(t, depsDir, "postgresql")
	writeDepFile(t, depsDir, "elasticsearch")

	top := &Topology{
		Name:          "hub-2orch",
		SharedStorage: "elasticsearch",
		Releases: []TopologyRelease{
			{
				Role:            "hub",
				NamespaceSuffix: "hub",
				Features:        []string{"hub"},
				Identity:        "keycloak",
				Dependencies:    []string{"keycloak", "postgresql", "elasticsearch"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orcha",
				ModelerClusterID:   "orcha",
				ModelerClusterName: "Orchestration A",
				Features:           []string{"orchestration"},
				Identity:           "keycloak-external",
				Persistence:        "elasticsearch-external",
				DependsOn:          "hub",
				Env:                map[string]string{"ORCH_ORCHESTRATION_CLIENT_ID": "orchestration-orcha"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orchb",
				ModelerClusterID:   "orchb",
				ModelerClusterName: "Orchestration B",
				Features:           []string{"orchestration"},
				Identity:           "keycloak-external",
				Persistence:        "elasticsearch-external",
				DependsOn:          "hub",
			},
		},
	}
	if err := top.Validate("ctx", dir, depsDir); err != nil {
		t.Fatalf("expected valid topology, got: %v", err)
	}
	if got := top.Releases[1].Env["ORCH_ORCHESTRATION_CLIENT_ID"]; got != "orchestration-orcha" {
		t.Fatalf("release env = %q", got)
	}
}

func TestTopologyValidate_ValidWithOptimizeReleases(t *testing.T) {
	dir := t.TempDir()
	depsDir := filepath.Join(t.TempDir(), "dependencies")
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	writeValuesFile(t, dir, "identity/keycloak.yaml")
	writeValuesFile(t, dir, "identity/keycloak-external.yaml")
	writeValuesFile(t, dir, "persistence/elasticsearch-external.yaml")
	writeDepFile(t, depsDir, "elasticsearch")

	top := &Topology{
		Name:          "hub-orch-2optimize",
		SharedStorage: "elasticsearch",
		Releases: []TopologyRelease{
			{
				Role:            "hub",
				NamespaceSuffix: "hub",
				Features:        []string{"hub"},
				Identity:        "keycloak",
				Dependencies:    []string{"elasticsearch"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orcha",
				ModelerClusterID:   "orcha",
				ModelerClusterName: "Orchestration A",
				Features:           []string{"orchestration"},
				Identity:           "keycloak-external",
				Persistence:        "elasticsearch-external",
				DependsOn:          "hub",
			},
			{
				Role:                "optimize",
				NamespaceSuffix:     "opta",
				Serves:              "orcha",
				OptimizeContextPath: "/optimize-orcha",
				Features:            []string{"optimize"},
				Identity:            "keycloak-external",
				Persistence:         "elasticsearch-external",
				DependsOn:           "hub",
			},
		},
	}
	if err := top.Validate("ctx", dir, depsDir); err != nil {
		t.Fatalf("expected valid topology with optimize releases, got: %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleNeedsNoModelerCluster(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	top := &Topology{
		Name: "optimize-no-modeler-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("optimize role must not require modeler-cluster-id/name, got: %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleRequiresDependsOn(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	top := &Topology{
		Name: "optimize-without-depends-on",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `depends-on must be "hub"`) {
		t.Fatalf("expected depends-on=hub requirement for optimize role, got %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleRejectsNonHubDependsOn(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	top := &Topology{
		Name: "optimize-depends-on-orchestration",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}, DependsOn: "orchestration"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `depends-on must be "hub"`) {
		t.Fatalf("expected optimize role to reject depends-on=orchestration, got %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleRequiresServesAndContextPath(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	top := &Topology{
		Name: "optimize-missing-mapping",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "serves is required") || !strings.Contains(err.Error(), "optimize-context-path is required") {
		t.Fatalf("expected serves and optimize-context-path requirements, got %v", err)
	}
}

func TestTopologyValidate_OptimizeServesMustNameOrchestrationRelease(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	top := &Topology{
		Name: "optimize-unknown-serves",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "nope", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "does not reference a declared orchestration release") {
		t.Fatalf("expected serves cross-check, got %v", err)
	}
}

func TestTopologyValidate_AcceptsOptimizeLayerFollowingDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)

	if err := optimizeTopology("orcha", "/optimize-orcha").Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("a layer that references the published placeholders must validate, got: %v", err)
	}
}

// Repointing serves must not leave the layer reading the old release's records.
func TestTopologyValidate_RejectsOptimizeLayerPinnedToAnotherServesPrefix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      prefix: "${ORCHA_ORCHESTRATION_INDEX_PREFIX}"
`)

	err := validatePreparedTest(t, optimizeTopology("orchb", "/optimize-orchb"), dir)
	if err == nil || !strings.Contains(err.Error(), "SERVED_ORCHESTRATION_INDEX_PREFIX") {
		t.Fatalf("expected the prefix to be rejected for not following serves, got %v", err)
	}
}

func TestTopologyValidate_RejectsOptimizeLayerContextPathMismatch(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "/optimize-stale"
  database:
    elasticsearch:
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "but the release declares optimize-context-path") {
		t.Fatalf("expected the hardcoded contextPath to be rejected, got %v", err)
	}
}

func TestTopologyValidate_RejectsOptimizeLayerOpensearchPrefixNotFollowingServes(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    opensearch:
      prefix: "job-orcha"
`)

	err := validatePreparedTest(t, optimizeTopology("orchb", "/optimize-orchb"), dir)
	if err == nil || !strings.Contains(err.Error(), "optimize.database.opensearch.prefix") {
		t.Fatalf("expected the opensearch prefix to be cross-checked too, got %v", err)
	}
}

func TestTopologyValidate_RejectsOptimizeOnlyFieldsOnOtherRoles(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "serves-on-orchestration",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub", Serves: "orcha", OptimizeContextPath: "/optimize"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "must not set serves") || !strings.Contains(err.Error(), "must not set optimize-context-path") {
		t.Fatalf("expected serves/optimize-context-path to be rejected on non-optimize roles, got %v", err)
	}
}

func TestTopologyValidate_RequiresUniqueModelerClusters(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "duplicate-modeler-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "shared", ModelerClusterName: "Shared"},
			{Role: "orchestration", NamespaceSuffix: "orchb", Features: []string{"orchestration"}, ModelerClusterID: "shared", ModelerClusterName: "Shared"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "duplicate modeler-cluster-id") || !strings.Contains(err.Error(), "duplicate modeler-cluster-name") {
		t.Fatalf("expected duplicate Modeler cluster validation errors, got %v", err)
	}
}

func TestTopologyValidate_RequiresOrchestrationRelease(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "hub-only",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "at least one release with role \"orchestration\"") {
		t.Fatalf("expected missing orchestration release error, got %v", err)
	}
}

func TestTopologyValidate_RequiresDNS1123NamespaceSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "invalid-suffix",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "Hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orch_a", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "must be a lowercase DNS-1123 label") {
		t.Fatalf("expected DNS-1123 namespace suffix errors, got %v", err)
	}
}

func TestTopologyValidate_MissingValuesFile(t *testing.T) {
	dir := t.TempDir()
	top := &Topology{
		Name: "hub-1orch",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing values files")
	}
}

func TestTopologyValidate_MissingIdentityLayer(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "bad-identity",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}, Identity: "does-not-exist"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing identity layer file")
	}
}

func TestTopologyValidate_MissingPersistenceLayer(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "bad-persistence",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"orchestration"}, Persistence: "does-not-exist"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing persistence layer file")
	}
}

func TestTopologyValidate_MissingDependency(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "bad-dep",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}, Dependencies: []string{"does-not-exist"}},
		},
	}
	if err := top.Validate("ctx", dir, filepath.Join(t.TempDir(), "dependencies")); err == nil {
		t.Fatal("expected error for missing dependency file")
	}
}

func TestTopologyValidate_NoHubRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "no-hub",
		Releases: []TopologyRelease{
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}},
			{Role: "orchestration", NamespaceSuffix: "orchb", Features: []string{"orchestration"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing Hub role")
	}
}

func TestTopologyValidate_TwoHubRoles(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "two-hub",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "huba", Features: []string{"hub"}},
			{Role: "hub", NamespaceSuffix: "hubb", Features: []string{"hub"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for two Hub roles")
	}
}

func TestTopologyValidate_DuplicateNamespaceSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "dup-suffix",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "a", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "a", Features: []string{"orchestration"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for duplicate namespace-suffix")
	}
}

func TestTopologyValidate_DependsOnUnknownRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "bad-depends-on",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, DependsOn: "storage"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for depends-on referencing an undeclared role")
	}
}

func TestTopologyValidate_InvalidRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "weird.yaml")
	top := &Topology{
		Name: "bad-role",
		Releases: []TopologyRelease{
			{Role: "weird", NamespaceSuffix: "w", Features: []string{"weird"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestTopologyValidate_EmptyReleases(t *testing.T) {
	top := &Topology{Name: "empty"}
	if err := top.Validate("ctx", t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("expected error for empty releases")
	}
}

func TestTopologyValidate_AllowsSeveralOptimizeReleasesPerOrchestration(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	top := &Topology{
		Name: "two-tenants-one-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta1", Serves: "orcha", OptimizeContextPath: "/optimize-tenanta", Features: []string{"optimize"}, DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta2", Serves: "orcha", OptimizeContextPath: "/optimize-tenantb", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("several optimize releases may serve one orchestration release, got: %v", err)
	}
}

func TestTopologyValidate_RejectsDuplicateOptimizeContextPath(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)
	top := &Topology{
		Name: "colliding-optimize-paths",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta1", Serves: "orcha", OptimizeContextPath: "/optimize", Features: []string{"optimize"}, DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta2", Serves: "orcha", OptimizeContextPath: "/optimize", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "share optimize-context-path") {
		t.Fatalf("expected colliding optimize-context-path to be rejected, got %v", err)
	}
}

func TestTopologyValidate_RejectsLegacyValuesKey(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "legacy-values-key",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub", Values: "features/orchestration.yaml"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "is no longer supported") {
		t.Fatalf("expected the legacy values key to be rejected, got %v", err)
	}
}

func TestTopologyValidate_RequiresFeatures(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "no-features",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "features is required") {
		t.Fatalf("expected features to be required, got %v", err)
	}
}

func TestTopologyValidate_RejectsMissingFeatureFile(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "missing-feature-file",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"typo-orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `feature "typo-orchestration": missing values file`) {
		t.Fatalf("expected a missing feature layer to be reported, got %v", err)
	}
}

// A dollar sign is not a declaration: an optimize layer that substitutes some
// other variable follows nothing the topology states.
func TestTopologyValidate_RejectsOptimizeLayerContextPathFromForeignPlaceholder(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${STALE_PATH}"
  database:
    elasticsearch:
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "RELEASE_OPTIMIZE_CONTEXT_PATH") {
		t.Fatalf("expected a foreign placeholder to be rejected, got %v", err)
	}
}

// A literal that matches the declaration today is still a second copy of it, and
// the next change to the declaration updates only one of the two. Being in sync
// right now buys no exemption.
func TestTopologyValidate_RejectsOptimizeLayerContextPathAsASynchronizedLiteral(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "/optimize-orcha"
  database:
    elasticsearch:
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "RELEASE_OPTIMIZE_CONTEXT_PATH") {
		t.Fatalf("expected a literal matching the declaration to be rejected, got %v", err)
	}
}

// The placeholder has to lead the prefix, which is what makes repointing serves
// repoint the records: a value that only mentions it further along does not.
func TestTopologyValidate_RejectsOptimizeLayerPrefixNotLedByThePlaceholder(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      prefix: "wrong-${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "optimize.database.elasticsearch.prefix") {
		t.Fatalf("expected a prefix not led by the placeholder to be rejected, got %v", err)
	}
}

// A Physical Tenant's Optimize reads the tenant's slice of the served
// orchestration's records, so a suffix after the placeholder is the shape that
// design needs.
func TestTopologyValidate_AcceptsOptimizeLayerPrefixWithATenantSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}-ta"
`)

	if err := optimizeTopology("orcha", "/optimize-orcha").Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("a per-tenant suffix after the placeholder must validate, got: %v", err)
	}
}

// os.Expand ends an unbraced name at the first character that cannot continue
// one, so "$NAME-ta" substitutes the published variable and must validate the
// same as "${NAME}-ta". Rejecting it would fail a layer the deploy expands
// correctly.
func TestTopologyValidate_AcceptsOptimizeLayerPrefixFromABareTerminatedPlaceholder(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "$RELEASE_OPTIMIZE_CONTEXT_PATH"
  database:
    elasticsearch:
      prefix: "$SERVED_ORCHESTRATION_INDEX_PREFIX-ta"
`)

	if err := optimizeTopology("orcha", "/optimize-orcha").Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("a bare placeholder terminated by a hyphen must validate, got: %v", err)
	}
}

// A suffix that keeps the name going names a different variable, which os.Expand
// resolves to the empty string, so it must not pass as a per-tenant suffix.
func TestTopologyValidate_RejectsOptimizeLayerPrefixFromABareUnterminatedPlaceholder(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      prefix: "$SERVED_ORCHESTRATION_INDEX_PREFIXta"
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "optimize.database.elasticsearch.prefix") {
		t.Fatalf("expected an unterminated bare placeholder to be rejected, got %v", err)
	}
}

// A layer that states neither value is as wrong as one that hardcodes them: the
// release inherits whatever a base layer left behind.
func TestTopologyValidate_RejectsOptimizeLayerStatingNeitherValue(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  enabled: true
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil {
		t.Fatal("expected a layer stating neither contextPath nor prefix to be rejected")
	}
	for _, want := range []string{"no feature layer sets optimize.contextPath", "no feature layer sets optimize.database.elasticsearch.prefix"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in %v", want, err)
		}
	}
}

// A release env entry may not shadow a variable the driver derives from the
// declaration: buildTopologyReleaseEnv applies the derived value last, so the
// entry could only mislead its author.
func TestTopologyValidate_RejectsReleaseEnvShadowingDerivedKeys(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)

	for _, key := range []string{"RELEASE_OPTIMIZE_CONTEXT_PATH", "SERVED_ORCHESTRATION_INDEX_PREFIX", "SERVED_NAMESPACE", "SERVED_HOST", "ORCH_NAMESPACE", "ORCH_HOST", "ORCH_ZEEBE_GRPC", "ORCH_ZEEBE_REST"} {
		top := optimizeTopology("orcha", "/optimize-orcha")
		top.Releases[3].Env = map[string]string{key: "hand-written"}
		err := top.Validate("ctx", dir, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "env sets "+key) {
			t.Fatalf("expected release env %s to be rejected, got %v", key, err)
		}
	}
}

// Every other release env key stays free-form: the reserved list is exactly the
// derived one.
func TestTopologyValidate_AcceptsUnreservedReleaseEnv(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerFollowingDeclaration)

	top := optimizeTopology("orcha", "/optimize-orcha")
	top.Releases[3].Env = map[string]string{"ORCH_OPTIMIZE_CLIENT_ID": "optimize-orcha"}
	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("an unreserved release env key must validate, got: %v", err)
	}
}

// hubInventoryRegisteringOptimize is a Hub layer that registers cluster "orcha"'s
// Optimize, spelling its redirect URL and advertised path through the cross-ref
// placeholder the topology driver publishes for the "opta" optimize release.
const hubInventoryRegisteringOptimize = `global:
  topology:
    mode: hub
    clusters:
      - id: orcha
        contextPaths:
          optimize: "${OPTA_OPTIMIZE_CONTEXT_PATH}"
        components:
          optimize:
            enabled: true
            clientId: optimize-orcha
            audience: optimize-orcha-api
            redirectUrl: "https://${HUB_HOST}${OPTA_OPTIMIZE_CONTEXT_PATH}"
            secret:
              existingSecret: integration-test-credentials
              existingSecretKey: identity-optimize-client-token
`

// optimizeLayerMatchingHubInventory presents the identity the Hub above
// registers, reaching the same redirect URL through the release-local
// placeholder rather than the cross-ref one.
const optimizeLayerMatchingHubInventory = `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      enabled: true
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
  security:
    authentication:
      oidc:
        clientId: optimize-orcha
        audience: optimize-orcha-api
        redirectUrl: "https://${HUB_HOST}${RELEASE_OPTIMIZE_CONTEXT_PATH}"
        secret:
          existingSecret: integration-test-credentials
          existingSecretKey: identity-optimize-client-token
`

// optimizeLayerMatchingHubInventoryViaGlobal presents exactly the identity the
// Hub registers, but through the chart-wide fallback the Optimize helpers resolve
// when the component-scoped block leaves a field unset
// (optimize.effectiveAuthClientId, optimize.effectiveAuthRedirectUrl,
// camundaPlatform.authAudienceOptimize and optimize.effectiveAuthSecret all
// default to global.identity.auth.optimize). This is a supported way to configure
// the release, so the cross-check has to read it.
const optimizeLayerMatchingHubInventoryViaGlobal = `global:
  identity:
    auth:
      optimize:
        clientId: optimize-orcha
        audience: optimize-orcha-api
        redirectUrl: "https://${HUB_HOST}${RELEASE_OPTIMIZE_CONTEXT_PATH}"
        secret:
          existingSecret: integration-test-credentials
          existingSecretKey: identity-optimize-client-token
optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      enabled: true
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`

// The documented global fallback configures the same identity the component-scoped
// block does, so a release using it must be accepted rather than reported as
// setting nothing.
func TestTopologyValidate_AcceptsOptimizeIdentityFromTheGlobalFallback(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerMatchingHubInventoryViaGlobal)

	if err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir); err != nil {
		t.Fatalf("the global.identity.auth.optimize fallback configures this release, got: %v", err)
	}
}

// Reading the fallback must not make the cross-check toothless there: a global
// client id that disagrees with the Hub is exactly as broken as a component-scoped
// one, and the message has to name the key that actually holds the wrong value.
func TestTopologyValidate_RejectsOptimizeGlobalFallbackClientIdDisagreeingWithTheHub(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", strings.Replace(optimizeLayerMatchingHubInventoryViaGlobal, "clientId: optimize-orcha", "clientId: optimize-renamed", 1))

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "the Hub registers this cluster's Optimize client id as \"optimize-orcha\"") {
		t.Fatalf("expected the client id mismatch to be reported, got %v", err)
	}
}

// The component-scoped value wins over the fallback, the way `|default` does in
// optimize.effectiveAuthClientId, so a correct override must not be judged against
// the stale global value it replaces.
func TestTopologyValidate_OptimizeComponentIdentityOverridesTheGlobalFallback(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", strings.Replace(
		optimizeLayerMatchingHubInventoryViaGlobal,
		"        clientId: optimize-orcha\n",
		"        clientId: optimize-stale\n",
		1,
	)+`  security:
    authentication:
      oidc:
        clientId: optimize-orcha
`)

	if err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir); err != nil {
		t.Fatalf("the component-scoped client id wins over the fallback, got: %v", err)
	}
}

// A release-scoped existingSecretKey on its own only renames the key inside the
// inherited existingSecret (optimize.effectiveAuthSecret), so resolving it as a
// standalone reference with no Secret name would report a secret the release never
// sends.
func TestTopologyValidate_OptimizeSecretKeyAloneRenamesTheInheritedSecret(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", strings.Replace(hubInventoryRegisteringOptimize, "identity-optimize-client-token", "renamed-optimize-key", 1))
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerMatchingHubInventoryViaGlobal+`  security:
    authentication:
      oidc:
        secret:
          existingSecretKey: renamed-optimize-key
`)

	if err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir); err != nil {
		t.Fatalf("the release-scoped key renames the inherited Secret's key, got: %v", err)
	}
}

// Optimize reads only the enabled backend's prefix (optimize.indexPrefix in
// templates/optimize/_helpers.tpl checks Elasticsearch first), so a layer that
// sets the placeholder on the other backend leaves the release on the
// "zeebe-record" fallback while looking correct.
func TestTopologyValidate_RejectsOptimizePrefixSetOnTheDisabledBackend(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "base.yaml", "optimize:\n  database:\n    elasticsearch:\n      enabled: true\n")
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    opensearch:
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil {
		t.Fatal("expected an opensearch prefix to be rejected while elasticsearch is the enabled backend")
	}
	if want := `rendered Optimize elasticsearch index prefix is "zeebe-record"`; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q in %v", want, err)
	}
}

// The component-level switch enables a backend just as the global one does, and
// the choice must follow it rather than assume Elasticsearch.
func TestTopologyValidate_AcceptsOptimizePrefixOnTheEnabledOpensearchBackend(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    opensearch:
      enabled: true
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	if err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir); err != nil {
		t.Fatalf("an opensearch prefix must satisfy an opensearch-enabled release, got: %v", err)
	}
}

// A later layer turning Elasticsearch off moves the release to OpenSearch, the
// same way Helm's scalar precedence does, so the required key moves with it.
func TestTopologyValidate_BackendChoiceFollowsTheLastLayerToStateIt(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "base.yaml", "global:\n  elasticsearch:\n    enabled: true\n")
	writeLayer(t, dir, "persistence/opensearch.yaml", "global:\n  elasticsearch:\n    enabled: false\n  opensearch:\n    enabled: true\n")
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    opensearch:
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	top := optimizeTopology("orcha", "/optimize-orcha")
	top.Releases[3].Persistence = "opensearch"
	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("the persistence layer switches the release to opensearch, got: %v", err)
	}
}

// The client id Optimize presents and the client id Identity provisions for it
// live in two layers. Nothing but this cross-check couples them, and every
// workload still reports ready when they disagree.
func TestTopologyValidate_RejectsOptimizeClientIdDisagreeingWithTheHubInventory(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", strings.Replace(optimizeLayerMatchingHubInventory, "clientId: optimize-orcha", "clientId: optimize-renamed", 1))

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "the Hub registers this cluster's Optimize client id as \"optimize-orcha\"") {
		t.Fatalf("expected the client id mismatch to be reported, got %v", err)
	}
}

func TestTopologyValidate_RejectsOptimizeAudienceDisagreeingWithTheHubInventory(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", strings.Replace(optimizeLayerMatchingHubInventory, "audience: optimize-orcha-api", "audience: optimize-orcha", 1))

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "the Hub registers this cluster's Optimize audience as \"optimize-orcha-api\"") {
		t.Fatalf("expected the audience mismatch to be reported, got %v", err)
	}
}

// A release that states no client id at all inherits the chart default, which is
// never the client the Hub registered.
func TestTopologyValidate_RejectsOptimizeStatingNoneOfTheRegisteredIdentity(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      enabled: true
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}"
`)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), `Optimize client id "optimize" is not represented`) {
		t.Fatalf("expected the missing client id to be reported, got %v", err)
	}
}

// The Hub names the optimize release's path through the cross-ref placeholder and
// the release names its own through the release-local one. Both follow the same
// declaration, so the cross-check has to resolve them rather than compare the raw
// strings.
func TestTopologyValidate_AcceptsOptimizeIdentityReachedThroughDifferentPlaceholders(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerMatchingHubInventory)

	if err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir); err != nil {
		t.Fatalf("two spellings of the same declared path must agree, got: %v", err)
	}
}

// A redirect URL that resolves to a different path than the one Identity
// registered fails only at login, so it has to fail here.
func TestTopologyValidate_RejectsOptimizeRedirectUrlResolvingElsewhere(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", strings.Replace(
		optimizeLayerMatchingHubInventory,
		`redirectUrl: "https://${HUB_HOST}${RELEASE_OPTIMIZE_CONTEXT_PATH}"`,
		`redirectUrl: "https://${HUB_HOST}/optimize"`, 1))

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "optimize.security.authentication.oidc.redirectUrl") {
		t.Fatalf("expected the redirect URL mismatch to be reported, got %v", err)
	}
}

// The inventory context path is what the Hub's Console and Web Modeler link to; a
// stale one sends users to a path no ingress serves.
func TestTopologyValidate_RejectsHubInventoryPathDisagreeingWithTheDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", strings.Replace(
		hubInventoryRegisteringOptimize,
		`optimize: "${OPTA_OPTIMIZE_CONTEXT_PATH}"`,
		`optimize: /optimize-stale`, 1))
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerMatchingHubInventory)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "the Hub advertises this cluster's Optimize at \"/optimize-stale\"") {
		t.Fatalf("expected the advertised path mismatch to be reported, got %v", err)
	}
}

// The driver derives a release's cross-reference variable prefix from its
// namespace-suffix, and the cross-checks look those variables up, so both sides
// have to spell them the same way.
func TestTopologyEnvToken(t *testing.T) {
	for value, want := range map[string]string{
		"opta":       "OPTA",
		"orch-a":     "ORCH_A",
		"opt.a":      "OPT_A",
		"-opta-":     "OPTA",
		"tenant-a-1": "TENANT_A_1",
	} {
		if got := TopologyEnvToken(value); got != want {
			t.Errorf("TopologyEnvToken(%q) = %q, want %q", value, got, want)
		}
	}
}

// twoTenantOptimizeTopology returns a topology whose single orchestration release
// is served by two optimize releases, the shape a Physical Tenant deployment
// takes: the cluster record can name only one of them, so the other has to be
// provisioned as a standalone Identity client.
func twoTenantOptimizeTopology() *Topology {
	return &Topology{
		Name: "optimize-two-tenants",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}, DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "optb", Serves: "orcha", OptimizeContextPath: "/optimize-orcha-b", Features: []string{"optimize-b"}, DependsOn: "hub"},
		},
	}
}

// hubInventoryWithSecondOptimizeClient registers tenant A's Optimize on the
// cluster record and tenant B's as a plain client, which is the only place a
// second Optimize for one cluster can go.
const hubInventoryWithSecondOptimizeClient = hubInventoryRegisteringOptimize + `identity:
  clients:
    - id: optimize-orcha-b
      name: Optimize Orchestration A tenant B
      type: confidential
      rootUrl: "https://${HUB_HOST}${OPTB_OPTIMIZE_CONTEXT_PATH}"
      redirectUris: /api/authentication/callback
      secret:
        existingSecret: integration-test-credentials
        existingSecretKey: identity-optimize-client-token
      permissions:
        - resourceServerId: optimize-orcha-api
          definition: write:*
`

// optimizeLayerSecondTenant presents the identity the standalone client above
// provisions.
const optimizeLayerSecondTenant = `optimize:
  contextPath: "${RELEASE_OPTIMIZE_CONTEXT_PATH}"
  database:
    elasticsearch:
      enabled: true
      prefix: "${SERVED_ORCHESTRATION_INDEX_PREFIX}-b"
  security:
    authentication:
      oidc:
        clientId: optimize-orcha-b
        audience: optimize-orcha-api
        redirectUrl: "https://${HUB_HOST}${RELEASE_OPTIMIZE_CONTEXT_PATH}"
        secret:
          existingSecret: integration-test-credentials
          existingSecretKey: identity-optimize-client-token
`

func writeTwoTenantLayers(t *testing.T, dir, hubLayer, tenantALayer, tenantBLayer string) {
	t.Helper()
	writeLayer(t, dir, "features/hub.yaml", hubLayer)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", tenantALayer)
	writeLayer(t, dir, "features/optimize-b.yaml", tenantBLayer)
}

func TestTopologyValidate_AcceptsASecondTenantOptimizeProvisionedAsAnIdentityClient(t *testing.T) {
	dir := t.TempDir()
	writeTwoTenantLayers(t, dir, hubInventoryWithSecondOptimizeClient, optimizeLayerMatchingHubInventory, optimizeLayerSecondTenant)

	if err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir); err != nil {
		t.Fatalf("expected the two-tenant topology to validate, got %v", err)
	}
}

// A second optimize release must not switch the check off for the release the
// cluster record does name: that release's identity is still duplicated, and a
// drift in it still fails only at login.
func TestTopologyValidate_StillChecksTheRecordReleaseWhenASecondTenantServesTheSameOrchestration(t *testing.T) {
	dir := t.TempDir()
	drifted := strings.Replace(optimizeLayerMatchingHubInventory, "audience: optimize-orcha-api", "audience: optimize-renamed-api", 1)
	writeTwoTenantLayers(t, dir, hubInventoryWithSecondOptimizeClient, drifted, optimizeLayerSecondTenant)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "the Hub registers this cluster's Optimize audience as \"optimize-orcha-api\"") {
		t.Fatalf("expected the record release's audience drift to still be reported, got %v", err)
	}
}

func TestTopologyValidate_RejectsASecondTenantOptimizeNothingProvisions(t *testing.T) {
	dir := t.TempDir()
	unprovisioned := strings.Replace(optimizeLayerSecondTenant, "clientId: optimize-orcha-b", "clientId: optimize-orcha-c", 1)
	writeTwoTenantLayers(t, dir, hubInventoryWithSecondOptimizeClient, optimizeLayerMatchingHubInventory, unprovisioned)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "so nothing provisions this release's client") {
		t.Fatalf("expected the unprovisioned client to be reported, got %v", err)
	}
}

// hubWithNonKeycloakIssuer rewrites the Hub layer to a non-Keycloak issuer.
// templates/identity/configmap.yaml renders the whole identity.clients block only
// under authIssuerType KEYCLOAK, so under this issuer those entries provision
// nothing and the operator creates the client in their own IdP.
func hubWithNonKeycloakIssuer(layer string) string {
	return strings.Replace(layer, "global:\n  topology:", "global:\n  identity:\n    auth:\n      type: MICROSOFT\n  topology:", 1)
}

// A client with no rootUrl and no absolute redirectUris registers no callback at
// all, so Identity has nowhere to send Optimize back to. The deploy cannot see it:
// every workload reports ready and only the login fails.
func TestTopologyValidate_RejectsASecondTenantOptimizeClientRegisteringNoRedirect(t *testing.T) {
	dir := t.TempDir()
	hub := strings.Replace(hubInventoryWithSecondOptimizeClient, "      rootUrl: \"https://${HUB_HOST}${OPTB_OPTIMIZE_CONTEXT_PATH}\"\n", "", 1)
	writeTwoTenantLayers(t, dir, hub, optimizeLayerMatchingHubInventory, optimizeLayerSecondTenant)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "no rootUrl and no absolute redirectUris") {
		t.Fatalf("expected the missing redirect registration to be reported, got %v", err)
	}
}

// A client with no permissions is minted no token for any resource server, so the
// audience the release declares is unreachable rather than merely unmatched.
func TestTopologyValidate_RejectsASecondTenantOptimizeClientWithNoPermissions(t *testing.T) {
	dir := t.TempDir()
	hub := strings.Replace(hubInventoryWithSecondOptimizeClient, "      permissions:\n        - resourceServerId: optimize-orcha-api\n          definition: write:*\n", "", 1)
	writeTwoTenantLayers(t, dir, hub, optimizeLayerMatchingHubInventory, optimizeLayerSecondTenant)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "no permission naming a resource server") {
		t.Fatalf("expected the missing permissions to be reported, got %v", err)
	}
}

// A permissions list whose entries name no resource server is non-empty and still
// grants access to nothing, so counting entries rather than usable servers would
// let it through as a permission that merely fails to match.
func TestTopologyValidate_RejectsASecondTenantOptimizeClientWhosePermissionsNameNoResourceServer(t *testing.T) {
	dir := t.TempDir()
	hub := strings.Replace(
		hubInventoryWithSecondOptimizeClient,
		"      permissions:\n        - resourceServerId: optimize-orcha-api\n          definition: write:*\n",
		"      permissions:\n        - definition: write:*\n",
		1,
	)
	writeTwoTenantLayers(t, dir, hub, optimizeLayerMatchingHubInventory, optimizeLayerSecondTenant)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "no permission naming a resource server") {
		t.Fatalf("expected a permissions list naming no resource server to be reported, got %v", err)
	}
}

// Under a non-Keycloak issuer identity.clients provisions nothing, so the same
// incomplete entry is not a defect: the real client lives in the operator's IdP
// and the chart cannot see it. Requiring these fields here would fail a supported
// topology, which is why both requirements above are scoped to Keycloak.
func TestTopologyValidate_AcceptsAnIncompleteClientUnderANonKeycloakIssuer(t *testing.T) {
	dir := t.TempDir()
	hub := strings.Replace(hubInventoryWithSecondOptimizeClient, "      rootUrl: \"https://${HUB_HOST}${OPTB_OPTIMIZE_CONTEXT_PATH}\"\n", "", 1)
	hub = strings.Replace(hub, "      permissions:\n        - resourceServerId: optimize-orcha-api\n          definition: write:*\n", "", 1)
	writeTwoTenantLayers(t, dir, hubWithNonKeycloakIssuer(hub), optimizeLayerMatchingHubInventory, optimizeLayerSecondTenant)

	if err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir); err != nil {
		t.Fatalf("a non-Keycloak issuer provisions no identity.clients, so nothing here is required: %v", err)
	}
}

// Keycloak honours an absolute redirectUris entry as it stands and resolves only
// relative ones against rootUrl, so an absolute entry is a complete registration
// and an empty rootUrl beside one is correct rather than broken.
func TestTopologyValidate_AcceptsASecondTenantOptimizeRedirectFromAnAbsoluteRedirectUri(t *testing.T) {
	dir := t.TempDir()
	hub := strings.Replace(
		hubInventoryWithSecondOptimizeClient,
		"      rootUrl: \"https://${HUB_HOST}${OPTB_OPTIMIZE_CONTEXT_PATH}\"\n      redirectUris: /api/authentication/callback\n",
		"      redirectUris:\n        - \"https://${HUB_HOST}${OPTB_OPTIMIZE_CONTEXT_PATH}\"\n",
		1,
	)
	writeTwoTenantLayers(t, dir, hub, optimizeLayerMatchingHubInventory, optimizeLayerSecondTenant)

	if err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir); err != nil {
		t.Fatalf("an absolute redirectUri registers the callback on its own: %v", err)
	}
}

func TestTopologyValidate_RejectsASecondTenantOptimizeRedirectingOffItsRegisteredRootUrl(t *testing.T) {
	dir := t.TempDir()
	drifted := strings.Replace(optimizeLayerSecondTenant, "${RELEASE_OPTIMIZE_CONTEXT_PATH}\"\n        secret", "${OPTA_OPTIMIZE_CONTEXT_PATH}\"\n        secret", 1)
	writeTwoTenantLayers(t, dir, hubInventoryWithSecondOptimizeClient, optimizeLayerMatchingHubInventory, drifted)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "the Hub registers this client's redirect as") {
		t.Fatalf("expected the redirect URL to be checked against the registered root URL, got %v", err)
	}
	if !strings.Contains(err.Error(), `.rootUrl)`) {
		t.Fatalf("the message must name rootUrl as the registered source, got %v", err)
	}
}

func TestTopologyValidate_RejectsASecondTenantOptimizeAudiencedToAnUnpermittedResourceServer(t *testing.T) {
	dir := t.TempDir()
	drifted := strings.Replace(optimizeLayerSecondTenant, "audience: optimize-orcha-api", "audience: optimize-orcha-b-api", 1)
	writeTwoTenantLayers(t, dir, hubInventoryWithSecondOptimizeClient, optimizeLayerMatchingHubInventory, drifted)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "the Hub permits this client only on optimize-orcha-api") {
		t.Fatalf("expected the unpermitted audience to be reported, got %v", err)
	}
}

// hubInventoryResetToNoClusters is what a later Hub layer does when it replaces
// the cluster inventory with an empty list: Helm deploys the empty one, so
// Identity provisions nothing, however much an earlier layer registered.
const hubInventoryResetToNoClusters = `global:
  topology:
    clusters: []
`

// Helm replaces a list rather than merging it, so the last Hub layer to state the
// cluster inventory is the whole inventory. A validator that only replaces its
// stored copy when the new list has entries keeps checking registrations the
// deploy no longer makes.
func TestTopologyValidate_RejectsOptimizeWhoseHubClusterInventoryIsResetToEmpty(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "base.yaml", hubInventoryRegisteringOptimize)
	writeLayer(t, dir, "features/hub.yaml", hubInventoryResetToNoClusters)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerMatchingHubInventory)

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "so nothing provisions this release's client") {
		t.Fatalf("expected the emptied cluster inventory to leave the client unprovisioned, got %v", err)
	}
}

// The control for the test above: the same two layers, with the later one silent
// about the inventory rather than emptying it. Presence of the key is what has to
// decide, not the length of the list it parses to.
func TestTopologyValidate_AcceptsOptimizeWhoseLaterHubLayerLeavesTheInventoryAlone(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "base.yaml", hubInventoryRegisteringOptimize)
	writeLayer(t, dir, "features/hub.yaml", "global:\n  topology:\n    mode: hub\n")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", optimizeLayerMatchingHubInventory)

	if err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir); err != nil {
		t.Fatalf("a later layer that states no inventory must leave the registrations standing, got %v", err)
	}
}

// identity.clients is replaced the same way, and it is the only place a second
// tenant's Optimize can be provisioned.
func TestTopologyValidate_RejectsSecondTenantOptimizeWhoseHubClientListIsResetToEmpty(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "base.yaml", hubInventoryWithSecondOptimizeClient)
	writeTwoTenantLayers(t, dir, "identity:\n  clients: []\n", optimizeLayerMatchingHubInventory, optimizeLayerSecondTenant)

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "so nothing provisions this release's client") {
		t.Fatalf("expected the emptied client list to leave tenant B unprovisioned, got %v", err)
	}
}

// Both sides also accept an inline client secret, and comparing only the
// existing-Secret fields never looks at it: two different literals passed, and
// Optimize authenticated with a secret Identity had not registered.
func TestTopologyValidate_RejectsOptimizeInlineSecretDifferingFromTheHubInventory(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", inlineSecretOn(hubInventoryRegisteringOptimize, "hub-registered-secret"))
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", inlineSecretOn(optimizeLayerMatchingHubInventory, "release-sent-secret"))

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "are different literals") {
		t.Fatalf("expected the differing inline secrets to be reported, got %v", err)
	}
	if strings.Contains(err.Error(), "release-sent-secret") || strings.Contains(err.Error(), "hub-registered-secret") {
		t.Fatalf("a client secret must not be quoted into the problem, got %v", err)
	}
}

func TestTopologyValidate_AcceptsOptimizeInlineSecretMatchingTheHubInventory(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", inlineSecretOn(hubInventoryRegisteringOptimize, "shared-secret"))
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", inlineSecretOn(optimizeLayerMatchingHubInventory, "shared-secret"))

	if err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir); err != nil {
		t.Fatalf("the same inline secret on both sides must validate, got %v", err)
	}
}

// An inline literal and a Secret reference cannot be shown to hold the same
// value, and the Deployment sends only one of them.
func TestTopologyValidate_RejectsOptimizeInlineSecretAgainstARegisteredSecretReference(t *testing.T) {
	dir := t.TempDir()
	writeLayer(t, dir, "features/hub.yaml", hubInventoryRegisteringOptimize)
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeLayer(t, dir, "features/optimize.yaml", inlineSecretOn(optimizeLayerMatchingHubInventory, "release-sent-secret"))

	err := validatePreparedTest(t, optimizeTopology("orcha", "/optimize-orcha"), dir)
	if err == nil || !strings.Contains(err.Error(), "state it in different forms") {
		t.Fatalf("expected the mismatched secret forms to be reported, got %v", err)
	}
}

// The standalone-client path resolves the secret with inlineSecret first, the
// order identity.clients[] is rendered in.
func TestTopologyValidate_RejectsSecondTenantOptimizeInlineSecretDifferingFromItsIdentityClient(t *testing.T) {
	dir := t.TempDir()
	hubLayer := hubInventoryRegisteringOptimize + inlineSecretOn(
		strings.TrimPrefix(hubInventoryWithSecondOptimizeClient, hubInventoryRegisteringOptimize),
		"hub-registered-secret")
	writeTwoTenantLayers(t, dir, hubLayer, optimizeLayerMatchingHubInventory,
		inlineSecretOn(optimizeLayerSecondTenant, "release-sent-secret"))

	err := validatePreparedTest(t, twoTenantOptimizeTopology(), dir)
	if err == nil || !strings.Contains(err.Error(), "are different literals") {
		t.Fatalf("expected the differing inline secrets to be reported on the client path, got %v", err)
	}
}

// inlineSecretOn rewrites every existing-Secret reference in a layer to the given
// inline literal, keeping the indentation each reference sits at.
func inlineSecretOn(layer, value string) string {
	var out []string
	for _, line := range strings.Split(layer, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := line[:len(line)-len(trimmed)]
		switch {
		case strings.HasPrefix(trimmed, "existingSecret:"):
			out = append(out, indent+"inlineSecret: "+value)
		case strings.HasPrefix(trimmed, "existingSecretKey:"):
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

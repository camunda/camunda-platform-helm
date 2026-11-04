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

package matrix

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"scripts/deploy-camunda/deploy"
)

type chainGolden struct {
	Scenario      string   `yaml:"scenario"`
	Shortname     string   `yaml:"shortname"`
	Flow          string   `yaml:"flow"`
	Tier          int      `yaml:"tier"`
	Layers        []string `yaml:"scenarioLayers,omitempty"`
	Chain         []string `yaml:"chain"`
	ResolvedImage string   `yaml:"resolvedImageFile,omitempty"`
	ResolvedBody  string   `yaml:"resolvedImageContent,omitempty"`
	MergedSHA256  string   `yaml:"mergedValuesSha256"`
}

// goldenEnvKeys lists every variable the scenario env substitutes into the
// values layers; goldenEnv pins each to a fixed placeholder so the
// merged-values hash does not depend on the host.
var goldenEnvKeys = []string{
	"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD",
	"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD",
	"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD",
	"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET",
	"ENTRA_APP_CLIENT_ID",
	"ENTRA_APP_CLIENT_SECRET",
	"ENTRA_APP_DIRECTORY_ID",
	"ENTRA_APP_OBJECT_ID",
	"RDBMS_POSTGRESQL_USERNAME",
	"RDBMS_POSTGRESQL_PASSWORD",
	"AUTH0_AUDIENCE",
	"AUTH0_CONNECTORS_CLIENT_ID",
	"AUTH0_CONSOLE_CLIENT_ID",
	"AUTH0_IDENTITY_CLIENT_ID",
	"AUTH0_INITIAL_ADMIN_EMAIL",
	"AUTH0_ISSUER_URL",
	"AUTH0_OPTIMIZE_CLIENT_ID",
	"AUTH0_ORCHESTRATION_CLIENT_ID",
	"AUTH0_WEB_MODELER_CLIENT_ID",
}

func goldenEnv() map[string]string {
	env := make(map[string]string, len(goldenEnvKeys))
	for _, key := range goldenEnvKeys {
		env[key] = "golden-" + strings.ToLower(key)
	}
	return env
}

// TestValuesChainGolden pins, per tier-1/2 registry scenario, the ordered -f
// chain BuildValuesChain produces, the resolved-image file and the sha256 of
// the merged values. Refresh with:
//
//	cd scripts/deploy-camunda && go test ./matrix/ -run TestValuesChainGolden -update-golden
func TestValuesChainGolden(t *testing.T) {
	repoRoot := findRepoRoot(t)
	for _, version := range []string{"8.9", "8.10", "8.11"} {
		t.Run(version, func(t *testing.T) {
			entries, err := Generate(repoRoot, GenerateOptions{Versions: []string{version}, Platform: "gke"})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			var got []chainGolden
			for _, entry := range entries {
				if entry.Tier != 1 && entry.Tier != 2 {
					continue
				}
				got = append(got, goldenForEntry(t, repoRoot, entry))
			}
			if len(got) == 0 {
				t.Fatalf("no tier-1/2 entries for %s", version)
			}
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(got); err != nil {
				t.Fatalf("yaml encode: %v", err)
			}
			_ = enc.Close()

			goldenPath := filepath.Join("testdata", "values-chain-golden", version+".yaml")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read %s: %v (run with -update-golden to create)", goldenPath, err)
			}
			if !bytes.Equal(buf.Bytes(), want) {
				t.Errorf("values chain drifted for %s; inspect with -update-golden and git diff %s", version, goldenPath)
			}
		})
	}
}

func goldenForEntry(t *testing.T, repoRoot string, entry Entry) chainGolden {
	t.Helper()
	label := entry.Scenario + "/" + entry.Shortname + "/" + entry.Flow
	envFile := filepath.Join(t.TempDir(), "golden.env")
	flags, namespace, _, _, err := BuildEntryFlags(entry, RunOptions{
		RepoRoot:          repoRoot,
		Platform:          "gke",
		NamespaceOverride: "golden",
	})
	if err != nil {
		t.Fatalf("%s: BuildEntryFlags: %v", label, err)
	}
	flags.EnvFile = envFile
	flags.ExtraEnv = goldenEnv()
	flags.Docker.SkipDockerLogin = true
	flags.SkipPreflight = true
	prepared, err := deploy.PrepareScenario(context.Background(), &deploy.ScenarioContext{
		ScenarioName:             entry.Scenario,
		Namespace:                namespace,
		Release:                  "integration",
		IngressHost:              "golden.example.test",
		KeycloakRealm:            "golden-realm",
		OptimizeIndexPrefix:      "golden-optimize",
		OrchestrationIndexPrefix: "golden-orchestration",
		TasklistIndexPrefix:      "golden-tasklist",
		OperateIndexPrefix:       "golden-operate",
	}, flags)
	if err != nil {
		t.Fatalf("%s: PrepareScenario: %v", label, err)
	}
	t.Cleanup(prepared.Cleanup)

	merged, err := deploy.MergeYAMLFiles(prepared.ValuesFiles, filepath.Join(t.TempDir(), "merged.yaml"))
	if err != nil {
		t.Fatalf("%s: merge: %v", label, err)
	}
	mergedBytes, err := os.ReadFile(merged)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(mergedBytes)

	g := chainGolden{
		Scenario: entry.Scenario, Shortname: entry.Shortname, Flow: entry.Flow, Tier: entry.Tier,
		MergedSHA256: hex.EncodeToString(sum[:]),
	}
	g.Layers = prepared.LayeredFiles
	for _, file := range prepared.ValuesFiles {
		g.Chain = append(g.Chain, goldenPathLabel(repoRoot, prepared.TempDir, file))
	}
	if last := prepared.ValuesFiles[len(prepared.ValuesFiles)-1]; filepath.Base(last) == "values-resolved-images.yaml" {
		body, err := os.ReadFile(last)
		if err != nil {
			t.Fatal(err)
		}
		g.ResolvedImage = goldenPathLabel(repoRoot, prepared.TempDir, last)
		g.ResolvedBody = string(body)
	}
	return g
}

func goldenPathLabel(repoRoot, tempDir, file string) string {
	if rel, err := filepath.Rel(tempDir, file); err == nil && !strings.HasPrefix(rel, "..") {
		return "<tmp>/" + filepath.ToSlash(rel)
	}
	if rel, err := filepath.Rel(repoRoot, file); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return file
}

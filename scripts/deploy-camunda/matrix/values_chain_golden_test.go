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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/prepare-helm-values/pkg/placeholders"
	"scripts/prepare-helm-values/pkg/values"

	"gopkg.in/yaml.v3"
)

// Regenerate with:
//
//	cd scripts/deploy-camunda && go test ./matrix/ -run TestValuesChainGolden -update-golden
var valuesChainGoldenVersions = []string{"8.9", "8.10", "8.11"}

const valuesChainGoldenBaseDomain = "golden.example.com"

type valuesChainRecord struct {
	Entry                string   `yaml:"entry"`
	Layers               []string `yaml:"layers,omitempty"`
	Chain                []string `yaml:"chain"`
	ResolvedImages       string   `yaml:"resolvedImages,omitempty"`
	ResolvedImagesSHA256 string   `yaml:"resolvedImagesSha256,omitempty"`
	MergedValuesSHA256   string   `yaml:"mergedValuesSha256"`
}

// TestValuesChainGolden pins the ordered -f chain, the resolved-image file and
// the merged values of every tier-1/2 registry scenario.
func TestValuesChainGolden(t *testing.T) {
	repoRoot := findRepoRoot(t)
	t.Setenv("INFRA_INGRESS_HOSTNAME_BASE", valuesChainGoldenBaseDomain)
	for _, version := range valuesChainGoldenVersions {
		t.Run(version, func(t *testing.T) {
			chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+version)
			if !HasRegistry(chartDir) {
				t.Skipf("chart %s has no registry", version)
			}
			entries, err := Generate(repoRoot, GenerateOptions{Versions: []string{version}})
			if err != nil {
				t.Fatalf("Generate(%s): %v", version, err)
			}
			dummies := placeholderDummies(t, chartDir)
			var records []valuesChainRecord
			for _, entry := range entries {
				if entry.Tier != 1 && entry.Tier != 2 {
					continue
				}
				records = append(records, recordValuesChain(t, repoRoot, entry, dummies))
			}
			if len(records) == 0 {
				t.Fatalf("no tier-1/2 entries for %s", version)
			}
			compareValuesChainGolden(t, version, records)
		})
	}
}

func compareValuesChainGolden(t *testing.T, version string, records []valuesChainRecord) {
	t.Helper()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(records); err != nil {
		t.Fatalf("yaml encode: %v", err)
	}
	_ = enc.Close()
	got := buf.Bytes()
	goldenPath := filepath.Join("testdata", "values-chain", version+".yaml")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (rerun with -update-golden)", goldenPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: values chain drifted from %s; rerun with -update-golden and inspect the diff\n%s",
			version, goldenPath, firstDifference(string(want), string(got)))
	}
}

func recordValuesChain(t *testing.T, repoRoot string, entry Entry, dummies map[string]string) valuesChainRecord {
	t.Helper()
	key := fmt.Sprintf("%s/%s/%s/%s", entry.Shortname, entry.Scenario, entry.Flow, entry.Platform)
	flags, namespace, _, _, err := BuildEntryFlags(entry, RunOptions{RepoRoot: repoRoot, Config: &config.RootConfig{}})
	if err != nil {
		t.Fatalf("%s: BuildEntryFlags: %v", key, err)
	}
	flags.SkipPreflight = true
	flags.Secrets.AutoGenerateSecrets = false
	flags.Secrets.VaultSecretMapping = ""
	scenarioCtx := &deploy.ScenarioContext{
		ScenarioName:             entry.Scenario,
		Namespace:                namespace,
		Release:                  "integration",
		IngressHost:              flags.ResolveIngressHostname(),
		KeycloakRealm:            "golden-realm",
		OptimizeIndexPrefix:      "golden-opt",
		OrchestrationIndexPrefix: "golden-orch",
		TasklistIndexPrefix:      "golden-task",
		OperateIndexPrefix:       "golden-op",
	}
	envFile := filepath.Join(t.TempDir(), "golden.env")
	env := dummies
	for _, companion := range flags.CompanionCharts {
		if data, err := os.ReadFile(companion.ValuesFile); err == nil {
			env = withDummies(env, placeholders.Find(string(data)))
		}
	}
	for attempt := 0; ; attempt++ {
		writeDummyEnv(t, envFile, env)
		flags.EnvFile = envFile
		prepared, err := deploy.PrepareScenario(context.Background(), scenarioCtx, flags)
		if missing, names := values.IsMissingEnv(err); missing && attempt < 5 {
			env = withDummies(env, names)
			continue
		}
		if err != nil {
			t.Fatalf("%s: PrepareScenario: %v", key, err)
		}
		defer prepared.Cleanup()
		record := summarizeChain(t, key, repoRoot, prepared.TempDir, prepared.ValuesFiles)
		for _, layer := range prepared.LayeredFiles {
			record.Layers = append(record.Layers, filepath.ToSlash(layer))
		}
		return record
	}
}

func summarizeChain(t *testing.T, key, repoRoot, tempDir string, files []string) valuesChainRecord {
	t.Helper()
	record := valuesChainRecord{Entry: key}
	merged := map[string]any{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("%s: read %s: %v", key, file, err)
		}
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: parse %s: %v", key, file, err)
		}
		merged = helmMergeMaps(merged, doc)
		name := normalizeChainPath(file, repoRoot, tempDir)
		record.Chain = append(record.Chain, name)
		if filepath.Base(file) == "values-resolved-images.yaml" {
			record.ResolvedImages = name
			record.ResolvedImagesSHA256 = sha256Hex(data)
		}
	}
	out, err := yaml.Marshal(merged)
	if err != nil {
		t.Fatalf("%s: marshal merged: %v", key, err)
	}
	record.MergedValuesSHA256 = sha256Hex(out)
	return record
}

// helmMergeMaps mirrors helm's pkg/cli/values mergeMaps used for -f files.
func helmMergeMaps(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if vm, ok := v.(map[string]any); ok {
			if bv, ok := out[k].(map[string]any); ok {
				out[k] = helmMergeMaps(bv, vm)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func normalizeChainPath(file, repoRoot, tempDir string) string {
	if rel, err := filepath.Rel(tempDir, file); err == nil && !strings.HasPrefix(rel, "..") {
		return "$TMP/" + filepath.ToSlash(rel)
	}
	if rel, err := filepath.Rel(repoRoot, file); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(file)
}

// placeholderDummies maps every placeholder in the chart's values files to a
// generated dummy so the process environment cannot leak into the chain.
func placeholderDummies(t *testing.T, chartDir string) map[string]string {
	t.Helper()
	var names []string
	for _, root := range []string{chartDir} {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == "templates" || d.Name() == "charts" || d.Name() == "unit" {
					return filepath.SkipDir
				}
				return nil
			}
			if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err == nil {
				names = append(names, placeholders.Find(string(data))...)
			}
			return nil
		})
	}
	return withDummies(map[string]string{}, names)
}

func withDummies(env map[string]string, names []string) map[string]string {
	out := make(map[string]string, len(env)+len(names))
	for k, v := range env {
		out[k] = v
	}
	for _, name := range names {
		if _, ok := out[name]; !ok {
			out[name] = "dummy-" + strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		}
	}
	return out
}

func writeDummyEnv(t *testing.T, path string, env map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s=%s\n", k, env[k])
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func firstDifference(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("line %d:\n  want: %s\n  got:  %s", i+1, w, g)
		}
	}
	return ""
}

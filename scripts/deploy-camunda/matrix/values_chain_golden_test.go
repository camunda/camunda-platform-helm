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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/camunda-core/pkg/scenarios"
	"scripts/deploy-camunda/deploy"
)

// TestValuesChainGolden pins, for every enabled matrix entry of 8.9, 8.10 and
// 8.11, the ordered values chain BuildEntryFlags and prepareLayeredValues
// hand to Helm (chart-root overlays, scenario layers, scenario extra values)
// and the selection that produced it. Common values and the per-run debug
// and image-resolution files are generated at deploy time and not recorded.
//
//	cd scripts/deploy-camunda && go test ./matrix/ -run TestValuesChainGolden -update-golden
func TestValuesChainGolden(t *testing.T) {
	repoRoot := findRepoRoot(t)
	for _, version := range []string{"8.9", "8.10", "8.11"} {
		t.Run(version, func(t *testing.T) {
			entries, err := Generate(repoRoot, GenerateOptions{Versions: []string{version}})
			if err != nil {
				t.Fatalf("Generate(%s): %v", version, err)
			}
			var buf bytes.Buffer
			for _, entry := range entries {
				writeValuesChain(t, &buf, repoRoot, entry)
			}
			assertValuesChainGolden(t, filepath.Join("testdata", "values-chain", version+".golden"), buf.Bytes())
		})
	}
}

func writeValuesChain(t *testing.T, buf *bytes.Buffer, repoRoot string, entry Entry) {
	t.Helper()
	fmt.Fprintf(buf, "%s/%s flow=%s platform=%s\n", entry.Scenario, entry.Shortname, entry.Flow, entry.Platform)
	flags, _, _, _, err := BuildEntryFlags(entry, RunOptions{RepoRoot: repoRoot})
	if err != nil {
		fmt.Fprintf(buf, "  error: %v\n", err)
		return
	}
	scenarioDir := flags.Deployment.ScenarioPath
	cfg, err := scenarios.BuildDeploymentConfig(scenarioDir, entry.Scenario, scenarios.BuilderOverrides{
		Identity:    flags.Selection.Identity,
		Persistence: flags.Selection.Persistence,
		Platform:    flags.Deployment.Platform,
		Features:    flags.Selection.Features,
		InfraType:   flags.Selection.InfraType,
		Flow:        flags.Deployment.Flow,
		QA:          flags.Selection.QA,
		ImageTags:   flags.Selection.ImageTags,
		Upgrade:     flags.Selection.UpgradeFlow,
	})
	if err != nil {
		fmt.Fprintf(buf, "  error: %v\n", err)
		return
	}
	layers, err := cfg.ResolvePaths(scenarioDir)
	if err != nil {
		fmt.Fprintf(buf, "  error: %v\n", err)
		return
	}
	fmt.Fprintf(buf, "  identity=%s persistence=%s platform=%s infra=%s features=%s\n",
		cfg.Identity, cfg.Persistence, cfg.Platform, cfg.InfraType, strings.Join(cfg.Features, ","))
	overlays := deploy.ChartRootOverlayFiles(entry.ChartPath, flags.Chart.ChartRootOverlays)
	for _, f := range deploy.BuildValuesChain(nil, overlays, flags.Deployment.ExtraValues, layers, "") {
		rel, err := filepath.Rel(repoRoot, f)
		if err != nil {
			rel = f
		}
		fmt.Fprintf(buf, "  - %s\n", filepath.ToSlash(rel))
	}
}

func assertValuesChainGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (rerun with -update-golden)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s drifted; rerun with -update-golden and inspect the diff", path)
	}
}

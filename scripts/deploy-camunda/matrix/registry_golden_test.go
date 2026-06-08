// Copyright 2025 Camunda Services GmbH
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
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// updateGolden regenerates registry snapshots in place. Snapshots live at
// charts/camunda-platform-<v>/test/ci/registry-snapshot.yaml — co-located with
// the registry they pin so contributors can diff "what my registry edit
// produces" against the previous compiled CITestConfig view.
//
// Refresh via `make go.update-golden-only` (which chains both template
// goldens and registry snapshots) or `make go.update-registry-golden`
// standalone. Direct invocation:
//
//	cd scripts/deploy-camunda && go test ./matrix/ -run TestRegistryGolden -update-golden
//
// Inspect the per-version diff in charts/camunda-platform-*/test/ci/
// before committing.
var updateGolden = flag.Bool("update-golden", false, "regenerate registry snapshots")

// TestRegistryGolden pins LoadRegistry's CITestConfig output for every chart
// version that has a registry. Replaces the safety net provided by
// TestRegistryEquivalence once #6302 removes the legacy file + loader:
// after that point the equivalence test is gone, and only this snapshot
// catches a silent registry-loader regression that no unit test exercises
// (a field-tag rename, a fan-out off-by-one, a hook-cache miscompute that
// passes the cmp.Diff slice-equality check).
//
// Output format: YAML serialization of the *CITestConfig with stable
// map-key ordering (yaml.Marshal sorts map keys). Manifest scenario order
// is preserved by the loader, so post-fan-out PR.Scenarios is already
// deterministic — no normalize() pass needed.
func TestRegistryGolden(t *testing.T) {
	repoRoot := findRepoRoot(t)
	chartsDir := filepath.Join(repoRoot, "charts")
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		t.Fatalf("read charts dir: %v", err)
	}

	const prefix = "camunda-platform-"
	any := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		version := name[len(prefix):]
		chartDir := filepath.Join(chartsDir, name)
		if !HasRegistry(chartDir) {
			continue
		}
		any = true
		t.Run(version, func(t *testing.T) {
			cfg, err := LoadRegistry(chartDir)
			if err != nil {
				t.Fatalf("LoadRegistry(%s): %v", version, err)
			}
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(cfg); err != nil {
				t.Fatalf("yaml encode: %v", err)
			}
			_ = enc.Close()
			got := buf.Bytes()

			goldenPath := filepath.Join(chartDir, "test", "ci", "registry-snapshot.yaml")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("updated snapshot: %s (%d bytes)", goldenPath, len(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read snapshot %s: %v (run `make go.update-registry-golden` to create)", goldenPath, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s: registry snapshot drifted\n"+
					"  snapshot: %s\n"+
					"  to update: make go.update-golden-only (or make go.update-registry-golden)\n"+
					"  inspect the diff before committing",
					version, goldenPath)
			}
		})
	}
	if !any {
		t.Skip("no chart version has a registry yet")
	}
}

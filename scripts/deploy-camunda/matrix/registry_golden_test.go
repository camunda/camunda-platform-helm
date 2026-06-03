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
	"testing"

	"gopkg.in/yaml.v3"
)

// updateGolden regenerates registry golden snapshots in place. Run:
//
//	cd scripts/deploy-camunda && go test ./matrix/ -run TestRegistryGolden -update-golden
//
// Inspect the diff in `testdata/golden/registry/` before committing.
var updateGolden = flag.Bool("update-golden", false, "regenerate registry golden snapshots")

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
		if !filepathHasPrefix(name, prefix) {
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

			goldenPath := filepath.Join("testdata", "golden", "registry", version+".yaml")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("updated golden: %s (%d bytes)", goldenPath, len(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with -update-golden to create)", goldenPath, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s: registry snapshot drifted from golden\n"+
					"  golden: %s\n"+
					"  to update: cd scripts/deploy-camunda && go test ./matrix/ -run TestRegistryGolden -update-golden\n"+
					"  inspect the diff before committing",
					version, goldenPath)
			}
		})
	}
	if !any {
		t.Skip("no chart version has a registry yet")
	}
}

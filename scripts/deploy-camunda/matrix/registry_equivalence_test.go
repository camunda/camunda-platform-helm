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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestRegistryEquivalence is the one-shot migration gate from ADR 0093 §4:
// for every chart version that has both a legacy ci-test-config.yaml and a
// composable registry under test/ci/registry/, the two loaders must produce
// equal *CITestConfig values. Removed in the follow-up that deletes
// LoadCITestConfig and the frozen legacy files.
//
// Comparison rules: empty/nil slice equivalence (yaml-omitempty round-trip
// erases the distinction), and the post-fan-out PR.Scenarios list is sorted
// by a stable key before diff — only the *contents* of the matrix matter,
// not the order, and the runtime callers don't depend on slice order across
// load paths.
func TestRegistryEquivalence(t *testing.T) {
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
			continue // pre-migration version; nothing to compare yet.
		}
		// Legacy file must still exist at this point (frozen snapshot per ADR §5).
		if _, err := os.Stat(filepath.Join(chartDir, "test", "ci-test-config.yaml")); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				t.Logf("%s: ci-test-config.yaml already deleted; skipping equivalence", version)
				continue
			}
			t.Errorf("%s: stat ci-test-config.yaml: %v", version, err)
			continue
		}
		any = true
		t.Run(version, func(t *testing.T) {
			legacy, err := LoadCITestConfig(chartDir)
			if err != nil {
				t.Fatalf("LoadCITestConfig: %v", err)
			}
			registry, err := LoadRegistry(chartDir)
			if err != nil {
				t.Fatalf("LoadRegistry: %v", err)
			}

			normalize(legacy)
			normalize(registry)

			if diff := cmp.Diff(legacy, registry, equivalenceOpts()...); diff != "" {
				t.Errorf("legacy vs registry CITestConfig mismatch (-legacy +registry):\n%s", diff)
			}
		})
	}
	if !any {
		t.Skip("no chart version has a registry yet")
	}
}

// equivalenceOpts captures the comparison rules: empty vs nil slice equivalence
// (yaml-omitempty erases that distinction across a round trip), plus we ignore
// CITestConfig.Integration.DependencyProfiles. The registry stores fully-
// inlined dependencies per scenario, so DependencyProfiles is unused on the
// registry path; the legacy path populates the map but ResolveProfiles already
// expanded those references into each scenario's Dependencies, making the map
// informational from the loader's perspective. matrix.Generate never consults
// it after load, so divergence is harmless.
func equivalenceOpts() []cmp.Option {
	type integration struct{} // sentinel for the anonymous struct path; not used directly.
	_ = integration{}
	return []cmp.Option{
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(CITestConfig{}.Integration, "DependencyProfiles"),
	}
}

// normalize stably sorts the post-fan-out PR scenario list so the
// equivalence check is order-independent. Sort key includes Name, Shortname,
// Flow, and the first platform; that is enough to disambiguate every legacy
// entry (no two legacy CIScenarios share all four).
func normalize(cfg *CITestConfig) {
	scns := cfg.Integration.Case.PR.Scenarios
	sort.SliceStable(scns, func(i, j int) bool {
		a, b := scns[i], scns[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Shortname != b.Shortname {
			return a.Shortname < b.Shortname
		}
		if a.Flow != b.Flow {
			return a.Flow < b.Flow
		}
		ap, bp := firstPlatform(a.Platforms), firstPlatform(b.Platforms)
		return ap < bp
	})
}

func firstPlatform(p []string) string {
	if len(p) == 0 {
		return ""
	}
	return p[0]
}

// filepathHasPrefix is a small wrapper around strings.HasPrefix so the
// equivalence test compiles without importing strings just for one call.
func filepathHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

package matrix

import (
	"path/filepath"
	"testing"
)

// registryGoodRepoRoot is the fake repo root whose charts/ holds the testdata
// registry chart (camunda-platform-99.99).
const registryGoodRepoRoot = "testdata/registry-good"

func TestResolveE2ESuiteFromRegistry(t *testing.T) {
	root, err := filepath.Abs(registryGoodRepoRoot)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	for _, tc := range []struct {
		scenario string
		want     E2ESuite
	}{
		{"beta", E2ESuite{FullSuite: true, NonBlocking: true}},
		{"gamma", E2ESuite{FullSuite: true}},
		{"alpha", E2ESuite{}},
		{"not-a-scenario", E2ESuite{}},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			got, err := ResolveE2ESuite(root, "camunda-platform-99.99", tc.scenario)
			if err != nil {
				t.Fatalf("ResolveE2ESuite: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolveE2ESuite(%q) = %+v, want %+v", tc.scenario, got, tc.want)
			}
		})
	}
}

// A chart version without a registry keeps the historical smoke/blocking
// behavior instead of failing the resolution step.
func TestResolveE2ESuiteWithoutRegistry(t *testing.T) {
	got, err := ResolveE2ESuite(t.TempDir(), "camunda-platform-8.6", "alwaysgreen")
	if err != nil {
		t.Fatalf("ResolveE2ESuite: %v", err)
	}
	if got != (E2ESuite{}) {
		t.Errorf("ResolveE2ESuite = %+v, want zero value", got)
	}
}

// The real registries drive the AlwaysGreen gate: alwaysgreen runs the full
// suite on every active version and blocks, while a smoke scenario does not.
func TestResolveE2ESuiteAlwaysgreenAcrossVersions(t *testing.T) {
	repoRoot := findRepoRoot(t)
	for _, version := range planActiveVersions {
		chartDir := "camunda-platform-" + version
		got, err := ResolveE2ESuite(repoRoot, chartDir, "alwaysgreen")
		if err != nil {
			t.Fatalf("%s: ResolveE2ESuite: %v", version, err)
		}
		if want := (E2ESuite{FullSuite: true}); got != want {
			t.Errorf("%s: alwaysgreen = %+v, want %+v", version, got, want)
		}

		smoke, err := ResolveE2ESuite(repoRoot, chartDir, "elasticsearch")
		if err != nil {
			t.Fatalf("%s: ResolveE2ESuite: %v", version, err)
		}
		if smoke != (E2ESuite{}) {
			t.Errorf("%s: elasticsearch = %+v, want zero value", version, smoke)
		}
	}
}

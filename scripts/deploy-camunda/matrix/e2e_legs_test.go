package matrix

import (
	"path/filepath"
	"reflect"
	"testing"
)

// registryGoodRepoRoot is the fake repo root whose charts/ holds the testdata
// registry chart (camunda-platform-99.99).
const registryGoodRepoRoot = "testdata/registry-good"

func smokeLeg(blocking bool) E2ELeg {
	return E2ELeg{Suite: SuiteSmoke, Blocking: blocking, ShardIndex: 1, ShardTotal: 1}
}

func fullLeg(blocking bool) E2ELeg {
	return E2ELeg{Suite: SuiteFull, Blocking: blocking, ShardIndex: 1, ShardTotal: 1}
}

func TestResolveE2ELegsFromRegistry(t *testing.T) {
	root, err := filepath.Abs(registryGoodRepoRoot)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	for _, tc := range []struct {
		name     string
		scenario string
		want     []E2ELeg
	}{
		// alpha declares nothing: the historical single blocking smoke leg.
		{"defaults to blocking smoke", "alpha", []E2ELeg{smokeLeg(true)}},
		// beta inverts both blocking defaults, proving the *bool overrides apply
		// in both directions rather than only turning blocking off.
		{"honors both blocking overrides", "beta", []E2ELeg{smokeLeg(false), fullLeg(true)}},
		// gamma opts into the full suite and leaves blocking alone.
		{"full suite defaults to non-blocking", "gamma", []E2ELeg{smokeLeg(true), fullLeg(false)}},
		{"unknown scenario falls back", "not-a-scenario", []E2ELeg{smokeLeg(true)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveE2ELegs(root, "camunda-platform-99.99", "", tc.scenario)
			if err != nil {
				t.Fatalf("ResolveE2ELegs: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveE2ELegs(%q) = %+v, want %+v", tc.scenario, got, tc.want)
			}
		})
	}
}

// Smoke must always be the first leg: the workflow serializes legs with
// max-parallel 1, so ordering decides which suite reports first.
func TestResolveE2ELegsOrdersSmokeFirst(t *testing.T) {
	root, err := filepath.Abs(registryGoodRepoRoot)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	legs, err := ResolveE2ELegs(root, "camunda-platform-99.99", "", "gamma")
	if err != nil {
		t.Fatalf("ResolveE2ELegs: %v", err)
	}
	if len(legs) != 2 || legs[0].Suite != SuiteSmoke || legs[1].Suite != SuiteFull {
		t.Errorf("legs = %+v, want smoke then full", legs)
	}
}

// A chart version without a registry keeps the historical blocking smoke leg
// instead of failing the resolution step or silently dropping e2e.
func TestResolveE2ELegsWithoutRegistry(t *testing.T) {
	got, err := ResolveE2ELegs(t.TempDir(), "camunda-platform-8.6", "agrn", "alwaysgreen")
	if err != nil {
		t.Fatalf("ResolveE2ELegs: %v", err)
	}
	if want := []E2ELeg{smokeLeg(true)}; !reflect.DeepEqual(got, want) {
		t.Errorf("ResolveE2ELegs = %+v, want %+v", got, want)
	}
}

// skip-e2e yields no legs, which GitHub Actions renders as a skipped job.
// E2ELegsJSON must still emit "[]" and never "null".
func TestResolveE2ELegsSkipE2E(t *testing.T) {
	repoRoot := findRepoRoot(t)
	got, err := ResolveE2ELegs(repoRoot, "camunda-platform-8.9", "oidc", "oidc")
	if err != nil {
		t.Fatalf("ResolveE2ELegs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ResolveE2ELegs = %+v, want no legs for a skip-e2e scenario", got)
	}
	legsJSON, err := E2ELegsJSON(got)
	if err != nil {
		t.Fatalf("E2ELegsJSON: %v", err)
	}
	if legsJSON != "[]" {
		t.Errorf("E2ELegsJSON = %q, want []", legsJSON)
	}
}

// The real registries drive the AlwaysGreen gate: alwaysgreen runs a blocking
// smoke leg plus a non-blocking full leg on every active version.
func TestResolveE2ELegsAlwaysgreenAcrossVersions(t *testing.T) {
	repoRoot := findRepoRoot(t)
	for _, version := range planActiveVersions {
		got, err := ResolveE2ELegs(repoRoot, "camunda-platform-"+version, "agrn", "alwaysgreen")
		if err != nil {
			t.Fatalf("%s: ResolveE2ELegs: %v", version, err)
		}
		if want := []E2ELeg{smokeLeg(true), fullLeg(false)}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: alwaysgreen = %+v, want %+v", version, got, want)
		}
	}
}

// Every scenario in every real registry must resolve to legs matching its own
// declarations — no legs when skip-e2e, a blocking smoke leg otherwise, plus a
// full leg only when opted in. Asserting the invariant rather than naming
// scenarios keeps this honest as the registries change.
func TestResolveE2ELegsMatchRealRegistryDeclarations(t *testing.T) {
	repoRoot := findRepoRoot(t)
	for _, version := range planActiveVersions {
		chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+version)
		cfg, err := LoadRegistry(chartDir)
		if err != nil {
			t.Fatalf("%s: LoadRegistry: %v", version, err)
		}

		for _, scn := range cfg.Integration.Case.PR.Scenarios {
			want := []E2ELeg{}
			if !scn.SkipE2E {
				want = append(want, smokeLeg(e2eBlocking(scn.E2ESmokeBlocking, defaultSmokeBlocking)))
				if scn.E2EFullSuite {
					want = append(want, fullLeg(e2eBlocking(scn.E2EFullSuiteBlocking, defaultFullBlocking)))
				}
			}

			got, err := ResolveE2ELegs(repoRoot, "camunda-platform-"+version, scn.Shortname, scn.Name)
			if err != nil {
				t.Fatalf("%s/%s: ResolveE2ELegs: %v", version, scn.Name, err)
			}
			if len(got) == 0 && len(want) == 0 {
				continue
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s/%s: legs = %+v, want %+v (skip-e2e=%v full-suite=%v)",
					version, scn.Name, got, want, scn.SkipE2E, scn.E2EFullSuite)
			}
		}
	}
}

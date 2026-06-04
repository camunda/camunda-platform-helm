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

//go:build migrate

// One-shot translator from ci-test-config.yaml to the composable scenario
// registry layout introduced by ADR 0093. Compiled only under the `migrate`
// build tag so the normal Go test suite never picks it up. Deleted in the
// same follow-up (#6302) that removes LoadCITestConfig and the frozen legacy
// files.
//
// Usage (run from this package directory):
//
//   go test -tags=migrate -run=TestMigrate \
//       -chartDir=../../../charts/camunda-platform-8.10 ./
//
// Pass any active chart version via -chartDir. The legacy file is read via
// LoadCITestConfig, which applies ResolveProfiles (the #6288 dependency-
// profiles expansion) before the translator sees the data — so the emitted
// registry stores fully-inlined dependencies regardless of which authoring
// style the legacy file used.
//
// After running, eyeball the generated tree under
//   <chartDir>/test/ci/registry/
// and run TestRegistryEquivalence against it to confirm round-trip equality
// with the legacy file.

package matrix

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var migrateChartDir = flag.String("chartDir", "", "absolute or repo-relative path to charts/camunda-platform-X.Y")

// outScenario is the registry on-disk shape. Field order here drives the YAML
// output order; `omitempty` lets the round-trip stay clean against legacy
// scenarios that omit defaults.
type outScenario struct {
	Name         string            `yaml:"name"`
	Auth         string            `yaml:"auth,omitempty"`
	Flows        []string          `yaml:"flows"`
	Identity     string            `yaml:"identity,omitempty"`
	Persistence  string            `yaml:"persistence,omitempty"`
	Features     []string          `yaml:"features,omitempty"`
	Platforms    []string          `yaml:"platforms,omitempty"`
	Exclude      []string          `yaml:"exclude,omitempty"`
	InfraType    map[string]string `yaml:"infra-type,omitempty"`
	QA           bool              `yaml:"qa,omitempty"`
	ImageTags    bool              `yaml:"image-tags,omitempty"`
	Upgrade      bool              `yaml:"upgrade,omitempty"`
	Enterprise   bool              `yaml:"enterprise,omitempty"`
	HelmVersion  string            `yaml:"helmVersion,omitempty"`
	SkipE2E      bool              `yaml:"skip-e2e,omitempty"`
	SkipIT       bool              `yaml:"skip-it,omitempty"`
	PrefixKey    string            `yaml:"prefix-key,omitempty"`
	PreInstall   string            `yaml:"pre-install,omitempty"`
	PostDeploy   string            `yaml:"post-deploy,omitempty"`
	Dependencies []string          `yaml:"dependencies,omitempty"`
}

type outManifestEntry struct {
	ID        string `yaml:"id"`
	Shortname string `yaml:"shortname"`
	Tier      int    `yaml:"tier,omitempty"`
	Enabled   bool   `yaml:"enabled"`
}

type outManifest struct {
	Integration struct {
		Vars      any                   `yaml:"vars"`
		Flows     map[string]*FlowHooks `yaml:"flows,omitempty"`
		Scenarios []outManifestEntry    `yaml:"scenarios"`
	} `yaml:"integration"`
}

// dedup tracks unique values by content hash, preserving insertion order so
// the emitted directory contents are deterministic across translator runs.
type dedup[T any] struct {
	bySlug map[string]T
	byHash map[string]string // content-hash -> slug
	order  []string
}

func newDedup[T any]() *dedup[T] {
	return &dedup[T]{bySlug: map[string]T{}, byHash: map[string]string{}}
}

// put returns the slug assigned to value. If hash was seen before, the prior
// slug is returned (no duplicate file). Otherwise the requested slug is used,
// with a numeric suffix appended on collision.
func (d *dedup[T]) put(hash, slug string, value T) string {
	if existing, ok := d.byHash[hash]; ok {
		return existing
	}
	final := slug
	for i := 2; ; i++ {
		if _, taken := d.bySlug[final]; !taken {
			break
		}
		final = fmt.Sprintf("%s-%d", slug, i)
	}
	d.bySlug[final] = value
	d.byHash[hash] = final
	d.order = append(d.order, final)
	return final
}

func hashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("hashJSON: %v", err))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hookSlug derives a human-readable slug from a LifecycleHook payload.
//
//   - Script-based: drop the well-known `pre-install-` / `post-deploy-`
//     prefix and `.sh` suffix so the slug matches what an author would write.
//   - Single-fixture: when the fixture is `postgresql-cluster.yaml`, map to
//     the `cnpg` family established by #6288 (`cnpg` default, `cnpg-rdbms`
//     for the three-database bootstrap). Other fixtures keep the
//     `-self-signed` / `-default` derivation.
//   - Multi-fixture: when `postgresql-cluster.yaml` is one of the fixtures,
//     extend the `cnpg-` family using the other fixture basenames (with any
//     `postgresql` segments trimmed). Otherwise join basenames with `-`.
//   - Empty (no script, no fixtures) is invalid per LifecycleHook.Validate
//     and never reaches the translator; fall back to "hook" so the call
//     remains total.
func hookSlug(h *LifecycleHook) string {
	if h.Script != "" {
		s := strings.TrimSuffix(h.Script, ".sh")
		s = strings.TrimPrefix(s, "pre-install-")
		s = strings.TrimPrefix(s, "post-deploy-")
		return s
	}
	const pgClusterFixture = "postgresql-cluster.yaml"
	if len(h.Fixtures) == 1 {
		if h.Fixtures[0] == pgClusterFixture {
			if strings.Contains(h.Description, "three databases:") &&
				strings.Contains(h.Description, "`app`") {
				return "cnpg-rdbms"
			}
			return "cnpg"
		}
		base := strings.TrimSuffix(h.Fixtures[0], ".yaml")
		switch {
		case strings.Contains(h.Description, "self-signed CA"):
			return base + "-self-signed"
		default:
			return base + "-default"
		}
	}
	if len(h.Fixtures) > 1 {
		hasPgCluster := false
		var others []string
		for _, f := range h.Fixtures {
			if f == pgClusterFixture {
				hasPgCluster = true
				continue
			}
			others = append(others, trimPostgresqlSegments(strings.TrimSuffix(f, ".yaml")))
		}
		if hasPgCluster && len(others) > 0 {
			return "cnpg-" + strings.Join(others, "-")
		}
		parts := make([]string, 0, len(h.Fixtures))
		for _, f := range h.Fixtures {
			parts = append(parts, strings.TrimSuffix(f, ".yaml"))
		}
		return strings.Join(parts, "-")
	}
	return "hook"
}

// trimPostgresqlSegments drops leading/trailing `postgresql` word segments so
// a multi-fixture cnpg companion (e.g. `hub-external-postgresql.yaml`)
// contributes its semantic tag (`hub-external`) without the redundant word —
// `cnpg-` already conveys the PostgreSQL role.
func trimPostgresqlSegments(s string) string {
	for {
		switch {
		case strings.HasPrefix(s, "postgresql-"):
			s = strings.TrimPrefix(s, "postgresql-")
		case strings.HasSuffix(s, "-postgresql"):
			s = strings.TrimSuffix(s, "-postgresql")
		default:
			return s
		}
	}
}

// depSlug derives a slug from a ChartDependency. Base is the release name;
// the pinned version lives in the dep YAML's `version:` field, not the
// filename. A `-qa` or `-tls` suffix is appended when the values-file
// basename signals a variant.
func depSlug(d ChartDependency) string {
	name := d.ReleaseName
	switch {
	case strings.HasSuffix(d.ValuesFile, "-qa.yaml"):
		name = name + "-qa"
	case strings.HasSuffix(d.ValuesFile, "-tls.yaml"):
		name = name + "-tls"
	}
	return name
}

// writeYAML marshals v with 2-space indent and writes it to path. Buffered
// through a string builder so the file write is atomic from the caller's
// perspective (the yaml encoder otherwise emits incrementally).
func writeYAML(path string, v any) error {
	var buf strings.Builder
	enc := yaml.NewEncoder(&yamlStringWriter{&buf})
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(buf.String()), 0644)
}

type yamlStringWriter struct{ sb *strings.Builder }

func (w *yamlStringWriter) Write(p []byte) (int, error) {
	w.sb.Write(p)
	return len(p), nil
}

// sanitize maps a free-form name to a filesystem-safe slug: lowercase ASCII,
// alphanumeric + `-` + `_` only. CamelCase is split (`noSecondaryStorage` →
// `no-secondary-storage`). Spaces and slashes become `-`; all other runes
// are dropped.
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	var withHyphens strings.Builder
	var prev rune
	for _, r := range s {
		isUpper := r >= 'A' && r <= 'Z'
		prevIsLower := prev >= 'a' && prev <= 'z'
		prevIsDigit := prev >= '0' && prev <= '9'
		if isUpper && (prevIsLower || prevIsDigit) {
			withHyphens.WriteRune('-')
		}
		withHyphens.WriteRune(r)
		prev = r
	}
	s = strings.ToLower(withHyphens.String())
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ', r == '/':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}

// flowSlug maps full flow names to short, human-readable suffixes used in
// scenario IDs when a scenario name collides across multiple flow values.
// Unknown flows are emitted with hyphens stripped so they still produce a
// valid slug; the empty flow returns the empty string (callers treat that
// as "this scenario does not contribute to the flow axis").
func flowSlug(f string) string {
	switch f {
	case "":
		return ""
	case "install":
		return "install"
	case "upgrade-minor":
		return "upgrade"
	case "modular-upgrade-minor":
		return "modular"
	case "upgrade-patch":
		return "patch"
	default:
		return strings.ReplaceAll(f, "-", "")
	}
}

// axisKey builds a per-scenario disambiguation key from the requested axes.
// Each axis contributes a part only when the scenario actually carries that
// dimension (flow non-empty, platforms non-empty, tier > 0). Empty parts are
// dropped so a `{flow,tier}` subset still produces "upgrade" for a scenario
// that has flow but no tier set. Returns "" when the scenario contributes
// nothing to any requested axis — callers treat that as "subset invalid for
// this scenario" and try the next subset.
func axisKey(scn CIScenario, axes []string) string {
	parts := []string{}
	for _, a := range axes {
		switch a {
		case "flow":
			if f := flowSlug(scn.Flow); f != "" {
				parts = append(parts, f)
			}
		case "platform":
			if len(scn.Platforms) > 0 {
				parts = append(parts, scn.Platforms[0])
			}
		case "tier":
			if scn.Tier > 0 {
				parts = append(parts, fmt.Sprintf("tier%d", scn.Tier))
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}

// assignScenarioIDs picks a deterministic, human-readable ID per scenario by
// grouping by sanitize(Name) and, for any group of size > 1, finding the
// smallest subset of disambiguation axes (flow, platform, tier) that
// produces distinct non-empty keys across the group. The chosen axis
// combination is applied uniformly to every scenario in the group, so two
// scenarios in the same group share the same dimension labels (e.g. both
// gain a flow suffix, not one gaining flow and the other gaining platform).
//
// Subset order is tuned for legacy collision shapes seen across 8.7-8.10:
// flow first (most common axis), then platform, then tier, then their
// pairwise combinations, then the triple. Unique-name scenarios skip
// disambiguation and use the plain sanitized name. A pathological group
// that cannot be disambiguated by any subset falls back to a numeric
// suffix; this never fires on current chart data but is kept for safety.
func assignScenarioIDs(scenarios []CIScenario) []string {
	ids := make([]string, len(scenarios))
	groups := map[string][]int{}
	for i, s := range scenarios {
		groups[sanitize(s.Name)] = append(groups[sanitize(s.Name)], i)
	}

	subsetOrder := [][]string{
		{"flow"},
		{"platform"},
		{"tier"},
		{"flow", "platform"},
		{"flow", "tier"},
		{"platform", "tier"},
		{"flow", "platform", "tier"},
	}

	for name, idxs := range groups {
		if len(idxs) == 1 {
			ids[idxs[0]] = name
			continue
		}
		assigned := false
		for _, subset := range subsetOrder {
			keys := make([]string, len(idxs))
			validSubset := true
			seen := map[string]bool{}
			for k, i := range idxs {
				key := axisKey(scenarios[i], subset)
				if key == "" || seen[key] {
					validSubset = false
					break
				}
				seen[key] = true
				keys[k] = key
			}
			if !validSubset {
				continue
			}
			for k, i := range idxs {
				ids[i] = name + "-" + keys[k]
			}
			assigned = true
			break
		}
		if !assigned {
			for k, i := range idxs {
				if k == 0 {
					ids[i] = name
				} else {
					ids[i] = fmt.Sprintf("%s-%d", name, k+1)
				}
			}
		}
	}
	return ids
}

// TestMigrate8_10 reads the legacy ci-test-config.yaml from -chartDir and
// emits the composable registry under <chartDir>/test/ci/registry/. The
// translator is intentionally non-destructive — it overwrites existing files
// rather than diffing, so re-running it is the supported way to refresh the
// registry against changes in the legacy file.
//
// For 8.9 / 8.8 / 8.7 migrations, copy this function with a per-version name
// (TestMigrate8_9, etc.) or just re-run this one with the appropriate
// -chartDir flag — Go test functions are not required to match the chart
// version name. Once the equivalence test holds green for every version,
// this file is deleted in #6302.
func TestMigrate8_10(t *testing.T) {
	if *migrateChartDir == "" {
		t.Fatal("set -chartDir on the test command (e.g. -chartDir=charts/camunda-platform-8.10)")
	}
	chartDir, err := filepath.Abs(*migrateChartDir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	cfg, err := LoadCITestConfig(chartDir)
	if err != nil {
		t.Fatalf("LoadCITestConfig: %v", err)
	}

	registryDir := filepath.Join(chartDir, "test", "ci", "registry")
	for _, sub := range []string{"scenarios", "hooks", "dependencies"} {
		if err := os.MkdirAll(filepath.Join(registryDir, sub), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}

	hooks := newDedup[*LifecycleHook]()
	deps := newDedup[ChartDependency]()
	scenarioIDs := assignScenarioIDs(cfg.Integration.Case.PR.Scenarios)
	var entries []outManifestEntry

	hookID := func(h *LifecycleHook) string {
		if h == nil {
			return ""
		}
		return hooks.put(hashJSON(h), hookSlug(h), h)
	}
	depID := func(d ChartDependency) string {
		return deps.put(hashJSON(d), depSlug(d), d)
	}

	for i, scn := range cfg.Integration.Case.PR.Scenarios {
		id := scenarioIDs[i]
		out := outScenario{
			Name:        scn.Name,
			Auth:        scn.Auth,
			Flows:       []string{scn.Flow},
			Identity:    scn.Identity,
			Persistence: scn.Persistence,
			Features:    scn.Features,
			Platforms:   scn.Platforms,
			Exclude:     scn.Exclude,
			InfraType:   scn.InfraType,
			QA:          scn.QA,
			ImageTags:   scn.ImageTags,
			Upgrade:     scn.Upgrade,
			Enterprise:  scn.Enterprise,
			HelmVersion: scn.HelmVersion,
			SkipE2E:     scn.SkipE2E,
			SkipIT:      scn.SkipIT,
			PrefixKey:   scn.PrefixKey,
			PreInstall:  hookID(scn.PreInstall),
			PostDeploy:  hookID(scn.PostDeploy),
		}
		for _, d := range scn.Dependencies {
			out.Dependencies = append(out.Dependencies, depID(d))
		}

		if err := writeYAML(filepath.Join(registryDir, "scenarios", id+".yaml"), out); err != nil {
			t.Fatalf("write scenario %s: %v", id, err)
		}
		entries = append(entries, outManifestEntry{ID: id, Shortname: scn.Shortname, Tier: scn.Tier, Enabled: scn.Enabled})
	}

	for _, slug := range hooks.order {
		if err := writeYAML(filepath.Join(registryDir, "hooks", slug+".yaml"), hooks.bySlug[slug]); err != nil {
			t.Fatalf("write hook %s: %v", slug, err)
		}
	}
	for _, slug := range deps.order {
		if err := writeYAML(filepath.Join(registryDir, "dependencies", slug+".yaml"), deps.bySlug[slug]); err != nil {
			t.Fatalf("write dep %s: %v", slug, err)
		}
	}

	var manifest outManifest
	manifest.Integration.Vars = cfg.Integration.Vars
	manifest.Integration.Flows = cfg.Integration.Flows
	manifest.Integration.Scenarios = entries
	if err := writeYAML(filepath.Join(registryDir, "manifest.yaml"), manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	sort.Strings(hooks.order)
	sort.Strings(deps.order)
	t.Logf("wrote %d scenarios, %d hooks (%v), %d deps (%v)",
		len(entries), len(hooks.order), hooks.order, len(deps.order), deps.order)
}

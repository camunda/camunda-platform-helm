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
	Shortname    string            `yaml:"shortname"`
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
	ID      string `yaml:"id"`
	Tier    int    `yaml:"tier,omitempty"`
	Enabled bool   `yaml:"enabled"`
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
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hookSlug derives a human-readable slug from a LifecycleHook payload.
//
//   - Script-based: drop the well-known `pre-install-` / `post-deploy-`
//     prefix and `.sh` suffix so the slug matches what an author would write.
//   - Single-fixture: append a `-rdbms`, `-self-signed`, or `-default`
//     suffix based on description keywords so the popular
//     `postgresql-cluster.yaml` fixture distinguishes its variants.
//   - Multi-fixture: join fixture basenames with `-` so multi-fixture hooks
//     have a unique, predictable slug.
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
	if len(h.Fixtures) == 1 {
		base := strings.TrimSuffix(h.Fixtures[0], ".yaml")
		switch {
		case strings.Contains(h.Description, "three databases:") &&
			strings.Contains(h.Description, "`app`"):
			return base + "-rdbms"
		case strings.Contains(h.Description, "self-signed CA"):
			return base + "-self-signed"
		default:
			return base + "-default"
		}
	}
	if len(h.Fixtures) > 1 {
		parts := make([]string, 0, len(h.Fixtures))
		for _, f := range h.Fixtures {
			parts = append(parts, strings.TrimSuffix(f, ".yaml"))
		}
		return strings.Join(parts, "-")
	}
	return "hook"
}

// depSlug derives a slug from a ChartDependency. Base is the release name
// optionally suffixed by version (so `elastic-elasticsearch-8.5.1`
// distinguishes from a hypothetical future bump). A `-qa` or `-tls` suffix
// is appended when the values-file basename signals a variant.
func depSlug(d ChartDependency) string {
	name := d.ReleaseName
	if d.Version != "" {
		name = name + "-" + d.Version
	}
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
// alphanumeric + `-` + `_` only. Spaces and slashes become `-`; all other
// runes are dropped.
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
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

// uniqueScenarioID picks the shortest readable ID that is still unique among
// already-assigned IDs. Strategy: `name` → `name-shortname` →
// `name-shortname-flow` → numeric suffix. Reviewers should hand-rename ugly
// numeric-suffix IDs after the first run when the underlying collision is
// expected to persist.
func uniqueScenarioID(taken map[string]bool, name, shortname, flow string) string {
	candidates := []string{
		sanitize(name),
		sanitize(name + "-" + shortname),
		sanitize(name + "-" + shortname + "-" + flow),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if !taken[c] {
			taken[c] = true
			return c
		}
	}
	base := sanitize(name + "-" + shortname + "-" + flow)
	if base == "" {
		base = "scenario"
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s-%d", base, i)
		if !taken[c] {
			taken[c] = true
			return c
		}
	}
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
	takenIDs := map[string]bool{}
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

	for _, scn := range cfg.Integration.Case.PR.Scenarios {
		id := uniqueScenarioID(takenIDs, scn.Name, scn.Shortname, scn.Flow)
		out := outScenario{
			Name:        scn.Name,
			Shortname:   scn.Shortname,
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
		entries = append(entries, outManifestEntry{ID: id, Tier: scn.Tier, Enabled: scn.Enabled})
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

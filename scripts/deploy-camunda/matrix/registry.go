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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"

	"gopkg.in/yaml.v3"
)

// RegistryDirName is the directory under <chartDir>/test/ that holds the
// composable CI scenario registry described by ADR 0093. Presence of
// <chartDir>/test/<RegistryDirName>/manifest.yaml is the runtime signal
// that the loader should read the registry instead of ci-test-config.yaml.
const RegistryDirName = "ci/registry"

// registryManifest is the parsed shape of <registry>/manifest.yaml.
// It carries the ordered list of scenario IDs plus the non-scenario blocks
// (integration.vars and integration.flows) that the legacy ci-test-config.yaml
// kept colocated with the scenarios.
type registryManifest struct {
	Integration struct {
		Vars struct {
			TasksBaseDir  string `yaml:"tasksBaseDir"`
			ValuesBaseDir string `yaml:"valuesBaseDir"`
			ChartsBaseDir string `yaml:"chartsBaseDir"`
		} `yaml:"vars"`
		Flows     map[string]*FlowHooks   `yaml:"flows,omitempty"`
		Scenarios []registryManifestEntry `yaml:"scenarios"`
	} `yaml:"integration"`
}

// registryManifestEntry is one row in the manifest's ordered scenario list.
// It carries the manifest-scoped fields: ID (resolves to scenarios/<id>.yaml),
// Shortname (the human-facing CLI/CI handle — same surface as `--shortname-filter`
// and the K8s namespace fragment), Tier (curation), and Enabled.
type registryManifestEntry struct {
	ID        string `yaml:"id"`
	Shortname string `yaml:"shortname"`
	Tier      int    `yaml:"tier,omitempty"`
	Enabled   bool   `yaml:"enabled"`
}

// validateManifestTier accepts tier 1 and 2, plus an absent tier (0) on a
// disabled entry. An enabled entry must declare a tier: an untiered entry
// matches no --tier filter, so it is reachable only through the unfiltered
// matrix and never through tier-scoped selection.
func validateManifestTier(entry registryManifestEntry) error {
	switch entry.Tier {
	case 1, 2:
		return nil
	case 0:
		if !entry.Enabled {
			return nil
		}
		return fmt.Errorf("manifest scenario %q is enabled but declares no tier: set tier 1 (PR CI) or 2 (merge-queue only)", entry.ID)
	default:
		return fmt.Errorf("manifest scenario %q declares tier %d: supported values are 1 (PR CI) and 2 (merge-queue only)", entry.ID, entry.Tier)
	}
}

// registryScenario is the parsed shape of <registry>/scenarios/<id>.yaml.
// Mirrors CIScenario field-for-field except:
//   - Flow is plural (Flows) — the loader fans out to N CIScenario entries.
//   - PreInstall, PostDeploy carry hook *IDs* (basenames under hooks/).
//   - Dependencies carries dep *IDs* (basenames under dependencies/).
type registryScenario struct {
	Name        string            `yaml:"name"`
	Auth        string            `yaml:"auth"`
	Flows       []string          `yaml:"flows"`
	Platforms   []string          `yaml:"platforms,omitempty"`
	Exclude     []string          `yaml:"exclude,omitempty"`
	InfraType   map[string]string `yaml:"infra-type,omitempty"`
	Identity    string            `yaml:"identity,omitempty"`
	Persistence string            `yaml:"persistence,omitempty"`
	Features    []string          `yaml:"features,omitempty"`
	ExtraValues []string          `yaml:"extra-values,omitempty"`
	QA          bool              `yaml:"qa,omitempty"`
	ImageTags   bool              `yaml:"image-tags,omitempty"`
	Upgrade     bool              `yaml:"upgrade,omitempty"`
	Enterprise  bool              `yaml:"enterprise,omitempty"`
	HelmVersion string            `yaml:"helmVersion,omitempty"`
	SkipE2E     bool              `yaml:"skip-e2e,omitempty"`
	PrefixKey   string            `yaml:"prefix-key,omitempty"`

	E2EFullSuite         bool  `yaml:"e2e-full-suite,omitempty"`
	E2ESmokeBlocking     *bool `yaml:"e2e-smoke-blocking,omitempty"`
	E2EFullSuiteBlocking *bool `yaml:"e2e-full-suite-blocking,omitempty"`

	PreInstallID  string   `yaml:"pre-install,omitempty"`
	PostInfraID   string   `yaml:"post-infra,omitempty"`
	PostDeployID  string   `yaml:"post-deploy,omitempty"`
	DependencyIDs []string `yaml:"dependencies,omitempty"`

	// Topology, when set, describes a multi-namespace deployment shape for
	// this scenario (see topology.go). Scenarios without it are unaffected.
	Topology *Topology `yaml:"topology,omitempty"`
}

// HasRegistry reports whether <chartDir>/test/<RegistryDirName>/manifest.yaml
// exists.
func HasRegistry(chartDir string) bool {
	_, err := os.Stat(filepath.Join(chartDir, "test", RegistryDirName, "manifest.yaml"))
	return err == nil
}

// LoadRegistry reads the composable CI scenario registry under
// <chartDir>/test/<RegistryDirName>/ and returns a *CITestConfig. Plural
// flows fan out to N CIScenario entries with distinct singular Flow values.
// Validation runs after assembly; assembly errors are returned immediately
// so the caller sees the file-resolution problem rather than a downstream
// validation one.
func LoadRegistry(chartDir string) (*CITestConfig, error) {
	registryDir := filepath.Join(chartDir, "test", RegistryDirName)
	manifestPath := filepath.Join(registryDir, "manifest.yaml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}
	var manifest registryManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", manifestPath, err)
	}

	var cfg CITestConfig
	cfg.Integration.Vars.TasksBaseDir = manifest.Integration.Vars.TasksBaseDir
	cfg.Integration.Vars.ValuesBaseDir = manifest.Integration.Vars.ValuesBaseDir
	cfg.Integration.Vars.ChartsBaseDir = manifest.Integration.Vars.ChartsBaseDir
	cfg.Integration.Flows = manifest.Integration.Flows

	scenariosDir := filepath.Join(registryDir, "scenarios")
	hooksDir := filepath.Join(registryDir, "hooks")
	depsDir := filepath.Join(registryDir, "dependencies")

	// Per-ID caches — hooks and deps are referenced by many scenarios; load each file once.
	hookCache := map[string]*LifecycleHook{}
	depCache := map[string]ChartDependency{}

	loadHook := func(id, scenarioID string) (*LifecycleHook, error) {
		if id == "" {
			return nil, nil
		}
		if !isPlainFilename(id) {
			return nil, fmt.Errorf("scenario %q: hook reference %q must be a plain filename (no path separators)", scenarioID, id)
		}
		if cached, ok := hookCache[id]; ok {
			return cached, nil
		}
		hookPath := filepath.Join(hooksDir, id+".yaml")
		data, err := os.ReadFile(hookPath)
		if err != nil {
			return nil, fmt.Errorf("scenario %q: read hook %s: %w", scenarioID, hookPath, err)
		}
		var h LifecycleHook
		if err := yaml.Unmarshal(data, &h); err != nil {
			return nil, fmt.Errorf("scenario %q: parse hook %s: %w", scenarioID, hookPath, err)
		}
		hookCache[id] = &h
		return &h, nil
	}

	loadDep := func(id, scenarioID string) (ChartDependency, error) {
		if !isPlainFilename(id) {
			return ChartDependency{}, fmt.Errorf("scenario %q: dependency reference %q must be a plain filename (no path separators)", scenarioID, id)
		}
		if cached, ok := depCache[id]; ok {
			return cached, nil
		}
		depPath := filepath.Join(depsDir, id+".yaml")
		data, err := os.ReadFile(depPath)
		if err != nil {
			return ChartDependency{}, fmt.Errorf("scenario %q: read dependency %s: %w", scenarioID, depPath, err)
		}
		var d ChartDependency
		if err := yaml.Unmarshal(data, &d); err != nil {
			return ChartDependency{}, fmt.Errorf("scenario %q: parse dependency %s: %w", scenarioID, depPath, err)
		}
		depCache[id] = d
		return d, nil
	}

	for _, entry := range manifest.Integration.Scenarios {
		if !isPlainFilename(entry.ID) {
			return nil, fmt.Errorf("manifest scenario id %q must be a plain filename (no path separators)", entry.ID)
		}
		if err := validateManifestTier(entry); err != nil {
			return nil, err
		}
		scnPath := filepath.Join(scenariosDir, entry.ID+".yaml")
		scnData, err := os.ReadFile(scnPath)
		if err != nil {
			return nil, fmt.Errorf("read scenario %s: %w", scnPath, err)
		}
		var rscn registryScenario
		if err := yaml.Unmarshal(scnData, &rscn); err != nil {
			return nil, fmt.Errorf("parse scenario %s: %w", scnPath, err)
		}

		preInstall, err := loadHook(rscn.PreInstallID, entry.ID)
		if err != nil {
			return nil, err
		}
		postInfra, err := loadHook(rscn.PostInfraID, entry.ID)
		if err != nil {
			return nil, err
		}
		postDeploy, err := loadHook(rscn.PostDeployID, entry.ID)
		if err != nil {
			return nil, err
		}
		var deps []ChartDependency
		for _, depID := range rscn.DependencyIDs {
			d, err := loadDep(depID, entry.ID)
			if err != nil {
				return nil, err
			}
			deps = append(deps, d)
		}

		// Resolve each topology release's per-release dependency IDs into
		// ChartDependency specs, mirroring rscn.DependencyIDs above. Mutates
		// rscn.Topology.Releases in place before it's shared onto every
		// fanned-out CIScenario below (safe: read-only afterwards, and every
		// fanned-out entry for this manifest row should see the same
		// resolved companions).
		if rscn.Topology != nil {
			for i := range rscn.Topology.Releases {
				rel := &rscn.Topology.Releases[i]
				for _, depID := range rel.Dependencies {
					d, err := loadDep(depID, entry.ID)
					if err != nil {
						return nil, err
					}
					rel.ResolvedDependencies = append(rel.ResolvedDependencies, d)
				}
			}
		}

		flows := rscn.Flows
		if len(flows) == 0 {
			// Match legacy default behavior: an empty/missing flow ends up as
			// "" on the CIScenario and is defaulted to "install" downstream
			// in matrix.Generate. Preserve that here so the equivalence test
			// holds against the legacy file.
			flows = []string{""}
		}

		for _, flow := range flows {
			cfg.Integration.Case.PR.Scenarios = append(cfg.Integration.Case.PR.Scenarios, CIScenario{
				Name:                 rscn.Name,
				Enabled:              entry.Enabled,
				Shortname:            entry.Shortname,
				Auth:                 rscn.Auth,
				Flow:                 flow,
				Platforms:            rscn.Platforms,
				Exclude:              rscn.Exclude,
				Tier:                 entry.Tier,
				InfraType:            rscn.InfraType,
				Identity:             rscn.Identity,
				Persistence:          rscn.Persistence,
				Features:             rscn.Features,
				ExtraValues:          rscn.ExtraValues,
				QA:                   rscn.QA,
				ImageTags:            rscn.ImageTags,
				Upgrade:              rscn.Upgrade,
				Enterprise:           rscn.Enterprise,
				HelmVersion:          rscn.HelmVersion,
				SkipE2E:              rscn.SkipE2E,
				E2EFullSuite:         rscn.E2EFullSuite,
				E2ESmokeBlocking:     rscn.E2ESmokeBlocking,
				E2EFullSuiteBlocking: rscn.E2EFullSuiteBlocking,
				Dependencies:         append([]ChartDependency(nil), deps...),
				PrefixKey:            rscn.PrefixKey,
				PreInstall:           preInstall,
				PostInfra:            postInfra,
				PostDeploy:           postDeploy,
				Topology:             rscn.Topology,
			})
		}
	}

	if err := (&RegistryValidator{ChartDir: chartDir}).Validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ResolveScenario(flags *config.RuntimeFlags, root *config.RootConfig) error {
	if len(flags.Deployment.Scenarios) != 1 || flags.Chart.ChartPath == "" {
		return nil
	}
	manifest := filepath.Join(flags.Chart.ChartPath, "test", RegistryDirName, "manifest.yaml")
	if _, err := os.Stat(manifest); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read scenario registry: %w", err)
	}
	registry, err := LoadRegistry(flags.Chart.ChartPath)
	if err != nil {
		return err
	}
	name := flags.Deployment.Scenarios[0]
	flow := config.FirstNonEmpty(flags.Deployment.Flow, "install")
	platform := config.FirstNonEmpty(flags.Selection.TestPlatform, flags.Deployment.Platform, "gke")
	var matched *CIScenario
	known := false
	for _, scenario := range registry.Integration.Case.PR.Scenarios {
		if scenario.Name != name {
			continue
		}
		known = true
		if config.FirstNonEmpty(scenario.Flow, "install") != flow ||
			(len(scenario.Platforms) > 0 && !slices.Contains(scenario.Platforms, platform)) {
			continue
		}
		if matched != nil {
			return fmt.Errorf("scenario %q is ambiguous for flow %q and platform %q; use matrix run", name, flow, platform)
		}
		matched = &scenario
	}
	if matched == nil {
		if known {
			return fmt.Errorf("scenario %q has no registry entry for flow %q and platform %q; use a declared combination", name, flow, platform)
		}
		return nil
	}
	if matched.Topology != nil || matched.PostInfra != nil || matched.PostDeploy != nil {
		return fmt.Errorf("scenario %q requires matrix lifecycle orchestration; use matrix run", name)
	}
	if hooks := registry.Integration.Flows[flow]; hooks != nil && hooks.PreUpgrade != nil {
		return fmt.Errorf("scenario %q flow %q requires a pre-upgrade hook; use matrix run", name, flow)
	}
	repoRoot, version, err := deriveRepoRootAndVersion(flags.Chart.ChartPath)
	if err != nil {
		return err
	}
	defaults := config.SelectionFlags{
		Identity: matched.Identity, Persistence: matched.Persistence,
		Features:  append([]string(nil), matched.Features...),
		InfraType: resolveInfraType(matched.InfraType, platform),
		QA:        matched.QA, ImageTags: matched.ImageTags, UpgradeFlow: matched.Upgrade,
	}
	if err := config.ApplySelectionDefaults(flags, defaults, root); err != nil {
		return err
	}
	flags.SelectionResolved = true
	if flags.Deployment.ScenarioPath == "" {
		flags.Deployment.ScenarioPath = filepath.Join(flags.Chart.ChartPath, "test/integration/scenarios/chart-full-setup")
	}
	flags.CompanionCharts = companionChartsForEntry(Entry{Dependencies: matched.Dependencies}, repoRoot)
	if err := registerDeclarativePreInstallHook(flags, matched.PreInstall, repoRoot, version, name); err != nil {
		return err
	}
	if differences := selectionDifferences(defaults, flags.Selection); len(differences) > 0 {
		logging.Logger.Warn().Str("scenario", name).Strs("overrides", differences).
			Msg("Registry scenario overridden; deployment differs from its declaration")
	}
	return nil
}

func selectionDifferences(declared, effective config.SelectionFlags) []string {
	var differences []string
	for _, field := range []struct{ name, declared, effective string }{
		{"identity", declared.Identity, effective.Identity},
		{"persistence", declared.Persistence, effective.Persistence},
		{"features", strings.Join(declared.Features, ","), strings.Join(effective.Features, ",")},
		{"qa", fmt.Sprint(declared.QA), fmt.Sprint(effective.QA)},
		{"image-tags", fmt.Sprint(declared.ImageTags), fmt.Sprint(effective.ImageTags)},
		{"upgrade-flow", fmt.Sprint(declared.UpgradeFlow), fmt.Sprint(effective.UpgradeFlow)},
	} {
		if field.declared != field.effective {
			differences = append(differences, fmt.Sprintf("%s: registry=%q effective=%q", field.name, field.declared, field.effective))
		}
	}
	return differences
}

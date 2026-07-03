package matrix

import (
	"fmt"
	"os"
	"path/filepath"

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

	PreInstallID  string   `yaml:"pre-install,omitempty"`
	PostInfraID   string   `yaml:"post-infra,omitempty"`
	PostDeployID  string   `yaml:"post-deploy,omitempty"`
	DependencyIDs []string `yaml:"dependencies,omitempty"`
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
				Name:         rscn.Name,
				Enabled:      entry.Enabled,
				Shortname:    entry.Shortname,
				Auth:         rscn.Auth,
				Flow:         flow,
				Platforms:    rscn.Platforms,
				Exclude:      rscn.Exclude,
				Tier:         entry.Tier,
				InfraType:    rscn.InfraType,
				Identity:     rscn.Identity,
				Persistence:  rscn.Persistence,
				Features:     rscn.Features,
				ExtraValues:  rscn.ExtraValues,
				QA:           rscn.QA,
				ImageTags:    rscn.ImageTags,
				Upgrade:      rscn.Upgrade,
				Enterprise:   rscn.Enterprise,
				HelmVersion:  rscn.HelmVersion,
				SkipE2E:      rscn.SkipE2E,
				Dependencies: append([]ChartDependency(nil), deps...),
				PrefixKey:    rscn.PrefixKey,
				PreInstall:   preInstall,
				PostInfra:    postInfra,
				PostDeploy:   postDeploy,
			})
		}
	}

	if err := (&RegistryValidator{ChartDir: chartDir}).Validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

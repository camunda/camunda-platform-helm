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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const E2EPartitionsFileName = "e2e-partitions.yaml"

type E2EPartitionRegistry struct {
	Version    string                 `yaml:"version"`
	Partitions []E2EPartitionTemplate `yaml:"partitions"`
}

type E2EPartitionTemplate struct {
	ID                string `yaml:"id"`
	ScenarioID        string `yaml:"scenario"`
	Flow              string `yaml:"flow"`
	PlaywrightProject string `yaml:"playwright-project"`
	FilePattern       string `yaml:"file-pattern,omitempty"`
	IsRBA             bool   `yaml:"is-rba,omitempty"`
	IsMT              bool   `yaml:"is-mt,omitempty"`
	IsDS              bool   `yaml:"is-ds,omitempty"`
	IsLicenseKey      bool   `yaml:"is-license-key,omitempty"`
	IsMigration       bool   `yaml:"is-migration,omitempty"`
	IsOpenSearch      bool   `yaml:"is-opensearch,omitempty"`
	MCPGatewayEnabled bool   `yaml:"mcp-gateway-enabled,omitempty"`
	Blocked           bool   `yaml:"blocked,omitempty"`
	BlockedReason     string `yaml:"blocked-reason,omitempty"`
}

type E2EPartition struct {
	ID                string `json:"id"`
	Scenario          string `json:"scenario"`
	Shortname         string `json:"shortname"`
	Flow              string `json:"flow"`
	Auth              string `json:"auth"`
	InfraType         string `json:"infra_type"`
	Identity          string `json:"identity"`
	Persistence       string `json:"persistence"`
	Features          string `json:"features"`
	QA                bool   `json:"qa"`
	ImageTags         bool   `json:"image_tags"`
	PlaywrightProject string `json:"playwright_project"`
	FilePattern       string `json:"file_pattern"`
	IsRBA             bool   `json:"is_rba"`
	IsMT              bool   `json:"is_mt"`
	IsDS              bool   `json:"is_ds"`
	IsLicenseKey      bool   `json:"is_license_key"`
	IsMigration       bool   `json:"is_migration"`
	IsOpenSearch      bool   `json:"is_opensearch"`
	MCPGatewayEnabled bool   `json:"mcp_gateway_enabled"`
}

func ResolveE2EPartitions(repoRoot, version string) ([]E2EPartition, error) {
	chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+version)
	path := filepath.Join(chartDir, "test", RegistryDirName, E2EPartitionsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read e2e partition registry %s: %w", path, err)
	}

	var registry E2EPartitionRegistry
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&registry); err != nil {
		return nil, fmt.Errorf("parse e2e partition registry %s: %w", path, err)
	}
	if registry.Version != version {
		return nil, fmt.Errorf("e2e partition registry version %q does not match requested version %q", registry.Version, version)
	}

	cfg, err := LoadRegistry(chartDir)
	if err != nil {
		return nil, err
	}
	permittedFlows, err := LoadPermittedFlows(repoRoot)
	if err != nil {
		return nil, err
	}

	manifest, err := loadRegistryManifest(chartDir)
	if err != nil {
		return nil, err
	}
	manifestEntries := make(map[string]registryManifestEntry, len(manifest.Integration.Scenarios))
	for _, entry := range manifest.Integration.Scenarios {
		manifestEntries[entry.ID] = entry
	}

	seen := make(map[string]bool)
	partitions := make([]E2EPartition, 0, len(registry.Partitions))
	for _, declared := range registry.Partitions {
		if declared.ID == "" || seen[declared.ID] {
			return nil, fmt.Errorf("e2e partition id %q must be non-empty and unique", declared.ID)
		}
		seen[declared.ID] = true
		if declared.Blocked != (strings.TrimSpace(declared.BlockedReason) != "") {
			return nil, fmt.Errorf("e2e partition %q must set blocked and blocked-reason together", declared.ID)
		}
		if declared.PlaywrightProject != "full-suite" && declared.PlaywrightProject != "full-suite-v1" {
			return nil, fmt.Errorf("e2e partition %q has unsupported Playwright project %q", declared.ID, declared.PlaywrightProject)
		}
		if len(FilterFlows(permittedFlows, version, []string{declared.Flow})) != 1 {
			return nil, fmt.Errorf("e2e partition %q uses flow %q, which is not permitted for version %s", declared.ID, declared.Flow, version)
		}

		manifestEntry, ok := manifestEntries[declared.ScenarioID]
		if !ok {
			return nil, fmt.Errorf("e2e partition %q references unknown scenario %q", declared.ID, declared.ScenarioID)
		}
		scenarioPath := filepath.Join(chartDir, "test", RegistryDirName, "scenarios", declared.ScenarioID+".yaml")
		scenarioData, err := os.ReadFile(scenarioPath)
		if err != nil {
			return nil, fmt.Errorf("read e2e partition %q scenario %s: %w", declared.ID, scenarioPath, err)
		}
		var scenario registryScenario
		scenarioDecoder := yaml.NewDecoder(bytes.NewReader(scenarioData))
		scenarioDecoder.KnownFields(true)
		if err := scenarioDecoder.Decode(&scenario); err != nil {
			return nil, fmt.Errorf("parse e2e partition %q scenario %s: %w", declared.ID, scenarioPath, err)
		}
		if manifestEntry.Enabled {
			return nil, fmt.Errorf("e2e partition %q scenario %q must remain disabled in the normal matrix", declared.ID, declared.ScenarioID)
		}
		if !containsString(scenario.Flows, declared.Flow) {
			return nil, fmt.Errorf("e2e partition %q flow %q is not declared by scenario %q", declared.ID, declared.Flow, declared.ScenarioID)
		}
		if !hasResolvedScenario(cfg, manifestEntry.Shortname, scenario.Name, declared.Flow) {
			return nil, fmt.Errorf("e2e partition %q does not resolve to scenario %q, shortname %q, flow %q", declared.ID, scenario.Name, manifestEntry.Shortname, declared.Flow)
		}
		if !scenario.QA || !scenario.ImageTags {
			return nil, fmt.Errorf("e2e partition %q scenario %q must enable qa and image-tags", declared.ID, declared.ScenarioID)
		}
		if !containsString(scenario.Platforms, "gke") {
			return nil, fmt.Errorf("e2e partition %q scenario %q does not support gke", declared.ID, declared.ScenarioID)
		}
		features := append([]string(nil), scenario.Features...)
		if err := validatePartitionFeatures(chartDir, declared.ID, features); err != nil {
			return nil, err
		}
		for feature, enabled := range map[string]bool{
			"rba": declared.IsRBA, "multitenancy": declared.IsMT, "documentstore": declared.IsDS,
			"license": declared.IsLicenseKey, "mcp": declared.MCPGatewayEnabled,
		} {
			if enabled != containsString(features, feature) {
				return nil, fmt.Errorf("e2e partition %q flag for feature %q does not match its resolved features", declared.ID, feature)
			}
		}
		if declared.IsOpenSearch != strings.HasPrefix(scenario.Persistence, "opensearch") {
			return nil, fmt.Errorf("e2e partition %q opensearch flag does not match persistence %q", declared.ID, scenario.Persistence)
		}
		if declared.IsMigration != strings.Contains(declared.Flow, "upgrade") {
			return nil, fmt.Errorf("e2e partition %q migration flag does not match flow %q", declared.ID, declared.Flow)
		}
		if declared.Blocked {
			continue
		}

		partitions = append(partitions, E2EPartition{
			ID: declared.ID, Scenario: scenario.Name, Shortname: manifestEntry.Shortname,
			Flow: declared.Flow, Auth: scenario.Auth, InfraType: scenario.InfraType["gke"],
			Identity: scenario.Identity, Persistence: scenario.Persistence,
			Features: strings.Join(features, ","), QA: scenario.QA, ImageTags: scenario.ImageTags,
			PlaywrightProject: declared.PlaywrightProject, FilePattern: declared.FilePattern,
			IsRBA: declared.IsRBA, IsMT: declared.IsMT, IsDS: declared.IsDS,
			IsLicenseKey: declared.IsLicenseKey, IsMigration: declared.IsMigration,
			IsOpenSearch: declared.IsOpenSearch, MCPGatewayEnabled: declared.MCPGatewayEnabled,
		})
	}
	return partitions, nil
}

func loadRegistryManifest(chartDir string) (*registryManifest, error) {
	path := filepath.Join(chartDir, "test", RegistryDirName, "manifest.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var manifest registryManifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return &manifest, nil
}

func E2EPartitionsJSON(partitions []E2EPartition) (string, error) {
	data, err := json.Marshal(partitions)
	if err != nil {
		return "", fmt.Errorf("marshal e2e partitions: %w", err)
	}
	return string(data), nil
}

func validatePartitionFeatures(chartDir, partitionID string, features []string) error {
	seen := make(map[string]bool)
	for _, feature := range features {
		if seen[feature] {
			return fmt.Errorf("e2e partition %q declares duplicate feature %q", partitionID, feature)
		}
		seen[feature] = true
		path := filepath.Join(chartDir, "test", "integration", "scenarios", "chart-full-setup", "values", "features", feature+".yaml")
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return fmt.Errorf("e2e partition %q feature %q does not resolve at %s", partitionID, feature, path)
		}
	}
	return nil
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func hasResolvedScenario(cfg *CITestConfig, shortname, name, flow string) bool {
	for _, scenario := range cfg.Integration.Case.PR.Scenarios {
		if scenario.Shortname == shortname && scenario.Name == name && scenario.Flow == flow {
			return true
		}
	}
	return false
}

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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// OrphanAllowlistFile is the per-chart YAML file that exempts specific files
// in pre-setup-scripts/ and common/resources/ from the orphan check (i.e.,
// files that exist on disk but are not referenced by any LifecycleHook in
// ci-test-config.yaml). Living in the chart tree alongside the data it
// exempts, the file decouples allowlist maintenance from the deploy-camunda
// Go binary — adding an exempt file is a pure data change.
const OrphanAllowlistFile = "test/integration/scenarios/.orphan-allowlist.yaml"

// OrphanAllowlist is the on-disk schema for OrphanAllowlistFile. Every entry
// requires a `reason:` so reviewers can tell from a YAML diff alone why an
// exemption was added.
type OrphanAllowlist struct {
	PreSetupScripts []OrphanAllowlistEntry `yaml:"pre-setup-scripts"`
	CommonResources []OrphanAllowlistEntry `yaml:"common-resources"`
}

type OrphanAllowlistEntry struct {
	Name   string `yaml:"name"`
	Reason string `yaml:"reason"`
}

// LoadOrphanAllowlist reads <chartDir>/test/integration/scenarios/.orphan-allowlist.yaml
// and returns lookup maps for the script and fixture exemptions. A missing
// file is not an error — it just means "no exemptions for this chart
// version" — so chart versions with nothing to exempt don't need a stub
// file. Entries with empty names or reasons are rejected so that a malformed
// allowlist fails loudly instead of silently exempting an empty string.
func LoadOrphanAllowlist(chartDir string) (scripts, resources map[string]bool, err error) {
	scripts = map[string]bool{}
	resources = map[string]bool{}

	path := filepath.Join(chartDir, OrphanAllowlistFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return scripts, resources, nil
		}
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}

	var doc OrphanAllowlist
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}

	for i, e := range doc.PreSetupScripts {
		if e.Name == "" || e.Reason == "" {
			return nil, nil, fmt.Errorf("%s: pre-setup-scripts[%d]: name and reason are required", path, i)
		}
		scripts[e.Name] = true
	}
	for i, e := range doc.CommonResources {
		if e.Name == "" || e.Reason == "" {
			return nil, nil, fmt.Errorf("%s: common-resources[%d]: name and reason are required", path, i)
		}
		resources[e.Name] = true
	}
	return scripts, resources, nil
}

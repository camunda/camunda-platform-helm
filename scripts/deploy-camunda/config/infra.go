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

package config

import (
	"fmt"
	"reflect"
	"strings"
)

// MergeIntField applies the matrix/root value to target when the CLI flag was
// not explicitly set by the user. Pointer-typed config values (nil = unset)
// allow distinguishing "not configured" from zero.
func MergeIntField(target *int, matrixVal, rootVal *int, changedFlags map[string]bool, flagName string) {
	if changedFlags != nil && changedFlags[flagName] {
		return // CLI flag was explicitly set; do not override
	}
	if matrixVal != nil {
		*target = *matrixVal
	} else if rootVal != nil {
		*target = *rootVal
	}
}

// InfraMaps holds per-platform and per-version infra values. Only matrix commands read them.
type InfraMaps struct {
	KubeContexts       map[string]string `mapstructure:"kubeContexts" yaml:"kubeContexts,omitempty"`
	IngressBaseDomains map[string]string `mapstructure:"ingressBaseDomains" yaml:"ingressBaseDomains,omitempty"`
	VaultBackedSecrets map[string]bool   `mapstructure:"vaultBackedSecrets" yaml:"vaultBackedSecrets,omitempty"`
	EnvFiles           map[string]string `mapstructure:"envFiles" yaml:"envFiles,omitempty"`
}

// InfraOverride holds the infra values set on the command line.
type InfraOverride struct {
	InfraConfig
	InfraMaps
}

func (m InfraMaps) pick(platform, version string) InfraConfig {
	infra := InfraConfig{KubeContext: m.KubeContexts[platform], IngressBaseDomain: m.IngressBaseDomains[platform], EnvFile: m.EnvFiles[version]}
	if enabled, ok := m.VaultBackedSecrets[platform]; ok {
		infra.UseVaultBackedSecrets = &enabled
	}
	return infra
}

// ActiveProfile returns deployments[current], or the only profile when current is empty.
func (rc *RootConfig) ActiveProfile() (*DeploymentConfig, error) {
	if len(rc.Deployments) == 0 {
		return nil, nil
	}
	name := strings.TrimSpace(rc.Current)
	if name == "" && len(rc.Deployments) == 1 {
		for only := range rc.Deployments {
			name = only
		}
	}
	if name == "" {
		return nil, nil
	}
	dep, ok := rc.Deployments[name]
	if !ok {
		return nil, fmt.Errorf("active deployment %q not found in config", name)
	}
	return &dep, nil
}

// ResolveInfra merges the infra values for one platform and chart version. Each field takes the
// first value set in: cli per-platform/per-version values, cli values, matrix maps, the active
// profile, matrix values, root values. The matrix layers apply only when matrixMode is true. An
// unknown current profile is skipped here; ActiveProfile reports it.
func (rc *RootConfig) ResolveInfra(matrixMode bool, platform, version string, cli InfraOverride) InfraConfig {
	if rc == nil {
		rc = &RootConfig{}
	}
	layers := []InfraConfig{cli.pick(platform, version), cli.InfraConfig}
	if matrixMode {
		layers = append(layers, rc.Matrix.pick(platform, version))
	}
	if dep, _ := rc.ActiveProfile(); dep != nil {
		layers = append(layers, dep.InfraConfig)
	}
	if matrixMode {
		layers = append(layers, rc.Matrix.InfraConfig)
	}
	var infra InfraConfig
	resolved := reflect.ValueOf(&infra).Elem()
	for _, layer := range append(layers, rc.InfraConfig) {
		values := reflect.ValueOf(layer)
		for i := range resolved.NumField() {
			if resolved.Field(i).IsZero() {
				resolved.Field(i).Set(values.Field(i))
			}
		}
	}
	return infra
}

// infraKeys returns the YAML keys, with prefix, of the profile-scoped fields infra sets.
func infraKeys(prefix string, infra InfraConfig) []string {
	var keys []string
	for _, field := range []struct{ key, value string }{
		{"platform", infra.Platform}, {"repoRoot", infra.RepoRoot}, {"kubeContext", infra.KubeContext},
		{"ingressBaseDomain", infra.IngressBaseDomain}, {"envFile", infra.EnvFile},
	} {
		if field.value != "" {
			keys = append(keys, prefix+field.key)
		}
	}
	return keys
}

// LoadMatrixConfig loads the config file and returns the parsed RootConfig
// suitable for use by matrix subcommands. Environment overrides are applied.
func LoadMatrixConfig(configPath string) (*RootConfig, error) {
	res, err := ResolvePath(configPath)
	if err != nil {
		return nil, err
	}
	rc, err := Read(res.Path, true)
	if err != nil {
		return nil, err
	}
	_, err = rc.ActiveProfile()
	return rc, err
}

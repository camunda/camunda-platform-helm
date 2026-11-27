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

package credentials

import (
	"fmt"
	"path/filepath"
)

// Provider makes SecretName exist in a namespace.
type Provider interface {
	Name() string
	Describe() string
}

// VaultStore is the ExternalSecrets store whose manifests carry the -vault suffix.
const VaultStore = "vault-backend"

// ESO provides SecretName through an ExternalSecret manifest applied to the namespace.
type ESO struct {
	// Store is the effective ExternalSecrets store name.
	Store string
	// Override, when set, is the only manifest considered.
	Override string
}

func (ESO) Name() string { return "eso" }

// VaultSuffix is "-vault" for the vault-backed store, else empty.
func (e ESO) VaultSuffix() string {
	if e.Store == VaultStore {
		return "-vault"
	}
	return ""
}

// ManifestName is the file name of the integration-test-credentials ExternalSecret for e.Store.
func (e ESO) ManifestName() string {
	return ManifestFileName(e.VaultSuffix())
}

// ManifestFileName is the integration-test-credentials ExternalSecret file name for vaultSuffix.
func ManifestFileName(vaultSuffix string) string {
	return fmt.Sprintf("external-secret-integration-test-credentials%s.yaml", vaultSuffix)
}

// ChartManifest is the chart's own manifest under chartDir.
func (e ESO) ChartManifest(chartDir string) string {
	return filepath.Join(chartDir, "test", "integration", "external-secrets", e.ManifestName())
}

// Manifest picks the one manifest that provides a namespace's SecretName.
func (e ESO) Manifest(chartPath, externalSecretDir string, exists func(string) bool) (string, error) {
	return SelectManifest(chartPath, externalSecretDir, e.VaultSuffix(), e.Override, exists)
}

// Describe renders the store, override and manifest name.
func (e ESO) Describe() string {
	store := e.Store
	if store == "" {
		store = "(default)"
	}
	override := e.Override
	if override == "" {
		override = "(none)"
	}
	return fmt.Sprintf("eso: store=%s manifest=%s override=%s order=override,chart,shared", store, e.ManifestName(), override)
}

// SelectManifest picks the scenario's own manifest when given (and then never
// the CI one), otherwise the chart-specific file, otherwise the shared
// fallback. An empty result means there is none to apply.
func SelectManifest(chartPath, externalSecretDir, vaultSuffix, override string, exists func(string) bool) (string, error) {
	if override != "" {
		if !exists(override) {
			return "", fmt.Errorf("credentials manifest %q does not exist", override)
		}
		return override, nil
	}
	name := ManifestFileName(vaultSuffix)
	for _, candidate := range []string{
		filepath.Join(chartPath, "test", "integration", "external-secrets", name),
		filepath.Join(externalSecretDir, name),
	} {
		if exists(candidate) {
			return candidate, nil
		}
	}
	return "", nil
}

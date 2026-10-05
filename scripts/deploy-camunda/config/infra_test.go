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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplySelectionDefaultsAliasPrecedence(t *testing.T) {
	for _, profile := range []bool{false, true} {
		for _, modern := range []bool{false, true} {
			name := "root"
			if profile {
				name = "profile"
			}
			if modern {
				name += "/modern"
			}
			t.Run(name, func(t *testing.T) {
				disabled := false
				spec := DeploySpecConfig{Identity: "basic", Persistence: "elasticsearch", Features: []string{"documentstore"}, TestPlatform: "gke", QA: &disabled}
				root := &RootConfig{DeploySpecConfig: spec}
				if profile {
					root = &RootConfig{Deployments: map[string]DeploymentConfig{"dev": {DeploySpecConfig: spec}}}
				}
				flags := &RuntimeFlags{
					Deprecated:   DeprecatedFlags{ValuesAuth: "oidc", ValuesBackend: "opensearch", ValuesFeatures: []string{"rdbms", "upgrade", "multitenancy"}, ValuesInfra: "eks", ValuesQA: true},
					ChangedFlags: map[string]bool{"values-auth": true, "values-backend": true, "values-features": true, "values-infra": true, "values-qa": true},
				}
				want := SelectionFlags{Identity: "oidc", Persistence: "opensearch", Features: []string{"multitenancy"}, TestPlatform: "eks", QA: true, UpgradeFlow: true}
				if modern {
					want = SelectionFlags{Identity: "hybrid", Persistence: "rdbms-external", Features: []string{}, TestPlatform: "gke"}
					flags.Selection = want
					for _, flag := range []string{"identity", "persistence", "features", "test-platform", "qa", "upgrade-flow"} {
						flags.ChangedFlags[flag] = true
					}
				}
				require.NoError(t, ApplyActiveDeployment(root, flags))
				flags.MigrateDeprecatedFlags()
				require.NoError(t, ApplySelectionDefaults(flags, SelectionFlags{Identity: "keycloak", Persistence: "elasticsearch"}, root))
				require.Equal(t, want, flags.Selection)
			})
		}
	}
}

func TestApplySelectionDefaultsAliasClearing(t *testing.T) {
	enabled := true
	root := &RootConfig{DeploySpecConfig: DeploySpecConfig{Persistence: "elasticsearch", Features: []string{"documentstore"}, QA: &enabled}}
	defaults := SelectionFlags{Identity: "keycloak", Persistence: "opensearch"}
	flags := &RuntimeFlags{ChangedFlags: map[string]bool{"values-features": true, "values-qa": true}}
	require.NoError(t, ApplyActiveDeployment(root, flags))
	flags.MigrateDeprecatedFlags()
	require.NoError(t, ApplySelectionDefaults(flags, defaults, root))
	require.Empty(t, flags.Selection.Features)
	require.False(t, flags.Selection.QA)
	flags = &RuntimeFlags{
		Deprecated:   DeprecatedFlags{ValuesFeatures: []string{"rdbms", "upgrade", "multitenancy"}},
		ChangedFlags: map[string]bool{"values-features": true},
	}
	require.NoError(t, ApplyActiveDeployment(root, flags))
	flags.MigrateDeprecatedFlags()
	require.NoError(t, ApplySelectionDefaults(flags, defaults, root))
	require.Equal(t, "rdbms", flags.Selection.Persistence)
	require.True(t, flags.Selection.UpgradeFlow)
	flags = &RuntimeFlags{
		Deprecated:   DeprecatedFlags{ValuesFeatures: []string{"rdbms", "upgrade"}},
		ChangedFlags: map[string]bool{"values-features": true, "features": true},
	}
	flags.MigrateDeprecatedFlags()
	require.NoError(t, ApplySelectionDefaults(flags, defaults, nil))
	require.Empty(t, flags.Selection.Features)
	require.Equal(t, "opensearch", flags.Selection.Persistence)
	require.False(t, flags.Selection.UpgradeFlow)
}

func TestApplySelectionDefaults(t *testing.T) {
	t.Parallel()
	disabled := false
	defaults := SelectionFlags{Identity: "keycloak", Persistence: "elasticsearch", Features: []string{"registry"}, QA: true, ImageTags: true, UpgradeFlow: true}
	root := &RootConfig{DeploySpecConfig: DeploySpecConfig{Identity: "basic", Features: []string{"root"}}}
	flags := &RuntimeFlags{}
	require.NoError(t, ApplySelectionDefaults(flags, defaults, nil))
	require.Equal(t, defaults, flags.Selection)
	require.NoError(t, ApplySelectionDefaults(flags, defaults, root))
	require.Equal(t, "basic", flags.Selection.Identity)
	require.Equal(t, []string{"root"}, flags.Selection.Features)
	root.Deployments = map[string]DeploymentConfig{
		"only": {DeploySpecConfig: DeploySpecConfig{Identity: "oidc", Features: []string{}, QA: &disabled, ImageTags: &disabled, UpgradeFlow: &disabled}},
	}
	require.NoError(t, ApplySelectionDefaults(flags, defaults, root))
	require.Equal(t, "oidc", flags.Selection.Identity)
	require.Empty(t, flags.Selection.Features)
	require.False(t, flags.Selection.QA)
	require.False(t, flags.Selection.ImageTags)
	require.False(t, flags.Selection.UpgradeFlow)
	flags.Selection = SelectionFlags{Identity: "hybrid", Persistence: "custom", Features: []string{"cli"}, TestPlatform: "eks"}
	flags.ChangedFlags = map[string]bool{"identity": true, "persistence": true, "features": true, "test-platform": true, "qa": true, "image-tags": true, "upgrade-flow": true}
	want := flags.Selection
	require.NoError(t, ApplySelectionDefaults(flags, defaults, root))
	require.Equal(t, want, flags.Selection)
	flags.Selection.Features = []string{}
	require.NoError(t, ApplySelectionDefaults(flags, defaults, root))
	require.Empty(t, flags.Selection.Features)
	root.Deployments = nil
	require.NoError(t, ApplySelectionDefaults(flags, defaults, root))
	require.Empty(t, flags.Selection.Features, "empty CLI features must override nonempty root features")
	flags = &RuntimeFlags{
		Deprecated:   DeprecatedFlags{ValuesAuth: "basic", ValuesFeatures: []string{"rdbms", "upgrade", "custom"}},
		ChangedFlags: map[string]bool{"values-auth": true, "values-features": true},
	}
	flags.MigrateDeprecatedFlags()
	require.NoError(t, ApplySelectionDefaults(flags, defaults, nil))
	require.Equal(t, "basic", flags.Selection.Identity)
	require.Equal(t, "rdbms", flags.Selection.Persistence)
	require.Equal(t, []string{"custom"}, flags.Selection.Features)
	require.True(t, flags.Selection.UpgradeFlow)
}

func TestResolveInfraPrecedence(t *testing.T) {
	yes, no := true, false
	rc := &RootConfig{
		InfraConfig: InfraConfig{KubeContext: "root", IngressBaseDomain: "root", EnvFile: "root", RepoRoot: "root", LogLevel: "root"},
		Current:     "dev",
		Deployments: map[string]DeploymentConfig{"dev": {InfraConfig: InfraConfig{KubeContext: "profile", IngressBaseDomain: "profile", UseVaultBackedSecrets: &no}}},
		Matrix: MatrixConfig{
			InfraConfig: InfraConfig{KubeContext: "matrix", RepoRoot: "matrix", EnvFile: "matrix"},
			InfraMaps:   InfraMaps{KubeContexts: map[string]string{"gke": "matrix-gke"}, EnvFiles: map[string]string{"8.10": "matrix-8.10"}, VaultBackedSecrets: map[string]bool{"eks": true}},
		},
	}
	cli := InfraOverride{InfraConfig: InfraConfig{IngressBaseDomain: "cli"}, InfraMaps: InfraMaps{KubeContexts: map[string]string{"eks": "cli-eks"}}}
	for _, tc := range []struct {
		name              string
		matrixMode        bool
		platform, version string
		cli               InfraOverride
		want              InfraConfig
	}{
		{name: "single deploy reads profile then root", want: InfraConfig{KubeContext: "profile", IngressBaseDomain: "profile", EnvFile: "root", RepoRoot: "root", LogLevel: "root", UseVaultBackedSecrets: &no}},
		{name: "matrix maps beat the profile", matrixMode: true, platform: "gke", version: "8.10", want: InfraConfig{KubeContext: "matrix-gke", IngressBaseDomain: "profile", EnvFile: "matrix-8.10", RepoRoot: "matrix", LogLevel: "root", UseVaultBackedSecrets: &no}},
		{name: "profile beats matrix scalars", matrixMode: true, platform: "aks", version: "8.9", want: InfraConfig{KubeContext: "profile", IngressBaseDomain: "profile", EnvFile: "matrix", RepoRoot: "matrix", LogLevel: "root", UseVaultBackedSecrets: &no}},
		{name: "cli beats config", matrixMode: true, platform: "eks", version: "8.10", cli: cli, want: InfraConfig{KubeContext: "cli-eks", IngressBaseDomain: "cli", EnvFile: "matrix-8.10", RepoRoot: "matrix", LogLevel: "root", UseVaultBackedSecrets: &yes}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, rc.ResolveInfra(tc.matrixMode, tc.platform, tc.version, tc.cli))
		})
	}
}

func TestResolveInfraMatchesSingleDeploy(t *testing.T) {
	enabled := true
	rc := &RootConfig{
		InfraConfig: InfraConfig{Platform: "eks", KubeContext: "root-ctx", LogLevel: "debug"},
		Deployments: map[string]DeploymentConfig{"only": {InfraConfig: InfraConfig{Platform: "gke", KubeContext: "profile-ctx", IngressBaseDomain: "ci.distro.ultrawombat.com", EnvFile: ".env.profile", RepoRoot: "/repo", EnsureDockerHub: &enabled}}},
	}
	matrixInfra := rc.ResolveInfra(true, "gke", "8.10", InfraOverride{})
	require.Equal(t, rc.ResolveInfra(false, "", "", InfraOverride{}), matrixInfra)

	flags := &RuntimeFlags{}
	require.NoError(t, ApplyActiveDeployment(rc, flags))
	require.Equal(t,
		[]string{matrixInfra.Platform, matrixInfra.KubeContext, matrixInfra.IngressBaseDomain, matrixInfra.EnvFile, matrixInfra.RepoRoot, matrixInfra.LogLevel},
		[]string{flags.Deployment.Platform, flags.Test.KubeContext, flags.Ingress.IngressBaseDomain, flags.EnvFile, flags.Chart.RepoRoot, flags.LogLevel})
	require.True(t, flags.Docker.EnsureDockerHub)
}

func TestActiveProfileRejectsUnknownCurrent(t *testing.T) {
	rc := &RootConfig{Current: "missing", Deployments: map[string]DeploymentConfig{"dev": {}}}
	_, err := rc.ActiveProfile()
	require.ErrorContains(t, err, `"missing"`)
	require.ErrorContains(t, ApplyActiveDeployment(rc, &RuntimeFlags{}), `"missing"`)
}

func TestReadRecordsDeprecatedInfraKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.yaml")
	configFile := "repoRoot: /repo\nlogLevel: debug\nmatrix:\n  kubeContext: ctx\n  namespacePrefix: distribution\n  envFiles:\n    \"8.10\": .env.810\ndeployments:\n  dev:\n    kubeContext: dev\n"
	require.NoError(t, os.WriteFile(path, []byte(configFile), 0o600))
	t.Setenv("CAMUNDA_PLATFORM", "eks")

	rc, err := Read(path, true)
	require.NoError(t, err)
	require.Equal(t, []string{"repoRoot", "matrix.kubeContext"}, rc.DeprecatedInfra)
	require.Equal(t, "eks", rc.Platform)
	require.Equal(t, ".env.810", rc.Matrix.EnvFiles["8.10"])
}

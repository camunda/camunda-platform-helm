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

package cmd

import (
	"os"
	"path/filepath"
	"scripts/deploy-camunda/acceptance"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestE2EEnvMergeFailsOnMissingRenderScript(t *testing.T) {
	cmd := newE2EEnvMergeCommand()
	cmd.SetArgs([]string{
		"--orchestration-namespace", "matrix-810-mns-orcha",
		"--hub-namespace", "matrix-810-mns-hub",
		"--absolute-chart-path", "/workspace/charts/camunda-platform-8.10",
		"--render-script", "/nonexistent/render-e2e-env.sh",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --render-script points at a non-existent path")
	}
	if !strings.Contains(err.Error(), "render script failed") {
		t.Fatalf("expected error to mention render script failure, got: %v", err)
	}
}

func TestE2EEnvMergeRejectsHalfConfiguredOptimizeBeforeRendering(t *testing.T) {
	cmd := newE2EEnvMergeCommand()
	cmd.SetArgs([]string{
		"--orchestration-namespace", "matrix-810-mns-orcha",
		"--hub-namespace", "matrix-810-mns-hub",
		"--absolute-chart-path", "/workspace/charts/camunda-platform-8.10",
		"--render-script", "/nonexistent/render-e2e-env.sh",
		"--optimize-namespace", "matrix-810-mns-opt-orcha",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when only --optimize-namespace is supplied")
	}
	// The render script is deliberately bogus: the flag check must fail first,
	// proving it runs before any cluster or filesystem work.
	if !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("expected the flag pairing error ahead of the render step, got: %v", err)
	}
}

func TestE2EEnvAPISuiteCommandAfterSourcingTheRenderedEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	var rendered strings.Builder
	for key, value := range map[string]string{
		"PLAYWRIGHT_BASE_URL": "https://ci.example.com",
		"BASE_URL":            "https://ci.example.com",
		"OAUTH_URL":           "https://ci.example.com/auth/realms/camunda-platform/protocol/openid-connect/token",
		"MINOR_VERSION":       "SM-8.10",
		"CONNECTORS_BASE_URL": "https://ci.example.com/connectors/inbound",
		"IDENTITY_BASE_URL":   "https://ci.example.com/identity/",
		"CONSOLE_BASE_URL":    "https://ci.example.com",
		"WEBMODELER_BASE_URL": "https://ci.example.com/modeler",
	} {
		rendered.WriteString(key + "=" + value + "\n")
		t.Setenv(key, value)
	}
	writeFile(t, envFile, rendered.String())
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "kubectl"), []byte("#!/bin/sh\necho v-secret\n"), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output := filepath.Join(dir, ".env.api")

	root := NewRootCommand()
	root.AddCommand(newE2EEnvCommand())
	root.SetOut(&strings.Builder{})
	root.SetArgs([]string{"e2e-env", "api-suite", "--env-file", envFile, "--namespace", "ns", "--output", output})
	require.NoError(t, root.Execute())

	assert.Contains(t, readFile(t, output), "BASE_URL='https://ci.example.com/orchestration'\n")
}

// The physical-tenants spec logs in as venom with VENOM_CLIENT_SECRET, which e2e-env resolves
// from acceptance.VenomClientSecretKey. That key must stay the one the physicaltenants Hub layer registers
// venom with, or Keycloak rejects the client credentials.
func TestVenomClientSecretKeyMatchesPhysicalTenantsHubLayer(t *testing.T) {
	raw, err := os.ReadFile("../../../charts/camunda-platform-8.10/test/integration/scenarios/chart-full-setup/values/features/physicaltenants-hub.yaml")
	require.NoError(t, err)
	var values struct {
		Identity struct {
			Clients []struct {
				ID     string `yaml:"id"`
				Secret struct {
					ExistingSecretKey string `yaml:"existingSecretKey"`
				} `yaml:"secret"`
			} `yaml:"clients"`
		} `yaml:"identity"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &values))
	for _, client := range values.Identity.Clients {
		if client.ID == "venom" {
			assert.Equal(t, acceptance.VenomClientSecretKey, client.Secret.ExistingSecretKey)
			return
		}
	}
	t.Fatal("physicaltenants-hub.yaml registers no venom client")
}

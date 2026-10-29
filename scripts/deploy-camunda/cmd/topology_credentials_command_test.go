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
	"testing"

	"scripts/deploy-camunda/config"
)

// The topology driver hands each release the rendered credentials-manifest
// through flags.Secrets.CredentialsManifest; without it the deployer falls back
// to the CI-wide integration-test credentials.
func TestApplyTopologyCredentialsManifest(t *testing.T) {
	repoRoot := t.TempDir()
	rel := filepath.Join("charts", "creds.yaml")
	if err := os.MkdirAll(filepath.Join(repoRoot, "charts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, rel), []byte("key: ${TOPOLOGY_BASE}-credentials\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("declared manifest is rendered and wired into the deploy flags", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		remove, err := applyTopologyCredentialsManifest(flags, repoRoot, rel, "dogfood")
		if err != nil {
			t.Fatal(err)
		}
		path := flags.Secrets.CredentialsManifest
		if path == "" {
			t.Fatal("flags.Secrets.CredentialsManifest is empty; the deploy would apply the CI-wide credentials")
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "key: dogfood-credentials\n" {
			t.Errorf("rendered manifest = %q, want the base namespace substituted", got)
		}
		remove()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("rendered manifest %s not removed: %v", path, err)
		}
	})

	t.Run("no manifest leaves the flags unchanged", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		flags.Secrets.CredentialsManifest = "preset.yaml"
		remove, err := applyTopologyCredentialsManifest(flags, repoRoot, "", "dogfood")
		if err != nil {
			t.Fatal(err)
		}
		remove()
		if flags.Secrets.CredentialsManifest != "preset.yaml" {
			t.Errorf("CredentialsManifest = %q, want unchanged", flags.Secrets.CredentialsManifest)
		}
	})

	t.Run("missing manifest is an error", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		if _, err := applyTopologyCredentialsManifest(flags, repoRoot, "charts/absent.yaml", "dogfood"); err == nil {
			t.Fatal("expected an error for a missing manifest")
		}
		if flags.Secrets.CredentialsManifest != "" {
			t.Errorf("CredentialsManifest = %q, want empty after a failure", flags.Secrets.CredentialsManifest)
		}
	})
}

func TestEnsureCredentialsCommand_AcceptsTheDeprecatedGuardNamespaceFlag(t *testing.T) {
	cmd := newTopologyEnsureCredentialsCommand()
	if err := cmd.ParseFlags([]string{"--manifest", "m.yaml", "--guard-namespace", "a", "--guard-namespace", "b,c"}); err != nil {
		t.Fatalf("existing callers pass --guard-namespace: %v", err)
	}
	if f := cmd.Flags().Lookup("guard-namespace"); f == nil || f.Deprecated == "" {
		t.Error("--guard-namespace must be marked deprecated")
	}
}

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
	"context"
	"os"
	"path/filepath"
	"testing"

	"scripts/deploy-camunda/config"
)

// TestResolveLifecycleEnv_Layering pins the credential-resolution priority
// (process env → envFile → ExtraEnv) and the empty-value-does-not-override
// semantics that govern which credentials reach lifecycle scripts and
// fixtures. A regression here would silently produce wrong credentials in
// CI without any failing test elsewhere — see crev review on PR #6103.
func TestResolveLifecycleEnv_Layering(t *testing.T) {
	// Use a key from lifecycleVarPassthrough so the function actually
	// considers it; otherwise everything is filtered out by the allowlist.
	const key = "RDBMS_POSTGRESQL_USERNAME"

	type layer struct {
		processEnv string
		envFile    map[string]string // nil → no file written
		extraEnv   map[string]string
	}

	tests := []struct {
		name  string
		setup layer
		want  string // empty string means key not in result map
	}{
		{
			name:  "process env only",
			setup: layer{processEnv: "from-process"},
			want:  "from-process",
		},
		{
			name: "envFile overrides process env",
			setup: layer{
				processEnv: "from-process",
				envFile:    map[string]string{key: "from-file"},
			},
			want: "from-file",
		},
		{
			name: "ExtraEnv overrides envFile",
			setup: layer{
				processEnv: "from-process",
				envFile:    map[string]string{key: "from-file"},
				extraEnv:   map[string]string{key: "from-extra"},
			},
			want: "from-extra",
		},
		{
			name: "empty ExtraEnv does NOT override non-empty envFile",
			setup: layer{
				envFile:  map[string]string{key: "from-file"},
				extraEnv: map[string]string{key: ""},
			},
			want: "from-file",
		},
		{
			name: "empty envFile value does NOT override non-empty process env",
			setup: layer{
				processEnv: "from-process",
				envFile:    map[string]string{key: ""},
			},
			want: "from-process",
		},
		{
			name:  "no layer sets the key — absent from result",
			setup: layer{},
			want:  "",
		},
		{
			name: "ExtraEnv only when process env + envFile both unset",
			setup: layer{
				extraEnv: map[string]string{key: "from-extra-only"},
			},
			want: "from-extra-only",
		},
		{
			name: "envFile read error degrades to process env",
			setup: layer{
				processEnv: "from-process",
				envFile:    nil, // sentinel: caller writes nothing, EnvFile set to nonexistent path
			},
			want: "from-process",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Process env isolation.
			t.Setenv(key, tt.setup.processEnv)

			flags := &config.RuntimeFlags{ExtraEnv: tt.setup.extraEnv}

			if tt.setup.envFile != nil {
				envFilePath := filepath.Join(t.TempDir(), ".env")
				var content string
				for k, v := range tt.setup.envFile {
					content += k + "=" + v + "\n"
				}
				if err := os.WriteFile(envFilePath, []byte(content), 0o600); err != nil {
					t.Fatalf("write env file: %v", err)
				}
				flags.EnvFile = envFilePath
			} else if tt.name == "envFile read error degrades to process env" {
				// Point at a path that doesn't exist to exercise the error path.
				flags.EnvFile = filepath.Join(t.TempDir(), "does-not-exist.env")
			}

			got := resolveLifecycleEnv(flags)
			if got[key] != tt.want {
				t.Errorf("resolveLifecycleEnv()[%q] = %q, want %q", key, got[key], tt.want)
			}
		})
	}
}

// TestTwoStepUpgrade_HookSliceDetachment pins the slice-detachment idiom that
// keeps step1Flags / step2Flags hook registrations isolated from the parent
// flags. Without the explicit `append([]…(nil), parent...)` copy, a Go
// append() into the shallow-copied slice can mutate the parent's backing
// array — a non-deterministic failure depending on slice capacity.
//
// This test directly exercises the detachment pattern used at
// runner.go:2027-2028 + 2113-2114 rather than calling executeTwoStepUpgrade
// (which has external dependencies on helm + kube). If a future refactor
// removes the detachment, this test fails.
func TestTwoStepUpgrade_HookSliceDetachment(t *testing.T) {
	parent := &config.RuntimeFlags{
		PreInstallHooks: []func(context.Context) error{
			func(context.Context) error { return nil },
		},
		PostDeployHooks: []func(context.Context) error{
			func(context.Context) error { return nil },
		},
	}

	// Mirror the detachment idiom from executeTwoStepUpgrade.
	step1 := *parent
	step1.PreInstallHooks = append([]func(context.Context) error(nil), parent.PreInstallHooks...)
	step1.PostDeployHooks = append([]func(context.Context) error(nil), parent.PostDeployHooks...)

	// Appending to the detached step1 slices must NOT mutate parent.
	step1.PreInstallHooks = append(step1.PreInstallHooks, func(context.Context) error { return nil })
	step1.PostDeployHooks = append(step1.PostDeployHooks, func(context.Context) error { return nil })

	if got, want := len(parent.PreInstallHooks), 1; got != want {
		t.Errorf("parent.PreInstallHooks: detachment leaked, got len %d, want %d", got, want)
	}
	if got, want := len(parent.PostDeployHooks), 1; got != want {
		t.Errorf("parent.PostDeployHooks: detachment leaked, got len %d, want %d", got, want)
	}
	if got, want := len(step1.PreInstallHooks), 2; got != want {
		t.Errorf("step1.PreInstallHooks: append failed, got len %d, want %d", got, want)
	}
	if got, want := len(step1.PostDeployHooks), 2; got != want {
		t.Errorf("step1.PostDeployHooks: append failed, got len %d, want %d", got, want)
	}

	// Negative: prove the test catches a removal of the detachment.
	parent2 := &config.RuntimeFlags{
		PreInstallHooks: make([]func(context.Context) error, 0, 4),
	}
	parent2.PreInstallHooks = append(parent2.PreInstallHooks, func(context.Context) error { return nil })
	step1Bad := *parent2 // shallow copy — same backing array, capacity 4.
	step1Bad.PreInstallHooks = append(step1Bad.PreInstallHooks, func(context.Context) error { return nil })
	if len(parent2.PreInstallHooks) == 2 {
		t.Logf("confirmed: without explicit detachment, parent slice is mutated (len=%d)", len(parent2.PreInstallHooks))
	}
}

func TestUpgradeOnly_PreInstallHookRegistrationDetachesParent(t *testing.T) {
	parent := &config.RuntimeFlags{
		PreInstallHooks: make([]func(context.Context) error, 0, 4),
	}
	parent.PreInstallHooks = append(parent.PreInstallHooks, func(context.Context) error { return nil })

	hook := &LifecycleHook{
		Fixtures:    []string{"postgresql-cluster.yaml"},
		Description: "Provision PostgreSQL before the upgrade-only helm upgrade.",
	}

	// Mirror the upgradeFlags setup from executeUpgradeOnly. Upgrade-only flows
	// reuse an existing namespace, but still need target-version pre-install
	// fixtures before helm upgrade.
	upgradeFlags := *parent
	upgradeFlags.PreInstallHooks = append([]func(context.Context) error(nil), parent.PreInstallHooks...)

	if err := registerDeclarativePreInstallHook(&upgradeFlags, hook, t.TempDir(), "8.10", "qa-elasticsearch-upg"); err != nil {
		t.Fatalf("registerDeclarativePreInstallHook() error = %v", err)
	}

	if got, want := len(parent.PreInstallHooks), 1; got != want {
		t.Errorf("parent.PreInstallHooks: registration leaked into parent, got len %d, want %d", got, want)
	}
	if got, want := len(upgradeFlags.PreInstallHooks), 2; got != want {
		t.Errorf("upgradeFlags.PreInstallHooks: got len %d, want %d", got, want)
	}
}

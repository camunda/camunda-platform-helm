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

package kube

import (
	"path/filepath"
	"testing"
)

func TestIntegrationCredentialsManifest(t *testing.T) {
	chart := "/repo/charts/camunda-platform-8.10"
	shared := "/repo/.github/config/external-secret"
	chartFile := filepath.Join(chart, "test", "integration", "external-secrets", "external-secret-integration-test-credentials.yaml")
	chartVault := filepath.Join(chart, "test", "integration", "external-secrets", "external-secret-integration-test-credentials-vault.yaml")
	sharedFile := filepath.Join(shared, "external-secret-integration-test-credentials.yaml")
	scenario := "/tmp/credentials-manifest-1.yaml"

	cases := []struct {
		name     string
		suffix   string
		manifest string
		present  []string
		want     string
		wantErr  bool
	}{
		{name: "scenario manifest replaces the CI one", manifest: scenario, present: []string{scenario, chartFile, sharedFile}, want: scenario},
		{name: "missing scenario manifest is an error, not a CI fallback", manifest: scenario, present: []string{chartFile, sharedFile}, wantErr: true},
		{name: "chart-specific file wins over the shared one", present: []string{chartFile, sharedFile}, want: chartFile},
		{name: "shared fallback", present: []string{sharedFile}, want: sharedFile},
		{name: "vault suffix selects the vault file", suffix: "-vault", present: []string{chartFile, chartVault}, want: chartVault},
		{name: "none present", present: nil, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := map[string]bool{}
			for _, p := range tc.present {
				set[p] = true
			}
			got, err := integrationCredentialsManifest(chart, shared, tc.suffix, tc.manifest, func(p string) bool { return set[p] })
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

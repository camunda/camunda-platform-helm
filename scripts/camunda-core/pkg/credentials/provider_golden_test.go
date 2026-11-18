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
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate golden files")

// TestCredentialsProviderGolden records Describe() and the manifest each
// provider variant selects for a chart that has its own manifest, one that
// only has the shared fallback, and one that has neither.
//
//	cd scripts/camunda-core && go test ./pkg/credentials/ -run TestCredentialsProviderGolden -update-golden
func TestCredentialsProviderGolden(t *testing.T) {
	present := map[string]bool{
		"charts/own/test/integration/external-secrets/external-secret-integration-test-credentials.yaml":       true,
		"charts/own/test/integration/external-secrets/external-secret-integration-test-credentials-vault.yaml": true,
		"shared/external-secret-integration-test-credentials.yaml":                                             true,
		"shared/external-secret-integration-test-credentials-vault.yaml":                                       true,
		"scenario/credentials.yaml": true,
	}
	exists := func(p string) bool { return present[p] }
	cases := []struct {
		name     string
		provider ESO
	}{
		{"eso-default", ESO{}},
		{"eso-named-store", ESO{Store: "distribution-team"}},
		{"eso-vault", ESO{Store: VaultStore}},
		{"eso-override", ESO{Override: "scenario/credentials.yaml"}},
		{"eso-vault-override", ESO{Store: VaultStore, Override: "scenario/credentials.yaml"}},
		{"eso-missing-override", ESO{Override: "scenario/absent.yaml"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p Provider = tc.provider
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "name: %s\ndescribe: %s\n", p.Name(), p.Describe())
			fmt.Fprintf(&buf, "chart-manifest: %s\n", tc.provider.ChartManifest("charts/own"))
			for _, chart := range []string{"charts/own", "charts/none"} {
				for _, dir := range []string{"shared", "empty"} {
					file, err := tc.provider.Manifest(chart, dir, exists)
					fmt.Fprintf(&buf, "manifest(%s, %s): %q err=%v\n", chart, dir, file, err)
				}
			}
			path := filepath.Join("testdata", "provider", tc.name+".golden")
			got := buf.Bytes()
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v (rerun with -update-golden)", path, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s drifted:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

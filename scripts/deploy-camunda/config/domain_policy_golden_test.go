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
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate golden files")

// TestDomainPolicyGolden records the exact Validate error strings, the
// required-domain error and the prune suffixes of each DomainPolicy variant.
//
//	cd scripts/deploy-camunda && go test ./config/ -run TestDomainPolicyGolden -update-golden
func TestDomainPolicyGolden(t *testing.T) {
	inputs := []string{
		"ci.distro.ultrawombat.com",
		"distribution.aws.camunda.cloud",
		"example.com",
		"*",
	}
	cases := []struct {
		name   string
		policy DomainPolicy
	}{
		{"default", DefaultDomainPolicy()},
		{"custom", DomainPolicy{Allowed: []string{"apps.example.com", "test.example.org"}}},
		{"star", DomainPolicy{Allowed: []string{"*"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "domains: %s\n", strings.Join(tc.policy.Domains(), ", "))
			fmt.Fprintf(&buf, "prune-suffixes: %s\n", strings.Join(tc.policy.PruneSuffixes(), ", "))
			fmt.Fprintf(&buf, "required: %v\n", tc.policy.RequiredError())
			for _, in := range inputs {
				fmt.Fprintf(&buf, "validate %q: %v\n", in, tc.policy.Validate("ingress-base-domain", in))
			}
			assertGolden(t, filepath.Join("testdata", "domain-policy", tc.name+".golden"), buf.Bytes())
		})
	}
}

func TestDefaultDomainPolicyKeepsLegacyErrors(t *testing.T) {
	legacy := strings.Join([]string{"ci.distro.ultrawombat.com", "distribution.aws.camunda.cloud"}, ", ")
	if got, want := DefaultDomainPolicy().Validate("ingress-base-domain-gke", "x").Error(), "--ingress-base-domain-gke must be one of: "+legacy; got != want {
		t.Errorf("Validate = %q, want %q", got, want)
	}
	if got, want := DefaultDomainPolicy().RequiredError().Error(), "--ingress-base-domain is required when using --ingress-subdomain; valid values: "+legacy; got != want {
		t.Errorf("RequiredError = %q, want %q", got, want)
	}
	if &ValidIngressBaseDomains[0] != &DefaultIngressBaseDomains[0] {
		t.Error("ValidIngressBaseDomains no longer aliases DefaultIngressBaseDomains")
	}
	if DefaultDomainPolicy().PruneSuffixes()[0] != ".ci.distro.ultrawombat.com" {
		t.Error("default policy's first prune suffix diverged from entra's ciDomainSuffix")
	}
}

func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
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
}

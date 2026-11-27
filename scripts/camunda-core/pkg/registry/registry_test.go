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

package registry

import "testing"

func TestChainResolve(t *testing.T) {
	env := map[string]string{"B": "fromB", "C": "fromC", "Q": "q"}
	lookup := func(n string) string { return env[n] }
	c := Chain{Username: []string{"A", "B", "C"}, Password: []string{"P", "Q"}}
	cases := []struct {
		name, flagUser, flagPass, wantUser, wantPass string
	}{
		{"flags win", "u", "p", "u", "p"},
		{"first non-empty env", "", "", "fromB", "q"},
		{"fields resolve independently", "u", "", "u", "q"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, p := c.Resolve(tc.flagUser, tc.flagPass, lookup)
			if u != tc.wantUser || p != tc.wantPass {
				t.Errorf("Resolve = (%q, %q), want (%q, %q)", u, p, tc.wantUser, tc.wantPass)
			}
		})
	}
	if u, p := (Chain{Username: []string{"Z"}}).Resolve("", "", lookup); u != "" || p != "" {
		t.Errorf("unset chain = (%q, %q), want empty", u, p)
	}
}

func TestHarborProviderKeepsPullSecretContract(t *testing.T) {
	if Harbor.Host != "registry.camunda.cloud" || Harbor.PullSecret != "registry-"+"camunda-cloud" {
		t.Errorf("Harbor = %+v", Harbor)
	}
}

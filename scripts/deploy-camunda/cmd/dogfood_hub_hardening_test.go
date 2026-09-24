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
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The public-host block on Keycloak's admin API and master realm is the control
// that keeps the internet-facing dogfood Keycloak from being administered or
// brute-forced through its master realm. Pin its shape.
func TestDogfoodHub_BlocksKeycloakAdminPublicly(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "charts", "camunda-platform-8.10", "test", "integration",
		"scenarios", "chart-full-setup", "values", "features", "dogfood-hub.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var layer struct {
		Global struct {
			ExtraManifests []string `yaml:"extraManifests"`
		} `yaml:"global"`
	}
	if err := yaml.Unmarshal(raw, &layer); err != nil {
		t.Fatal(err)
	}

	type backend struct {
		Service struct {
			Name string `yaml:"name"`
		} `yaml:"service"`
	}
	var blocked []string
	serviceHasSelector := map[string]bool{}
	for _, m := range layer.Global.ExtraManifests {
		var obj struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Spec struct {
				Selector map[string]any `yaml:"selector"`
				Rules    []struct {
					HTTP struct {
						Paths []struct {
							Path     string  `yaml:"path"`
							PathType string  `yaml:"pathType"`
							Backend  backend `yaml:"backend"`
						} `yaml:"paths"`
					} `yaml:"http"`
				} `yaml:"rules"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal([]byte(m), &obj); err != nil {
			t.Fatalf("extraManifest does not parse: %v", err)
		}
		switch obj.Kind {
		case "Service":
			serviceHasSelector[obj.Metadata.Name] = len(obj.Spec.Selector) > 0
		case "Ingress":
			for _, r := range obj.Spec.Rules {
				for _, p := range r.HTTP.Paths {
					if !strings.HasPrefix(p.Path, "/auth") {
						continue
					}
					if p.Backend.Service.Name != "keycloak-admin-blocked" || p.PathType != "Prefix" {
						t.Errorf("%s routes to %q (%s), want Prefix to keycloak-admin-blocked", p.Path, p.Backend.Service.Name, p.PathType)
					}
					blocked = append(blocked, p.Path)
				}
			}
		}
	}
	sort.Strings(blocked)
	want := []string{"/auth/admin/realms", "/auth/admin/serverinfo", "/auth/realms/master"}
	if strings.Join(blocked, ",") != strings.Join(want, ",") {
		t.Errorf("blocked paths = %v, want exactly %v (anything broader breaks the camunda-platform realm or Identity's wait)", blocked, want)
	}
	hasSelector, found := serviceHasSelector["keycloak-admin-blocked"]
	if !found || hasSelector {
		t.Errorf("keycloak-admin-blocked Service must exist without a selector, so it has no endpoints (found=%v selector=%v)", found, hasSelector)
	}
}

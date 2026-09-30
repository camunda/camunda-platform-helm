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
	want := []string{"/auth/admin", "/auth/realms/master"}
	if strings.Join(blocked, ",") != strings.Join(want, ",") {
		t.Errorf("blocked paths = %v, want exactly %v (anything broader breaks the camunda-platform realm)", blocked, want)
	}
	// Prefix matching must cover the admin console pages as well as the admin
	// REST API, and must leave the camunda-platform realm reachable.
	covered := func(path string) bool {
		for _, b := range blocked {
			if path == b || strings.HasPrefix(path, b+"/") {
				return true
			}
		}
		return false
	}
	for _, p := range []string{"/auth/admin/", "/auth/admin/master/console/", "/auth/admin/realms/master", "/auth/admin/serverinfo", "/auth/realms/master/protocol/openid-connect/token"} {
		if !covered(p) {
			t.Errorf("%s must be blocked on the public host", p)
		}
	}
	for _, p := range []string{"/auth/realms/camunda-platform/protocol/openid-connect/auth", "/auth/resources/"} {
		if covered(p) {
			t.Errorf("%s must stay reachable on the public host", p)
		}
	}
	hasSelector, found := serviceHasSelector["keycloak-admin-blocked"]
	if !found || hasSelector {
		t.Errorf("keycloak-admin-blocked Service must exist without a selector, so it has no endpoints (found=%v selector=%v)", found, hasSelector)
	}
}

// The realm-hardening hook is where brute-force protection, the password policy
// and the master realm's port-forward frontend URL come from: Keycloak keeps
// these in its database, so nothing else re-applies them after an upgrade.
func TestDogfoodHub_HardensKeycloakRealms(t *testing.T) {
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
	var script, hook string
	for _, m := range layer.Global.ExtraManifests {
		var job struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name        string            `yaml:"name"`
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"metadata"`
			Spec struct {
				Template struct {
					Spec struct {
						Containers []struct {
							Args []string `yaml:"args"`
						} `yaml:"containers"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal([]byte(m), &job); err != nil {
			t.Fatal(err)
		}
		if job.Kind == "Job" && job.Metadata.Name == "keycloak-realm-hardening" {
			hook = job.Metadata.Annotations["helm.sh/hook"]
			script = strings.Join(job.Spec.Template.Spec.Containers[0].Args, "\n")
		}
	}
	if hook != "post-install,post-upgrade" {
		t.Fatalf("keycloak-realm-hardening hook = %q, want post-install,post-upgrade", hook)
	}
	if strings.Contains(script, "$") {
		t.Error("the script must not contain a dollar sign: the values pipeline substitutes it before Helm sees it")
	}
	for _, realm := range []string{"master", "camunda-platform"} {
		var line string
		for _, l := range strings.Split(script, "\n") {
			if strings.Contains(l, "update realms/"+realm+" ") && strings.Contains(l, "bruteForceProtected") {
				line = l
			}
		}
		for _, want := range []string{"bruteForceProtected=true", "permanentLockout=false", "passwordPolicy=length(12) and notUsername"} {
			if !strings.Contains(line, want) {
				t.Errorf("realm %s: hardening command lacks %q", realm, want)
			}
		}
	}
	lines := strings.Split(strings.TrimSpace(script), "\n")
	if len(lines) < 2 || !strings.Contains(lines[len(lines)-2], "attributes.frontendUrl=http://localhost:18080/auth") {
		t.Error("setting the master realm frontend URL must be the last kcadm call: it changes the issuer and invalidates the session's token")
	}
}

// Dogfood Elasticsearch runs with X-Pack security on because the CI cluster does
// not enforce NetworkPolicy. Pin the auth contract end to end: the server takes
// its elastic password from integration-test-credentials, every consumer reads
// the same key, and the physical-tenant exporters keep a runtime Spring
// placeholder instead of receiving the password in values.
func TestDogfood_ElasticsearchAuthContract(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	read := func(parts ...string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(append([]string{repo}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	var server struct {
		ESConfig  map[string]string `yaml:"esConfig"`
		ExtraEnvs []struct {
			Name      string `yaml:"name"`
			ValueFrom struct {
				SecretKeyRef struct {
					Name string `yaml:"name"`
					Key  string `yaml:"key"`
				} `yaml:"secretKeyRef"`
			} `yaml:"valueFrom"`
		} `yaml:"extraEnvs"`
	}
	if err := yaml.Unmarshal([]byte(read("test", "integration", "companion-values", "elasticsearch-dogfood.yaml")), &server); err != nil {
		t.Fatal(err)
	}
	esYAML := server.ESConfig["elasticsearch.yml"]
	if !strings.Contains(esYAML, "xpack.security.enabled: true") {
		t.Error("elasticsearch-dogfood.yaml must enable X-Pack security")
	}
	if strings.Contains(esYAML, "reindex.remote.whitelist") {
		t.Error("elasticsearch-dogfood.yaml must not allow remote reindex: dogfood is install-only")
	}
	var passwordRef string
	for _, e := range server.ExtraEnvs {
		if e.Name == "ELASTIC_PASSWORD" {
			passwordRef = e.ValueFrom.SecretKeyRef.Name + "/" + e.ValueFrom.SecretKeyRef.Key
		}
	}
	if passwordRef != "integration-test-credentials/elasticsearch-password" {
		t.Errorf("ELASTIC_PASSWORD comes from %q, want integration-test-credentials/elasticsearch-password", passwordRef)
	}

	featuresDir := filepath.Join(repo, "charts", "camunda-platform-8.10", "test", "integration", "scenarios", "chart-full-setup", "values", "features")
	consumers, err := filepath.Glob(filepath.Join(featuresDir, "dogfood-o*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(consumers) != 8 {
		t.Fatalf("found %d dogfood orchestration/optimize layers, want 8: %v", len(consumers), consumers)
	}
	for _, path := range consumers {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content := string(raw)
		if !strings.Contains(content, "existingSecret: integration-test-credentials") ||
			!strings.Contains(content, "existingSecretKey: elasticsearch-password") {
			t.Errorf("%s: Elasticsearch auth must read integration-test-credentials/elasticsearch-password", filepath.Base(path))
		}
	}

	pt := read("charts", "camunda-platform-8.10", "test", "integration", "scenarios", "chart-full-setup", "values", "features", "dogfood-orchestration-pt.yaml")
	passwords := 0
	for _, line := range strings.Split(pt, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "password:") {
			passwords++
			if trimmed != `password: "${ELASTICSEARCH_PASSWORD_REF}"` {
				t.Errorf("dogfood-orchestration-pt.yaml: exporter %q, want the ELASTICSEARCH_PASSWORD_REF placeholder", trimmed)
			}
		}
	}
	if passwords != 3 {
		t.Errorf("dogfood-orchestration-pt.yaml: %d exporter passwords, want 3 (default, tenanta, tenantb)", passwords)
	}

	var scenario struct {
		Topology struct {
			Releases []struct {
				NamespaceSuffix string            `yaml:"namespace-suffix"`
				Env             map[string]string `yaml:"env"`
			} `yaml:"releases"`
		} `yaml:"topology"`
	}
	if err := yaml.Unmarshal([]byte(read("charts", "camunda-platform-8.10", "test", "ci", "registry", "scenarios", "dogfood.yaml")), &scenario); err != nil {
		t.Fatal(err)
	}
	var ref string
	for _, r := range scenario.Topology.Releases {
		if r.NamespaceSuffix == "pt" {
			ref = r.Env["ELASTICSEARCH_PASSWORD_REF"]
		}
	}
	// The values pipeline expands placeholders in one os.Expand pass, so the
	// Spring placeholder in the variable's value reaches the chart verbatim.
	expanded := os.Expand(`password: "${ELASTICSEARCH_PASSWORD_REF}"`, func(name string) string {
		if name == "ELASTICSEARCH_PASSWORD_REF" {
			return ref
		}
		return "$" + name
	})
	if expanded != `password: "${VALUES_ELASTICSEARCH_PASSWORD}"` {
		t.Errorf("pt exporter password renders as %s, want the runtime Spring placeholder VALUES_ELASTICSEARCH_PASSWORD", expanded)
	}
}

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

package deploy

import (
	"testing"

	"scripts/deploy-camunda/config"
)

func TestBuildTopologyCrossRefEnv(t *testing.T) {
	hubCtx := &ScenarioContext{
		Namespace:     "matrix-810-mns-hub",
		KeycloakRealm: "mns-abcdef12",
	}

	env := BuildTopologyCrossRefEnv(hubCtx, "elasticsearch", "9200", "http")

	if got := env["HUB_NAMESPACE"]; got != "matrix-810-mns-hub" {
		t.Errorf("HUB_NAMESPACE = %q, want %q", got, "matrix-810-mns-hub")
	}
	if got := env["KEYCLOAK_REALM"]; got != "mns-abcdef12" {
		t.Errorf("KEYCLOAK_REALM = %q, want %q", got, "mns-abcdef12")
	}
	if got, want := env["EXTERNAL_ELASTICSEARCH_HOST"], "elasticsearch.matrix-810-mns-hub.svc.cluster.local"; got != want {
		t.Errorf("EXTERNAL_ELASTICSEARCH_HOST = %q, want %q", got, want)
	}
	if got := env["EXTERNAL_ELASTICSEARCH_PORT"]; got != "9200" {
		t.Errorf("EXTERNAL_ELASTICSEARCH_PORT = %q, want %q", got, "9200")
	}
	if got := env["EXTERNAL_ELASTICSEARCH_SCHEME"]; got != "http" {
		t.Errorf("EXTERNAL_ELASTICSEARCH_SCHEME = %q, want %q", got, "http")
	}
}

func TestBuildTopologyCrossRefEnv_NoSharedStorage(t *testing.T) {
	hubCtx := &ScenarioContext{Namespace: "ns-hub", KeycloakRealm: "realm"}
	env := BuildTopologyCrossRefEnv(hubCtx, "", "", "")

	if _, ok := env["EXTERNAL_ELASTICSEARCH_HOST"]; ok {
		t.Errorf("expected no EXTERNAL_ELASTICSEARCH_HOST when SharedStorageServiceName is empty, got %v", env)
	}
	if env["HUB_NAMESPACE"] != "ns-hub" || env["KEYCLOAK_REALM"] != "realm" {
		t.Errorf("expected HUB_NAMESPACE/KEYCLOAK_REALM to always be set, got %v", env)
	}
}

func TestGenerateTopologyContexts_Exported(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{Namespace: "matrix-810-mns"},
	}
	contexts, err := GenerateTopologyContexts("multinamespace", topologyTestReleases(), flags)
	if err != nil {
		t.Fatalf("GenerateTopologyContexts returned error: %v", err)
	}
	if len(contexts) != 3 {
		t.Fatalf("expected 3 contexts, got %d", len(contexts))
	}
}

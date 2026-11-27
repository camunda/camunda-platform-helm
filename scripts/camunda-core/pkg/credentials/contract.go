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

// Package credentials holds the integration-test credentials contract: the
// Secret every test namespace receives, the keys it carries, and the
// providers that make it exist.
package credentials

// SecretName is the Kubernetes Secret holding a namespace's integration-test credentials.
const SecretName = "integration-test-credentials"

// Keys every consumer outside the chart reads from SecretName.
const (
	KeyOIDCClient        = "client-secret"
	KeyFirstUser         = "identity-user-password"
	KeyKeycloakAdmin     = "identity-keycloak-admin-password"
	KeyVenomClient       = "identity-admin-client-password"
	KeyKeycloakPGUser    = "identity-keycloak-postgresql-user-password"
	KeyKeycloakPGAdmin   = "identity-keycloak-postgresql-admin-password"
	KeyWebModelerPGUser  = "webmodeler-postgresql-user-password"
	KeyWebModelerPGAdmin = "webmodeler-postgresql-admin-password"
)

// Keys is the key schema of SecretName, in the order of target.template.data in
// .github/config/external-secret/external-secret-integration-test-credentials.yaml.
var Keys = []string{
	"client-secret",
	"entra-child-app-client-secret",
	"identity-firstuser-password",
	"operate-secret",
	"tasklist-secret",
	"optimize-secret",
	"connectors-secret",
	"console-secret",
	"keycloak-secret",
	"zeebe-secret",
	"orchestration-secret",
	"admin-password",
	"management-password",
	"postgres-password",
	"password",
	"smtp-password",
	"identity-admin-client-token",
	"identity-connectors-client-token",
	"identity-console-client-token",
	"identity-orchestration-client-token",
	"identity-operate-client-token",
	"identity-optimize-client-token",
	"identity-optimize-default-client-token",
	"identity-optimize-tenanta-client-token",
	"identity-optimize-tenantb-client-token",
	"identity-tasklist-client-token",
	"identity-zeebe-client-token",
	"identity-admin-client-password",
	"identity-connectors-client-password",
	"identity-console-client-password",
	"identity-orchestration-client-password",
	"identity-operate-client-password",
	"identity-optimize-client-password",
	"identity-tasklist-client-password",
	"identity-zeebe-client-password",
	"identity-user-password",
	"identity-keycloak-admin-password",
	"identity-keycloak-postgresql-admin-password",
	"identity-keycloak-postgresql-user-password",
	"webmodeler-postgresql-admin-password",
	"webmodeler-postgresql-user-password",
	"identity-postgresql-admin-password",
	"identity-postgresql-user-password",
	"web-modeler-postgresql-admin-password",
	"web-modeler-postgresql-user-password",
	"identity-migration-client-secret",
}

// RequiredKeys are the keys deploy-camunda itself reads from SecretName to
// drive e2e and acceptance runs.
var RequiredKeys = []string{
	KeyOIDCClient,
	KeyVenomClient,
	KeyKeycloakAdmin,
	KeyFirstUser,
}

// MissingKeys returns the entries of required that have no non-empty value in
// data, in the order of required.
func MissingKeys(data map[string]string, required []string) []string {
	var missing []string
	for _, k := range required {
		if data[k] == "" {
			missing = append(missing, k)
		}
	}
	return missing
}

package deploy

import (
	"sort"
	"testing"

	"scripts/vault-secret-mapper/pkg/mapper"
)

// oldHardcodedMapping is the literal that previously lived in secrets.go. The
// embedded data file must produce the exact same set of required env vars.
const oldHardcodedMapping = "" +
	"ci/path DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD;" +
	"ci/path DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD;" +
	"ci/path DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD;" +
	"ci/path DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET;" +
	"ci/path IDP_AWS_ACCESSKEY;" +
	"ci/path IDP_AWS_BUCKET_NAME;" +
	"ci/path IDP_AWS_REGION;" +
	"ci/path IDP_AWS_SECRETKEY;" +
	"ci/path IDP_GCP_SERVICE_ACCOUNT;" +
	"ci/path IDP_GCP_VERTEX_BUCKET_NAME;" +
	"ci/path IDP_GCP_VERTEX_PROJECT_ID;" +
	"ci/path IDP_GCP_DOCUMENT_AI_PROJECT_ID;" +
	"ci/path IDP_GCP_DOCUMENT_AI_PROCESSOR_ID;" +
	"ci/path IDP_GCP_DOCUMENT_AI_REGION;" +
	"ci/path IDP_GCP_VERTEX_REGION;" +
	"ci/path IDP_AZURE_DOCUMENT_INTELLIGENCE_KEY;" +
	"ci/path IDP_AZURE_DOCUMENT_INTELLIGENCE_ENDPOINT;" +
	"ci/path IDP_AZURE_AI_FOUNDRY_ENDPOINT;" +
	"ci/path IDP_AZURE_AI_FOUNDRY_KEY;" +
	"ci/path IDP_AZURE_OPEN_AI_ENDPOINT;" +
	"ci/path IDP_AZURE_OPEN_AI_KEY;" +
	"ci/path OPENAI_API_KEY;"

func TestEmbeddedMappingMatchesLegacy(t *testing.T) {
	got, err := embeddedTestSecretMapping()
	if err != nil {
		t.Fatalf("embeddedTestSecretMapping: %v", err)
	}

	gotVars := mapper.RequiredEnvVars(got)
	wantVars := mapper.RequiredEnvVars(oldHardcodedMapping)
	sort.Strings(gotVars)
	sort.Strings(wantVars)

	if len(gotVars) != len(wantVars) {
		t.Fatalf("var count mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(gotVars), len(wantVars), gotVars, wantVars)
	}
	for i := range wantVars {
		if gotVars[i] != wantVars[i] {
			t.Errorf("var[%d] = %q, want %q", i, gotVars[i], wantVars[i])
		}
	}
}

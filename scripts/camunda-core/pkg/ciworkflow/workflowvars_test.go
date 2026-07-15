// Copyright 2026 Camunda Services GmbH
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

package ciworkflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/camunda-core/pkg/ghactions"
)

// infraFixture is a frozen copy of .github/config/infra.yaml so expected
// values below never drift with live infrastructure changes.
const infraFixture = `gke:
  ingress-hostname-base: ci.distro.ultrawombat.com
  namespace-prefix: camunda

eks:
  ingress-hostname-base: distribution.aws.camunda.cloud
  namespace-prefix: distribution
  cluster-name: camunda-ci-eks
  aws-profile: distribution

rosa:
  ingress-hostname-base: ci.distro.ultrawombat.com
  namespace-prefix: camunda

postgresql:
  jdbc-host: postgresql-cluster-rw.distribution-postgresql.svc.cluster.local
  jdbc-port: "5432"

teleport:
  proxy: camunda.teleport.sh:443
`

func loadTestInfra(t *testing.T) InfraConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "infra.yaml")
	if err := os.WriteFile(path, []byte(infraFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	infra, err := LoadInfraConfig(path)
	if err != nil {
		t.Fatalf("load infra: %v", err)
	}
	return infra
}

func chartsRepoRoot(t *testing.T, dirs ...string) string {
	t.Helper()
	repoRoot := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(repoRoot, "charts", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repoRoot
}

// Expected values were captured from the original composite-action shell
// steps (bash 5, yq v4.53.3) running on identical inputs with infraFixture
// and a charts dir containing camunda-platform-8.9 and -8.10.
func TestComputeWorkflowVarsBashParity(t *testing.T) {
	infra := loadTestInfra(t)
	repoRoot := chartsRepoRoot(t, "camunda-platform-8.9", "camunda-platform-8.10")

	for _, tc := range []struct {
		name string
		in   WorkflowVarsInput
		want WorkflowVars
	}{
		{
			name: "gke-install-pr",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "install", DeploymentTTL: "1h", IdentifierBase: "6598", PRNumber: "1234", RunID: "16400000001", Flow: "install"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "gke", JobID: "b73169", Namespace: "camunda-pr-6598", Identifier: "gke-6598", IngressHost: "gke-6598.ci.distro.ultrawombat.com", Flow: "install"},
		},
		{
			name: "gke-install-nottl",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "install", IdentifierBase: "6598", RunID: "16400000002", Flow: "install"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "gke", JobID: "20203d", Namespace: "camunda-id-6598-20203d", Identifier: "gke-6598", IngressHost: "20203d-gke-6598.ci.distro.ultrawombat.com", Flow: "install"},
		},
		{
			name: "eks-install-pr",
			in:   WorkflowVarsInput{Platform: "EKS", SetupFlow: "install", DeploymentTTL: "1h", IdentifierBase: "6598", PRNumber: "1234", RunID: "16400000003", Flow: "install"},
			want: WorkflowVars{NamespacePrefix: "distribution", Platform: "eks", JobID: "b87450", Namespace: "distribution-pr-6598", Identifier: "eks-6598", IngressHost: "eks-6598.distribution.aws.camunda.cloud", Flow: "install"},
		},
		{
			name: "gke-upgpatch-nottl",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "upgrade-patch", IdentifierBase: "6598", RunID: "16400000004", Flow: "upgrade-patch"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "gke", JobID: "71bf01", Namespace: "camunda-id-6598-71bf01-upgp", Identifier: "gke-6598-upgp", IngressHost: "71bf01-gke-6598-upgp.ci.distro.ultrawombat.com", Flow: "upgrade-patch"},
		},
		{
			name: "gke-upgminor-nottl",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "upgrade-minor", IdentifierBase: "6598", RunID: "16400000005", Flow: "upgrade-minor"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "gke", JobID: "7a6e94", Namespace: "camunda-id-6598-7a6e94-upgm", Identifier: "gke-6598-upgm", IngressHost: "7a6e94-gke-6598-upgm.ci.distro.ultrawombat.com", Flow: "upgrade-minor"},
		},
		{
			name: "gke-modupgminor-nottl",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "modular-upgrade-minor", IdentifierBase: "6598", RunID: "16400000006", Flow: "modular-upgrade-minor"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "gke", JobID: "668ceb", Namespace: "camunda-id-6598-668ceb", Identifier: "gke-6598", IngressHost: "668ceb-gke-6598.ci.distro.ultrawombat.com", Flow: "install"},
		},
		{
			name: "rosa-install-id",
			in:   WorkflowVarsInput{Platform: "rosa,gke", SetupFlow: "install", DeploymentTTL: "1h", IdentifierBase: "nightly-8.10", PRNumber: "987", RunID: "16400000007", Flow: "install"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "rosa", JobID: "312fef", Namespace: "camunda-pr-nightly-8-10", Identifier: "rosa-nightly-8-10", IngressHost: "rosa-nightly-8-10.ci.distro.ultrawombat.com", Flow: "install"},
		},
		{
			name: "gke-longid-nottl",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "upgrade-minor", IdentifierBase: "integration-oidc-multitenancy-documentstore-very-long-name-x", RunID: "16400000008", Flow: "upgrade-minor"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "gke", JobID: "97b460", Namespace: "camunda-id-integration-oidc-multitenancy-documentst-97b460-upgm", Identifier: "gke-integration-oidc-multitenancy-documentstore-very-long-name-x-upgm", IngressHost: "97b460-gke-integration-oidc-multitenancy-documentstore-very-long-name-x-upgm.ci.distro.ultrawombat.com", Flow: "upgrade-minor"},
		},
		{
			name: "gke-prefix-override",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "install", IdentifierBase: "6598", Prefix: "distribution", RunID: "16400000009", Flow: "install"},
			want: WorkflowVars{NamespacePrefix: "distribution", Platform: "gke", JobID: "7820f5", Namespace: "distribution-id-6598-7820f5", Identifier: "gke-6598", IngressHost: "7820f5-gke-6598.ci.distro.ultrawombat.com", Flow: "install"},
		},
		{
			name: "gke-dots-id",
			in:   WorkflowVarsInput{Platform: "gke", SetupFlow: "install", IdentifierBase: "venom.8.10.test", RunID: "16400000010", Flow: "install"},
			want: WorkflowVars{NamespacePrefix: "camunda", Platform: "gke", JobID: "c36c12", Namespace: "camunda-id-venom-8-10-test-c36c12", Identifier: "gke-venom-8-10-test", IngressHost: "c36c12-gke-venom-8-10-test.ci.distro.ultrawombat.com", Flow: "install"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			in.RepoRoot = repoRoot
			got, err := ComputeWorkflowVars(in, infra)
			if err != nil {
				t.Fatalf("ComputeWorkflowVars: %v", err)
			}
			for _, f := range []struct{ name, got, want string }{
				{"NamespacePrefix", got.NamespacePrefix, tc.want.NamespacePrefix},
				{"Platform", got.Platform, tc.want.Platform},
				{"JobID", got.JobID, tc.want.JobID},
				{"Namespace", got.Namespace, tc.want.Namespace},
				{"Identifier", got.Identifier, tc.want.Identifier},
				{"IngressHost", got.IngressHost, tc.want.IngressHost},
				{"Flow", got.Flow, tc.want.Flow},
			} {
				if f.got != f.want {
					t.Errorf("%s = %q, want %q", f.name, f.got, f.want)
				}
			}
		})
	}
}

// TestEmitWorkflowContract pins the full set of variable names and values the
// action contract exposes to workflows, byte-compared against the bash-captured
// output for the gke-install-pr case.
func TestEmitWorkflowContract(t *testing.T) {
	infra := loadTestInfra(t)
	repoRoot := chartsRepoRoot(t, "camunda-platform-8.9", "camunda-platform-8.10")
	vars, err := ComputeWorkflowVars(WorkflowVarsInput{
		Platform: "gke", SetupFlow: "install", DeploymentTTL: "1h",
		IdentifierBase: "6598", PRNumber: "1234", RunID: "16400000001",
		Flow: "install", RepoRoot: repoRoot,
	}, infra)
	if err != nil {
		t.Fatalf("ComputeWorkflowVars: %v", err)
	}

	envFile := filepath.Join(t.TempDir(), "env")
	outFile := filepath.Join(t.TempDir(), "out")
	if err := vars.Emit(&ghactions.Writer{Path: envFile}, &ghactions.Writer{Path: outFile}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	wantEnv := `INFRA_INGRESS_HOSTNAME_BASE=ci.distro.ultrawombat.com
INFRA_NAMESPACE_PREFIX=camunda
INFRA_CLUSTER_NAME=camunda-ci-eks
CLUSTER_NAME=camunda-ci-eks
AWS_PROFILE=distribution
POSTGRESQL_JDBC_URL=jdbc:postgresql://postgresql-cluster-rw.distribution-postgresql.svc.cluster.local:5432
INFRA_TELEPORT_PROXY=camunda.teleport.sh:443
NAMESPACE_PREFIX=camunda
PLATFORM=gke
GITHUB_WORKFLOW_JOB_ID=b73169
GITHUB_WORKFLOW_RUN_ID=16400000001
TEST_NAMESPACE=camunda-pr-6598
TEST_CAMUNDA_HELM_DIR_ALPHA=camunda-platform-8.10
ORCHESTRATION_INDEX_PREFIX=b73169-orchestration
OPTIMIZE_INDEX_PREFIX=b73169-optimize
OPERATE_INDEX_PREFIX=b73169-operate
TASKLIST_INDEX_PREFIX=b73169-tasklist
FLOW=install
KEYCLOAK_REALM=b73169-realm
`
	wantOut := `namespace=camunda-pr-6598
identifier=gke-6598
ingress-host=gke-6598.ci.distro.ultrawombat.com
`
	assertFileContent(t, envFile, wantEnv)
	assertFileContent(t, outFile, wantOut)
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s content differs\n--- want\n%s\n--- got\n%s", filepath.Base(path), want, got)
	}
}

func TestComputeWorkflowVarsEmptyIdentifierUsesRandomID(t *testing.T) {
	infra := loadTestInfra(t)
	vars, err := ComputeWorkflowVars(WorkflowVarsInput{
		Platform: "gke", SetupFlow: "install", DeploymentTTL: "1h",
		RunID: "1", Flow: "install", RandomID: "abc123",
		RepoRoot: chartsRepoRoot(t, "camunda-platform-8.10"),
	}, infra)
	if err != nil {
		t.Fatal(err)
	}
	if vars.Identifier != "gke-no-id-use-ran-abc123" {
		t.Errorf("identifier = %q", vars.Identifier)
	}
}

func TestComputeWorkflowVarsPlatformValidation(t *testing.T) {
	infra := loadTestInfra(t)
	for _, tc := range []struct {
		platform string
		wantErr  string
	}{
		{"", "must not be empty"},
		{"aks", "not found"},
	} {
		_, err := ComputeWorkflowVars(WorkflowVarsInput{Platform: tc.platform}, infra)
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("platform %q: err = %v, want containing %q", tc.platform, err, tc.wantErr)
		}
	}
}

func TestComputeWorkflowVarsRequiredInputs(t *testing.T) {
	infra := loadTestInfra(t)
	if _, err := ComputeWorkflowVars(WorkflowVarsInput{Platform: "gke"}, infra); err == nil || !strings.Contains(err.Error(), "run ID must not be empty") {
		t.Fatalf("empty run ID: err = %v", err)
	}

	delete(infra.Platforms, "eks")
	if _, err := ComputeWorkflowVars(WorkflowVarsInput{Platform: "gke", RunID: "1"}, infra); err == nil || !strings.Contains(err.Error(), `platform "eks" not found`) {
		t.Fatalf("missing EKS config: err = %v", err)
	}
}

func TestAlphaChartDirVersionSort(t *testing.T) {
	repoRoot := chartsRepoRoot(t, "camunda-platform-8.2", "camunda-platform-8.9", "camunda-platform-8.10", "camunda-platform-8.11")
	got, err := alphaChartDir(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got != "camunda-platform-8.11" {
		t.Errorf("alphaChartDir = %q, want camunda-platform-8.11", got)
	}
}

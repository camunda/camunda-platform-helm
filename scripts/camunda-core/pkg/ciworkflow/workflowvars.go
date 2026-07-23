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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"scripts/camunda-core/pkg/ghactions"

	"gopkg.in/yaml.v3"
)

// InfraPlatform is one platform section of .github/config/infra.yaml.
type InfraPlatform struct {
	IngressHostnameBase string `yaml:"ingress-hostname-base"`
	NamespacePrefix     string `yaml:"namespace-prefix"`
	ClusterName         string `yaml:"cluster-name"`
	AWSProfile          string `yaml:"aws-profile"`
}

// InfraConfig is the parsed .github/config/infra.yaml.
type InfraConfig struct {
	Platforms  map[string]InfraPlatform
	PostgreSQL struct {
		JDBCHost string `yaml:"jdbc-host"`
		JDBCPort string `yaml:"jdbc-port"`
	}
	TeleportProxy string
}

// LoadInfraConfig parses .github/config/infra.yaml. Non-platform sections
// (postgresql, teleport) are extracted; every other top-level map is a
// platform entry.
func LoadInfraConfig(path string) (InfraConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return InfraConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	var sections map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &sections); err != nil {
		return InfraConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg := InfraConfig{Platforms: map[string]InfraPlatform{}}
	for name, node := range sections {
		var sectionErr error
		switch name {
		case "postgresql":
			sectionErr = node.Decode(&cfg.PostgreSQL)
		case "teleport":
			var tp struct {
				Proxy string `yaml:"proxy"`
			}
			sectionErr = node.Decode(&tp)
			cfg.TeleportProxy = tp.Proxy
		default:
			var p InfraPlatform
			sectionErr = node.Decode(&p)
			cfg.Platforms[name] = p
		}
		if sectionErr != nil {
			return InfraConfig{}, fmt.Errorf("parse %s section %q: %w", path, name, sectionErr)
		}
	}
	return cfg, nil
}

// WorkflowVarsInput carries the workflow-vars composite-action inputs plus the
// ambient GitHub context values the computation depends on.
type WorkflowVarsInput struct {
	// Platform is the deployment platform input; may be a comma-separated
	// list and any case — only the first entry, lowercased, is used.
	Platform string
	// SetupFlow is install, upgrade-patch, upgrade-minor, or modular-upgrade-minor.
	SetupFlow string
	// DeploymentTTL is the deployment lifespan input; empty selects the
	// per-run hashed namespace/ingress variant.
	DeploymentTTL string
	// IdentifierBase is the fixed identifier part (PR number or a name).
	IdentifierBase string
	// Prefix overrides the platform namespace prefix when non-empty.
	Prefix string
	// PRNumber is github.event.pull_request.number; empty on non-PR events.
	PRNumber string
	// RunID is github.run_id.
	RunID string
	// Flow is the ambient $FLOW environment value re-emitted to GITHUB_ENV.
	Flow string
	// RandomID replaces an empty IdentifierBase ("no-id-use-ran-<RandomID>").
	RandomID string
	// RepoRoot is the directory chart paths are resolved against. Empty means ".".
	RepoRoot string
}

// WorkflowVars is the computed result, consumable directly as a struct and
// emittable to $GITHUB_ENV / $GITHUB_OUTPUT via Emit.
type WorkflowVars struct {
	IngressHostnameBase  string
	NamespacePrefixInfra string
	ClusterName          string
	AWSProfile           string
	PostgresJDBCURL      string
	TeleportProxy        string
	NamespacePrefix      string
	Platform             string
	JobID                string
	RunID                string
	Namespace            string
	AlphaChartDir        string
	Identifier           string
	IngressHost          string
	Flow                 string
	KeycloakRealm        string
}

// ComputeWorkflowVars reproduces the "Load infra config" and "Set workflow
// vars" shell steps of the workflow-vars composite action.
func ComputeWorkflowVars(in WorkflowVarsInput, infra InfraConfig) (WorkflowVars, error) {
	platform, err := normalizePlatform(in.Platform, infra)
	if err != nil {
		return WorkflowVars{}, err
	}
	if in.RunID == "" {
		return WorkflowVars{}, fmt.Errorf("run ID must not be empty")
	}
	eks, ok := infra.Platforms["eks"]
	if !ok {
		return WorkflowVars{}, fmt.Errorf("platform %q not found in infra config", "eks")
	}
	p := infra.Platforms[platform]

	out := WorkflowVars{
		IngressHostnameBase:  p.IngressHostnameBase,
		NamespacePrefixInfra: p.NamespacePrefix,
		ClusterName:          eks.ClusterName,
		AWSProfile:           eks.AWSProfile,
		PostgresJDBCURL:      fmt.Sprintf("jdbc:postgresql://%s:%s", infra.PostgreSQL.JDBCHost, infra.PostgreSQL.JDBCPort),
		TeleportProxy:        infra.TeleportProxy,
		Platform:             platform,
		RunID:                in.RunID,
	}

	out.NamespacePrefix = in.Prefix
	if out.NamespacePrefix == "" {
		out.NamespacePrefix = p.NamespacePrefix
	}

	triggerKey := "id"
	if in.PRNumber != "" {
		triggerKey = "pr"
	}
	namespace := out.NamespacePrefix + "-" + dotsToDashes(triggerKey+"-"+in.IdentifierBase)

	// Deterministic per-run hash; the pre-truncation namespace feeds it. The
	// run attempt is deliberately not part of the hash so "re-run failed
	// jobs" computes the same namespace as the original install job.
	sum := sha256.Sum256([]byte(namespace + "-" + in.RunID))
	out.JobID = hex.EncodeToString(sum[:])[:6]

	// K8s caps namespace length at 63. Reserve room for the hash (-XXXXXX)
	// and the flow suffix (-upgp/-upgm) before appending them so they survive
	// truncation on long scenario names.
	reserved := 0
	if in.DeploymentTTL == "" {
		reserved += 7
	}
	switch in.SetupFlow {
	case "upgrade-patch", "upgrade-minor":
		reserved += 5
	}
	namespace = trimOneTrailingDash(truncateBytes(namespace, 63-reserved))
	if in.DeploymentTTL == "" {
		namespace += "-" + out.JobID
	}
	switch in.SetupFlow {
	case "upgrade-patch":
		namespace += "-upgp"
	case "upgrade-minor":
		namespace += "-upgm"
	case "modular-upgrade-minor":
		// The namespace must match the one used during installation, so no
		// flow suffix is appended.
	}
	out.Namespace = trimOneTrailingDash(truncateBytes(namespace, 63))

	out.AlphaChartDir, err = alphaChartDir(in.RepoRoot)
	if err != nil {
		return WorkflowVars{}, err
	}

	localIdentifier := in.IdentifierBase
	if localIdentifier == "" {
		localIdentifier = "no-id-use-ran-" + in.RandomID
	}
	identifier := dotsToDashes(platform + "-" + localIdentifier)
	switch in.SetupFlow {
	case "upgrade-patch":
		identifier += "-upgp"
	case "upgrade-minor":
		identifier += "-upgm"
	}
	out.Identifier = identifier

	out.IngressHost = identifier + "." + p.IngressHostnameBase
	if in.DeploymentTTL == "" {
		out.IngressHost = out.JobID + "-" + out.IngressHost
	}

	// For modular-upgrade-minor the upgrade step must reuse the FLOW value
	// from installation ("install") so index prefixes keep matching.
	out.Flow = in.Flow
	if in.SetupFlow == "modular-upgrade-minor" {
		out.Flow = "install"
	}
	out.KeycloakRealm = out.JobID + "-realm"

	return out, nil
}

func normalizePlatform(input string, infra InfraConfig) (string, error) {
	platform, _, _ := strings.Cut(input, ",")
	platform = strings.ToLower(platform)
	if platform == "" {
		return "", fmt.Errorf("platform input must not be empty")
	}
	if _, ok := infra.Platforms[platform]; !ok {
		return "", fmt.Errorf("platform %q not found in infra config", platform)
	}
	return platform, nil
}

func dotsToDashes(s string) string { return strings.ReplaceAll(s, ".", "-") }

func truncateBytes(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if len(s) > n {
		return s[:n]
	}
	return s
}

func trimOneTrailingDash(s string) string { return strings.TrimSuffix(s, "-") }

// alphaChartDir returns the basename of the highest-versioned
// charts/camunda-platform-8.* directory (version sort, like `sort -V`).
func alphaChartDir(repoRoot string) (string, error) {
	if repoRoot == "" {
		repoRoot = "."
	}
	matches, err := filepath.Glob(filepath.Join(repoRoot, "charts", "camunda-platform-8.*"))
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && fi.IsDir() {
			dirs = append(dirs, filepath.Base(m))
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no charts/camunda-platform-8.* directories under %s", repoRoot)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return versionLess(strings.TrimPrefix(dirs[i], "camunda-platform-"), strings.TrimPrefix(dirs[j], "camunda-platform-"))
	})
	return dirs[len(dirs)-1], nil
}

func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		ai, aerr := strconv.Atoi(as[i])
		bi, berr := strconv.Atoi(bs[i])
		if aerr == nil && berr == nil {
			if ai != bi {
				return ai < bi
			}
			continue
		}
		if as[i] != bs[i] {
			return as[i] < bs[i]
		}
	}
	return len(as) < len(bs)
}

// Emit writes the environment variables and step outputs in the same order as
// the original shell steps.
func (v WorkflowVars) Emit(env, out *ghactions.Writer) error {
	envPairs := [][2]string{
		{"INFRA_INGRESS_HOSTNAME_BASE", v.IngressHostnameBase},
		{"INFRA_NAMESPACE_PREFIX", v.NamespacePrefixInfra},
		{"INFRA_CLUSTER_NAME", v.ClusterName},
		{"CLUSTER_NAME", v.ClusterName},
		{"AWS_PROFILE", v.AWSProfile},
		{"POSTGRESQL_JDBC_URL", v.PostgresJDBCURL},
		{"INFRA_TELEPORT_PROXY", v.TeleportProxy},
		{"NAMESPACE_PREFIX", v.NamespacePrefix},
		{"PLATFORM", v.Platform},
		{"GITHUB_WORKFLOW_JOB_ID", v.JobID},
		{"GITHUB_WORKFLOW_RUN_ID", v.RunID},
		{"TEST_NAMESPACE", v.Namespace},
		{"TEST_CAMUNDA_HELM_DIR_ALPHA", v.AlphaChartDir},
		{"ORCHESTRATION_INDEX_PREFIX", v.JobID + "-orchestration"},
		{"OPTIMIZE_INDEX_PREFIX", v.JobID + "-optimize"},
		{"OPERATE_INDEX_PREFIX", v.JobID + "-operate"},
		{"TASKLIST_INDEX_PREFIX", v.JobID + "-tasklist"},
		{"FLOW", v.Flow},
		{"KEYCLOAK_REALM", v.KeycloakRealm},
	}
	for _, kv := range envPairs {
		if err := env.Set(kv[0], kv[1]); err != nil {
			return err
		}
	}
	for _, kv := range [][2]string{
		{"namespace", v.Namespace},
		{"identifier", v.Identifier},
		{"ingress-host", v.IngressHost},
	} {
		if err := out.Set(kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

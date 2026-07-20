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

// Package ciworkflow ports GitHub Actions CI variable-computation logic from
// bash to testable Go. It replaces the shell body of the composite action at
// .github/actions/test-type-vars.
package ciworkflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"scripts/camunda-core/pkg/ghactions"

	"gopkg.in/yaml.v3"
)

// UnitMatrixEntry is one entry of the unit-test matrix declared under `unit:`
// in a chart's ci-test-config.yaml. The JSON tags reproduce the compact output
// previously produced by `yq --indent=0 --output-format json`.
type UnitMatrixEntry struct {
	Name     string `json:"name" yaml:"name"`
	Packages string `json:"packages" yaml:"packages"`
}

// ciTestConfig is the minimal view of ci-test-config.yaml consumed here. The
// integration block was migrated to the composable scenario registry, so only
// the `unit:` block remains as the source of truth for these vars.
type ciTestConfig struct {
	Unit struct {
		Enabled bool              `yaml:"enabled"`
		Matrix  []UnitMatrixEntry `yaml:"matrix"`
	} `yaml:"unit"`
}

// TestTypeVarsInput carries the composite-action inputs plus the ambient env
// values the computation depends on.
type TestTypeVarsInput struct {
	// ChartDir is the chart directory name, e.g. "camunda-platform-8.10".
	ChartDir string
	// Flow is the setup flow, e.g. "install", "upgrade-patch", "upgrade-minor".
	Flow string
	// CamundaVersionPrevious is the previous Camunda minor, e.g. "8.9".
	CamundaVersionPrevious string
	// UpgradeStep is true when called from the upgrade phase of an upgrade flow.
	UpgradeStep bool
	// ValuesEnterprise enables the enterprise values file.
	ValuesEnterprise bool
	// ValuesDigest enables the digest values file when it exists.
	ValuesDigest bool
	// GitHubWorkspace is $GITHUB_WORKSPACE, used for AbsoluteTestChartDir.
	GitHubWorkspace string
	// KeycloakClientsSecret is the passthrough DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET.
	KeycloakClientsSecret string
	// RepoRoot is the directory the chart paths are resolved against. Empty
	// means "." (the CI working directory / repo root).
	RepoRoot string
}

// TestTypeVars is the computed result. It is returned as a struct (so callers
// can consume it directly and env-var passing can be removed later) and can be
// emitted to $GITHUB_ENV / $GITHUB_OUTPUT via Emit.
type TestTypeVars struct {
	ChartPath             string
	AbsoluteTestChartDir  string
	TestHelmExtraArgs     string
	TestHelmDigestValues  string
	KeycloakClientsSecret string
	UnitEnabled           bool
	UnitMatrix            []UnitMatrixEntry
}

// Compute reproduces the test-type-vars shell logic.
func Compute(in TestTypeVarsInput) (TestTypeVars, error) {
	repoRoot := in.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}

	// Chart path. For upgrade-minor, the install step deploys the previous
	// chart version, but the upgrade step must use the current (target) chart.
	var chartPath string
	if in.Flow == "upgrade-minor" && in.CamundaVersionPrevious != "" && !in.UpgradeStep {
		chartPath = "charts/camunda-platform-" + in.CamundaVersionPrevious
	} else {
		chartPath = "charts/" + in.ChartDir
	}

	// Relative path used by the chart-full-setup Taskfile
	// (test/integration/scenarios/chart-full-setup).
	chartPathRelative := "../../../../" + chartPath

	matrixFile := filepath.Join(repoRoot, "charts", in.ChartDir, "test", "ci-test-config.yaml")
	raw, err := os.ReadFile(matrixFile)
	if err != nil {
		return TestTypeVars{}, fmt.Errorf("read %s: %w", matrixFile, err)
	}
	var cfg ciTestConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return TestTypeVars{}, fmt.Errorf("parse %s: %w", matrixFile, err)
	}

	out := TestTypeVars{
		ChartPath:             chartPath,
		AbsoluteTestChartDir:  in.GitHubWorkspace + "/" + chartPath,
		KeycloakClientsSecret: in.KeycloakClientsSecret,
		UnitEnabled:           cfg.Unit.Enabled,
		UnitMatrix:            cfg.Unit.Matrix,
	}

	if in.ValuesEnterprise {
		out.TestHelmExtraArgs = "--values " + chartPathRelative + "/values-enterprise.yaml"
	}

	// The digest values file is used only when enabled and the file exists for
	// the resolved chart path.
	if in.ValuesDigest {
		if _, err := os.Stat(filepath.Join(repoRoot, chartPath, "values-digest.yaml")); err == nil {
			out.TestHelmDigestValues = chartPathRelative + "/values-digest.yaml"
		}
	}

	return out, nil
}

// UnitMatrixJSON returns the unit matrix as compact JSON, matching the previous
// `yq --indent=0 --output-format json` output consumed as a GHA job matrix.
func (v TestTypeVars) UnitMatrixJSON() (string, error) {
	b, err := json.Marshal(v.UnitMatrix)
	if err != nil {
		return "", fmt.Errorf("marshal unit matrix: %w", err)
	}
	return string(b), nil
}

// Emit writes the environment variables to env and the step outputs to out.
// TestHelmExtraArgs and TestHelmDigestValues are only written when non-empty,
// matching the original conditional shell behavior.
func (v TestTypeVars) Emit(env, out *ghactions.Writer) error {
	if err := env.Set("CHART_PATH", v.ChartPath); err != nil {
		return err
	}
	if err := env.Set("ABSOLUTE_TEST_CHART_DIR", v.AbsoluteTestChartDir); err != nil {
		return err
	}
	if v.TestHelmExtraArgs != "" {
		if err := env.Set("TEST_HELM_EXTRA_ARGS", v.TestHelmExtraArgs); err != nil {
			return err
		}
	}
	if v.TestHelmDigestValues != "" {
		if err := env.Set("TEST_HELM_DIGEST_VALUES", v.TestHelmDigestValues); err != nil {
			return err
		}
	}
	if err := env.Set("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", v.KeycloakClientsSecret); err != nil {
		return err
	}

	if err := out.Set("unit-enabled", fmt.Sprintf("%t", v.UnitEnabled)); err != nil {
		return err
	}
	matrixJSON, err := v.UnitMatrixJSON()
	if err != nil {
		return err
	}
	return out.Set("unit-matrix", matrixJSON)
}

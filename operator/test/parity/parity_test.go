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

// Package parity asserts that manifests rendered by the operator's embedded Helm
// SDK are byte-identical to those produced by the pinned Helm CLI.
//
// This is the contract the whole operator design rests on: customers who stay on
// plain Helm and customers who adopt the operator must get the same Kubernetes
// objects. If this suite fails, the operator is no longer a driver of the chart —
// it has become a second rendering path.
package parity

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"

	operatorhelm "operator/internal/helm"
)

const (
	chartRelPath = "../../../charts/camunda-platform-8.10"
	releaseName  = "camunda"
	namespace    = "camunda-hub"
)

type ParityTest struct {
	suite.Suite
	chartPath string
}

func TestParity(t *testing.T) {
	suite.Run(t, new(ParityTest))
}

func (s *ParityTest) SetupSuite() {
	abs, err := filepath.Abs(chartRelPath)
	s.Require().NoError(err)
	s.chartPath = abs

	if _, err := os.Stat(filepath.Join(abs, "Chart.yaml")); err != nil {
		s.T().Fatalf("chart not found at %s: %v", abs, err)
	}
	if _, err := os.Stat(filepath.Join(abs, "charts")); err != nil {
		s.T().Skip("chart dependencies missing; run: make helm.dependency-update chartPath=charts/camunda-platform-8.10")
	}
}

// TestHubModeRenderMatchesCLI renders the hub topology role both ways and requires
// the results to be identical.
func (s *ParityTest) TestHubModeRenderMatchesCLI() {
	valuesFile := filepath.Join("testdata", "hub-minimal.yaml")

	cliOut := s.renderWithCLI(valuesFile)
	sdkOut := s.renderWithSDK(valuesFile)

	s.Require().NotEmpty(cliOut, "helm CLI produced an empty manifest")

	if cliOut != sdkOut && os.Getenv("PARITY_DUMP_DIR") != "" {
		dir := os.Getenv("PARITY_DUMP_DIR")
		s.Require().NoError(os.WriteFile(filepath.Join(dir, "cli.yaml"), []byte(cliOut), 0o600))
		s.Require().NoError(os.WriteFile(filepath.Join(dir, "sdk.yaml"), []byte(sdkOut), 0o600))
	}

	s.Equal(cliOut, sdkOut, "SDK-rendered manifest diverged from the Helm CLI")
}

func (s *ParityTest) renderWithCLI(valuesFile string) string {
	bin := os.Getenv("HELM_BIN")
	if bin == "" {
		bin = "helm"
	}
	cmd := exec.Command(bin, "template", releaseName, s.chartPath,
		"--namespace", namespace, "-f", valuesFile)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(s.T(), err, "helm template failed: %s", string(out))
	return normalize(s.T(), string(out))
}

func (s *ParityTest) renderWithSDK(valuesFile string) string {
	raw, err := os.ReadFile(valuesFile)
	s.Require().NoError(err)

	var vals map[string]any
	s.Require().NoError(yaml.Unmarshal(raw, &vals))

	flags := genericclioptions.NewConfigFlags(false)
	flags.Namespace = ptr(namespace)

	driver, err := operatorhelm.NewDriver(flags, namespace)
	s.Require().NoError(err)

	manifest, err := driver.Template(context.Background(), operatorhelm.Options{
		ReleaseName: releaseName,
		Namespace:   namespace,
		ChartPath:   s.chartPath,
		Values:      vals,
		Timeout:     5 * time.Minute,
	})
	s.Require().NoError(err)
	return normalize(s.T(), manifest)
}

func ptr[T any](v T) *T { return &v }

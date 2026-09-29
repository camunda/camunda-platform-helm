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

package deployer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"scripts/deploy-camunda/pkg/types"
)

// A chart that publishes no contract template must not fail the render. Charts before 8.10 take
// part in a mixed chart-version topology as workload-only orchestration releases, and helm fails
// the whole render when --show-only names a template the chart does not have.
func TestRenderTopologyContractSkipsChartWithoutTemplate(t *testing.T) {
	chartPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("name: camunda-platform\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := RenderTopologyContract(context.Background(), types.Options{
		ChartPath: chartPath, ReleaseName: "orchd", Namespace: "test",
	})
	if err != nil {
		t.Fatalf("a chart without the contract template should not error: %v", err)
	}
	if manifest != nil {
		t.Fatalf("expected no manifest, got %q", manifest)
	}
}

// The positive control: when the template is present the render is actually attempted. The chart
// is deliberately not renderable, so reaching helm at all is what proves we did not short-circuit.
func TestRenderTopologyContractRendersWhenTemplatePresent(t *testing.T) {
	chartPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(chartPath, "templates", "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chartPath, topologyContractTemplate), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RenderTopologyContract(context.Background(), types.Options{
		ChartPath: chartPath, ReleaseName: "orcha", Namespace: "test",
	}); err == nil {
		t.Fatal("expected the render to be attempted and fail on an invalid chart, got no error")
	}
}

// Every chart version a topology scenario can pin must either publish the contract template or be
// skipped by name here, so adding a chart cannot silently reintroduce the render failure.
func TestTopologyContractTemplatePresenceByChartVersion(t *testing.T) {
	for version, published := range map[string]bool{
		"8.7": false, "8.8": false, "8.9": false, "8.10": true,
	} {
		chartPath := filepath.Join("..", "..", "..", "..", "charts", "camunda-platform-"+version)
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			t.Fatalf("chart %s not found at %s", version, chartPath)
		}
		_, err := os.Stat(filepath.Join(chartPath, topologyContractTemplate))
		if published && err != nil {
			t.Errorf("chart %s should publish %s: %v", version, topologyContractTemplate, err)
		}
		if !published && err == nil {
			t.Errorf("chart %s now publishes %s; give it a contract-backed expectation instead of skipping it", version, topologyContractTemplate)
		}
	}
}

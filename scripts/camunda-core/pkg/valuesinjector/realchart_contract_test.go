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

package valuesinjector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealChartContract(t *testing.T) {
	cases := []struct {
		version    string
		components []string
	}{
		{
			version: "8.7",
			components: []string{
				"console", "zeebe", "zeebeGateway", "operate", "tasklist",
				"optimize", "identity", "webModeler", "connectors",
			},
		},
		{
			version: "8.8",
			components: []string{
				"identity", "console", "webModeler", "connectors", "orchestration", "optimize",
			},
		},
		{
			version: "8.9",
			components: []string{
				"identity", "console", "webModeler", "connectors", "orchestration", "optimize",
			},
		},
		{
			version: "8.10",
			components: []string{
				"identity", "webModeler", "connectors", "orchestration", "optimize",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			dir := filepath.Join("..", "..", "..", "..", "charts", "camunda-platform-"+tc.version)
			valuesPath := filepath.Join(dir, "values.yaml")
			if _, err := os.Stat(valuesPath); err != nil {
				t.Skipf("chart values.yaml absent at %s", valuesPath)
			}

			valuesYAML, err := os.ReadFile(valuesPath)
			if err != nil {
				t.Fatalf("read values.yaml: %v", err)
			}

			result, err := mergeRealChartContract(tc.version, string(valuesYAML))
			if err != nil {
				t.Fatalf("version %s: injector targets a component missing from values.yaml — "+
					"update the injector's component list for this version to match the chart: %v",
					tc.version, err)
			}

			for _, component := range tc.components {
				sentinel := contractSentinel(component)
				if !strings.Contains(result, sentinel) {
					t.Errorf("version %s: injector did not bump component %q (sentinel %q missing)",
						tc.version, component, sentinel)
				}
			}

			if tc.version == "8.10" {
				if strings.Contains(result, contractSentinel("console")) {
					t.Errorf("version 8.10: console must not be injected (no standalone console in values.yaml)")
				}
			}
		})
	}
}

func contractSentinel(component string) string {
	return "CONTRACT-" + component
}

func contractTag(component string) *ComponentImage {
	return &ComponentImage{Image: ImageTag{Tag: contractSentinel(component)}}
}

func buildOverrides86() *ValuesYAML86 {
	return &ValuesYAML86{
		Console:      contractTag("console"),
		Zeebe:        contractTag("zeebe"),
		ZeebeGateway: contractTag("zeebeGateway"),
		Operate:      contractTag("operate"),
		Tasklist:     contractTag("tasklist"),
		Optimize:     contractTag("optimize"),
		Identity:     contractTag("identity"),
		WebModeler:   contractTag("webModeler"),
		Connectors:   contractTag("connectors"),
	}
}

func buildOverrides88() *ValuesYAML88 {
	return &ValuesYAML88{
		Identity:      contractTag("identity"),
		Console:       contractTag("console"),
		WebModeler:    contractTag("webModeler"),
		Connectors:    contractTag("connectors"),
		Orchestration: contractTag("orchestration"),
		Optimize:      contractTag("optimize"),
	}
}

func mergeRealChartContract(version, valuesYAML string) (string, error) {
	switch version {
	case "8.7":
		return MergeImageTags87(valuesYAML, buildOverrides86())
	case "8.8":
		return MergeImageTags88(valuesYAML, buildOverrides88())
	case "8.9":
		return MergeImageTags89(valuesYAML, buildOverrides88())
	case "8.10":
		return MergeImageTags810(valuesYAML, buildOverrides88())
	default:
		return "", fmt.Errorf("unsupported chart version for contract test: %s", version)
	}
}

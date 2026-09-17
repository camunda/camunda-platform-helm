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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
)

// writeArgvEchoHelm puts a fake `helm` on PATH that prints its own argv, one
// argument per line, so a render call can be inspected as the exact command
// line Helm would have received. Fakes at the process boundary: nothing in the
// deployer chain is stubbed.
func writeArgvEchoHelm(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "helm"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestRenderPreparedTopologyContract_OmitsIngressHostOption pins down that the
// topology contract render never turns a release's host into
// global.ingress.host.
//
// pkg/deployer/helm.go appendHelmValueArgs converts a non-empty
// types.Options.IngressHost into "--set global.ingress.host=<v>", and
// charts/camunda-platform-8.10/templates/common/constraints.tpl declares that
// key REMOVED via camundaPlatform.keyRemoved — setting it is a hard template
// failure, so the contract render dies before the topology can be validated.
// The host reaches the chart through SetPairs instead, as global.host.
func TestRenderPreparedTopologyContract_OmitsIngressHostOption(t *testing.T) {
	writeArgvEchoHelm(t)

	const host = "matrix-810-mns-hub.ci.distro.ultrawombat.com"
	prepared := &PreparedScenario{
		ScenarioCtx: &ScenarioContext{
			ScenarioName: "multinamespace",
			Namespace:    "matrix-810-mns-hub",
			Release:      "integration",
			IngressHost:  host,
		},
	}
	flags := &config.RuntimeFlags{
		Chart: config.ChartFlags{ChartPath: "charts/camunda-platform-8.10"},
		Deployment: config.DeploymentFlags{
			ExtraHelmSets: map[string]string{"global.host": host},
		},
	}

	out, err := RenderPreparedTopologyContract(context.Background(), prepared, flags)
	if err != nil {
		t.Fatalf("RenderPreparedTopologyContract() error = %v", err)
	}
	args := strings.Split(strings.TrimRight(string(out), "\n"), "\n")

	var sawGlobalHost bool
	for _, arg := range args {
		if arg == "global.host="+host {
			sawGlobalHost = true
		}
	}
	if !sawGlobalHost {
		t.Fatalf("helm args = %v, want --set global.host=%s (SetPairs is the supported channel for the host)", args, host)
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "global.ingress.host=") {
			t.Errorf("helm args contain %q; global.ingress.host is a removed key and fails the chart's constraints.tpl — ScenarioCtx.IngressHost must not become types.Options.IngressHost (full args: %v)", arg, args)
		}
	}
}

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
	"path/filepath"
	"runtime"
	"testing"
)

func chartFullSetupPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "charts", "camunda-platform-8.10", "test", "integration", "scenarios", "chart-full-setup")
}

func TestCompanionSchedulingFromInfra(t *testing.T) {
	scenarioPath := chartFullSetupPath(t)

	nodeSelector, tolerations := companionSchedulingFromInfra(scenarioPath, "distroci")
	if nodeSelector["workload"] != "distroci" {
		t.Fatalf("expected nodeSelector[workload]=distroci, got %#v", nodeSelector)
	}
	if len(tolerations) == 0 {
		t.Fatal("expected non-empty tolerations")
	}
	if tolerations[0]["value"] != "distroci" {
		t.Fatalf("expected first toleration value=distroci, got %#v", tolerations[0])
	}
}

func TestCompanionSchedulingFromInfraEmptyType(t *testing.T) {
	scenarioPath := chartFullSetupPath(t)

	nodeSelector, tolerations := companionSchedulingFromInfra(scenarioPath, "")
	if nodeSelector != nil || tolerations != nil {
		t.Fatalf("expected nil,nil for empty infraType, got %#v, %#v", nodeSelector, tolerations)
	}
}

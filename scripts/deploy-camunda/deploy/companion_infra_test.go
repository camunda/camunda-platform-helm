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

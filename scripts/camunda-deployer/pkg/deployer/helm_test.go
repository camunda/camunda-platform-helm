package deployer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestHelmError_Error(t *testing.T) {
	err := &HelmError{
		Reason:  "helm upgrade --install failed",
		Command: "helm upgrade --install integration /path/to/chart -n ns --wait",
		Cause:   fmt.Errorf("exit status 1"),
	}

	got := err.Error()
	want := "helm upgrade --install failed: exit status 1"
	if got != want {
		t.Errorf("HelmError.Error() = %q, want %q", got, want)
	}
}

func TestHelmError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("exit status 1")
	err := &HelmError{
		Reason:  "helm upgrade --install failed",
		Command: "helm upgrade --install integration /path/to/chart -n ns --wait",
		Cause:   cause,
	}

	if !errors.Is(err, cause) {
		t.Error("HelmError.Unwrap: expected errors.Is to find cause")
	}
}

func TestHelmError_ShortCommand(t *testing.T) {
	err := &HelmError{
		Reason: "helm upgrade --install failed",
		Command: "helm upgrade --install integration /Users/user/workspaces/camunda-platform-helm/charts/camunda-platform-8.9 " +
			"-n distribution-89-esarm-inst --create-namespace --kube-context gke_camunda-distribution_europe-west1-b_distro-ci " +
			"--wait --atomic --timeout 300s " +
			"-f /var/folders/5n/00q4pg1s0fv11255p5zzlbg40000gn/T/camunda-values-elasticsearch-arm-4206716148/base.yaml " +
			"-f /var/folders/5n/00q4pg1s0fv11255p5zzlbg40000gn/T/camunda-values-elasticsearch-arm-4206716148/keycloak.yaml " +
			"-f /var/folders/5n/00q4pg1s0fv11255p5zzlbg40000gn/T/camunda-values-elasticsearch-arm-4206716148/elasticsearch.yaml",
		Cause: fmt.Errorf("exit status 1"),
	}

	short := err.ShortCommand()

	// Chart path should be shortened
	if !strings.Contains(short, "camunda-platform-8.9") {
		t.Errorf("ShortCommand: expected shortened chart name in output, got: %s", short)
	}
	if strings.Contains(short, "/Users/user/workspaces") {
		t.Errorf("ShortCommand: expected long chart path to be shortened, got: %s", short)
	}

	// Values file paths should be shortened
	if strings.Contains(short, "/var/folders") {
		t.Errorf("ShortCommand: expected long values paths to be shortened, got: %s", short)
	}
	if !strings.Contains(short, "base.yaml") || !strings.Contains(short, "keycloak.yaml") || !strings.Contains(short, "elasticsearch.yaml") {
		t.Errorf("ShortCommand: expected base filenames to be preserved, got: %s", short)
	}

	// Non-path args should be preserved
	if !strings.Contains(short, "--kube-context") || !strings.Contains(short, "gke_camunda-distribution_europe-west1-b_distro-ci") {
		t.Errorf("ShortCommand: expected non-path args to be preserved, got: %s", short)
	}
}

func TestShortenPaths_NoAbsolutePaths(t *testing.T) {
	cmd := "helm upgrade --install integration camunda/camunda-platform -n ns --version 13.5.0 --wait"
	got := shortenPaths(cmd)
	if got != cmd {
		t.Errorf("shortenPaths: should not modify command without absolute paths: got %q, want %q", got, cmd)
	}
}

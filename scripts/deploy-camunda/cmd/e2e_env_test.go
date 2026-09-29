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

package cmd

import (
	"strings"
	"testing"
)

func TestMergeEnvOverridesReplacesExistingKey(t *testing.T) {
	content := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\nKEYCLOAK_URL=https://orcha.example.com\n"
	overrides := map[string]string{
		"KEYCLOAK_URL": "https://hub.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\nKEYCLOAK_URL=https://hub.example.com\n"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

func TestDecodeSecretValueRoundTrip(t *testing.T) {
	// "s3cr3t" base64 == "czNjcjN0", with surrounding whitespace kubectl may emit.
	got, err := decodeSecretValue("  czNjcjN0\n")
	if err != nil {
		t.Fatalf("decodeSecretValue() unexpected error: %v", err)
	}
	if got != "s3cr3t" {
		t.Fatalf("decodeSecretValue() = %q, want %q", got, "s3cr3t")
	}
}

func TestDecodeSecretValueEmptyStringSucceeds(t *testing.T) {
	got, err := decodeSecretValue("")
	if err != nil {
		t.Fatalf("decodeSecretValue() unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("decodeSecretValue() = %q, want empty string", got)
	}
}

func TestDecodeSecretValueRejectsInvalidBase64(t *testing.T) {
	if _, err := decodeSecretValue("not!base64!"); err == nil {
		t.Fatal("decodeSecretValue() expected error on invalid base64, got nil")
	}
}

func TestMergeEnvOverridesAppendsMissingKeysSorted(t *testing.T) {
	content := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\n"
	overrides := map[string]string{
		"OAUTH_URL":           "https://hub.example.com/token",
		"MANAGEMENT_BASE_URL": "https://hub.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "PLAYWRIGHT_BASE_URL=https://orcha.example.com\nMANAGEMENT_BASE_URL=https://hub.example.com\nOAUTH_URL=https://hub.example.com/token\n"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

func TestMergeEnvOverridesRoutesHubApplicationsToHubHost(t *testing.T) {
	content := strings.Join([]string{
		"PLAYWRIGHT_BASE_URL=https://orcha.example.com",
		"CONSOLE_BASE_URL=https://orcha.example.com",
		"IDENTITY_BASE_URL=https://orcha.example.com/identity/",
		"KEYCLOAK_BASE_URL=https://orcha.example.com/auth",
		"WEBMODELER_BASE_URL=https://orcha.example.com/modeler",
		"",
	}, "\n")
	overrides := map[string]string{
		"CONSOLE_BASE_URL":                 "https://hub.example.com",
		"CONSOLE_CONTEXT_PATH":             "https://hub.example.com/modeler",
		"IDENTITY_BASE_URL":                "https://hub.example.com/identity/",
		"KEYCLOAK_BASE_URL":                "https://hub.example.com/auth",
		"MANAGEMENT_IDENTITY_CONTEXT_PATH": "https://hub.example.com/identity",
		"MODELER_CONTEXT_PATH":             "https://hub.example.com/modeler",
		"WEBMODELER_BASE_URL":              "https://hub.example.com/modeler",
	}

	got := mergeEnvOverrides(content, overrides)
	if !strings.Contains(got, "PLAYWRIGHT_BASE_URL=https://orcha.example.com\n") {
		t.Fatalf("mergeEnvOverrides() changed orchestration base URL: %q", got)
	}
	for key, value := range overrides {
		if !strings.Contains(got, key+"="+value+"\n") {
			t.Errorf("mergeEnvOverrides() missing %s Hub URL: %q", key, got)
		}
	}
}

func TestMergeEnvOverridesPreservesNoTrailingNewline(t *testing.T) {
	content := "PLAYWRIGHT_BASE_URL=https://orcha.example.com"
	overrides := map[string]string{
		"PLAYWRIGHT_BASE_URL": "https://hub.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "PLAYWRIGHT_BASE_URL=https://hub.example.com"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

func TestE2EEnvMergeFailsOnMissingRenderScript(t *testing.T) {
	cmd := newE2EEnvMergeCommand()
	cmd.SetArgs([]string{
		"--orchestration-namespace", "matrix-810-mns-orcha",
		"--hub-namespace", "matrix-810-mns-hub",
		"--absolute-chart-path", "/workspace/charts/camunda-platform-8.10",
		"--render-script", "/nonexistent/render-e2e-env.sh",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --render-script points at a non-existent path")
	}
	if !strings.Contains(err.Error(), "render script failed") {
		t.Fatalf("expected error to mention render script failure, got: %v", err)
	}
}

func TestSelectIngressHostFiltersZeebeAndGrpcHosts(t *testing.T) {
	raw := "matrix-810-mns-hub.ci.distro.ultrawombat.com zeebe-matrix-810-mns-hub.ci.distro.ultrawombat.com grpc-matrix-810-mns-hub.ci.distro.ultrawombat.com"

	got, err := selectIngressHost(raw)
	if err != nil {
		t.Fatalf("selectIngressHost() unexpected error: %v", err)
	}
	want := "matrix-810-mns-hub.ci.distro.ultrawombat.com"

	if got != want {
		t.Fatalf("selectIngressHost() = %q, want %q", got, want)
	}
}

func TestSelectIngressHostPassesThroughSingleHost(t *testing.T) {
	raw := "matrix-810-mns-hub.ci.distro.ultrawombat.com"

	got, err := selectIngressHost(raw)
	if err != nil {
		t.Fatalf("selectIngressHost() unexpected error: %v", err)
	}
	want := "matrix-810-mns-hub.ci.distro.ultrawombat.com"

	if got != want {
		t.Fatalf("selectIngressHost() = %q, want %q", got, want)
	}
}

func TestSelectIngressHostEmptyInputReturnsEmpty(t *testing.T) {
	got, err := selectIngressHost("")
	if err != nil {
		t.Fatalf("selectIngressHost() unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("selectIngressHost() = %q, want empty string", got)
	}
}

// TestSelectIngressHostDedupesRepeatedHost pins down the shared-host
// multi-namespace topology case: a namespace's Ingress can list the same
// host across multiple rules, so the kubectl jsonpath query
// ({.items[*].spec.rules[*].host}) emits it N times. selectIngressHost must
// collapse the repeats into a single host rather than joining "host,host,host".
func TestSelectIngressHostDedupesRepeatedHost(t *testing.T) {
	raw := "matrix-810-mns-hub.ci.distro.ultrawombat.com matrix-810-mns-hub.ci.distro.ultrawombat.com matrix-810-mns-hub.ci.distro.ultrawombat.com"

	got, err := selectIngressHost(raw)
	if err != nil {
		t.Fatalf("selectIngressHost() unexpected error: %v", err)
	}
	want := "matrix-810-mns-hub.ci.distro.ultrawombat.com"

	if got != want {
		t.Fatalf("selectIngressHost() = %q, want %q (repeated host must collapse to one)", got, want)
	}
}

func TestSelectIngressHostRejectsMultipleDistinctHosts(t *testing.T) {
	raw := "b.example.com a.example.com b.example.com zeebe-a.example.com a.example.com"

	got, err := selectIngressHost(raw)
	if err == nil {
		t.Fatalf("selectIngressHost() = %q, want ambiguity error", got)
	}
	if !strings.Contains(err.Error(), "multiple distinct HTTP hosts") {
		t.Fatalf("selectIngressHost() error = %q, want ambiguity error", err)
	}
}

func TestMergeEnvOverridesIgnoresLinesWithoutEquals(t *testing.T) {
	content := "# a comment\n\nPLAYWRIGHT_BASE_URL=https://orcha.example.com\n"
	overrides := map[string]string{
		"PLAYWRIGHT_BASE_URL": "https://hub.example.com",
	}

	got := mergeEnvOverrides(content, overrides)
	want := "# a comment\n\nPLAYWRIGHT_BASE_URL=https://hub.example.com\n"

	if got != want {
		t.Fatalf("mergeEnvOverrides() = %q, want %q", got, want)
	}
}

func TestAssertOptimizeEnabledRejectsDisabledOptimize(t *testing.T) {
	merged := "CAMUNDA_OPTIMIZE_BASE_URL=https://hub.example.com/optimize-orcha\nIS_OPTIMIZE=false\n"

	err := assertOptimizeEnabled(merged, "ns-opta")
	if err == nil || !strings.Contains(err.Error(), "IS_OPTIMIZE=false") {
		t.Fatalf("expected a hard failure when Optimize specs would be skipped, got %v", err)
	}
}

func TestAssertOptimizeEnabledAcceptsOverriddenOptimize(t *testing.T) {
	merged := mergeEnvOverrides(
		"CAMUNDA_OPTIMIZE_BASE_URL=https://orcha.example.com/optimize\nIS_OPTIMIZE=false\n",
		map[string]string{
			"CAMUNDA_OPTIMIZE_BASE_URL": "https://hub.example.com/optimize-orcha",
			"IS_OPTIMIZE":               "true",
		},
	)

	if !strings.Contains(merged, "IS_OPTIMIZE=true") {
		t.Fatalf("merge did not enable Optimize: %q", merged)
	}
	if !strings.Contains(merged, "CAMUNDA_OPTIMIZE_BASE_URL=https://hub.example.com/optimize-orcha") {
		t.Fatalf("merge did not repoint Optimize at the Hub host: %q", merged)
	}
	if err := assertOptimizeEnabled(merged, "ns-opta"); err != nil {
		t.Fatalf("expected the overridden env to pass, got %v", err)
	}
}

// Both Optimize URL keys must be set and agree. CAMUNDA_OPTIMIZE_BASE_URL alone made the Optimize
// specs pass while Basic Navigation and the all-apps smoke flow failed on an nginx 404, because
// NavigationPage.goToOptimize navigates with OPTIMIZE_CONTEXT_PATH, whose default "/optimize" resolves
// against the orchestration host rather than the Hub host where the Optimize release runs.
func TestOptimizeEnvOverridesSetsBothUrlKeys(t *testing.T) {
	got := optimizeEnvOverrides("hub.example.com", "/optimize-orcha")

	want := "https://hub.example.com/optimize-orcha"
	for _, key := range []string{"CAMUNDA_OPTIMIZE_BASE_URL", "OPTIMIZE_CONTEXT_PATH"} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}
	if got["CAMUNDA_OPTIMIZE_BASE_URL"] != got["OPTIMIZE_CONTEXT_PATH"] {
		t.Errorf("the two Optimize URL keys disagree: %q vs %q", got["CAMUNDA_OPTIMIZE_BASE_URL"], got["OPTIMIZE_CONTEXT_PATH"])
	}
	if got["IS_OPTIMIZE"] != "true" {
		t.Errorf("IS_OPTIMIZE = %q, want \"true\"; every Optimize spec is guarded on it", got["IS_OPTIMIZE"])
	}
}

// The URLs must be absolute: page.goto honours an absolute URL over Playwright's baseURL, which is the
// orchestration host and therefore the wrong place to look for a separate Optimize release.
func TestOptimizeEnvOverridesUsesAbsoluteUrls(t *testing.T) {
	for _, v := range optimizeEnvOverrides("hub.example.com", "/optimize-orcha-ta") {
		if v == "true" {
			continue
		}
		if !strings.HasPrefix(v, "https://") {
			t.Errorf("override %q must be an absolute URL", v)
		}
	}
}

func TestValidateOptimizeFlagsRejectsNamespaceWithoutContextPath(t *testing.T) {
	err := validateOptimizeFlags("matrix-810-mns-opt-orcha", "")
	if err == nil {
		t.Fatal("expected an error when --optimize-namespace is set without --optimize-context-path")
	}
	if !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("expected error to say the flags pair, got: %v", err)
	}
}

func TestValidateOptimizeFlagsRejectsContextPathWithoutNamespace(t *testing.T) {
	err := validateOptimizeFlags("", "/optimize-orcha")
	if err == nil {
		t.Fatal("expected an error when --optimize-context-path is set without --optimize-namespace")
	}
	if !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("expected error to say the flags pair, got: %v", err)
	}
}

func TestValidateOptimizeFlagsRejectsContextPathWithoutLeadingSlash(t *testing.T) {
	err := validateOptimizeFlags("matrix-810-mns-opt-orcha", "optimize-orcha")
	if err == nil {
		t.Fatal("expected an error when --optimize-context-path has no leading slash")
	}
	if !strings.Contains(err.Error(), "must start with") {
		t.Fatalf("expected error to name the missing leading slash, got: %v", err)
	}
}

func TestValidateOptimizeFlagsAcceptsBothSetAndBothOmitted(t *testing.T) {
	if err := validateOptimizeFlags("matrix-810-mns-opt-orcha", "/optimize-orcha"); err != nil {
		t.Fatalf("expected a paired namespace and context path to validate, got: %v", err)
	}
	if err := validateOptimizeFlags("", ""); err != nil {
		t.Fatalf("expected both flags omitted to validate, got: %v", err)
	}
}

func TestE2EEnvMergeRejectsHalfConfiguredOptimizeBeforeRendering(t *testing.T) {
	cmd := newE2EEnvMergeCommand()
	cmd.SetArgs([]string{
		"--orchestration-namespace", "matrix-810-mns-orcha",
		"--hub-namespace", "matrix-810-mns-hub",
		"--absolute-chart-path", "/workspace/charts/camunda-platform-8.10",
		"--render-script", "/nonexistent/render-e2e-env.sh",
		"--optimize-namespace", "matrix-810-mns-opt-orcha",
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when only --optimize-namespace is supplied")
	}
	// The render script is deliberately bogus: the flag check must fail first,
	// proving it runs before any cluster or filesystem work.
	if !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("expected the flag pairing error ahead of the render step, got: %v", err)
	}
}

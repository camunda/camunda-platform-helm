package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"scripts/deploy-camunda/config"
	"strings"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"
)

// Wizard collects user input and produces a RootConfig.
type Wizard struct {
	ds         DataSource
	existing   *config.RootConfig
	accessible bool
}

// NewWizard creates a wizard. If existing is non-nil, fields are pre-populated (edit mode).
func NewWizard(ds DataSource, existing *config.RootConfig, accessible bool) *Wizard {
	return &Wizard{ds: ds, existing: existing, accessible: accessible}
}

// Run executes the interactive wizard and returns the populated config.
// It does NOT write to disk — the caller owns the write.
func (w *Wizard) Run() (rc *config.RootConfig, configPath string, err error) {
	// --- answers ---
	var (
		configChoice   = "local"
		customPath     string
		deploymentName = "default"
		platform       = "gke"
		cs             = chartSourceChoice{Mode: "local"}
		namespace      string
		release        string
		scenario       string
		flow           = "install"
		wantAdvanced   bool
		ingressMode    = "skip"
		ingressHost    string
		ingressSub     string
		ingressBase    string
		auth           = "keycloak"
		extSecrets     = true
		autoGenSecrets bool
	)

	// Auto-detect repo root for chart path default
	repoRoot, _ := w.ds.DetectRepoRoot()

	// Pre-populate defaults from existing config (edit mode)
	if w.existing != nil {
		if w.existing.Platform != "" {
			platform = w.existing.Platform
		}
		if w.existing.Flow != "" {
			flow = w.existing.Flow
		}
	}

	// Build a single form with conditional groups via WithHideFunc.
	// This avoids stdin buffering issues with sequential forms in accessible mode.
	form := huh.NewForm(
		// --- Core settings ---
		stepConfigLocation(&configChoice),
		stepCustomPath(&customPath).
			WithHideFunc(func() bool { return configChoice != "custom" }),
		stepDeploymentProfile(&deploymentName),
		stepPlatform(&platform, w.ds),
		stepChartSource(&cs),
		stepChartLocal(&cs, repoRoot).
			WithHideFunc(func() bool { return cs.Mode != "local" }),
		stepChartRemote(&cs).
			WithHideFunc(func() bool { return cs.Mode != "remote" }),
		stepDeploymentIdentity(&namespace, &release, &scenario, &flow, w.ds, repoRoot),

		// --- Advanced settings ---
		stepAdvancedPrompt(&wantAdvanced),
		stepIngress(&ingressMode, &ingressHost, &ingressSub, &ingressBase).
			WithHideFunc(func() bool { return !wantAdvanced }),
		stepIngressHostname(&ingressHost).
			WithHideFunc(func() bool { return !wantAdvanced || ingressMode != "hostname" }),
		stepIngressSubdomain(&ingressSub, &ingressBase).
			WithHideFunc(func() bool { return !wantAdvanced || ingressMode != "subdomain" }),
		stepAuth(&auth).
			WithHideFunc(func() bool { return !wantAdvanced }),
		stepSecrets(&extSecrets, &autoGenSecrets).
			WithHideFunc(func() bool { return !wantAdvanced }),
	).WithTheme(camundaTheme()).WithAccessible(w.accessible)

	if err := form.Run(); err != nil {
		return nil, "", fmt.Errorf("wizard cancelled: %w", err)
	}

	// --- resolve config path ---
	switch configChoice {
	case "local":
		configPath = ".camunda-deploy.yaml"
	case "global":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		configPath = filepath.Join(home, ".config", "camunda", "deploy.yaml")
	case "custom":
		configPath = customPath
	}

	// --- build config ---
	rc = w.existing
	if rc == nil {
		rc = &config.RootConfig{}
	}
	rc.FilePath = configPath

	// Set root-level defaults
	if rc.Platform == "" {
		rc.Platform = platform
	}

	// Build the deployment
	dep := config.DeploymentConfig{}
	dep.Platform = platform
	dep.Flow = flow
	dep.Namespace = namespace
	dep.Release = release
	dep.Scenario = scenario

	switch cs.Mode {
	case "local":
		dep.ChartPath = cs.ChartPath
	case "remote":
		dep.Chart = cs.Chart
		dep.Version = cs.Version
	}

	// Ingress
	switch ingressMode {
	case "hostname":
		dep.IngressHost = ingressHost
	case "subdomain":
		dep.IngressSubdomain = ingressSub
		dep.IngressBaseDomain = ingressBase
	}

	// Auth
	if auth != "keycloak" {
		dep.Auth = auth
	}

	// Secrets
	if !extSecrets {
		f := false
		dep.ExternalSecrets = &f
	}
	if autoGenSecrets {
		t := true
		dep.AutoGenerateSecrets = &t
	}

	// Add deployment to config
	if rc.Deployments == nil {
		rc.Deployments = make(map[string]config.DeploymentConfig)
	}
	rc.Deployments[deploymentName] = dep
	rc.Current = deploymentName

	return rc, configPath, nil
}

// buildSummary produces a human-readable summary of the wizard answers.
func buildSummary(name, platform string, cs *chartSourceChoice, ns, release, scenario, flow string,
	ingressMode, ingressHost, ingressSub, ingressBase, auth string, extSecrets bool) string {

	dep := map[string]any{
		"platform":  platform,
		"namespace": ns,
		"release":   release,
		"scenario":  scenario,
		"flow":      flow,
	}
	switch cs.Mode {
	case "local":
		dep["chartPath"] = cs.ChartPath
	case "remote":
		dep["chart"] = cs.Chart
		if cs.Version != "" {
			dep["version"] = cs.Version
		}
	}
	switch ingressMode {
	case "hostname":
		dep["ingressHost"] = ingressHost
	case "subdomain":
		dep["ingressSubdomain"] = ingressSub
		dep["ingressBaseDomain"] = ingressBase
	}
	if auth != "keycloak" {
		dep["auth"] = auth
	}

	preview := map[string]any{
		"current":     name,
		"deployments": map[string]any{name: dep},
	}

	out, err := yaml.Marshal(preview)
	if err != nil {
		return fmt.Sprintf("Error generating preview: %v", err)
	}

	return fmt.Sprintf("\nConfig preview:\n\n%s", strings.TrimSpace(string(out)))
}

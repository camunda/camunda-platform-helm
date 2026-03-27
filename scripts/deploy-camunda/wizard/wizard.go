package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"scripts/deploy-camunda/config"
	"strconv"
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
	// --- answers with sensible defaults ---
	var (
		configChoice   = "local"
		customPath     string
		deploymentName = "default"
		platform       = "gke"
		cs             = chartSourceChoice{Mode: "local"}
		namespace      = "camunda"
		release        = "camunda"
		scenario       = "default"
		flow           = "install"
		wantAdvanced   bool
		ingressMode    = "skip"
		ingressHost    string
		ingressSub     string
		ingressBase    string
		auth           = "keycloak"
		extSecrets     = true
		autoGenSecrets bool

		// Matrix settings
		wantMatrix            bool
		matrixVersions        string
		matrixScenarioFilt    string
		matrixFlowFilt        string
		matrixMaxParallelStr  = "4"
		matrixHelmTimeoutStr  = "20"
		matrixStopOnFail      = true
		matrixCleanup         = true
		matrixDryRun          bool
		matrixTestIT          bool
		matrixTestE2E         bool
		matrixNsPrefix        string
		matrixPlatform        string
		matrixLogLevel        string
	)

	// Auto-detect repo root for chart path default
	repoRoot, _ := w.ds.DetectRepoRoot()

	// Pre-fill chart path from repo root when available
	if repoRoot != "" {
		cs.ChartPath = filepath.Join(repoRoot, "charts", "camunda-platform")
	}

	// Auto-detect platform from current kube context
	if contexts, err := w.ds.KubeContexts(); err == nil {
		for _, ctx := range contexts {
			lower := strings.ToLower(ctx)
			if strings.Contains(lower, "gke") {
				platform = "gke"
				break
			} else if strings.Contains(lower, "eks") {
				platform = "eks"
				break
			} else if strings.Contains(lower, "rosa") || strings.Contains(lower, "openshift") {
				platform = "rosa"
				break
			}
		}
	}

	// Pre-populate from existing config in edit mode
	if w.existing != nil {
		if w.existing.Platform != "" {
			platform = w.existing.Platform
		}
		if w.existing.Flow != "" {
			flow = w.existing.Flow
		}
		// Pre-populate from current deployment
		if w.existing.Current != "" {
			deploymentName = w.existing.Current
			if dep, ok := w.existing.Deployments[w.existing.Current]; ok {
				if dep.Namespace != "" {
					namespace = dep.Namespace
				}
				if dep.Release != "" {
					release = dep.Release
				}
				if dep.Scenario != "" {
					scenario = dep.Scenario
				}
				if dep.Platform != "" {
					platform = dep.Platform
				}
				if dep.Flow != "" {
					flow = dep.Flow
				}
				if dep.ChartPath != "" {
					cs.Mode = "local"
					cs.ChartPath = dep.ChartPath
				}
				if dep.Chart != "" {
					cs.Mode = "remote"
					cs.Chart = dep.Chart
					cs.Version = dep.Version
				}
				if dep.Auth != "" {
					auth = dep.Auth
				}
				if dep.IngressHost != "" {
					ingressMode = "hostname"
					ingressHost = dep.IngressHost
					wantAdvanced = true
				}
				if dep.IngressSubdomain != "" {
					ingressMode = "subdomain"
					ingressSub = dep.IngressSubdomain
					ingressBase = dep.IngressBaseDomain
					wantAdvanced = true
				}
				if dep.ExternalSecrets != nil {
					extSecrets = *dep.ExternalSecrets
					wantAdvanced = true
				}
				if dep.AutoGenerateSecrets != nil {
					autoGenSecrets = *dep.AutoGenerateSecrets
					wantAdvanced = true
				}
			}
		}
	}

	// Pre-populate matrix from existing config in edit mode
	if w.existing != nil {
		m := w.existing.Matrix
		if len(m.Versions) > 0 || m.ScenarioFilter != "" || m.FlowFilter != "" ||
			m.MaxParallel != nil || m.StopOnFailure != nil {
			wantMatrix = true
		}
		if len(m.Versions) > 0 {
			matrixVersions = strings.Join(m.Versions, ", ")
		}
		if m.ScenarioFilter != "" {
			matrixScenarioFilt = m.ScenarioFilter
		}
		if m.FlowFilter != "" {
			matrixFlowFilt = m.FlowFilter
		}
		if m.MaxParallel != nil {
			matrixMaxParallelStr = strconv.Itoa(*m.MaxParallel)
		}
		if m.HelmTimeout != nil {
			matrixHelmTimeoutStr = strconv.Itoa(*m.HelmTimeout)
		}
		if m.StopOnFailure != nil {
			matrixStopOnFail = *m.StopOnFailure
		}
		if m.Cleanup != nil {
			matrixCleanup = *m.Cleanup
		}
		if m.DryRun != nil {
			matrixDryRun = *m.DryRun
		}
		if m.TestIT != nil {
			matrixTestIT = *m.TestIT
		}
		if m.TestE2E != nil {
			matrixTestE2E = *m.TestE2E
		}
		if m.NamespacePrefix != "" {
			matrixNsPrefix = m.NamespacePrefix
		}
		if m.Platform != "" {
			matrixPlatform = m.Platform
		}
		if m.LogLevel != "" {
			matrixLogLevel = m.LogLevel
		}
	}

	// Discover scenarios from repo
	var discoveredScenarios []string
	if repoRoot != "" {
		scenarioPath := filepath.Join(repoRoot, "test", "integration", "scenarios")
		if s, err := w.ds.ListScenarios(scenarioPath); err == nil && len(s) > 0 {
			discoveredScenarios = s
		}
	}

	// --- Build the form ---
	form := huh.NewForm(
		stepWelcome(),
		stepDeploymentProfile(&deploymentName),
		stepPlatform(&platform, w.ds),
		stepChartSource(&cs),
		stepChartLocal(&cs, repoRoot).
			WithHideFunc(func() bool { return cs.Mode != "local" }),
		stepChartRemote(&cs).
			WithHideFunc(func() bool { return cs.Mode != "remote" }),
		func() *huh.Group {
			if len(discoveredScenarios) > 0 {
				return stepScenarioSelect(&scenario, discoveredScenarios)
			}
			return stepScenarioInput(&scenario)
		}(),
		stepDeploymentIdentity(&namespace, &release, &flow),
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
		stepMatrixPrompt(&wantMatrix),
		stepMatrixFiltering(&matrixVersions, &matrixScenarioFilt, &matrixFlowFilt).
			WithHideFunc(func() bool { return !wantMatrix }),
		stepMatrixExecution(&matrixMaxParallelStr, &matrixHelmTimeoutStr, &matrixStopOnFail, &matrixCleanup, &matrixDryRun).
			WithHideFunc(func() bool { return !wantMatrix }),
		stepMatrixTests(&matrixTestIT, &matrixTestE2E).
			WithHideFunc(func() bool { return !wantMatrix }),
		stepMatrixInfra(&matrixNsPrefix, &matrixPlatform, &matrixLogLevel).
			WithHideFunc(func() bool { return !wantMatrix }),
		stepConfigLocation(&configChoice),
		stepCustomPath(&customPath).
			WithHideFunc(func() bool { return configChoice != "custom" }),
		stepSummary(func() string {
			return buildSummary(deploymentName, platform, &cs, namespace, release, scenario, flow,
				ingressMode, ingressHost, ingressSub, ingressBase, auth, extSecrets,
				wantMatrix, matrixVersions, matrixMaxParallelStr, matrixHelmTimeoutStr,
				matrixStopOnFail, matrixCleanup)
		}),
	).
		WithTheme(camundaTheme()).
		WithAccessible(w.accessible).
		WithWidth(80).
		WithShowHelp(true)

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

	// Matrix configuration
	if wantMatrix {
		mc := config.MatrixConfig{}

		// Filtering
		if matrixVersions != "" {
			for _, v := range strings.Split(matrixVersions, ",") {
				if t := strings.TrimSpace(v); t != "" {
					mc.Versions = append(mc.Versions, t)
				}
			}
		}
		if matrixScenarioFilt != "" {
			mc.ScenarioFilter = matrixScenarioFilt
		}
		if matrixFlowFilt != "" {
			mc.FlowFilter = matrixFlowFilt
		}

		// Execution
		if n, err := strconv.Atoi(matrixMaxParallelStr); err == nil {
			mc.MaxParallel = &n
		}
		if n, err := strconv.Atoi(matrixHelmTimeoutStr); err == nil {
			mc.HelmTimeout = &n
		}
		mc.StopOnFailure = &matrixStopOnFail
		mc.Cleanup = &matrixCleanup
		if matrixDryRun {
			mc.DryRun = &matrixDryRun
		}

		// Tests
		if matrixTestIT {
			mc.TestIT = &matrixTestIT
		}
		if matrixTestE2E {
			mc.TestE2E = &matrixTestE2E
		}

		// Infra overrides
		if matrixNsPrefix != "" {
			mc.NamespacePrefix = matrixNsPrefix
		}
		if matrixPlatform != "" {
			mc.Platform = matrixPlatform
		}
		if matrixLogLevel != "" {
			mc.LogLevel = matrixLogLevel
		}

		rc.Matrix = mc
	}

	return rc, configPath, nil
}

// buildSummary produces a human-readable YAML preview of the wizard answers.
func buildSummary(name, platform string, cs *chartSourceChoice, ns, release, scenario, flow string,
	ingressMode, ingressHost, ingressSub, ingressBase, auth string, extSecrets bool,
	wantMatrix bool, matrixVersions string, matrixMaxParallelStr, matrixHelmTimeoutStr string,
	matrixStopOnFail, matrixCleanup bool) string {

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

	if wantMatrix {
		matrix := map[string]any{}
		if matrixVersions != "" {
			var versions []string
			for _, v := range strings.Split(matrixVersions, ",") {
				if t := strings.TrimSpace(v); t != "" {
					versions = append(versions, t)
				}
			}
			if len(versions) > 0 {
				matrix["versions"] = versions
			}
		}
		if n, err := strconv.Atoi(matrixMaxParallelStr); err == nil {
			matrix["maxParallel"] = n
		}
		if n, err := strconv.Atoi(matrixHelmTimeoutStr); err == nil {
			matrix["helmTimeout"] = n
		}
		matrix["stopOnFailure"] = matrixStopOnFail
		matrix["cleanup"] = matrixCleanup
		preview["matrix"] = matrix
	}

	out, err := yaml.Marshal(preview)
	if err != nil {
		return fmt.Sprintf("Error generating preview: %v", err)
	}

	return strings.TrimSpace(string(out))
}

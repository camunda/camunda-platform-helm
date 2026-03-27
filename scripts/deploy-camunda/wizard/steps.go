package wizard

import (
	"fmt"
	"path/filepath"
	"scripts/deploy-camunda/config"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
)

// k8sNameValidator validates Kubernetes-compatible names (RFC 1123 DNS labels).
func k8sNameValidator(fieldName string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("%s cannot be empty", fieldName)
		}
		if len(s) > 63 {
			return fmt.Errorf("%s must be 63 characters or fewer", fieldName)
		}
		for i, c := range s {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (c == '-' && i > 0 && i < len(s)-1)) {
				if c == '-' {
					return fmt.Errorf("%s cannot start or end with a hyphen", fieldName)
				}
				return fmt.Errorf("%s must contain only lowercase letters, numbers, and hyphens", fieldName)
			}
		}
		return nil
	}
}

// stepWelcome shows the wizard introduction.
func stepWelcome() *huh.Group {
	return huh.NewGroup(
		huh.NewNote().
			Title("Camunda Platform — Deployment Config Wizard").
			Description("This wizard will guide you through creating a deployment\nconfiguration for Camunda Platform.\n\nYou'll configure:\n  • Platform and chart source\n  • Namespace, release, and scenario\n  • Optionally: ingress, auth, and secrets\n").
			Next(true).
			NextLabel("Let's go →"),
	)
}

// stepDeploymentProfile asks for the deployment profile name.
func stepDeploymentProfile(deploymentName *string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Name this deployment").
			Description("A short label for this configuration (e.g., dev, staging, prod).\nYou can have multiple deployment profiles.").
			Placeholder("default").
			Value(deploymentName).
			Validate(k8sNameValidator("deployment name")),
	).Title("Getting Started  (1/5)")
}

// stepPlatform asks for the deployment platform.
func stepPlatform(platform *string, ds DataSource) *huh.Group {
	options := make([]huh.Option[string], 0, len(config.DeployPlatforms))
	for _, p := range config.DeployPlatforms {
		label := p
		switch p {
		case "gke":
			label = "gke — Google Kubernetes Engine"
		case "eks":
			label = "eks — Amazon EKS"
		case "rosa":
			label = "rosa — Red Hat OpenShift on AWS"
		}
		options = append(options, huh.NewOption(label, p))
	}

	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Target platform").
			Options(options...).
			Value(platform),
	).Title("Platform  (2/5)")
}

// chartSourceChoice tracks which chart source mode the user picked.
type chartSourceChoice struct {
	Mode      string // "local" or "remote"
	ChartPath string
	Chart     string
	Version   string
}

// stepChartSource asks how the user wants to specify the chart.
func stepChartSource(cs *chartSourceChoice) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("How should we find the Helm chart?").
			Description("Local path if you have a repo checkout; remote for OCI/Helm registry.").
			Options(
				huh.NewOption("Local chart path (repo checkout)", "local"),
				huh.NewOption("Remote chart (OCI registry / Helm repo)", "remote"),
			).
			Value(&cs.Mode),
	).Title("Chart Source  (3/5)")
}

// stepChartLocal asks for the local chart path.
func stepChartLocal(cs *chartSourceChoice, repoRoot string) *huh.Group {
	placeholder := "path/to/charts/camunda-platform"
	if repoRoot != "" {
		placeholder = filepath.Join(repoRoot, "charts", "camunda-platform")
	}

	return huh.NewGroup(
		huh.NewInput().
			Title("Chart path").
			Description("Path to the local Helm chart directory.").
			Placeholder(placeholder).
			Value(&cs.ChartPath).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("chart path cannot be empty")
				}
				return nil
			}),
	)
}

// stepChartRemote asks for the remote chart name and version.
func stepChartRemote(cs *chartSourceChoice) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Chart name").
			Description("OCI or Helm repo chart reference.").
			Placeholder("oci://ghcr.io/camunda/helm/camunda-platform").
			Value(&cs.Chart).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("chart name cannot be empty")
				}
				return nil
			}),
		huh.NewInput().
			Title("Chart version").
			Description("Leave empty for latest.").
			Placeholder("11.0.0").
			Value(&cs.Version),
	)
}

// stepDeploymentIdentity asks for namespace, release, and flow.
func stepDeploymentIdentity(ns, release *string, flow *string) *huh.Group {
	flowOptions := []huh.Option[string]{
		huh.NewOption("Install — fresh deployment", "install"),
		huh.NewOption("Upgrade — upgrade existing deployment", "upgrade"),
	}

	return huh.NewGroup(
		huh.NewInput().
			Title("Kubernetes namespace").
			Value(ns).
			Validate(k8sNameValidator("namespace")),
		huh.NewInput().
			Title("Helm release name").
			Value(release).
			Validate(k8sNameValidator("release name")),
		huh.NewSelect[string]().
			Title("What do you want to do?").
			Options(flowOptions...).
			Value(flow),
	).Title("Deployment Identity  (4/5)")
}

// stepScenarioSelect shows a select when scenarios are discoverable.
func stepScenarioSelect(scenario *string, scenarios []string) *huh.Group {
	opts := make([]huh.Option[string], len(scenarios))
	for i, s := range scenarios {
		opts[i] = huh.NewOption(s, s)
	}
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Deployment scenario").
			Description("Select a scenario from the repository.").
			Options(opts...).
			Filtering(true).
			Value(scenario),
	)
}

// stepScenarioInput shows a text input when scenarios can't be discovered.
func stepScenarioInput(scenario *string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Deployment scenario").
			Description("Scenario name (e.g., default, keycloak-mt).").
			Value(scenario).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("scenario cannot be empty")
				}
				return nil
			}),
	)
}

// stepAdvancedPrompt asks if the user wants to configure advanced settings.
func stepAdvancedPrompt(wantAdvanced *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Configure advanced settings?").
			Description("Ingress, authentication, and secrets.").
			Affirmative("Yes").
			Negative("No, use defaults").
			Value(wantAdvanced),
	).Title("Advanced Settings  (5/5)")
}

// stepIngress asks for ingress configuration.
func stepIngress(ingressMode *string, ingressHost, ingressSub, ingressBase *string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Ingress configuration").
			Options(
				huh.NewOption("Skip — no ingress", "skip"),
				huh.NewOption("Full hostname (e.g., camunda.example.com)", "hostname"),
				huh.NewOption("Subdomain + base domain", "subdomain"),
			).
			Value(ingressMode),
	).Title("Ingress")
}

// stepIngressHostname asks for the full ingress hostname.
func stepIngressHostname(hostname *string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Ingress hostname").
			Placeholder("camunda.example.com").
			Value(hostname).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("hostname cannot be empty")
				}
				return nil
			}),
	)
}

// stepIngressSubdomain asks for subdomain + base domain.
func stepIngressSubdomain(subdomain, baseDomain *string) *huh.Group {
	domainOptions := make([]huh.Option[string], 0, len(config.ValidIngressBaseDomains))
	for _, d := range config.ValidIngressBaseDomains {
		domainOptions = append(domainOptions, huh.NewOption(d, d))
	}

	return huh.NewGroup(
		huh.NewInput().
			Title("Ingress subdomain").
			Placeholder("my-deployment").
			Value(subdomain).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("subdomain cannot be empty")
				}
				return nil
			}),
		huh.NewSelect[string]().
			Title("Ingress base domain").
			Options(domainOptions...).
			Value(baseDomain),
	)
}

// stepAuth asks for auth configuration.
func stepAuth(auth *string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Authentication").
			Options(
				huh.NewOption("Bundled Keycloak (default)", "keycloak"),
				huh.NewOption("External Keycloak", "keycloak-external"),
				huh.NewOption("Generic OIDC provider", "oidc"),
				huh.NewOption("Basic authentication", "basic"),
				huh.NewOption("Hybrid (combined)", "hybrid"),
			).
			Value(auth),
	).Title("Authentication")
}

// stepSecrets asks for secrets configuration.
func stepSecrets(externalSecrets, autoGenerate *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Enable external secrets?").
			Description("Use external secrets operator for secret management.").
			Affirmative("Yes").
			Negative("No").
			Value(externalSecrets),
		huh.NewConfirm().
			Title("Auto-generate secrets?").
			Description("Generate random secrets for testing (not for production).").
			Affirmative("Yes").
			Negative("No").
			Value(autoGenerate),
	).Title("Secrets")
}

// --- Matrix steps ---

// stepMatrixPrompt asks if the user wants to configure the matrix runner.
func stepMatrixPrompt(wantMatrix *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Configure matrix runner?").
			Description("The matrix runner executes deployments across multiple versions\nand scenarios in parallel. Skip if you only deploy a single config.").
			Affirmative("Yes").
			Negative("No, skip").
			Value(wantMatrix),
	).Title("Matrix Runner")
}

// stepMatrixFiltering configures which versions and scenarios the matrix targets.
func stepMatrixFiltering(versions, scenarioFilter, flowFilter *string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Chart versions").
			Description("Comma-separated list of versions to test (e.g., 8.6, 8.7, 8.8).").
			Placeholder("8.6, 8.7").
			Value(versions),
		huh.NewInput().
			Title("Scenario filter").
			Description("Regex to match scenario names. Leave empty for all scenarios.").
			Placeholder("").
			Value(scenarioFilter),
		huh.NewInput().
			Title("Flow filter").
			Description("Regex to match flow types (install, upgrade). Leave empty for all.").
			Placeholder("").
			Value(flowFilter),
	).Title("Matrix — Filtering")
}

// stepMatrixExecution configures parallelism, timeouts, and behavior.
func stepMatrixExecution(maxParallel, helmTimeout *string, stopOnFailure, cleanup, dryRun *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Max parallel deployments").
			Description("How many deployments to run concurrently.").
			Value(maxParallel).
			Validate(intStringValidator("max parallel", 1, 32)),
		huh.NewInput().
			Title("Helm timeout (minutes)").
			Description("Timeout for each Helm install/upgrade operation.").
			Value(helmTimeout).
			Validate(intStringValidator("helm timeout", 1, 120)),
		huh.NewConfirm().
			Title("Stop on first failure?").
			Description("Abort remaining deployments when one fails.").
			Affirmative("Yes").
			Negative("No, continue").
			Value(stopOnFailure),
		huh.NewConfirm().
			Title("Clean up after run?").
			Description("Delete namespaces after the matrix run completes.").
			Affirmative("Yes").
			Negative("No, keep").
			Value(cleanup),
		huh.NewConfirm().
			Title("Dry run?").
			Description("Simulate the matrix run without deploying.").
			Affirmative("Yes").
			Negative("No").
			Value(dryRun),
	).Title("Matrix — Execution")
}

// stepMatrixTests configures which tests to run after matrix deployments.
func stepMatrixTests(testIT, testE2E *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Run integration tests?").
			Description("Execute integration tests after each deployment.").
			Affirmative("Yes").
			Negative("No").
			Value(testIT),
		huh.NewConfirm().
			Title("Run E2E tests?").
			Description("Execute end-to-end tests after each deployment.").
			Affirmative("Yes").
			Negative("No").
			Value(testE2E),
	).Title("Matrix — Tests")
}

// stepMatrixInfra configures optional infrastructure overrides for the matrix runner.
func stepMatrixInfra(nsPrefix, platform, logLevel *string) *huh.Group {
	platformOptions := []huh.Option[string]{
		huh.NewOption("(inherit from deployment)", ""),
	}
	for _, p := range config.DeployPlatforms {
		label := p
		switch p {
		case "gke":
			label = "gke — Google Kubernetes Engine"
		case "eks":
			label = "eks — Amazon EKS"
		case "rosa":
			label = "rosa — Red Hat OpenShift on AWS"
		}
		platformOptions = append(platformOptions, huh.NewOption(label, p))
	}

	logOptions := []huh.Option[string]{
		huh.NewOption("(inherit from deployment)", ""),
		huh.NewOption("debug", "debug"),
		huh.NewOption("info", "info"),
		huh.NewOption("warn", "warn"),
		huh.NewOption("error", "error"),
	}

	return huh.NewGroup(
		huh.NewInput().
			Title("Namespace prefix").
			Description("Prefix for matrix-created namespaces (e.g., 'distribution').").
			Placeholder("").
			Value(nsPrefix),
		huh.NewSelect[string]().
			Title("Platform override").
			Description("Override the platform for all matrix deployments.").
			Options(platformOptions...).
			Value(platform),
		huh.NewSelect[string]().
			Title("Log level").
			Description("Override the log level for matrix runs.").
			Options(logOptions...).
			Value(logLevel),
	).Title("Matrix — Infrastructure")
}

// intStringValidator validates that a string represents an integer within [min, max].
func intStringValidator(fieldName string, min, max int) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("%s cannot be empty", fieldName)
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("%s must be a number", fieldName)
		}
		if n < min || n > max {
			return fmt.Errorf("%s must be between %d and %d", fieldName, min, max)
		}
		return nil
	}
}

// stepConfigLocation asks where to save the config file.
func stepConfigLocation(configPath *string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Where should your config live?").
			Description("Project-local configs are great for per-repo settings;\nglobal configs apply everywhere.").
			Options(
				huh.NewOption(".camunda-deploy.yaml (this project)", "local"),
				huh.NewOption("~/.config/camunda/deploy.yaml (global)", "global"),
				huh.NewOption("Custom path", "custom"),
			).
			Value(configPath),
	).Title("Save Location")
}

// stepCustomPath asks for a custom config file path.
func stepCustomPath(customPath *string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Config file path").
			Placeholder("/path/to/deploy.yaml").
			Value(customPath).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("path cannot be empty")
				}
				return nil
			}),
	)
}

// stepSummary shows a preview of the config before writing.
func stepSummary(summaryFn func() string) *huh.Group {
	return huh.NewGroup(
		huh.NewNote().
			Title("Review Your Configuration").
			DescriptionFunc(summaryFn, nil).
			Next(true).
			NextLabel("Write config →"),
	).Title("Confirm")
}

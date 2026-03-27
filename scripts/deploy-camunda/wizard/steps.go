package wizard

import (
	"fmt"
	"path/filepath"
	"scripts/deploy-camunda/config"
	"strings"

	"github.com/charmbracelet/huh"
)

// stepConfigLocation asks where to save the config file.
func stepConfigLocation(configPath *string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Where should your config live?").
			Description("Project-local configs are great for per-repo settings; global configs apply everywhere.").
			Options(
				huh.NewOption(".camunda-deploy.yaml (this project)", "local"),
				huh.NewOption("~/.config/camunda/deploy.yaml (global)", "global"),
				huh.NewOption("Custom path", "custom"),
			).
			Value(configPath),
	)
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

// stepDeploymentProfile asks for the deployment profile name.
func stepDeploymentProfile(deploymentName *string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Deployment profile name").
			Description("A name for this deployment configuration (e.g., dev, staging, prod).").
			Placeholder("default").
			Value(deploymentName).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("name cannot be empty")
				}
				for _, c := range s {
					if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
						return fmt.Errorf("use lowercase letters, numbers, and hyphens only")
					}
				}
				return nil
			}),
	)
}

// stepPlatform asks for the deployment platform.
func stepPlatform(platform *string, ds DataSource) *huh.Group {
	options := make([]huh.Option[string], 0, len(config.DeployPlatforms))
	for _, p := range config.DeployPlatforms {
		label := p
		switch p {
		case "gke":
			label = "gke (Google Kubernetes Engine)"
		case "eks":
			label = "eks (Amazon EKS)"
		case "rosa":
			label = "rosa (Red Hat OpenShift on AWS)"
		}
		options = append(options, huh.NewOption(label, p))
	}

	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Target platform").
			Options(options...).
			Value(platform),
	)
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
			Title("Chart source").
			Description("How do you want to specify the Helm chart?").
			Options(
				huh.NewOption("Local chart path (from repo checkout)", "local"),
				huh.NewOption("Remote chart (OCI registry / Helm repo)", "remote"),
			).
			Value(&cs.Mode),
	)
}

// stepChartLocal asks for the local chart path.
func stepChartLocal(cs *chartSourceChoice, repoRoot string) *huh.Group {
	defaultPath := ""
	if repoRoot != "" {
		defaultPath = filepath.Join(repoRoot, "charts", "camunda-platform")
	}

	return huh.NewGroup(
		huh.NewInput().
			Title("Chart path").
			Description("Path to the local Helm chart directory.").
			Placeholder(defaultPath).
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

// stepDeploymentIdentity asks for namespace, release, scenario, and flow.
func stepDeploymentIdentity(ns, release, scenario *string, flow *string, ds DataSource, repoRoot string) *huh.Group {
	flowOptions := []huh.Option[string]{
		huh.NewOption("install (fresh deployment)", "install"),
		huh.NewOption("upgrade (upgrade existing)", "upgrade"),
	}

	return huh.NewGroup(
		huh.NewInput().
			Title("Kubernetes namespace").
			Placeholder("camunda").
			Value(ns).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("namespace cannot be empty")
				}
				return nil
			}),
		huh.NewInput().
			Title("Helm release name").
			Placeholder("camunda").
			Value(release).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("release name cannot be empty")
				}
				return nil
			}),
		huh.NewInput().
			Title("Scenario").
			Description("Deployment scenario name (e.g., default, keycloak-mt).").
			Placeholder("default").
			Value(scenario).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("scenario cannot be empty")
				}
				return nil
			}),
		huh.NewSelect[string]().
			Title("Flow").
			Options(flowOptions...).
			Value(flow),
	)
}

// stepAdvancedPrompt asks if the user wants to configure advanced settings.
func stepAdvancedPrompt(wantAdvanced *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Configure advanced settings?").
			Description("Ingress, auth, secrets, and testing options.").
			Affirmative("Yes").
			Negative("No").
			Value(wantAdvanced),
	)
}

// stepIngress asks for ingress configuration.
func stepIngress(ingressMode *string, hostname, subdomain, baseDomain *string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Ingress configuration").
			Options(
				huh.NewOption("Skip (no ingress)", "skip"),
				huh.NewOption("Full hostname", "hostname"),
				huh.NewOption("Subdomain + base domain", "subdomain"),
			).
			Value(ingressMode),
	)
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
				huh.NewOption("keycloak (default, bundled Keycloak)", "keycloak"),
				huh.NewOption("keycloak-external (external Keycloak)", "keycloak-external"),
				huh.NewOption("oidc (generic OIDC provider)", "oidc"),
				huh.NewOption("basic (basic auth)", "basic"),
				huh.NewOption("hybrid (combined)", "hybrid"),
			).
			Value(auth),
	)
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
	)
}


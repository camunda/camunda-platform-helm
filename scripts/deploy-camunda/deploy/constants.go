package deploy

// Constants for deployment configuration.
// These values are centralized here to make them easy to update and maintain.

const (
	// KeycloakVersionSafe is the underscore-separated version string used in environment variable names.
	// This corresponds to Keycloak version 24.9.0.
	KeycloakVersionSafe = "24_9_0"

	// DefaultKeycloakHost is the default external Keycloak hostname for CI environments.
	DefaultKeycloakHost = "keycloak-24-9-0.ci.distro.ultrawombat.com"

	// DefaultKeycloakProtocol is the default protocol for Keycloak connections.
	DefaultKeycloakProtocol = "https"

	// MaxRealmNameLength is the maximum length for Keycloak realm names.
	MaxRealmNameLength = 36

	// DefaultReleaseName is the default Helm release name for deployments.
	DefaultReleaseName = "integration"

	// DefaultTimeoutMinutes is the default timeout for Helm deployments.
	DefaultTimeoutMinutes = 5

	// DefaultTTL is the default time-to-live for deployed resources.
	DefaultTTL = "30m"

	// DefaultIngressBaseDomain is the base domain for CI ingress hostnames.
	DefaultIngressBaseDomain = "ci.distro.ultrawombat.com"
)

// ScenarioFilePrefix is the prefix used for scenario values files.
const ScenarioFilePrefix = "values-integration-test-ingress-"

// ScenarioFileSuffix is the suffix used for scenario values files.
const ScenarioFileSuffix = ".yaml"

// DefaultScenarioSubdir is the default subdirectory path for scenario files within a chart.
const DefaultScenarioSubdir = "test/integration/scenarios/chart-full-setup"


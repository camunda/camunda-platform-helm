package deploy

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
)

var nonIdentifierChars = regexp.MustCompile(`[^a-z0-9-]`)

func normalizeIdentifierPart(s string) string {
	s = strings.ToLower(s)
	s = nonIdentifierChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "x"
	}
	return s
}

// ScenarioContext holds scenario-specific deployment configuration.
type ScenarioContext struct {
	ScenarioName             string
	Namespace                string
	Release                  string
	IngressHost              string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	TasklistIndexPrefix      string
	OperateIndexPrefix       string
	TempDir                  string
}

// ScenarioResult holds the result of a scenario deployment.
type ScenarioResult struct {
	Scenario                 string
	Namespace                string
	Release                  string
	IngressHost              string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	FirstUserPassword        string
	SecondUserPassword       string
	ThirdUserPassword        string
	KeycloakClientsSecret    string
	TestEnvFile              string   // Path to generated E2E test .env file
	LayeredFiles             []string // Source values files resolved from layers (pre-processing)
	Error                    error
}

// PreparedScenario holds the result of values preparation for a scenario,
// ready to be deployed in parallel.
type PreparedScenario struct {
	ScenarioCtx         *ScenarioContext
	ValuesFiles         []string
	LayeredFiles        []string // Source values files resolved from layers (pre-processing)
	VaultSecretPath     string
	TempDir             string
	RealmName           string
	OptimizePrefix      string
	OrchestrationPrefix string
	// Secrets holds auto-generated test credentials (DISTRO_QA_* passwords, keycloak
	// clients secret) so that they can flow from prepareScenarioValues to
	// executeDeployment without going through the process environment.
	Secrets map[string]string
}

// generateScenarioContext creates a scenario-specific deployment context.
func generateScenarioContext(scenario string, flags *config.RuntimeFlags) (*ScenarioContext, error) {
	// Use EffectiveNamespace() to apply any namespace prefix (e.g., for EKS)
	effectiveNs := flags.EffectiveNamespace()
	suffix := namespaceDerivedSuffix(effectiveNs)

	// Generate unique identifiers for this scenario
	var realmName, optimizePrefix, orchestrationPrefix, tasklistPrefix, operatePrefix string
	var namespace, release, ingressHost string

	if flags.Auth.KeycloakRealm != "" && len(flags.Deployment.Scenarios) == 1 {
		realmName = flags.Auth.KeycloakRealm
	} else {
		// Keycloak realm name has a maximum length of 36 characters
		// Generate a compact name that fits within this limit
		realmName = generateCompactRealmName(normalizeIdentifierPart(effectiveNs), normalizeIdentifierPart(scenario), suffix)
	}

	normalizedScenario := normalizeIdentifierPart(scenario)

	if flags.Index.OptimizeIndexPrefix != "" && len(flags.Deployment.Scenarios) == 1 {
		optimizePrefix = flags.Index.OptimizeIndexPrefix
	} else {
		optimizePrefix = fmt.Sprintf("opt-%s-%s", normalizedScenario, suffix)
	}

	if flags.Index.OrchestrationIndexPrefix != "" && len(flags.Deployment.Scenarios) == 1 {
		orchestrationPrefix = flags.Index.OrchestrationIndexPrefix
	} else {
		orchestrationPrefix = fmt.Sprintf("orch-%s-%s", normalizedScenario, suffix)
	}

	if flags.Index.TasklistIndexPrefix != "" && len(flags.Deployment.Scenarios) == 1 {
		tasklistPrefix = flags.Index.TasklistIndexPrefix
	} else {
		tasklistPrefix = fmt.Sprintf("task-%s-%s", normalizedScenario, suffix)
	}

	if flags.Index.OperateIndexPrefix != "" && len(flags.Deployment.Scenarios) == 1 {
		operatePrefix = flags.Index.OperateIndexPrefix
	} else {
		operatePrefix = fmt.Sprintf("op-%s-%s", normalizedScenario, suffix)
	}

	// Generate unique namespace for multi-scenario, but always use "integration" as release name
	// since we never have multiple deployments in the same namespace
	// Use EffectiveNamespace() to apply any namespace prefix (e.g., for EKS)
	resolvedHost := flags.ResolveIngressHostname()
	baseNamespace := flags.EffectiveNamespace()
	if len(flags.Deployment.Scenarios) > 1 {
		namespace = fmt.Sprintf("%s-%s", baseNamespace, scenario)
		if resolvedHost != "" {
			ingressHost = fmt.Sprintf("%s-%s", scenario, resolvedHost)
		}
	} else {
		namespace = baseNamespace
		ingressHost = resolvedHost
	}

	// Always use "integration" as the release name
	release = "integration"

	return &ScenarioContext{
		ScenarioName:             scenario,
		Namespace:                namespace,
		Release:                  release,
		IngressHost:              ingressHost,
		KeycloakRealm:            realmName,
		OptimizeIndexPrefix:      optimizePrefix,
		OrchestrationIndexPrefix: orchestrationPrefix,
		TasklistIndexPrefix:      tasklistPrefix,
		OperateIndexPrefix:       operatePrefix,
	}, nil
}

// namespaceDerivedSuffix produces a deterministic 8-character hex suffix from a
// namespace name. This ensures that install and upgrade deployments targeting the
// same namespace always generate identical index prefixes and Keycloak realm names,
// even when running in separate CI jobs.
//
// Using FNV-1a for speed and good distribution; 32 bits (8 hex chars) provides
// sufficient uniqueness across test namespaces (which already contain shortnames
// and version identifiers).
func namespaceDerivedSuffix(namespace string) string {
	h := fnv.New32a()
	h.Write([]byte(namespace))
	return fmt.Sprintf("%08x", h.Sum32())
}

// generateCompactRealmName creates a realm name that fits within Keycloak's 36 character limit.
// Format: {prefix}-{hash} where hash is derived from namespace+scenario+suffix.
func generateCompactRealmName(namespace, scenario, suffix string) string {
	const maxLength = 36

	// Try simple format first: scenario-suffix (e.g., "keycloak-mt-a8x9z3k1")
	simple := fmt.Sprintf("%s-%s", scenario, suffix)
	if len(simple) <= maxLength {
		return simple
	}

	// If scenario name is too long, truncate it and add a short hash for uniqueness
	// Format: {truncated-scenario}-{short-hash}
	// Reserve 9 characters for "-" + 8 char hash
	maxScenarioLen := maxLength - 9

	if len(scenario) > maxScenarioLen {
		scenario = scenario[:maxScenarioLen]
	}

	// Create a short hash from the full identifier for uniqueness
	fullId := fmt.Sprintf("%s-%s-%s", namespace, scenario, suffix)
	h := fnv.New32a()
	h.Write([]byte(fullId))
	hash := fmt.Sprintf("%08x", h.Sum32())

	result := fmt.Sprintf("%s-%s", scenario, hash)

	// Final safety check - truncate if still too long
	if len(result) > maxLength {
		result = result[:maxLength]
	}

	return result
}

// keycloakVersionSuffix extracts a version suffix from a Keycloak hostname.
// For example, "keycloak-24-9-0.ci.distro.ultrawombat.com" → "24_9_0".
// The hostname is expected to have the form "keycloak-<version>.<domain>",
// where <version> uses hyphens that are replaced with underscores.
// If the hostname does not match this pattern, the full hostname (with
// dots and hyphens replaced by underscores) is returned as a safe fallback.
func keycloakVersionSuffix(host string) string {
	// Take everything before the first dot (the subdomain).
	subdomain := host
	if idx := strings.Index(host, "."); idx >= 0 {
		subdomain = host[:idx]
	}
	// Strip the "keycloak-" prefix if present.
	version := strings.TrimPrefix(subdomain, "keycloak-")
	// Replace hyphens with underscores for env var safety.
	return strings.ReplaceAll(version, "-", "_")
}

// ComputeExpectedOrchestrationPrefix returns the orchestration index prefix that
// will be used when deploy.Execute() runs for the given scenario and flags. This
// allows callers to validate the prefix against external state (e.g., a live Helm
// release) before executing the deployment.
func ComputeExpectedOrchestrationPrefix(scenario string, flags *config.RuntimeFlags) string {
	if flags.Index.OrchestrationIndexPrefix != "" {
		return flags.Index.OrchestrationIndexPrefix
	}
	normalizedScenario := normalizeIdentifierPart(scenario)
	suffix := namespaceDerivedSuffix(flags.EffectiveNamespace())
	return fmt.Sprintf("orch-%s-%s", normalizedScenario, suffix)
}

// PinScenarioPrefixes derives a deterministic suffix from the namespace and writes
// index prefixes + Keycloak realm name into flags so that subsequent calls to
// Execute() (which internally call generateScenarioContext) will reuse the same
// values.
//
// This is critical for multi-step upgrade flows where Step 1 (install old
// version) and Step 2 (upgrade to new version) must share the same index
// prefixes, otherwise the upgraded components try to read/write indices that
// don't match what Step 1 created.
//
// Only call this when len(flags.Deployment.Scenarios) == 1, which is always
// true in the matrix runner.
func PinScenarioPrefixes(scenario string, flags *config.RuntimeFlags) error {
	normalizedScenario := normalizeIdentifierPart(scenario)
	effectiveNs := flags.EffectiveNamespace()
	suffix := namespaceDerivedSuffix(effectiveNs)

	// Pin index prefixes (only if not already set).
	if flags.Index.OptimizeIndexPrefix == "" {
		flags.Index.OptimizeIndexPrefix = fmt.Sprintf("opt-%s-%s", normalizedScenario, suffix)
	}
	if flags.Index.OrchestrationIndexPrefix == "" {
		flags.Index.OrchestrationIndexPrefix = fmt.Sprintf("orch-%s-%s", normalizedScenario, suffix)
	}
	if flags.Index.TasklistIndexPrefix == "" {
		flags.Index.TasklistIndexPrefix = fmt.Sprintf("task-%s-%s", normalizedScenario, suffix)
	}
	if flags.Index.OperateIndexPrefix == "" {
		flags.Index.OperateIndexPrefix = fmt.Sprintf("op-%s-%s", normalizedScenario, suffix)
	}

	// Pin Keycloak realm name (only if not already set).
	if flags.Auth.KeycloakRealm == "" {
		flags.Auth.KeycloakRealm = generateCompactRealmName(
			normalizeIdentifierPart(effectiveNs),
			normalizedScenario,
			suffix,
		)
	}

	logging.Logger.Info().
		Str("scenario", scenario).
		Str("namespace", effectiveNs).
		Str("orchestrationPrefix", flags.Index.OrchestrationIndexPrefix).
		Str("operatePrefix", flags.Index.OperateIndexPrefix).
		Str("optimizePrefix", flags.Index.OptimizeIndexPrefix).
		Str("tasklistPrefix", flags.Index.TasklistIndexPrefix).
		Str("keycloakRealm", flags.Auth.KeycloakRealm).
		Msg("Pinned scenario prefixes")

	return nil
}

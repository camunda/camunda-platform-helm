package deploy

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"math/big"
	"regexp"
	"strings"

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
	suffix, err := generateRandomSuffix()
	if err != nil {
		return nil, fmt.Errorf("failed to generate random suffix: %w", err)
	}

	// Generate unique identifiers for this scenario
	var realmName, optimizePrefix, orchestrationPrefix, tasklistPrefix, operatePrefix string
	var namespace, release, ingressHost string

	// Use provided values or generate unique ones
	// Use EffectiveNamespace() to apply any namespace prefix (e.g., for EKS)
	effectiveNs := flags.EffectiveNamespace()
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

// generateRandomSuffix creates an 8-character random string.
func generateRandomSuffix() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 8)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", fmt.Errorf("crypto/rand failed: %w", err)
		}
		result[i] = chars[num.Int64()]
	}
	return string(result), nil
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

// PinScenarioPrefixes generates a random suffix and writes index prefixes +
// Keycloak realm name into flags so that subsequent calls to Execute() (which
// internally call generateScenarioContext) will reuse the same values instead
// of generating new random ones.
//
// This is critical for multi-step upgrade flows where Step 1 (install old
// version) and Step 2 (upgrade to new version) must share the same index
// prefixes, otherwise the upgraded components try to read/write indices that
// don't match what Step 1 created.
//
// Only call this when len(flags.Deployment.Scenarios) == 1, which is always
// true in the matrix runner.
func PinScenarioPrefixes(scenario string, flags *config.RuntimeFlags) error {
	suffix, err := generateRandomSuffix()
	if err != nil {
		return fmt.Errorf("failed to generate random suffix: %w", err)
	}

	normalizedScenario := normalizeIdentifierPart(scenario)
	effectiveNs := flags.EffectiveNamespace()

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

	return nil
}

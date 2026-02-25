package scenarios

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ValuesFilePrefix = "values-integration-test-ingress-"
	ValuesDir        = "values"

	// Directory names within values/
	IdentityDir    = "identity"
	PersistenceDir = "persistence"
	PlatformDir    = "platform"
	FeaturesDir    = "features"

	// InfraDir is the directory (sibling to the scenario dir) that holds
	// infrastructure pool values files (nodeSelector, tolerations, etc.).
	InfraDir = "infra"
)

// DeploymentConfig describes the configuration for a deployment using the
// selection + composition model. This replaces the old LayeredConfig.
type DeploymentConfig struct {
	// Required selections
	Identity    string // keycloak, keycloak-external, oidc, basic, hybrid
	Persistence string // elasticsearch, opensearch, rdbms, rdbms-oracle
	Platform    string // gke, eks, openshift

	// Optional features (combinable with constraints)
	Features []string // multitenancy, rba, documentstore

	// Optional base modifiers
	QA        bool // Enable QA configuration (test users, etc.)
	ImageTags bool // Enable image tag overrides from env vars
	Upgrade   bool // Enable upgrade flow configuration

	// Infrastructure
	InfraType string // Infrastructure pool type (preemptible, distroci, standard, arm, etc.)

	// Migration support (for helm chart schema changes)
	ChartVersion string // Chart version (e.g., "13.0.0") - used to determine if migrator is needed
	Flow         string // Deployment flow: install, upgrade-patch, upgrade-minor
}

// Validate checks that required fields are set and feature constraints are satisfied.
func (c *DeploymentConfig) Validate() error {
	if c.Identity == "" {
		return errors.New("--identity is required (keycloak, keycloak-external, oidc, basic, hybrid)")
	}
	if c.Persistence == "" {
		return errors.New("--persistence is required (elasticsearch, opensearch, rdbms, rdbms-oracle)")
	}
	if c.Platform == "" {
		return errors.New("--platform is required (gke, eks, openshift)")
	}

	// Validate identity values
	validIdentities := []string{"keycloak", "keycloak-external", "oidc", "basic", "hybrid"}
	if !contains(validIdentities, c.Identity) {
		return fmt.Errorf("invalid --identity value %q: must be one of: %s", c.Identity, strings.Join(validIdentities, ", "))
	}

	// Validate persistence values
	validPersistence := []string{"elasticsearch", "elasticsearch-external", "opensearch", "rdbms", "rdbms-oracle"}
	if !contains(validPersistence, c.Persistence) {
		return fmt.Errorf("invalid --persistence value %q: must be one of: %s", c.Persistence, strings.Join(validPersistence, ", "))
	}

	// Validate platform values
	validPlatforms := []string{"gke", "eks", "openshift"}
	if !contains(validPlatforms, c.Platform) {
		return fmt.Errorf("invalid --platform value %q: must be one of: %s", c.Platform, strings.Join(validPlatforms, ", "))
	}

	// Feature constraints
	if contains(c.Features, "multitenancy") && contains(c.Features, "rba") {
		return errors.New("multitenancy and rba cannot be combined")
	}

	return nil
}

// ResolvePaths returns the ordered list of values files based on the configuration.
// Files are returned in order: base -> base modifiers -> identity -> persistence -> platform -> infra -> features -> migrator -> image-tags
func (c *DeploymentConfig) ResolvePaths(scenariosDir string) ([]string, error) {
	valuesDir := filepath.Join(scenariosDir, ValuesDir)

	// Check if values directory exists
	if _, err := os.Stat(valuesDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("values directory not found: %s", valuesDir)
	}

	var files []string

	// Layer 1: Base (always required)
	basePath := filepath.Join(valuesDir, "base.yaml")
	if _, err := os.Stat(basePath); err != nil {
		return nil, fmt.Errorf("base.yaml not found: %s", basePath)
	}
	files = append(files, basePath)

	// Layer 2: Base modifiers (QA, Upgrade) - applied early as they extend base
	if c.QA {
		qaPath := filepath.Join(valuesDir, "base-qa.yaml")
		if _, err := os.Stat(qaPath); err == nil {
			files = append(files, qaPath)
		}
	}
	if c.Upgrade {
		upgradePath := filepath.Join(valuesDir, "base-upgrade.yaml")
		if _, err := os.Stat(upgradePath); err == nil {
			files = append(files, upgradePath)
		}
	}

	// Layer 3: Identity selection
	if c.Identity != "" {
		identityPath := filepath.Join(valuesDir, IdentityDir, c.Identity+".yaml")
		if _, err := os.Stat(identityPath); err == nil {
			files = append(files, identityPath)
		}
	}

	// Layer 4: Persistence selection
	if c.Persistence != "" {
		persistencePath := filepath.Join(valuesDir, PersistenceDir, c.Persistence+".yaml")
		if _, err := os.Stat(persistencePath); err == nil {
			files = append(files, persistencePath)
		}
	}

	// Layer 5: Platform selection
	if c.Platform != "" {
		platformPath := filepath.Join(valuesDir, PlatformDir, c.Platform+".yaml")
		if _, err := os.Stat(platformPath); err == nil {
			files = append(files, platformPath)
		}
	}

	// Layer 5b: Infrastructure pool selection (nodeSelector/tolerations)
	// Infra files live in a sibling "infra/" directory next to the scenario dir
	// (e.g., ../infra/values-infra-<suffix>.yaml relative to scenariosDir).
	// The suffix follows the legacy Taskfile convention:
	//   - For EKS: "eks-<infraType>" (e.g., "eks-preemptible")
	//   - For other platforms: "<infraType>" (e.g., "preemptible", "distroci")
	if c.InfraType != "" {
		infraSuffix := c.InfraType
		if c.Platform == "eks" {
			infraSuffix = "eks-" + c.InfraType
		}
		infraPath := filepath.Join(scenariosDir, "..", InfraDir, fmt.Sprintf("values-infra-%s.yaml", infraSuffix))
		if _, err := os.Stat(infraPath); err == nil {
			files = append(files, infraPath)
		}
	}

	// Layer 6: Features (in deterministic order)
	featureOrder := []string{"multitenancy", "rba", "documentstore"}
	for _, feature := range featureOrder {
		if contains(c.Features, feature) {
			featurePath := filepath.Join(valuesDir, FeaturesDir, feature+".yaml")
			if _, err := os.Stat(featurePath); err == nil {
				files = append(files, featurePath)
			}
		}
	}

	// Layer 7: Migrator (for chart version 13.x minor upgrades)
	// The migrator handles schema migrations like global.security.initialization -> orchestration.security.initialization
	// Include when: chart version starts with "13" AND flow is not "upgrade-patch" (i.e., it's a minor upgrade or install)
	if c.needsMigrator() {
		// Migrator file is at chart-full-setup/values-enable-migrator.yaml (one level up from values/)
		migratorPath := filepath.Join(scenariosDir, "values-enable-migrator.yaml")
		if _, err := os.Stat(migratorPath); err == nil {
			files = append(files, migratorPath)
		}
	}

	// Layer 8: Image tags (applied last, needs env var processing)
	// Note: ImageTags file path is returned but the caller is responsible
	// for processing environment variable substitution
	if c.ImageTags {
		imageTagsPath := filepath.Join(valuesDir, "base-image-tags.yaml")
		if _, err := os.Stat(imageTagsPath); err == nil {
			files = append(files, imageTagsPath)
		}
	}

	return files, nil
}

// needsMigrator returns true if the migrator values file should be included.
// The migrator is needed for chart version 13.x when the flow is not a patch upgrade.
func (c *DeploymentConfig) needsMigrator() bool {
	// Only needed for chart version 13.x
	if !strings.HasPrefix(c.ChartVersion, "13") {
		return false
	}

	// Skip for patch upgrades (they don't need schema migration)
	if c.Flow == "upgrade-patch" {
		return false
	}

	// Include for install, upgrade-minor, or any other flow on chart 13.x
	return true
}

// MapScenarioToConfig converts a legacy scenario name to a DeploymentConfig.
// This provides backward compatibility with the old --scenario flag.
func MapScenarioToConfig(scenario string) *DeploymentConfig {
	config := &DeploymentConfig{}

	// Normalize scenario name for matching
	s := strings.ToLower(scenario)

	// Derive QA mode from prefix
	if strings.HasPrefix(s, "qa-") {
		config.QA = true
	}

	// Handle well-known composite scenarios that can't be derived from name parsing alone.
	// keycloak-original historically means: external Keycloak + external Elasticsearch.
	// The name is misleading (it refers to the "original" test config format), but
	// 3rd parties call test-integration-template.yaml with scenario: keycloak-original,
	// so it must keep working.
	if s == "keycloak-original" {
		config.Identity = "keycloak-external"
		config.Persistence = "elasticsearch-external"
		config.Platform = "gke"
		return config
	}

	// Derive identity
	switch {
	case strings.Contains(s, "keycloak-mt") || strings.Contains(s, "-mt-") || strings.Contains(s, "multitenancy"):
		config.Identity = "keycloak-external"
	case strings.Contains(s, "oidc") || strings.Contains(s, "entra"):
		config.Identity = "oidc"
	case strings.Contains(s, "basic"):
		config.Identity = "basic"
	case strings.Contains(s, "hybrid"):
		config.Identity = "hybrid"
	default:
		config.Identity = "keycloak"
	}

	// Derive persistence
	switch {
	case strings.Contains(s, "opensearch"):
		config.Persistence = "opensearch"
	case strings.Contains(s, "rdbms") && strings.Contains(s, "oracle"):
		config.Persistence = "rdbms-oracle"
	case strings.Contains(s, "rdbms"):
		config.Persistence = "rdbms"
	default:
		config.Persistence = "elasticsearch"
	}

	// Derive platform (default to gke)
	switch {
	case strings.Contains(s, "eks"):
		config.Platform = "eks"
	case strings.Contains(s, "openshift") || strings.Contains(s, "rosa"):
		config.Platform = "openshift"
	default:
		config.Platform = "gke"
	}

	// Derive features
	if strings.Contains(s, "-mt") || strings.Contains(s, "multitenancy") {
		config.Features = append(config.Features, "multitenancy")
	}
	if strings.Contains(s, "-rba") || strings.Contains(s, "rba") {
		config.Features = append(config.Features, "rba")
	}
	if strings.Contains(s, "document") {
		config.Features = append(config.Features, "documentstore")
	}

	// Derive upgrade mode
	if strings.Contains(s, "upgrade") || strings.Contains(s, "-upg") {
		config.Upgrade = true
	}

	return config
}

// LayeredConfig is deprecated - use DeploymentConfig instead.
// Kept for backward compatibility during transition.
type LayeredConfig = DeploymentConfig

// DeriveLayeredConfig is deprecated - use MapScenarioToConfig instead.
func DeriveLayeredConfig(scenario string) *LayeredConfig {
	return MapScenarioToConfig(scenario)
}

// ResolvePath determines the source values file path for a scenario.
// It first tries the new values structure (values/ directory), then falls back
// to the legacy single-file approach (values-integration-test-ingress-*.yaml).
func ResolvePath(scenariosDir, scenario string) (string, error) {
	// First, try to resolve using new values structure
	config := MapScenarioToConfig(scenario)
	paths, err := config.ResolvePaths(scenariosDir)
	if err == nil && len(paths) > 0 {
		// Return the first file (base.yaml) as the "source" for compatibility
		// The full list should be obtained via ResolvePaths
		return paths[0], nil
	}

	// Fall back to legacy single-file approach
	var filename string
	if strings.HasPrefix(scenario, ValuesFilePrefix) && strings.HasSuffix(scenario, ".yaml") {
		filename = scenario
	} else {
		filename = fmt.Sprintf("%s%s.yaml", ValuesFilePrefix, scenario)
	}

	sourceValuesFile := filepath.Join(scenariosDir, filename)
	if _, err := os.Stat(sourceValuesFile); err != nil {
		return "", fmt.Errorf("scenario values file not found: %w", err)
	}
	return sourceValuesFile, nil
}

// ResolveLayeredPaths is deprecated - use DeploymentConfig.ResolvePaths instead.
// Kept for backward compatibility during transition.
func ResolveLayeredPaths(scenariosDir, scenario string, explicitConfig *LayeredConfig) ([]string, error) {
	config := explicitConfig
	if config == nil {
		config = MapScenarioToConfig(scenario)
	}
	return config.ResolvePaths(scenariosDir)
}

// HasLayeredValues checks if the scenario directory has the values structure.
func HasLayeredValues(scenariosDir string) bool {
	valuesDir := filepath.Join(scenariosDir, ValuesDir)
	basePath := filepath.Join(valuesDir, "base.yaml")
	_, err := os.Stat(basePath)
	return err == nil
}

// List returns a list of available scenario names (stripped of prefix/suffix).
// This returns legacy scenarios - for the new model, use List* functions.
func List(scenariosDir string) ([]string, error) {
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return nil, err
	}

	var scenarios []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), ValuesFilePrefix) && strings.HasSuffix(e.Name(), ".yaml") {
			name := strings.TrimPrefix(e.Name(), ValuesFilePrefix)
			name = strings.TrimSuffix(name, ".yaml")
			scenarios = append(scenarios, name)
		}
	}
	return scenarios, nil
}

// ListIdentities returns available identity types from the values structure.
func ListIdentities(scenariosDir string) ([]string, error) {
	identityDir := filepath.Join(scenariosDir, ValuesDir, IdentityDir)
	return listYamlFiles(identityDir)
}

// ListPersistence returns available persistence types from the values structure.
func ListPersistence(scenariosDir string) ([]string, error) {
	persistenceDir := filepath.Join(scenariosDir, ValuesDir, PersistenceDir)
	return listYamlFiles(persistenceDir)
}

// ListPlatforms returns available platforms from the values structure.
func ListPlatforms(scenariosDir string) ([]string, error) {
	platformDir := filepath.Join(scenariosDir, ValuesDir, PlatformDir)
	return listYamlFiles(platformDir)
}

// ListFeatures returns available features from the values structure.
func ListFeatures(scenariosDir string) ([]string, error) {
	featuresDir := filepath.Join(scenariosDir, ValuesDir, FeaturesDir)
	return listYamlFiles(featuresDir)
}

// Deprecated compatibility functions - these map to the new naming

// ListLayeredAuthTypes is deprecated - use ListIdentities instead.
func ListLayeredAuthTypes(scenariosDir string) ([]string, error) {
	return ListIdentities(scenariosDir)
}

// ListLayeredBackends is deprecated - use ListPersistence instead.
func ListLayeredBackends(scenariosDir string) ([]string, error) {
	return ListPersistence(scenariosDir)
}

// ListLayeredFeatures is deprecated - use ListFeatures instead.
func ListLayeredFeatures(scenariosDir string) ([]string, error) {
	return ListFeatures(scenariosDir)
}

// listYamlFiles returns a list of yaml files in a directory (without extension).
func listYamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			name := strings.TrimSuffix(e.Name(), ".yaml")
			files = append(files, name)
		}
	}
	return files, nil
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

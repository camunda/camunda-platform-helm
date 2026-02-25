// Package versionmatrix provides utilities for loading and querying
// the Camunda version-matrix JSON files that map app versions to Helm
// chart versions. It is used by the matrix runner to resolve the
// "from" chart version for upgrade-patch and upgrade-minor flows.
package versionmatrix

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Pre-upgrade script constants.
const (
	// PreSetupScriptsDir is the directory name under each chart version's test/integration/scenarios
	// that holds pre-upgrade lifecycle scripts.
	PreSetupScriptsDir = "pre-setup-scripts"
)

// PreUpgradeScriptName returns the pre-upgrade script filename for a given flow.
// Maps flow to script name:
//
//	"upgrade-patch"           → "pre-upgrade-patch.sh"
//	"upgrade-minor"           → "pre-upgrade-minor.sh"
//	"modular-upgrade-minor"   → "pre-upgrade-minor.sh"
//
// Returns empty string for non-upgrade flows.
func PreUpgradeScriptName(flow string) string {
	switch flow {
	case "upgrade-patch":
		return "pre-upgrade-patch.sh"
	case "upgrade-minor", "modular-upgrade-minor":
		return "pre-upgrade-minor.sh"
	default:
		return ""
	}
}

// PreUpgradeScriptPath returns the absolute path to the pre-upgrade script for a given
// app version and flow. The path follows the convention:
//
//	charts/camunda-platform-<appVersion>/test/integration/scenarios/pre-setup-scripts/pre-upgrade-<suffix>.sh
//
// Returns empty string if the flow doesn't have a pre-upgrade script.
func PreUpgradeScriptPath(repoRoot, appVersion, flow string) string {
	name := PreUpgradeScriptName(flow)
	if name == "" {
		return ""
	}
	return filepath.Join(repoRoot, "charts", "camunda-platform-"+appVersion,
		"test", "integration", "scenarios", PreSetupScriptsDir, name)
}

// HasPreUpgradeScript returns true if a pre-upgrade script exists on disk for the
// given app version and flow.
func HasPreUpgradeScript(repoRoot, appVersion, flow string) bool {
	p := PreUpgradeScriptPath(repoRoot, appVersion, flow)
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// ChartEntry represents a single entry in a version-matrix.json file.
type ChartEntry struct {
	ChartVersion string   `json:"chart_version"`
	ChartImages  []string `json:"chart_images"`
}

// LoadVersionMatrix reads and parses the version-matrix.json for the given app version.
// repoRoot is the repository root, appVersion is e.g. "8.8".
func LoadVersionMatrix(repoRoot, appVersion string) ([]ChartEntry, error) {
	path := filepath.Join(repoRoot, "version-matrix", "camunda-"+appVersion, "version-matrix.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read version-matrix for %s: %w", appVersion, err)
	}
	var entries []ChartEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse version-matrix for %s: %w", appVersion, err)
	}
	return entries, nil
}

// IsStableVersion returns true if the chart version has no pre-release suffix.
// A stable version contains only numeric dot-separated components (e.g., "13.5.0").
// Versions like "13.0.0-alpha2" or "14.0.0-alpha4.1" are NOT stable.
func IsStableVersion(version string) bool {
	return !strings.Contains(version, "-")
}

// StableVersions filters a slice of ChartEntry to only stable (non-pre-release) versions.
func StableVersions(entries []ChartEntry) []ChartEntry {
	var stable []ChartEntry
	for _, e := range entries {
		if IsStableVersion(e.ChartVersion) {
			stable = append(stable, e)
		}
	}
	return stable
}

// LatestStableVersion returns the highest stable chart version from the given entries.
// Returns an error if no stable versions exist.
func LatestStableVersion(entries []ChartEntry) (string, error) {
	stable := StableVersions(entries)
	if len(stable) == 0 {
		return "", fmt.Errorf("no stable chart versions found")
	}

	slices.SortFunc(stable, func(a, b ChartEntry) int {
		return CompareChartVersions(a.ChartVersion, b.ChartVersion)
	})

	return stable[len(stable)-1].ChartVersion, nil
}

// PreviousAppVersion decrements the minor component of an app version.
// "8.8" -> "8.7", "8.2" -> "8.1".
// Returns an error if the minor version is 0 or the format is invalid.
func PreviousAppVersion(appVersion string) (string, error) {
	parts := strings.SplitN(appVersion, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid app version format %q: expected major.minor", appVersion)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}
	if minor <= 0 {
		return "", fmt.Errorf("cannot compute previous app version for %q: minor version is %d", appVersion, minor)
	}
	return fmt.Sprintf("%d.%d", major, minor-1), nil
}

// ResolveUpgradeFromVersion determines the "from" chart version for upgrade flows.
//
// For "upgrade-patch": returns the latest stable chart version for the SAME app version.
// For "upgrade-minor": returns the latest stable chart version for the PREVIOUS app version.
//
// repoRoot is the repository root containing version-matrix/ directories.
// appVersion is the target app version (e.g., "8.8").
// flow is "upgrade-patch" or "upgrade-minor".
func ResolveUpgradeFromVersion(repoRoot, appVersion, flow string) (string, error) {
	var lookupVersion string

	switch flow {
	case "upgrade-patch":
		lookupVersion = appVersion
	case "upgrade-minor", "modular-upgrade-minor":
		prev, err := PreviousAppVersion(appVersion)
		if err != nil {
			return "", fmt.Errorf("resolve upgrade-minor from version: %w", err)
		}
		lookupVersion = prev
	default:
		return "", fmt.Errorf("unsupported upgrade flow %q: expected upgrade-patch, upgrade-minor, or modular-upgrade-minor", flow)
	}

	entries, err := LoadVersionMatrix(repoRoot, lookupVersion)
	if err != nil {
		return "", fmt.Errorf("load version-matrix for %s (flow %s): %w", lookupVersion, flow, err)
	}

	version, err := LatestStableVersion(entries)
	if err != nil {
		return "", fmt.Errorf("find latest stable version for %s (flow %s): %w", lookupVersion, flow, err)
	}

	return version, nil
}

// CompareChartVersions compares two semver-like chart version strings.
// Only the numeric components (major.minor.patch) are compared.
// Pre-release suffixes are ignored for ordering purposes -- they are
// filtered out before this function is typically called.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareChartVersions(a, b string) int {
	aParts := parseChartVersion(a)
	bParts := parseChartVersion(b)

	for i := 0; i < 3; i++ {
		if aParts[i] != bParts[i] {
			if aParts[i] < bParts[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// parseChartVersion extracts [major, minor, patch] from a version string.
// Pre-release suffixes (after "-") are stripped before parsing.
// Returns [0,0,0] for unparseable input.
func parseChartVersion(v string) [3]int {
	// Strip pre-release suffix (e.g., "13.0.0-alpha2" -> "13.0.0")
	if idx := strings.Index(v, "-"); idx >= 0 {
		v = v[:idx]
	}

	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}

// IsUpgradeFlow returns true if the flow string represents any kind of upgrade flow
// (two-step or upgrade-only). Use IsTwoStepUpgradeFlow or IsUpgradeOnlyFlow for
// more specific checks.
func IsUpgradeFlow(flow string) bool {
	return flow == "upgrade-patch" || flow == "upgrade-minor" || flow == "modular-upgrade-minor"
}

// IsTwoStepUpgradeFlow returns true if the flow is a self-contained two-step upgrade:
// Step 1 installs the previous chart version from the Helm repo, Step 2 upgrades to
// the current local chart. This applies to "upgrade-patch" and "upgrade-minor".
//
// "modular-upgrade-minor" is NOT a two-step flow — it assumes an existing deployment
// from a prior "install" run and only performs the upgrade step.
func IsTwoStepUpgradeFlow(flow string) bool {
	return flow == "upgrade-patch" || flow == "upgrade-minor"
}

// IsUpgradeOnlyFlow returns true if the flow performs only the upgrade step against
// an already-running deployment. This applies to "modular-upgrade-minor", which
// expects a prior "install" flow to have deployed the previous version.
//
// In CI, "modular-upgrade-minor" skips the install job entirely and runs only the
// upgrade job (pre-upgrade script + helm upgrade) against the install flow's namespace.
func IsUpgradeOnlyFlow(flow string) bool {
	return flow == "modular-upgrade-minor"
}

// Helm repo constants for the Camunda chart repository.
const (
	// DefaultHelmRepoName is the local alias for the Camunda Helm repository.
	DefaultHelmRepoName = "camunda"

	// DefaultHelmRepoURL is the URL of the Camunda Helm chart repository.
	DefaultHelmRepoURL = "https://helm.camunda.io"

	// DefaultHelmChartRef is the repo/chart reference for helm install/upgrade.
	DefaultHelmChartRef = "camunda/camunda-platform"
)

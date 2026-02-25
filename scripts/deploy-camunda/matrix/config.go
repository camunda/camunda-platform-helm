package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChartVersions holds the parsed content of charts/chart-versions.yaml.
type ChartVersions struct {
	CamundaVersions struct {
		Alpha           []string `yaml:"alpha"`
		SupportStandard []string `yaml:"supportStandard"`
		SupportExtended []string `yaml:"supportExtended"`
		EndOfLife       []string `yaml:"endOfLife"`
	} `yaml:"camundaVersions"`
}

// ActiveVersions returns the list of active versions (alpha + supportStandard).
func (cv *ChartVersions) ActiveVersions() []string {
	var versions []string
	versions = append(versions, cv.CamundaVersions.Alpha...)
	versions = append(versions, cv.CamundaVersions.SupportStandard...)
	return versions
}

// LoadChartVersions reads and parses charts/chart-versions.yaml from the repo root.
func LoadChartVersions(repoRoot string) (*ChartVersions, error) {
	path := filepath.Join(repoRoot, "charts", "chart-versions.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read chart-versions.yaml: %w", err)
	}
	var cv ChartVersions
	if err := yaml.Unmarshal(data, &cv); err != nil {
		return nil, fmt.Errorf("failed to parse chart-versions.yaml: %w", err)
	}
	return &cv, nil
}

// CITestConfig holds the parsed content of a ci-test-config.yaml file.
type CITestConfig struct {
	Integration struct {
		Vars struct {
			TasksBaseDir  string `yaml:"tasksBaseDir"`
			ValuesBaseDir string `yaml:"valuesBaseDir"`
			ChartsBaseDir string `yaml:"chartsBaseDir"`
		} `yaml:"vars"`
		Case struct {
			PR struct {
				Scenarios []CIScenario `yaml:"scenario"`
			} `yaml:"pr"`
			Nightly struct {
				Scenarios []CIScenario `yaml:"scenario"`
			} `yaml:"nightly"`
		} `yaml:"case"`
	} `yaml:"integration"`
}

// CIScenario represents a single scenario entry in ci-test-config.yaml.
type CIScenario struct {
	Name      string   `yaml:"name"`
	Enabled   bool     `yaml:"enabled"`
	Shortname string   `yaml:"shortname"`
	Auth      string   `yaml:"auth"`
	Flow      string   `yaml:"flow"`
	Platforms []string `yaml:"platforms"`
	Exclude   []string `yaml:"exclude"`

	// InfraType maps platform names to infrastructure pool types, e.g.,
	// {"gke": "distroci", "eks": "preemptible"}.
	// The resolved value selects the values-infra-<suffix>.yaml file at deployment time.
	InfraType map[string]string `yaml:"infra-type,omitempty"`

	// Selection + Composition fields (explicit layer overrides).
	// When set, these take precedence over name-based derivation in MapScenarioToConfig.
	Identity    string   `yaml:"identity,omitempty"`
	Persistence string   `yaml:"persistence,omitempty"`
	Features    []string `yaml:"features,omitempty"`
}

// LoadCITestConfig reads and parses the ci-test-config.yaml for a given chart directory.
func LoadCITestConfig(chartDir string) (*CITestConfig, error) {
	path := filepath.Join(chartDir, "test", "ci-test-config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read ci-test-config.yaml from %s: %w", chartDir, err)
	}
	var cfg CITestConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse ci-test-config.yaml from %s: %w", chartDir, err)
	}
	return &cfg, nil
}

// PermittedFlows holds the parsed content of .github/config/permitted-flows.yaml.
type PermittedFlows struct {
	Defaults struct {
		Flows []string `yaml:"flows"`
	} `yaml:"defaults"`
	Rules []PermittedFlowRule `yaml:"rules"`
}

// PermittedFlowRule represents a single deny rule.
type PermittedFlowRule struct {
	Match string   `yaml:"match"`
	Deny  []string `yaml:"deny"`
}

// LoadPermittedFlows reads and parses .github/config/permitted-flows.yaml.
func LoadPermittedFlows(repoRoot string) (*PermittedFlows, error) {
	path := filepath.Join(repoRoot, ".github", "config", "permitted-flows.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read permitted-flows.yaml: %w", err)
	}
	var pf PermittedFlows
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse permitted-flows.yaml: %w", err)
	}
	return &pf, nil
}

// FilterFlows removes denied flows for a given version based on the permitted-flows rules.
// It returns the filtered list of flows that are permitted for the version.
func FilterFlows(pf *PermittedFlows, version string, flows []string) []string {
	// Build the set of denied flows for this version
	denied := make(map[string]bool)
	for _, rule := range pf.Rules {
		if matchesVersion(rule.Match, version) {
			for _, flow := range rule.Deny {
				denied[flow] = true
			}
		}
	}

	// Filter out denied flows
	var permitted []string
	for _, flow := range flows {
		if !denied[flow] {
			permitted = append(permitted, flow)
		}
	}
	return permitted
}

// matchesVersion checks if a version matches a semver-like constraint.
// Supports: "==X.Y", "<=X.Y", ">=X.Y", "<X.Y", ">X.Y".
func matchesVersion(constraint, version string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return false
	}

	// Extract operator and target version
	var op, target string
	if strings.HasPrefix(constraint, "<=") {
		op = "<="
		target = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, ">=") {
		op = ">="
		target = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, "==") {
		op = "=="
		target = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, "<") {
		op = "<"
		target = strings.TrimSpace(constraint[1:])
	} else if strings.HasPrefix(constraint, ">") {
		op = ">"
		target = strings.TrimSpace(constraint[1:])
	} else {
		// No operator means exact match
		op = "=="
		target = constraint
	}

	cmp := compareVersions(version, target)
	switch op {
	case "==":
		return cmp == 0
	case "<=":
		return cmp <= 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case ">":
		return cmp > 0
	}
	return false
}

// compareVersions compares two "major.minor" version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) int {
	aMajor, aMinor := parseVersion(a)
	bMajor, bMinor := parseVersion(b)

	if aMajor != bMajor {
		if aMajor < bMajor {
			return -1
		}
		return 1
	}
	if aMinor != bMinor {
		if aMinor < bMinor {
			return -1
		}
		return 1
	}
	return 0
}

// parseVersion extracts major and minor from a "X.Y" string.
func parseVersion(v string) (int, int) {
	parts := strings.SplitN(v, ".", 2)
	major := 0
	minor := 0
	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}

package versionmatrix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsStableVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"13.5.0", true},
		{"12.7.6", true},
		{"1.0.0", true},
		{"13.0.0-alpha2", false},
		{"14.0.0-alpha4.1", false},
		{"13.0.0-rc1", false},
		{"0.0.0-ci-snapshot", false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := IsStableVersion(tt.version)
			if got != tt.want {
				t.Errorf("IsStableVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestCompareChartVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"13.5.0", "13.5.0", 0},
		{"13.5.0", "13.4.0", 1},
		{"13.4.0", "13.5.0", -1},
		{"12.7.6", "13.0.0", -1},
		{"14.0.0", "13.5.0", 1},
		{"13.0.0", "13.0.1", -1},
		{"13.0.1", "13.0.0", 1},
		// Pre-release suffixes are stripped; only numeric parts compared.
		{"13.0.0-alpha2", "13.0.0", 0},
		{"14.0.0-alpha1", "13.5.0", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareChartVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareChartVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestLatestStableVersion(t *testing.T) {
	tests := []struct {
		name    string
		entries []ChartEntry
		want    string
		wantErr bool
	}{
		{
			name: "mixed stable and alpha",
			entries: []ChartEntry{
				{ChartVersion: "13.0.0-alpha2"},
				{ChartVersion: "13.0.0-alpha3"},
				{ChartVersion: "13.0.0"},
				{ChartVersion: "13.1.0"},
				{ChartVersion: "13.2.0"},
				{ChartVersion: "13.5.0"},
				{ChartVersion: "13.4.0"},
			},
			want: "13.5.0",
		},
		{
			name: "all alpha",
			entries: []ChartEntry{
				{ChartVersion: "14.0.0-alpha1"},
				{ChartVersion: "14.0.0-alpha2"},
				{ChartVersion: "14.0.0-alpha4"},
			},
			wantErr: true,
		},
		{
			name:    "empty",
			entries: []ChartEntry{},
			wantErr: true,
		},
		{
			name: "single stable",
			entries: []ChartEntry{
				{ChartVersion: "11.0.0"},
			},
			want: "11.0.0",
		},
		{
			name: "unsorted input",
			entries: []ChartEntry{
				{ChartVersion: "12.7.6"},
				{ChartVersion: "12.0.0"},
				{ChartVersion: "12.7.3"},
				{ChartVersion: "12.5.0"},
				{ChartVersion: "12.7.5"},
			},
			want: "12.7.6",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LatestStableVersion(tt.entries)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LatestStableVersion() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Errorf("LatestStableVersion() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("LatestStableVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreviousAppVersion(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"8.8", "8.7", false},
		{"8.2", "8.1", false},
		{"8.9", "8.8", false},
		{"10.3", "10.2", false},
		// Error cases.
		{"8.0", "", true},   // minor is 0
		{"8", "", true},     // no minor component
		{"abc.2", "", true}, // non-numeric major
		{"8.xyz", "", true}, // non-numeric minor
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := PreviousAppVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("PreviousAppVersion(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("PreviousAppVersion(%q) error = %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("PreviousAppVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsUpgradeFlow(t *testing.T) {
	tests := []struct {
		flow string
		want bool
	}{
		{"upgrade-patch", true},
		{"upgrade-minor", true},
		{"modular-upgrade-minor", true},
		{"install", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			if got := IsUpgradeFlow(tt.flow); got != tt.want {
				t.Errorf("IsUpgradeFlow(%q) = %v, want %v", tt.flow, got, tt.want)
			}
		})
	}
}

func TestIsTwoStepUpgradeFlow(t *testing.T) {
	tests := []struct {
		flow string
		want bool
	}{
		{"upgrade-patch", true},
		{"upgrade-minor", true},
		{"modular-upgrade-minor", false}, // modular-upgrade-minor is upgrade-only, NOT two-step
		{"install", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			if got := IsTwoStepUpgradeFlow(tt.flow); got != tt.want {
				t.Errorf("IsTwoStepUpgradeFlow(%q) = %v, want %v", tt.flow, got, tt.want)
			}
		})
	}
}

func TestIsUpgradeOnlyFlow(t *testing.T) {
	tests := []struct {
		flow string
		want bool
	}{
		{"modular-upgrade-minor", true},
		{"upgrade-patch", false},
		{"upgrade-minor", false},
		{"install", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			if got := IsUpgradeOnlyFlow(tt.flow); got != tt.want {
				t.Errorf("IsUpgradeOnlyFlow(%q) = %v, want %v", tt.flow, got, tt.want)
			}
		})
	}
}

// TestResolveUpgradeFromVersion uses real version-matrix files from the repo.
// If the repo root doesn't have version-matrix data, the test is skipped.
func TestResolveUpgradeFromVersion(t *testing.T) {
	// Find repo root by walking up from the test file location.
	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("cannot find repo root with version-matrix data")
	}

	tests := []struct {
		name       string
		appVersion string
		flow       string
		wantErr    bool
		// We check minimum expectations rather than exact versions
		// since version-matrix files change over time.
		checkFn func(t *testing.T, version string)
	}{
		{
			name:       "upgrade-patch 8.8",
			appVersion: "8.8",
			flow:       "upgrade-patch",
			checkFn: func(t *testing.T, version string) {
				// Should be a 13.x.x version (chart major for 8.8).
				if !IsStableVersion(version) {
					t.Errorf("expected stable version, got %q", version)
				}
				parts := parseChartVersion(version)
				if parts[0] != 13 {
					t.Errorf("expected major 13 for 8.8 patch upgrade, got %d (version %q)", parts[0], version)
				}
			},
		},
		{
			name:       "upgrade-minor 8.8",
			appVersion: "8.8",
			flow:       "upgrade-minor",
			checkFn: func(t *testing.T, version string) {
				// Should be a 12.x.x version (chart major for 8.7).
				if !IsStableVersion(version) {
					t.Errorf("expected stable version, got %q", version)
				}
				parts := parseChartVersion(version)
				if parts[0] != 12 {
					t.Errorf("expected major 12 for 8.8 minor upgrade, got %d (version %q)", parts[0], version)
				}
			},
		},
		{
			name:       "unsupported flow",
			appVersion: "8.8",
			flow:       "install",
			wantErr:    true,
		},
		{
			name:       "upgrade-minor from 8.0 (no previous)",
			appVersion: "8.0",
			flow:       "upgrade-minor",
			wantErr:    true, // minor is 0, can't go to 8.-1
		},
		{
			name:       "modular-upgrade-minor 8.8 (same as upgrade-minor)",
			appVersion: "8.8",
			flow:       "modular-upgrade-minor",
			checkFn: func(t *testing.T, version string) {
				// Should behave the same as upgrade-minor: 12.x.x version (chart major for 8.7).
				if !IsStableVersion(version) {
					t.Errorf("expected stable version, got %q", version)
				}
				parts := parseChartVersion(version)
				if parts[0] != 12 {
					t.Errorf("expected major 12 for 8.8 modular-upgrade-minor, got %d (version %q)", parts[0], version)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUpgradeFromVersion(repoRoot, tt.appVersion, tt.flow)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveUpgradeFromVersion() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveUpgradeFromVersion() error = %v", err)
				return
			}
			if tt.checkFn != nil {
				tt.checkFn(t, got)
			}
		})
	}
}

// TestLoadVersionMatrix verifies that LoadVersionMatrix can read real version-matrix files.
func TestLoadVersionMatrix(t *testing.T) {
	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("cannot find repo root with version-matrix data")
	}

	entries, err := LoadVersionMatrix(repoRoot, "8.8")
	if err != nil {
		t.Fatalf("LoadVersionMatrix(repoRoot, '8.8') error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("LoadVersionMatrix returned no entries for 8.8")
	}

	// Verify structure: each entry should have a non-empty chart_version.
	for i, e := range entries {
		if e.ChartVersion == "" {
			t.Errorf("entry[%d] has empty chart_version", i)
		}
	}
}

// TestStableVersions verifies stable version filtering.
func TestStableVersions(t *testing.T) {
	entries := []ChartEntry{
		{ChartVersion: "13.0.0-alpha2"},
		{ChartVersion: "13.0.0"},
		{ChartVersion: "13.0.0-alpha4.1"},
		{ChartVersion: "13.1.0"},
		{ChartVersion: "13.2.0-rc1"},
	}

	stable := StableVersions(entries)
	if len(stable) != 2 {
		t.Fatalf("StableVersions() returned %d entries, want 2", len(stable))
	}
	if stable[0].ChartVersion != "13.0.0" {
		t.Errorf("stable[0] = %q, want 13.0.0", stable[0].ChartVersion)
	}
	if stable[1].ChartVersion != "13.1.0" {
		t.Errorf("stable[1] = %q, want 13.1.0", stable[1].ChartVersion)
	}
}

// findRepoRoot walks up from the current directory to find the repo root
// (identified by the presence of version-matrix/ directory).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	// Start from the current working directory.
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "version-matrix")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root
		}
		dir = parent
	}
}

func TestPreUpgradeScriptName(t *testing.T) {
	tests := []struct {
		flow string
		want string
	}{
		{"upgrade-patch", "pre-upgrade-patch.sh"},
		{"upgrade-minor", "pre-upgrade-minor.sh"},
		{"modular-upgrade-minor", "pre-upgrade-minor.sh"},
		{"install", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			got := PreUpgradeScriptName(tt.flow)
			if got != tt.want {
				t.Errorf("PreUpgradeScriptName(%q) = %q, want %q", tt.flow, got, tt.want)
			}
		})
	}
}

func TestPreUpgradeScriptPath(t *testing.T) {
	got := PreUpgradeScriptPath("/repo", "8.9", "upgrade-patch")
	want := filepath.Join("/repo", "charts", "camunda-platform-8.9",
		"test", "integration", "scenarios", "pre-setup-scripts", "pre-upgrade-patch.sh")
	if got != want {
		t.Errorf("PreUpgradeScriptPath() = %q, want %q", got, want)
	}

	// Non-upgrade flow returns empty.
	if got := PreUpgradeScriptPath("/repo", "8.9", "install"); got != "" {
		t.Errorf("PreUpgradeScriptPath(install) = %q, want empty", got)
	}
}

func TestHasPreUpgradeScript(t *testing.T) {
	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("cannot find repo root")
	}

	// 8.9 has a pre-upgrade-patch.sh with real content.
	if !HasPreUpgradeScript(repoRoot, "8.9", "upgrade-patch") {
		t.Error("expected HasPreUpgradeScript(8.9, upgrade-patch) = true")
	}

	// Non-upgrade flow should always return false.
	if HasPreUpgradeScript(repoRoot, "8.9", "install") {
		t.Error("expected HasPreUpgradeScript(8.9, install) = false")
	}

	// Non-existent version should return false.
	if HasPreUpgradeScript(repoRoot, "99.99", "upgrade-patch") {
		t.Error("expected HasPreUpgradeScript(99.99, upgrade-patch) = false")
	}
}

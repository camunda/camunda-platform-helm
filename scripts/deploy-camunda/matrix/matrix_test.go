package matrix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Version comparison tests ---

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input     string
		wantMajor int
		wantMinor int
	}{
		{"8.6", 8, 6},
		{"8.9", 8, 9},
		{"10.0", 10, 0},
		{"0.1", 0, 1},
		{"8", 8, 0},
		{"", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			major, minor := parseVersion(tt.input)
			if major != tt.wantMajor || minor != tt.wantMinor {
				t.Errorf("parseVersion(%q) = (%d, %d), want (%d, %d)", tt.input, major, minor, tt.wantMajor, tt.wantMinor)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"8.6", "8.6", 0},
		{"8.7", "8.6", 1},
		{"8.6", "8.7", -1},
		{"8.9", "8.6", 1},
		{"9.0", "8.9", 1},
		{"8.9", "9.0", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMatchesVersion(t *testing.T) {
	tests := []struct {
		constraint string
		version    string
		want       bool
	}{
		// Exact match
		{"==8.9", "8.9", true},
		{"==8.9", "8.8", false},
		// Less than or equal
		{"<=8.7", "8.6", true},
		{"<=8.7", "8.7", true},
		{"<=8.7", "8.8", false},
		{"<=8.7", "8.9", false},
		// Greater than or equal
		{">=8.8", "8.8", true},
		{">=8.8", "8.9", true},
		{">=8.8", "8.7", false},
		// Strict less than
		{"<8.8", "8.7", true},
		{"<8.8", "8.8", false},
		// Strict greater than
		{">8.7", "8.8", true},
		{">8.7", "8.7", false},
		// No operator means ==
		{"8.9", "8.9", true},
		{"8.9", "8.8", false},
		// Empty constraint
		{"", "8.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.constraint+"_"+tt.version, func(t *testing.T) {
			got := matchesVersion(tt.constraint, tt.version)
			if got != tt.want {
				t.Errorf("matchesVersion(%q, %q) = %v, want %v", tt.constraint, tt.version, got, tt.want)
			}
		})
	}
}

// --- Flow filtering tests ---

func TestFilterFlows(t *testing.T) {
	pf := &PermittedFlows{
		Rules: []PermittedFlowRule{
			{Match: "<=8.7", Deny: []string{"upgrade-minor"}},
			{Match: "==8.9", Deny: []string{"upgrade-patch", "upgrade-minor"}},
		},
	}

	tests := []struct {
		name    string
		version string
		flows   []string
		want    []string
	}{
		{
			name:    "8.6 denies upgrade-minor",
			version: "8.6",
			flows:   []string{"install", "upgrade-patch", "upgrade-minor"},
			want:    []string{"install", "upgrade-patch"},
		},
		{
			name:    "8.7 denies upgrade-minor",
			version: "8.7",
			flows:   []string{"install", "upgrade-patch", "upgrade-minor"},
			want:    []string{"install", "upgrade-patch"},
		},
		{
			name:    "8.8 allows all flows",
			version: "8.8",
			flows:   []string{"install", "upgrade-patch", "upgrade-minor"},
			want:    []string{"install", "upgrade-patch", "upgrade-minor"},
		},
		{
			name:    "8.9 only allows install",
			version: "8.9",
			flows:   []string{"install", "upgrade-patch", "upgrade-minor"},
			want:    []string{"install"},
		},
		{
			name:    "8.9 with only install",
			version: "8.9",
			flows:   []string{"install"},
			want:    []string{"install"},
		},
		{
			name:    "8.9 with only denied flows returns empty",
			version: "8.9",
			flows:   []string{"upgrade-patch", "upgrade-minor"},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterFlows(pf, tt.version, tt.flows)
			if !sliceEqual(got, tt.want) {
				t.Errorf("FilterFlows(pf, %q, %v) = %v, want %v", tt.version, tt.flows, got, tt.want)
			}
		})
	}
}

// --- Matrix filter tests ---

func TestFilter(t *testing.T) {
	entries := []Entry{
		{Version: "8.8", Scenario: "elasticsearch", Shortname: "eske", Auth: "keycloak", Flow: "install", Platform: "gke", Enabled: true},
		{Version: "8.8", Scenario: "elasticsearch", Shortname: "eske", Auth: "keycloak", Flow: "install", Platform: "eks", Enabled: true},
		{Version: "8.8", Scenario: "elasticsearch", Shortname: "eshy", Auth: "hybrid", Flow: "install", Platform: "", Enabled: true},
		{Version: "8.8", Scenario: "oidc", Shortname: "esoi", Auth: "oidc", Flow: "upgrade-minor", Enabled: true},
		{Version: "8.9", Scenario: "elasticsearch", Shortname: "eske", Auth: "keycloak", Flow: "install", Platform: "gke", Enabled: true},
		{Version: "8.9", Scenario: "elasticsearch", Shortname: "eske", Auth: "keycloak", Flow: "install", Platform: "eks", Enabled: true},
	}

	t.Run("no filter returns all", func(t *testing.T) {
		got := Filter(entries, FilterOptions{})
		if len(got) != len(entries) {
			t.Errorf("Filter with no options: got %d entries, want %d", len(got), len(entries))
		}
	})

	t.Run("scenario filter", func(t *testing.T) {
		got := Filter(entries, FilterOptions{ScenarioFilter: "oidc"})
		if len(got) != 1 || got[0].Scenario != "oidc" {
			t.Errorf("Filter(scenario=oidc): got %d entries, want 1 oidc entry", len(got))
		}
	})

	t.Run("scenario filter multiple comma-separated", func(t *testing.T) {
		got := Filter(entries, FilterOptions{ScenarioFilter: "oidc,elasticsearch"})
		// Should match 1 oidc + 5 elasticsearch entries = 6 total
		if len(got) != 6 {
			t.Errorf("Filter(scenario=oidc,elasticsearch): got %d entries, want 6", len(got))
		}
		for _, e := range got {
			if e.Scenario != "oidc" && e.Scenario != "elasticsearch" {
				t.Errorf("Filter(scenario=oidc,elasticsearch): unexpected scenario %q", e.Scenario)
			}
		}
	})

	t.Run("scenario filter with spaces around commas", func(t *testing.T) {
		got := Filter(entries, FilterOptions{ScenarioFilter: " oidc , elasticsearch "})
		if len(got) != 6 {
			t.Errorf("Filter(scenario=' oidc , elasticsearch '): got %d entries, want 6", len(got))
		}
	})

	t.Run("flow filter", func(t *testing.T) {
		got := Filter(entries, FilterOptions{FlowFilter: "upgrade-minor"})
		if len(got) != 1 || got[0].Flow != "upgrade-minor" {
			t.Errorf("Filter(flow=upgrade-minor): got %d entries, want 1", len(got))
		}
	})

	t.Run("platform filter gke", func(t *testing.T) {
		got := Filter(entries, FilterOptions{Platform: "gke"})
		// Entries with platform="gke" match, entries with platform="" also match (no restriction)
		if len(got) != 4 {
			t.Errorf("Filter(platform=gke): got %d entries, want 4", len(got))
		}
	})

	t.Run("platform filter eks", func(t *testing.T) {
		got := Filter(entries, FilterOptions{Platform: "eks"})
		// Entries with platform="eks" match, entries with platform="" also match (no restriction)
		if len(got) != 4 {
			t.Errorf("Filter(platform=eks): got %d entries, want 4", len(got))
		}
	})

	t.Run("platform filter rosa", func(t *testing.T) {
		got := Filter(entries, FilterOptions{Platform: "rosa"})
		// Only entries with platform="" (no restriction) match
		if len(got) != 2 {
			t.Errorf("Filter(platform=rosa): got %d entries, want 2 (entries with no platform restriction)", len(got))
		}
	})
}

// --- GroupByVersion / VersionOrder tests ---

func TestGroupByVersionAndOrder(t *testing.T) {
	entries := []Entry{
		{Version: "8.9", Scenario: "es"},
		{Version: "8.8", Scenario: "es"},
		{Version: "8.8", Scenario: "oidc"},
		{Version: "8.9", Scenario: "os"},
	}

	groups := GroupByVersion(entries)
	if len(groups["8.8"]) != 2 {
		t.Errorf("GroupByVersion 8.8: got %d, want 2", len(groups["8.8"]))
	}
	if len(groups["8.9"]) != 2 {
		t.Errorf("GroupByVersion 8.9: got %d, want 2", len(groups["8.9"]))
	}

	order := VersionOrder(entries)
	if len(order) != 2 || order[0] != "8.9" || order[1] != "8.8" {
		t.Errorf("VersionOrder: got %v, want [8.9 8.8]", order)
	}
}

// --- Print tests ---

func TestPrintTable(t *testing.T) {
	entries := []Entry{
		{Version: "8.8", Scenario: "elasticsearch", Shortname: "eske", Auth: "keycloak", Flow: "install", Enabled: true},
	}
	output, err := Print(entries, "table")
	if err != nil {
		t.Fatalf("Print(table): %v", err)
	}
	if output == "" {
		t.Error("Print(table): empty output")
	}
	if !strings.Contains(output, "elasticsearch") || !strings.Contains(output, "8.8") || !strings.Contains(output, "Total: 1") {
		t.Errorf("Print(table): missing expected content in output: %s", output)
	}
}

func TestPrintJSON(t *testing.T) {
	entries := []Entry{
		{Version: "8.8", Scenario: "elasticsearch", Shortname: "eske", Auth: "keycloak", Flow: "install", Enabled: true},
	}
	output, err := Print(entries, "json")
	if err != nil {
		t.Fatalf("Print(json): %v", err)
	}
	if !strings.Contains(output, `"version": "8.8"`) || !strings.Contains(output, `"scenario": "elasticsearch"`) {
		t.Errorf("Print(json): missing expected JSON content in output: %s", output)
	}
}

func TestPrintInvalidFormat(t *testing.T) {
	_, err := Print(nil, "xml")
	if err == nil {
		t.Error("Print(xml): expected error for unknown format")
	}
}

func TestPrintTableEmpty(t *testing.T) {
	output, err := Print(nil, "table")
	if err != nil {
		t.Fatalf("Print(table, empty): %v", err)
	}
	if !strings.Contains(output, "No matrix entries found") {
		t.Errorf("Print(table, empty): expected 'No matrix entries found', got: %s", output)
	}
}

// --- Integration tests using real repo config files ---

func TestGenerateWithRealConfigs(t *testing.T) {
	repoRoot := findRepoRoot(t)

	entries, err := Generate(repoRoot, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("Generate: expected entries, got 0")
	}

	// Verify we have entries for multiple versions
	versions := VersionOrder(entries)
	if len(versions) < 2 {
		t.Errorf("Generate: expected entries for at least 2 versions, got %d: %v", len(versions), versions)
	}

	// Verify no denied flows leaked through
	for _, e := range entries {
		if e.Version == "8.9" && (e.Flow == "upgrade-patch" || e.Flow == "upgrade-minor") {
			t.Errorf("Generate: 8.9 entry has denied flow %q (scenario=%s)", e.Flow, e.Scenario)
		}
		if (e.Version == "8.6" || e.Version == "8.7") && e.Flow == "upgrade-minor" {
			t.Errorf("Generate: %s entry has denied flow upgrade-minor (scenario=%s)", e.Version, e.Scenario)
		}
	}

	// Verify all entries are enabled (default behavior)
	for _, e := range entries {
		if !e.Enabled {
			t.Errorf("Generate: entry %s/%s is disabled but should be filtered out by default", e.Version, e.Scenario)
		}
	}
}

func TestGenerateWithVersionFilter(t *testing.T) {
	repoRoot := findRepoRoot(t)

	entries, err := Generate(repoRoot, GenerateOptions{
		Versions: []string{"8.8"},
	})
	if err != nil {
		t.Fatalf("Generate(versions=8.8): %v", err)
	}

	for _, e := range entries {
		if e.Version != "8.8" {
			t.Errorf("Generate(versions=8.8): unexpected version %s", e.Version)
		}
	}
}

func TestGenerateWithIncludeDisabled(t *testing.T) {
	repoRoot := findRepoRoot(t)

	withDisabled, err := Generate(repoRoot, GenerateOptions{IncludeDisabled: true})
	if err != nil {
		t.Fatalf("Generate(includeDisabled): %v", err)
	}

	withoutDisabled, err := Generate(repoRoot, GenerateOptions{IncludeDisabled: false})
	if err != nil {
		t.Fatalf("Generate(no includeDisabled): %v", err)
	}

	if len(withDisabled) < len(withoutDisabled) {
		t.Errorf("Generate: withDisabled (%d) < withoutDisabled (%d); including disabled should never reduce entries",
			len(withDisabled), len(withoutDisabled))
	} else if len(withDisabled) == len(withoutDisabled) {
		t.Logf("Generate: withDisabled=%d == withoutDisabled=%d (all scenarios may be enabled)",
			len(withDisabled), len(withoutDisabled))
	}
}

func TestGenerateInvalidVersion(t *testing.T) {
	repoRoot := findRepoRoot(t)

	_, err := Generate(repoRoot, GenerateOptions{
		Versions: []string{"99.99"},
	})
	if err == nil {
		t.Error("Generate(versions=99.99): expected error for invalid version")
	}
}

// --- Config loader tests ---

func TestLoadChartVersions(t *testing.T) {
	repoRoot := findRepoRoot(t)

	cv, err := LoadChartVersions(repoRoot)
	if err != nil {
		t.Fatalf("LoadChartVersions: %v", err)
	}

	active := cv.ActiveVersions()
	if len(active) == 0 {
		t.Fatal("LoadChartVersions: no active versions")
	}

	// 8.9 should be alpha
	found := false
	for _, v := range cv.CamundaVersions.Alpha {
		if v == "8.9" {
			found = true
		}
	}
	if !found {
		t.Error("LoadChartVersions: 8.9 not found in alpha")
	}
}

func TestLoadPermittedFlows(t *testing.T) {
	repoRoot := findRepoRoot(t)

	pf, err := LoadPermittedFlows(repoRoot)
	if err != nil {
		t.Fatalf("LoadPermittedFlows: %v", err)
	}

	if len(pf.Defaults.Flows) == 0 {
		t.Error("LoadPermittedFlows: no default flows")
	}
	if len(pf.Rules) == 0 {
		t.Error("LoadPermittedFlows: no rules")
	}
}

func TestLoadCITestConfig(t *testing.T) {
	repoRoot := findRepoRoot(t)

	chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-8.8")
	cfg, err := LoadCITestConfig(chartDir)
	if err != nil {
		t.Fatalf("LoadCITestConfig: %v", err)
	}

	if len(cfg.Integration.Case.PR.Scenarios) == 0 {
		t.Error("LoadCITestConfig: no PR scenarios")
	}
}

// --- RunSummary test ---

func TestPrintRunSummary(t *testing.T) {
	results := []RunResult{
		{Entry: Entry{Version: "8.8", Scenario: "es", Shortname: "eske", Flow: "install"}, Namespace: "matrix-88-eske"},
		{Entry: Entry{Version: "8.8", Scenario: "oidc", Shortname: "esoi", Flow: "install"}, Namespace: "matrix-88-esoi", Error: os.ErrNotExist},
	}

	summary := PrintRunSummary(results)
	if !strings.Contains(summary, "Total:   2") || !strings.Contains(summary, "Success: 1") || !strings.Contains(summary, "Failed:  1") {
		t.Errorf("PrintRunSummary: unexpected output: %s", summary)
	}
	if !strings.Contains(summary, "oidc") {
		t.Errorf("PrintRunSummary: expected failed entry in output: %s", summary)
	}
}

func TestPrintRunSummaryEmpty(t *testing.T) {
	summary := PrintRunSummary(nil)
	if !strings.Contains(summary, "No entries executed") {
		t.Errorf("PrintRunSummary(nil): expected 'No entries executed', got: %s", summary)
	}
}

// --- buildNamespace tests ---

func TestBuildNamespace(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		entry  Entry
		want   string
	}{
		{
			name:   "install flow without platform",
			prefix: "matrix",
			entry:  Entry{Version: "8.8", Shortname: "eske", Flow: "install"},
			want:   "matrix-88-eske-inst",
		},
		{
			name:   "install flow with gke platform",
			prefix: "matrix",
			entry:  Entry{Version: "8.8", Shortname: "eske", Flow: "install", Platform: "gke"},
			want:   "matrix-88-eske-inst-gke",
		},
		{
			name:   "install flow with eks platform",
			prefix: "matrix",
			entry:  Entry{Version: "8.8", Shortname: "eske", Flow: "install", Platform: "eks"},
			want:   "matrix-88-eske-inst-eks",
		},
		{
			name:   "upgrade-patch flow with gke platform",
			prefix: "matrix",
			entry:  Entry{Version: "8.7", Shortname: "es", Flow: "upgrade-patch", Platform: "gke"},
			want:   "matrix-87-es-upgp-gke",
		},
		{
			name:   "upgrade-minor flow without platform",
			prefix: "matrix",
			entry:  Entry{Version: "8.8", Shortname: "kemt", Flow: "upgrade-minor"},
			want:   "matrix-88-kemt-upgm",
		},
		{
			name:   "alpha version with platform",
			prefix: "matrix",
			entry:  Entry{Version: "8.9", Shortname: "oske", Flow: "install", Platform: "gke"},
			want:   "matrix-89-oske-inst-gke",
		},
		{
			name:   "custom prefix",
			prefix: "ci",
			entry:  Entry{Version: "8.6", Shortname: "kemt", Flow: "install"},
			want:   "ci-86-kemt-inst",
		},
		{
			name:   "falls back to scenario when shortname empty",
			prefix: "matrix",
			entry:  Entry{Version: "8.8", Scenario: "elasticsearch", Shortname: "", Flow: "install"},
			want:   "matrix-88-elasticsearch-inst",
		},
		{
			name:   "empty flow defaults to inst",
			prefix: "matrix",
			entry:  Entry{Version: "8.8", Shortname: "eske"},
			want:   "matrix-88-eske-inst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildNamespace(tt.prefix, tt.entry)
			if got != tt.want {
				t.Errorf("buildNamespace(%q, entry) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

// --- flowAbbrev tests ---

func TestFlowAbbrev(t *testing.T) {
	tests := []struct {
		flow string
		want string
	}{
		{"install", "inst"},
		{"upgrade-patch", "upgp"},
		{"upgrade-minor", "upgm"},
		{"", "inst"},
		{"custom-flow-long", "cust"},
		{"abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			got := flowAbbrev(tt.flow)
			if got != tt.want {
				t.Errorf("flowAbbrev(%q) = %q, want %q", tt.flow, got, tt.want)
			}
		})
	}
}

// --- ingressSubdomain tests ---

func TestIngressSubdomain(t *testing.T) {
	tests := []struct {
		name       string
		baseDomain string
		namespace  string
		want       string
	}{
		{
			name:       "returns namespace when baseDomain is set",
			baseDomain: "ci.distro.ultrawombat.com",
			namespace:  "matrix-89-eske",
			want:       "matrix-89-eske",
		},
		{
			name:       "returns empty when baseDomain is empty",
			baseDomain: "",
			namespace:  "matrix-89-eske",
			want:       "",
		},
		{
			name:       "works with custom namespace",
			baseDomain: "distribution.aws.camunda.cloud",
			namespace:  "ci-88-oske",
			want:       "ci-88-oske",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ingressSubdomain(tt.baseDomain, tt.namespace)
			if got != tt.want {
				t.Errorf("ingressSubdomain(%q, %q) = %q, want %q", tt.baseDomain, tt.namespace, got, tt.want)
			}
		})
	}
}

// --- resolveKubeContext tests ---

func TestResolveKubeContext(t *testing.T) {
	tests := []struct {
		name     string
		opts     RunOptions
		platform string
		want     string
	}{
		{
			name:     "returns platform-specific context for gke",
			opts:     RunOptions{KubeContexts: map[string]string{"gke": "gke-ctx", "eks": "eks-ctx"}},
			platform: "gke",
			want:     "gke-ctx",
		},
		{
			name:     "returns platform-specific context for eks",
			opts:     RunOptions{KubeContexts: map[string]string{"gke": "gke-ctx", "eks": "eks-ctx"}},
			platform: "eks",
			want:     "eks-ctx",
		},
		{
			name:     "falls back to KubeContext when platform not in map",
			opts:     RunOptions{KubeContexts: map[string]string{"gke": "gke-ctx"}, KubeContext: "fallback-ctx"},
			platform: "eks",
			want:     "fallback-ctx",
		},
		{
			name:     "falls back to KubeContext when map is nil",
			opts:     RunOptions{KubeContext: "fallback-ctx"},
			platform: "gke",
			want:     "fallback-ctx",
		},
		{
			name:     "returns empty when nothing configured",
			opts:     RunOptions{},
			platform: "gke",
			want:     "",
		},
		{
			name:     "platform-specific takes priority over fallback",
			opts:     RunOptions{KubeContexts: map[string]string{"gke": "gke-ctx"}, KubeContext: "fallback-ctx"},
			platform: "gke",
			want:     "gke-ctx",
		},
		{
			name:     "skips empty string in map and falls back",
			opts:     RunOptions{KubeContexts: map[string]string{"gke": ""}, KubeContext: "fallback-ctx"},
			platform: "gke",
			want:     "fallback-ctx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveKubeContext(tt.opts, tt.platform)
			if got != tt.want {
				t.Errorf("resolveKubeContext(opts, %q) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}

// --- resolvePlatform tests ---

func TestResolvePlatform(t *testing.T) {
	tests := []struct {
		name  string
		opts  RunOptions
		entry Entry
		want  string
	}{
		{
			name:  "opts.Platform overrides entry",
			opts:  RunOptions{Platform: "eks"},
			entry: Entry{Platform: "gke"},
			want:  "eks",
		},
		{
			name:  "uses entry platform when no override",
			opts:  RunOptions{},
			entry: Entry{Platform: "eks"},
			want:  "eks",
		},
		{
			name:  "defaults to gke when no platform set",
			opts:  RunOptions{},
			entry: Entry{},
			want:  "gke",
		},
		{
			name:  "defaults to gke when entry platform empty",
			opts:  RunOptions{},
			entry: Entry{Platform: ""},
			want:  "gke",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePlatform(tt.opts, tt.entry)
			if got != tt.want {
				t.Errorf("resolvePlatform(opts, entry) = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- resolveEnvFile tests ---

func TestResolveEnvFile(t *testing.T) {
	tests := []struct {
		name    string
		opts    RunOptions
		version string
		want    string
	}{
		{
			name:    "version-specific file takes priority",
			opts:    RunOptions{EnvFiles: map[string]string{"8.9": ".env.89"}, EnvFile: ".env"},
			version: "8.9",
			want:    ".env.89",
		},
		{
			name:    "falls back to EnvFile when version not in map",
			opts:    RunOptions{EnvFiles: map[string]string{"8.9": ".env.89"}, EnvFile: ".env"},
			version: "8.8",
			want:    ".env",
		},
		{
			name:    "falls back to EnvFile when map is nil",
			opts:    RunOptions{EnvFile: ".env.default"},
			version: "8.7",
			want:    ".env.default",
		},
		{
			name:    "returns empty when nothing configured",
			opts:    RunOptions{},
			version: "8.6",
			want:    "",
		},
		{
			name:    "skips empty string in version map",
			opts:    RunOptions{EnvFiles: map[string]string{"8.8": ""}, EnvFile: ".env.fallback"},
			version: "8.8",
			want:    ".env.fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEnvFile(tt.opts, tt.version)
			if got != tt.want {
				t.Errorf("resolveEnvFile(opts, %q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

// --- resolveUseVaultBackedSecrets tests ---

func TestResolveUseVaultBackedSecrets(t *testing.T) {
	tests := []struct {
		name     string
		opts     RunOptions
		platform string
		want     bool
	}{
		{
			name:     "returns platform-specific value for eks (true)",
			opts:     RunOptions{VaultBackedSecrets: map[string]bool{"eks": true, "gke": false}},
			platform: "eks",
			want:     true,
		},
		{
			name:     "returns platform-specific value for gke (false)",
			opts:     RunOptions{VaultBackedSecrets: map[string]bool{"eks": true, "gke": false}},
			platform: "gke",
			want:     false,
		},
		{
			name:     "falls back to UseVaultBackedSecrets when platform not in map",
			opts:     RunOptions{VaultBackedSecrets: map[string]bool{"gke": false}, UseVaultBackedSecrets: true},
			platform: "eks",
			want:     true,
		},
		{
			name:     "falls back to UseVaultBackedSecrets when map is nil",
			opts:     RunOptions{UseVaultBackedSecrets: true},
			platform: "gke",
			want:     true,
		},
		{
			name:     "returns false when nothing configured",
			opts:     RunOptions{},
			platform: "gke",
			want:     false,
		},
		{
			name:     "platform-specific false overrides fallback true",
			opts:     RunOptions{VaultBackedSecrets: map[string]bool{"gke": false}, UseVaultBackedSecrets: true},
			platform: "gke",
			want:     false,
		},
		{
			name:     "platform-specific true overrides fallback false",
			opts:     RunOptions{VaultBackedSecrets: map[string]bool{"eks": true}, UseVaultBackedSecrets: false},
			platform: "eks",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveUseVaultBackedSecrets(tt.opts, tt.platform)
			if got != tt.want {
				t.Errorf("resolveUseVaultBackedSecrets(opts, %q) = %v, want %v", tt.platform, got, tt.want)
			}
		})
	}
}

// --- resolveIngressBaseDomain tests ---

func TestResolveIngressBaseDomain(t *testing.T) {
	tests := []struct {
		name     string
		opts     RunOptions
		platform string
		want     string
	}{
		{
			name:     "returns platform-specific domain for gke",
			opts:     RunOptions{IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com", "eks": "distribution.aws.camunda.cloud"}},
			platform: "gke",
			want:     "ci.distro.ultrawombat.com",
		},
		{
			name:     "returns platform-specific domain for eks",
			opts:     RunOptions{IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com", "eks": "distribution.aws.camunda.cloud"}},
			platform: "eks",
			want:     "distribution.aws.camunda.cloud",
		},
		{
			name:     "falls back to IngressBaseDomain when platform not in map",
			opts:     RunOptions{IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"}, IngressBaseDomain: "fallback.example.com"},
			platform: "eks",
			want:     "fallback.example.com",
		},
		{
			name:     "falls back to IngressBaseDomain when map is nil",
			opts:     RunOptions{IngressBaseDomain: "fallback.example.com"},
			platform: "gke",
			want:     "fallback.example.com",
		},
		{
			name:     "returns empty when nothing configured",
			opts:     RunOptions{},
			platform: "gke",
			want:     "",
		},
		{
			name:     "platform-specific takes priority over fallback",
			opts:     RunOptions{IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"}, IngressBaseDomain: "fallback.example.com"},
			platform: "gke",
			want:     "ci.distro.ultrawombat.com",
		},
		{
			name:     "skips empty string in map and falls back",
			opts:     RunOptions{IngressBaseDomains: map[string]string{"gke": ""}, IngressBaseDomain: "fallback.example.com"},
			platform: "gke",
			want:     "fallback.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveIngressBaseDomain(tt.opts, tt.platform)
			if got != tt.want {
				t.Errorf("resolveIngressBaseDomain(opts, %q) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}

// --- helpers ---

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// findRepoRoot walks up from the current working directory to find the repo root.
func findRepoRoot(t *testing.T) string {
	t.Helper()

	// Try working directory first
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	// Walk up until we find charts/chart-versions.yaml
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "charts", "chart-versions.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Skip("cannot find repo root (charts/chart-versions.yaml); skipping integration test")
	return ""
}

package config

import (
	"strings"
	"testing"
)

func TestParseScenarios(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single scenario",
			input: "keycloak",
			want:  []string{"keycloak"},
		},
		{
			name:  "comma-separated scenarios",
			input: "keycloak,elasticsearch,opensearch",
			want:  []string{"keycloak", "elasticsearch", "opensearch"},
		},
		{
			name:  "scenarios with whitespace",
			input: " keycloak , elasticsearch , opensearch ",
			want:  []string{"keycloak", "elasticsearch", "opensearch"},
		},
		{
			name:  "empty entries are skipped",
			input: "keycloak,,elasticsearch,,",
			want:  []string{"keycloak", "elasticsearch"},
		},
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "only commas returns nil",
			input: ",,,",
			want:  nil,
		},
		{
			name:  "scenario with hyphens",
			input: "keycloak-mt,gateway-keycloak,qa-elasticsearch",
			want:  []string{"keycloak-mt", "gateway-keycloak", "qa-elasticsearch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseScenarios(tt.input)
			if !strSlicesEqual(got, tt.want) {
				t.Errorf("parseScenarios(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid minimal flags", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath: "/some/chart/path",
			Namespace: "test-ns",
			Release:   "test-release",
			Scenario:  "keycloak",
		}
		if err := Validate(flags); err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
		if len(flags.Scenarios) != 1 || flags.Scenarios[0] != "keycloak" {
			t.Errorf("Validate() Scenarios = %v, want [keycloak]", flags.Scenarios)
		}
	})

	t.Run("valid with chart instead of chart-path", func(t *testing.T) {
		flags := &RuntimeFlags{
			Chart:     "oci://some-registry/chart",
			Namespace: "test-ns",
			Release:   "test-release",
			Scenario:  "keycloak",
		}
		if err := Validate(flags); err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("error when neither chart nor chart-path", func(t *testing.T) {
		flags := &RuntimeFlags{
			Namespace: "test-ns",
			Release:   "test-release",
			Scenario:  "keycloak",
		}
		err := Validate(flags)
		if err == nil {
			t.Fatal("Validate() expected error for missing chart/chart-path")
		}
		if !strings.Contains(err.Error(), "chart-path") {
			t.Errorf("Validate() error = %q, want mention of chart-path", err.Error())
		}
	})

	t.Run("error when namespace missing", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath: "/some/path",
			Release:   "test-release",
			Scenario:  "keycloak",
		}
		err := Validate(flags)
		if err == nil {
			t.Fatal("Validate() expected error for missing namespace")
		}
		if !strings.Contains(err.Error(), "namespace") {
			t.Errorf("Validate() error = %q, want mention of namespace", err.Error())
		}
	})

	t.Run("error when release missing", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath: "/some/path",
			Namespace: "test-ns",
			Scenario:  "keycloak",
		}
		err := Validate(flags)
		if err == nil {
			t.Fatal("Validate() expected error for missing release")
		}
		if !strings.Contains(err.Error(), "release") {
			t.Errorf("Validate() error = %q, want mention of release", err.Error())
		}
	})

	t.Run("error when scenario missing", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath: "/some/path",
			Namespace: "test-ns",
			Release:   "test-release",
		}
		err := Validate(flags)
		if err == nil {
			t.Fatal("Validate() expected error for missing scenario")
		}
		if !strings.Contains(err.Error(), "scenario") {
			t.Errorf("Validate() error = %q, want mention of scenario", err.Error())
		}
	})

	t.Run("multiple scenarios parsed correctly", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath: "/some/path",
			Namespace: "test-ns",
			Release:   "test-release",
			Scenario:  "keycloak,elasticsearch,opensearch",
		}
		if err := Validate(flags); err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
		want := []string{"keycloak", "elasticsearch", "opensearch"}
		if !strSlicesEqual(flags.Scenarios, want) {
			t.Errorf("Validate() Scenarios = %v, want %v", flags.Scenarios, want)
		}
	})

	t.Run("version requires chart flag", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath:    "/some/path",
			ChartVersion: "1.0.0",
			Namespace:    "test-ns",
			Release:      "test-release",
			Scenario:     "keycloak",
		}
		err := Validate(flags)
		if err == nil {
			t.Fatal("Validate() expected error for --version without --chart")
		}
		if !strings.Contains(err.Error(), "version") {
			t.Errorf("Validate() error = %q, want mention of version", err.Error())
		}
	})

	t.Run("ingress-hostname and ingress-subdomain are mutually exclusive", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath:        "/some/path",
			Namespace:        "test-ns",
			Release:          "test-release",
			Scenario:         "keycloak",
			IngressHostname:  "custom.example.com",
			IngressSubdomain: "sub",
		}
		err := Validate(flags)
		if err == nil {
			t.Fatal("Validate() expected error for conflicting ingress flags")
		}
		if !strings.Contains(err.Error(), "ingress-hostname") {
			t.Errorf("Validate() error = %q, want mention of ingress-hostname", err.Error())
		}
	})

	t.Run("ingress-subdomain requires ingress-base-domain", func(t *testing.T) {
		flags := &RuntimeFlags{
			ChartPath:        "/some/path",
			Namespace:        "test-ns",
			Release:          "test-release",
			Scenario:         "keycloak",
			IngressSubdomain: "sub",
		}
		err := Validate(flags)
		if err == nil {
			t.Fatal("Validate() expected error for subdomain without base domain")
		}
		if !strings.Contains(err.Error(), "ingress-base-domain") {
			t.Errorf("Validate() error = %q, want mention of ingress-base-domain", err.Error())
		}
	})
}

func TestResolveIngressHostname(t *testing.T) {
	tests := []struct {
		name              string
		ingressHostname   string
		ingressSubdomain  string
		ingressBaseDomain string
		want              string
	}{
		{
			name:            "explicit hostname takes precedence",
			ingressHostname: "custom.example.com",
			want:            "custom.example.com",
		},
		{
			name:              "subdomain + base domain composed",
			ingressSubdomain:  "my-app",
			ingressBaseDomain: "ci.distro.ultrawombat.com",
			want:              "my-app.ci.distro.ultrawombat.com",
		},
		{
			name:              "hostname overrides subdomain+base",
			ingressHostname:   "override.example.com",
			ingressSubdomain:  "ignored",
			ingressBaseDomain: "ignored.com",
			want:              "override.example.com",
		},
		{
			name: "empty returns empty",
			want: "",
		},
		{
			name:             "subdomain without base returns empty",
			ingressSubdomain: "orphan",
			want:             "",
		},
		{
			name:              "base without subdomain returns empty",
			ingressBaseDomain: "ci.distro.ultrawombat.com",
			want:              "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &RuntimeFlags{
				IngressHostname:   tt.ingressHostname,
				IngressSubdomain:  tt.ingressSubdomain,
				IngressBaseDomain: tt.ingressBaseDomain,
			}
			got := f.ResolveIngressHostname()
			if got != tt.want {
				t.Errorf("ResolveIngressHostname() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectiveNamespace(t *testing.T) {
	tests := []struct {
		name            string
		namespace       string
		namespacePrefix string
		want            string
	}{
		{
			name:      "no prefix",
			namespace: "my-namespace",
			want:      "my-namespace",
		},
		{
			name:            "with prefix",
			namespace:       "my-namespace",
			namespacePrefix: "distribution",
			want:            "distribution-my-namespace",
		},
		{
			name:            "empty namespace with prefix",
			namespace:       "",
			namespacePrefix: "distribution",
			want:            "",
		},
		{
			name:      "empty prefix",
			namespace: "my-namespace",
			want:      "my-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &RuntimeFlags{
				Namespace:       tt.namespace,
				NamespacePrefix: tt.namespacePrefix,
			}
			got := f.EffectiveNamespace()
			if got != tt.want {
				t.Errorf("EffectiveNamespace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDebugFlag(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		defaultPort int
		wantComp    string
		wantPort    int
		wantErr     bool
	}{
		{
			name:        "component without port",
			value:       "orchestration",
			defaultPort: 5005,
			wantComp:    "orchestration",
			wantPort:    5005,
		},
		{
			name:        "component with port",
			value:       "orchestration:9999",
			defaultPort: 5005,
			wantComp:    "orchestration",
			wantPort:    9999,
		},
		{
			name:        "connectors without port",
			value:       "connectors",
			defaultPort: 5005,
			wantComp:    "connectors",
			wantPort:    5005,
		},
		{
			name:        "uppercase normalized to lowercase",
			value:       "ORCHESTRATION",
			defaultPort: 5005,
			wantComp:    "orchestration",
			wantPort:    5005,
		},
		{
			name:        "whitespace trimmed",
			value:       "  orchestration : 9999  ",
			defaultPort: 5005,
			wantComp:    "orchestration",
			wantPort:    9999,
		},
		{
			name:        "empty component",
			value:       "",
			defaultPort: 5005,
			wantErr:     true,
		},
		{
			name:        "invalid port",
			value:       "orchestration:abc",
			defaultPort: 5005,
			wantErr:     true,
		},
		{
			name:        "port out of range",
			value:       "orchestration:99999",
			defaultPort: 5005,
			wantErr:     true,
		},
		{
			name:        "port zero",
			value:       "orchestration:0",
			defaultPort: 5005,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, port, err := ParseDebugFlag(tt.value, tt.defaultPort)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDebugFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if comp != tt.wantComp {
				t.Errorf("ParseDebugFlag() component = %q, want %q", comp, tt.wantComp)
			}
			if port != tt.wantPort {
				t.Errorf("ParseDebugFlag() port = %d, want %d", port, tt.wantPort)
			}
		})
	}
}

func TestApplyActiveDeployment(t *testing.T) {
	t.Run("nil config is no-op", func(t *testing.T) {
		flags := &RuntimeFlags{}
		if err := ApplyActiveDeployment(nil, "", flags); err != nil {
			t.Fatalf("ApplyActiveDeployment() unexpected error: %v", err)
		}
	})

	t.Run("applies root defaults", func(t *testing.T) {
		rc := &RootConfig{
			ChartPath: "/root/chart",
			Namespace: "root-ns",
			Platform:  "gke",
			Auth:      "keycloak",
		}
		flags := &RuntimeFlags{}
		if err := ApplyActiveDeployment(rc, "", flags); err != nil {
			t.Fatalf("ApplyActiveDeployment() unexpected error: %v", err)
		}
		if flags.ChartPath != "/root/chart" {
			t.Errorf("ChartPath = %q, want /root/chart", flags.ChartPath)
		}
		if flags.Namespace != "root-ns" {
			t.Errorf("Namespace = %q, want root-ns", flags.Namespace)
		}
		if flags.Platform != "gke" {
			t.Errorf("Platform = %q, want gke", flags.Platform)
		}
		if flags.Auth != "keycloak" {
			t.Errorf("Auth = %q, want keycloak", flags.Auth)
		}
	})

	t.Run("deployment overrides root for ScenarioPath", func(t *testing.T) {
		rc := &RootConfig{
			ScenarioPath: "/root/scenarios",
			Deployments: map[string]DeploymentConfig{
				"dev": {
					ScenarioPath: "/dev/scenarios",
				},
			},
		}
		flags := &RuntimeFlags{}
		if err := ApplyActiveDeployment(rc, "dev", flags); err != nil {
			t.Fatalf("ApplyActiveDeployment() unexpected error: %v", err)
		}
		if flags.ScenarioPath != "/dev/scenarios" {
			t.Errorf("ScenarioPath = %q, want /dev/scenarios", flags.ScenarioPath)
		}
	})

	t.Run("falls back to ScenarioRoot when ScenarioPath empty", func(t *testing.T) {
		rc := &RootConfig{
			ScenarioRoot: "/root/scenario-root",
			Deployments: map[string]DeploymentConfig{
				"dev": {
					ScenarioRoot: "/dev/scenario-root",
				},
			},
		}
		flags := &RuntimeFlags{}
		if err := ApplyActiveDeployment(rc, "dev", flags); err != nil {
			t.Fatalf("ApplyActiveDeployment() unexpected error: %v", err)
		}
		if flags.ScenarioPath != "/dev/scenario-root" {
			t.Errorf("ScenarioPath = %q, want /dev/scenario-root", flags.ScenarioPath)
		}
	})

	t.Run("CLI flag takes precedence over config", func(t *testing.T) {
		rc := &RootConfig{
			ScenarioPath: "/root/scenarios",
			Deployments: map[string]DeploymentConfig{
				"dev": {
					ScenarioPath: "/dev/scenarios",
				},
			},
		}
		flags := &RuntimeFlags{
			ScenarioPath: "/cli/scenarios",
		}
		if err := ApplyActiveDeployment(rc, "dev", flags); err != nil {
			t.Fatalf("ApplyActiveDeployment() unexpected error: %v", err)
		}
		// CLI flag should NOT be overwritten
		if flags.ScenarioPath != "/cli/scenarios" {
			t.Errorf("ScenarioPath = %q, want /cli/scenarios (CLI flag should take precedence)", flags.ScenarioPath)
		}
	})

	t.Run("auto-selects single deployment", func(t *testing.T) {
		rc := &RootConfig{
			Deployments: map[string]DeploymentConfig{
				"only-one": {
					ChartPath: "/auto/chart",
					Namespace: "auto-ns",
				},
			},
		}
		flags := &RuntimeFlags{}
		if err := ApplyActiveDeployment(rc, "", flags); err != nil {
			t.Fatalf("ApplyActiveDeployment() unexpected error: %v", err)
		}
		if flags.ChartPath != "/auto/chart" {
			t.Errorf("ChartPath = %q, want /auto/chart (auto-selected single deployment)", flags.ChartPath)
		}
	})

	t.Run("error for nonexistent deployment", func(t *testing.T) {
		rc := &RootConfig{
			Deployments: map[string]DeploymentConfig{
				"dev": {},
			},
		}
		flags := &RuntimeFlags{}
		err := ApplyActiveDeployment(rc, "nonexistent", flags)
		if err == nil {
			t.Fatal("ApplyActiveDeployment() expected error for nonexistent deployment")
		}
		if !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("error = %q, want mention of 'nonexistent'", err.Error())
		}
	})
}

func TestMergeStringField(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		depVal  string
		rootVal string
		want    string
	}{
		{
			name:    "empty target gets dep value",
			target:  "",
			depVal:  "dep",
			rootVal: "root",
			want:    "dep",
		},
		{
			name:    "empty target falls back to root",
			target:  "",
			depVal:  "",
			rootVal: "root",
			want:    "root",
		},
		{
			name:    "non-empty target preserved",
			target:  "original",
			depVal:  "dep",
			rootVal: "root",
			want:    "original",
		},
		{
			name:    "all empty stays empty",
			target:  "",
			depVal:  "",
			rootVal: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			MergeStringField(&target, tt.depVal, tt.rootVal)
			if target != tt.want {
				t.Errorf("MergeStringField() target = %q, want %q", target, tt.want)
			}
		})
	}
}

func TestMergeBoolField(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	t.Run("nil pointers leave target unchanged", func(t *testing.T) {
		target := false
		MergeBoolField(&target, nil, nil, "", nil)
		if target != false {
			t.Errorf("target = %v, want false", target)
		}
	})

	t.Run("dep value applied", func(t *testing.T) {
		target := false
		MergeBoolField(&target, boolPtr(true), nil, "", nil)
		if target != true {
			t.Errorf("target = %v, want true", target)
		}
	})

	t.Run("root value applied when dep nil", func(t *testing.T) {
		target := false
		MergeBoolField(&target, nil, boolPtr(true), "", nil)
		if target != true {
			t.Errorf("target = %v, want true", target)
		}
	})

	t.Run("CLI flag takes precedence over config when changed", func(t *testing.T) {
		target := false // user passed --skip-dependency-update=false
		changed := map[string]bool{"skip-dependency-update": true}
		MergeBoolField(&target, boolPtr(true), boolPtr(true), "skip-dependency-update", changed)
		if target != false {
			t.Errorf("target = %v, want false (CLI should take precedence)", target)
		}
	})

	t.Run("config applies when flag not changed", func(t *testing.T) {
		target := true               // cobra default
		changed := map[string]bool{} // user did NOT pass the flag
		MergeBoolField(&target, boolPtr(false), nil, "skip-dependency-update", changed)
		if target != false {
			t.Errorf("target = %v, want false (config should apply)", target)
		}
	})

	t.Run("nil changedFlags treats all as not changed", func(t *testing.T) {
		target := true
		MergeBoolField(&target, boolPtr(false), nil, "skip-dependency-update", nil)
		if target != false {
			t.Errorf("target = %v, want false (nil changedFlags = no CLI override)", target)
		}
	})
}

func TestMergeStringSliceField(t *testing.T) {
	t.Run("empty target gets dep value", func(t *testing.T) {
		var target []string
		MergeStringSliceField(&target, []string{"a", "b"}, nil)
		if len(target) != 2 || target[0] != "a" || target[1] != "b" {
			t.Errorf("target = %v, want [a b]", target)
		}
	})

	t.Run("empty target falls back to root", func(t *testing.T) {
		var target []string
		MergeStringSliceField(&target, nil, []string{"x"})
		if len(target) != 1 || target[0] != "x" {
			t.Errorf("target = %v, want [x]", target)
		}
	})

	t.Run("non-empty target preserved", func(t *testing.T) {
		target := []string{"original"}
		MergeStringSliceField(&target, []string{"dep"}, []string{"root"})
		if len(target) != 1 || target[0] != "original" {
			t.Errorf("target = %v, want [original]", target)
		}
	})
}

func TestIsValidIngressBaseDomain(t *testing.T) {
	// Ensure known valid domains are accepted
	for _, d := range ValidIngressBaseDomains {
		if !isValidIngressBaseDomain(d) {
			t.Errorf("isValidIngressBaseDomain(%q) = false, want true", d)
		}
	}

	// Ensure unknown domains are rejected
	invalid := []string{"example.com", "localhost", "invalid.domain", ""}
	for _, d := range invalid {
		if isValidIngressBaseDomain(d) {
			t.Errorf("isValidIngressBaseDomain(%q) = true, want false", d)
		}
	}
}

// strSlicesEqual compares two string slices.
func strSlicesEqual(a, b []string) bool {
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

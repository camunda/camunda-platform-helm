package cmd

import (
	"strings"
	"testing"

	"scripts/deploy-camunda/matrix"
)

func TestValidateChartRefFlags(t *testing.T) {
	tests := []struct {
		name            string
		chartRef        string
		chartRefVersion string
		wantErr         string // substring; empty = expect success
	}{
		{
			name:    "both empty is allowed",
			wantErr: "",
		},
		{
			name:            "version without ref is rejected",
			chartRefVersion: "13-rc-latest",
			wantErr:         "--chart-version requires --chart-ref",
		},
		{
			name:            "OCI ref with version is allowed",
			chartRef:        "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			chartRefVersion: "13-rc-latest",
		},
		{
			name:     "OCI ref without version is rejected",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			wantErr:  "--chart-version is required when --chart-ref is an OCI reference",
		},
		{
			name:     "tgz ref without version is allowed",
			chartRef: "/tmp/camunda-platform-13.4.0-rc.tgz",
		},
		{
			name:            "tgz ref with version is allowed",
			chartRef:        "/tmp/camunda-platform-13.4.0-rc.tgz",
			chartRefVersion: "13.4.0-rc",
		},
		{
			name:     "directory ref is rejected",
			chartRef: "/tmp/camunda-platform-8.9",
			wantErr:  "must be an OCI reference (oci://...) or a packaged chart (.tgz)",
		},
		{
			name:     "bare chart name is rejected",
			chartRef: "camunda/camunda-platform",
			wantErr:  "must be an OCI reference (oci://...) or a packaged chart (.tgz)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChartRefFlags(tt.chartRef, tt.chartRefVersion)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateChartRefVersionSpan(t *testing.T) {
	entriesFor := func(versions ...string) []matrix.Entry {
		out := make([]matrix.Entry, 0, len(versions))
		for _, v := range versions {
			out = append(out, matrix.Entry{Version: v})
		}
		return out
	}

	tests := []struct {
		name     string
		chartRef string
		entries  []matrix.Entry
		wantErr  string // substring; empty = expect success
	}{
		{
			name:    "no chart-ref allows any version span",
			entries: entriesFor("8.8", "8.9", "8.10"),
		},
		{
			name:     "single version single entry is allowed",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  entriesFor("8.10"),
		},
		{
			name:     "single version multiple entries is allowed",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  entriesFor("8.10", "8.10", "8.10"),
		},
		{
			name:     "multiple versions are rejected",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  entriesFor("8.8", "8.9"),
			wantErr:  "spans 2 versions (8.8, 8.9)",
		},
		{
			name:     "empty entries with chart-ref does not panic",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChartRefVersionSpan(tt.chartRef, tt.entries)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

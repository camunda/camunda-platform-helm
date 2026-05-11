package cmd

import (
	"errors"
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

func TestAggregateRunError(t *testing.T) {
	preExisting := errors.New("stopping on failure: helm failed")
	entryErr := errors.New("helm upgrade --install failed: exit status 1")

	tests := []struct {
		name    string
		runErr  error
		results []matrix.RunResult
		wantErr string // substring; empty = expect nil
	}{
		{
			name:    "no results no error",
			results: nil,
			wantErr: "",
		},
		{
			name: "all entries succeeded",
			results: []matrix.RunResult{
				{Entry: matrix.Entry{Version: "8.9"}, Namespace: "ns-a"},
				{Entry: matrix.Entry{Version: "8.9"}, Namespace: "ns-b"},
			},
			wantErr: "",
		},
		{
			name: "single entry failed",
			results: []matrix.RunResult{
				{Entry: matrix.Entry{Version: "8.9"}, Namespace: "ns-a", Error: entryErr},
			},
			wantErr: "1 matrix entry failed",
		},
		{
			name: "multiple entries failed",
			results: []matrix.RunResult{
				{Entry: matrix.Entry{Version: "8.8"}, Namespace: "ns-a", Error: entryErr},
				{Entry: matrix.Entry{Version: "8.9"}, Namespace: "ns-b", Error: entryErr},
				{Entry: matrix.Entry{Version: "8.9"}, Namespace: "ns-c"},
			},
			wantErr: "2 matrix entries failed",
		},
		{
			name:   "preserves pre-existing run error",
			runErr: preExisting,
			results: []matrix.RunResult{
				{Entry: matrix.Entry{Version: "8.9"}, Namespace: "ns-a", Error: entryErr},
			},
			wantErr: "stopping on failure: helm failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := aggregateRunError(tc.runErr, tc.results)
			if tc.wantErr == "" {
				if got != nil {
					t.Fatalf("expected nil error, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(got.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, got.Error())
			}
		})
	}
}

package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestParseRunRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		repo      string
		wantOwner string
		wantRepo  string
		wantRun   string
		wantJob   string
		wantErr   bool
	}{
		{
			name:      "full job url",
			input:     "https://github.com/camunda/camunda-platform-helm/actions/runs/28577691601/job/84730355206",
			wantOwner: "camunda",
			wantRepo:  "camunda-platform-helm",
			wantRun:   "28577691601",
			wantJob:   "84730355206",
		},
		{
			name:      "run url without job",
			input:     "https://github.com/camunda/camunda-platform-helm/actions/runs/28577691601",
			wantOwner: "camunda",
			wantRepo:  "camunda-platform-helm",
			wantRun:   "28577691601",
		},
		{
			name:      "bare run id falls back to default repo",
			input:     "28577691601",
			wantOwner: "camunda",
			wantRepo:  "camunda-platform-helm",
			wantRun:   "28577691601",
		},
		{
			name:      "repo override wins over url",
			input:     "https://github.com/someone/fork/actions/runs/42",
			repo:      "camunda/camunda-platform-helm",
			wantOwner: "camunda",
			wantRepo:  "camunda-platform-helm",
			wantRun:   "42",
		},
		{
			name:    "garbage input errors",
			input:   "not-a-run",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := parseRunRef(tc.input, tc.repo)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got ref %+v", ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.Owner != tc.wantOwner || ref.Repo != tc.wantRepo || ref.RunID != tc.wantRun || ref.JobID != tc.wantJob {
				t.Errorf("got %+v, want owner=%s repo=%s run=%s job=%s",
					ref, tc.wantOwner, tc.wantRepo, tc.wantRun, tc.wantJob)
			}
		})
	}
}

func TestParseMatrixCell(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want matrixCell
	}{
		{
			name: "full cell",
			in:   "8.9 - cprst - install - pr - gke",
			want: matrixCell{Version: "8.9", Shortname: "cprst", Flow: "install", Case: "pr", Platform: "gke"},
		},
		{
			name: "8.10 double-digit minor",
			in:   "8.10 - keyco - upgrade-minor - pr - gke",
			want: matrixCell{Version: "8.10", Shortname: "keyco", Flow: "upgrade-minor", Case: "pr", Platform: "gke"},
		},
		{
			name: "real job name with trailing prose keeps only the platform token",
			in:   "8.9 - cprst - install - pr - gke / gke - ITs / cprst - install - gke / install for install on gke - cprst",
			want: matrixCell{Version: "8.9", Shortname: "cprst", Flow: "install", Case: "pr", Platform: "gke"},
		},
		{
			name: "non-version leading token is not a cell",
			in:   "install for install on gke - cprst",
			want: matrixCell{},
		},
		{
			name: "too few parts",
			in:   "CI Gate",
			want: matrixCell{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMatrixCell(tc.in)
			if got != tc.want {
				t.Errorf("parseMatrixCell(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

// sampleLog mirrors the shape of a real failing cprst install job log: GitHub
// timestamp prefixes, an env-var dump block, the helm command, the matrix-failed
// reason, the diagnostics path, and a PodInitializing signal.
const sampleLog = "2026-07-02T08:55:05.7514311Z   POSTGRESQL_JDBC_URL: jdbc:postgresql://host:5432\n" +
	"2026-07-02T08:55:05.7527586Z   DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET: ***\n" +
	"2026-07-02T09:15:24.7142716Z \x1b[90m09:15:24\x1b[0m WARN \x1b[1m[cmd:helm] Error: context deadline exceeded\x1b[0m\n" +
	"2026-07-02T09:15:34.8612245Z \x1b[90m09:15:34\x1b[0m ERROR \x1b[1mMatrix entry failed\x1b[0m error=\"helm upgrade --install failed: exit status 1\" command=\"helm upgrade --install integration camunda-platform-8.9 -n camunda-id--intg-8-9-gke-cprst-aa43b0 --create-namespace --wait=legacy --timeout 1200s -f values-digest.yaml\" flow=install scenario=component-persistence version=8.9\n" +
	"2026-07-02T09:15:34.8629904Z     Diagnostics: diagnostics/camunda-id--intg-8-9-gke-cprst-aa43b0/20260702T091524Z\n" +
	"2026-07-02T09:15:37.6651835Z Error from server (BadRequest): container \"migration\" in pod \"integration-optimize-58bb88d6-q8q2p\" is waiting to start: PodInitializing\n"

func TestExtractFailureRecord(t *testing.T) {
	t.Parallel()
	rec := extractFailureRecord(sampleLog)

	if !strings.HasPrefix(rec.HelmCommand, "helm upgrade --install integration camunda-platform-8.9") {
		t.Errorf("HelmCommand not extracted cleanly: %q", rec.HelmCommand)
	}
	if strings.Contains(rec.HelmCommand, "failed: exit status") {
		t.Errorf("HelmCommand should be the real invocation, not the error message: %q", rec.HelmCommand)
	}
	if strings.Contains(rec.HelmCommand, "2026-07-02T") || strings.Contains(rec.HelmCommand, "\x1b[") {
		t.Errorf("HelmCommand should have timestamp and ANSI stripped: %q", rec.HelmCommand)
	}
	if strings.Contains(rec.Reason, "\x1b[") {
		t.Errorf("Reason should have ANSI stripped: %q", rec.Reason)
	}
	if rec.DiagnosticsDir != "diagnostics/camunda-id--intg-8-9-gke-cprst-aa43b0/20260702T091524Z" {
		t.Errorf("DiagnosticsDir = %q", rec.DiagnosticsDir)
	}
	if !strings.Contains(rec.Reason, "Matrix entry failed") {
		t.Errorf("Reason = %q, want it to mention 'Matrix entry failed'", rec.Reason)
	}
	wantSignals := map[string]bool{"context deadline exceeded": true, "PodInitializing": true, "waiting to start": true}
	for _, s := range rec.Signals {
		delete(wantSignals, s)
	}
	if len(wantSignals) != 0 {
		t.Errorf("missing signals %v in %v", wantSignals, rec.Signals)
	}
}

func TestStripANSILeavesNonANSIIntact(t *testing.T) {
	t.Parallel()
	// Real ANSI (with ESC) must be stripped; bare "[Nm" text must NOT be touched.
	cases := map[string]string{
		"\x1b[90m09:15:24\x1b[0m WARN":            "09:15:24 WARN",
		"waiting: [1m PodInitializing":            "waiting: [1m PodInitializing",
		"context deadline exceeded after [20m] x": "context deadline exceeded after [20m] x",
		"##[error]something failed":               "##[error]something failed",
	}
	for in, want := range cases {
		if got := stripANSI(in); got != want {
			t.Errorf("stripANSI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripLogNoise(t *testing.T) {
	t.Parallel()
	out := stripLogNoise(sampleLog)

	if strings.Contains(out, "POSTGRESQL_JDBC_URL:") || strings.Contains(out, "DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET:") {
		t.Errorf("env-var dump lines should be removed:\n%s", out)
	}
	if strings.Contains(out, "2026-07-02T09:15:34") {
		t.Errorf("timestamp prefixes should be stripped:\n%s", out)
	}
	if !strings.Contains(out, "Matrix entry failed") {
		t.Errorf("meaningful lines should be preserved:\n%s", out)
	}
	if !strings.Contains(out, "Diagnostics: diagnostics/") {
		t.Errorf("diagnostics line should be preserved:\n%s", out)
	}
}

func TestSelectFailedJob(t *testing.T) {
	t.Parallel()
	jobs := []ghJob{
		{ID: 1, Name: "CI Gate", Conclusion: "success"},
		{ID: 2, Name: "Generate chart matrix", Conclusion: "failure"},
		{ID: 3, Name: "8.9 - cprst - install - pr - gke", Conclusion: "failure"},
	}
	got := selectFailedJob(jobs)
	if got == nil || got.ID != 3 {
		t.Fatalf("expected the matrix-cell job (id 3) to be selected, got %+v", got)
	}

	// When no failed job looks like a matrix cell, fall back to the first failure.
	got = selectFailedJob([]ghJob{
		{ID: 1, Name: "CI Gate", Conclusion: "success"},
		{ID: 2, Name: "Generate chart matrix", Conclusion: "failure"},
	})
	if got == nil || got.ID != 2 {
		t.Fatalf("expected first failed job (id 2), got %+v", got)
	}

	// All green -> nil.
	if got := selectFailedJob([]ghJob{{ID: 1, Conclusion: "success"}}); got != nil {
		t.Fatalf("expected nil for all-green jobs, got %+v", got)
	}

	// timed_out is treated as a failure; cancelled is not.
	got = selectFailedJob([]ghJob{
		{ID: 1, Name: "CI Gate", Conclusion: "cancelled"},
		{ID: 2, Name: "8.10 - keyco - install - pr - gke", Conclusion: "timed_out"},
	})
	if got == nil || got.ID != 2 {
		t.Fatalf("expected the timed_out matrix job (id 2), got %+v", got)
	}
	if got := selectFailedJob([]ghJob{{ID: 1, Conclusion: "cancelled"}}); got != nil {
		t.Fatalf("cancelled-only jobs should not be selected, got %+v", got)
	}
}

func TestFirstFailedStep(t *testing.T) {
	t.Parallel()
	job := ghJob{Steps: []struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	}{
		{Name: "Setup", Conclusion: "success"},
		{Name: "Install Camunda chart", Conclusion: "timed_out"},
		{Name: "Later", Conclusion: "failure"},
	}}
	// timed_out must count as failed (matches selectFailedJob's failedConclusions).
	if got := firstFailedStep(job); got != "Install Camunda chart" {
		t.Errorf("firstFailedStep = %q, want the timed_out step", got)
	}
	if got := firstFailedStep(ghJob{}); got != "" {
		t.Errorf("no steps should yield empty, got %q", got)
	}
}

func TestFetchFailingJobLog(t *testing.T) {
	ref := runRef{Owner: "camunda", Repo: "camunda-platform-helm", RunID: "42"}

	t.Run("selects failed job then fetches its log", func(t *testing.T) {
		orig := ghAPI
		defer func() { ghAPI = orig }()
		var logJobID string
		ghAPI = func(_ context.Context, args []string) ([]byte, error) {
			arg := args[len(args)-1]
			if strings.Contains(arg, "/runs/42/jobs") {
				return []byte(`{"total_count":2,"jobs":[
					{"id":1,"name":"CI Gate","conclusion":"success"},
					{"id":2,"name":"8.9 - cprst - install - pr - gke","conclusion":"failure"}]}`), nil
			}
			if strings.Contains(arg, "/jobs/2/logs") {
				logJobID = "2"
				return []byte("some log body\n"), nil
			}
			return nil, fmt.Errorf("unexpected api call: %s", arg)
		}
		log, job, err := fetchFailingJobLog(context.Background(), ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logJobID != "2" || !strings.Contains(log, "some log body") {
			t.Errorf("expected log for job 2, got job=%q log=%q", logJobID, log)
		}
		if job == nil || job.ID != 2 {
			t.Errorf("expected selected job 2, got %+v", job)
		}
	})

	t.Run("all-green run returns the guidance error", func(t *testing.T) {
		orig := ghAPI
		defer func() { ghAPI = orig }()
		ghAPI = func(_ context.Context, _ []string) ([]byte, error) {
			return []byte(`{"total_count":1,"jobs":[{"id":1,"name":"CI Gate","conclusion":"success"}]}`), nil
		}
		_, _, err := fetchFailingJobLog(context.Background(), ref)
		if err == nil || !strings.Contains(err.Error(), "no failed job found") {
			t.Fatalf("expected no-failed-job error, got %v", err)
		}
	})

	t.Run("malformed jobs response errors", func(t *testing.T) {
		orig := ghAPI
		defer func() { ghAPI = orig }()
		ghAPI = func(_ context.Context, _ []string) ([]byte, error) {
			return []byte("not json"), nil
		}
		_, _, err := fetchFailingJobLog(context.Background(), ref)
		if err == nil || !strings.Contains(err.Error(), "parse jobs response") {
			t.Fatalf("expected parse error, got %v", err)
		}
	})

	t.Run("pinned job id bypasses the jobs list", func(t *testing.T) {
		orig := ghAPI
		defer func() { ghAPI = orig }()
		pinned := ref
		pinned.JobID = "999"
		var calls []string
		ghAPI = func(_ context.Context, args []string) ([]byte, error) {
			arg := args[len(args)-1]
			calls = append(calls, arg)
			if strings.Contains(arg, "/jobs/999/logs") {
				return []byte("pinned log\n"), nil
			}
			if strings.Contains(arg, "/jobs/999") {
				return []byte(`{"id":999,"name":"8.9 - cprst - install - pr - gke","conclusion":"failure"}`), nil
			}
			return nil, fmt.Errorf("unexpected api call: %s", arg)
		}
		log, job, err := fetchFailingJobLog(context.Background(), pinned)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(log, "pinned log") {
			t.Errorf("expected pinned log, got %q", log)
		}
		if job == nil || job.ID != 999 {
			t.Errorf("expected pinned job metadata, got %+v", job)
		}
		for _, c := range calls {
			if strings.Contains(c, "/runs/42/jobs") {
				t.Errorf("pinned path must not list run jobs, but called %s", c)
			}
		}
	})
}

func TestReproCommand(t *testing.T) {
	t.Parallel()
	cmd := reproCommand(matrixCell{Version: "8.9", Shortname: "cprst", Flow: "install", Case: "pr", Platform: "gke"})
	for _, want := range []string{"--versions 8.9", "--shortname-filter cprst", "--shortname-exact", "--flow-filter install", "--platform gke"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("repro command missing %q: %s", want, cmd)
		}
	}
	if reproCommand(matrixCell{}) != "" {
		t.Error("empty cell should yield no repro command")
	}
	// A 'pr' platform token should be normalized to gke.
	if got := reproCommand(matrixCell{Version: "8.10", Shortname: "keyco", Flow: "install", Platform: "pr"}); !strings.Contains(got, "--platform gke") {
		t.Errorf("pr platform should normalize to gke: %s", got)
	}
}

func TestExtractHelmCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		log  string
		want string
	}{
		{
			name: "quoted command field wins",
			log:  `x ERROR failed command="helm upgrade --install integration -n ns" version=8.9`,
			want: "helm upgrade --install integration -n ns",
		},
		{
			name: "shell comment mention is not a command",
			log:  "2026-07-06T10:11:23Z \x1b[36;1m# but the upgrade step must use the current chart so helm upgrade\x1b[0m",
			want: "",
		},
		{
			name: "bare mention with no args is not a command",
			log:  "we then run helm upgrade to roll the release",
			want: "",
		},
		{
			name: "real invocation on its own line is the fallback",
			log:  "2026-07-06T10:11:23Z helm upgrade --install integration camunda-platform-8.10 -n test",
			want: "helm upgrade --install integration camunda-platform-8.10 -n test",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractHelmCommand(stripANSI(tc.log))
			if got != tc.want {
				t.Errorf("extractHelmCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractFailureRecordEmptyLog(t *testing.T) {
	t.Parallel()
	rec := extractFailureRecord("nothing interesting here\n")
	if rec.HelmCommand != "" || rec.DiagnosticsDir != "" || rec.Reason != "" || len(rec.Signals) != 0 {
		t.Errorf("expected empty record for a boring log, got %+v", rec)
	}
}

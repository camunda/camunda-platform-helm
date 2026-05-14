package agentwatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nowSuffix() string { return time.Now().UTC().Format("20060102T150405") }

func TestParseVerdict_StrictJSON(t *testing.T) {
	raw := []byte(`{
		"diagnosis": "secret missing",
		"causal_chain": ["t+12s FailedMount"],
		"confidence": 0.92,
		"recommended_action": "abort",
		"evidence": ["pod=keycloak-0"]
	}`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.RecommendedAction != ActionAbort {
		t.Fatalf("expected abort, got %q", v.RecommendedAction)
	}
	if v.Confidence != 0.92 {
		t.Fatalf("expected confidence 0.92, got %v", v.Confidence)
	}
	if len(v.CausalChain) != 1 {
		t.Fatalf("expected 1 causal chain entry, got %d", len(v.CausalChain))
	}
}

func TestParseVerdict_ProseWrapped(t *testing.T) {
	// Some agent CLIs prepend explanatory prose. We should still recover
	// the embedded JSON.
	raw := []byte(`Here's the verdict you asked for:

	{
	  "diagnosis": "image tag missing",
	  "causal_chain": ["t+0s helm.installed", "t+30s ImagePullBackOff"],
	  "confidence": 0.88,
	  "recommended_action": "abort"
	}

	Hope this helps!`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.RecommendedAction != ActionAbort {
		t.Fatalf("expected abort, got %q", v.RecommendedAction)
	}
}

func TestParseVerdict_ClaudeCodeEnvelope(t *testing.T) {
	// Claude Code -p --output-format json wraps the assistant text in
	// {"type":"result","result":"..."}.
	inner := `{"diagnosis":"OOM","causal_chain":["t+25s OOMKilled"],"confidence":0.9,"recommended_action":"abort"}`
	raw := []byte(`{"type":"result","result":` + jsonString(inner) + `}`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Diagnosis != "OOM" {
		t.Fatalf("expected diagnosis OOM, got %q", v.Diagnosis)
	}
}

func TestParseVerdict_RejectsInvalidAction(t *testing.T) {
	raw := []byte(`{"diagnosis":"x","confidence":0.5,"recommended_action":"panic"}`)
	if _, err := ParseVerdict(raw); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestParseVerdict_OpencodeNDJSON(t *testing.T) {
	// opencode run --format json emits newline-delimited JSON events.
	// The verdict text is in {"type":"text","part":{"text":"..."}} events.
	raw := []byte(`{"type":"step_start","timestamp":1778579787017,"sessionID":"ses_abc","part":{"id":"prt_1","messageID":"msg_1","sessionID":"ses_abc","type":"step-start"}}
{"type":"text","timestamp":1778579787124,"sessionID":"ses_abc","part":{"id":"prt_2","messageID":"msg_1","sessionID":"ses_abc","type":"text","text":"` + "```json\\n{\\\"diagnosis\\\":\\\"pod OOMKilled\\\",\\\"causal_chain\\\":[\\\"memory limit 256Mi\\\"],\\\"confidence\\\":0.88,\\\"recommended_action\\\":\\\"abort\\\"}\\n```" + `","time":{"start":1778579787018,"end":1778579787122}}}
{"type":"step_finish","timestamp":1778579787192,"sessionID":"ses_abc","part":{"id":"prt_3","reason":"stop","messageID":"msg_1","sessionID":"ses_abc","type":"step-finish"}}`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Diagnosis != "pod OOMKilled" {
		t.Fatalf("expected diagnosis 'pod OOMKilled', got %q", v.Diagnosis)
	}
	if v.Confidence != 0.88 {
		t.Fatalf("expected confidence 0.88, got %v", v.Confidence)
	}
	if v.RecommendedAction != ActionAbort {
		t.Fatalf("expected abort, got %q", v.RecommendedAction)
	}
}

func TestParseVerdict_OpencodeNDJSONNoFence(t *testing.T) {
	// opencode sometimes returns the JSON without code fences.
	raw := []byte(`{"type":"step_start","timestamp":1,"sessionID":"s","part":{"type":"step-start"}}
{"type":"text","timestamp":2,"sessionID":"s","part":{"type":"text","text":"{\"diagnosis\":\"all healthy\",\"causal_chain\":[],\"confidence\":0.95,\"recommended_action\":\"wait\"}"}}
{"type":"step_finish","timestamp":3,"sessionID":"s","part":{"type":"step-finish"}}`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.RecommendedAction != ActionWait {
		t.Fatalf("expected wait, got %q", v.RecommendedAction)
	}
}

func TestParseVerdict_RejectsConfidenceOutOfRange(t *testing.T) {
	raw := []byte(`{"diagnosis":"x","confidence":1.5,"recommended_action":"wait"}`)
	if _, err := ParseVerdict(raw); err == nil {
		t.Fatal("expected error for confidence > 1")
	}
}

func TestParseVerdict_RejectsEmptyDiagnosis(t *testing.T) {
	raw := []byte(`{"diagnosis":"   ","confidence":0.5,"recommended_action":"wait"}`)
	if _, err := ParseVerdict(raw); err == nil {
		t.Fatal("expected error for blank diagnosis")
	}
}

func TestParseVerdict_NoJSON(t *testing.T) {
	raw := []byte("the model failed to respond")
	if _, err := ParseVerdict(raw); err == nil {
		t.Fatal("expected error when no JSON object is present")
	}
}

func TestParseVerdict_TrailingProseWithBraces(t *testing.T) {
	// Regression for the greedy-regex bug: a literal '}' in trailing prose
	// used to make the match run to the very last '}' in the input,
	// producing an unparseable substring on otherwise valid output.
	raw := []byte(`Here you go:
	{"diagnosis":"shard OOM","causal_chain":[],"confidence":0.92,"recommended_action":"investigate"}
	Note: example of bad output below for contrast: { "diagnosis": ""`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.RecommendedAction != ActionInvestigate {
		t.Fatalf("expected investigate, got %q", v.RecommendedAction)
	}
}

func TestParseVerdict_RecoversAfterUnterminatedOpener(t *testing.T) {
	// Regression for the brace-walker continuation bug: an unmatched '{'
	// before the real verdict used to make the walker give up entirely.
	raw := []byte(`Here's the JSON I'll return: { (oops, malformed
	{"diagnosis":"image not found","causal_chain":[],"confidence":0.95,"recommended_action":"abort"}`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.RecommendedAction != ActionAbort {
		t.Fatalf("expected abort, got %q", v.RecommendedAction)
	}
}

func TestRedactForCorpus_StripsSensitiveEnvValues(t *testing.T) {
	in := []byte(`{
	  "pods": {"items": [{"spec": {"containers": [{
	    "name": "runner",
	    "env": [
	      {"name": "DB_PASSWORD", "value": "hunter2"},
	      {"name": "HARBOR_TOKEN", "value": "abc.def.ghi"},
	      {"name": "PUBLIC_URL", "value": "https://camunda.example"},
	      {"name": "OIDC_CLIENT_SECRET", "valueFrom": {"secretKeyRef": {"name": "x", "key": "y"}}}
	    ]
	  }]}}]}
	}`)
	out := RedactForCorpus(in)
	s := string(out)
	if strings.Contains(s, "hunter2") {
		t.Fatal("DB_PASSWORD plaintext leaked into corpus")
	}
	if strings.Contains(s, "abc.def.ghi") {
		t.Fatal("HARBOR_TOKEN plaintext leaked into corpus")
	}
	if !strings.Contains(s, "https://camunda.example") {
		t.Fatal("non-sensitive PUBLIC_URL was incorrectly redacted")
	}
	if !strings.Contains(s, "DB_PASSWORD") {
		t.Fatal("env name was redacted; should remain visible")
	}
	// valueFrom references don't carry plaintext, so we leave them alone
	if !strings.Contains(s, "secretKeyRef") {
		t.Fatal("valueFrom reference was unexpectedly stripped")
	}
}

func TestRedactForCorpus_StripsBearerTokensFromLogs(t *testing.T) {
	in := []byte(`{"events":{"items":[{"message":"GET /x: Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.signature"}]}}`)
	out := string(RedactForCorpus(in))
	if strings.Contains(out, "eyJhbGciOiJIUzI1NiJ9") {
		t.Fatal("JWT-shaped token leaked into corpus")
	}
}

func TestParseVerdict_BracesInsideStringFields(t *testing.T) {
	raw := []byte(`prefix {"diagnosis":"got } here in the message","causal_chain":[],"confidence":0.5,"recommended_action":"wait"} trailing`)
	v, err := ParseVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Diagnosis != "got } here in the message" {
		t.Fatalf("string-literal handling broken; diagnosis=%q", v.Diagnosis)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name            string
		action          Action
		confidence      float64
		abortConfidence float64
		want            Decision
	}{
		{"wait", ActionWait, 0.99, 0.85, DecisionContinue},
		{"investigate", ActionInvestigate, 0.99, 0.85, DecisionSurface},
		{"abort high confidence", ActionAbort, 0.9, 0.85, DecisionAbort},
		{"abort low confidence", ActionAbort, 0.6, 0.85, DecisionSurface},
		{"abort threshold disabled", ActionAbort, 0.99, 0, DecisionSurface},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Verdict{RecommendedAction: tc.action, Confidence: tc.confidence, Diagnosis: "x"}
			if got := classify(v, tc.abortConfidence); got != tc.want {
				t.Fatalf("classify(%+v, %v) = %v, want %v", v, tc.abortConfidence, got, tc.want)
			}
		})
	}
}

func TestSupportedCLIs_StableOrder(t *testing.T) {
	got := SupportedCLIs()
	if len(got) < 2 || got[0] != "opencode" || got[1] != "claude" {
		t.Fatalf("expected [opencode claude ...], got %v", got)
	}
	// Ensure callers can't mutate the package's internal slice.
	got[0] = "tampered"
	if SupportedCLIs()[0] != "opencode" {
		t.Fatal("SupportedCLIs returns shared slice; callers can mutate package state")
	}
}

func TestBuildArgs_Claude(t *testing.T) {
	args := buildArgs(AgentCLI{Name: "claude", Path: "/usr/bin/claude"}, "be helpful")
	if !contains(args, "-p") || !contains(args, "be helpful") || !contains(args, "json") {
		t.Fatalf("unexpected claude args: %v", args)
	}
}

func TestBuildArgs_Opencode(t *testing.T) {
	args := buildArgs(AgentCLI{Name: "opencode", Path: "/usr/bin/opencode"}, "be helpful")
	if !contains(args, "run") || !contains(args, "be helpful") {
		t.Fatalf("unexpected opencode args: %v", args)
	}
}

func TestResolveCLI_EmptyFallsBackToDetect(t *testing.T) {
	// When name is empty, ResolveCLI should behave like DetectCLI.
	cli, err := ResolveCLI("")
	if err != nil {
		t.Skipf("no agent CLI on PATH (expected in CI): %v", err)
	}
	if cli.Name == "" || cli.Path == "" {
		t.Fatal("ResolveCLI returned empty CLI for empty name")
	}
}

func TestResolveCLI_NonExistentErrors(t *testing.T) {
	_, err := ResolveCLI("nonexistent-agent-cli-xyz")
	if err == nil {
		t.Fatal("expected error for non-existent CLI name")
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestWatch_SnapshotFailureRespectsMaxTicks(t *testing.T) {
	// Regression: snapshot-fail path used to skip tick++, so MaxTicks
	// could not bound the loop when kubectl was permanently broken.
	// We simulate "always-broken kubectl" by pointing GatherSnapshot at a
	// nonexistent namespace with no kubeconfig; it returns quickly with
	// an error. With MaxTicks=2 and a sub-second interval the loop must
	// exit within a small wall-clock budget.
	opts := Options{
		CLI: AgentCLI{Name: "claude", Path: "/nonexistent-but-required-by-validation"},
		Snapshot: SnapshotOptions{
			Namespace:      "definitely-does-not-exist-" + nowSuffix(),
			SkipHelmStatus: true,
			KubeContext:    "definitely-does-not-exist-context",
		},
		Interval: 10 * time.Millisecond,
		MaxTicks: 2,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	decision, verdict, err := Watch(ctx, opts)
	if err != nil {
		t.Fatalf("expected nil error after MaxTicks exit, got %v", err)
	}
	if decision != DecisionContinue {
		t.Fatalf("expected DecisionContinue after MaxTicks exit, got %v", decision)
	}
	if verdict != nil {
		t.Fatalf("expected nil verdict (no successful tick), got %+v", verdict)
	}
}

func TestReplayReport_FormatLabelsRegressions(t *testing.T) {
	wait := Verdict{Diagnosis: "x", Confidence: 0.9, RecommendedAction: ActionWait}
	abort := Verdict{Diagnosis: "x", Confidence: 0.9, RecommendedAction: ActionAbort}
	report := ReplayReport{
		CorpusDir: "/tmp/c",
		Results: []ReplayResult{
			{File: "/tmp/c/ok.json", Recorded: &wait, Replayed: &wait, ActionChanged: false},
			{File: "/tmp/c/bad.json", Recorded: &abort, Replayed: &wait, ActionChanged: true, ConfidenceDelta: 0.0},
		},
		Regressions: 1,
	}
	out := report.Format()
	if !strings.Contains(out, "REGRESS") {
		t.Fatalf("expected REGRESS marker in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Regressions: 1") {
		t.Fatalf("expected Regressions: 1 in output, got:\n%s", out)
	}
}

func TestSnapshot_AllPodsReady(t *testing.T) {
	cases := []struct {
		name string
		pods string
		want bool
	}{
		{
			name: "empty pods field",
			pods: "",
			want: false,
		},
		{
			name: "no items",
			pods: `{"items":[]}`,
			want: false,
		},
		{
			name: "single running ready",
			pods: `{"items":[{"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}]}`,
			want: true,
		},
		{
			name: "running but not ready",
			pods: `{"items":[{"status":{"phase":"Running","conditions":[{"type":"Ready","status":"False"}]}}]}`,
			want: false,
		},
		{
			name: "running with no Ready condition",
			pods: `{"items":[{"status":{"phase":"Running","conditions":[{"type":"Initialized","status":"True"}]}}]}`,
			want: false,
		},
		{
			name: "pending pod blocks readiness",
			pods: `{"items":[
				{"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}},
				{"status":{"phase":"Pending"}}
			]}`,
			want: false,
		},
		{
			name: "succeeded job pod counts as ready",
			pods: `{"items":[
				{"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}},
				{"status":{"phase":"Succeeded"}}
			]}`,
			want: true,
		},
		{
			name: "malformed JSON returns false",
			pods: `not json at all`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := Snapshot{Pods: json.RawMessage(tc.pods)}
			if got := snap.AllPodsReady(); got != tc.want {
				t.Fatalf("AllPodsReady = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWatch_MaxDurationStopsLoop(t *testing.T) {
	// With a sub-second wall-clock cap and no real cluster, snapshot
	// gathering will fail repeatedly. The MaxDuration timeout must close
	// the loop and surface context.DeadlineExceeded.
	opts := Options{
		CLI: AgentCLI{Name: "claude", Path: "/nonexistent-but-required-by-validation"},
		Snapshot: SnapshotOptions{
			Namespace:      "definitely-does-not-exist-" + nowSuffix(),
			SkipHelmStatus: true,
			KubeContext:    "definitely-does-not-exist-context",
		},
		Interval:    10 * time.Millisecond,
		MaxTicks:    -1, // disable tick bound to isolate duration check
		MaxDuration: 200 * time.Millisecond,
		MaxErrors:   -1, // disable error bound too
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, _, err := Watch(ctx, opts)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("Watch should have stopped near MaxDuration; elapsed=%s", elapsed)
	}
}

func TestWatch_MaxErrorsStopsRetryStorm(t *testing.T) {
	opts := Options{
		CLI: AgentCLI{Name: "claude", Path: "/nonexistent-but-required-by-validation"},
		Snapshot: SnapshotOptions{
			Namespace:      "definitely-does-not-exist-" + nowSuffix(),
			SkipHelmStatus: true,
			KubeContext:    "definitely-does-not-exist-context",
		},
		Interval:  10 * time.Millisecond,
		MaxTicks:  -1,
		MaxErrors: 2,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := Watch(ctx, opts)
	if err == nil || !strings.Contains(err.Error(), "consecutive errors") {
		t.Fatalf("expected consecutive-errors abort, got %v", err)
	}
}

func TestCleanupClaudeSession_RemovesMatchingTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectDir := filepath.Join(home, ".claude", "projects", "-tmp-fake-cwd")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	keep := filepath.Join(projectDir, "00000000-0000-0000-0000-000000000000.jsonl")
	target := filepath.Join(projectDir, "deadbeef-1234-5678-9abc-fedcba987654.jsonl")
	if err := os.WriteFile(keep, []byte(`{"keep":true}`), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(target, []byte(`{"target":true}`), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	envelope := []byte(`{"type":"result","session_id":"deadbeef-1234-5678-9abc-fedcba987654","result":"..."}`)
	cleanupClaudeSession(envelope)

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target transcript should have been removed; stat err=%v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unrelated transcript should remain; got err=%v", err)
	}
}

func TestCleanupClaudeSession_NoSessionIDIsSilent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cleanupClaudeSession([]byte(`{"type":"result","result":"no session id"}`))
	cleanupClaudeSession([]byte(`not even json`))
}

// jsonString is a tiny helper to embed an arbitrary string as a JSON string
// literal in test fixtures.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

package agentwatch

import (
	"strings"
	"testing"
)

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
	if len(got) < 2 || got[0] != "claude" || got[1] != "opencode" {
		t.Fatalf("expected [claude opencode ...], got %v", got)
	}
	// Ensure callers can't mutate the package's internal slice.
	got[0] = "tampered"
	if SupportedCLIs()[0] != "claude" {
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

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
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

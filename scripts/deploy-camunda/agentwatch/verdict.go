package agentwatch

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Action is the agent's recommendation for what deploy-camunda should do
// after reading the verdict.
type Action string

const (
	// ActionWait means the install is progressing normally; keep polling.
	ActionWait Action = "wait"
	// ActionInvestigate means something looks off but the agent is not
	// confident enough to recommend abort. Surface the diagnosis to the
	// user but keep polling.
	ActionInvestigate Action = "investigate"
	// ActionAbort means the install is broken and unlikely to recover.
	// Whether deploy-camunda actually aborts depends on confidence and
	// the user's --abort-on-confidence flag.
	ActionAbort Action = "abort"
)

// Verdict is the structured response the agent skill is asked to produce on
// every poll tick. Field names match the schema documented in the
// debug-failing-pods skill.
type Verdict struct {
	Diagnosis         string   `json:"diagnosis"`
	CausalChain       []string `json:"causal_chain"`
	Confidence        float64  `json:"confidence"`
	RecommendedAction Action   `json:"recommended_action"`
	Evidence          []string `json:"evidence,omitempty"`
}

// Valid reports whether the verdict has the minimum fields required for
// deploy-camunda to act on it. A verdict is considered valid even if Evidence
// is empty, but Diagnosis and RecommendedAction must be populated.
func (v Verdict) Valid() error {
	if strings.TrimSpace(v.Diagnosis) == "" {
		return fmt.Errorf("verdict missing diagnosis")
	}
	switch v.RecommendedAction {
	case ActionWait, ActionInvestigate, ActionAbort:
	default:
		return fmt.Errorf("verdict has unknown recommended_action %q "+
			"(want wait|investigate|abort)", v.RecommendedAction)
	}
	if v.Confidence < 0 || v.Confidence > 1 {
		return fmt.Errorf("verdict confidence %v out of range [0,1]", v.Confidence)
	}
	return nil
}

// jsonObjectPattern finds the first balanced JSON object in a string. Agent
// CLIs sometimes wrap the structured output with prose ("Here is the
// verdict: { ... }") or fence it in a markdown code block; this lets us
// recover the embedded JSON without insisting the CLI return a pristine
// stdout. The pattern is intentionally simple (greedy outer braces); for
// nested structures the JSON decoder gives the authoritative answer.
var jsonObjectPattern = regexp.MustCompile(`(?s)\{.*\}`)

// ParseVerdict extracts a Verdict from raw agent CLI output. It first tries
// strict JSON decoding; if that fails it falls back to extracting the first
// balanced JSON object substring and decoding that. This keeps the parser
// robust to small wrapping differences between Claude Code and opencode.
func ParseVerdict(raw []byte) (Verdict, error) {
	var v Verdict
	if err := json.Unmarshal(raw, &v); err == nil {
		if validateErr := v.Valid(); validateErr == nil {
			return v, nil
		}
		// Fall through: the top-level decode succeeded but the verdict
		// itself was malformed. The substring search below may find a
		// nested object that is what we actually want.
	}

	// Some CLI envelopes wrap the model's output as e.g.
	// {"role":"assistant","content":"<verdict-json>"}. Try unwrapping
	// known shapes before giving up.
	if unwrapped, ok := unwrapEnvelope(raw); ok {
		if err := json.Unmarshal(unwrapped, &v); err == nil {
			if validateErr := v.Valid(); validateErr == nil {
				return v, nil
			}
		}
		raw = unwrapped
	}

	match := jsonObjectPattern.Find(raw)
	if match == nil {
		return Verdict{}, fmt.Errorf("no JSON object found in agent output")
	}
	if err := json.Unmarshal(match, &v); err != nil {
		return Verdict{}, fmt.Errorf("failed to decode verdict JSON: %w", err)
	}
	if err := v.Valid(); err != nil {
		return Verdict{}, err
	}
	return v, nil
}

// unwrapEnvelope handles a couple of known CLI output shapes that wrap the
// real assistant content. Returns the unwrapped JSON bytes and true on a
// successful unwrap, or nil/false when raw doesn't look like a known shape.
func unwrapEnvelope(raw []byte) ([]byte, bool) {
	// Shape 1: Claude Code -p --output-format json returns
	//   {"type":"result","result":"<assistant text>", ...}.
	var claude struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(raw, &claude); err == nil && claude.Result != "" {
		return []byte(claude.Result), true
	}

	// Shape 2: opencode run --format json returns
	//   {"output":"<assistant text>"} or similar.
	var opencode struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &opencode); err == nil && opencode.Output != "" {
		return []byte(opencode.Output), true
	}

	return nil, false
}

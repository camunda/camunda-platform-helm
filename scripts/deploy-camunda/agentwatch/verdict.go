package agentwatch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

// ParseVerdict extracts a Verdict from raw agent CLI output. It tries, in
// order: strict JSON decoding, unwrapping known CLI envelopes, then walking
// the string for balanced JSON objects and decoding each in turn. This keeps
// the parser robust to small wrapping differences between Claude Code and
// opencode and to trailing prose that contains braces.
func ParseVerdict(raw []byte) (Verdict, error) {
	candidates := [][]byte{raw}
	if unwrapped, ok := unwrapEnvelope(raw); ok {
		candidates = append([][]byte{unwrapped}, candidates...)
	}

	for _, candidate := range candidates {
		var v Verdict
		if err := json.Unmarshal(candidate, &v); err == nil {
			if v.Valid() == nil {
				return v, nil
			}
		}
		for _, sub := range extractBalancedObjects(candidate) {
			if err := json.Unmarshal(sub, &v); err != nil {
				continue
			}
			if v.Valid() == nil {
				return v, nil
			}
		}
	}
	return Verdict{}, fmt.Errorf("no valid verdict JSON found in agent output")
}

// extractBalancedObjects returns every balanced top-level JSON object
// substring in s, in order. It tracks string literals + escapes so braces
// inside strings don't unbalance the count. A greedy regex like `\{.*\}`
// would overshoot when prose after the verdict contains '}', and a lazy
// regex would stop at the first '}' inside a nested structure; the brace
// walker handles both.
func extractBalancedObjects(s []byte) [][]byte {
	var out [][]byte
	for i := 0; i < len(s); {
		if s[i] != '{' {
			i++
			continue
		}
		start := i
		depth := 0
		inString := false
		escape := false
		closed := false
		for ; i < len(s); i++ {
			ch := s[i]
			if inString {
				if escape {
					escape = false
				} else if ch == '\\' {
					escape = true
				} else if ch == '"' {
					inString = false
				}
				continue
			}
			if ch == '"' {
				inString = true
				continue
			}
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					out = append(out, s[start:i+1])
					i++
					closed = true
					break
				}
			}
		}
		if !closed {
			// Unterminated candidate '{'. Don't give up on the whole input —
			// a later '{' may still open a balanced object. Resume one
			// character past the unbalanced opener.
			i = start + 1
		}
	}
	return out
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

	// Shape 2: opencode run --format json returns a single JSON envelope
	//   {"output":"<assistant text>"}.
	var opencodeSingle struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &opencodeSingle); err == nil && opencodeSingle.Output != "" {
		return []byte(opencodeSingle.Output), true
	}

	// Shape 3: opencode run --format json (current versions) emits NDJSON —
	// newline-delimited JSON events. The assistant text lives in events with
	// type "text", inside part.text. Concatenate all text parts to form the
	// full response, then strip markdown code fences.
	if text, ok := unwrapOpencodeNDJSON(raw); ok {
		return []byte(text), true
	}

	return nil, false
}

// unwrapOpencodeNDJSON parses a stream of newline-delimited JSON events from
// opencode's --format json output and extracts the concatenated assistant text.
// Events have the shape: {"type":"text","part":{"text":"..."},...}
func unwrapOpencodeNDJSON(raw []byte) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024) // allow large lines

	var textParts []string
	foundText := false

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event struct {
			Type string `json:"type"`
			Part struct {
				Text string `json:"text"`
			} `json:"part"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if event.Type == "text" && event.Part.Text != "" {
			textParts = append(textParts, event.Part.Text)
			foundText = true
		}
	}

	if !foundText {
		return "", false
	}

	text := strings.Join(textParts, "")
	// Strip markdown code fences (```json ... ```) that the agent often wraps around JSON.
	text = stripCodeFences(text)
	return text, true
}

// stripCodeFences removes markdown code fences from text, handling both
// ```json\n...\n``` and ```\n...\n``` patterns.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	// Try to strip opening fence
	if strings.HasPrefix(s, "```") {
		// Find end of opening fence line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
	}
	// Strip closing fence
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

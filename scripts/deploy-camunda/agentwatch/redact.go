package agentwatch

import (
	"encoding/json"
	"regexp"
)

// Redaction for snapshot bytes persisted to the eval corpus. Pod specs
// surface plain-value env entries for credentials (DB_PASSWORD,
// HARBOR_TOKEN, KEYCLOAK_CLIENT_SECRET, etc.) and event/log strings leak
// bearer tokens. The corpus is intended to be portable across repos and
// shared, so anything written there must be scrubbed.
//
// We redact a deep clone (via JSON round-trip) rather than the in-memory
// struct the agent reads — the agent's signal value comes from SEEING the
// shape of an env var named DB_PASSWORD, while the corpus only needs the
// shape of that fact, not its value.

var (
	sensitiveEnvNamePattern = regexp.MustCompile(`(?i)TOKEN|SECRET|PASSWORD|KEY|CREDENTIAL|BEARER|AUTH`)
	bearerTokenPattern      = regexp.MustCompile(`\bBearer\s+[A-Za-z0-9._\-+/=]+`)
	authHeaderPattern       = regexp.MustCompile(`(?i)\bAuthorization:\s*[^\s,;]+`)
	jwtPattern              = regexp.MustCompile(`eyJ[A-Za-z0-9._\-]{20,}`)
)

const redactedPlaceholder = "[redacted]"

// RedactRawAgentOutput strips bearer tokens, JWTs, and Authorization
// headers from a raw agent CLI response. The agent reads the unredacted
// in-memory snapshot, so its diagnosis (or causal chain, or cited
// evidence) can quote credential-bearing log lines verbatim. The
// persisted raw output should be scrubbed with the same rules used on
// the snapshot.
func RedactRawAgentOutput(raw string) string {
	return redactString(raw)
}

// RedactForCorpus returns a redacted JSON copy of snapshotBytes suitable
// for writing to the eval corpus. Returns the original bytes on parse
// failure rather than refusing to persist — corpus entries are
// best-effort, and a failed redaction must not silently lose telemetry.
func RedactForCorpus(snapshotBytes []byte) []byte {
	var node any
	if err := json.Unmarshal(snapshotBytes, &node); err != nil {
		return snapshotBytes
	}
	walkAndRedact(&node)
	out, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return snapshotBytes
	}
	return out
}

// walkAndRedact mutates v in place. It handles two redaction shapes:
//   - K8s container env entries ({name: ..., value: ...}) — if name matches
//     the sensitive pattern, value is replaced.
//   - String values anywhere — bearer tokens / JWTs / Authorization
//     headers are replaced with placeholders.
func walkAndRedact(v *any) {
	switch val := (*v).(type) {
	case map[string]any:
		// Special case: env entry shape.
		if name, ok := val["name"].(string); ok {
			if rawValue, hasValue := val["value"]; hasValue {
				if _, isString := rawValue.(string); isString {
					if sensitiveEnvNamePattern.MatchString(name) {
						val["value"] = redactedPlaceholder
					}
				}
			}
		}
		for k, child := range val {
			if s, ok := child.(string); ok {
				val[k] = redactString(s)
				continue
			}
			c := child
			walkAndRedact(&c)
			val[k] = c
		}
	case []any:
		for i, child := range val {
			if s, ok := child.(string); ok {
				val[i] = redactString(s)
				continue
			}
			c := child
			walkAndRedact(&c)
			val[i] = c
		}
	}
}

func redactString(s string) string {
	s = bearerTokenPattern.ReplaceAllString(s, "Bearer [redacted]")
	s = authHeaderPattern.ReplaceAllString(s, "Authorization: [redacted]")
	s = jwtPattern.ReplaceAllString(s, "[redacted-jwt]")
	return s
}

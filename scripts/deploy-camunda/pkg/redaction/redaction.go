// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redaction

import (
	"regexp"
	"strings"
)

const Placeholder = "[REDACTED]"

var (
	sensitiveNamePattern = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|credential|api[-_.]?key|private[-_.]?key)`)
	assignmentPattern    = regexp.MustCompile(`(?im)^(\s*(?:[-*]\s*)?)([A-Za-z0-9_.-]+)(\s*(?::|=)\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\r\n]+)`)
	jsonPattern          = regexp.MustCompile(`(?i)"((?:\\.|[^"\\])*)"\s*:\s*("(?:\\.|[^"\\])*"|[^,}\r\n]+)`)
	authHeaderPattern    = regexp.MustCompile(`(?im)(^\s*(?:(?:proxy-)?authorization|cookie|set-cookie|x-api-key)\s*:\s*)[^\r\n]+`)
	bearerPattern        = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/-]+=*`)
	jwtPattern           = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+`)
	urlUserInfoPattern   = regexp.MustCompile(`(://[^\s/:@]+:)[^\s/@]+(@)`)
	queryPattern         = regexp.MustCompile(`(?i)([?&][A-Za-z0-9_.-]*(?:password|passwd|pwd|secret|token|credential|api[-_.]?key|private[-_.]?key)[A-Za-z0-9_.-]*=)[^&#\s]+`)
	inlineAssignment     = regexp.MustCompile(`(^|[ \t])([A-Za-z0-9_.-]+)=([^\s,]+)`)
	privateKeyPattern    = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
)

func IsSensitiveName(name string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(name))
	for _, suffix := range []string{"secretname", "secretkeyref", "tokenurl", "passwordpolicy"} {
		if strings.HasSuffix(normalized, suffix) {
			return false
		}
	}
	return sensitiveNamePattern.MatchString(name)
}

func Text(value string) string {
	value = privateKeyPattern.ReplaceAllString(value, Placeholder)
	value = authHeaderPattern.ReplaceAllString(value, `${1}`+Placeholder)
	value = bearerPattern.ReplaceAllString(value, "Bearer "+Placeholder)
	value = jwtPattern.ReplaceAllString(value, Placeholder)
	value = urlUserInfoPattern.ReplaceAllString(value, `${1}`+Placeholder+`${2}`)
	value = queryPattern.ReplaceAllString(value, `${1}`+Placeholder)
	value = jsonPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := jsonPattern.FindStringSubmatch(match)
		if len(parts) != 3 || !IsSensitiveName(parts[1]) {
			return match
		}
		colon := strings.Index(match, ":")
		return match[:colon+1] + ` "` + Placeholder + `"`
	})
	value = redactInlineAssignments(value)
	return assignmentPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := assignmentPattern.FindStringSubmatch(match)
		if len(parts) != 4 || !IsSensitiveName(parts[2]) {
			return match
		}
		return parts[1] + parts[2] + parts[3] + Placeholder
	})
}

func redactInlineAssignments(value string) string {
	return inlineAssignment.ReplaceAllStringFunc(value, func(match string) string {
		parts := inlineAssignment.FindStringSubmatch(match)
		if len(parts) != 4 || !IsSensitiveName(parts[2]) {
			return match
		}
		return parts[1] + parts[2] + "=" + Placeholder
	})
}

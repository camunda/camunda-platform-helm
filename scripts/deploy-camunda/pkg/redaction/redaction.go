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

import "regexp"

const Placeholder = "[REDACTED]"

var (
	sensitiveNamePattern = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|credential|api[-_.]?key|private[-_.]?key)`)
	assignmentPattern    = regexp.MustCompile(`(?im)(^\s*(?:[-*]\s*)?[A-Za-z0-9_.-]*(?:password|passwd|pwd|secret|token|credential|api[-_.]?key|private[-_.]?key)[A-Za-z0-9_.-]*\s*(?::|=)\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,\r\n]+)`)
	jsonPattern          = regexp.MustCompile(`(?i)("[^"]*(?:password|passwd|pwd|secret|token|credential|api[-_.]?key|private[-_.]?key)[^"]*"\s*:\s*)"[^"]*"`)
	authHeaderPattern    = regexp.MustCompile(`(?i)(\b(?:proxy-)?authorization\s*:\s*)(?:bearer\s+)?[^\s,;]+`)
	bearerPattern        = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/-]+=*`)
	jwtPattern           = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+`)
	urlUserInfoPattern   = regexp.MustCompile(`(://[^\s/:@]+:)[^\s/@]+(@)`)
	privateKeyPattern    = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
)

func IsSensitiveName(name string) bool {
	return sensitiveNamePattern.MatchString(name)
}

func Text(value string) string {
	value = privateKeyPattern.ReplaceAllString(value, Placeholder)
	value = authHeaderPattern.ReplaceAllString(value, `${1}`+Placeholder)
	value = bearerPattern.ReplaceAllString(value, "Bearer "+Placeholder)
	value = jwtPattern.ReplaceAllString(value, Placeholder)
	value = urlUserInfoPattern.ReplaceAllString(value, `${1}`+Placeholder+`${2}`)
	value = jsonPattern.ReplaceAllString(value, `${1}"`+Placeholder+`"`)
	return assignmentPattern.ReplaceAllString(value, `${1}`+Placeholder)
}

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

import "testing"

func TestText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "env assignment", in: "DB_PASSWORD=super-secret", want: "DB_PASSWORD=" + Placeholder},
		{name: "prefixed env assignment", in: "2026-08-05 INFO API_TOKEN=secret", want: "2026-08-05 INFO API_TOKEN=" + Placeholder},
		{name: "yaml assignment", in: "  clientSecret: 'secret value'", want: "  clientSecret: " + Placeholder},
		{name: "json value", in: `{"access_token":"secret-token","status":"failed"}`, want: `{"access_token": "[REDACTED]","status":"failed"}`},
		{name: "authorization header", in: "Authorization: Bearer abc.def-123", want: "Authorization: " + Placeholder},
		{name: "basic authorization header", in: "Authorization: Basic dXNlcjpwYXNz", want: "Authorization: " + Placeholder},
		{name: "curl authorization header", in: "> Authorization: Basic dXNlcjpwYXNz", want: "> Authorization: " + Placeholder},
		{name: "inline curl authorization header", in: `curl -H 'Authorization: Basic dXNlcjpwYXNz' https://example.test`, want: `curl -H 'Authorization: ` + Placeholder + `' https://example.test`},
		{name: "cookie header", in: "Set-Cookie: session=secret; Secure", want: "Set-Cookie: " + Placeholder},
		{name: "standalone bearer", in: "request failed with Bearer abc.def-123", want: "request failed with Bearer " + Placeholder},
		{name: "URL user info", in: "postgres://user:secret@database:5432/app", want: "postgres://user:" + Placeholder + "@database:5432/app"},
		{name: "query token", in: "https://example.test/callback?code=ok&access_token=secret", want: "https://example.test/callback?code=ok&access_token=" + Placeholder},
		{name: "private key", in: "before\n-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\nafter", want: "before\n" + Placeholder + "\nafter"},
		{name: "ordinary diagnostics", in: "pod-a 0/1 Running\nFailedMount: secret volume unavailable", want: "pod-a 0/1 Running\nFailedMount: secret volume unavailable"},
		{name: "assignment inside a json string", in: `"stdout": "ZEEBE_CLIENT_SECRET=super-secret"`, want: `"stdout": "ZEEBE_CLIENT_SECRET=` + Placeholder + `"`},
		{name: "assignment inside a json array element", in: `["API_TOKEN=abc123", "ok"]`, want: `["API_TOKEN=` + Placeholder + `", "ok"]`},
		// Only quotes are excluded from the value, so a trailing non-quote
		// delimiter is consumed with it. That is deliberate: narrowing the value
		// class further would leak the remainder of any secret containing one of
		// those characters, and over-redacting is the safer failure.
		{name: "bracketed assignment consumes the closing delimiter", in: "helm(DB_PASSWORD=hunter2)", want: "helm(DB_PASSWORD=" + Placeholder},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Text(tt.in); got != tt.want {
				t.Fatalf("Text() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSensitiveName(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		want bool
	}{
		{name: "global.identity.clientSecret", want: true},
		{name: "database.password", want: true},
		{name: "secretKeyRef", want: false},
		{name: "SECRET_KEY", want: true},
		{name: "oauth.tokenUrl", want: false},
		{name: "global.ingress.host", want: false},
	} {
		if got := IsSensitiveName(tt.name); got != tt.want {
			t.Errorf("IsSensitiveName(%q) = %t, want %t", tt.name, got, tt.want)
		}
	}
}

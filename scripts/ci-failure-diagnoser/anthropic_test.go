// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiagnose_HappyPath(t *testing.T) {
	var captured messagesRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/v1/messages"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Errorf("missing anthropic-version header")
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"### Likely cause\nFoo\n"}],"stop_reason":"end_turn"}`))
	}))
	defer srv.Close()

	c := NewAnthropicClient("test-key", "claude-sonnet-4-6")
	c.BaseURL = srv.URL

	out, err := c.Diagnose(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	if !strings.HasPrefix(out, "### Likely cause") {
		t.Errorf("response = %q", out)
	}

	if captured.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q", captured.Model)
	}
	if len(captured.System) == 0 || captured.System[0].Text != "sys" {
		t.Errorf("system block missing or wrong: %+v", captured.System)
	}
	if captured.System[0].CacheControl == nil || captured.System[0].CacheControl.Type != "ephemeral" {
		t.Errorf("system block should be cache-eligible: %+v", captured.System[0])
	}
	if len(captured.Messages) != 1 || captured.Messages[0].Content != "user" {
		t.Errorf("messages wrong: %+v", captured.Messages)
	}
}

func TestDiagnose_DefaultModel(t *testing.T) {
	c := NewAnthropicClient("k", "")
	if c.Model != "claude-sonnet-4-6" {
		t.Errorf("default model = %q", c.Model)
	}
}

func TestDiagnose_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	c := NewAnthropicClient("k", "m")
	c.BaseURL = srv.URL
	if _, err := c.Diagnose(context.Background(), "s", "u"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDiagnose_NoTextBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"tool_use","text":""}],"stop_reason":"tool_use"}`))
	}))
	defer srv.Close()

	c := NewAnthropicClient("k", "m")
	c.BaseURL = srv.URL
	if _, err := c.Diagnose(context.Background(), "s", "u"); err == nil {
		t.Fatal("expected error when no text block returned")
	}
}

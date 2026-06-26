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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicClient is the minimal surface we need for one-shot Messages calls.
type AnthropicClient struct {
	APIKey  string
	BaseURL string // override for tests; defaults to https://api.anthropic.com
	Model   string // e.g. "claude-sonnet-4-6"
	HTTP    *http.Client
}

// NewAnthropicClient builds a client with sensible defaults for a CI tool.
// Sonnet is the cost/quality sweet spot for CI diagnosis; callers can pin
// a different model via env var.
func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &AnthropicClient{
		APIKey:  apiKey,
		BaseURL: "https://api.anthropic.com",
		Model:   model,
		HTTP:    &http.Client{Timeout: 90 * time.Second},
	}
}

type messagesRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    []systemBlock `json:"system,omitempty"`
	Messages  []userMessage `json:"messages"`
}

type systemBlock struct {
	Type         string             `json:"type"`
	Text         string             `json:"text"`
	CacheControl *cacheControlBlock `json:"cache_control,omitempty"`
}

type cacheControlBlock struct {
	Type string `json:"type"`
}

type userMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// Diagnose calls the Messages API once and returns the first text block.
// The system prompt is marked cache-eligible: parallel matrix scenarios that
// fail in the same workflow_run will share a cache hit.
func (c *AnthropicClient) Diagnose(ctx context.Context, system, user string) (string, error) {
	payload := messagesRequest{
		Model:     c.Model,
		MaxTokens: 1024,
		System: []systemBlock{{
			Type:         "text",
			Text:         system,
			CacheControl: &cacheControlBlock{Type: "ephemeral"},
		}},
		Messages: []userMessage{{Role: "user", Content: user}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic %s: %s", resp.Status, truncate(string(respBody), 500))
	}

	var parsed messagesResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	for _, block := range parsed.Content {
		if block.Type == "text" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", fmt.Errorf("no text block in response (stop_reason=%s)", parsed.StopReason)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

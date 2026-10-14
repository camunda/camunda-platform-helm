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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	slackPending  = "triage:slack-pending"
	slackNotified = "triage:slack-notified"
	slackRetry    = "triage:slack-retry"
)

type slackClient struct {
	http    *http.Client
	webhook string
}

func (s *slackClient) notify(c *githubClient, original issue, status string) error {
	i, err := c.getIssue(original.Number)
	if err != nil {
		return err
	}
	if i.has(slackNotified) {
		return nil
	}
	if i.has(slackPending) {
		return fmt.Errorf("issue #%d has an ambiguous Slack delivery; reconcile triage:slack-pending before retrying", i.Number)
	}
	if s == nil || s.webhook == "" {
		return fmt.Errorf("SLACK_WEBHOOK is required for external intake")
	}
	for _, name := range []string{slackPending, slackNotified, slackRetry} {
		if err := c.ensureLabel(name); err != nil {
			return err
		}
	}
	text := fmt.Sprintf("🎫 [%s] #%d %s\n%s\n%s", c.repo, i.Number, i.Title, i.HTMLURL, status)
	payload := map[string]any{"blocks": []any{map[string]any{"type": "section", "text": map[string]any{"type": "plain_text", "text": text, "emoji": true}}}}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", s.webhook, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("invalid Slack webhook configuration")
	}
	if req.URL.Host == "" || req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("invalid Slack webhook configuration")
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.addLabels(i.Number, slackPending, slackRetry); err != nil {
		return err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("ambiguous Slack delivery for #%d; inspect Slack and reconcile %s", i.Number, slackPending)
	}
	defer resp.Body.Close()
	response, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusOK && readErr == nil && strings.TrimSpace(string(response)) == "ok" {
		if err := c.addLabels(i.Number, slackNotified); err != nil {
			return fmt.Errorf("Slack delivered but receipt failed; reconcile %s: %w", slackPending, err)
		}
		if err := c.removeLabel(i.Number, slackPending); err != nil {
			return err
		}
		return c.removeLabel(i.Number, slackRetry)
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		if err := c.removeLabel(i.Number, slackPending); err != nil {
			return fmt.Errorf("Slack rejected delivery; clearing pending state failed: %w", err)
		}
		return fmt.Errorf("Slack rejected delivery with HTTP %d; retry is safe", resp.StatusCode)
	}
	return fmt.Errorf("ambiguous Slack response HTTP %d; inspect Slack and reconcile %s", resp.StatusCode, slackPending)
}

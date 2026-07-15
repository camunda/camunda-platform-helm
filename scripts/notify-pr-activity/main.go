// Copyright 2025 Camunda Services GmbH
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
	"net/http"
	"os"
	"strings"
	"time"
)

const timeLayout = time.RFC3339

type slackBlock struct {
	Type string    `json:"type"`
	Text slackText `json:"text"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackPayload struct {
	Blocks []slackBlock `json:"blocks"`
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "error: required env var %s is not set\n", key)
		os.Exit(1)
	}
	return v
}

// userMapPath returns the slack-user-map.json path, overridable via SLACK_USER_MAP_PATH.
func userMapPath() string {
	if p := os.Getenv("SLACK_USER_MAP_PATH"); p != "" {
		return p
	}
	return "slack-user-map.json"
}

func shortRepo(name string) string {
	return strings.TrimPrefix(name, "camunda-platform-")
}

func formatDuration(from, to time.Time) string {
	diff := to.Sub(from)
	days := int(diff.Hours()) / 24
	hours := int(diff.Hours()) % 24
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return "< 1h"
	}
}

// loadUserMap reads a GitHub-login -> Slack-user-ID JSON map. A missing or
// unreadable file yields an empty map, so reviewers fall back to "@login".
func loadUserMap(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ℹ️  no user map at %s: %v (falling back to @login)\n", path, err)
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  invalid user map %s: %v (falling back to @login)\n", path, err)
		return map[string]string{}
	}
	return m
}

// slackMention resolves a GitHub login to a Slack "<@UID>" mention, or "@login" when unmapped.
func slackMention(login string, userMap map[string]string) string {
	if uid, ok := userMap[login]; ok && uid != "" {
		return "<@" + uid + ">"
	}
	return "@" + login
}

// parseReviewers decodes a JSON array of GitHub user objects and returns a
// comma-separated list of Slack mentions (or "@login" fallbacks).
func parseReviewers(raw string, userMap map[string]string) string {
	var users []struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal([]byte(raw), &users); err != nil || len(users) == 0 {
		return ""
	}
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, slackMention(u.Login, userMap))
	}
	return strings.Join(names, ", ")
}

// hasLabel returns true if PR_LABELS_JSON contains a label with the given name.
func hasLabel(name string) bool {
	raw := os.Getenv("PR_LABELS_JSON")
	var labels []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &labels); err != nil {
		return false
	}
	for _, l := range labels {
		if l.Name == name {
			return true
		}
	}
	return false
}

func buildMessage() string {
	action := mustEnv("GH_ACTION")
	repo := shortRepo(mustEnv("PR_REPO"))
	prURL := mustEnv("PR_URL")
	prNum := "#" + mustEnv("PR_NUMBER")
	prTitle := mustEnv("PR_TITLE")

	link := fmt.Sprintf("<%s|%s %s>", prURL, prNum, prTitle)

	switch action {
	case "opened", "ready_for_review":
		userMap := loadUserMap(userMapPath())
		reviewers := parseReviewers(os.Getenv("PR_REVIEWERS_JSON"), userMap)
		if reviewers != "" {
			return fmt.Sprintf("↗ [%s] %s — review: %s", repo, link, reviewers)
		}
		return fmt.Sprintf("↗ [%s] %s", repo, link)

	case "closed":
		merged := os.Getenv("PR_MERGED") == "true"
		if !merged {
			// Closed without merge: no notification.
			return ""
		}
		createdAt, err1 := time.Parse(timeLayout, mustEnv("PR_CREATED_AT"))
		mergedAt, err2 := time.Parse(timeLayout, mustEnv("PR_MERGED_AT"))
		duration := "unknown"
		if err1 == nil && err2 == nil {
			duration = formatDuration(createdAt, mergedAt)
		}
		numLink := fmt.Sprintf("<%s|%s %s>", prURL, prNum, prTitle)
		return fmt.Sprintf("✅ [%s] %s merged after %s", repo, numLink, duration)

	default:
		fmt.Fprintf(os.Stderr, "error: unhandled action %q\n", action)
		os.Exit(1)
		return ""
	}
}

func sendSlack(webhook, message string) error {
	payload := slackPayload{
		Blocks: []slackBlock{
			{Type: "section", Text: slackText{Type: "mrkdwn", Text: message}},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}

func main() {
	// Suppress notifications for bot authors, unless it's a renovate major-version update.
	if strings.HasSuffix(os.Getenv("PR_AUTHOR"), "[bot]") && !hasLabel("upgrade:major") {
		fmt.Println("ℹ️  Skipping Slack notification: bot author (non-major update).")
		return
	}

	webhook := mustEnv("SLACK_WEBHOOK")
	message := buildMessage()

	if message == "" {
		fmt.Println("ℹ️  Skipping Slack notification: no message to send (e.g. closed without merge).")
		return
	}

	fmt.Printf("📣 Sending Slack notification: %s\n", message)

	if err := sendSlack(webhook, message); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Slack notification failed (non-fatal): %v\n", err)
	}
}

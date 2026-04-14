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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const timeLayout = "2006-01-02T15:04:05Z"

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

// parseReviewers decodes a JSON array of GitHub user objects and returns "@login, ..." string.
func parseReviewers(raw string) string {
	var users []struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal([]byte(raw), &users); err != nil || len(users) == 0 {
		return ""
	}
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, "@"+u.Login)
	}
	return strings.Join(names, ", ")
}

func buildMessage() string {
	action := mustEnv("GH_ACTION")
	repo := shortRepo(mustEnv("PR_REPO"))
	prURL := mustEnv("PR_URL")
	prNum := "#" + mustEnv("PR_NUMBER")
	prTitle := mustEnv("PR_TITLE")
	author := "@" + mustEnv("PR_AUTHOR")

	link := fmt.Sprintf("<%s|%s %s>", prURL, prNum, prTitle)

	switch action {
	case "assigned":
		size := fmt.Sprintf("+%s/-%s", mustEnv("PR_ADDITIONS"), mustEnv("PR_DELETIONS"))
		reviewers := parseReviewers(os.Getenv("PR_REVIEWERS_JSON"))
		reviewText := ""
		if reviewers != "" {
			reviewText = " · review: " + reviewers
		}
		return fmt.Sprintf(":arrow_heading_up: [%s] PR opened · by %s%s · %s — %s",
			repo, author, reviewText, size, link)

	case "closed":
		merged := os.Getenv("PR_MERGED") == "true"
		if merged {
			createdAt, err1 := time.Parse(timeLayout, mustEnv("PR_CREATED_AT"))
			mergedAt, err2 := time.Parse(timeLayout, mustEnv("PR_MERGED_AT"))
			duration := "unknown"
			if err1 == nil && err2 == nil {
				duration = formatDuration(createdAt, mergedAt)
			}
			return fmt.Sprintf(":tada: [%s] PR merged after %s · by %s — %s",
				repo, duration, author, link)
		}
		return fmt.Sprintf(":x: [%s] PR closed without merge · by %s — %s",
			repo, author, link)

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
	resp, err := http.Post(webhook, "application/json", bytes.NewReader(body)) //nolint:noctx
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
	webhook := mustEnv("SLACK_WEBHOOK")
	message := buildMessage()

	fmt.Printf("📣 Sending Slack notification: %s\n", message)

	if err := sendSlack(webhook, message); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Slack notification failed (non-fatal): %v\n", err)
	}
}

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
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	gleanAPIURL           = "https://camunda-be.glean.com/rest/api/v1/chat"
	gleanRequestTimeout   = 120 * time.Second
	defaultRequestTimeout = 20 * time.Second
)

type config struct {
	WeekOffset      int
	GleanAPIToken   string
	SlackWebhookURL string
	MedicHandle     string
	SlackChannel    string
	AlertChannel    string
	SupportChannels string
}

type gleanRequest struct {
	Messages []gleanMessage `json:"messages"`
}

type gleanMessage struct {
	Author      string          `json:"author"`
	MessageType string          `json:"messageType"`
	Fragments   []gleanFragment `json:"fragments"`
}

type gleanFragment struct {
	Text string `json:"text"`
}

type gleanResponse struct {
	Messages []gleanMessage `json:"messages"`
}

type slackPayload struct {
	Channel string       `json:"channel"`
	Blocks  []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type string    `json:"type"`
	Text slackText `json:"text"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func mustEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		fmt.Fprintf(os.Stderr, "missing required environment variable: %s\n", name)
		os.Exit(1)
	}
	return v
}

func loadConfig() config {
	weekOffsetRaw := mustEnv("WEEK_OFFSET")
	weekOffset, err := strconv.Atoi(weekOffsetRaw)
	if err != nil || weekOffset < 0 {
		fmt.Fprintf(os.Stderr, "WEEK_OFFSET must be a non-negative integer: %q\n", weekOffsetRaw)
		os.Exit(1)
	}

	return config{
		WeekOffset:      weekOffset,
		GleanAPIToken:   mustEnv("GLEAN_API_TOKEN"),
		SlackWebhookURL: mustEnv("SLACK_DISTRO_BOT_WEBHOOK_REPORTS"),
		MedicHandle:     mustEnv("MEDIC_HANDLE"),
		SlackChannel:    mustEnv("SLACK_CHANNEL"),
		AlertChannel:    mustEnv("ALERT_CHANNEL"),
		SupportChannels: mustEnv("SUPPORT_CHANNELS"),
	}
}

func weekWindow(now time.Time, offsetWeeks int) (time.Time, time.Time) {
	reference := now.UTC().AddDate(0, 0, -7*offsetWeeks)
	weekday := int(reference.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(reference.Year(), reference.Month(), reference.Day(), 0, 0, 0, 0, time.UTC)
	start = start.AddDate(0, 0, -(weekday - 1))
	end := start.AddDate(0, 0, 6)
	return start, end
}

func doJSONRequest(ctx context.Context, method, endpoint string, headers map[string]string, reqBody any, respBody any, timeout time.Duration) error {
	var bodyReader io.Reader
	if reqBody != nil {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s: %s", resp.Status, strings.TrimSpace(string(respRaw)))
	}

	if respBody != nil {
		if err := json.Unmarshal(respRaw, respBody); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

func buildPrompt(cfg config, weekStart, weekEnd time.Time) string {
	_, isoWeek := weekStart.ISOWeek()
	return fmt.Sprintf(`You are generating the weekly distro-medic report for the Distribution team at Camunda.

Report period: %s to %s (W%02d)

Follow the Distro - Medic Report Guidelines from the Camunda Confluence documentation exactly.
Search all data sources listed in the guidelines for activity during this period.
Determine who was medic for this report period by looking up the Slack @%s user group membership/activity in that period.
Include a line in the report: Current medic: <name>.
Ensure you include activity from support channels (%s) and alert channel (%s) when relevant.

Respond ONLY with the Slack message content. No wrapping, no explanation.
`, weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"), isoWeek, strings.TrimPrefix(cfg.MedicHandle, "@"), cfg.SupportChannels, cfg.AlertChannel)
}

func generateReport(ctx context.Context, cfg config, prompt string) (string, error) {
	var out gleanResponse
	err := doJSONRequest(ctx, http.MethodPost, gleanAPIURL, map[string]string{
		"Authorization": "Bearer " + cfg.GleanAPIToken,
		"Content-Type":  "application/json",
	}, gleanRequest{Messages: []gleanMessage{{
		Author:      "USER",
		MessageType: "CONTENT",
		Fragments:   []gleanFragment{{Text: prompt}},
	}}}, &out, gleanRequestTimeout)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, msg := range out.Messages {
		if msg.MessageType != "CONTENT" || msg.Author != "GLEAN_AI" {
			continue
		}
		for _, frag := range msg.Fragments {
			if frag.Text != "" {
				sb.WriteString(frag.Text)
			}
		}
	}

	report := strings.TrimSpace(sb.String())
	if report == "" {
		return "", fmt.Errorf("glean response did not contain report content")
	}
	return report, nil
}

func postSlack(ctx context.Context, cfg config, report string) error {
	payload := slackPayload{
		Channel: cfg.SlackChannel,
		Blocks: []slackBlock{{
			Type: "section",
			Text: slackText{Type: "mrkdwn", Text: report},
		}},
	}
	return doJSONRequest(ctx, http.MethodPost, cfg.SlackWebhookURL, map[string]string{
		"Content-Type": "application/json",
	}, payload, nil, defaultRequestTimeout)
}

func main() {
	cfg := loadConfig()
	ctx := context.Background()

	weekStart, weekEnd := weekWindow(time.Now(), cfg.WeekOffset)
	_, isoWeek := weekStart.ISOWeek()
	fmt.Printf("Reporting period: %s to %s (W%02d)\n", weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"), isoWeek)

	prompt := buildPrompt(cfg, weekStart, weekEnd)
	report, err := generateReport(ctx, cfg, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate report from Glean: %v\n", err)
		os.Exit(1)
	}

	if err := postSlack(ctx, cfg, report); err != nil {
		fmt.Fprintf(os.Stderr, "post report to Slack: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Weekly medic report generated and posted to Slack successfully")
}

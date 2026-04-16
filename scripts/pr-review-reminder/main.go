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

// pr-review-reminder scans a set of GitHub repositories for open (non-draft) PRs that have gone
// stale without a review action, then posts a grouped Slack Block Kit message mentioning each
// team member who needs to act. Team membership and GitHub→Slack user resolution are driven by
// ../slack-user-map.json (or the path set in SLACK_USER_MAP_PATH).
//
// Environment variables (all required unless noted):
//
//	GITHUB_TOKEN        – GitHub token with read access to all monitored repos
//	SLACK_WEBHOOK       – Incoming-webhook URL for the target Slack channel
//	SLACK_USER_MAP_PATH – (optional) path to slack-user-map.json; default: ../slack-user-map.json
//	DRY_RUN             – (optional) set to "true" to log the payload without sending

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---- Configuration -------------------------------------------------------

const (
	stalenessThresholdDays = 3
	githubAPIBase          = "https://api.github.com"
	slackBlockSafeLimit    = 48 // Block Kit max is 50; leave room for overflow block
)

var monitoredRepos = []string{
	"camunda/camunda-platform-helm",
	"camunda/team-distribution",
	"camunda/camunda-distributions",
	"camunda/camunda-docs",
	"camunda/camunda",
}

var skipLabels = map[string]bool{
	"do-not-merge/work-in-progress": true,
	"skip-review-reminder":          true,
}

var nudges = []string{
	"These PRs are gathering cobwebs :tumbling-tumbleweed:",
	"Your reviews await :eyes: :please:",
	":code-review-intensifies: Reviews needed!",
	"These PRs miss you :pusheen_sad:",
	":sadpanda: PRs are feeling unloved",
	":grumpycat: These PRs have been waiting a while",
	":blob_help: Help, these PRs need reviews!",
	":code-review: Time to review some code :party_parrot:",
	":tumbleweed: Is anyone reviewing PRs around here?",
	":pusheenmad: Review these PRs or else",
}

// ---- GitHub API types ----------------------------------------------------

type pullRequest struct {
	Number             int          `json:"number"`
	Title              string       `json:"title"`
	HTMLURL            string       `json:"html_url"`
	Draft              bool         `json:"draft"`
	CreatedAt          time.Time    `json:"created_at"`
	User               githubUser   `json:"user"`
	Labels             []label      `json:"labels"`
	RequestedReviewers []githubUser `json:"requested_reviewers"`
}

type githubUser struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type label struct {
	Name string `json:"name"`
}

type review struct {
	User  githubUser `json:"user"`
	State string     `json:"state"` // APPROVED | CHANGES_REQUESTED | DISMISSED | COMMENTED
}

type timelineEvent struct {
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
}

// ---- Application types ---------------------------------------------------

type stalePR struct {
	Title         string
	URL           string
	Number        int
	Author        string
	BusinessDays  int
	Status        string // "needs_review" | "changes_requested" | "approved"
	Assignees     []assignee
	RepoShortName string
}

type assignee struct {
	Login        string
	SlackMention string // "<@UID>" or "@login"
}

// ---- GitHub client -------------------------------------------------------

type githubClient struct {
	token  string
	client *http.Client
}

func newGitHubClient(token string) *githubClient {
	return &githubClient{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

var linkNextRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// fetchAllPages follows GitHub pagination (Link header) and returns every item as raw JSON.
func (g *githubClient) fetchAllPages(ctx context.Context, startURL string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	url := startURL

	for url != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request %s: %w", url, err)
		}
		req.Header.Set("Authorization", "Bearer "+g.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", url, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read body: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s: status %s: %s", url, resp.Status, body)
		}

		var page []json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("unmarshal page from %s: %w", url, err)
		}
		all = append(all, page...)

		url = ""
		if link := resp.Header.Get("Link"); link != "" {
			if m := linkNextRe.FindStringSubmatch(link); len(m) > 1 {
				url = m[1]
			}
		}
	}

	return all, nil
}

func (g *githubClient) listOpenPRs(ctx context.Context, owner, repo string) ([]pullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=100", githubAPIBase, owner, repo)
	raw, err := g.fetchAllPages(ctx, url)
	if err != nil {
		return nil, err
	}
	prs := make([]pullRequest, 0, len(raw))
	for _, r := range raw {
		var pr pullRequest
		if err := json.Unmarshal(r, &pr); err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func (g *githubClient) listReviews(ctx context.Context, owner, repo string, prNumber int) ([]review, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews?per_page=100", githubAPIBase, owner, repo, prNumber)
	raw, err := g.fetchAllPages(ctx, url)
	if err != nil {
		return nil, err
	}
	reviews := make([]review, 0, len(raw))
	for _, r := range raw {
		var rv review
		if err := json.Unmarshal(r, &rv); err != nil {
			return nil, err
		}
		reviews = append(reviews, rv)
	}
	return reviews, nil
}

func (g *githubClient) listTimeline(ctx context.Context, owner, repo string, issueNumber int) ([]timelineEvent, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/timeline?per_page=100", githubAPIBase, owner, repo, issueNumber)
	raw, err := g.fetchAllPages(ctx, url)
	if err != nil {
		return nil, err
	}
	events := make([]timelineEvent, 0, len(raw))
	for _, r := range raw {
		var ev timelineEvent
		if err := json.Unmarshal(r, &ev); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}

// ---- Business logic ------------------------------------------------------

func businessDaysBetween(from, to time.Time) int {
	count := 0
	cur := from.UTC().Truncate(24 * time.Hour)
	end := to.UTC().Truncate(24 * time.Hour)
	for cur.Before(end) {
		cur = cur.Add(24 * time.Hour)
		wd := cur.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			count++
		}
	}
	return count
}

func hasSkipLabel(labels []label) bool {
	for _, l := range labels {
		if skipLabels[l.Name] {
			return true
		}
	}
	return false
}

func slackMention(login string, userMap map[string]string) string {
	if uid, ok := userMap[login]; ok {
		return "<@" + uid + ">"
	}
	return "@" + login
}

func getStalePRs(ctx context.Context, gh *githubClient, repoFull string, userMap map[string]string, teamMembers map[string]bool) ([]stalePR, error) {
	parts := strings.SplitN(repoFull, "/", 2)
	owner, repo := parts[0], parts[1]

	prs, err := gh.listOpenPRs(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	var result []stalePR

	for _, pr := range prs {
		if pr.Draft || hasSkipLabel(pr.Labels) {
			continue
		}

		authorIsTeamMember := teamMembers[pr.User.Login]
		var memberReviewers []githubUser
		for _, r := range pr.RequestedReviewers {
			if teamMembers[r.Login] {
				memberReviewers = append(memberReviewers, r)
			}
		}
		if !authorIsTeamMember && len(memberReviewers) == 0 {
			continue
		}

		reviews, err := gh.listReviews(ctx, owner, repo, pr.Number)
		if err != nil {
			return nil, fmt.Errorf("list reviews %s#%d: %w", repoFull, pr.Number, err)
		}

		// Compute latest non-dismissed review state per reviewer.
		latestByUser := map[string]string{}
		for _, r := range reviews {
			switch r.State {
			case "APPROVED", "CHANGES_REQUESTED":
				latestByUser[r.User.Login] = r.State
			case "DISMISSED":
				delete(latestByUser, r.User.Login)
			}
		}
		// Re-requested reviewers have their previous review invalidated.
		for _, r := range pr.RequestedReviewers {
			delete(latestByUser, r.Login)
		}

		var hasApproved, hasChangesRequested bool
		for _, s := range latestByUser {
			if s == "APPROVED" {
				hasApproved = true
			}
			if s == "CHANGES_REQUESTED" {
				hasChangesRequested = true
			}
		}

		var status string
		switch {
		case hasChangesRequested:
			status = "changes_requested"
		case hasApproved:
			status = "approved"
		default:
			status = "needs_review"
		}

		// Find when the PR last became ready for review.
		reviewableAt := pr.CreatedAt
		timeline, err := gh.listTimeline(ctx, owner, repo, pr.Number)
		if err != nil {
			return nil, fmt.Errorf("list timeline %s#%d: %w", repoFull, pr.Number, err)
		}
		for _, ev := range timeline {
			if ev.Event == "ready_for_review" && !ev.CreatedAt.IsZero() {
				reviewableAt = ev.CreatedAt
			}
		}

		businessDays := businessDaysBetween(reviewableAt, time.Now())

		// Determine who needs to act.
		// For non-bot PRs with review activity: the author needs to merge or address feedback.
		// For needs_review (or bot-authored PRs regardless of review state): remind the reviewers.
		var assignees []assignee
		authorIsBot := pr.User.Type == "Bot"

		if (status == "approved" || status == "changes_requested") && !authorIsBot {
			if !authorIsTeamMember {
				continue
			}
			assignees = append(assignees, assignee{
				Login:        pr.User.Login,
				SlackMention: slackMention(pr.User.Login, userMap),
			})
		} else {
			if len(memberReviewers) == 0 || businessDays < stalenessThresholdDays {
				continue
			}
			status = "needs_review"
			for _, r := range memberReviewers {
				assignees = append(assignees, assignee{
					Login:        r.Login,
					SlackMention: slackMention(r.Login, userMap),
				})
			}
		}

		result = append(result, stalePR{
			Title:         pr.Title,
			URL:           pr.HTMLURL,
			Number:        pr.Number,
			Author:        pr.User.Login,
			BusinessDays:  businessDays,
			Status:        status,
			Assignees:     assignees,
			RepoShortName: repo,
		})
	}

	return result, nil
}

// ---- Slack Block Kit -----------------------------------------------------

type slackText struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

type slackBlock struct {
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

type slackPayload struct {
	Blocks []slackBlock `json:"blocks"`
}

func escapeSlack(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func sectionBlock(text string) slackBlock {
	return slackBlock{Type: "section", Text: &slackText{Type: "mrkdwn", Text: text}}
}

func divider() slackBlock { return slackBlock{Type: "divider"} }

func buildSlackBlocks(byPerson map[string][]stalePR, totalCount int) []slackBlock {
	pluPR := func(n int) string {
		if n == 1 {
			return "PR needs"
		}
		return "PRs need"
	}

	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackText{
				Type:  "plain_text",
				Emoji: true,
				Text:  fmt.Sprintf("%d %s attention", totalCount, pluPR(totalCount)),
			},
		},
		sectionBlock(nudges[rand.Intn(len(nudges))]),
		divider(),
	}

	// Sort people by descending PR count.
	type entry struct {
		mention string
		prs     []stalePR
	}
	people := make([]entry, 0, len(byPerson))
	for mention, prs := range byPerson {
		people = append(people, entry{mention, prs})
	}
	sort.Slice(people, func(i, j int) bool {
		return len(people[i].prs) > len(people[j].prs)
	})

	statusOrder := map[string]int{"needs_review": 0, "changes_requested": 1, "approved": 2}

	for _, pe := range people {
		prs := pe.prs
		sort.Slice(prs, func(i, j int) bool {
			oi, oj := statusOrder[prs[i].Status], statusOrder[prs[j].Status]
			if oi != oj {
				return oi < oj
			}
			return prs[i].BusinessDays > prs[j].BusinessDays
		})

		prLines := make([]string, 0, len(prs))
		for _, pr := range prs {
			var icon, suffix string
			switch pr.Status {
			case "approved":
				icon, suffix = ":white_check_mark:", " — ready to merge"
			case "changes_requested":
				icon, suffix = ":arrows_counterclockwise:", " — changes requested"
			default:
				icon, suffix = ":eyes:", ""
			}
			days := "1 business day"
			if pr.BusinessDays != 1 {
				days = fmt.Sprintf("%d business days", pr.BusinessDays)
			}
			prLines = append(prLines, fmt.Sprintf(
				"%s <%s|#%d: %s>\n`%s` · by %s · %s%s",
				icon, pr.URL, pr.Number, escapeSlack(pr.Title),
				pr.RepoShortName, pr.Author, days, suffix,
			))
		}

		const maxTextLen = 3000
		pluCount := "PRs"
		if len(prs) == 1 {
			pluCount = "PR"
		}
		current := fmt.Sprintf("*%s* — %d %s", pe.mention, len(prs), pluCount)
		for _, line := range prLines {
			if len(current)+2+len(line) > maxTextLen {
				blocks = append(blocks, sectionBlock(current))
				current = line
			} else {
				current += "\n\n" + line
			}
		}
		blocks = append(blocks, sectionBlock(current), divider())

		if len(blocks) >= slackBlockSafeLimit {
			blocks = append(blocks, sectionBlock("_...and more. Check GitHub for the full list._"))
			break
		}
	}

	return blocks
}

// ---- Slack sender --------------------------------------------------------

func sendSlack(ctx context.Context, webhook string, payload slackPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

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

// ---- Helpers -------------------------------------------------------------

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "error: required env var %s is not set\n", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadUserMap(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ---- main ----------------------------------------------------------------

func main() {
	ghToken := mustEnv("GITHUB_TOKEN")
	slackWebhook := mustEnv("SLACK_WEBHOOK")
	userMapPath := envOr("SLACK_USER_MAP_PATH", "../slack-user-map.json")
	dryRun := os.Getenv("DRY_RUN") == "true"

	userMap, err := loadUserMap(userMapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load user map %s: %v\n", userMapPath, err)
		os.Exit(1)
	}

	teamMembers := make(map[string]bool, len(userMap))
	for login := range userMap {
		teamMembers[login] = true
	}

	ctx := context.Background()
	gh := newGitHubClient(ghToken)

	var allStalePRs []stalePR
	for _, repo := range monitoredRepos {
		prs, err := getStalePRs(ctx, gh, repo, userMap, teamMembers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to process %s: %v\n", repo, err)
			continue
		}
		allStalePRs = append(allStalePRs, prs...)
	}

	if len(allStalePRs) == 0 {
		fmt.Println("ℹ️  No stale PR reviews found. Skipping Slack notification.")
		return
	}

	// Group by assignee (a PR with multiple reviewers appears under each).
	byPerson := map[string][]stalePR{}
	for _, pr := range allStalePRs {
		for _, a := range pr.Assignees {
			byPerson[a.SlackMention] = append(byPerson[a.SlackMention], pr)
		}
	}

	totalItems := 0
	for _, prs := range byPerson {
		totalItems += len(prs)
	}
	fmt.Printf("📊 %d action item(s) across %d team member(s).\n", totalItems, len(byPerson))

	payload := slackPayload{Blocks: buildSlackBlocks(byPerson, totalItems)}

	if dryRun {
		out, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Printf("🔍 Dry run — would send:\n%s\n", out)
		return
	}

	fmt.Printf("📣 Sending Slack notification (%d blocks)...\n", len(payload.Blocks))
	if err := sendSlack(ctx, slackWebhook, payload); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Slack notification failed (non-fatal): %v\n", err)
	}
}

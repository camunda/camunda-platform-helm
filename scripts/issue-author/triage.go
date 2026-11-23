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
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	gleanChatURL = "https://camunda-be.glean.com/rest/api/v1/chat"
	inbox        = "📥 Inbox"
	backlog      = "📋 Backlog"
	promptFormat = `You are an expert bug and task triage assistant for the %[1]s repository.

Analyze the following GitHub issue and provide a triage assessment following the Distro Bug Triage and Task/Toil/Non-Feature Triage guidelines from the Camunda documentation.

**Issue #%[2]s: %[3]s**

**Description:**
%[4]s

**Current Labels:** %[5]s

**Instructions:**
1. Check if this issue is within the scope of the SM Platform - Team Boundaries
2. Determine the work type: BUG, TASK, TOIL, FEATURE_REQUEST, or QUESTION
3. For bugs, assess severity (low, mid, high, critical, unknown) and likelihood (low, mid, high)
4. For tasks/toil, assess severity and likelihood based on impact and frequency
5. Search for similar or duplicate existing issues in %[1]s
6. Provide a confidence score (0.0-1.0) for your assessment
7. Provide a detailed rationale citing the triage guidelines
8. Determine the kind based on the Distro triage guidelines: bug (something not working as intended), chore (maintenance: docs updates, dependency bumps, etc), docs (documentation-only change), feature-request (new capability requested), internal (general internal improvement, not customer-facing), refactor (code restructuring without behavior change), research (investigation/spike/poc/proposal to reduce uncertainty)

**IMPORTANT:** Respond ONLY with valid JSON in this exact format:
{"work_type":"BUG|TASK|TOIL|FEATURE_REQUEST|QUESTION","kind":"bug|chore|docs|feature-request|internal|refactor|research","severity":"low|mid|high|critical|unknown","likelihood":"low|mid|high","confidence":0.0,"rationale":"...","similar_issues":["#123"],"missing_info":false,"missing_info_details":"..."}`
	projectQuery = `query($owner:String!, $repo:String!, $issueNumber:Int!, $projectNumber:Int!) {
  repository(owner:$owner, name:$repo) {
    issue(number:$issueNumber) {
      projectItems(first:10) {
        nodes {
          id
          project { id number }
          fieldValueByName(name:"Status") {
            ... on ProjectV2ItemFieldSingleSelectValue { name }
          }
        }
      }
    }
  }
  organization(login:$owner) {
    projectV2(number:$projectNumber) {
      id
      field(name:"Status") {
        ... on ProjectV2SingleSelectField {
          id
          options { id name }
        }
      }
    }
  }
}`
	moveMutation = `mutation($projectId:ID!,$itemId:ID!,$fieldId:ID!,$optionId:String!){updateProjectV2ItemFieldValue(input:{projectId:$projectId,itemId:$itemId,fieldId:$fieldId,value:{singleSelectOptionId:$optionId}}){projectV2Item{id}}}`
)

var (
	rule            = strings.Repeat("━", 80)
	severityEmoji   = map[string]string{"low": "ℹ️", "mid": "⚠️", "high": "🔴", "critical": "🚨"}
	likelihoodEmoji = map[string]string{"low": "🔹", "mid": "🔶", "high": "🔥"}
	slackPrefix     = map[string]string{"high": "⚠️ HIGH — ", "critical": "🚨 CRITICAL — "}
)

type triage struct {
	gh                              func(args ...string) ([]byte, error)
	author                          client
	http                            *http.Client
	gleanURL, gleanToken, slackURL  string
	owner, repo, serverURL, summary string
	project                         int
	threshold                       float64
	classificationOnly              bool
}

type issue struct {
	Title, Body, State string
	User               struct{ Login string }
	Labels             []struct{ Name string }
}

type assessment struct {
	workType, kind, severity, likelihood, confidence, rationale, similar, missingInfo, missingDetails string
}

func runTriage() error {
	owner, repo, token := os.Getenv("OWNER"), os.Getenv("REPO"), os.Getenv("GH_TOKEN")
	project, err := strconv.Atoi(os.Getenv("PROJECT_NUMBER"))
	if err != nil {
		return fmt.Errorf("PROJECT_NUMBER: %w", err)
	}
	threshold, err := strconv.ParseFloat(os.Getenv("CONFIDENCE_THRESHOLD"), 64)
	if err != nil {
		return fmt.Errorf("CONFIDENCE_THRESHOLD: %w", err)
	}
	if token == "" || !repoPattern.MatchString(owner+"/"+repo) {
		return fmt.Errorf("GH_TOKEN, OWNER and REPO are required")
	}
	tr := triage{
		gh:                 gh,
		author:             newClient(owner+"/"+repo, token),
		http:               &http.Client{CheckRedirect: noRedirect},
		gleanURL:           gleanChatURL,
		gleanToken:         os.Getenv("GLEAN_API_TOKEN"),
		slackURL:           os.Getenv("SLACK_DISTRO_BOT_WEBHOOK_GH"),
		owner:              owner,
		repo:               repo,
		serverURL:          os.Getenv("GITHUB_SERVER_URL"),
		summary:            os.Getenv("GITHUB_STEP_SUMMARY"),
		project:            project,
		threshold:          threshold,
		classificationOnly: os.Getenv("CLASSIFICATION_ONLY") == "true",
	}
	for number := range strings.FieldsSeq(os.Getenv("ISSUE_NUMBERS")) {
		if err := tr.process(number); err != nil {
			return err
		}
	}
	fmt.Printf("\n%s\n✅ AI triage workflow completed\n%s\n", rule, rule)
	return nil
}

func gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func (tr *triage) slug() string { return tr.owner + "/" + tr.repo }

func (tr *triage) edit(number string, args ...string) error {
	_, err := tr.gh(append([]string{"issue", "edit", number, "--repo", tr.slug()}, args...)...)
	return err
}

func (tr *triage) summarize(text string) error {
	file, err := os.OpenFile(tr.summary, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(text)
	return err
}

func (tr *triage) process(number string) error {
	fmt.Printf("\n%s\n🔍 Processing issue #%s\n%s\n", rule, number, rule)
	id, err := strconv.Atoi(number)
	if err != nil {
		return err
	}
	out, err := tr.gh("api", "/repos/"+tr.slug()+"/issues/"+number)
	if err != nil {
		return err
	}
	var data issue
	if err := json.Unmarshal(out, &data); err != nil {
		return err
	}
	names := make([]string, len(data.Labels))
	for i, label := range data.Labels {
		names[i] = label.Name
	}
	labels := strings.Join(names, ",")
	if tr.classificationOnly {
		internal, err := tr.author.internal(id)
		if err != nil {
			return err
		}
		return tr.summarize(fmt.Sprintf("App-token classification: issue #%s internal=%t\n", number, internal))
	}
	fmt.Printf("📄 Title: %s\n🏷️  Labels: %s\n", data.Title, cmp.Or(labels, "none"))
	switch {
	case data.State == "closed":
		fmt.Println("⏭️  Skipping: issue is closed")
		return nil
	case strings.Contains(strings.ToLower(labels), "triage:completed"):
		fmt.Println("⏭️  Skipping: already has TRIAGE:COMPLETED label")
		return nil
	case strings.Contains(labels, "triage:manual"):
		fmt.Println("⏭️  Skipping: has triage:manual label")
		return nil
	}
	internal, err := tr.author.internal(id)
	if err != nil {
		return err
	}
	text, err := tr.ask(fmt.Sprintf(promptFormat, tr.slug(), number, data.Title, strings.TrimRight(data.Body, "\n"), cmp.Or(labels, "none")))
	if err != nil || text == "" {
		return err
	}
	result, ok := parseAssessment(text)
	if !ok {
		return tr.invalid(number, text, internal)
	}
	return tr.apply(number, data, labels, result, internal)
}

func (tr *triage) ask(prompt string) (string, error) {
	request, err := json.Marshal(map[string]any{"messages": []any{map[string]any{
		"author": "USER", "fragments": []any{map[string]string{"text": prompt}}, "messageType": "CONTENT",
	}}})
	if err != nil {
		return "", err
	}
	fmt.Println("🤖 Calling Glean Chat API...")
	req, err := http.NewRequest(http.MethodPost, tr.gleanURL, bytes.NewReader(request))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tr.gleanToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := tr.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var chat struct {
		Messages []struct {
			Author, MessageType string
			Fragments           []struct{ Text string }
		}
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		fmt.Printf("❌ Invalid JSON response from Glean API\nResponse: %s\n", body)
		return "", nil
	}
	fmt.Println("✅ Received valid JSON response from Glean API")
	var text strings.Builder
	for _, message := range chat.Messages {
		if message.MessageType == "CONTENT" && message.Author == "GLEAN_AI" {
			for _, fragment := range message.Fragments {
				text.WriteString(fragment.Text)
			}
		}
	}
	answer := strings.TrimRight(text.String(), "\n")
	if answer == "" || answer == "null" {
		fmt.Printf("❌ No response from Glean AI\nResponse: %s\n", body)
		return "", nil
	}
	fmt.Println("✅ Received AI response")
	return answer, nil
}

func extractJSON(text string) string {
	var block []string
	inBlock := false
	for line := range strings.SplitSeq(text, "\n") {
		switch {
		case inBlock:
			block = append(block, line)
			inBlock = line != "```"
		case line == "```json":
			block = append(block, line)
			inBlock = true
		}
	}
	if len(block) > 2 {
		if fenced := strings.TrimRight(strings.Join(block[1:len(block)-1], "\n"), "\n"); fenced != "" {
			return fenced
		}
	}
	if start := strings.Index(text, "{"); start >= 0 {
		var object json.RawMessage
		if json.NewDecoder(strings.NewReader(text[start:])).Decode(&object) == nil {
			return string(object)
		}
	}
	return text
}

func jqString(raw json.RawMessage, fallback string) string {
	var text string
	switch value := string(raw); {
	case value == "" || value == "null" || value == "false":
		return fallback
	case json.Unmarshal(raw, &text) == nil:
		return strings.TrimRight(text, "\n")
	default:
		return value
	}
}

func parseAssessment(text string) (assessment, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(extractJSON(text)), &fields) != nil || fields == nil {
		return assessment{}, false
	}
	var similar []json.RawMessage
	_ = json.Unmarshal(fields["similar_issues"], &similar)
	issues := make([]string, len(similar))
	for i, value := range similar {
		issues[i] = jqString(value, "")
	}
	return assessment{
		workType:       jqString(fields["work_type"], "unknown"),
		kind:           jqString(fields["kind"], ""),
		severity:       jqString(fields["severity"], "unknown"),
		likelihood:     jqString(fields["likelihood"], "low"),
		confidence:     jqString(fields["confidence"], "0.0"),
		rationale:      jqString(fields["rationale"], "No rationale provided"),
		similar:        strings.Join(issues, ", "),
		missingInfo:    jqString(fields["missing_info"], "false"),
		missingDetails: jqString(fields["missing_info_details"], ""),
	}, true
}

func (tr *triage) invalid(number, text string, internal bool) error {
	fmt.Printf("❌ Invalid JSON in AI response\nAI response: %s\n", text)
	if internal {
		if err := tr.edit(number, "--add-label", "triage:manual"); err != nil {
			return err
		}
		return tr.summarize("Issue #" + number + ": assessment failed; manual review needed.\n")
	}
	fmt.Println("💬 Posting error comment and applying triage:manual label...")
	_, _ = tr.gh("issue", "comment", number, "--repo", tr.slug(), "--body", "❌ **Error triaging #"+number+" — manual triage required**\n\nThe AI triage assistant failed to produce a valid response for this issue. Please triage it manually and apply the appropriate labels.\n\nThe `triage:manual` label has been applied to suppress future automated triage attempts.")
	_ = tr.edit(number, "--add-label", "triage:manual")
	return nil
}

func (tr *triage) apply(number string, data issue, labels string, result assessment, internal bool) error {
	fmt.Printf("📊 AI Assessment:\n   Work Type: %s\n   Kind: %s\n   Severity: %s\n   Likelihood: %s\n   Confidence: %s\n", result.workType, cmp.Or(result.kind, "none"), result.severity, result.likelihood, result.confidence)
	if err := tr.summarize(fmt.Sprintf("\n### Issue #%s — suggested priority\nSeverity: %s; likelihood: %s; confidence: %s\n\n%s\n", number, result.severity, result.likelihood, result.confidence, result.rationale)); err != nil {
		return err
	}
	if confidence, err := strconv.ParseFloat(result.confidence, 64); err != nil || confidence < tr.threshold {
		fmt.Printf("⏭️  Skipping: confidence (%s) below threshold (%s)\n", result.confidence, strconv.FormatFloat(tr.threshold, 'f', -1, 64))
		return tr.manual(number, internal)
	}
	if result.severity == "unknown" {
		fmt.Println("⏭️  Skipping: severity is unknown")
		return tr.manual(number, internal)
	}
	if !internal {
		routed, err := tr.post(number, data.User.Login, labels, result)
		if err != nil || routed {
			return err
		}
	}
	if err := tr.label(number, result, internal); err != nil {
		return err
	}
	if proceed, err := tr.move(number, internal); err != nil || !proceed {
		return err
	}
	if internal {
		return tr.edit(number, "--add-label", "TRIAGE:COMPLETED")
	}
	tr.notify(number, data.Title, result.severity)
	fmt.Printf("✅ Triage completed for issue #%s\n", number)
	return nil
}

func (tr *triage) manual(number string, internal bool) error {
	if !internal {
		return nil
	}
	return tr.edit(number, "--add-label", "triage:manual")
}

func capitalize(text string) string {
	first, size := utf8.DecodeRuneInString(text)
	if size == 0 {
		return ""
	}
	return string(unicode.ToUpper(first)) + text[size:]
}

func comment(result assessment, author string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## 🧭 Triage Assessment\n\n| Field | Value |\n|-------|-------|\n| Work type | **%s** |\n| Kind | **%s** |\n| Severity | %s **%s** |\n| Likelihood | %s **%s** |\n| Confidence | **%s** |\n\n### 📝 Rationale\n\n%s",
		result.workType, cmp.Or(result.kind, "unknown"), severityEmoji[result.severity], capitalize(result.severity), likelihoodEmoji[result.likelihood], capitalize(result.likelihood), result.confidence, result.rationale)
	if result.similar != "" && result.similar != "null" {
		fmt.Fprintf(&b, "\n\n### 🔍 Similar Issues\n\n%s", result.similar)
	}
	if result.missingInfo == "true" && result.missingDetails != "" {
		fmt.Fprintf(&b, "\n\n### ⚠️ Missing Information\n\n%s\n\n@%s - Please provide the missing information above.", result.missingDetails, author)
	}
	b.WriteString("\n\n---\n🤖 This issue was automatically triaged by an AI-based system.\n✋ If you believe this assessment is incorrect, update the labels or add the `triage:manual` label for human review.")
	return b.String()
}

func (tr *triage) post(number, author, labels string, result assessment) (bool, error) {
	fmt.Println("💬 Posting triage assessment comment...")
	if _, err := tr.gh("issue", "comment", number, "--repo", tr.slug(), "--body", comment(result, author)); err != nil {
		return false, err
	}
	if result.kind != "feature-request" {
		return false, nil
	}
	if strings.Contains(strings.ToLower(labels), "kind/internal") {
		fmt.Println("ℹ️  kind/internal feature request — normal auto-triage")
		return false, nil
	}
	fmt.Println("📌 Non-internal feature request — routing to PM")
	if err := tr.edit(number, "--add-label", "kind/feature-request", "--add-label", "triage:manual"); err != nil {
		return false, err
	}
	if err := tr.edit(number, "--add-assignee", "mlowe-camunda"); err != nil {
		return false, err
	}
	fmt.Println("✅ Assigned to @mlowe-camunda for PM triage")
	return true, nil
}

func (tr *triage) label(number string, result assessment, internal bool) error {
	fmt.Println("🏷️  Applying labels...")
	if !internal {
		if err := tr.edit(number, "--add-label", "severity/"+result.severity, "--add-label", "likelihood/"+result.likelihood, "--add-label", "TRIAGE:COMPLETED"); err != nil {
			return err
		}
		if result.kind != "" {
			if err := tr.edit(number, "--add-label", "kind/"+result.kind); err != nil {
				return err
			}
		}
		if result.missingInfo != "true" {
			return nil
		}
		return tr.edit(number, "--add-label", "BLOCKER/INFO")
	}
	out, err := tr.gh("issue", "view", number, "--repo", tr.slug(), "--json", "labels", "--jq", ".labels[].name")
	if err != nil {
		return err
	}
	current := "\n" + strings.ToLower(string(out))
	add := func(prefix, value string) error {
		if strings.Contains(current, "\n"+prefix) {
			return nil
		}
		return tr.edit(number, "--add-label", prefix+value)
	}
	if err := add("severity/", result.severity); err != nil {
		return err
	}
	if err := add("likelihood/", result.likelihood); err != nil || result.kind == "" {
		return err
	}
	return add("kind/", result.kind)
}

func (tr *triage) move(number string, internal bool) (bool, error) {
	fmt.Println("📦 Moving issue to Backlog...")
	out, err := tr.gh("api", "graphql", "-f", "query="+projectQuery, "-F", "owner="+tr.owner, "-F", "repo="+tr.repo, "-F", "issueNumber="+number, "-F", "projectNumber="+strconv.Itoa(tr.project))
	if err != nil {
		return false, err
	}
	var result struct {
		Data struct {
			Repository struct {
				Issue struct {
					ProjectItems struct {
						Nodes []struct {
							ID               string
							Project          struct{ Number int }
							FieldValueByName struct{ Name string }
						}
					}
				}
			}
			Organization struct {
				ProjectV2 struct {
					ID    string
					Field struct {
						ID      string
						Options []struct{ ID, Name string }
					}
				}
			}
		}
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return false, err
	}
	project := result.Data.Organization.ProjectV2
	var item, status, option string
	for _, node := range result.Data.Repository.Issue.ProjectItems.Nodes {
		if node.Project.Number == tr.project {
			item, status = node.ID, node.FieldValueByName.Name
		}
	}
	for _, candidate := range project.Field.Options {
		if candidate.Name == backlog {
			option = candidate.ID
		}
	}
	switch {
	case internal && status != "" && status != inbox:
		fmt.Println("Preserving internal issue status: " + status)
	case item == "":
		fmt.Printf("⚠️  Issue not in project #%d, skipping status update\n", tr.project)
		return !internal, nil
	case option == "":
		fmt.Println("⚠️  'Backlog' status option not found in project, skipping status update")
		return !internal, nil
	default:
		if _, err := tr.gh("api", "graphql", "-f", "query="+moveMutation, "-f", "projectId="+project.ID, "-f", "itemId="+item, "-f", "fieldId="+project.Field.ID, "-f", "optionId="+option); err != nil {
			return false, err
		}
		fmt.Println("✅ Moved to Backlog")
	}
	return true, nil
}

func (tr *triage) notify(number, title, severity string) {
	fmt.Println("📣 Sending Slack notification...")
	text := "🎫 [" + strings.Replace(tr.repo, "camunda-platform-", "", 1) + "] <" + tr.serverURL + "/" + tr.slug() + "/issues/" + number + "|#" + number + " " + slackPrefix[severity] + title + ">"
	payload, _ := json.Marshal(map[string]any{"blocks": []any{map[string]any{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": text}}}})
	resp, err := tr.http.Post(tr.slackURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Println("⚠️  Slack notification failed (non-fatal)")
		return
	}
	resp.Body.Close()
}

// Package cache manages CI result caching via GitHub commit statuses.
//
// Each cached result is stored as a commit status on the PR HEAD commit with:
//   - Context: "ci-cache/{version}/{shortname}/{flow}"
//   - Description: "hash:{sha256},ts:{unix_timestamp}"
//   - State: "success" (cached pass) or "pending" (invalidated)
//
// The cache supports TTL-based expiration: results older than the configured
// TTL are treated as cache misses even if the content hash matches.
package cache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultTTL is the default time-to-live for cached results.
const DefaultTTL = 24 * time.Hour

// StatusContext returns the commit status context string for a scenario.
func StatusContext(version, shortname, flow string) string {
	return fmt.Sprintf("ci-cache/%s/%s/%s", version, shortname, flow)
}

// Entry represents a cached CI result.
type Entry struct {
	Hash      string
	Timestamp time.Time
	State     string // "success", "pending", "failure", "error"
}

// ParseDescription extracts hash and timestamp from a status description.
// Format: "hash:{sha256},ts:{unix_timestamp}"
func ParseDescription(desc string) (Entry, error) {
	entry := Entry{}
	parts := strings.Split(desc, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "hash":
			entry.Hash = kv[1]
		case "ts":
			ts, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return entry, fmt.Errorf("parsing timestamp %q: %w", kv[1], err)
			}
			entry.Timestamp = time.Unix(ts, 0)
		}
	}
	if entry.Hash == "" {
		return entry, fmt.Errorf("no hash found in description %q", desc)
	}
	return entry, nil
}

// FormatDescription creates a status description from hash and timestamp.
func FormatDescription(contentHash string, ts time.Time) string {
	return fmt.Sprintf("hash:%s,ts:%d", contentHash, ts.Unix())
}

// GitHubClient wraps the GitHub API for commit status operations.
type GitHubClient struct {
	Token      string
	Repository string // "owner/repo"
	HTTPClient *http.Client
}

// NewGitHubClient creates a client from environment variables.
// Requires GITHUB_TOKEN and GITHUB_REPOSITORY.
func NewGitHubClient() (*GitHubClient, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		// Also accept GH_TOKEN as fallback.
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN or GH_TOKEN environment variable is required")
	}

	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY environment variable is required")
	}

	return &GitHubClient{
		Token:      token,
		Repository: repo,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// commitStatus represents the GitHub API response for a commit status.
type commitStatus struct {
	State       string `json:"state"`
	Context     string `json:"context"`
	Description string `json:"description"`
	TargetURL   string `json:"target_url,omitempty"`
}

// GetStatuses retrieves all commit statuses for a given SHA.
// Returns only statuses with the "ci-cache/" prefix.
func (c *GitHubClient) GetStatuses(sha string) ([]commitStatus, error) {
	var allStatuses []commitStatus
	page := 1

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s/statuses?per_page=100&page=%d",
			c.Repository, sha, page)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "token "+c.Token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching statuses: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
		}

		var statuses []commitStatus
		if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		if len(statuses) == 0 {
			break
		}

		for _, s := range statuses {
			if strings.HasPrefix(s.Context, "ci-cache/") {
				allStatuses = append(allStatuses, s)
			}
		}

		page++
	}

	return allStatuses, nil
}

// SetStatus creates or updates a commit status.
func (c *GitHubClient) SetStatus(sha, state, context, description, targetURL string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/statuses/%s", c.Repository, sha)

	payload := commitStatus{
		State:       state,
		Context:     context,
		Description: description,
		TargetURL:   targetURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("posting status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Check verifies whether a scenario result is cached and still valid.
// Returns true if:
//  1. A "success" status exists for this context on the given SHA
//  2. The content hash in the description matches the current hash
//  3. The timestamp is within the TTL window
func Check(statuses []commitStatus, context, currentHash string, ttl time.Duration) bool {
	// GitHub returns statuses in reverse chronological order.
	// Find the most recent status for this context.
	for _, s := range statuses {
		if s.Context != context {
			continue
		}
		if s.State != "success" {
			return false // explicitly invalidated or failed
		}

		entry, err := ParseDescription(s.Description)
		if err != nil {
			return false // malformed description
		}

		if entry.Hash != currentHash {
			return false // content changed
		}

		if ttl > 0 && time.Since(entry.Timestamp) > ttl {
			return false // expired
		}

		return true
	}
	return false // no status found
}

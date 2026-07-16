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

package gate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGitHubClient(t *testing.T, srv *httptest.Server) *GitHubClient {
	t.Helper()
	return &GitHubClient{
		Token:      "test-token",
		Repository: "camunda/camunda-platform-helm",
		HTTPClient: srv.Client(),
		baseURL:    srv.URL,
	}
}

func TestGitHubClient_GetPullRequest(t *testing.T) {
	tests := []struct {
		name         string
		changedFiles int
	}{
		{name: "returns changed_files from JSON", changedFiles: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/repos/camunda/camunda-platform-helm/pulls/7", r.URL.Path)
				assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
				assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

				w.Header().Set("Content-Type", "application/json")
				require.NoError(t, json.NewEncoder(w).Encode(map[string]int{"changed_files": tt.changedFiles}))
			}))
			defer srv.Close()

			meta, err := testGitHubClient(t, srv).GetPullRequest(7)
			require.NoError(t, err)
			assert.Equal(t, PRMeta{ChangedFiles: tt.changedFiles}, meta)
		})
	}
}

func TestGitHubClient_ListPullRequestFiles_pagination(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "paginates and maps renames"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/repos/camunda/camunda-platform-helm/pulls/9/files", r.URL.Path)
				assert.Equal(t, "100", r.URL.Query().Get("per_page"))
				assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
				assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

				page := r.URL.Query().Get("page")
				w.Header().Set("Content-Type", "application/json")

				switch page {
				case "1":
					batch := make([]pullRequestFileResponse, 100)
					for i := range batch {
						batch[i] = pullRequestFileResponse{Filename: fmt.Sprintf("file-%d.txt", i)}
					}
					require.NoError(t, json.NewEncoder(w).Encode(batch))
				case "2":
					require.NoError(t, json.NewEncoder(w).Encode([]pullRequestFileResponse{
						{Filename: "file-100.txt"},
						{Filename: "renamed.txt", PreviousFilename: "old-name.txt"},
						{Filename: "file-102.txt"},
					}))
				case "3":
					require.NoError(t, json.NewEncoder(w).Encode([]pullRequestFileResponse{}))
				default:
					t.Errorf("unexpected page query: %q", page)
					http.Error(w, "unexpected page", http.StatusBadRequest)
				}
			}))
			defer srv.Close()

			files, err := testGitHubClient(t, srv).ListPullRequestFiles(9)
			require.NoError(t, err)
			require.Len(t, files, 103)
			assert.Equal(t, "file-0.txt", files[0].Filename)
			assert.Equal(t, "renamed.txt", files[101].Filename)
			assert.Equal(t, "old-name.txt", files[101].PreviousFilename)
			assert.Equal(t, "file-102.txt", files[102].Filename)
		})
	}
}

func TestGitHubClient_getJSON_errors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "non-200 status",
			statusCode: http.StatusForbidden,
			body:       "rate limit exceeded",
			wantErr:    "GitHub API returned 403",
		},
		{
			name:       "malformed JSON",
			statusCode: http.StatusOK,
			body:       "{not-json",
			wantErr:    "decoding response:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := testGitHubClient(t, srv).GetPullRequest(1)
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.wantErr), "error %q should contain %q", err.Error(), tt.wantErr)
		})
	}
}

func TestGitHubClient_GetPullRequestHeadSHA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/repos/camunda/camunda-platform-helm/pulls/7", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"head": map[string]string{"sha": "abc123"},
		}))
	}))
	defer srv.Close()

	sha, err := testGitHubClient(t, srv).GetPullRequestHeadSHA(7)
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)
}

func TestGitHubClient_ListReviews_pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/repos/camunda/camunda-platform-helm/pulls/9/reviews", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))

		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case "1":
			require.NoError(t, json.NewEncoder(w).Encode([]reviewResponse{
				{ID: 1, User: struct {
					Login string `json:"login"`
				}{Login: "github-actions[bot]"}, CommitID: "sha1", State: "APPROVED"},
				{ID: 2, User: struct {
					Login string `json:"login"`
				}{Login: "distro-ci[bot]"}, CommitID: "sha2", State: "APPROVED"},
			}))
		case "2":
			require.NoError(t, json.NewEncoder(w).Encode([]reviewResponse{}))
		default:
			t.Errorf("unexpected page query: %q", page)
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	reviews, err := testGitHubClient(t, srv).ListReviews(9)
	require.NoError(t, err)
	require.Len(t, reviews, 2)
	assert.Equal(t, Review{ID: 1, UserLogin: "github-actions[bot]", CommitID: "sha1", State: "APPROVED"}, reviews[0])
	assert.Equal(t, Review{ID: 2, UserLogin: "distro-ci[bot]", CommitID: "sha2", State: "APPROVED"}, reviews[1])
}

func TestGitHubClient_CreateReview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/repos/camunda/camunda-platform-helm/pulls/5/reviews", r.URL.Path)
		assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{
			"commit_id": "abc123",
			"event":     "APPROVE",
			"body":      "Auto-approved",
		}, body)

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	err := testGitHubClient(t, srv).CreateReview(5, "abc123", "APPROVE", "Auto-approved")
	require.NoError(t, err)
}

func TestGitHubClient_CreateReview_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("commit sha mismatch"))
	}))
	defer srv.Close()

	err := testGitHubClient(t, srv).CreateReview(5, "abc123", "APPROVE", "Auto-approved")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub API returned 422")
}

func TestGitHubClient_DismissReview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/repos/camunda/camunda-platform-helm/pulls/5/reviews/42/dismissals", r.URL.Path)
		assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{
			"message": "stale approval",
			"event":   "DISMISS",
		}, body)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := testGitHubClient(t, srv).DismissReview(5, 42, "stale approval")
	require.NoError(t, err)
}

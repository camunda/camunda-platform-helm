package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type capturedUpload struct {
	object      string
	bucket      string
	body        string
	authz       string
	contentType string
}

// newUploadRecorder stands in for the Cloud Storage JSON API and records what a
// real upload would have received.
func newUploadRecorder(t *testing.T, status int) (*httptest.Server, func() []capturedUpload) {
	t.Helper()

	var (
		mu   sync.Mutex
		seen []capturedUpload
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			if _, err := r.Body.Read(body); err != nil && err.Error() != "EOF" {
				t.Errorf("read body: %v", err)
			}
		}
		// /upload/storage/v1/b/<bucket>/o
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		bucket := ""
		if len(parts) >= 5 {
			bucket = parts[4]
		}
		mu.Lock()
		seen = append(seen, capturedUpload{
			object:      r.URL.Query().Get("name"),
			bucket:      bucket,
			body:        string(body),
			authz:       r.Header.Get("Authorization"),
			contentType: r.Header.Get("Content-Type"),
		})
		mu.Unlock()

		w.WriteHeader(status)
		if status > 299 {
			fmt.Fprint(w, `{"error":{"message":"denied"}}`)
			return
		}
		fmt.Fprint(w, `{"kind":"storage#object"}`)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []capturedUpload {
		mu.Lock()
		defer mu.Unlock()
		out := make([]capturedUpload, len(seen))
		copy(out, seen)
		return out
	}
}

func withUploadEndpoint(t *testing.T, base string) {
	t.Helper()
	previous := gcsUploadBaseURL
	gcsUploadBaseURL = base
	t.Cleanup(func() { gcsUploadBaseURL = previous })
}

func seedTraceTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for path, body := range map[string]string{
		"login-flow/trace.zip":  "trace-one",
		"upload-flow/trace.zip": "trace-two",
		"login-flow/stdout.txt": "not-a-trace",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
}

func TestUploadArtifactsUploadsOnlyMatchingFiles(t *testing.T) {
	srv, captured := newUploadRecorder(t, http.StatusOK)
	withUploadEndpoint(t, srv.URL)

	var out strings.Builder
	n, err := uploadArtifacts(context.Background(), artifactUploadOptions{
		Bucket:      "camunda-ci-e2e-traces",
		Source:      seedTraceTree(t),
		Prefix:      "123/1/eske-install",
		Pattern:     "trace.zip",
		Token:       "test-token",
		ContentType: "application/zip",
		Client:      srv.Client(),
	}, &out)
	if err != nil {
		t.Fatalf("uploadArtifacts: %v", err)
	}
	if n != 2 {
		t.Fatalf("uploaded %d objects, want 2", n)
	}

	got := captured()
	if len(got) != 2 {
		t.Fatalf("server saw %d requests, want 2", len(got))
	}

	objects := []string{got[0].object, got[1].object}
	want := []string{
		"123/1/eske-install/login-flow/trace.zip",
		"123/1/eske-install/upload-flow/trace.zip",
	}
	for i := range want {
		if objects[i] != want[i] {
			t.Errorf("object[%d] = %q, want %q", i, objects[i], want[i])
		}
	}
	for _, u := range got {
		if u.bucket != "camunda-ci-e2e-traces" {
			t.Errorf("bucket = %q", u.bucket)
		}
		if u.authz != "Bearer test-token" {
			t.Errorf("authorization = %q", u.authz)
		}
		if u.contentType != "application/zip" {
			t.Errorf("content-type = %q", u.contentType)
		}
		if strings.Contains(u.body, "not-a-trace") {
			t.Errorf("uploaded a non-matching file: %q", u.body)
		}
	}

	// The gs:// URIs are the only record a triage session gets, so they must be
	// printed for every object.
	for _, uri := range want {
		if !strings.Contains(out.String(), "gs://camunda-ci-e2e-traces/"+uri) {
			t.Errorf("missing gs:// URI for %s in:\n%s", uri, out.String())
		}
	}
}

func TestUploadArtifactsMissingSourceIsNotAnError(t *testing.T) {
	srv, captured := newUploadRecorder(t, http.StatusOK)
	withUploadEndpoint(t, srv.URL)

	var out strings.Builder
	n, err := uploadArtifacts(context.Background(), artifactUploadOptions{
		Bucket: "b",
		Source: filepath.Join(t.TempDir(), "never-created"),
		Prefix: "p",
		// A passing run leaves no trace directory behind; that must not fail the
		// job on top of whatever else happened.
		Pattern: "trace.zip",
		Token:   "t",
		Client:  srv.Client(),
	}, &out)
	if err != nil {
		t.Fatalf("expected nil error for a missing source, got %v", err)
	}
	if n != 0 {
		t.Errorf("uploaded %d objects, want 0", n)
	}
	if len(captured()) != 0 {
		t.Errorf("server received requests for a missing source")
	}
}

func TestUploadArtifactsSurfacesServerRejection(t *testing.T) {
	srv, _ := newUploadRecorder(t, http.StatusForbidden)
	withUploadEndpoint(t, srv.URL)

	var out strings.Builder
	_, err := uploadArtifacts(context.Background(), artifactUploadOptions{
		Bucket:  "b",
		Source:  seedTraceTree(t),
		Prefix:  "p",
		Pattern: "trace.zip",
		Token:   "t",
		Client:  srv.Client(),
	}, &out)
	if err == nil {
		t.Fatal("expected an error when the API rejects the upload")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should carry the status, got: %v", err)
	}
}

func TestUploadArtifactsSkipsSymlinks(t *testing.T) {
	srv, captured := newUploadRecorder(t, http.StatusOK)
	withUploadEndpoint(t, srv.URL)

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.zip")
	if err := os.WriteFile(outside, []byte("outside-the-tree"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "trace.zip")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var out strings.Builder
	n, err := uploadArtifacts(context.Background(), artifactUploadOptions{
		Bucket:  "b",
		Source:  root,
		Prefix:  "p",
		Pattern: "trace.zip",
		Token:   "t",
		Client:  srv.Client(),
	}, &out)
	if err != nil {
		t.Fatalf("uploadArtifacts: %v", err)
	}
	if n != 0 {
		t.Errorf("uploaded %d objects via symlink, want 0", n)
	}
	for _, u := range captured() {
		if strings.Contains(u.body, "outside-the-tree") {
			t.Error("followed a symlink out of the source tree")
		}
	}
}

func TestGCSObjectName(t *testing.T) {
	t.Parallel()

	tests := []struct{ prefix, rel, want string }{
		{"123/1/leg", "dir/trace.zip", "123/1/leg/dir/trace.zip"},
		{"/123/1/leg/", "trace.zip", "123/1/leg/trace.zip"},
		{"", "trace.zip", "trace.zip"},
	}
	for _, tt := range tests {
		if got := gcsObjectName(tt.prefix, tt.rel); got != tt.want {
			t.Errorf("gcsObjectName(%q, %q) = %q, want %q", tt.prefix, tt.rel, got, tt.want)
		}
	}
}

// An object name with a space or a '#' must survive the round trip, otherwise the
// upload silently lands under a different key than the log reports.
func TestUploadArtifactsEscapesObjectNames(t *testing.T) {
	srv, captured := newUploadRecorder(t, http.StatusOK)
	withUploadEndpoint(t, srv.URL)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "trace copy#1.zip"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var out strings.Builder
	if _, err := uploadArtifacts(context.Background(), artifactUploadOptions{
		Bucket:  "b",
		Source:  root,
		Prefix:  "p",
		Pattern: "*.zip",
		Token:   "t",
		Client:  srv.Client(),
	}, &out); err != nil {
		t.Fatalf("uploadArtifacts: %v", err)
	}

	got := captured()
	if len(got) != 1 {
		t.Fatalf("saw %d requests, want 1", len(got))
	}
	if got[0].object != "p/trace copy#1.zip" {
		t.Errorf("object = %q, want %q", got[0].object, "p/trace copy#1.zip")
	}
	if _, err := url.Parse(srv.URL + "/x?name=" + url.QueryEscape(got[0].object)); err != nil {
		t.Errorf("object name does not round-trip through a URL: %v", err)
	}
}

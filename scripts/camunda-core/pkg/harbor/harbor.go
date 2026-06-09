// Package harbor performs idempotent Harbor artifact-tag operations (digest
// lookup, add/move/untag, post-verify). The HTTP transport is injectable (the Do
// field) so the tag decisions can be exercised with canned responses and no
// network.
//
// Untagging always uses the deleteTag endpoint
// (`DELETE /artifacts/{reference}/tags/{tag}`), never deleteArtifact
// (`DELETE /artifacts/{tag}`). Per the Harbor v2 API a tag reference passed to
// deleteArtifact removes the ENTIRE artifact and all its other tags; since a
// release artifact's dev/rc/rolling tags all point at one shared artifact, that
// would destroy sibling tags when moving a rolling tag. deleteTag removes only
// the named tag.
package harbor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// retryAttempts / retryDelay: 1 initial try + 3 retries, constant delay between
// attempts. retryDelay is a package var so it can be zeroed in tests.
var (
	retryAttempts = 4
	retryDelay    = 5 * time.Second
)

// Client issues Harbor v2 API tag operations against one repository.
type Client struct {
	APIBase string // e.g. https://registry.camunda.cloud/api/v2.0
	Repo    string // e.g. projects/team-distribution/repositories/camunda-platform
	User    string
	Pass    string
	DryRun  bool

	// Do issues an HTTP request. Defaults to a retrying client; tests inject a
	// fake that records requests and returns canned responses.
	Do func(*http.Request) (*http.Response, error)
	// Log receives human-readable progress lines (default: stderr).
	Log func(string)
}

// New returns a Client with the default retrying transport and stderr logging.
// Credentials are read from HARBOR_REGISTRY_USER / HARBOR_REGISTRY_PASSWORD.
func New(apiBase, repo string, dryRun bool) *Client {
	c := &Client{
		APIBase: apiBase,
		Repo:    repo,
		User:    os.Getenv("HARBOR_REGISTRY_USER"),
		Pass:    os.Getenv("HARBOR_REGISTRY_PASSWORD"),
		DryRun:  dryRun,
	}
	c.Do = c.defaultDo
	c.Log = func(s string) { fmt.Fprintln(os.Stderr, s) }
	return c
}

func (c *Client) logf(format string, a ...any) {
	if c.Log != nil {
		c.Log(fmt.Sprintf(format, a...))
	}
}

func (c *Client) artifactURL(ref string) string {
	return fmt.Sprintf("%s/%s/artifacts/%s", c.APIBase, c.Repo, ref)
}

func (c *Client) tagsURL(ref string) string { return c.artifactURL(ref) + "/tags" }
func (c *Client) tagURL(ref, tag string) string {
	return c.artifactURL(ref) + "/tags/" + tag
}

// defaultDo issues the request with basic auth, retrying on a transport error
// or any HTTP >= 400, with a constant delay between attempts.
func (c *Client) defaultDo(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		// The body must be re-readable across retries; callers pass bytes.Reader
		// via GetBody so net/http can rewind it.
		if req.GetBody != nil {
			b, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = b
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode < 400 {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(body))
		}
		if attempt < retryAttempts {
			time.Sleep(retryDelay)
		}
	}
	return nil, lastErr
}

func (c *Client) newRequest(method, url string, body []byte) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		b := body
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }
	}
	if c.User != "" || c.Pass != "" {
		req.SetBasicAuth(c.User, c.Pass)
	}
	return req, nil
}

// artifact is the subset of the Harbor artifact object we read.
type artifact struct {
	Digest string `json:"digest"`
}

// Digest returns the artifact digest for a reference (tag or digest), failing if
// absent.
func (c *Client) Digest(ref string) (string, error) {
	d, err := c.digest(ref)
	if err != nil {
		return "", err
	}
	if d == "" {
		return "", fmt.Errorf("no digest found for reference %q", ref)
	}
	return d, nil
}

// TagDigest returns the digest the given tag currently points at, or "" if the
// tag does not exist (any lookup error is treated as absent).
func (c *Client) TagDigest(tag string) string {
	d, err := c.digest(tag)
	if err != nil {
		return ""
	}
	return d
}

func (c *Client) digest(ref string) (string, error) {
	req, err := c.newRequest(http.MethodGet, c.artifactURL(ref), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GET %s: HTTP %d", c.artifactURL(ref), resp.StatusCode)
	}
	var a artifact
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return "", fmt.Errorf("decode artifact %q: %w", ref, err)
	}
	return a.Digest, nil
}

// postTag POSTs a tag onto the artifact identified by digest and returns the
// HTTP status code. In dry-run mode it logs the intended request and returns 201.
func (c *Client) postTag(digest, tag string) (int, error) {
	body, _ := json.Marshal(map[string]string{"name": tag})
	if c.DryRun {
		c.logf("[dry-run] POST %s %s", c.tagsURL(digest), body)
		return http.StatusCreated, nil
	}
	req, err := c.newRequest(http.MethodPost, c.tagsURL(digest), body)
	if err != nil {
		return 0, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// AddTag adds a tag to the artifact identified by digest, failing on any non-2xx
// response.
func (c *Client) AddTag(digest, tag string) error {
	c.logf("Adding tag: %s", tag)
	code, err := c.postTag(digest, tag)
	if err != nil {
		return fmt.Errorf("add tag %q: %w", tag, err)
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return fmt.Errorf("add tag %q: HTTP %d", tag, code)
	}
	return nil
}

// DeleteTag removes a single tag via the deleteTag endpoint
// (DELETE /artifacts/{reference}/tags/{tag}). It deletes only the tag, never the
// artifact. When ignoreMissing is set, a non-2xx response is tolerated.
func (c *Client) DeleteTag(ref, tag string, ignoreMissing bool) error {
	c.logf("Removing tag if present: %s", tag)
	if c.DryRun {
		c.logf("[dry-run] DELETE %s", c.tagURL(ref, tag))
		return nil
	}
	req, err := c.newRequest(http.MethodDelete, c.tagURL(ref, tag), nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		if ignoreMissing {
			return nil
		}
		return fmt.Errorf("delete tag %q: %w", tag, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 && !ignoreMissing {
		return fmt.Errorf("delete tag %q: HTTP %d", tag, resp.StatusCode)
	}
	return nil
}

// EnsureTag idempotently points tag at the artifact identified by digest. If the
// tag already resolves to that digest it is a no-op; otherwise the stale tag is
// untagged (deleteTag endpoint) and re-added. After adding, the tag is re-read to
// confirm it resolves to the expected digest (this also covers a 409 from a
// concurrent add).
func (c *Client) EnsureTag(digest, tag string, mustMove bool) error {
	existing := c.TagDigest(tag)
	if existing != "" {
		if existing == digest {
			c.logf("✅ Tag already present and up-to-date: %s -> %s", tag, digest)
			return nil
		}
		if mustMove {
			c.logf("ℹ️ Tag exists but points elsewhere; will move: %s (%s -> %s)", tag, existing, digest)
		} else {
			c.logf("ℹ️ Tag exists but points elsewhere; will overwrite: %s (%s -> %s)", tag, existing, digest)
		}
		// Untag the stale tag (deleteTag endpoint); tolerate a missing tag.
		if err := c.DeleteTag(tag, tag, true); err != nil {
			return err
		}
	}

	code, err := c.postTag(digest, tag)
	// A clean non-2xx (other than 409 Conflict) is a hard failure. A 409, or a
	// transport/HTTP error surfaced by the retrying transport, is not fatal on
	// its own: the tag may already have been added (concurrently) to the right
	// digest, so fall through to the post-add verification, which decides.
	if err == nil && code != http.StatusOK && code != http.StatusCreated && code != http.StatusConflict {
		return fmt.Errorf("failed to add tag %q: HTTP %d", tag, code)
	}

	if c.DryRun {
		return nil
	}
	// Verify the tag resolves to the desired digest. This is what makes a 409
	// tolerable: success iff the tag now points at the digest.
	final := c.TagDigest(tag)
	if final == digest {
		return nil
	}
	if err != nil {
		return fmt.Errorf("add tag %q: %w", tag, err)
	}
	got := final
	if got == "" {
		got = "<missing>"
	}
	return fmt.Errorf("tag %q does not point to expected digest after update: expected %s, got %s", tag, digest, got)
}

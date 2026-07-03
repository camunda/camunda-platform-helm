// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package harbor

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// jsonResp is a 200 response carrying a digest, or a 404 when digest is "".
func jsonResp(digest string) (*http.Response, error) {
	if digest == "" {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"digest":"` + digest + `"}`)), Header: make(http.Header)}, nil
}

func ensureClient(do func(*http.Request) (*http.Response, error)) *Client {
	return &Client{APIBase: "https://r/api/v2.0", Repo: "projects/p/repositories/r", Do: do, Log: func(string) {}}
}

// The retrying transport surfaces a 409 (and other 4xx) as a Go error. EnsureTag
// must tolerate that as long as the tag ends up on the target digest — matching
// the original bash, which accepted HTTP 409 and re-verified the digest.
func TestEnsureTagToleratesAddErrorWhenVerifyOK(t *testing.T) {
	digest := "sha256:newone"
	gets := 0
	c := ensureClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			return nil, fmt.Errorf("HTTP 409: conflict") // what defaultDo returns for a 409
		}
		gets++
		if gets == 1 {
			return jsonResp("") // existing lookup: tag absent
		}
		return jsonResp(digest) // post-add verify: resolves to target
	})
	if err := c.EnsureTag(digest, "13.4.0-rc", false); err != nil {
		t.Fatalf("EnsureTag must tolerate a 409 when the tag verifies to the digest: %v", err)
	}
}

func TestEnsureTagFailsWhenAddErrorsAndVerifyWrong(t *testing.T) {
	c := ensureClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			return nil, fmt.Errorf("HTTP 500: boom")
		}
		return jsonResp("") // never resolves
	})
	if err := c.EnsureTag("sha256:want", "13.4.0-rc", false); err == nil {
		t.Fatal("EnsureTag must fail when add errors and the tag does not verify")
	}
}

type call struct {
	method string
	url    string
	body   string
}

// fakeTransport records requests and answers them via handler.
type fakeTransport struct {
	calls   []call
	handler func(method, url, body string) (int, string)
}

func (f *fakeTransport) do(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	f.calls = append(f.calls, call{req.Method, req.URL.String(), body})
	code, respBody := f.handler(req.Method, req.URL.String(), body)
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(respBody)),
		Header:     make(http.Header),
	}, nil
}

func newClient(f *fakeTransport) *Client {
	return &Client{
		APIBase: "https://registry.camunda.cloud/api/v2.0",
		Repo:    "projects/team-distribution/repositories/camunda-platform",
		Do:      f.do,
		Log:     func(string) {},
	}
}

func TestEnsureTagNoOp(t *testing.T) {
	digest := "sha256:aaa"
	f := &fakeTransport{handler: func(method, url, body string) (int, string) {
		if method == http.MethodGet {
			return 200, `{"digest":"` + digest + `"}`
		}
		t.Fatalf("unexpected %s %s — no mutation expected for up-to-date tag", method, url)
		return 0, ""
	}}
	c := newClient(f)
	if err := c.EnsureTag(digest, "13.4.0-rc", false); err != nil {
		t.Fatalf("EnsureTag: %v", err)
	}
	for _, cl := range f.calls {
		if cl.method != http.MethodGet {
			t.Errorf("expected only GET calls, got %s %s", cl.method, cl.url)
		}
	}
}

func TestEnsureTagMoveUsesTagEndpoint(t *testing.T) {
	digest := "sha256:new"
	old := "sha256:old"
	get := 0
	f := &fakeTransport{handler: func(method, url, body string) (int, string) {
		switch method {
		case http.MethodGet:
			get++
			if get == 1 {
				return 200, `{"digest":"` + old + `"}` // existing points elsewhere
			}
			return 200, `{"digest":"` + digest + `"}` // post-verify
		case http.MethodDelete:
			return 200, ""
		case http.MethodPost:
			return 201, ""
		}
		return 0, ""
	}}
	c := newClient(f)
	if err := c.EnsureTag(digest, "13-rc-latest", true); err != nil {
		t.Fatalf("EnsureTag: %v", err)
	}

	var del, post *call
	for i := range f.calls {
		switch f.calls[i].method {
		case http.MethodDelete:
			del = &f.calls[i]
		case http.MethodPost:
			post = &f.calls[i]
		}
	}
	if del == nil {
		t.Fatal("expected a DELETE to untag the stale tag")
	}
	// The bug fix: the stale tag must be removed via the tag endpoint
	// (/artifacts/{ref}/tags/{tag}), NOT the artifact endpoint
	// (/artifacts/{ref}) which would delete the whole shared artifact.
	wantDel := "https://registry.camunda.cloud/api/v2.0/projects/team-distribution/repositories/camunda-platform/artifacts/13-rc-latest/tags/13-rc-latest"
	if del.url != wantDel {
		t.Errorf("delete URL = %q\n want %q (tag endpoint)", del.url, wantDel)
	}
	if !strings.HasSuffix(del.url, "/tags/13-rc-latest") {
		t.Errorf("delete must hit the tag endpoint, got %q", del.url)
	}
	if post == nil {
		t.Fatal("expected a POST to add the moved tag")
	}
	wantPost := "https://registry.camunda.cloud/api/v2.0/projects/team-distribution/repositories/camunda-platform/artifacts/" + digest + "/tags"
	if post.url != wantPost {
		t.Errorf("post URL = %q want %q", post.url, wantPost)
	}
	if post.body != `{"name":"13-rc-latest"}` {
		t.Errorf("post body = %q want {\"name\":\"13-rc-latest\"}", post.body)
	}
}

func TestEnsureTagFreshAdd(t *testing.T) {
	digest := "sha256:fresh"
	get := 0
	f := &fakeTransport{handler: func(method, url, body string) (int, string) {
		switch method {
		case http.MethodGet:
			get++
			if get == 1 {
				return 404, `{}` // tag absent
			}
			return 200, `{"digest":"` + digest + `"}`
		case http.MethodPost:
			return 201, ""
		case http.MethodDelete:
			t.Fatal("no DELETE expected when tag is absent")
		}
		return 0, ""
	}}
	c := newClient(f)
	if err := c.EnsureTag(digest, "13.4.0-rc", false); err != nil {
		t.Fatalf("EnsureTag: %v", err)
	}
}

func TestEnsureTagVerifyMismatch(t *testing.T) {
	digest := "sha256:want"
	get := 0
	f := &fakeTransport{handler: func(method, url, body string) (int, string) {
		switch method {
		case http.MethodGet:
			get++
			if get == 1 {
				return 404, `{}`
			}
			return 200, `{"digest":"sha256:other"}` // verify finds wrong digest
		case http.MethodPost:
			return 201, ""
		}
		return 0, ""
	}}
	c := newClient(f)
	if err := c.EnsureTag(digest, "13.4.0-rc", false); err == nil {
		t.Fatal("expected verify mismatch error")
	}
}

func TestAddTag(t *testing.T) {
	f := &fakeTransport{handler: func(method, url, body string) (int, string) { return 201, "" }}
	c := newClient(f)
	if err := c.AddTag("sha256:x", "13.4.0-dev-abc1234"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	last := f.calls[len(f.calls)-1]
	if last.method != http.MethodPost || last.body != `{"name":"13.4.0-dev-abc1234"}` {
		t.Errorf("unexpected add call: %+v", last)
	}

	ff := &fakeTransport{handler: func(method, url, body string) (int, string) { return 500, "boom" }}
	if err := newClient(ff).AddTag("sha256:x", "t"); err == nil {
		t.Error("expected error on non-2xx add")
	}
}

func TestDeleteTagIgnoreMissing(t *testing.T) {
	f := &fakeTransport{handler: func(method, url, body string) (int, string) { return 404, "" }}
	c := newClient(f)
	if err := c.DeleteTag("13-dev-latest", "13-dev-latest", true); err != nil {
		t.Fatalf("DeleteTag ignoreMissing should tolerate 404: %v", err)
	}
	if err := c.DeleteTag("x", "x", false); err == nil {
		t.Error("expected error on 404 when not ignoring")
	}
	last := f.calls[len(f.calls)-1]
	if !strings.HasSuffix(last.url, "/artifacts/x/tags/x") {
		t.Errorf("delete must use tag endpoint, got %q", last.url)
	}
}

func TestDigestFatalOnEmpty(t *testing.T) {
	f := &fakeTransport{handler: func(method, url, body string) (int, string) { return 200, `{"digest":""}` }}
	if _, err := newClient(f).Digest("13.4.0-dev-abc1234"); err == nil {
		t.Error("expected error when digest is empty")
	}

	g := &fakeTransport{handler: func(method, url, body string) (int, string) { return 200, `{"digest":"sha256:ok"}` }}
	d, err := newClient(g).Digest("13.4.0-dev-abc1234")
	if err != nil || d != "sha256:ok" {
		t.Errorf("Digest = %q, %v want sha256:ok, nil", d, err)
	}
}

func TestDryRunNoMutation(t *testing.T) {
	get := 0
	f := &fakeTransport{handler: func(method, url, body string) (int, string) {
		if method == http.MethodGet {
			get++
			return 404, `{}`
		}
		t.Fatalf("dry-run must not issue %s %s", method, url)
		return 0, ""
	}}
	c := newClient(f)
	c.DryRun = true
	if err := c.EnsureTag("sha256:x", "13.4.0-rc", false); err != nil {
		t.Fatalf("dry-run EnsureTag: %v", err)
	}
	for _, cl := range f.calls {
		if cl.method != http.MethodGet {
			t.Errorf("dry-run issued mutation: %s %s", cl.method, cl.url)
		}
	}
}

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

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type releaseSourceFixture struct {
	root, bin, output, log, currentPackage, previousPackage string
	a, b, c, currentDigest, previousDigest                  string
	releases                                                []publishedRelease
	mu                                                      sync.Mutex
	artifacts                                               map[string]string
	requests                                                []string
	retag                                                   bool
	registry                                                map[string]registryBlob
}

type registryBlob struct {
	body      []byte
	mediaType string
	digest    string
}

func sourceTestWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func sourceTestCommand(t *testing.T, name string, args ...string) string {
	t.Helper()
	data, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, data)
	}
	return strings.TrimSpace(string(data))
}

func sourceTestPackage(t *testing.T, path, version, app string) []byte {
	t.Helper()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tw := tar.NewWriter(gz)
	for _, file := range [][2]string{
		{"Chart.yaml", "apiVersion: v2\nname: camunda-platform\nversion: " + version + "\nappVersion: " + app + ".x\n"},
		{"values.yaml", "generated: true\nimage:\n  tag: release-override\n"},
		{"templates/configmap.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: generated\n"},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: "camunda-platform/" + file[0], Mode: 0o644, Size: int64(len(file[1]))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	sourceTestWrite(t, path, data.Bytes(), 0o644)
	return data.Bytes()
}

func sourceTestHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newReleaseSourceFixture(t *testing.T) *releaseSourceFixture {
	t.Helper()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	f := &releaseSourceFixture{
		root: t.TempDir(), bin: t.TempDir(), output: filepath.Join(t.TempDir(), "outputs"),
		log: filepath.Join(t.TempDir(), "calls"), artifacts: map[string]string{}, registry: map[string]registryBlob{},
		currentDigest: "sha256:" + strings.Repeat("b", 64), previousDigest: "sha256:" + strings.Repeat("a", 64),
	}
	for _, path := range []string{".tool-versions", ".github/config/cliff.toml", "scripts/harbor-retry.sh", "scripts/set-prs-version-label.sh"} {
		data, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil {
			t.Fatal(err)
		}
		sourceTestWrite(t, filepath.Join(f.root, path), data, 0o644)
	}
	t.Chdir(f.root)
	sourceTestCommand(t, "git", "init", "-b", "main", "--quiet")
	sourceTestCommand(t, "git", "config", "user.name", "Release tests")
	sourceTestCommand(t, "git", "config", "user.email", "release-tests@example.invalid")
	sourceTestCommand(t, "git", "config", "commit.gpgsign", "false")
	sourceTestCommand(t, "git", "config", "tag.gpgsign", "false")
	sourceTestCommand(t, "git", "config", "core.hooksPath", t.TempDir())
	chart := "charts/camunda-platform-8.9/"
	sourceTestWrite(t, chart+"Chart.yaml", []byte("name: camunda-platform\nversion: 14.11.0\nappVersion: 8.9.x\n"), 0o644)
	sourceTestWrite(t, chart+"templates/change.yaml", []byte("initial\n"), 0o644)
	sourceTestCommand(t, "git", "add", ".")
	sourceTestCommand(t, "git", "commit", "--quiet", "-m", "chore: initial chart")
	f.a = sourceTestCommand(t, "git", "rev-parse", "HEAD")
	sourceTestWrite(t, chart+"templates/change.yaml", []byte("included\n"), 0o644)
	sourceTestCommand(t, "git", "commit", "--quiet", "-am", "fix: included in RC (#101)")
	f.b = sourceTestCommand(t, "git", "rev-parse", "HEAD")
	sourceTestWrite(t, chart+"templates/change.yaml", []byte("late change\n"), 0o644)
	sourceTestCommand(t, "git", "commit", "--quiet", "-am", "fix: merged after RC (#102)")
	f.c = sourceTestCommand(t, "git", "rev-parse", "HEAD")
	remote := t.TempDir()
	sourceTestCommand(t, "git", "init", "--bare", "--quiet", remote)
	sourceTestCommand(t, "git", "remote", "add", "origin", remote)
	sourceTestCommand(t, "git", "push", "--quiet", "-u", "origin", "main")
	f.tag(t, "camunda-platform-8.9-14.11.0", f.b)
	f.previousPackage = filepath.Join(t.TempDir(), "previous.tgz")
	f.currentPackage = filepath.Join(t.TempDir(), "current.tgz")
	previousBytes := sourceTestPackage(t, f.previousPackage, "14.11.0", "8.9")
	sourceTestPackage(t, f.currentPackage, "14.11.1", "8.9")
	f.setArtifact(t, "14.11.0-rc", f.previousDigest, "14.11.0-rc", "14.11.0-dev-"+f.a[:7])
	f.setArtifact(t, "14.11.1-rc", f.currentDigest, "14.11.1-rc", "14-rc-latest", "14.11.1-dev-"+f.b[:7])
	f.setArtifact(t, "14-rc-latest", f.currentDigest, "14.11.1-rc", "14-rc-latest", "14.11.1-dev-"+f.b[:7])
	f.releases = []publishedRelease{sourceTestPublished("camunda-platform-8.9-14.11.0", "14.11.0", sourceTestHash(previousBytes), 1)}
	f.writeReleases(t)
	sourceTestWrite(t, filepath.Join(f.bin, "helm"), []byte(`#!/usr/bin/env bash
set -euo pipefail
printf 'helm %s\n' "$*" >> "$SOURCE_CALLS"
test "$1" = pull
test "$3" = --destination
case "$2" in
  *@"$SOURCE_CURRENT_DIGEST") cp "$SOURCE_CURRENT_PACKAGE" "$4/pulled.tgz" ;;
  *@"$SOURCE_PREVIOUS_DIGEST") cp "$SOURCE_PREVIOUS_PACKAGE" "$4/pulled.tgz" ;;
  *) echo "unexpected mutable or unknown artifact: $2" >&2; exit 1 ;;
esac
`), 0o755)
	sourceTestWrite(t, filepath.Join(f.bin, "gh"), []byte(`#!/usr/bin/env bash
set -euo pipefail
printf 'gh %s\n' "$*" >> "$SOURCE_CALLS"
if [[ "${SOURCE_GH_FAILURE:-}" == true ]]; then echo 'GitHub API failed' >&2; exit 1; fi
case "$1" in
  api) test "$2" = 'repos/{owner}/{repo}/releases?per_page=100'; cat "$SOURCE_RELEASES" ;;
  release)
    test "$2" = download
    case "$3" in
      camunda-platform-8.9-14.11.0) cp "$SOURCE_PREVIOUS_PACKAGE" "$7" ;;
      camunda-platform-8.9-14.11.1) cp "$SOURCE_CURRENT_PACKAGE" "$7" ;;
      *) exit 1 ;;
    esac ;;
  *) echo 'unexpected GitHub mutation' >&2; exit 1 ;;
esac
`), 0o755)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			t.Errorf("unexpected mutation: %s %s", r.Method, r.URL)
			http.Error(w, "mutation forbidden", http.StatusMethodNotAllowed)
			return
		}
		f.requests = append(f.requests, r.URL.String())
		if blob, ok := f.registry[r.URL.Path]; ok {
			w.Header().Set("Content-Type", blob.mediaType)
			w.Header().Set("Content-Length", fmt.Sprint(len(blob.body)))
			w.Header().Set("Docker-Content-Digest", blob.digest)
			if r.Method != http.MethodHead {
				_, _ = w.Write(blob.body)
			}
			return
		}
		body, ok := f.artifacts[filepath.Base(r.URL.Path)]
		if !ok || r.URL.Query().Get("with_tag") != "true" {
			http.Error(w, "unexpected artifact lookup", http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprint(w, body)
		if f.retag && filepath.Base(r.URL.Path) == "14-rc-latest" {
			f.artifacts["14-rc-latest"] = `{"digest":"sha256:changed","tags":[]}`
		}
	}))
	t.Cleanup(server.Close)
	for key, value := range map[string]string{
		"PATH":          f.bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"GITHUB_OUTPUT": f.output, "HARBOR_API": server.URL + "/api/v2.0",
		"HARBOR_REGISTRY_HOST": strings.TrimPrefix(server.URL, "http://"), "HARBOR_REGISTRY_PROJECT": "charts",
		"HARBOR_REPO": "", "HARBOR_REGISTRY_USER": "", "HARBOR_REGISTRY_PASSWORD": "",
		"SOURCE_CALLS": f.log, "SOURCE_RELEASES": filepath.Join(f.root, "releases.json"),
		"SOURCE_CURRENT_PACKAGE": f.currentPackage, "SOURCE_PREVIOUS_PACKAGE": f.previousPackage,
		"SOURCE_CURRENT_DIGEST": f.currentDigest, "SOURCE_PREVIOUS_DIGEST": f.previousDigest,
		"SOURCE_GH_FAILURE": "", "HARBOR_RETRY_MAX": "1", "CI": "true",
	} {
		t.Setenv(key, value)
	}
	return f
}

func sourceTestPublished(tag, version, digest string, day int) publishedRelease {
	release := publishedRelease{Tag: tag, PublishedAt: time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC)}
	release.Assets = append(release.Assets, struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
	}{"camunda-platform-" + version + ".tgz", digest})
	return release
}

func (f *releaseSourceFixture) setArtifact(t *testing.T, ref, digest string, tags ...string) {
	t.Helper()
	names := make([]map[string]string, 0, len(tags))
	for _, name := range tags {
		names = append(names, map[string]string{"name": name})
	}
	body, err := json.Marshal(map[string]any{"digest": digest, "tags": names})
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.artifacts[ref] = string(body)
	f.mu.Unlock()
}

func (f *releaseSourceFixture) tag(t *testing.T, tag, commit string) {
	t.Helper()
	sourceTestCommand(t, "git", "tag", tag, commit)
	sourceTestCommand(t, "git", "push", "--quiet", "origin", "refs/tags/"+tag)
}

func (f *releaseSourceFixture) writeReleases(t *testing.T) {
	t.Helper()
	body, err := json.Marshal([][]publishedRelease{f.releases})
	if err != nil {
		t.Fatal(err)
	}
	sourceTestWrite(t, filepath.Join(f.root, "releases.json"), body, 0o644)
}

func (f *releaseSourceFixture) outputs(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(f.output)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid command output: %q", line)
		}
		out[key] = value
	}
	return out
}

func TestReleaseSourcePrepareUsesArtifactBoundaries(t *testing.T) {
	f := newReleaseSourceFixture(t)
	destination := t.TempDir()
	if err := runReleaseSource([]string{"prepare", "--input-tag", "14.11.1-rc", "--destination", destination}); err != nil {
		t.Fatal(err)
	}
	out := f.outputs(t)
	if out["commit_sha"] != f.b || out["previous_commit_sha"] != f.a || out["dev_tag"] != "14.11.1-dev-"+f.b[:7] {
		t.Fatalf("wrong artifact boundaries: %v", out)
	}
	if out["commit_sha"] == f.c {
		t.Fatal("publishing HEAD must not become the artifact source")
	}
	expected, err := os.ReadFile(f.currentPackage)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(destination, out["package_file"]))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) || out["package_digest"] != sourceTestHash(expected) || out["artifact_digest"] != f.currentDigest {
		t.Fatal("tested package bytes or digests changed")
	}
	commits := sourceTestCommand(t, "git-cliff", out["previous_commit_sha"]+".."+out["commit_sha"],
		"--context", "--config", ".github/config/cliff.toml", "--include-path", "charts/camunda-platform-8.9/**")
	if !strings.Contains(commits, "#101") || strings.Contains(commits, "#102") {
		t.Fatalf("wrong shipped PR set: %s", commits)
	}
}

func TestReleaseSourcePinsRollingArtifact(t *testing.T) {
	f := newReleaseSourceFixture(t)
	f.mu.Lock()
	f.retag = true
	f.mu.Unlock()
	if err := runReleaseSource([]string{"prepare", "--input-tag", "14-rc-latest", "--destination", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	out := f.outputs(t)
	if out["commit_sha"] != f.b || out["artifact_digest"] != f.currentDigest {
		t.Fatalf("rolling tag changed the selected artifact: %v", out)
	}
}

func TestReleaseSourcePreparationFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*testing.T, *releaseSourceFixture)
		want string
	}{
		{"missing source", func(t *testing.T, f *releaseSourceFixture) {
			f.setArtifact(t, "14.11.1-rc", f.currentDigest, "14.11.1-rc")
		}, "no source dev tag"},
		{"ambiguous source", func(t *testing.T, f *releaseSourceFixture) {
			f.setArtifact(t, "14.11.1-rc", f.currentDigest, "14.11.1-rc", "14.11.1-dev-"+f.b[:7], "14.11.1-dev-"+f.c[:7])
		}, "ambiguous source"},
		{"wrong dev version", func(t *testing.T, f *releaseSourceFixture) {
			f.setArtifact(t, "14.11.1-rc", f.currentDigest, "14.11.1-rc", "14.11.0-dev-"+f.b[:7])
		}, "no source dev tag"},
		{"unknown source", func(t *testing.T, f *releaseSourceFixture) {
			f.setArtifact(t, "14.11.1-rc", f.currentDigest, "14.11.1-rc", "14.11.1-dev-"+strings.Repeat("0", 40))
		}, "resolve source commit"},
		{"wrong packaged version", func(t *testing.T, f *releaseSourceFixture) {
			sourceTestPackage(t, f.currentPackage, "14.11.2", "8.9")
		}, "differs from packaged version"},
		{"invalid manifest digest", func(t *testing.T, f *releaseSourceFixture) {
			f.setArtifact(t, "14.11.1-rc", "invalid", "14.11.1-rc", "14.11.1-dev-"+f.b[:7])
		}, "invalid OCI digest"},
		{"previous package mismatch", func(t *testing.T, f *releaseSourceFixture) {
			f.releases[0].Assets[0].Digest = "sha256:" + strings.Repeat("f", 64)
			f.writeReleases(t)
		}, "historical provenance requires review"},
		{"GitHub lookup failure", func(t *testing.T, f *releaseSourceFixture) {
			t.Setenv("SOURCE_GH_FAILURE", "true")
		}, "GitHub API failed"},
		{"null release listing", func(t *testing.T, f *releaseSourceFixture) {
			sourceTestWrite(t, filepath.Join(f.root, "releases.json"), []byte("[null]"), 0o644)
		}, "null page"},
		{"invalid release record", func(t *testing.T, f *releaseSourceFixture) {
			sourceTestWrite(t, filepath.Join(f.root, "releases.json"), []byte("[[{}]]"), 0o644)
		}, "invalid release record"},
		{"conflicting tag", func(t *testing.T, f *releaseSourceFixture) {
			f.tag(t, "camunda-platform-8.9-14.11.1", f.c)
		}, "not artifact source"},
		{"existing release", func(t *testing.T, f *releaseSourceFixture) {
			f.releases = append(f.releases, sourceTestPublished("camunda-platform-8.9-14.11.1", "14.11.1", "", 2))
			f.writeReleases(t)
		}, "already exists"},
		{"predecessor newer than source", func(t *testing.T, f *releaseSourceFixture) {
			f.setArtifact(t, "14.11.0-rc", f.previousDigest, "14.11.0-rc", "14.11.0-dev-"+f.c[:7])
		}, "not an ancestor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReleaseSourceFixture(t)
			tc.edit(t, f)
			destination := t.TempDir()
			err := runReleaseSource([]string{"prepare", "--input-tag", "14.11.1-rc", "--destination", destination})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
			if data, err := os.ReadFile(f.output); err == nil && len(data) != 0 {
				t.Fatalf("failed validation emitted success outputs: %s", data)
			}
			entries, err := os.ReadDir(destination)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("failed validation retained packages: %v", entries)
			}
		})
	}
}

func TestReleaseSourceDryRunUsesHistoricalPredecessor(t *testing.T) {
	f := newReleaseSourceFixture(t)
	currentBytes, err := os.ReadFile(f.currentPackage)
	if err != nil {
		t.Fatal(err)
	}
	f.releases = append(f.releases,
		sourceTestPublished("camunda-platform-8.9-14.11.1", "14.11.1", sourceTestHash(currentBytes), 2),
		sourceTestPublished("camunda-platform-8.9-14.11.2", "14.11.2", "sha256:"+strings.Repeat("c", 64), 3))
	f.writeReleases(t)
	f.tag(t, "camunda-platform-8.9-14.11.1", f.c)
	f.setArtifact(t, "14-rc-dryrun-latest", f.currentDigest, "14-rc-dryrun-latest", "14.11.1-rc-dryrun", "14.11.1-dev-"+f.b[:7])
	if err := runReleaseSource([]string{"prepare", "--input-tag", "14-rc-dryrun-latest", "--destination", t.TempDir(), "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	out := f.outputs(t)
	if out["commit_sha"] != f.b || out["previous_commit_sha"] != f.a {
		t.Fatalf("historical rehearsal selected wrong boundaries: %v", out)
	}
	actual, err := remoteTagCommit(context.Background(), "camunda-platform-8.9-14.11.1")
	if err != nil || actual != f.c {
		t.Fatalf("rehearsal changed a public tag: %s (%v)", actual, err)
	}
}

func TestReleaseSourcePreviousUsesPublishedArtifact(t *testing.T) {
	f := newReleaseSourceFixture(t)
	f.releases[0].Assets[0].Digest = ""
	f.writeReleases(t)
	if err := runReleaseSource([]string{"previous", "--chart-dir", "charts/camunda-platform-8.9", "--source", f.c}); err != nil {
		t.Fatal(err)
	}
	if got := f.outputs(t)["previous_commit_sha"]; got != f.a {
		t.Fatalf("used incorrect public tag instead of previous artifact: got %s want %s", got, f.a)
	}
}

func TestReleaseSourcePredecessorPublicationOrder(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(fmt.Sprint(ambiguous), func(t *testing.T) {
			f := newReleaseSourceFixture(t)
			latest := f.releases[0]
			latest.PublishedAt = latest.PublishedAt.Add(24 * time.Hour)
			older := sourceTestPublished("camunda-platform-8.9-14.10.0", "14.10.0", "", 1)
			other := sourceTestPublished("camunda-platform-8.9-14.10.1", "14.10.1", "", 1)
			if ambiguous {
				other.PublishedAt = latest.PublishedAt
			}
			f.releases = []publishedRelease{older, other, latest}
			f.writeReleases(t)
			err := runReleaseSource([]string{"previous", "--chart-dir", "charts/camunda-platform-8.9", "--source", f.c})
			if ambiguous {
				if err == nil || !strings.Contains(err.Error(), "ambiguous predecessor") {
					t.Fatalf("ambiguous latest release was accepted: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := f.outputs(t)["previous_commit_sha"]; got != f.a {
				t.Fatalf("wrong predecessor: %s", got)
			}
		})
	}
}

func TestReleaseSourcePublishedVerification(t *testing.T) {
	for _, tc := range []struct {
		name, target  string
		missingDigest bool
		wrongDigest   bool
		wantError     bool
	}{
		{"correct", "base", false, false, false},
		{"download when digest absent", "base", true, false, false},
		{"wrong tag", "head", false, false, true},
		{"wrong package", "base", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReleaseSourceFixture(t)
			target := f.b
			if tc.target == "head" {
				target = f.c
			}
			f.tag(t, "camunda-platform-8.9-14.11.1", target)
			digest, err := fileSHA256(f.currentPackage)
			if err != nil {
				t.Fatal(err)
			}
			if tc.missingDigest {
				digest = ""
			}
			if tc.wrongDigest {
				digest = "sha256:" + strings.Repeat("f", 64)
			}
			f.releases = append(f.releases, sourceTestPublished("camunda-platform-8.9-14.11.1", "14.11.1", digest, 2))
			f.writeReleases(t)
			err = runReleaseSource([]string{"verify", "--package", f.currentPackage, "--source", f.b})
			if (err != nil) != tc.wantError {
				t.Fatalf("verification error=%v, want error=%v", err, tc.wantError)
			}
		})
	}
}

func TestReleaseSourceExplicitBootstrap(t *testing.T) {
	for _, withTag := range []bool{true, false} {
		t.Run(fmt.Sprint(withTag), func(t *testing.T) {
			f := newReleaseSourceFixture(t)
			sourceTestWrite(t, "charts/camunda-platform-8.11/Chart.yaml", []byte("name: camunda-platform\nversion: 16.0.0-alpha0\nappVersion: 8.11.x\n"), 0o644)
			sourceTestCommand(t, "git", "add", ".")
			sourceTestCommand(t, "git", "commit", "--quiet", "-m", "feat: initialize chart")
			base := sourceTestCommand(t, "git", "rev-parse", "HEAD")
			sourceTestCommand(t, "git", "push", "--quiet", "origin", "main")
			if withTag {
				f.tag(t, "camunda-platform-8.11-16.0.0-alpha0", base)
			}
			sourceTestPackage(t, f.currentPackage, "16.0.0-alpha1", "8.11")
			f.setArtifact(t, "16.0.0-alpha1-rc", f.currentDigest, "16.0.0-alpha1-rc", "16.0.0-alpha1-dev-"+base[:7])
			err := runReleaseSource([]string{"prepare", "--input-tag", "16.0.0-alpha1-rc", "--destination", t.TempDir()})
			if !withTag {
				if err == nil || !strings.Contains(err.Error(), "explicit bootstrap tag") {
					t.Fatalf("expected missing-bootstrap failure, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := f.outputs(t)["previous_commit_sha"]; got != base {
				t.Fatalf("bootstrap=%s want %s", got, base)
			}
		})
	}
}

func TestReleaseSourceRealHelmDigestPull(t *testing.T) {
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Fatal(err)
	}
	f := newReleaseSourceFixture(t)
	sourceTestWrite(t, filepath.Join(f.root, "scripts/harbor-retry.sh"), []byte(`harbor_helm_pull() { "$SOURCE_REAL_HELM" pull --plain-http "$@"; }`), 0o644)
	t.Setenv("SOURCE_REAL_HELM", helm)
	t.Setenv("HELM_CONFIG_HOME", t.TempDir())
	t.Setenv("HELM_CACHE_HOME", t.TempDir())
	t.Setenv("HELM_DATA_HOME", t.TempDir())
	t.Setenv("HELM_REGISTRY_CONFIG", filepath.Join(t.TempDir(), "registry.json"))
	current := f.serveOCI(t, f.currentPackage, "14.11.1")
	previous := f.serveOCI(t, f.previousPackage, "14.11.0")
	f.setArtifact(t, "14.11.1-rc", current, "14.11.1-rc", "14.11.1-dev-"+f.b[:7])
	f.setArtifact(t, "14.11.0-rc", previous, "14.11.0-rc", "14.11.0-dev-"+f.a[:7])
	if err := runReleaseSource([]string{"prepare", "--input-tag", "14.11.1-rc", "--destination", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if got := f.outputs(t)["artifact_digest"]; got != current {
		t.Fatalf("pulled digest=%s want %s", got, current)
	}
}

func (f *releaseSourceFixture) serveOCI(t *testing.T, packageFile, version string) string {
	t.Helper()
	pkg, err := os.ReadFile(packageFile)
	if err != nil {
		t.Fatal(err)
	}
	config := []byte(fmt.Sprintf(`{"name":"camunda-platform","version":%q,"apiVersion":"v2","appVersion":"8.9.x"}`, version))
	manifest := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.cncf.helm.config.v1+json","digest":%q,"size":%d},"layers":[{"mediaType":"application/vnd.cncf.helm.chart.content.v1.tar+gzip","digest":%q,"size":%d}]}`,
		sourceTestHash(config), len(config), sourceTestHash(pkg), len(pkg)))
	digest := sourceTestHash(manifest)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registry["/v2/"] = registryBlob{[]byte("{}"), "application/json", ""}
	f.registry["/v2/charts/camunda-platform/manifests/"+digest] = registryBlob{manifest, "application/vnd.oci.image.manifest.v1+json", digest}
	f.registry["/v2/charts/camunda-platform/blobs/"+sourceTestHash(config)] = registryBlob{config, "application/vnd.cncf.helm.config.v1+json", sourceTestHash(config)}
	f.registry["/v2/charts/camunda-platform/blobs/"+sourceTestHash(pkg)] = registryBlob{pkg, "application/vnd.cncf.helm.chart.content.v1.tar+gzip", sourceTestHash(pkg)}
	return digest
}

func TestReleaseSourceRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"verify", "--source", "HEAD"}, {"verify", "--package", "x", "--source", "abc1234"}} {
		if err := runReleaseSource(args); err == nil {
			t.Fatalf("expected argument error for %v", args)
		}
	}
	if _, err := resolveSourceCommit(context.Background(), "HEAD"); err == nil {
		t.Fatal("HEAD must not be accepted as explicit provenance")
	}
}

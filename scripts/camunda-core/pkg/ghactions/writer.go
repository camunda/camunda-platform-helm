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

// Package ghactions provides small helpers for GitHub Actions step logic that
// is being ported from bash to Go. Writer appends key/value pairs to the files
// named by $GITHUB_ENV and $GITHUB_OUTPUT.
package ghactions

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// Writer appends key/value pairs to the file named by a GitHub Actions
// environment variable ($GITHUB_ENV or $GITHUB_OUTPUT). When that env var is
// unset it falls back to stdout so the values stay visible on local runs —
// unless $GITHUB_ACTIONS is "true", where a missing target file means the
// step is misconfigured and Set returns an error instead of echoing values
// (potentially secrets) into the job log. Multiline values use the heredoc
// form GitHub requires.
type Writer struct {
	// Path is the target file. Empty means stdout.
	Path string
}

// NewGitHubEnv targets $GITHUB_ENV, the job-scoped environment file.
func NewGitHubEnv() *Writer { return &Writer{Path: os.Getenv("GITHUB_ENV")} }

// NewGitHubOutput targets $GITHUB_OUTPUT, the step outputs file.
func NewGitHubOutput() *Writer { return &Writer{Path: os.Getenv("GITHUB_OUTPUT")} }

// Set appends a single key/value pair.
func (w *Writer) Set(key, value string) error {
	var line string
	if strings.Contains(value, "\n") {
		// GitHub Actions requires a heredoc delimiter that does not occur in
		// the value; a random per-call delimiter guarantees that for arbitrary
		// content.
		delim, err := randomDelimiter()
		if err != nil {
			return err
		}
		line = fmt.Sprintf("%s<<%s\n%s\n%s\n", key, delim, value, delim)
	} else {
		line = fmt.Sprintf("%s=%s\n", key, value)
	}
	if w.Path == "" {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			return fmt.Errorf("set %s: no target file configured while running in GitHub Actions", key)
		}
		_, err := fmt.Fprint(os.Stdout, line)
		return err
	}
	f, err := os.OpenFile(w.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", w.Path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write %s: %w", w.Path, err)
	}
	return nil
}

// randomDelimiter returns a delimiter that is not feasibly present in any value.
func randomDelimiter() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate delimiter: %w", err)
	}
	return "ghadelimiter_" + hex.EncodeToString(b), nil
}

package main

import (
	"fmt"
	"os"
	"strings"
)

// ghWriter appends key/value pairs to the file named by a GitHub Actions
// environment variable ($GITHUB_OUTPUT or $GITHUB_ENV). When that env var is
// unset (local runs/tests) it falls back to stdout so the values are still
// visible. Multiline values use the heredoc form GitHub requires.
type ghWriter struct {
	path string // "" → stdout
}

func newGitHubOutput() ghWriter { return ghWriter{path: os.Getenv("GITHUB_OUTPUT")} }

// newGitHubEnv targets $GITHUB_ENV (the job-scoped environment file). Falls back
// to stdout when unset.
func newGitHubEnv() ghWriter { return ghWriter{path: os.Getenv("GITHUB_ENV")} }

func (w ghWriter) set(key, value string) error {
	var line string
	if strings.Contains(value, "\n") {
		line = fmt.Sprintf("%s<<__EOF__\n%s\n__EOF__\n", key, value)
	} else {
		line = fmt.Sprintf("%s=%s\n", key, value)
	}
	if w.path == "" {
		_, err := fmt.Fprint(os.Stdout, line)
		return err
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", w.path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write %s: %w", w.path, err)
	}
	return nil
}

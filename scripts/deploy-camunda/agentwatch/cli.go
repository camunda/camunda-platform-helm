// Package agentwatch wraps a long-running Helm deploy with a polling loop that
// hands the cluster's current state to a local agent CLI (Claude Code or
// opencode) and acts on the verdict it returns.
//
// The package deliberately knows nothing about the user's API keys or models —
// it shells out to whichever agent CLI is installed and lets that CLI's own
// configuration drive auth and model choice.
package agentwatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// claudeSessionIDPattern matches the UUID format Claude Code emits for
// session identifiers. We validate before using session_id in filesystem
// operations: filepath.Join calls filepath.Clean, so a crafted value
// containing "../" would otherwise escape ~/.claude/projects/.
var claudeSessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// AgentCLI describes a discovered local agent CLI.
type AgentCLI struct {
	// Name is the binary name as found on PATH ("claude" or "opencode").
	Name string
	// Path is the absolute path returned by exec.LookPath.
	Path string
}

// supportedCLIs lists the agent CLIs we know how to invoke, in preference
// order. Detection picks the first one found on PATH.
var supportedCLIs = []string{"opencode", "claude"}

// ErrNoAgentCLI is returned when neither supported CLI is installed.
var ErrNoAgentCLI = errors.New(
	"no agent CLI found on PATH; install Claude Code (https://claude.com/claude-code) " +
		"or opencode (https://opencode.ai), then re-run",
)

// DetectCLI searches PATH for a supported agent CLI and returns the first
// match. If none is found, it returns ErrNoAgentCLI.
func DetectCLI() (AgentCLI, error) {
	for _, name := range supportedCLIs {
		if path, err := exec.LookPath(name); err == nil {
			return AgentCLI{Name: name, Path: path}, nil
		}
	}
	return AgentCLI{}, ErrNoAgentCLI
}

// ResolveCLI returns an AgentCLI for the given name. If name is empty, it
// falls back to DetectCLI (auto-detection). If a name is provided but the
// binary cannot be found on PATH, an error is returned.
func ResolveCLI(name string) (AgentCLI, error) {
	if name == "" {
		return DetectCLI()
	}
	supported := false
	for _, s := range supportedCLIs {
		if name == s {
			supported = true
			break
		}
	}
	if !supported {
		return AgentCLI{}, fmt.Errorf("unsupported agent CLI %q; supported: %v", name, supportedCLIs)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return AgentCLI{}, fmt.Errorf("agent CLI %q not found on PATH: %w", name, err)
	}
	return AgentCLI{Name: name, Path: path}, nil
}

// buildArgs returns the command-line args for invoking the given CLI in
// non-interactive mode with the supplied prompt. The snapshot JSON is piped
// in via stdin; both supported CLIs read the prompt's primary content from
// stdin when no positional input file is given.
func buildArgs(cli AgentCLI, prompt string) []string {
	switch cli.Name {
	case "claude":
		// claude -p <prompt> --output-format json reads additional context from stdin.
		return []string{"-p", prompt, "--output-format", "json"}
	case "opencode":
		// opencode run accepts a prompt and emits structured output via --format.
		return []string{"run", prompt, "--format", "json"}
	default:
		// Should not reach here because DetectCLI only returns supported names.
		return []string{prompt}
	}
}

// Invoke runs the agent CLI once with the given prompt and snapshot JSON,
// returning whatever the CLI wrote to stdout. The caller is responsible for
// parsing the output into a Verdict.
//
// Network errors, rate limits, and auth failures are the user's CLI's
// problem to surface; this function only translates non-zero exits into
// a Go error and returns combined stdout/stderr for diagnostics.
func Invoke(ctx context.Context, cli AgentCLI, prompt string, snapshot []byte) ([]byte, error) {
	args := buildArgs(cli, prompt)
	cmd := exec.CommandContext(ctx, cli.Path, args...)
	cmd.Stdin = bytes.NewReader(snapshot)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	out := stdout.Bytes()
	// Claude Code's -p mode persists a transcript at
	// ~/.claude/projects/<cwd-hash>/<session_id>.jsonl on every invocation.
	// Because agentwatch polls every Interval, an hour-long install can
	// leave hundreds of throwaway sessions cluttering the user's history.
	// Best-effort delete the file using session_id from the JSON envelope.
	if cli.Name == "claude" {
		cleanupClaudeSession(out)
	}
	if runErr != nil {
		return out, fmt.Errorf("%s exited with error: %w; stderr: %s",
			cli.Name, runErr, truncate(stderr.String(), 2000))
	}
	return out, nil
}

// cleanupClaudeSession parses the session_id from a Claude Code -p
// --output-format json envelope and removes the matching transcript file.
// All failures are silent: this is housekeeping, not correctness.
func cleanupClaudeSession(raw []byte) {
	var env struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || env.SessionID == "" {
		return
	}
	if !claudeSessionIDPattern.MatchString(env.SessionID) {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	matches, err := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", env.SessionID+".jsonl"))
	if err != nil {
		return
	}
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

// truncate trims s to at most n runes, appending an ellipsis when shortened.
// Used to keep error messages from blowing up when an agent CLI prints a
// large stack trace.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// SupportedCLIs returns the names of the agent CLIs this package knows about,
// in detection-preference order. Exposed for documentation and tests.
func SupportedCLIs() []string {
	out := make([]string, len(supportedCLIs))
	copy(out, supportedCLIs)
	return out
}

package env

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"scripts/camunda-core/pkg/logging"

	"github.com/joho/godotenv"
)

// Load attempts to load a .env file. It does not error if the file is missing.
func Load(path string) error {
	if err := godotenv.Load(path); err != nil {
		// Only return error if it's not a "path not found" type of error
		if !os.IsNotExist(err) {
			return err
		}
		logging.Logger.Debug().Str("path", path).Msg(".env file not found, skipping")
	} else {
		logging.Logger.Info().Str("path", path).Msg("Loaded .env file")
	}
	return nil
}

// ReadFile reads a .env file and returns its key-value pairs as a map
// without modifying the process environment. Returns an empty map (not
// an error) if the file does not exist.
func ReadFile(path string) (map[string]string, error) {
	m, err := godotenv.Read(path)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Logger.Debug().Str("path", path).Msg(".env file not found, returning empty map")
			return make(map[string]string), nil
		}
		return nil, err
	}
	logging.Logger.Debug().Str("path", path).Int("count", len(m)).Msg("Read .env file into map")
	return m, nil
}

// fileMutexes provides per-file locking so that concurrent goroutines
// writing to the same .env file don't race. Keyed by absolute path.
var (
	fileMutexesMu sync.Mutex
	fileMutexes   = make(map[string]*sync.Mutex)
)

// lockFile returns a per-file mutex, creating one if needed.
func lockFile(path string) *sync.Mutex {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path // fallback to raw path
	}
	fileMutexesMu.Lock()
	defer fileMutexesMu.Unlock()
	mu, ok := fileMutexes[abs]
	if !ok {
		mu = &sync.Mutex{}
		fileMutexes[abs] = mu
	}
	return mu
}

// Append adds or updates a single key-value pair in the .env file.
// It is format-preserving: comments, ordering, quoting, and export prefixes
// in the original file are retained. Only the targeted key's value is changed
// (or appended at the end if the key does not yet exist).
// Safe for concurrent use on the same file from multiple goroutines.
func Append(path, key, value string) error {
	return AppendMultiple(path, map[string]string{key: value})
}

// AppendMultiple adds or updates multiple key-value pairs in a single
// read-write cycle. Format-preserving and safe for concurrent use.
func AppendMultiple(path string, updates map[string]string) error {
	mu := lockFile(path)
	mu.Lock()
	defer mu.Unlock()

	return appendMultipleLocked(path, updates)
}

// appendMultipleLocked performs the format-preserving read-modify-write.
// Caller must hold the per-file mutex.
//
// Strategy: read the raw file line-by-line. For each line that sets a key
// present in updates, rewrite that line with the new value. Any keys from
// updates not found in the file are appended at the end. Lines that are
// comments, blank, or set unrelated keys are passed through verbatim.
func appendMultipleLocked(path string, updates map[string]string) error {
	if len(updates) == 0 {
		return nil
	}

	// Track which keys still need to be written.
	remaining := make(map[string]string, len(updates))
	for k, v := range updates {
		remaining[k] = v
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var lines []string
	if len(data) > 0 {
		lines = strings.Split(string(data), "\n")
	}

	// Rewrite existing lines in-place.
	for i, line := range lines {
		key := extractKey(line)
		if key == "" {
			continue // blank, comment, or unparseable — keep verbatim
		}
		if newVal, ok := remaining[key]; ok {
			lines[i] = rewriteLine(line, key, newVal)
			delete(remaining, key)
		}
	}

	// Append keys that were not found in the existing file.
	// Use a deterministic order (sorted) so output is stable.
	if len(remaining) > 0 {
		// Ensure there's a trailing newline before appending.
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		// Append in the order they appear in the updates map's iteration
		// (callers wanting deterministic order should use AppendMultiple
		// with a small map, which is typical — 4 keys in generateTestSecrets).
		for k, v := range remaining {
			lines = append(lines, formatLine(k, v))
		}
	}

	content := strings.Join(lines, "\n")
	// Ensure file ends with exactly one newline.
	content = strings.TrimRight(content, "\n") + "\n"

	return os.WriteFile(path, []byte(content), 0o644)
}

// extractKey returns the key from a line like "KEY=value", "export KEY=value",
// or "" if the line is a comment, blank, or unparseable.
func extractKey(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	// Strip optional "export " prefix.
	raw := trimmed
	if strings.HasPrefix(raw, "export ") {
		raw = strings.TrimPrefix(raw, "export ")
		raw = strings.TrimSpace(raw)
	}
	idx := strings.IndexByte(raw, '=')
	if idx <= 0 {
		return "" // no '=' or starts with '='
	}
	return strings.TrimSpace(raw[:idx])
}

// rewriteLine replaces the value portion of a KEY=value line, preserving
// any leading whitespace, "export" prefix, and the key name exactly as-is.
func rewriteLine(line, key, newVal string) string {
	// Find the key assignment in the raw line (handles "export KEY=..." and "KEY=...").
	// We search for "KEY=" (possibly with export prefix and spaces).
	idx := strings.Index(line, key+"=")
	if idx < 0 {
		// Shouldn't happen since extractKey matched, but be safe.
		return formatLine(key, newVal)
	}
	// Everything up to and including "KEY=" is the prefix.
	prefix := line[:idx+len(key)+1]
	return prefix + quoteValue(newVal)
}

// quoteValue wraps a value in double quotes if it contains spaces, special
// characters, or is empty. Simple alphanumeric values are left unquoted to
// match common .env conventions.
func quoteValue(v string) string {
	if v == "" {
		return `""`
	}
	// Quote if value contains characters that could be problematic.
	if strings.ContainsAny(v, " \t\n'\"\\#$`!{}()[]|&;<>") {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return v
}

// formatLine produces a new "KEY=value" line for appending to the file.
func formatLine(key, value string) string {
	return key + "=" + quoteValue(value)
}

// readResult holds the result of a non-blocking stdin read.
type readResult struct {
	value string
	err   error
}

// Prompt interactively asks the user for a value for the given key.
// It respects context cancellation so that Ctrl+C (which cancels the
// signal-aware context) can interrupt a blocking stdin read.
func Prompt(ctx context.Context, key, defaultValue string) (string, error) {
	fmt.Printf("Enter value for %s", key)
	if defaultValue != "" {
		fmt.Printf(" [default: %s]", defaultValue)
	}
	fmt.Print(": ")

	ch := make(chan readResult, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		ch <- readResult{value: input, err: err}
	}()

	select {
	case <-ctx.Done():
		// Print a newline so the terminal prompt isn't garbled.
		fmt.Println()
		return "", ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return "", res.err
		}
		input := strings.TrimSpace(res.value)
		if input == "" {
			return defaultValue, nil
		}
		return input, nil
	}
}

package env

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

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

// Append adds a key-value pair to the .env file.
func Append(path, key, value string) error {
	// Read existing map
	envMap, err := godotenv.Read(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if envMap == nil {
		envMap = make(map[string]string)
	}

	// Update value
	envMap[key] = value

	// Write back to file using godotenv to handle formatting/quoting
	return godotenv.Write(envMap, path)
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

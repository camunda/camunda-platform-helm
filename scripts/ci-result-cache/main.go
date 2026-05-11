package main

// ci-result-cache: progressive CI result caching for merge queue optimization.
// See cmd/ for the available subcommands.

import (
	"errors"
	"fmt"
	"os"

	"scripts/ci-result-cache/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// ErrNotCached means the check command already printed its
		// "NOT CACHED" message — no additional output needed.
		if !errors.Is(err, cmd.ErrNotCached) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

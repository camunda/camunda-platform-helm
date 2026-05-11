package main

// ci-result-cache: progressive CI result caching for merge queue optimization.
// See cmd/ for the available subcommands.

import (
	"fmt"
	"os"

	"scripts/ci-result-cache/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

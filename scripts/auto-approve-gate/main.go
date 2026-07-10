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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"scripts/auto-approve-gate/pkg/gate"
)

const (
	defaultAllowlistPath              = ".github/auto-approve-allowlist.txt"
	defaultProtectedPathsPath         = ".github/auto-approve-protected-paths.txt"
	defaultRenovateProtectedPathsPath = ".github/auto-approve-protected-paths-renovate.txt"
)

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(stdout io.Writer) error {
	author := os.Getenv("PR_AUTHOR")
	if author == "" {
		return fmt.Errorf("PR_AUTHOR environment variable is required")
	}

	prStr := os.Getenv("PR_NUMBER")
	if prStr == "" {
		return fmt.Errorf("PR_NUMBER environment variable is required")
	}
	eventActor := os.Getenv("EVENT_ACTOR")
	if eventActor == "" {
		return fmt.Errorf("EVENT_ACTOR environment variable is required")
	}

	prNumber, err := strconv.Atoi(prStr)
	if err != nil {
		return fmt.Errorf("parse PR_NUMBER %q: %w", prStr, err)
	}

	cfg := gate.Config{
		Author:                     author,
		EventActor:                 eventActor,
		PRNumber:                   prNumber,
		AllowlistPath:              resolveListPath("AUTO_APPROVE_ALLOWLIST", defaultAllowlistPath),
		ProtectedPathsPath:         resolveListPath("AUTO_APPROVE_PROTECTED_PATHS", defaultProtectedPathsPath),
		RenovateProtectedPathsPath: resolveListPath("AUTO_APPROVE_PROTECTED_PATHS_RENOVATE", defaultRenovateProtectedPathsPath),
	}

	gh, err := gate.NewGitHubClientFromEnv()
	if err != nil {
		return err
	}

	return gate.Run(cfg, gh, stdout)
}

func resolveListPath(envKey, defaultPath string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if fileExists(defaultPath) {
		return defaultPath
	}
	fromModule := filepath.Join("..", "..", defaultPath)
	if fileExists(fromModule) {
		return fromModule
	}
	return defaultPath
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

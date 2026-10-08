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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"scripts/ci-result-cache/pkg/hash"

	"github.com/spf13/cobra"
)

var resolveRunnerImageCmd = &cobra.Command{
	Use:   "resolve-runner-image",
	Short: "Resolve the Playwright runner to an immutable image reference",
	Args:  cobra.NoArgs,
	RunE:  runResolveRunnerImage,
}

func runResolveRunnerImage(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
	defer cancel()
	image, err := resolveRunnerImage(ctx)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "::warning::Could not resolve the Playwright runner image. E2E jobs use latest and the result cache is off for this run.")
		return nil
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "image=%s\n", image)
	return err
}

func resolveRunnerImage(ctx context.Context) (string, error) {
	token, actor := os.Getenv("GITHUB_TOKEN"), os.Getenv("GITHUB_ACTOR")
	if token == "" || actor == "" {
		return "", fmt.Errorf("GITHUB_TOKEN and GITHUB_ACTOR are required")
	}
	command := exec.CommandContext(ctx, "oras", "resolve", "--username", actor, "--password-stdin", hash.PlaywrightRunnerImage+":latest")
	command.Stdin = strings.NewReader(token + "\n")
	digest, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolving runner image: %w", err)
	}
	image := hash.PlaywrightRunnerImage + "@" + strings.TrimSpace(string(digest))
	if err := hash.ValidateRunnerImage(image); err != nil {
		return "", err
	}
	return image, nil
}

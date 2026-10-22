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
	"os"
	"os/signal"
	"syscall"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/triage"

	"github.com/spf13/cobra"
)

// newTriageCommand creates the `triage` subcommand: from a failed integration
// run URL/ID, pull the failing job log, strip the env-var noise, and print the
// structured failure record (matrix cell, helm command, error, diagnostics path)
// plus a local-repro command — the post-mortem entry point that replaces manual
// `gh api` + grep.
func newTriageCommand() *cobra.Command {
	var repoFlag string
	var jobFlag string
	var showLog bool

	cmd := &cobra.Command{
		Use:   "triage <run-url|run-id>",
		Short: "Triage a failed integration run: extract the failure record from its job log",
		Long: `Turn a failed GitHub Actions run into a structured failure summary.

Accepts a run URL (https://github.com/<owner>/<repo>/actions/runs/<id>[/job/<jobid>])
or a bare run ID. Resolves the failing job(s), pulls the job log via the reliable
` + "`gh api .../actions/jobs/<id>/logs`" + ` path (not --log-failed, which is empty
for merge-queue runs), strips the env-var noise, and prints:

  - the decoded matrix cell (version / shortname / flow / platform),
  - the failing step and the helm command that failed,
  - the error/reason and notable log signals,
  - the Diagnostics bundle path (download with: gh run download <run-id> --name 'diagnostics-*'),
  - a ready-to-run local reproduction command.

Requires the GitHub CLI (gh) to be installed and authenticated.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := logging.Setup(logging.Options{
				LevelString:  flags.LogLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			return triage.Run(ctx, os.Stdout, args[0], triage.Options{Repo: repoFlag, Job: jobFlag, ShowLog: showLog})
		},
	}

	f := cmd.Flags()
	f.StringVar(&repoFlag, "repo", "", "owner/repo (default: derived from the URL, else "+triage.DefaultRepo+")")
	f.StringVar(&jobFlag, "job", "", "Triage a specific job ID instead of auto-selecting the failed job")
	f.BoolVar(&showLog, "show-log", false, "Also print the denoised job log after the summary")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info", "Log level")

	return cmd
}

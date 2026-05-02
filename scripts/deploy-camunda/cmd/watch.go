package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/agentwatch"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// newWatchCommand creates the "watch" subcommand. It runs alongside (or in
// place of) `helm install --wait` and hands per-tick cluster snapshots to a
// local agent CLI for diagnosis. The agent decides whether to keep waiting,
// surface a problem, or recommend abort.
//
// This is intentionally a standalone subcommand rather than a flag on the
// main install path: the watcher is useful both during a fresh deploy
// (separate terminal) and as a post-hoc triage tool when an install has
// stalled. Wiring it into the install path is a follow-up.
func newWatchCommand() *cobra.Command {
	var (
		namespace       string
		release         string
		kubeContext     string
		intervalSeconds int
		abortConfidence float64
		corpusDir       string
		maxTicks        int
		logLevel        string
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch a Helm install with a local agent CLI and surface diagnoses live",
		Long: `Poll the cluster every few seconds, hand the snapshot to a local agent
CLI (Claude Code or opencode), and act on the structured verdict it returns.

The watcher detects which agent CLI is installed at startup. If neither is
found, it errors out with installation pointers. Authentication, model
choice, and rate limiting are the local CLI's responsibility — this command
does not call any API directly.

Typical use:

  # In one terminal, run a normal install:
  deploy-camunda --scenario eske --namespace my-test --release int

  # In another, watch it:
  deploy-camunda watch --namespace my-test --release int \
    --abort-confidence 0.85 --corpus-dir ~/eval/snapshots`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logging.Setup(logging.Options{
				LevelString:  logLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}
			if strings.TrimSpace(namespace) == "" {
				return fmt.Errorf("--namespace is required")
			}

			cli, err := agentwatch.DetectCLI()
			if err != nil {
				return err
			}
			logging.Logger.Info().
				Str("cli", cli.Name).
				Str("path", cli.Path).
				Msg("agentwatch: using local agent CLI")

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			opts := agentwatch.Options{
				CLI: cli,
				Snapshot: agentwatch.SnapshotOptions{
					Namespace:   namespace,
					Release:     release,
					KubeContext: kubeContext,
				},
				Interval:        time.Duration(intervalSeconds) * time.Second,
				AbortConfidence: abortConfidence,
				CorpusDir:       corpusDir,
				MaxTicks:        maxTicks,
			}

			decision, verdict, err := agentwatch.Watch(ctx, opts)
			if err != nil {
				// Ctrl+C / SIGTERM is the documented way to stop the
				// watcher in interactive use. Treat it as a clean exit
				// rather than surfacing "Error: context canceled".
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
				return err
			}

			switch decision {
			case agentwatch.DecisionAbort:
				if verdict != nil {
					fmt.Fprintf(os.Stderr, "\nAGENT VERDICT: %s\n  confidence=%.2f\n  action=%s\n",
						verdict.Diagnosis, verdict.Confidence, verdict.RecommendedAction)
				}
				return fmt.Errorf("agent recommended abort")
			default:
				return nil
			}
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace to watch (required)")
	cmd.Flags().StringVarP(&release, "release", "r", "", "Helm release name (used for `helm status`)")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "Kube context (defaults to current)")
	cmd.Flags().IntVar(&intervalSeconds, "interval", 60, "Poll interval in seconds")
	cmd.Flags().Float64Var(&abortConfidence, "abort-confidence", 0,
		"Auto-abort when an 'abort' verdict has confidence at or above this value (0 disables auto-abort)")
	cmd.Flags().StringVar(&corpusDir, "corpus-dir", "",
		"Directory to persist snapshot+verdict pairs for the eval corpus (empty disables persistence)")
	cmd.Flags().IntVar(&maxTicks, "max-ticks", 0, "Maximum number of poll ticks before exiting (0 = unbounded)")
	cmd.Flags().StringVarP(&logLevel, "log-level", "l", "info", "Log level")

	cmd.AddCommand(newWatchReplayCommand())

	return cmd
}

// newWatchReplayCommand creates "watch replay" — the eval-on-corpus tool.
// Given a directory of captured tick records (written by `watch --corpus-dir`),
// it re-invokes the agent CLI on each snapshot, parses the new verdict, and
// prints a summary diff against the recorded verdict. Exits non-zero if any
// recorded verdict regresses on action class or drops below the operator's
// confidence threshold.
func newWatchReplayCommand() *cobra.Command {
	var (
		strict   bool
		logLevel string
	)
	cmd := &cobra.Command{
		Use:   "replay <corpus-dir>",
		Short: "Re-run the agent skill over a captured corpus and diff verdicts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logging.Setup(logging.Options{
				LevelString:  logLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}
			cli, err := agentwatch.DetectCLI()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			report, err := agentwatch.Replay(ctx, cli, args[0])
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
				return err
			}
			fmt.Print(report.Format())
			if strict && report.Regressions > 0 {
				return fmt.Errorf("%d verdict regression(s) detected", report.Regressions)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", true, "Exit non-zero if any verdict action class regresses")
	cmd.Flags().StringVarP(&logLevel, "log-level", "l", "warn", "Log level (replay is noisy at info)")
	return cmd
}

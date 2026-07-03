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
		maxDuration     time.Duration
		maxErrors       int
		logLevel        string
		cliName         string
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

			cli, err := agentwatch.ResolveCLI(cliName)
			if err != nil {
				return err
			}
			logging.Logger.Info().
				Str("cli", cli.Name).
				Str("path", cli.Path).
				Msg("agentwatch: using local agent CLI")

			if corpusDir != "" {
				fmt.Fprintf(os.Stderr,
					"agentwatch: NOTE — captured snapshots in %s contain pod specs, event messages, "+
						"and helm status output. Best-effort redaction is applied for env values matching "+
						"/TOKEN|SECRET|PASSWORD|KEY/i and for bearer tokens / JWTs / Authorization headers "+
						"in string values. Other forms of sensitive data (custom credential formats, secrets "+
						"in unexpected fields) may not be caught — review and treat the directory as "+
						"sensitive before sharing.\n",
					corpusDir)
			}

			// Track which signal (if any) interrupted the watch so we can
			// exit with the conventional code (130 for SIGINT, 143 for
			// SIGTERM). Without this, a CI wrapper cannot distinguish a
			// user-initiated stop from a clean completion.
			var interruptSignal os.Signal
			signalCh := make(chan os.Signal, 1)
			signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(signalCh)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				if s, ok := <-signalCh; ok {
					interruptSignal = s
					cancel()
				}
			}()

			interval := time.Duration(intervalSeconds) * time.Second
			if interval > 0 && interval < agentwatch.MinInterval {
				logging.Logger.Warn().
					Dur("requested", interval).
					Dur("floor", agentwatch.MinInterval).
					Msg("agentwatch: --interval below floor; clamping to MinInterval to avoid agent-API fire-hose")
				interval = agentwatch.MinInterval
			}

			opts := agentwatch.Options{
				CLI: cli,
				Snapshot: agentwatch.SnapshotOptions{
					Namespace:   namespace,
					Release:     release,
					KubeContext: kubeContext,
				},
				Interval:        interval,
				AbortConfidence: abortConfidence,
				CorpusDir:       corpusDir,
				MaxTicks:        maxTicks,
				MaxDuration:     maxDuration,
				MaxErrors:       maxErrors,
			}

			decision, verdict, err := agentwatch.Watch(ctx, opts)
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			if decision == agentwatch.DecisionAbort {
				if verdict != nil {
					fmt.Fprintf(os.Stderr, "\nAGENT VERDICT: %s\n  confidence=%.2f\n  action=%s\n",
						verdict.Diagnosis, verdict.Confidence, verdict.RecommendedAction)
				}
				return fmt.Errorf("agent recommended abort")
			}

			// Clean stop on signal: exit with the conventional code so CI
			// wrappers can tell a user-aborted watch from a normal exit.
			switch interruptSignal {
			case syscall.SIGINT:
				os.Exit(130)
			case syscall.SIGTERM:
				os.Exit(143)
			}
			return nil
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
	cmd.Flags().IntVar(&maxTicks, "max-ticks", 0,
		"Maximum number of poll ticks before exiting (0 = package default, negative = unbounded)")
	cmd.Flags().DurationVar(&maxDuration, "max-duration", 0,
		"Wall-clock cap for the entire watch run (0 = package default, e.g. 30m)")
	cmd.Flags().IntVar(&maxErrors, "max-errors", 0,
		"Consecutive snapshot/agent/parse errors tolerated before giving up (0 = package default)")
	cmd.Flags().StringVarP(&logLevel, "log-level", "l", "info", "Log level")
	cmd.Flags().StringVar(&cliName, "cli", "", "Agent CLI to use: opencode or claude (auto-detected if empty)")

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
		strict    bool
		logLevel  string
		replayCLI string
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
			cli, err := agentwatch.ResolveCLI(replayCLI)
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
	cmd.Flags().StringVar(&replayCLI, "cli", "", "Agent CLI to use: opencode or claude (auto-detected if empty)")
	return cmd
}

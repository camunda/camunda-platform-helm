package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/prepare-helm-values/pkg/env"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newInitCommand creates the `config init` wizard: an interactive first-run
// setup that scaffolds a config file deployment, optional docker credentials,
// and test secrets in .env, then ends with a doctor checklist. Mirrors
// `gh auth login` / `supabase init`.
func newInitCommand() *cobra.Command {
	var nonInteractive bool
	var envFile string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactively scaffold deploy-camunda config and .env",
		Long: `Guide first-time setup of deploy-camunda.

Prompts for a deployment profile (platform, kube-context, ingress base domain,
repo root), optionally captures Harbor credentials into .env, optionally
scaffolds the random test secrets, then runs the doctor preflight so you finish
with a ✓/✗ checklist instead of guessing what is missing.

Use --non-interactive in CI to ensure a config file exists and print the
checklist without prompting.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := logging.Setup(logging.Options{
				LevelString:  "info",
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			cfgRes, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			if nonInteractive {
				if !cfgRes.Found {
					return fmt.Errorf("no config file found at %s; run `deploy-camunda config init` interactively to create one", cfgRes.Path)
				}
				fmt.Fprintf(out, "Using config: %s\n", cfgRes.Path)
				return runDoctorAfterInit(ctx, out, cfgRes)
			}

			reader := bufio.NewReader(cmd.InOrStdin())
			fmt.Fprintf(out, "deploy-camunda setup — config: %s\n\n", cfgRes.Path)

			// 1. Deployment profile.
			name, err := promptLine(ctx, out, reader, "Deployment profile name", "local")
			if err != nil {
				return err
			}
			if err := config.CreateDeployment(cfgRes.Path, name); err != nil {
				// Already-exists is fine — we'll update it in place.
				if !errors.Is(err, config.ErrDeploymentExists) {
					return err
				}
				fmt.Fprintf(out, "  profile %q already exists — updating it\n", name)
			}

			repoDefault, _ := config.DetectRepoRoot()
			fields := []struct {
				key, prompt, def string
				choices          []string
			}{
				{"platform", "Platform", "gke", config.DeployPlatforms},
				{"kubeContext", "Kube context", currentKubeContext(), nil},
				{"ingressBaseDomain", "Ingress base domain", config.ValidIngressBaseDomains[0], config.ValidIngressBaseDomains},
				{"repoRoot", "Repo root", repoDefault, nil},
			}
			for _, f := range fields {
				if len(f.choices) > 0 {
					fmt.Fprintf(out, "  (options: %s)\n", strings.Join(f.choices, ", "))
				}
				val, err := promptLine(ctx, out, reader, f.prompt, f.def)
				if err != nil {
					return err
				}
				if strings.TrimSpace(val) == "" {
					continue
				}
				if err := config.SetValue(cfgRes.Path, name+"."+f.key, val); err != nil {
					return err
				}
			}
			if err := config.WriteCurrentOnly(cfgRes.Path, name); err != nil {
				return err
			}
			fmt.Fprintf(out, "Saved profile %q (now active) to %s\n\n", name, cfgRes.Path)

			// 2. Docker (Harbor) credentials → .env.
			if yes, err := promptYesNo(ctx, out, reader, "Store Harbor docker credentials in .env now?", false); err != nil {
				return err
			} else if yes {
				user, err := promptLine(ctx, out, reader, "Harbor username (env HARBOR_USERNAME)", "")
				if err != nil {
					return err
				}
				pass, err := promptSecret(ctx, out, reader, "Harbor password (env HARBOR_PASSWORD)")
				if err != nil {
					return err
				}
				updates := map[string]string{}
				if strings.TrimSpace(user) != "" {
					updates["HARBOR_USERNAME"] = user
				}
				if strings.TrimSpace(pass) != "" {
					updates["HARBOR_PASSWORD"] = pass
				}
				if len(updates) > 0 {
					if err := env.AppendMultiple(envFile, updates); err != nil {
						return fmt.Errorf("failed to write docker credentials to %s: %w", envFile, err)
					}
					fmt.Fprintf(out, "  wrote %d credential(s) to %s\n\n", len(updates), envFile)
				}
			}

			// 3. Test secrets scaffold.
			if yes, err := promptYesNo(ctx, out, reader, "Generate random test secrets into .env?", false); err != nil {
				return err
			} else if yes {
				names, err := deploy.ScaffoldTestSecrets(envFile)
				if err != nil {
					return fmt.Errorf("failed to scaffold test secrets: %w", err)
				}
				fmt.Fprintf(out, "  ensured %d test secret(s) in %s\n\n", len(names), envFile)
			}

			// 3b. Companion (Postgres/RDBMS) dev credentials. Scenarios in the
			// elasticsearch/rdbms family substitute these into the companion
			// PostgreSQL values; for a fresh local deploy any value works.
			if yes, err := promptYesNo(ctx, out, reader, "Generate local Postgres/RDBMS dev credentials into .env? (needed by elasticsearch/rdbms scenarios)", false); err != nil {
				return err
			} else if yes {
				existing, _ := env.ReadFile(envFile)
				updates := map[string]string{}
				if existing["RDBMS_POSTGRESQL_USERNAME"] == "" {
					updates["RDBMS_POSTGRESQL_USERNAME"] = "camunda"
				}
				if existing["RDBMS_POSTGRESQL_PASSWORD"] == "" {
					pw, err := deploy.RandomSecret()
					if err != nil {
						return err
					}
					updates["RDBMS_POSTGRESQL_PASSWORD"] = pw
				}
				if len(updates) > 0 {
					if err := env.AppendMultiple(envFile, updates); err != nil {
						return fmt.Errorf("failed to write RDBMS credentials to %s: %w", envFile, err)
					}
					fmt.Fprintf(out, "  wrote %d RDBMS credential(s) to %s\n\n", len(updates), envFile)
				} else {
					fmt.Fprintf(out, "  RDBMS credentials already present in %s\n\n", envFile)
				}
			}

			// 4. Finish with the doctor checklist.
			fmt.Fprintln(out, "Running doctor preflight…")
			return runDoctorAfterInit(ctx, out, cfgRes)
		},
	}

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Don't prompt; require an existing config and just run doctor")
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to the .env file to write secrets/credentials into")
	return cmd
}

// runDoctorAfterInit loads the freshly-written config and prints a preflight
// checklist, reusing the same Preflight engine as the doctor command. It never
// returns an error for failing checks — init is advisory — but surfaces them.
func runDoctorAfterInit(ctx context.Context, out io.Writer, cfgRes *config.ConfigResolution) error {
	var f config.RuntimeFlags
	if _, _, err := config.LoadAndMerge(configFile, true, &f); err != nil {
		return err
	}
	report := deploy.Preflight(ctx, &f, deploy.PreflightOptions{
		ConfigPath:           cfgRes.Path,
		ConfigFound:          true, // a config file exists by the time we run the checklist
		SkipKubeReachability: false,
	})
	var buf bytes.Buffer
	report.Render(&buf)
	fmt.Fprint(out, buf.String())
	if !report.OK() {
		fmt.Fprintln(out, "\nSome checks need attention — re-run `deploy-camunda doctor` after fixing them.")
	} else {
		fmt.Fprintln(out, "\nAll checks passed — you're ready to deploy.")
	}
	return nil
}

// currentKubeContext returns the active kubectl context, or "" if unavailable.
func currentKubeContext() string {
	contexts, err := getKubeContexts()
	if err != nil || len(contexts) == 0 {
		return ""
	}
	// getKubeContexts lists all; current-context is a separate lookup.
	if out, err := exec.Command("kubectl", "config", "current-context").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// promptLine asks for a value with an optional default, honoring ctx cancellation.
func promptLine(ctx context.Context, out io.Writer, r *bufio.Reader, label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	line, err := readLineCtx(ctx, r)
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func promptSecret(ctx context.Context, out io.Writer, r *bufio.Reader, label string) (string, error) {
	fmt.Fprintf(out, "%s: ", label)
	// On a real terminal, read with echo disabled so the secret never appears on
	// screen or in scrollback. Piped/redirected input (tests, scripts) has
	// nothing to hide and falls back to the buffered line reader.
	stdinFd := int(os.Stdin.Fd())
	if r.Buffered() == 0 && term.IsTerminal(stdinFd) {
		secret, err := readSecretCtx(ctx, stdinFd)
		fmt.Fprintln(out)
		return secret, err
	}
	line, err := readLineCtx(ctx, r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// readSecretCtx reads an echo-suppressed line from the terminal fd, aborting if
// ctx is cancelled. Like readLineCtx, the parked term.ReadPassword goroutine is
// an accepted leak for this single-run CLI.
func readSecretCtx(ctx context.Context, fd int) (string, error) {
	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		b, err := term.ReadPassword(fd)
		ch <- res{b, err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case rr := <-ch:
		if rr.err != nil && !errors.Is(rr.err, io.EOF) {
			return "", rr.err
		}
		return strings.TrimSpace(string(rr.b)), nil
	}
}

func promptYesNo(ctx context.Context, out io.Writer, r *bufio.Reader, label string, def bool) (bool, error) {
	suffix := "y/N"
	if def {
		suffix = "Y/n"
	}
	fmt.Fprintf(out, "%s [%s]: ", label, suffix)
	line, err := readLineCtx(ctx, r)
	if err != nil {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def, nil
	}
	return line == "y" || line == "yes", nil
}

// readLineCtx reads a line, returning ctx.Err() if the context is cancelled
// (e.g. Ctrl+C) before input arrives.
func readLineCtx(ctx context.Context, r *bufio.Reader) (string, error) {
	type res struct {
		line string
		err  error
	}
	// Buffered so this goroutine's send never blocks: on ctx cancellation the
	// ReadString stays parked (bufio reads aren't context-aware) until stdin
	// yields a newline/EOF or the process exits — an accepted leak for this
	// single-run CLI.
	ch := make(chan res, 1)
	go func() {
		line, err := r.ReadString('\n')
		ch <- res{line, err}
	}()
	select {
	case <-ctx.Done():
		fmt.Println()
		return "", ctx.Err()
	case rr := <-ch:
		// EOF with content (e.g. piped input without a trailing newline, or the
		// final answer) is a normal end-of-input, not an error.
		if errors.Is(rr.err, io.EOF) {
			return rr.line, nil
		}
		return rr.line, rr.err
	}
}

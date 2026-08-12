package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"scripts/deploy-camunda/pkg/redaction"

	"github.com/spf13/cobra"
)

// newRedactCommand creates `redact`: the persistence boundary for text artifacts
// that are produced by third-party tooling and therefore cannot be sanitized at
// the point of generation. Playwright's JSON results are the motivating case —
// they carry the per-test outcomes and error messages CI needs for triage, but
// they are written by the test runner, not by this tool.
//
// Binary artifacts (traces, videos, screenshots) are deliberately out of scope:
// they cannot be reliably redacted and are not retained by CI at all.
func newRedactCommand() *cobra.Command {
	var inPath, outPath string

	cmd := &cobra.Command{
		Use:   "redact",
		Short: "Redact credentials from a text file or stream",
		Long: `Apply the shared redactor to a text file or stream.

Reads --in (default stdin) and writes --out (default stdout), replacing
authorization headers, bearer tokens, JWTs, private keys, URL credentials, and
sensitive key=value / "key": "value" assignments with a placeholder.

Intended for text artifacts produced by third-party tooling that CI needs to
retain, where sanitizing at the point of generation is not possible. Output
files are created owner-only (0600).

This is a text-level redactor, not a guarantee: never use it to justify
publishing binary artifacts, which cannot be reliably sanitized.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRedact(inPath, outPath, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}

	f := cmd.Flags()
	f.StringVar(&inPath, "in", "", "Input file (default stdin)")
	f.StringVar(&outPath, "out", "", "Output file, created 0600 (default stdout); may equal --in for in-place rewrite")

	return cmd
}

// runRedact reads the input, redacts it, and writes the result. An --out equal
// to --in is supported by buffering the whole input before the write, so an
// in-place rewrite cannot truncate the file it is still reading.
func runRedact(inPath, outPath string, stdin io.Reader, stdout io.Writer) error {
	var (
		raw []byte
		err error
	)
	if inPath == "" {
		raw, err = io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
	} else {
		raw, err = os.ReadFile(inPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", inPath, err)
		}
	}

	redacted := redaction.Text(string(raw))

	if outPath == "" {
		if _, err := io.WriteString(stdout, redacted); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		return nil
	}

	if fi, statErr := os.Lstat(outPath); statErr == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write %s through a symbolic link", outPath)
	}
	if dir := filepath.Dir(outPath); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(outPath, []byte(redacted), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	if err := os.Chmod(outPath, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", outPath, err)
	}
	return nil
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"

	"github.com/spf13/cobra"
)

const defaultTriageRepo = "camunda/camunda-platform-helm"

// ghAPI shells out to the GitHub CLI's `gh api` and returns stdout. It is a
// package var so tests can inject canned responses without a network call.
var ghAPI = func(ctx context.Context, args []string) ([]byte, error) {
	return executil.RunCommandCapture(ctx, "gh", append([]string{"api"}, args...), nil, "")
}

// runRef identifies a GitHub Actions run (and optionally a specific job) to triage.
type runRef struct {
	Owner string
	Repo  string
	RunID string
	JobID string // optional; when set, that job is triaged directly
}

// matrixCell is the decoded integration matrix coordinate parsed from a job name
// like "8.9 - cprst - install - pr - gke".
type matrixCell struct {
	Version   string
	Shortname string
	Flow      string
	Case      string
	Platform  string
}

func (c matrixCell) empty() bool {
	return c.Version == "" && c.Shortname == "" && c.Flow == ""
}

// failureRecord is the denoised, structured summary extracted from a failing job log.
type failureRecord struct {
	JobName        string
	Conclusion     string
	Cell           matrixCell
	FailingStep    string
	HelmCommand    string
	Reason         string
	DiagnosticsDir string
	Signals        []string
}

// ghJob is the subset of the GitHub Actions job object we consume.
type ghJob struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Steps      []struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	} `json:"steps"`
}

type ghJobsResponse struct {
	Jobs []ghJob `json:"jobs"`
}

var (
	// runURLRe matches .../actions/runs/<runID>[/job/<jobID>] in a GitHub URL.
	runURLRe = regexp.MustCompile(`/actions/runs/(\d+)(?:/job/(\d+))?`)
	// repoURLRe matches github.com/<owner>/<repo> in a URL.
	repoURLRe = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)`)
	// diagnosticsRe matches the "Diagnostics: diagnostics/<ns>/<ts>" hint.
	diagnosticsRe = regexp.MustCompile(`Diagnostics:\s*(\S+)`)
	// quotedHelmCmdRe matches a quoted command="helm ..." field, which the matrix
	// runner emits with the exact failing invocation.
	quotedHelmCmdRe = regexp.MustCompile(`command="(helm [^"]*)"`)
	// ansiRe matches ANSI SGR color escapes present in raw gh job logs.
	ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m|\\[[0-9]{1,2}m")
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

			ref, err := parseRunRef(args[0], repoFlag)
			if err != nil {
				return err
			}
			if jobFlag != "" {
				ref.JobID = jobFlag
			}

			log, job, err := fetchFailingJobLog(ctx, ref)
			if err != nil {
				return err
			}

			rec := extractFailureRecord(log)
			if job != nil {
				if rec.JobName == "" {
					rec.JobName = job.Name
				}
				rec.Conclusion = job.Conclusion
				if rec.Cell.empty() {
					rec.Cell = parseMatrixCell(job.Name)
				}
				if rec.FailingStep == "" {
					rec.FailingStep = firstFailedStep(*job)
				}
			}

			fmt.Fprint(os.Stdout, renderTriageReport(ref, rec))
			if showLog {
				fmt.Fprintf(os.Stdout, "\n----- denoised log -----\n%s\n", stripLogNoise(log))
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&repoFlag, "repo", "", "owner/repo (default: derived from the URL, else "+defaultTriageRepo+")")
	f.StringVar(&jobFlag, "job", "", "Triage a specific job ID instead of auto-selecting the failed job")
	f.BoolVar(&showLog, "show-log", false, "Also print the denoised job log after the summary")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info", "Log level")

	return cmd
}

// parseRunRef resolves a run URL or bare run ID into a runRef. repoOverride, when
// non-empty ("owner/repo"), wins over any repo derived from the URL.
func parseRunRef(input, repoOverride string) (runRef, error) {
	ref := runRef{Owner: "", Repo: ""}

	if m := runURLRe.FindStringSubmatch(input); m != nil {
		ref.RunID = m[1]
		if len(m) > 2 {
			ref.JobID = m[2]
		}
		if rm := repoURLRe.FindStringSubmatch(input); rm != nil {
			ref.Owner = rm[1]
			ref.Repo = strings.TrimSuffix(rm[2], ".git")
		}
	} else if isAllDigits(strings.TrimSpace(input)) {
		ref.RunID = strings.TrimSpace(input)
	} else {
		return runRef{}, fmt.Errorf("could not parse a run URL or run ID from %q", input)
	}

	if repoOverride != "" {
		owner, repo, ok := splitOwnerRepo(repoOverride)
		if !ok {
			return runRef{}, fmt.Errorf("--repo must be owner/repo, got %q", repoOverride)
		}
		ref.Owner, ref.Repo = owner, repo
	}
	if ref.Owner == "" || ref.Repo == "" {
		owner, repo, _ := splitOwnerRepo(defaultTriageRepo)
		ref.Owner, ref.Repo = owner, repo
	}
	if ref.RunID == "" {
		return runRef{}, fmt.Errorf("no run ID found in %q", input)
	}
	return ref, nil
}

func splitOwnerRepo(s string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(s), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// fetchFailingJobLog resolves the failing job for the ref (unless a job is already
// pinned) and returns its raw log plus the job metadata (nil if a job was pinned
// by ID and the jobs list was not fetched).
func fetchFailingJobLog(ctx context.Context, ref runRef) (string, *ghJob, error) {
	var chosen *ghJob

	if ref.JobID == "" {
		out, err := ghAPI(ctx, []string{
			fmt.Sprintf("/repos/%s/%s/actions/runs/%s/jobs?per_page=100", ref.Owner, ref.Repo, ref.RunID),
		})
		if err != nil {
			return "", nil, fmt.Errorf("list jobs for run %s: %w (is `gh` installed and authenticated?)", ref.RunID, err)
		}
		var resp ghJobsResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return "", nil, fmt.Errorf("parse jobs response: %w", err)
		}
		job := selectFailedJob(resp.Jobs)
		if job == nil {
			return "", nil, fmt.Errorf(
				"no failed job found directly in run %s — integration failures often live in a nested reusable workflow that this run's job list does not include.\n"+
					"Open the failing job on GitHub and pass its URL (…/actions/runs/<id>/job/<jobid>), or `deploy-camunda triage --job <jobid> <run-id>`.",
				ref.RunID)
		}
		chosen = job
		ref.JobID = strconv.FormatInt(job.ID, 10)
	} else {
		// A job was pinned by ID/URL — fetch its metadata so the report still
		// carries the job name and matrix cell. Best-effort: log parsing works
		// even if this fails.
		if out, err := ghAPI(ctx, []string{
			fmt.Sprintf("/repos/%s/%s/actions/jobs/%s", ref.Owner, ref.Repo, ref.JobID),
		}); err == nil {
			var job ghJob
			if json.Unmarshal(out, &job) == nil && job.ID != 0 {
				chosen = &job
			}
		}
	}

	logOut, err := ghAPI(ctx, []string{
		fmt.Sprintf("/repos/%s/%s/actions/jobs/%s/logs", ref.Owner, ref.Repo, ref.JobID),
	})
	if err != nil {
		return "", chosen, fmt.Errorf("fetch log for job %s: %w", ref.JobID, err)
	}
	return string(logOut), chosen, nil
}

// selectFailedJob returns the first job whose conclusion is "failure", preferring
// one whose name looks like an integration matrix cell.
func selectFailedJob(jobs []ghJob) *ghJob {
	var firstFailed *ghJob
	for i := range jobs {
		if jobs[i].Conclusion != "failure" {
			continue
		}
		if firstFailed == nil {
			firstFailed = &jobs[i]
		}
		if !parseMatrixCell(jobs[i].Name).empty() {
			return &jobs[i]
		}
	}
	return firstFailed
}

func firstFailedStep(job ghJob) string {
	for _, s := range job.Steps {
		if s.Conclusion == "failure" {
			return s.Name
		}
	}
	return ""
}

// parseMatrixCell decodes an integration job name like
// "8.9 - cprst - install - pr - gke" into its coordinates. Job names carry extra
// prose (e.g. "install for install on gke - cprst"); only the leading
// " - "-delimited cell is parsed, and a non-version leading token yields empty.
func parseMatrixCell(name string) matrixCell {
	parts := strings.Split(name, " - ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) < 3 {
		return matrixCell{}
	}
	// The leading token must look like a chart version (e.g. 8.9, 8.10).
	if !regexp.MustCompile(`^\d+\.\d+$`).MatchString(parts[0]) {
		return matrixCell{}
	}
	cell := matrixCell{Version: parts[0]}
	if len(parts) > 1 {
		cell.Shortname = parts[1]
	}
	if len(parts) > 2 {
		cell.Flow = parts[2]
	}
	if len(parts) > 3 {
		cell.Case = firstToken(parts[3])
	}
	if len(parts) > 4 {
		// Integration job names append prose after the cell (e.g.
		// "gke / gke - ITs / ..."); keep only the leading platform token.
		cell.Platform = firstToken(parts[4])
	}
	return cell
}

// firstToken returns the first whitespace-delimited word of s.
func firstToken(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// extractFailureRecord pulls the structured failure fields out of a raw job log.
// The log is ANSI-stripped up front so extraction and matching see clean text.
func extractFailureRecord(rawLog string) failureRecord {
	log := stripANSI(rawLog)
	rec := failureRecord{}

	rec.HelmCommand = extractHelmCommand(log)
	if m := diagnosticsRe.FindStringSubmatch(log); m != nil {
		rec.DiagnosticsDir = m[1]
	}
	rec.Reason = extractReason(log)
	rec.Signals = collectSignals(log)
	return rec
}

// extractHelmCommand returns the real helm invocation from the (ANSI-stripped) log.
// It prefers the quoted command="helm ..." field the matrix runner emits, then a
// line that names a release/namespace, over the error-message line
// ("helm upgrade --install failed: exit status 1"), which carries no such args.
func extractHelmCommand(log string) string {
	if m := quotedHelmCmdRe.FindStringSubmatch(log); m != nil {
		return strings.TrimSpace(m[1])
	}
	var fallback string
	for _, line := range strings.Split(log, "\n") {
		idx := strings.Index(line, "helm upgrade")
		if idx < 0 {
			idx = strings.Index(line, "helm install")
		}
		if idx < 0 {
			continue
		}
		// Skip prose/comment mentions: a shell comment or narrative sentence that
		// merely refers to "helm upgrade" is not a command (e.g. script echoes
		// like "# ...so helm upgrade").
		if before := strings.TrimSpace(stripTimestamp(line[:idx])); strings.Contains(before, "#") {
			continue
		}
		cmd := strings.TrimSpace(stripTimestamp(line[idx:]))
		// Cut a trailing quoted-field tail if the invocation was embedded in a
		// larger structured log line (e.g. ... version=8.9).
		if q := strings.Index(cmd, `" `); q >= 0 {
			cmd = cmd[:q]
		}
		// A real invocation carries flags/args; a bare "helm upgrade" with nothing
		// after it is prose, not a command.
		if !looksLikeHelmInvocation(cmd) {
			continue
		}
		if strings.Contains(cmd, " -n ") || strings.Contains(cmd, "--namespace") || strings.Contains(cmd, "camunda-platform-") {
			return cmd
		}
		if fallback == "" {
			fallback = cmd
		}
	}
	return fallback
}

// looksLikeHelmInvocation reports whether cmd is a real helm command line rather
// than a prose mention — it must carry a flag or a release/chart argument.
func looksLikeHelmInvocation(cmd string) bool {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(cmd, "helm upgrade"), "helm install"))
	if rest == "" {
		return false
	}
	return strings.Contains(cmd, " -") || strings.Contains(cmd, "camunda-platform-") || strings.Contains(cmd, "integration")
}

// reasonPatterns are ordered most-specific-first; the first match wins.
var reasonPatterns = []string{
	"Matrix entry failed",
	"context deadline exceeded",
	"UPGRADE FAILED",
	"INSTALLATION FAILED",
	"Error: 1 of",
	"Error:",
}

func extractReason(log string) string {
	lines := strings.Split(stripANSI(log), "\n")
	// Priority order over the whole log: a more-specific reason (e.g. "Matrix
	// entry failed") wins even if a generic one appears earlier in the log.
	for _, p := range reasonPatterns {
		for _, line := range lines {
			if strings.Contains(line, p) {
				return strings.TrimSpace(stripTimestamp(line))
			}
		}
	}
	return ""
}

// signalPatterns are notable failure fingerprints worth surfacing verbatim.
var signalPatterns = []string{
	"Multi-Attach",
	"context deadline exceeded",
	"PodInitializing",
	"CrashLoopBackOff",
	"ImagePullBackOff",
	"ErrImagePull",
	"FailedMount",
	"FailedScheduling",
	"OOMKilled",
	"Unschedulable",
	"exceeded its progress deadline",
	"waiting to start",
}

func collectSignals(log string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range signalPatterns {
		if strings.Contains(log, p) && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// tsPrefixRe matches a leading ISO-8601 timestamp GitHub prepends to every log line.
var tsPrefixRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s*`)

func stripTimestamp(line string) string {
	return tsPrefixRe.ReplaceAllString(line, "")
}

// stripANSI removes ANSI SGR color escapes from log text.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// envDumpRe matches a step's echoed env entry: leading whitespace then
// "UPPER_SNAKE: value". These dominate integration logs and carry no signal.
var envDumpRe = regexp.MustCompile(`^\s*[A-Z][A-Z0-9_]{2,}:\s`)

// stripLogNoise removes the timestamp prefix and the per-step env-var dumps that
// bury the signal in integration logs, so a human (or a follow-up grep) reads only
// the meaningful lines.
func stripLogNoise(log string) string {
	var b strings.Builder
	for _, line := range strings.Split(log, "\n") {
		clean := stripTimestamp(stripANSI(line))
		if envDumpRe.MatchString(clean) {
			continue
		}
		b.WriteString(clean)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// renderTriageReport formats the failure record for the terminal.
func renderTriageReport(ref runRef, rec failureRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Triage: %s/%s run %s\n", ref.Owner, ref.Repo, ref.RunID)
	fmt.Fprintf(&b, "  https://github.com/%s/%s/actions/runs/%s\n\n", ref.Owner, ref.Repo, ref.RunID)

	if rec.JobName != "" {
		fmt.Fprintf(&b, "Job:          %s", rec.JobName)
		if rec.Conclusion != "" {
			fmt.Fprintf(&b, "  (%s)", rec.Conclusion)
		}
		b.WriteByte('\n')
	}
	if !rec.Cell.empty() {
		fmt.Fprintf(&b, "Matrix cell:  version=%s shortname=%s flow=%s", rec.Cell.Version, rec.Cell.Shortname, rec.Cell.Flow)
		if rec.Cell.Platform != "" {
			fmt.Fprintf(&b, " platform=%s", rec.Cell.Platform)
		}
		b.WriteByte('\n')
	}
	if rec.FailingStep != "" {
		fmt.Fprintf(&b, "Failing step: %s\n", rec.FailingStep)
	}
	if rec.Reason != "" {
		fmt.Fprintf(&b, "Reason:       %s\n", rec.Reason)
	}
	if len(rec.Signals) > 0 {
		fmt.Fprintf(&b, "Signals:      %s\n", strings.Join(rec.Signals, ", "))
	}
	if rec.HelmCommand != "" {
		fmt.Fprintf(&b, "Helm command: %s\n", rec.HelmCommand)
	}
	if rec.DiagnosticsDir != "" {
		fmt.Fprintf(&b, "Diagnostics:  %s\n", rec.DiagnosticsDir)
		fmt.Fprintf(&b, "  download:   gh run download %s --repo %s/%s --name 'diagnostics-*'\n", ref.RunID, ref.Owner, ref.Repo)
	}

	if repro := reproCommand(rec.Cell); repro != "" {
		fmt.Fprintf(&b, "\nReproduce locally:\n  %s\n", repro)
	}
	return b.String()
}

// reproCommand builds the deploy-camunda matrix run invocation that reproduces the
// failing cell locally. Returns "" when the cell could not be decoded.
func reproCommand(c matrixCell) string {
	if c.empty() {
		return ""
	}
	flow := c.Flow
	if flow == "" {
		flow = "install"
	}
	platform := c.Platform
	if platform == "" || platform == "pr" {
		platform = "gke"
	}
	return fmt.Sprintf(
		"deploy-camunda matrix run --repo-root . --versions %s --shortname-filter %s --shortname-exact --flow-filter %s --platform %s --delete-namespace --timeout 20 --yes",
		c.Version, c.Shortname, flow, platform,
	)
}

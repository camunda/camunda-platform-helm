package matrix

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Braille spinner frames for running entries.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// EntryStatus represents the execution state of a matrix entry.
type EntryStatus int

const (
	StatusPending EntryStatus = iota
	StatusRunning
	StatusPassed
	StatusFailed
	StatusSkipped
)

func (s EntryStatus) label() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusPassed:
		return "pass"
	case StatusFailed:
		return "FAIL"
	case StatusSkipped:
		return "skip"
	default:
		return "?"
	}
}

// ANSI escape codes. Used directly (not via gchalk/logging) because
// logging.ColorEnabled may be false when logs are redirected to a file,
// but the status display still writes to a real TTY.
const (
	ansiReset   = "\033[0m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiHideCur = "\033[?25l"
	ansiShowCur = "\033[?25h"
)

// entryState tracks the runtime state of a single matrix entry.
type entryState struct {
	Entry     Entry
	Status    EntryStatus
	Phase     string // current phase (e.g., "preparing", "deploying", "testing")
	Namespace string
	StartTime time.Time
	Duration  time.Duration
	Error     string
}

// StatusDisplay renders an in-place status table for matrix entries.
// It is safe for concurrent use from multiple goroutines.
type StatusDisplay struct {
	mu           sync.Mutex
	out          io.Writer
	states       map[string]*entryState
	order        []string // ordered entry IDs for stable rendering
	startTime    time.Time
	lastLines    int  // lines printed in last render (for ANSI cursor rewind)
	colorEnabled bool // whether to emit ANSI color codes
	logDir       string
	isTerminal   bool
	spinnerIdx   int // incremented by ticker for animated spinner

	// Ticker lifecycle.
	stopTicker chan struct{}
	tickerDone chan struct{}
}

// NewStatusDisplay creates a status display for the given matrix entries.
// When isTerminal is true, it renders an in-place ANSI table on out and
// starts a 1-second refresh ticker for live elapsed times and spinner.
// When false, it emits one-line-per-event output suitable for CI.
// logDir is the directory for per-entry summary files (empty disables file output).
func NewStatusDisplay(out io.Writer, entries []Entry, isTerminal bool, logDir string) *StatusDisplay {
	states := make(map[string]*entryState, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		id := entryID(e)
		states[id] = &entryState{Entry: e, Status: StatusPending}
		order = append(order, id)
	}
	d := &StatusDisplay{
		out:          out,
		states:       states,
		order:        order,
		startTime:    time.Now(),
		colorEnabled: isTerminal,
		logDir:       logDir,
		isTerminal:   isTerminal,
		stopTicker:   make(chan struct{}),
		tickerDone:   make(chan struct{}),
	}
	if isTerminal {
		fmt.Fprint(out, ansiHideCur) // hide cursor for cleaner redraws
		go d.tickerLoop()
	}
	return d
}

// tickerLoop re-renders every second while entries are running,
// keeping elapsed times and the spinner animation up to date.
func (d *StatusDisplay) tickerLoop() {
	defer close(d.tickerDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.mu.Lock()
			d.spinnerIdx++
			hasRunning := false
			for _, id := range d.order {
				if d.states[id].Status == StatusRunning {
					hasRunning = true
					break
				}
			}
			if hasRunning {
				d.render()
			}
			d.mu.Unlock()
		case <-d.stopTicker:
			return
		}
	}
}

// Stop halts the refresh ticker and restores the terminal cursor.
// Call this after matrix.Run returns, before printing the summary.
func (d *StatusDisplay) Stop() {
	if !d.isTerminal {
		return
	}
	close(d.stopTicker)
	<-d.tickerDone
	fmt.Fprint(d.out, ansiShowCur)
}

// OnEntryStart updates the display when a matrix entry begins execution.
// Pass this as RunOptions.OnEntryStart.
func (d *StatusDisplay) OnEntryStart(entry Entry, namespace string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := entryID(entry)
	if s, ok := d.states[id]; ok {
		s.Status = StatusRunning
		s.Namespace = namespace
		s.StartTime = time.Now()
	}
	if d.isTerminal {
		d.render()
	} else {
		fmt.Fprintf(d.out, "[%s] %s STARTED (%s)\n",
			fmtDuration(time.Since(d.startTime)), id, namespace)
	}
}

// OnPhaseChange updates the display when a matrix entry transitions to a new phase.
// Pass this as RunOptions.OnPhaseChange.
func (d *StatusDisplay) OnPhaseChange(entry Entry, phase string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := entryID(entry)
	if s, ok := d.states[id]; ok {
		s.Phase = phase
	}
	if d.isTerminal {
		d.render()
	} else {
		fmt.Fprintf(d.out, "[%s] %s → %s\n",
			fmtDuration(time.Since(d.startTime)), id, phase)
	}
}

// OnEntryComplete updates the display when a matrix entry finishes execution.
// Pass this as RunOptions.OnEntryComplete.
func (d *StatusDisplay) OnEntryComplete(entry Entry, result RunResult) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := entryID(entry)
	if s, ok := d.states[id]; ok {
		s.Duration = result.Duration
		if result.Error != nil {
			if result.Duration == 0 && strings.Contains(result.Error.Error(), "skipped") {
				s.Status = StatusSkipped
			} else {
				s.Status = StatusFailed
				errMsg := result.Error.Error()
				if len(errMsg) > 72 {
					errMsg = errMsg[:69] + "..."
				}
				s.Error = errMsg
			}
		} else {
			s.Status = StatusPassed
		}
	}
	if d.isTerminal {
		d.render()
	} else {
		st := d.states[id]
		if st.Error != "" {
			fmt.Fprintf(d.out, "[%s] %s %s (%s) — %s\n",
				fmtDuration(time.Since(d.startTime)), id, st.Status.label(),
				fmtDuration(st.Duration), st.Error)
		} else {
			fmt.Fprintf(d.out, "[%s] %s %s (%s)\n",
				fmtDuration(time.Since(d.startTime)), id, st.Status.label(),
				fmtDuration(st.Duration))
		}
	}

	// Write per-entry summary file.
	if d.logDir != "" {
		d.writeEntrySummary(entry, result)
	}
}

// render draws the full status table in-place using ANSI cursor movement.
// Must be called with d.mu held.
func (d *StatusDisplay) render() {
	var b strings.Builder

	// Rewind cursor to overwrite the previous render.
	if d.lastLines > 0 {
		for i := 0; i < d.lastLines; i++ {
			b.WriteString("\033[A\033[2K")
		}
	}

	// Compute aggregate stats.
	var running, passed, failed, pending, skipped int
	for _, id := range d.order {
		switch d.states[id].Status {
		case StatusRunning:
			running++
		case StatusPassed:
			passed++
		case StatusFailed:
			failed++
		case StatusSkipped:
			skipped++
		case StatusPending:
			pending++
		}
	}
	done := passed + failed + skipped
	total := len(d.order)
	elapsed := fmtDuration(time.Since(d.startTime))

	// ── Top rule ──
	fmt.Fprintf(&b, "\n  %s\n", d.color("━━━ matrix run ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", ansiDim))

	// ── Progress bar ──
	barWidth := 24
	filledWidth := 0
	if total > 0 {
		filledWidth = (done * barWidth) / total
	}
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)
	barColor := ansiCyan
	if failed > 0 {
		barColor = ansiRed
	} else if done == total && total > 0 {
		barColor = ansiGreen
	}
	fmt.Fprintf(&b, "\n  %s  %d/%d complete    elapsed %s\n",
		d.color(bar, barColor), done, total, d.color(elapsed, ansiBold))

	// ── Counters ──
	fmt.Fprintf(&b, "\n  %s    %s    %s    %s\n",
		d.color(fmt.Sprintf("✓ %d", passed), ansiGreen),
		d.color(fmt.Sprintf("✗ %d", failed), ansiRed),
		d.color(fmt.Sprintf("↻ %d", running), ansiYellow),
		d.color(fmt.Sprintf("○ %d", pending+skipped), ansiDim))
	fmt.Fprintln(&b)

	// ── Column headers ──
	// Pad raw text BEFORE wrapping in ANSI to keep alignment correct.
	fmt.Fprintf(&b, "  %s  %s  %s  %s  %s  %s  %s\n",
		d.color(pad("#", 3), ansiBold),
		d.color(pad("VER", 6), ansiBold),
		d.color(pad("SHORT", 8), ansiBold),
		d.color(pad("FLOW", 22), ansiBold),
		d.color(pad("STATUS", 12), ansiBold),
		d.color(pad("TIME", 7), ansiBold),
		d.color("NAMESPACE", ansiBold))

	// ── Entry rows ──
	for i, id := range d.order {
		s := d.states[id]

		// Status cell: pad raw text, then color.
		statusText := d.fmtStatusText(s.Status, s.Phase)
		statusCell := d.fmtStatusColored(statusText, s.Status)

		// Time cell.
		durText := d.color("-", ansiDim)
		if s.Status == StatusRunning && !s.StartTime.IsZero() {
			durText = d.color(fmtDuration(time.Since(s.StartTime)), ansiYellow)
		} else if s.Duration > 0 {
			durText = fmtDuration(s.Duration)
		}

		// Namespace cell.
		ns := d.color("-", ansiDim)
		if s.Namespace != "" {
			ns = s.Namespace
		}

		flow := s.Entry.Flow
		if len(flow) > 22 {
			flow = flow[:19] + "..."
		}

		fmt.Fprintf(&b, "  %-3d  %-6s  %-8s  %-22s  %s  %-7s  %s\n",
			i+1, s.Entry.Version, s.Entry.Shortname, flow,
			statusCell, durText, ns)

		// Inline error preview for failed entries.
		if s.Status == StatusFailed && s.Error != "" {
			fmt.Fprintf(&b, "  %s%s\n",
				strings.Repeat(" ", 5),
				d.color("↳ "+s.Error, ansiRed))
		}
	}

	// ── Footer ──
	fmt.Fprintln(&b)
	if d.logDir != "" {
		logFile := filepath.Join(d.logDir, "matrix-run.log")
		fmt.Fprintf(&b, "  %s  tail -f %s\n",
			d.color("logs", ansiBold), d.color(logFile, ansiCyan))
	}
	fmt.Fprintf(&b, "  %s\n", d.color("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", ansiDim))

	output := b.String()
	d.lastLines = strings.Count(output, "\n")
	fmt.Fprint(d.out, output)
}

// Clear erases the status table from the terminal, leaving a clean slate
// for the post-run summary.
func (d *StatusDisplay) Clear() {
	if !d.isTerminal || d.lastLines == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var b strings.Builder
	for i := 0; i < d.lastLines; i++ {
		b.WriteString("\033[A\033[2K")
	}
	fmt.Fprint(d.out, b.String())
	d.lastLines = 0
}

// fmtStatusText returns the raw (uncolored, fixed-width) status string.
func (d *StatusDisplay) fmtStatusText(s EntryStatus, phase string) string {
	switch s {
	case StatusRunning:
		frame := spinnerFrames[d.spinnerIdx%len(spinnerFrames)]
		if phase != "" {
			return frame + " " + phase
		}
		return frame + " running"
	case StatusPassed:
		return "✓ pass"
	case StatusFailed:
		return "✗ FAIL"
	case StatusSkipped:
		return "⊘ skip"
	default:
		return "· pending"
	}
}

// fmtStatusColored returns the status string padded to 12 display-chars
// and wrapped in the appropriate ANSI color.
func (d *StatusDisplay) fmtStatusColored(text string, s EntryStatus) string {
	padded := pad(text, 12)
	switch s {
	case StatusRunning:
		return d.color(padded, ansiYellow)
	case StatusPassed:
		return d.color(padded, ansiGreen)
	case StatusFailed:
		return d.color(padded, ansiRed)
	default:
		return d.color(padded, ansiDim)
	}
}

// color wraps s in an ANSI color code when color is enabled.
func (d *StatusDisplay) color(s, code string) string {
	if !d.colorEnabled {
		return s
	}
	return code + s + ansiReset
}

// pad right-pads s with spaces to width characters.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// ── Per-entry summary files ──────────────────────────────────────────

func (d *StatusDisplay) writeEntrySummary(entry Entry, result RunResult) {
	name := entryLogFileName(entry)
	path := filepath.Join(d.logDir, name+".summary")

	var b strings.Builder
	fmt.Fprintf(&b, "Entry:     %s\n", entryID(entry))
	fmt.Fprintf(&b, "Scenario:  %s\n", entry.Scenario)
	fmt.Fprintf(&b, "Auth:      %s\n", entry.Auth)
	fmt.Fprintf(&b, "Platform:  %s\n", entry.Platform)
	fmt.Fprintf(&b, "Namespace: %s\n", result.Namespace)
	fmt.Fprintf(&b, "Duration:  %s\n", result.Duration.Round(time.Second))

	if result.Error != nil {
		fmt.Fprintf(&b, "Status:    FAIL\n")
		fmt.Fprintf(&b, "Error:     %s\n", result.Error)
		if result.Diagnostics != "" {
			fmt.Fprintf(&b, "Diagnostics: %s\n", result.Diagnostics)
		}
	} else {
		fmt.Fprintf(&b, "Status:    PASS\n")
	}

	// Best-effort write; don't fail the run on file I/O errors.
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
}

// ── Helpers ──────────────────────────────────────────────────────────

// entryID returns a unique slash-separated identifier for display purposes.
func entryID(e Entry) string {
	id := e.Version + "/" + e.Shortname + "/" + e.Flow
	if e.Platform != "" {
		id += "/" + e.Platform
	}
	return id
}

// entryLogFileName returns a filesystem-safe name for per-entry log files.
func entryLogFileName(e Entry) string {
	parts := []string{e.Version, e.Shortname, e.Flow}
	if e.Platform != "" {
		parts = append(parts, e.Platform)
	}
	return strings.Join(parts, "-")
}

// fmtDuration formats a duration as "Xs" (under 1 min) or "M:SS".
func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

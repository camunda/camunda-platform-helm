package matrix

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
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
//
// In terminal mode, rendering is handled by a bubbletea program that manages
// cursor positioning, repainting, and Unicode width automatically.
// In non-terminal mode (CI), it emits one-line-per-event output.
type StatusDisplay struct {
	program *tea.Program // nil when !isTerminal

	// Fields below are used only in non-terminal mode.
	mu        sync.Mutex
	out       io.Writer
	states    map[string]*entryState
	order     []string
	startTime time.Time
	logDir    string

	isTerminal bool
}

// NewStatusDisplay creates a status display for the given matrix entries.
// When isTerminal is true, it launches a bubbletea program for live rendering.
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
		out:        out,
		states:     states,
		order:      order,
		startTime:  time.Now(),
		logDir:     logDir,
		isTerminal: isTerminal,
	}

	if isTerminal {
		// Clone the states map so the bubbletea model owns its state exclusively.
		// This avoids a shared-mutable-pointer hazard between the facade and the model.
		modelStates := make(map[string]*entryState, len(states))
		for k, v := range states {
			clone := *v
			modelStates[k] = &clone
		}
		model := newStatusModel(modelStates, order, logDir)
		d.program = tea.NewProgram(model,
			tea.WithOutput(out),
			tea.WithoutSignalHandler(), // parent context handles SIGINT/SIGTERM
		)
		d.states = nil // not used in terminal mode; model owns the state
		go d.program.Run() //nolint:errcheck // best-effort UI; errors are non-fatal
	}

	return d
}

// Stop halts the live display and restores the terminal.
// Call this after matrix.Run returns, before printing the summary.
func (d *StatusDisplay) Stop() {
	if !d.isTerminal || d.program == nil {
		return
	}
	d.program.Send(stopMsg{})
	d.program.Wait()
}

// Clear is a no-op when using bubbletea (the final empty View() clears the table).
// Retained for API compatibility.
func (d *StatusDisplay) Clear() {
	// bubbletea handles cleanup: returning "" from View() on quit clears the display.
}

// OnEntryStart updates the display when a matrix entry begins execution.
func (d *StatusDisplay) OnEntryStart(entry Entry, namespace string) {
	if d.isTerminal {
		d.program.Send(entryStartMsg{Entry: entry, Namespace: namespace})
	} else {
		d.mu.Lock()
		defer d.mu.Unlock()
		id := entryID(entry)
		if s, ok := d.states[id]; ok {
			s.Status = StatusRunning
			s.Namespace = namespace
			s.StartTime = time.Now()
		}
		fmt.Fprintf(d.out, "[%s] %s STARTED (%s)\n",
			fmtDuration(time.Since(d.startTime)), entryID(entry), namespace)
	}
}

// OnPhaseChange updates the display when a matrix entry transitions to a new phase.
func (d *StatusDisplay) OnPhaseChange(entry Entry, phase string) {
	if d.isTerminal {
		d.program.Send(phaseChangeMsg{Entry: entry, Phase: phase})
	} else {
		d.mu.Lock()
		defer d.mu.Unlock()
		id := entryID(entry)
		if s, ok := d.states[id]; ok {
			s.Phase = phase
		}
		fmt.Fprintf(d.out, "[%s] %s → %s\n",
			fmtDuration(time.Since(d.startTime)), id, phase)
	}
}

// OnEntryComplete updates the display when a matrix entry finishes execution.
func (d *StatusDisplay) OnEntryComplete(entry Entry, result RunResult) {
	if d.isTerminal {
		d.program.Send(entryCompleteMsg{Entry: entry, Result: result})
	} else {
		d.mu.Lock()
		id := entryID(entry)
		if s, ok := d.states[id]; ok {
			applyResult(s, result)
		}
		st := d.states[id]
		label := st.Status.label()
		dur := st.Duration
		errStr := st.Error
		d.mu.Unlock()

		if errStr != "" {
			fmt.Fprintf(d.out, "[%s] %s %s (%s) — %s\n",
				fmtDuration(time.Since(d.startTime)), id, label,
				fmtDuration(dur), errStr)
		} else {
			fmt.Fprintf(d.out, "[%s] %s %s (%s)\n",
				fmtDuration(time.Since(d.startTime)), id, label,
				fmtDuration(dur))
		}
	}

	// Write per-entry summary file (outside bubbletea to avoid blocking the render loop).
	if d.logDir != "" {
		d.writeEntrySummary(entry, result)
	}
}

// applyResult updates an entryState from a RunResult.
func applyResult(s *entryState, result RunResult) {
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

// pad right-pads s with spaces to width display columns,
// using runewidth to correctly measure Unicode character widths.
func pad(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

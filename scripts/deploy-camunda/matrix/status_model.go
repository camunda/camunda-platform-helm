package matrix

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// ── Messages sent from StatusDisplay callbacks into the bubbletea program ──

type entryStartMsg struct {
	Entry     Entry
	Namespace string
}

type phaseChangeMsg struct {
	Entry Entry
	Phase string
}

type entryCompleteMsg struct {
	Entry  Entry
	Result RunResult
}

type stopMsg struct{}

type tickMsg time.Time

// ── Styles ──

var (
	styleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleBold   = lipgloss.NewStyle().Bold(true)
)

// ── Model ──

type statusModel struct {
	states     map[string]*entryState
	order      []string
	startTime  time.Time
	logDir     string
	spinnerIdx int
	quitting   bool
}

func newStatusModel(states map[string]*entryState, order []string, logDir string) statusModel {
	return statusModel{
		states:    states,
		order:     order,
		startTime: time.Now(),
		logDir:    logDir,
	}
}

func (m statusModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.spinnerIdx++
		return m, tickCmd()

	case entryStartMsg:
		id := entryID(msg.Entry)
		if s, ok := m.states[id]; ok {
			s.Status = StatusRunning
			s.Namespace = msg.Namespace
			s.StartTime = time.Now()
		}
		return m, nil

	case phaseChangeMsg:
		id := entryID(msg.Entry)
		if s, ok := m.states[id]; ok {
			s.Phase = msg.Phase
		}
		return m, nil

	case entryCompleteMsg:
		id := entryID(msg.Entry)
		if s, ok := m.states[id]; ok {
			applyResult(s, msg.Result)
		}
		return m, nil

	case stopMsg:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		// Allow ctrl+c to quit the display.
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m statusModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	var b strings.Builder

	// Aggregate stats.
	var running, passed, failed, pending, skipped int
	for _, id := range m.order {
		switch m.states[id].Status {
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
	total := len(m.order)
	elapsed := fmtDuration(time.Since(m.startTime))

	// ── Top rule ──
	fmt.Fprintf(&b, "  %s\n", styleDim.Render("━━━ matrix run ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

	// ── Progress bar ──
	barWidth := 24
	filledWidth := 0
	if total > 0 {
		filledWidth = (done * barWidth) / total
	}
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)
	barStyle := styleCyan
	if failed > 0 {
		barStyle = styleRed
	} else if done == total && total > 0 {
		barStyle = styleGreen
	}
	fmt.Fprintf(&b, "  %s  %d/%d complete    elapsed %s\n",
		barStyle.Render(bar), done, total, styleBold.Render(elapsed))

	// ── Counters ──
	fmt.Fprintf(&b, "  %s    %s    %s    %s\n",
		styleGreen.Render(fmt.Sprintf("✓ %d", passed)),
		styleRed.Render(fmt.Sprintf("✗ %d", failed)),
		styleYellow.Render(fmt.Sprintf("↻ %d", running)),
		styleDim.Render(fmt.Sprintf("○ %d", pending+skipped)))

	// ── Column headers ──
	fmt.Fprintf(&b, "  %s  %s  %s  %s  %s  %s  %s\n",
		styleBold.Render(pad("#", 3)),
		styleBold.Render(pad("VER", 6)),
		styleBold.Render(pad("SHORT", 8)),
		styleBold.Render(pad("FLOW", 22)),
		styleBold.Render(pad("STATUS", 12)),
		styleBold.Render(pad("TIME", 7)),
		styleBold.Render("NAMESPACE"))

	// ── Entry rows ──
	for i, id := range m.order {
		s := m.states[id]

		statusText := m.fmtStatusText(s.Status, s.Phase)
		statusCell := m.fmtStatusColored(statusText, s.Status)

		var durRaw string
		if s.Status == StatusRunning && !s.StartTime.IsZero() {
			durRaw = fmtDuration(time.Since(s.StartTime))
		} else if s.Duration > 0 {
			durRaw = fmtDuration(s.Duration)
		} else {
			durRaw = "-"
		}
		// Pad before styling so width is computed on plain text.
		durPadded := pad(durRaw, 7)
		switch {
		case s.Status == StatusRunning && !s.StartTime.IsZero():
			durPadded = styleYellow.Render(durPadded)
		case s.Duration == 0:
			durPadded = styleDim.Render(durPadded)
		}

		ns := styleDim.Render("-")
		if s.Namespace != "" {
			ns = s.Namespace
		}

		flow := s.Entry.Flow
		if len(flow) > 22 {
			flow = flow[:19] + "..."
		}

		fmt.Fprintf(&b, "  %-3d  %-6s  %-8s  %-22s  %s  %s  %s\n",
			i+1, s.Entry.Version, s.Entry.Shortname, flow,
			statusCell, durPadded, ns)

		if s.Status == StatusFailed && s.Error != "" {
			fmt.Fprintf(&b, "  %s%s\n",
				strings.Repeat(" ", 5),
				styleRed.Render("↳ "+s.Error))
		}
	}

	// ── Footer ──
	fmt.Fprintln(&b)
	if m.logDir != "" {
		logFile := filepath.Join(m.logDir, "matrix-run.log")
		fmt.Fprintf(&b, "  %s  tail -f %s\n",
			styleBold.Render("logs"), styleCyan.Render(logFile))
	}
	fmt.Fprintf(&b, "  %s\n", styleDim.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

	return tea.NewView(b.String())
}

// fmtStatusText returns the raw (uncolored) status string.
func (m statusModel) fmtStatusText(s EntryStatus, phase string) string {
	switch s {
	case StatusRunning:
		frame := spinnerFrames[m.spinnerIdx%len(spinnerFrames)]
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

// fmtStatusColored returns the status string padded and styled.
func (m statusModel) fmtStatusColored(text string, s EntryStatus) string {
	padded := pad(text, 12)
	switch s {
	case StatusRunning:
		return styleYellow.Render(padded)
	case StatusPassed:
		return styleGreen.Render(padded)
	case StatusFailed:
		return styleRed.Render(padded)
	default:
		return styleDim.Render(padded)
	}
}

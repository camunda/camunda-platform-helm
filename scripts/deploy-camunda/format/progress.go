package format

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"scripts/camunda-core/pkg/logging"

	"github.com/jwalton/gchalk"
)

// Step represents a deployment step.
type Step struct {
	Name   string
	Status StepStatus
}

// StepStatus represents the status of a step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepCompleted
	StepFailed
	StepSkipped
)

// Progress tracks deployment progress through multiple steps.
type Progress struct {
	mu        sync.Mutex
	steps     []Step
	current   int
	startTime time.Time
	scenario  string
	isTTY     bool
}

// NewProgress creates a new progress tracker for a deployment.
func NewProgress(scenario string, steps []string) *Progress {
	s := make([]Step, len(steps))
	for i, name := range steps {
		s[i] = Step{Name: name, Status: StepPending}
	}
	return &Progress{
		steps:     s,
		current:   -1,
		startTime: time.Now(),
		scenario:  scenario,
		isTTY:     logging.IsTerminal(os.Stdout.Fd()),
	}
}

// DefaultDeploymentSteps returns the standard deployment steps.
func DefaultDeploymentSteps() []string {
	return []string{
		"Validating configuration",
		"Preparing values files",
		"Generating secrets",
		"Creating namespace",
		"Running helm deployment",
		"Waiting for pods",
	}
}

// Start begins tracking a step.
func (p *Progress) Start(stepIndex int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if stepIndex < 0 || stepIndex >= len(p.steps) {
		return
	}

	p.current = stepIndex
	p.steps[stepIndex].Status = StepRunning
	p.print()
}

// Complete marks the current step as completed.
func (p *Progress) Complete(stepIndex int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if stepIndex < 0 || stepIndex >= len(p.steps) {
		return
	}

	p.steps[stepIndex].Status = StepCompleted
	p.print()
}

// Fail marks the current step as failed.
func (p *Progress) Fail(stepIndex int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if stepIndex < 0 || stepIndex >= len(p.steps) {
		return
	}

	p.steps[stepIndex].Status = StepFailed
	p.print()
}

// Skip marks a step as skipped.
func (p *Progress) Skip(stepIndex int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if stepIndex < 0 || stepIndex >= len(p.steps) {
		return
	}

	p.steps[stepIndex].Status = StepSkipped
	p.print()
}

// print outputs the current progress state.
func (p *Progress) print() {
	if p.isTTY {
		p.printTTY()
	} else {
		p.printCI()
	}
}

// printTTY outputs pretty progress for interactive terminals.
func (p *Progress) printTTY() {
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleRun := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleSkip := func(s string) string { return logging.Emphasize(s, gchalk.Dim) }
	styleNum := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }

	var b strings.Builder

	// Header with scenario name and elapsed time
	elapsed := time.Since(p.startTime).Round(time.Second)
	if p.scenario != "" {
		fmt.Fprintf(&b, "[%s] ", styleNum(p.scenario))
	}
	fmt.Fprintf(&b, "Deployment Progress (%s)\n", elapsed)

	// Progress bar
	completed := 0
	for _, s := range p.steps {
		if s.Status == StepCompleted {
			completed++
		}
	}
	barWidth := 30
	filled := (completed * barWidth) / len(p.steps)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Fprintf(&b, "  [%s] %d/%d\n\n", bar, completed, len(p.steps))

	// Individual steps
	for i, step := range p.steps {
		var icon, status string
		switch step.Status {
		case StepPending:
			icon = "○"
			status = step.Name
		case StepRunning:
			icon = styleRun("●")
			status = styleRun(step.Name + "...")
		case StepCompleted:
			icon = styleOk("✓")
			status = styleOk(step.Name)
		case StepFailed:
			icon = styleErr("✗")
			status = styleErr(step.Name)
		case StepSkipped:
			icon = styleSkip("○")
			status = styleSkip(step.Name + " (skipped)")
		}
		fmt.Fprintf(&b, "  %s %s %s\n", styleNum(fmt.Sprintf("[%d/%d]", i+1, len(p.steps))), icon, status)
	}

	logging.Logger.Info().Msg(b.String())
}

// printCI outputs simple progress for CI/CD environments.
func (p *Progress) printCI() {
	// Find the current step being updated
	for i, step := range p.steps {
		switch step.Status {
		case StepRunning:
			logging.Logger.Info().
				Str("scenario", p.scenario).
				Int("step", i+1).
				Int("total", len(p.steps)).
				Str("status", "running").
				Msg(step.Name)
		case StepCompleted:
			// Only log completion once (when it just changed)
			if i == p.current {
				elapsed := time.Since(p.startTime).Round(time.Second)
				logging.Logger.Info().
					Str("scenario", p.scenario).
					Int("step", i+1).
					Int("total", len(p.steps)).
					Str("status", "completed").
					Str("elapsed", elapsed.String()).
					Msg(step.Name)
			}
		case StepFailed:
			if i == p.current {
				logging.Logger.Error().
					Str("scenario", p.scenario).
					Int("step", i+1).
					Int("total", len(p.steps)).
					Str("status", "failed").
					Msg(step.Name)
			}
		case StepSkipped:
			if i == p.current {
				logging.Logger.Info().
					Str("scenario", p.scenario).
					Int("step", i+1).
					Int("total", len(p.steps)).
					Str("status", "skipped").
					Msg(step.Name)
			}
		}
	}
}

// Summary outputs a final summary of the deployment.
func (p *Progress) Summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Since(p.startTime).Round(time.Second)

	completed := 0
	failed := 0
	skipped := 0
	for _, s := range p.steps {
		switch s.Status {
		case StepCompleted:
			completed++
		case StepFailed:
			failed++
		case StepSkipped:
			skipped++
		}
	}

	if !p.isTTY {
		return fmt.Sprintf("Deployment completed: %d/%d steps succeeded, %d failed, %d skipped in %s",
			completed, len(p.steps), failed, skipped, elapsed)
	}

	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }

	var b strings.Builder
	b.WriteString(styleHead("Deployment Summary"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Total time: %s\n", elapsed)
	fmt.Fprintf(&b, "  Steps completed: %s\n", styleOk(fmt.Sprintf("%d/%d", completed, len(p.steps))))
	if failed > 0 {
		fmt.Fprintf(&b, "  Steps failed: %s\n", styleErr(fmt.Sprintf("%d", failed)))
	}
	if skipped > 0 {
		fmt.Fprintf(&b, "  Steps skipped: %d\n", skipped)
	}

	return b.String()
}

// ProgressStep is a helper for running a step with automatic progress tracking.
type ProgressStep struct {
	progress *Progress
	index    int
}

// Run executes a function and tracks its progress.
func (ps *ProgressStep) Run(fn func() error) error {
	ps.progress.Start(ps.index)
	err := fn()
	if err != nil {
		ps.progress.Fail(ps.index)
		return err
	}
	ps.progress.Complete(ps.index)
	return nil
}

// Step returns a ProgressStep for the given step index.
func (p *Progress) Step(index int) *ProgressStep {
	return &ProgressStep{
		progress: p,
		index:    index,
	}
}


package agentwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// tickRecord mirrors the on-disk shape persisted by persistTick. We re-declare
// it here (rather than exporting the runtime type) because the eval flow
// only reads recorded fields and is tolerant of newer/older shapes.
type tickRecord struct {
	Timestamp string          `json:"timestamp"`
	Snapshot  json.RawMessage `json:"snapshot"`
	Verdict   *Verdict        `json:"verdict,omitempty"`
}

// ReplayResult is the outcome of replaying a single tick: the recorded and
// freshly-produced verdicts side by side, plus a regression flag.
type ReplayResult struct {
	File          string
	Recorded      *Verdict
	Replayed      *Verdict
	Error         string
	ActionChanged bool
	// ConfidenceDelta is replayed.Confidence - recorded.Confidence. Useful
	// for spotting silent calibration drift.
	ConfidenceDelta float64
}

// ReplayReport summarizes a corpus replay.
type ReplayReport struct {
	CorpusDir   string
	Results     []ReplayResult
	Regressions int
	Skipped     int
	Duration    time.Duration
}

// Replay re-runs the agent CLI over every tick record in dir and returns a
// ReplayReport comparing recorded verdicts to fresh ones. Tick files that
// have no recorded verdict (i.e. the original run failed to parse one) are
// skipped — they are an input to prompt iteration, not regressions.
func Replay(ctx context.Context, cli AgentCLI, dir string) (ReplayReport, error) {
	start := time.Now()
	files, err := listTickFiles(dir)
	if err != nil {
		return ReplayReport{}, err
	}
	report := ReplayReport{CorpusDir: dir, Results: make([]ReplayResult, 0, len(files))}

	for _, f := range files {
		select {
		case <-ctx.Done():
			report.Duration = time.Since(start)
			return report, ctx.Err()
		default:
		}

		rec, err := readTickRecord(f)
		if err != nil {
			report.Results = append(report.Results, ReplayResult{File: f, Error: err.Error()})
			report.Skipped++
			continue
		}
		if rec.Verdict == nil {
			report.Skipped++
			continue
		}

		raw, err := Invoke(ctx, cli, DefaultPrompt, rec.Snapshot)
		if err != nil {
			report.Results = append(report.Results, ReplayResult{
				File: f, Recorded: rec.Verdict, Error: err.Error(),
			})
			continue
		}
		fresh, err := ParseVerdict(raw)
		if err != nil {
			report.Results = append(report.Results, ReplayResult{
				File: f, Recorded: rec.Verdict, Error: "parse: " + err.Error(),
			})
			continue
		}

		result := ReplayResult{
			File:            f,
			Recorded:        rec.Verdict,
			Replayed:        &fresh,
			ActionChanged:   fresh.RecommendedAction != rec.Verdict.RecommendedAction,
			ConfidenceDelta: fresh.Confidence - rec.Verdict.Confidence,
		}
		if result.ActionChanged {
			report.Regressions++
		}
		report.Results = append(report.Results, result)
	}

	report.Duration = time.Since(start)
	return report, nil
}

// Format renders a human-readable summary for stdout.
func (r ReplayReport) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Replay corpus: %s\n", r.CorpusDir)
	fmt.Fprintf(&b, "Total: %d   Regressions: %d   Skipped: %d   Duration: %s\n\n",
		len(r.Results), r.Regressions, r.Skipped, r.Duration.Round(time.Millisecond))
	for _, res := range r.Results {
		base := filepath.Base(res.File)
		if res.Error != "" {
			fmt.Fprintf(&b, "  ERROR  %s  %s\n", base, res.Error)
			continue
		}
		marker := "  ok  "
		if res.ActionChanged {
			marker = "REGRESS"
		}
		fmt.Fprintf(&b, "  %s  %s  %s -> %s  Δconf=%+.2f\n",
			marker, base,
			res.Recorded.RecommendedAction, res.Replayed.RecommendedAction,
			res.ConfidenceDelta)
	}
	return b.String()
}

func listTickFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read corpus dir %q: %w", dir, err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

func readTickRecord(path string) (tickRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return tickRecord{}, err
	}
	var rec tickRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return tickRecord{}, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return rec, nil
}

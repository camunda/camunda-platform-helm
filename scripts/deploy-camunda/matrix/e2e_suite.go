package matrix

import (
	"path/filepath"

	"scripts/camunda-core/pkg/logging"
)

// E2ESuite is a scenario's resolved Playwright configuration: which project the
// after-install e2e job runs, and whether its failure fails the workflow run.
type E2ESuite struct {
	FullSuite   bool
	NonBlocking bool
}

// ResolveE2ESuite returns the E2ESuite declared by the named scenario in the
// registry under <repoRoot>/charts/<chartDir>.
//
// The zero value — the smoke-tests project, blocking — is returned for chart
// versions without a registry and for scenario names absent from it, so a
// caller passing an unregistered scenario keeps the historical behavior instead
// of failing. Lookup is by scenario Name and ignores the manifest's `enabled`
// flag: scenarios invoked directly by an external caller (the AlwaysGreen gate)
// are deliberately absent from the generated PR matrix.
func ResolveE2ESuite(repoRoot, chartDir, scenario string) (E2ESuite, error) {
	absChartDir := filepath.Join(repoRoot, "charts", chartDir)
	if !HasRegistry(absChartDir) {
		logging.Logger.Warn().
			Str("chartDir", chartDir).
			Msg("No CI scenario registry — defaulting to the smoke-tests project")
		return E2ESuite{}, nil
	}

	cfg, err := LoadRegistry(absChartDir)
	if err != nil {
		return E2ESuite{}, err
	}

	for _, scn := range cfg.Integration.Case.PR.Scenarios {
		if scn.Name == scenario {
			return E2ESuite{FullSuite: scn.E2EFullSuite, NonBlocking: scn.E2ENonBlocking}, nil
		}
	}

	logging.Logger.Warn().
		Str("chartDir", chartDir).
		Str("scenario", scenario).
		Msg("Scenario not found in the CI scenario registry — defaulting to the smoke-tests project")
	return E2ESuite{}, nil
}

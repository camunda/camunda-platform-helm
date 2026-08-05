package deploy

import (
	"bytes"
	"testing"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"

	"github.com/stretchr/testify/require"
)

func TestDeploymentSummariesDoNotOutputCredentials(t *testing.T) {
	result := &ScenarioResult{
		Scenario:  "canary",
		Namespace: "canary-namespace",
		Release:   "canary-release",
	}
	flags := &config.RuntimeFlags{}

	for _, testCase := range []struct {
		name  string
		print func()
	}{
		{name: "single scenario", print: func() { printDeploymentSummary(result, flags) }},
		{name: "multiple scenarios", print: func() { printMultiScenarioSummary([]*ScenarioResult{result}, flags) }},
	} {
		for _, terminal := range []bool{false, true} {
			t.Run(testCase.name, func(t *testing.T) {
				originalIsTerminal := isTerminal
				isTerminal = func(uintptr) bool { return terminal }
				t.Cleanup(func() { isTerminal = originalIsTerminal })

				var output bytes.Buffer
				require.NoError(t, logging.Setup(logging.Options{Writer: &output, ColorEnabled: false}))

				testCase.print()

				require.Contains(t, output.String(), "canary-namespace")
				require.NotContains(t, output.String(), "credential")
				require.NotContains(t, output.String(), "password")
				require.NotContains(t, output.String(), "secret")
			})
		}
	}
}

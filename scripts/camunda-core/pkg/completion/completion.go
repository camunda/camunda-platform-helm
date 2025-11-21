package completion

import (
	"scripts/camunda-core/pkg/scenarios"

	"github.com/spf13/cobra"
)

// RegisterScenarioCompletion adds tab completion for the scenario flag.
// It expects the command to have a flag for chart path (e.g., "chart" or "chart-path").
func RegisterScenarioCompletion(cmd *cobra.Command, flagName string, chartFlagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		chartPath, _ := cmd.Flags().GetString(chartFlagName)
		if chartPath == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --"+chartFlagName+" first to resolve scenarios"), cobra.ShellCompDirectiveNoFileComp
		}

		list, err := scenarios.List(chartPath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return list, cobra.ShellCompDirectiveNoFileComp
	})
}


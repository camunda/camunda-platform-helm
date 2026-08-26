// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package completion

import (
	"scripts/camunda-core/pkg/scenarios"

	"github.com/spf13/cobra"
)

// RegisterScenarioCompletion adds tab completion for the scenario flag.
// It expects the command to have a flag for chart path (e.g., "chart" or "chart-path").
func RegisterScenarioCompletion(cmd *cobra.Command, flagName string, scenarioDirFlagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioDir, _ := cmd.Flags().GetString(scenarioDirFlagName)
		if scenarioDir == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --"+scenarioDirFlagName+" first to resolve scenarios"), cobra.ShellCompDirectiveNoFileComp
		}

		list, err := scenarios.List(scenarioDir)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return list, cobra.ShellCompDirectiveNoFileComp
	})
}

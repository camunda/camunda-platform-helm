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

package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

var e2eExecutionFlags = []struct {
	flag string
	env  string
}{
	{"--playwright-project", "PLAYWRIGHT_PROJECT"},
	{"--file-pattern", "FILE_PATTERN"},
	{"--is-rba", "IS_RBA_OVERRIDE"},
	{"--is-mt", "IS_MT_OVERRIDE"},
	{"--is-ds", "IS_DS_OVERRIDE"},
	{"--is-license-key", "IS_LICENSE_KEY_OVERRIDE"},
	{"--is-migration", "IS_MIGRATION_OVERRIDE"},
	{"--is-opensearch", "IS_OPENSEARCH_OVERRIDE"},
	{"--mcp-gateway-enabled", "MCP_GATEWAY_ENABLED_OVERRIDE"},
}

func newCIE2ERunConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "e2e-run-config [run-e2e-tests arguments]",
		Short:              "Extract explicit E2E execution controls for the shell runner",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeE2ERunConfig(cmd.OutOrStdout(), args)
		},
	}
	return cmd
}

func writeE2ERunConfig(w io.Writer, args []string) error {
	values := make(map[string]string, len(e2eExecutionFlags))
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		name := ""
		for _, executionFlag := range e2eExecutionFlags {
			if args[i] == executionFlag.flag {
				name = executionFlag.env
				break
			}
		}
		if name == "" {
			remaining = append(remaining, args[i])
			continue
		}
		if i+1 >= len(args) {
			return fmt.Errorf("flag %s needs an argument", args[i])
		}
		i++
		values[name] = args[i]
	}

	for _, executionFlag := range e2eExecutionFlags {
		if _, err := fmt.Fprintf(w, "%s=%s\n", executionFlag.env, quoteBash(values[executionFlag.env])); err != nil {
			return err
		}
	}
	requireSM810Suite := values["FILE_PATTERN"] != "" || values["PLAYWRIGHT_PROJECT"] != "" && values["PLAYWRIGHT_PROJECT"] != "auth0-smoke"
	if _, err := fmt.Fprintf(w, "REQUIRE_SM_810_TEST_SUITE_OVERRIDE=%s\n", quoteBash(fmt.Sprintf("%t", requireSM810Suite))); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "apply_e2e_execution_env() {"); err != nil {
		return err
	}
	for _, name := range []string{"IS_RBA", "IS_MT", "IS_DS", "IS_LICENSE_KEY", "IS_MIGRATION", "IS_OPENSEARCH", "MCP_GATEWAY_ENABLED"} {
		if _, err := fmt.Fprintf(w, "  [[ -n \"$%s_OVERRIDE\" ]] && export %s=\"$%s_OVERRIDE\"\n", name, name, name); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "  [[ \"$REQUIRE_SM_810_TEST_SUITE_OVERRIDE\" == \"true\" ]] && export REQUIRE_SM_810_TEST_SUITE=true"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  return 0\n}"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "set -- %s\n", quoteBashArgs(remaining))
	return err
}

func quoteBashArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quoteBash(arg)
	}
	return strings.Join(quoted, " ")
}

func quoteBash(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

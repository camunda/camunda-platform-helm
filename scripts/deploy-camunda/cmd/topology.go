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

	"github.com/spf13/cobra"

	"scripts/deploy-camunda/deploy"
)

// newTopologyCommand groups multi-namespace topology utility subcommands.
func newTopologyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "topology",
		Short: "Multi-namespace topology utilities",
	}
	c.AddCommand(newTopologyNamespaceCommand())
	return c
}

// newTopologyNamespaceCommand prints the release namespace a topology
// deploy would derive for the given base namespace and release suffix,
// applying the same 63-character truncation as generateTopologyContexts.
func newTopologyNamespaceCommand() *cobra.Command {
	var (
		base   string
		suffix string
	)

	cmd := &cobra.Command{
		Use:   "namespace",
		Short: "Print the derived release namespace for a base namespace and suffix",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, err := deploy.DeriveReleaseNamespace(base, suffix)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), namespace)
			return nil
		},
	}

	cmd.Flags().StringVar(&base, "base", "", "base namespace")
	cmd.Flags().StringVar(&suffix, "suffix", "", "release namespace suffix")
	_ = cmd.MarkFlagRequired("base")
	_ = cmd.MarkFlagRequired("suffix")

	return cmd
}

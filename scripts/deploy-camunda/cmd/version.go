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
	"runtime/debug"

	"github.com/spf13/cobra"
)

const unknownBuildValue = "unknown"

type buildStamp struct {
	Revision  string
	Committed string
	Modified  string
	GoVersion string
}

func readBuildStamp() buildStamp {
	stamp := buildStamp{
		Revision:  unknownBuildValue,
		Committed: unknownBuildValue,
		Modified:  unknownBuildValue,
		GoVersion: unknownBuildValue,
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return stamp
	}
	if info.GoVersion != "" {
		stamp.GoVersion = info.GoVersion
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			stamp.Revision = setting.Value
		case "vcs.time":
			stamp.Committed = setting.Value
		case "vcs.modified":
			stamp.Modified = setting.Value
		}
	}
	return stamp
}

func (s buildStamp) write(w io.Writer) {
	fmt.Fprintf(w, "revision:  %s\n", s.Revision)
	fmt.Fprintf(w, "committed: %s\n", s.Committed)
	fmt.Fprintf(w, "modified:  %s\n", s.Modified)
	fmt.Fprintf(w, "go:        %s\n", s.GoVersion)
}

func newVersionCommand() *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the source revision this binary was built from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stamp := readBuildStamp()
			out := cmd.OutOrStdout()
			if short {
				fmt.Fprintln(out, stamp.Revision)
				return nil
			}
			stamp.write(out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "Print only the source revision")
	return cmd
}

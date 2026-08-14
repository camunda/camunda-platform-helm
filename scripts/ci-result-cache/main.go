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

package main

// ci-result-cache: progressive CI result caching for merge queue optimization.
// See cmd/ for the available subcommands.

import (
	"errors"
	"fmt"
	"os"

	"scripts/ci-result-cache/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// ErrNotCached means the check command already printed its
		// "NOT CACHED" message — no additional output needed.
		if !errors.Is(err, cmd.ErrNotCached) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

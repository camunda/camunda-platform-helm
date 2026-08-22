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

import (
	"flag"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/logging"
	"vault-secret-mapper/pkg/mapper"
)

func main() {
	// Initialize logger with default options
	_ = logging.Setup(logging.Options{
		LevelString:  "info",
		ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
	})

	mapping := flag.String("mapping", "", "Vault secret mapping content (multi-line, semicolon-terminated entries)")
	secretName := flag.String("secret-name", "vault-mapped-secrets", "Kubernetes Secret name to generate")
	outputPath := flag.String("output", "", "Path to write the generated Secret YAML")
	strict := flag.Bool("strict", false, "Fail if any mapped env var is unset, instead of omitting it from the Secret")
	flag.Parse()

	if *outputPath == "" {
		exitWithError("missing required flag: --output")
	}
	if *mapping == "" {
		exitWithError("missing required flag: --mapping")
	}

	generate := mapper.Generate
	if *strict {
		generate = mapper.GenerateStrict
	}
	if err := generate(*mapping, *secretName, *outputPath); err != nil {
		exitWithError("%v", err)
	}
}

func exitWithError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "vault-secret-mapper error: %s\n", msg)
	os.Exit(1)
}

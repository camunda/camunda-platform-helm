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

	"scripts/camunda-core/pkg/chartmeta"
)

// runComponentImageVersions builds the component-image-versions annotation block
// from the chart's values.yaml and prints it to stdout, e.g.:
//
//	release-tools component-image-versions --chart-dir "$dir" --camunda-version 8.10 > /tmp/image-versions.yaml
//	yq -i '.annotations."camunda.io/component-image-versions" = load_str("/tmp/image-versions.yaml")' Chart.yaml
//
// The component set is version-gated (8.8+ orchestration vs 8.6–8.7 classic).
func runComponentImageVersions(args []string) error {
	fs := flag.NewFlagSet("component-image-versions", flag.ContinueOnError)
	var (
		chartDir       string
		camundaVersion string
	)
	fs.StringVar(&chartDir, "chart-dir", "", "chart directory (e.g. charts/camunda-platform-<v>)")
	fs.StringVar(&camundaVersion, "camunda-version", "", "Camunda minor line, e.g. 8.10")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chartDir == "" || camundaVersion == "" {
		return fmt.Errorf("--chart-dir and --camunda-version are required")
	}

	block, err := chartmeta.ComponentImageVersions(chartDir, camundaVersion)
	if err != nil {
		return fmt.Errorf("build component-image-versions: %w", err)
	}
	if block == "" {
		return fmt.Errorf("empty component-image-versions for %s; refusing to record", chartDir)
	}
	_, err = fmt.Fprint(os.Stdout, block)
	return err
}

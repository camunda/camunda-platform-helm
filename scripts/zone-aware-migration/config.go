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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var errHelp = errors.New("help requested")

type config struct {
	chartDir    string
	scenarioDir string
	namespace   string
	release     string
	timeout     string
}

func parseConfig(args []string, lookupEnv func(string) string, stderr io.Writer) (config, error) {
	if lookupEnv == nil {
		lookupEnv = os.Getenv
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cwd, err := os.Getwd()
	if err != nil {
		return config{}, fmt.Errorf("get working directory: %w", err)
	}
	cfg := config{
		chartDir:  envOrDefault(lookupEnv, "CHART_DIR", filepath.Join(cwd, "charts/camunda-platform-8.10")),
		namespace: "zone-aware-migration",
		release:   envOrDefault(lookupEnv, "RELEASE", "zam"),
		timeout:   envOrDefault(lookupEnv, "TIMEOUT", "5m"),
	}
	flags := flag.NewFlagSet("zone-aware-migration", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: zone-aware-migration [namespace]")
		fmt.Fprintln(stderr, "Environment: CHART_DIR, RELEASE, TIMEOUT")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return config{}, errHelp
		}
		return config{}, fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return config{}, fmt.Errorf("expected at most one namespace argument")
	}
	if flags.NArg() == 1 {
		cfg.namespace = flags.Arg(0)
	}
	cfg.scenarioDir = filepath.Join(cfg.chartDir, "test/integration/scenarios/zone-aware-migration")
	return cfg, nil
}

func envOrDefault(lookupEnv func(string) string, key, fallback string) string {
	if value := lookupEnv(key); value != "" {
		return value
	}
	return fallback
}

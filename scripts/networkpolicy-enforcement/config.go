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
	"time"
)

// renovate: datasource=github-releases depName=projectcalico/calico
const calicoVersion = "v3.30.3"

var errHelp = errors.New("help requested")

type config struct {
	chartDir       string
	scenarioDir    string
	namespace      string
	release        string
	timeout        string
	installCalico  bool
	calicoManifest string
	retries        int
	retryInterval  time.Duration
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
		chartDir:       envOrDefault(lookupEnv, "CHART_DIR", filepath.Join(cwd, "charts/camunda-platform-8.11")),
		namespace:      "netpol",
		release:        envOrDefault(lookupEnv, "RELEASE", "netpol"),
		timeout:        envOrDefault(lookupEnv, "TIMEOUT", "5m"),
		calicoManifest: envOrDefault(lookupEnv, "CALICO_MANIFEST", "https://raw.githubusercontent.com/projectcalico/calico/"+calicoVersion+"/manifests/calico.yaml"),
		retries:        10,
		retryInterval:  3 * time.Second,
	}
	flags := flag.NewFlagSet("networkpolicy-enforcement", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&cfg.installCalico, "install-calico", false, "install Calico before the scenarios")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: networkpolicy-enforcement [-install-calico] [namespace]")
		fmt.Fprintln(stderr, "Environment: CHART_DIR, RELEASE, TIMEOUT, CALICO_MANIFEST")
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
	cfg.scenarioDir = filepath.Join(cfg.chartDir, "test/integration/scenarios/networkpolicy")
	return cfg, nil
}

func envOrDefault(lookupEnv func(string) string, key, fallback string) string {
	if value := lookupEnv(key); value != "" {
		return value
	}
	return fallback
}

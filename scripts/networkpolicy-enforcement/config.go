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
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// renovate: datasource=github-releases depName=projectcalico/calico
const calicoVersion = "v3.30.3"

// nodeExternalPort is the kube-apiserver port of the default external egress
// target: the first node's InternalIP.
const nodeExternalPort = 6443

// defaultExternalPort is used when EXTERNAL_TARGET names a host without a port.
const defaultExternalPort = 443

var errHelp = errors.New("help requested")

type config struct {
	chartDir       string
	scenarioDir    string
	namespace      string
	release        string
	namespace2     string
	release2       string
	timeout        string
	installCalico  bool
	calicoManifest string
	externalHost   string
	externalPort   int
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
		release2:       envOrDefault(lookupEnv, "RELEASE2", "netpol-split"),
		timeout:        envOrDefault(lookupEnv, "TIMEOUT", "5m"),
		calicoManifest: envOrDefault(lookupEnv, "CALICO_MANIFEST", "https://raw.githubusercontent.com/projectcalico/calico/"+calicoVersion+"/manifests/calico.yaml"),
		retries:        10,
		retryInterval:  3 * time.Second,
	}
	cfg.externalHost, cfg.externalPort, err = parseExternalTarget(lookupEnv("EXTERNAL_TARGET"))
	if err != nil {
		return config{}, err
	}
	flags := flag.NewFlagSet("networkpolicy-enforcement", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&cfg.installCalico, "install-calico", false, "install Calico before the scenarios")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: networkpolicy-enforcement [-install-calico] [namespace]")
		fmt.Fprintln(stderr, "Environment: CHART_DIR, RELEASE, RELEASE2, TIMEOUT, CALICO_MANIFEST, EXTERNAL_TARGET")
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
	cfg.namespace2 = cfg.namespace + "-allowed"
	cfg.scenarioDir = filepath.Join(cfg.chartDir, "test/integration/scenarios/networkpolicy")
	return cfg, nil
}

// parseExternalTarget resolves EXTERNAL_TARGET into the host and port the
// external egress probe dials. An empty value keeps the default target: the
// first node's InternalIP on nodeExternalPort. A value without a port uses
// defaultExternalPort. IPv6 hosts must be bracketed when a port is given.
func parseExternalTarget(raw string) (string, int, error) {
	if raw == "" {
		return "", nodeExternalPort, nil
	}
	host, portString, err := net.SplitHostPort(raw)
	if err != nil {
		if strings.Contains(raw, ":") && net.ParseIP(strings.Trim(raw, "[]")) == nil {
			return "", 0, fmt.Errorf("invalid EXTERNAL_TARGET %q: %w", raw, err)
		}
		return strings.Trim(raw, "[]"), defaultExternalPort, nil
	}
	if host == "" {
		return "", 0, fmt.Errorf("invalid EXTERNAL_TARGET %q: missing host", raw)
	}
	port, err := strconv.Atoi(portString)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid EXTERNAL_TARGET %q: invalid port %q", raw, portString)
	}
	return host, port, nil
}

func envOrDefault(lookupEnv func(string) string, key, fallback string) string {
	if value := lookupEnv(key); value != "" {
		return value
	}
	return fallback
}

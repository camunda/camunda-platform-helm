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
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeCommander struct {
	commands []string
	outputs  map[string]string
	errors   map[string]error
}

func (f *fakeCommander) run(_ context.Context, name string, args ...string) (string, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	f.commands = append(f.commands, command)
	return f.outputs[command], f.errors[command]
}

func TestParseConfig_usesCompatibilityDefaults(t *testing.T) {
	var stderr bytes.Buffer

	cfg, err := parseConfig(nil, nil, &stderr)

	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.namespace != "zone-aware-migration" || cfg.release != "zam" || cfg.timeout != "5m" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestParseConfig_acceptsPositionalNamespaceAndEnvironment(t *testing.T) {
	env := map[string]string{"CHART_DIR": "/chart", "RELEASE": "custom", "TIMEOUT": "9m"}

	cfg, err := parseConfig([]string{"other"}, envLookup(env), &bytes.Buffer{})

	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.namespace != "other" || cfg.chartDir != "/chart" || cfg.release != "custom" || cfg.timeout != "9m" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseConfig_helpReturnsHelp(t *testing.T) {
	_, err := parseConfig([]string{"--help"}, nil, &bytes.Buffer{})

	if !errors.Is(err, errHelp) {
		t.Fatalf("expected help, got %v", err)
	}
}

func TestRunner_rejectsExistingNamespaceWithoutCleanup(t *testing.T) {
	cmd := &fakeCommander{outputs: map[string]string{"kubectl get namespace occupied --ignore-not-found -o name": "namespace/occupied"}, errors: map[string]error{}}
	runner := runner{cfg: config{namespace: "occupied"}, command: cmd.run}

	err := runner.run(context.Background())

	if !errors.Is(err, errNamespaceExists) {
		t.Fatalf("expected existing namespace error, got %v", err)
	}
	if len(cmd.commands) != 1 {
		t.Fatalf("unexpected commands: %v", cmd.commands)
	}
}

func TestRunner_cleansOnlyResourcesItCreatedAfterFailure(t *testing.T) {
	cmd := successfulFake()
	install := "helm install zam /chart --namespace fresh --values /scenario/values-numbered.yaml --timeout 5m"
	cmd.errors[install] = errors.New("install failed")
	runner := runner{cfg: testConfig(), command: cmd.run}

	err := runner.run(context.Background())

	if err == nil {
		t.Fatal("expected failure")
	}
	wantLast := "kubectl delete namespace fresh --wait=false"
	if cmd.commands[len(cmd.commands)-1] != wantLast {
		t.Fatalf("last command = %q, want %q", cmd.commands[len(cmd.commands)-1], wantLast)
	}
	for _, command := range cmd.commands {
		if strings.HasPrefix(command, "helm uninstall") {
			t.Fatalf("unowned release cleanup: %v", cmd.commands)
		}
	}
}

func TestRunner_waitsForBothRolloutsBeforeComparingRetainedUID(t *testing.T) {
	cmd := successfulFake()
	runner := runner{cfg: testConfig(), command: cmd.run}

	if err := runner.run(context.Background()); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	zonedWait := commandIndex(cmd.commands, "kubectl rollout status statefulset/zam-zeebe-zone-a --namespace fresh --timeout 5m")
	retainedWait := commandIndexAfter(cmd.commands, "kubectl rollout status statefulset/zam-zeebe --namespace fresh --timeout 5m", zonedWait)
	uidAfter := commandIndexAfter(cmd.commands, "kubectl get pod zam-zeebe-0 --namespace fresh -o jsonpath={.metadata.uid}", retainedWait)
	if zonedWait < 0 || retainedWait < 0 || uidAfter < 0 {
		t.Fatalf("rollout and UID order missing: %v", cmd.commands)
	}
}

func successfulFake() *fakeCommander {
	uid := "kubectl get pod zam-zeebe-0 --namespace fresh -o jsonpath={.metadata.uid}"
	zonedUID := "kubectl get pod zam-zeebe-zone-a-0 --namespace fresh -o jsonpath={.metadata.uid}"
	return &fakeCommander{outputs: map[string]string{
		"kubectl get namespace fresh --ignore-not-found -o name": "",
		uid: "numbered-uid", zonedUID: "zoned-uid",
		"kubectl get statefulset/zam-zeebe --namespace fresh -o jsonpath={.spec.replicas}":                      "1",
		"kubectl get pod zam-zeebe-0 --namespace fresh -o jsonpath={.status.containerStatuses[0].restartCount}": "0",
		"kubectl get pod zam-zeebe-zone-a-0 --namespace fresh -o jsonpath={.status.phase}":                      "Running",
	}, errors: map[string]error{}}
}

func TestRunner_upgradesBaseChartBeforeRecordingMigrationUID(t *testing.T) {
	cmd := successfulFake()
	cfg := testConfig()
	cfg.baseChartDir = "/base-chart"
	r := runner{cfg: cfg, command: cmd.run}
	if err := r.run(context.Background()); err != nil {
		t.Fatalf("run base upgrade: %v", err)
	}
	install := commandIndex(cmd.commands, "helm install zam /base-chart --namespace fresh --values /scenario/values-numbered.yaml --timeout 5m")
	baseWait := commandIndexAfter(cmd.commands, "kubectl rollout status statefulset/zam-zeebe --namespace fresh --timeout 5m", install)
	upgrade := commandIndexAfter(cmd.commands, "helm upgrade zam /chart --namespace fresh --values /scenario/values-numbered.yaml --timeout 5m", baseWait)
	headWait := commandIndexAfter(cmd.commands, "kubectl rollout status statefulset/zam-zeebe --namespace fresh --timeout 5m", upgrade)
	uid := commandIndexAfter(cmd.commands, "kubectl get pod zam-zeebe-0 --namespace fresh -o jsonpath={.metadata.uid}", headWait)
	if install < 0 || baseWait < 0 || upgrade < 0 || headWait < 0 || uid < 0 {
		t.Fatalf("base install and settled chart-only upgrade must precede migration UID: %v", cmd.commands)
	}
}

func testConfig() config {
	return config{chartDir: "/chart", scenarioDir: "/scenario", namespace: "fresh", release: "zam", timeout: "5m"}
}

func envLookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func commandIndex(commands []string, want string) int {
	return commandIndexAfter(commands, want, -1)
}

func commandIndexAfter(commands []string, want string, after int) int {
	for index := after + 1; index < len(commands); index++ {
		if commands[index] == want {
			return index
		}
	}
	return -1
}

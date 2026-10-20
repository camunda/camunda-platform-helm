// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
// under one or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information regarding copyright
// ownership. Licensed under the Apache License, Version 2.0; you may not use
// this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	capacity "scripts/capacity-manager"
)

func main() {
	namespace := os.Getenv("TEST_NAMESPACE")
	release := os.Getenv("RELEASE_NAME")
	if namespace == "" || release == "" {
		fatalf("TEST_NAMESPACE and RELEASE_NAME are required")
	}

	// 1. Ramping load must saturate one broker and the scale-up must be
	// justified by backpressure and broker CPU, not throughput.
	up, err := waitFor(namespace, release, 2, 20*time.Minute)
	if err != nil {
		fatalf("load-triggered scale up verification: %v", err)
	}
	if err := requireChange(up.LastChange, 1, 2, "backpressure", "broker-cpu"); err != nil {
		fatalf("scale-up evidence: %v", err)
	}
	pressure := up.LastChange.Signals["backpressure"]
	fmt.Printf("scaled 1 -> 2 on %q (backpressure drop ratio %.3f, broker cpu %.2f)\n", up.LastChange.Reason, pressure, up.LastChange.Signals["broker-cpu"])

	// 2. The added broker must actually relieve the saturation.
	if err := waitForSignalBelow(namespace, release, "backpressure", pressure, 10*time.Minute); err != nil {
		fatalf("second broker did not relieve backpressure: %v", err)
	}

	// 3. Removing load must contract safely, justified by the scale-down signals.
	if _, err := kubectl("delete", "deployment", release+"-capacity-load", "-n", namespace, "--ignore-not-found", "--wait=true", "--timeout=2m"); err != nil {
		fatalf("stop load generator: %v", err)
	}
	down, err := waitFor(namespace, release, 1, 20*time.Minute)
	if err != nil {
		fatalf("idle-triggered scale down verification: %v", err)
	}
	if err := requireChange(down.LastChange, 2, 1, "no-backpressure", "fits-on-fewer-brokers"); err != nil {
		fatalf("scale-down evidence: %v", err)
	}
	fmt.Println("capacity manager verified backpressure-driven 1 -> 2 and idle-driven safe 2 -> 1 broker scaling")
}

func requireChange(change *capacity.CapacityChange, from, to int, signals ...string) error {
	if change == nil {
		return fmt.Errorf("status has no lastChange")
	}
	if change.From != from || change.To != to {
		return fmt.Errorf("last change was %d -> %d, want %d -> %d", change.From, change.To, from, to)
	}
	for _, signal := range signals {
		if !strings.Contains(change.Reason, signal) {
			return fmt.Errorf("reason %q does not name signal %q", change.Reason, signal)
		}
		if _, ok := change.Signals[signal]; !ok {
			return fmt.Errorf("signal %q has no recorded reading: %v", signal, change.Signals)
		}
	}
	return nil
}

func waitForSignalBelow(namespace, release, signal string, limit float64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := "no reading"
	for time.Now().Before(deadline) {
		status, err := getStatus(namespace, release)
		if err == nil {
			if value, ok := status.Signals[signal]; ok {
				last = strconv.FormatFloat(value, 'f', 3, 64)
				if value < limit {
					fmt.Printf("%s fell from %.3f to %s with two brokers\n", signal, limit, last)
					return nil
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("%s stayed at or above %.3f (last %s)", signal, limit, last)
}

func waitFor(namespace, release string, target int, timeout time.Duration) (capacity.Status, error) {
	deadline := time.Now().Add(timeout)
	var last capacity.Status
	for time.Now().Before(deadline) {
		status, err := getStatus(namespace, release)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		last = status
		replicas, err := getReplicas(namespace, release)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		brokers, err := getBrokerCount(namespace, release)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		if replicas < brokers {
			return status, fmt.Errorf("StatefulSet contracted before topology evacuation: replicas=%d brokers=%d status=%#v", replicas, brokers, status)
		}
		if brokers == target && replicas == target && status.Phase == "stable" {
			return status, nil
		}
		time.Sleep(5 * time.Second)
	}
	return last, fmt.Errorf("timed out waiting for %d brokers; last status %s", target, last.JSON())
}

func getReplicas(namespace, release string) (int, error) {
	output, err := kubectl("get", "statefulset", release+"-zeebe", "-n", namespace, "-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(output)))
}

func getBrokerCount(namespace, release string) (int, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s-zeebe-gateway:9600/proxy/orchestration/actuator/cluster", namespace, release)
	output, err := kubectl("get", "--raw", path)
	if err != nil {
		return 0, err
	}
	var topology capacity.Topology
	if err := json.Unmarshal(output, &topology); err != nil {
		return 0, err
	}
	return topology.ActiveBrokerCount(), nil
}

func getStatus(namespace, release string) (capacity.Status, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:%s-capacity-manager:8080/proxy/status", namespace, release)
	output, err := kubectl("get", "--raw", path)
	if err != nil {
		return capacity.Status{}, err
	}
	var status capacity.Status
	if err := json.Unmarshal(output, &status); err != nil {
		return capacity.Status{}, err
	}
	return status, nil
}

func kubectl(args ...string) ([]byte, error) {
	if context := os.Getenv("KUBE_CONTEXT"); context != "" {
		args = append([]string{"--context", context}, args...)
	}
	command := exec.Command("kubectl", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
